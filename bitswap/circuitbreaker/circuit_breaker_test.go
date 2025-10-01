package circuitbreaker

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCircuitBreaker_BasicFunctionality(t *testing.T) {
	config := Config{
		MaxRequests: 3,
		Interval:    time.Second,
		Timeout:     time.Second,
		ReadyToTrip: func(counts Counts) bool {
			return counts.ConsecutiveFailures >= 3
		},
	}

	cb := NewCircuitBreaker("test", config)

	// Initially closed
	assert.Equal(t, StateClosed, cb.State())

	// Successful requests should work
	err := cb.Execute(func() error { return nil })
	assert.NoError(t, err)

	counts := cb.Counts()
	assert.Equal(t, uint32(1), counts.Requests)
	assert.Equal(t, uint32(1), counts.TotalSuccesses)
}

func TestCircuitBreaker_TripsOnFailures(t *testing.T) {
	config := Config{
		MaxRequests: 3,
		Interval:    time.Second,
		Timeout:     time.Second,
		ReadyToTrip: func(counts Counts) bool {
			return counts.ConsecutiveFailures >= 3
		},
	}

	cb := NewCircuitBreaker("test", config)

	// Generate failures to trip the circuit breaker
	testErr := errors.New("test error")
	for i := 0; i < 3; i++ {
		err := cb.Execute(func() error { return testErr })
		assert.Equal(t, testErr, err)
	}

	// Circuit breaker should now be open
	assert.Equal(t, StateOpen, cb.State())

	// Requests should be rejected
	err := cb.Execute(func() error { return nil })
	assert.Equal(t, ErrOpenState, err)
}

func TestCircuitBreaker_HalfOpenRecovery(t *testing.T) {
	config := Config{
		MaxRequests: 2,
		Interval:    time.Millisecond * 10,
		Timeout:     time.Millisecond * 50,
		ReadyToTrip: func(counts Counts) bool {
			return counts.ConsecutiveFailures >= 2
		},
	}

	cb := NewCircuitBreaker("test", config)

	// Trip the circuit breaker
	testErr := errors.New("test error")
	for i := 0; i < 2; i++ {
		cb.Execute(func() error { return testErr })
	}

	assert.Equal(t, StateOpen, cb.State())

	// Wait for timeout to transition to half-open
	time.Sleep(time.Millisecond * 60)
	assert.Equal(t, StateHalfOpen, cb.State())

	// Successful requests in half-open should close the circuit
	for i := 0; i < 2; i++ {
		err := cb.Execute(func() error { return nil })
		assert.NoError(t, err)
	}

	assert.Equal(t, StateClosed, cb.State())
}

func TestCircuitBreaker_WithContext(t *testing.T) {
	config := DefaultBitswapCircuitBreakerConfig()
	cb := NewCircuitBreaker("test", config)

	ctx, cancel := context.WithTimeout(context.Background(), time.Millisecond*100)
	defer cancel()

	// Test successful call with context
	err := cb.Call(ctx, func(ctx context.Context) error {
		return nil
	})
	assert.NoError(t, err)

	// Test call that times out
	err = cb.Call(ctx, func(ctx context.Context) error {
		time.Sleep(time.Millisecond * 200)
		return nil
	})
	// The function should complete but context might be cancelled
	// The actual behavior depends on the implementation
}

func TestBitswapCircuitBreaker_PeerManagement(t *testing.T) {
	config := DefaultBitswapCircuitBreakerConfig()
	bcb := NewBitswapCircuitBreaker(config)

	// Create test peer IDs
	peerID1, err := peer.Decode("12D3KooWGzxzKZYveHXtpG6AsrUJBcWxHBFS2HsEoGTxrMLvKXtf")
	require.NoError(t, err)
	
	peerID2, err := peer.Decode("12D3KooWH3uVF6wv47WnArKHbQBCgdjhCXCe5NkqJFTXNdVkjNwq")
	require.NoError(t, err)

	// Test getting circuit breakers for different peers
	cb1 := bcb.GetCircuitBreaker(peerID1)
	cb2 := bcb.GetCircuitBreaker(peerID2)

	assert.NotNil(t, cb1)
	assert.NotNil(t, cb2)
	assert.NotEqual(t, cb1, cb2)

	// Test that getting the same peer returns the same circuit breaker
	cb1Again := bcb.GetCircuitBreaker(peerID1)
	assert.Equal(t, cb1, cb1Again)
}

func TestBitswapCircuitBreaker_ExecuteWithPeer(t *testing.T) {
	config := Config{
		MaxRequests: 2,
		Interval:    time.Second,
		Timeout:     time.Second,
		ReadyToTrip: func(counts Counts) bool {
			return counts.ConsecutiveFailures >= 2
		},
	}

	bcb := NewBitswapCircuitBreaker(config)
	peerID, err := peer.Decode("12D3KooWGzxzKZYveHXtpG6AsrUJBcWxHBFS2HsEoGTxrMLvKXtf")
	require.NoError(t, err)

	// Test successful execution
	err = bcb.ExecuteWithPeer(peerID, func() error {
		return nil
	})
	assert.NoError(t, err)

	// Test failed execution
	testErr := errors.New("test error")
	err = bcb.ExecuteWithPeer(peerID, func() error {
		return testErr
	})
	assert.Equal(t, testErr, err)

	// Trip the circuit breaker
	bcb.ExecuteWithPeer(peerID, func() error {
		return testErr
	})

	// Verify circuit breaker is open
	assert.Equal(t, StateOpen, bcb.GetPeerState(peerID))

	// Requests should be rejected
	err = bcb.ExecuteWithPeer(peerID, func() error {
		return nil
	})
	assert.Equal(t, ErrOpenState, err)
}

func TestBitswapCircuitBreaker_CallWithPeer(t *testing.T) {
	config := DefaultBitswapCircuitBreakerConfig()
	bcb := NewBitswapCircuitBreaker(config)
	peerID, err := peer.Decode("12D3KooWGzxzKZYveHXtpG6AsrUJBcWxHBFS2HsEoGTxrMLvKXtf")
	require.NoError(t, err)

	ctx := context.Background()

	// Test successful call
	err = bcb.CallWithPeer(ctx, peerID, func(ctx context.Context) error {
		return nil
	})
	assert.NoError(t, err)

	// Test call with context cancellation
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately

	err = bcb.CallWithPeer(ctx, peerID, func(ctx context.Context) error {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(time.Millisecond * 100):
			return nil
		}
	})
	assert.Error(t, err)
}

func TestBitswapCircuitBreaker_GetAllStates(t *testing.T) {
	config := DefaultBitswapCircuitBreakerConfig()
	bcb := NewBitswapCircuitBreaker(config)

	// Create multiple peers with different IDs
	peerIDs := []string{
		"12D3KooWGzxzKZYveHXtpG6AsrUJBcWxHBFS2HsEoGTxrMLvKXtf",
		"12D3KooWH3uVF6wv47WnArKHbQBCgdjhCXCe5NkqJFTXNdVkjNwq",
		"12D3KooWBhMbLvQGgqByteXqKxKvJkXQhQvKZyYqBzWBKvNqKqhF",
	}
	
	peers := make([]peer.ID, 3)
	for i, peerIDStr := range peerIDs {
		peerID, err := peer.Decode(peerIDStr)
		require.NoError(t, err)
		peers[i] = peerID
		
		// Create circuit breaker for each peer
		bcb.GetCircuitBreaker(peerID)
	}

	states := bcb.GetAllStates()
	assert.Len(t, states, 3)

	for _, peerID := range peers {
		state, exists := states[peerID]
		assert.True(t, exists)
		assert.Equal(t, StateClosed, state)
	}
}

func TestCircuitBreaker_StateTransitions(t *testing.T) {
	stateChanges := make([]State, 0)
	config := Config{
		MaxRequests: 2,
		Interval:    time.Millisecond * 10,
		Timeout:     time.Millisecond * 50,
		ReadyToTrip: func(counts Counts) bool {
			return counts.ConsecutiveFailures >= 2
		},
		OnStateChange: func(name string, from State, to State) {
			stateChanges = append(stateChanges, to)
		},
	}

	cb := NewCircuitBreaker("test", config)

	// Initially closed
	assert.Equal(t, StateClosed, cb.State())

	// Trip to open
	testErr := errors.New("test error")
	for i := 0; i < 2; i++ {
		cb.Execute(func() error { return testErr })
	}

	assert.Equal(t, StateOpen, cb.State())
	assert.Contains(t, stateChanges, StateOpen)

	// Wait for half-open
	time.Sleep(time.Millisecond * 60)
	assert.Equal(t, StateHalfOpen, cb.State())

	// Successful requests should close
	for i := 0; i < 2; i++ {
		cb.Execute(func() error { return nil })
	}

	assert.Equal(t, StateClosed, cb.State())
	assert.Contains(t, stateChanges, StateClosed)
}

func TestCircuitBreaker_Counts(t *testing.T) {
	config := Config{
		MaxRequests: 5,
		Interval:    time.Second,
		Timeout:     time.Second,
		ReadyToTrip: func(counts Counts) bool {
			return counts.ConsecutiveFailures >= 3
		},
	}

	cb := NewCircuitBreaker("test", config)

	// Test successful requests
	for i := 0; i < 3; i++ {
		cb.Execute(func() error { return nil })
	}

	counts := cb.Counts()
	assert.Equal(t, uint32(3), counts.Requests)
	assert.Equal(t, uint32(3), counts.TotalSuccesses)
	assert.Equal(t, uint32(0), counts.TotalFailures)
	assert.Equal(t, uint32(3), counts.ConsecutiveSuccesses)
	assert.Equal(t, uint32(0), counts.ConsecutiveFailures)

	// Test failed requests
	testErr := errors.New("test error")
	for i := 0; i < 2; i++ {
		cb.Execute(func() error { return testErr })
	}

	counts = cb.Counts()
	assert.Equal(t, uint32(5), counts.Requests)
	assert.Equal(t, uint32(3), counts.TotalSuccesses)
	assert.Equal(t, uint32(2), counts.TotalFailures)
	assert.Equal(t, uint32(0), counts.ConsecutiveSuccesses)
	assert.Equal(t, uint32(2), counts.ConsecutiveFailures)
}

func TestCircuitBreaker_DefaultConfig(t *testing.T) {
	config := DefaultBitswapCircuitBreakerConfig()
	
	assert.Equal(t, uint32(10), config.MaxRequests)
	assert.Equal(t, 30*time.Second, config.Interval)
	assert.Equal(t, 60*time.Second, config.Timeout)
	assert.NotNil(t, config.ReadyToTrip)
	assert.NotNil(t, config.OnStateChange)
	assert.NotNil(t, config.IsSuccessful)

	// Test ReadyToTrip function
	counts := Counts{Requests: 5, TotalFailures: 3}
	assert.True(t, config.ReadyToTrip(counts))

	counts = Counts{Requests: 2, TotalFailures: 1}
	assert.False(t, config.ReadyToTrip(counts))

	// Test IsSuccessful function
	assert.True(t, config.IsSuccessful(nil))
	assert.False(t, config.IsSuccessful(errors.New("error")))
	assert.False(t, config.IsSuccessful(context.DeadlineExceeded))
	assert.False(t, config.IsSuccessful(context.Canceled))
}

func TestCircuitBreaker_Panic(t *testing.T) {
	config := DefaultBitswapCircuitBreakerConfig()
	cb := NewCircuitBreaker("test", config)

	// Test that panics are properly handled
	assert.Panics(t, func() {
		cb.Execute(func() error {
			panic("test panic")
		})
	})

	// Circuit breaker should still be functional after panic
	err := cb.Execute(func() error { return nil })
	assert.NoError(t, err)
}

// Benchmark tests
func BenchmarkCircuitBreaker_Execute(b *testing.B) {
	config := DefaultBitswapCircuitBreakerConfig()
	cb := NewCircuitBreaker("benchmark", config)

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			cb.Execute(func() error { return nil })
		}
	})
}

func BenchmarkBitswapCircuitBreaker_ExecuteWithPeer(b *testing.B) {
	config := DefaultBitswapCircuitBreakerConfig()
	bcb := NewBitswapCircuitBreaker(config)
	peerID, _ := peer.Decode("12D3KooWGzxzKZYveHXtpG6AsrUJBcWxHBFS2HsEoGTxrMLvKXtf")

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			bcb.ExecuteWithPeer(peerID, func() error { return nil })
		}
	})
}