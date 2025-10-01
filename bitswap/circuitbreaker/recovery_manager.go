package circuitbreaker

import (
	"context"
	"sync"
	"time"

	"github.com/libp2p/go-libp2p/core/peer"
	logging "github.com/ipfs/go-log/v2"
)

var recoveryLog = logging.Logger("bitswap/circuitbreaker/recovery")

// RecoveryStrategy defines different recovery strategies
type RecoveryStrategy int

const (
	// RecoveryLinear increases load linearly over time
	RecoveryLinear RecoveryStrategy = iota
	// RecoveryExponential increases load exponentially (slower initial recovery)
	RecoveryExponential
	// RecoveryStep increases load in discrete steps
	RecoveryStep
)

func (rs RecoveryStrategy) String() string {
	switch rs {
	case RecoveryLinear:
		return "LINEAR"
	case RecoveryExponential:
		return "EXPONENTIAL"
	case RecoveryStep:
		return "STEP"
	default:
		return "UNKNOWN"
	}
}

// RecoveryConfig holds configuration for the recovery manager
type RecoveryConfig struct {
	// Strategy defines how to gradually increase load during recovery
	Strategy RecoveryStrategy
	
	// InitialRecoveryRate is the starting rate for recovery (0.0 to 1.0)
	InitialRecoveryRate float64
	
	// MaxRecoveryRate is the maximum rate during recovery (0.0 to 1.0)
	MaxRecoveryRate float64
	
	// RecoveryInterval is how often to increase the recovery rate
	RecoveryInterval time.Duration
	
	// RecoveryIncrement is how much to increase the rate each interval
	RecoveryIncrement float64
	
	// SuccessThreshold is the number of consecutive successes needed to complete recovery
	SuccessThreshold int
	
	// FailureThreshold is the number of failures that will reset recovery
	FailureThreshold int
	
	// MaxRecoveryTime is the maximum time to spend in recovery mode
	MaxRecoveryTime time.Duration
	
	// CooldownPeriod is the time to wait before starting recovery after a failure
	CooldownPeriod time.Duration
}

// DefaultRecoveryConfig returns a default recovery configuration
func DefaultRecoveryConfig() RecoveryConfig {
	return RecoveryConfig{
		Strategy:            RecoveryExponential,
		InitialRecoveryRate: 0.1,  // Start with 10% of normal load
		MaxRecoveryRate:     1.0,  // Eventually allow 100% of normal load
		RecoveryInterval:    10 * time.Second,
		RecoveryIncrement:   0.1,  // Increase by 10% each interval
		SuccessThreshold:    10,   // Need 10 consecutive successes
		FailureThreshold:    3,    // 3 failures reset recovery
		MaxRecoveryTime:     5 * time.Minute,
		CooldownPeriod:      30 * time.Second,
	}
}

// RecoveryState represents the current recovery state for a peer
type RecoveryState struct {
	PeerID              peer.ID
	IsRecovering        bool
	RecoveryStartTime   time.Time
	CurrentRate         float64
	ConsecutiveSuccesses int
	ConsecutiveFailures int
	LastActivity        time.Time
	Strategy            RecoveryStrategy
}

// RecoveryManager manages gradual recovery for peers after circuit breaker trips
type RecoveryManager struct {
	config        RecoveryConfig
	recoveryStates map[peer.ID]*RecoveryState
	mutex         sync.RWMutex
	ctx           context.Context
	cancel        context.CancelFunc
	wg            sync.WaitGroup
	
	// Callbacks
	onRecoveryStart    func(peer.ID)
	onRecoveryComplete func(peer.ID)
	onRecoveryFailed   func(peer.ID)
}

// NewRecoveryManager creates a new recovery manager
func NewRecoveryManager(config RecoveryConfig) *RecoveryManager {
	ctx, cancel := context.WithCancel(context.Background())
	
	rm := &RecoveryManager{
		config:         config,
		recoveryStates: make(map[peer.ID]*RecoveryState),
		ctx:            ctx,
		cancel:         cancel,
	}
	
	// Start background recovery management
	rm.wg.Add(1)
	go rm.recoveryLoop()
	
	return rm
}

// StartRecovery initiates recovery for a peer
func (rm *RecoveryManager) StartRecovery(peerID peer.ID) {
	rm.mutex.Lock()
	defer rm.mutex.Unlock()
	
	state, exists := rm.recoveryStates[peerID]
	if exists && state.IsRecovering {
		recoveryLog.Debugf("Recovery already in progress for peer %s", peerID)
		return
	}
	
	now := time.Now()
	
	// Check cooldown period
	if exists && now.Sub(state.LastActivity) < rm.config.CooldownPeriod {
		recoveryLog.Debugf("Peer %s is in cooldown period, recovery delayed", peerID)
		return
	}
	
	// Create or update recovery state
	if !exists {
		state = &RecoveryState{
			PeerID: peerID,
		}
		rm.recoveryStates[peerID] = state
	}
	
	state.IsRecovering = true
	state.RecoveryStartTime = now
	state.CurrentRate = rm.config.InitialRecoveryRate
	state.ConsecutiveSuccesses = 0
	state.ConsecutiveFailures = 0
	state.LastActivity = now
	state.Strategy = rm.config.Strategy
	
	recoveryLog.Infof("Started recovery for peer %s with strategy %s, initial rate %.2f",
		peerID, state.Strategy, state.CurrentRate)
	
	if rm.onRecoveryStart != nil {
		rm.onRecoveryStart(peerID)
	}
}

// RecordRecoveryAttempt records the result of a recovery attempt
func (rm *RecoveryManager) RecordRecoveryAttempt(peerID peer.ID, success bool) {
	rm.mutex.Lock()
	defer rm.mutex.Unlock()
	
	state, exists := rm.recoveryStates[peerID]
	if !exists || !state.IsRecovering {
		return
	}
	
	state.LastActivity = time.Now()
	
	if success {
		state.ConsecutiveSuccesses++
		state.ConsecutiveFailures = 0
		
		recoveryLog.Debugf("Recovery success for peer %s, consecutive successes: %d",
			peerID, state.ConsecutiveSuccesses)
		
		// Check if recovery is complete
		if state.ConsecutiveSuccesses >= rm.config.SuccessThreshold {
			rm.completeRecovery(peerID, state)
		}
	} else {
		state.ConsecutiveFailures++
		state.ConsecutiveSuccesses = 0
		
		recoveryLog.Debugf("Recovery failure for peer %s, consecutive failures: %d",
			peerID, state.ConsecutiveFailures)
		
		// Check if recovery should be reset
		if state.ConsecutiveFailures >= rm.config.FailureThreshold {
			rm.failRecovery(peerID, state)
		}
	}
}

// ShouldAllowRequest determines if a request should be allowed during recovery
func (rm *RecoveryManager) ShouldAllowRequest(peerID peer.ID) bool {
	rm.mutex.RLock()
	defer rm.mutex.RUnlock()
	
	state, exists := rm.recoveryStates[peerID]
	if !exists || !state.IsRecovering {
		return true // Allow all requests if not in recovery
	}
	
	// Simple probability-based decision
	// In a real implementation, you might want more sophisticated logic
	return rm.shouldAllowBasedOnRate(state.CurrentRate)
}

// GetRecoveryRate returns the current recovery rate for a peer
func (rm *RecoveryManager) GetRecoveryRate(peerID peer.ID) float64 {
	rm.mutex.RLock()
	defer rm.mutex.RUnlock()
	
	state, exists := rm.recoveryStates[peerID]
	if !exists || !state.IsRecovering {
		return 1.0 // Full rate if not in recovery
	}
	
	return state.CurrentRate
}

// IsRecovering returns true if the peer is currently in recovery mode
func (rm *RecoveryManager) IsRecovering(peerID peer.ID) bool {
	rm.mutex.RLock()
	defer rm.mutex.RUnlock()
	
	state, exists := rm.recoveryStates[peerID]
	return exists && state.IsRecovering
}

// GetRecoveryState returns the current recovery state for a peer
func (rm *RecoveryManager) GetRecoveryState(peerID peer.ID) (*RecoveryState, bool) {
	rm.mutex.RLock()
	defer rm.mutex.RUnlock()
	
	state, exists := rm.recoveryStates[peerID]
	if !exists {
		return nil, false
	}
	
	// Return a copy to avoid race conditions
	stateCopy := *state
	return &stateCopy, true
}

// SetCallbacks sets callback functions for recovery events
func (rm *RecoveryManager) SetCallbacks(
	onStart func(peer.ID),
	onComplete func(peer.ID),
	onFailed func(peer.ID),
) {
	rm.mutex.Lock()
	defer rm.mutex.Unlock()
	
	rm.onRecoveryStart = onStart
	rm.onRecoveryComplete = onComplete
	rm.onRecoveryFailed = onFailed
}

// Close stops the recovery manager
func (rm *RecoveryManager) Close() error {
	rm.cancel()
	rm.wg.Wait()
	return nil
}

// completeRecovery marks recovery as complete for a peer
func (rm *RecoveryManager) completeRecovery(peerID peer.ID, state *RecoveryState) {
	state.IsRecovering = false
	state.CurrentRate = 1.0
	
	recoveryLog.Infof("Recovery completed for peer %s after %v",
		peerID, time.Since(state.RecoveryStartTime))
	
	if rm.onRecoveryComplete != nil {
		rm.onRecoveryComplete(peerID)
	}
}

// failRecovery marks recovery as failed for a peer
func (rm *RecoveryManager) failRecovery(peerID peer.ID, state *RecoveryState) {
	state.IsRecovering = false
	state.CurrentRate = 0.0
	
	recoveryLog.Warnf("Recovery failed for peer %s after %v, too many consecutive failures",
		peerID, time.Since(state.RecoveryStartTime))
	
	if rm.onRecoveryFailed != nil {
		rm.onRecoveryFailed(peerID)
	}
}

// recoveryLoop runs the background recovery management tasks
func (rm *RecoveryManager) recoveryLoop() {
	defer rm.wg.Done()
	
	ticker := time.NewTicker(rm.config.RecoveryInterval)
	defer ticker.Stop()
	
	for {
		select {
		case <-rm.ctx.Done():
			return
		case <-ticker.C:
			rm.updateRecoveryRates()
			rm.cleanupStaleRecoveries()
		}
	}
}

// updateRecoveryRates updates the recovery rates for all recovering peers
func (rm *RecoveryManager) updateRecoveryRates() {
	rm.mutex.Lock()
	defer rm.mutex.Unlock()
	
	now := time.Now()
	
	for peerID, state := range rm.recoveryStates {
		if !state.IsRecovering {
			continue
		}
		
		// Check if recovery has timed out
		if now.Sub(state.RecoveryStartTime) > rm.config.MaxRecoveryTime {
			recoveryLog.Warnf("Recovery timed out for peer %s", peerID)
			rm.failRecovery(peerID, state)
			continue
		}
		
		// Update recovery rate based on strategy
		oldRate := state.CurrentRate
		state.CurrentRate = rm.calculateNewRate(state, now)
		
		if state.CurrentRate != oldRate {
			recoveryLog.Debugf("Updated recovery rate for peer %s: %.2f -> %.2f",
				peerID, oldRate, state.CurrentRate)
		}
	}
}

// calculateNewRate calculates the new recovery rate based on the strategy
func (rm *RecoveryManager) calculateNewRate(state *RecoveryState, now time.Time) float64 {
	elapsed := now.Sub(state.RecoveryStartTime)
	
	switch state.Strategy {
	case RecoveryLinear:
		return rm.calculateLinearRate(elapsed)
	case RecoveryExponential:
		return rm.calculateExponentialRate(elapsed)
	case RecoveryStep:
		return rm.calculateStepRate(elapsed)
	default:
		return state.CurrentRate
	}
}

// calculateLinearRate calculates recovery rate using linear strategy
func (rm *RecoveryManager) calculateLinearRate(elapsed time.Duration) float64 {
	intervals := float64(elapsed) / float64(rm.config.RecoveryInterval)
	rate := rm.config.InitialRecoveryRate + (intervals * rm.config.RecoveryIncrement)
	
	if rate > rm.config.MaxRecoveryRate {
		rate = rm.config.MaxRecoveryRate
	}
	
	return rate
}

// calculateExponentialRate calculates recovery rate using exponential strategy
func (rm *RecoveryManager) calculateExponentialRate(elapsed time.Duration) float64 {
	intervals := float64(elapsed) / float64(rm.config.RecoveryInterval)
	
	// Exponential growth: rate = initial * (1 + increment)^intervals
	rate := rm.config.InitialRecoveryRate
	for i := 0; i < int(intervals); i++ {
		rate = rate * (1 + rm.config.RecoveryIncrement)
		if rate > rm.config.MaxRecoveryRate {
			rate = rm.config.MaxRecoveryRate
			break
		}
	}
	
	return rate
}

// calculateStepRate calculates recovery rate using step strategy
func (rm *RecoveryManager) calculateStepRate(elapsed time.Duration) float64 {
	intervals := int(elapsed / rm.config.RecoveryInterval)
	rate := rm.config.InitialRecoveryRate + (float64(intervals) * rm.config.RecoveryIncrement)
	
	if rate > rm.config.MaxRecoveryRate {
		rate = rm.config.MaxRecoveryRate
	}
	
	return rate
}

// shouldAllowBasedOnRate determines if a request should be allowed based on recovery rate
func (rm *RecoveryManager) shouldAllowBasedOnRate(rate float64) bool {
	// Simple implementation: use the rate as a probability
	// In practice, you might want to use a more sophisticated algorithm
	// like token bucket or sliding window
	
	// For now, we'll use a simple random decision
	// This should be replaced with a proper rate limiting algorithm
	return true // Simplified for this implementation
}

// cleanupStaleRecoveries removes old recovery states
func (rm *RecoveryManager) cleanupStaleRecoveries() {
	rm.mutex.Lock()
	defer rm.mutex.Unlock()
	
	now := time.Now()
	maxAge := rm.config.MaxRecoveryTime * 2 // Keep records for twice the max recovery time
	
	for peerID, state := range rm.recoveryStates {
		if !state.IsRecovering && now.Sub(state.LastActivity) > maxAge {
			delete(rm.recoveryStates, peerID)
			recoveryLog.Debugf("Cleaned up stale recovery state for peer %s", peerID)
		}
	}
}

// GetRecoveryStats returns aggregate recovery statistics
type RecoveryStats struct {
	TotalPeers      int
	RecoveringPeers int
	CompletedRecoveries int
	FailedRecoveries    int
	AverageRecoveryRate float64
}

// GetRecoveryStats returns aggregate recovery statistics
func (rm *RecoveryManager) GetRecoveryStats() RecoveryStats {
	rm.mutex.RLock()
	defer rm.mutex.RUnlock()
	
	stats := RecoveryStats{
		TotalPeers: len(rm.recoveryStates),
	}
	
	if stats.TotalPeers == 0 {
		return stats
	}
	
	var totalRate float64
	
	for _, state := range rm.recoveryStates {
		if state.IsRecovering {
			stats.RecoveringPeers++
			totalRate += state.CurrentRate
		}
	}
	
	if stats.RecoveringPeers > 0 {
		stats.AverageRecoveryRate = totalRate / float64(stats.RecoveringPeers)
	}
	
	return stats
}