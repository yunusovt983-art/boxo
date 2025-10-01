// Package circuitbreaker provides circuit breaker pattern implementation
// for Bitswap operations to protect against overload and cascading failures.
package circuitbreaker

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/libp2p/go-libp2p/core/peer"
	logging "github.com/ipfs/go-log/v2"
)

var log = logging.Logger("bitswap/circuitbreaker")

// State represents the current state of the circuit breaker
type State int

const (
	// StateClosed - circuit is closed, requests are allowed through
	StateClosed State = iota
	// StateOpen - circuit is open, requests are rejected immediately
	StateOpen
	// StateHalfOpen - circuit is half-open, limited requests are allowed to test recovery
	StateHalfOpen
)

func (s State) String() string {
	switch s {
	case StateClosed:
		return "CLOSED"
	case StateOpen:
		return "OPEN"
	case StateHalfOpen:
		return "HALF_OPEN"
	default:
		return "UNKNOWN"
	}
}

// Config holds configuration parameters for the circuit breaker
type Config struct {
	// MaxRequests is the maximum number of requests allowed to pass through
	// when the circuit breaker is half-open
	MaxRequests uint32

	// Interval is the cyclic period of the closed state for the circuit breaker
	// to clear the internal counts
	Interval time.Duration

	// Timeout is the period of the open state, after which the state becomes half-open
	Timeout time.Duration

	// ReadyToTrip is called with a copy of Counts whenever a request fails in the closed state.
	// If ReadyToTrip returns true, the circuit breaker will be placed into the open state.
	ReadyToTrip func(counts Counts) bool

	// OnStateChange is called whenever the state of the circuit breaker changes
	OnStateChange func(name string, from State, to State)

	// IsSuccessful is used to determine whether a given error should be counted as a failure
	IsSuccessful func(err error) bool
}

// Counts holds the numbers of requests and their successes/failures
type Counts struct {
	Requests             uint32
	TotalSuccesses       uint32
	TotalFailures        uint32
	ConsecutiveSuccesses uint32
	ConsecutiveFailures  uint32
}

// CircuitBreaker implements the circuit breaker pattern for protecting Bitswap operations
type CircuitBreaker struct {
	name          string
	maxRequests   uint32
	interval      time.Duration
	timeout       time.Duration
	readyToTrip   func(counts Counts) bool
	isSuccessful  func(err error) bool
	onStateChange func(name string, from State, to State)

	mutex      sync.Mutex
	state      State
	generation uint64
	counts     Counts
	expiry     time.Time
}

// NewCircuitBreaker creates a new circuit breaker with the given configuration
func NewCircuitBreaker(name string, config Config) *CircuitBreaker {
	cb := &CircuitBreaker{
		name:        name,
		maxRequests: config.MaxRequests,
		interval:    config.Interval,
		timeout:     config.Timeout,
		readyToTrip: config.ReadyToTrip,
		isSuccessful: config.IsSuccessful,
		onStateChange: config.OnStateChange,
	}

	if cb.maxRequests == 0 {
		cb.maxRequests = 1
	}

	if cb.interval <= 0 {
		cb.interval = time.Duration(0)
	}

	if cb.timeout <= 0 {
		cb.timeout = 60 * time.Second
	}

	if cb.readyToTrip == nil {
		cb.readyToTrip = func(counts Counts) bool {
			return counts.ConsecutiveFailures > 5
		}
	}

	if cb.isSuccessful == nil {
		cb.isSuccessful = func(err error) bool {
			return err == nil
		}
	}

	cb.toNewGeneration(time.Now())

	return cb
}

// Execute runs the given request if the circuit breaker accepts it.
// Execute returns an error instantly if the circuit breaker rejects the request.
func (cb *CircuitBreaker) Execute(req func() error) error {
	generation, err := cb.beforeRequest()
	if err != nil {
		return err
	}

	defer func() {
		e := recover()
		if e != nil {
			cb.afterRequest(generation, false)
			panic(e)
		}
	}()

	result := req()
	cb.afterRequest(generation, cb.isSuccessful(result))
	return result
}

// Call is the same as Execute but allows for context cancellation
func (cb *CircuitBreaker) Call(ctx context.Context, req func(context.Context) error) error {
	generation, err := cb.beforeRequest()
	if err != nil {
		return err
	}

	defer func() {
		e := recover()
		if e != nil {
			cb.afterRequest(generation, false)
			panic(e)
		}
	}()

	result := req(ctx)
	cb.afterRequest(generation, cb.isSuccessful(result))
	return result
}

// State returns the current state of the circuit breaker
func (cb *CircuitBreaker) State() State {
	cb.mutex.Lock()
	defer cb.mutex.Unlock()

	now := time.Now()
	state, _ := cb.currentState(now)
	return state
}

// Counts returns a copy of the internal counts
func (cb *CircuitBreaker) Counts() Counts {
	cb.mutex.Lock()
	defer cb.mutex.Unlock()

	return cb.counts
}

// Name returns the name of the circuit breaker
func (cb *CircuitBreaker) Name() string {
	return cb.name
}

func (cb *CircuitBreaker) beforeRequest() (uint64, error) {
	cb.mutex.Lock()
	defer cb.mutex.Unlock()

	now := time.Now()
	state, generation := cb.currentState(now)

	if state == StateOpen {
		return generation, ErrOpenState
	} else if state == StateHalfOpen && cb.counts.Requests >= cb.maxRequests {
		return generation, ErrTooManyRequests
	}

	cb.counts.onRequest()
	return generation, nil
}

func (cb *CircuitBreaker) afterRequest(before uint64, success bool) {
	cb.mutex.Lock()
	defer cb.mutex.Unlock()

	now := time.Now()
	state, generation := cb.currentState(now)
	if generation != before {
		return
	}

	if success {
		cb.onSuccess(state, now)
	} else {
		cb.onFailure(state, now)
	}
}

func (cb *CircuitBreaker) onSuccess(state State, now time.Time) {
	cb.counts.onSuccess()

	if state == StateHalfOpen && cb.counts.ConsecutiveSuccesses >= cb.maxRequests {
		cb.setState(StateClosed, now)
	}
}

func (cb *CircuitBreaker) onFailure(state State, now time.Time) {
	cb.counts.onFailure()

	switch state {
	case StateClosed:
		if cb.readyToTrip(cb.counts) {
			cb.setState(StateOpen, now)
		}
	case StateHalfOpen:
		cb.setState(StateOpen, now)
	}
}

func (cb *CircuitBreaker) currentState(now time.Time) (State, uint64) {
	switch cb.state {
	case StateClosed:
		if !cb.expiry.IsZero() && cb.expiry.Before(now) {
			cb.toNewGeneration(now)
		}
	case StateOpen:
		if cb.expiry.Before(now) {
			cb.setState(StateHalfOpen, now)
		}
	}
	return cb.state, cb.generation
}

func (cb *CircuitBreaker) setState(state State, now time.Time) {
	if cb.state == state {
		return
	}

	prev := cb.state
	cb.state = state

	cb.toNewGeneration(now)

	if cb.onStateChange != nil {
		cb.onStateChange(cb.name, prev, state)
	}

	log.Infof("Circuit breaker '%s' state changed from %s to %s", cb.name, prev, state)
}

func (cb *CircuitBreaker) toNewGeneration(now time.Time) {
	cb.generation++
	cb.counts.clear()

	var zero time.Time
	switch cb.state {
	case StateClosed:
		if cb.interval == 0 {
			cb.expiry = zero
		} else {
			cb.expiry = now.Add(cb.interval)
		}
	case StateOpen:
		cb.expiry = now.Add(cb.timeout)
	default: // StateHalfOpen
		cb.expiry = zero
	}
}

func (c *Counts) onRequest() {
	c.Requests++
}

func (c *Counts) onSuccess() {
	c.TotalSuccesses++
	c.ConsecutiveSuccesses++
	c.ConsecutiveFailures = 0
}

func (c *Counts) onFailure() {
	c.TotalFailures++
	c.ConsecutiveFailures++
	c.ConsecutiveSuccesses = 0
}

func (c *Counts) clear() {
	c.Requests = 0
	c.TotalSuccesses = 0
	c.TotalFailures = 0
	c.ConsecutiveSuccesses = 0
	c.ConsecutiveFailures = 0
}

// Predefined errors
var (
	ErrOpenState        = errors.New("circuit breaker is open")
	ErrTooManyRequests  = errors.New("too many requests")
	ErrInvalidThreshold = errors.New("invalid threshold")
)

// BitswapCircuitBreaker manages circuit breakers for Bitswap operations per peer
type BitswapCircuitBreaker struct {
	breakers map[peer.ID]*CircuitBreaker
	mutex    sync.RWMutex
	config   Config
}

// NewBitswapCircuitBreaker creates a new circuit breaker manager for Bitswap operations
func NewBitswapCircuitBreaker(config Config) *BitswapCircuitBreaker {
	return &BitswapCircuitBreaker{
		breakers: make(map[peer.ID]*CircuitBreaker),
		config:   config,
	}
}

// GetCircuitBreaker returns the circuit breaker for a specific peer, creating one if it doesn't exist
func (bcb *BitswapCircuitBreaker) GetCircuitBreaker(peerID peer.ID) *CircuitBreaker {
	bcb.mutex.RLock()
	cb, exists := bcb.breakers[peerID]
	bcb.mutex.RUnlock()

	if exists {
		return cb
	}

	bcb.mutex.Lock()
	defer bcb.mutex.Unlock()

	// Double-check after acquiring write lock
	if cb, exists := bcb.breakers[peerID]; exists {
		return cb
	}

	// Create new circuit breaker for this peer
	name := fmt.Sprintf("bitswap-peer-%s", peerID.String()[:8])
	cb = NewCircuitBreaker(name, bcb.config)
	bcb.breakers[peerID] = cb

	log.Debugf("Created circuit breaker for peer %s", peerID)
	return cb
}

// ExecuteWithPeer executes a request for a specific peer through its circuit breaker
func (bcb *BitswapCircuitBreaker) ExecuteWithPeer(peerID peer.ID, req func() error) error {
	cb := bcb.GetCircuitBreaker(peerID)
	return cb.Execute(req)
}

// CallWithPeer executes a request for a specific peer through its circuit breaker with context
func (bcb *BitswapCircuitBreaker) CallWithPeer(ctx context.Context, peerID peer.ID, req func(context.Context) error) error {
	cb := bcb.GetCircuitBreaker(peerID)
	return cb.Call(ctx, req)
}

// GetPeerState returns the current state of the circuit breaker for a specific peer
func (bcb *BitswapCircuitBreaker) GetPeerState(peerID peer.ID) State {
	bcb.mutex.RLock()
	cb, exists := bcb.breakers[peerID]
	bcb.mutex.RUnlock()

	if !exists {
		return StateClosed // Default state for non-existent breakers
	}

	return cb.State()
}

// GetPeerCounts returns the current counts for a specific peer's circuit breaker
func (bcb *BitswapCircuitBreaker) GetPeerCounts(peerID peer.ID) Counts {
	bcb.mutex.RLock()
	cb, exists := bcb.breakers[peerID]
	bcb.mutex.RUnlock()

	if !exists {
		return Counts{} // Empty counts for non-existent breakers
	}

	return cb.Counts()
}

// GetAllStates returns the states of all circuit breakers
func (bcb *BitswapCircuitBreaker) GetAllStates() map[peer.ID]State {
	bcb.mutex.RLock()
	defer bcb.mutex.RUnlock()

	states := make(map[peer.ID]State, len(bcb.breakers))
	for peerID, cb := range bcb.breakers {
		states[peerID] = cb.State()
	}

	return states
}

// CleanupInactiveBreakers removes circuit breakers for peers that haven't been used recently
func (bcb *BitswapCircuitBreaker) CleanupInactiveBreakers(maxAge time.Duration) {
	bcb.mutex.Lock()
	defer bcb.mutex.Unlock()

	now := time.Now()
	for peerID, cb := range bcb.breakers {
		counts := cb.Counts()
		// Remove breakers that have no recent activity
		if counts.Requests == 0 && now.Sub(time.Time{}) > maxAge {
			delete(bcb.breakers, peerID)
			log.Debugf("Cleaned up inactive circuit breaker for peer %s", peerID)
		}
	}
}

// DefaultBitswapCircuitBreakerConfig returns a default configuration for Bitswap circuit breakers
func DefaultBitswapCircuitBreakerConfig() Config {
	return Config{
		MaxRequests: 10,
		Interval:    30 * time.Second,
		Timeout:     60 * time.Second,
		ReadyToTrip: func(counts Counts) bool {
			failureRatio := float64(counts.TotalFailures) / float64(counts.Requests)
			return counts.Requests >= 3 && failureRatio >= 0.6
		},
		OnStateChange: func(name string, from State, to State) {
			log.Infof("Circuit breaker %s changed from %s to %s", name, from, to)
		},
		IsSuccessful: func(err error) bool {
			// Consider network timeouts and context cancellations as temporary failures
			if errors.Is(err, context.DeadlineExceeded) || errors.Is(err, context.Canceled) {
				return false
			}
			return err == nil
		},
	}
}