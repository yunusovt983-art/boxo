package circuitbreaker

import (
	"context"
	"errors"
	"fmt"
	"math/rand"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestCircuitBreakerUnderNetworkFaults tests circuit breaker behavior under various network fault conditions
func TestCircuitBreakerUnderNetworkFaults(t *testing.T) {
	t.Run("IntermittentNetworkFailures", func(t *testing.T) {
		config := Config{
			MaxRequests: 5,
			Interval:    1 * time.Second,
			Timeout:     2 * time.Second,
			ReadyToTrip: func(counts Counts) bool {
				return counts.ConsecutiveFailures >= 3
			},
		}

		cb := NewCircuitBreaker("intermittent-test", config)
		networkError := errors.New("network timeout")

		// Simulate intermittent failures
		for i := 0; i < 10; i++ {
			var err error
			if i%3 == 0 { // Every third request fails
				err = cb.Execute(func() error { return networkError })
				assert.Equal(t, networkError, err)
			} else {
				err = cb.Execute(func() error { return nil })
				assert.NoError(t, err)
			}
		}

		// Circuit should still be closed due to intermittent nature
		assert.Equal(t, StateClosed, cb.State())
	})

	t.Run("BurstFailuresFollowedByRecovery", func(t *testing.T) {
		config := Config{
			MaxRequests: 3,
			Interval:    500 * time.Millisecond,
			Timeout:     1 * time.Second,
			ReadyToTrip: func(counts Counts) bool {
				return counts.ConsecutiveFailures >= 3
			},
		}

		cb := NewCircuitBreaker("burst-test", config)
		networkError := errors.New("burst network error")

		// Burst of failures
		for i := 0; i < 3; i++ {
			err := cb.Execute(func() error { return networkError })
			assert.Equal(t, networkError, err)
		}

		// Circuit should be open
		assert.Equal(t, StateOpen, cb.State())

		// Wait for half-open
		time.Sleep(1200 * time.Millisecond)
		assert.Equal(t, StateHalfOpen, cb.State())

		// Recovery with successful requests
		for i := 0; i < 3; i++ {
			err := cb.Execute(func() error { return nil })
			assert.NoError(t, err)
		}

		// Circuit should be closed
		assert.Equal(t, StateClosed, cb.State())
	})

	t.Run("HighLatencyRequests", func(t *testing.T) {
		config := Config{
			MaxRequests: 2,
			Interval:    1 * time.Second,
			Timeout:     2 * time.Second,
			ReadyToTrip: func(counts Counts) bool {
				return counts.ConsecutiveFailures >= 2
			},
			IsSuccessful: func(err error) bool {
				// Consider timeouts as failures
				return err == nil
			},
		}

		cb := NewCircuitBreaker("latency-test", config)

		// Simulate high latency requests that timeout
		timeoutError := context.DeadlineExceeded
		for i := 0; i < 2; i++ {
			err := cb.Execute(func() error { return timeoutError })
			assert.Equal(t, timeoutError, err)
		}

		// Circuit should trip on timeouts
		assert.Equal(t, StateOpen, cb.State())
	})
}

// TestCircuitBreakerConcurrentFaults tests circuit breaker under concurrent fault conditions
func TestCircuitBreakerConcurrentFaults(t *testing.T) {
	t.Run("ConcurrentFailuresAndRecoveries", func(t *testing.T) {
		config := Config{
			MaxRequests: 5,
			Interval:    1 * time.Second,
			Timeout:     2 * time.Second,
			ReadyToTrip: func(counts Counts) bool {
				return counts.ConsecutiveFailures >= 5
			},
		}

		cb := NewCircuitBreaker("concurrent-test", config)
		var wg sync.WaitGroup
		var successCount, failureCount int32

		// Concurrent requests with mixed success/failure
		for i := 0; i < 20; i++ {
			wg.Add(1)
			go func(requestID int) {
				defer wg.Done()

				var err error
				if requestID%4 == 0 { // 25% failure rate
					err = cb.Execute(func() error {
						return errors.New("concurrent failure")
					})
					if err != nil {
						atomic.AddInt32(&failureCount, 1)
					}
				} else {
					err = cb.Execute(func() error { return nil })
					if err == nil {
						atomic.AddInt32(&successCount, 1)
					}
				}
			}(i)
		}

		wg.Wait()

		t.Logf("Concurrent test results: %d successes, %d failures", successCount, failureCount)

		// Should handle concurrent requests properly
		assert.Greater(t, int(successCount), 10, "Should have successful requests")
		assert.Greater(t, int(failureCount), 0, "Should have some failures")
	})

	t.Run("RaceConditionOnStateTransition", func(t *testing.T) {
		config := Config{
			MaxRequests: 1,
			Interval:    100 * time.Millisecond,
			Timeout:     200 * time.Millisecond,
			ReadyToTrip: func(counts Counts) bool {
				return counts.ConsecutiveFailures >= 2
			},
		}

		cb := NewCircuitBreaker("race-test", config)

		// Trip the circuit breaker
		for i := 0; i < 2; i++ {
			cb.Execute(func() error { return errors.New("trip error") })
		}
		assert.Equal(t, StateOpen, cb.State())

		// Wait for half-open transition
		time.Sleep(250 * time.Millisecond)

		var wg sync.WaitGroup
		var results []error
		var mu sync.Mutex

		// Concurrent requests during state transition
		for i := 0; i < 10; i++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				err := cb.Execute(func() error { return nil })

				mu.Lock()
				results = append(results, err)
				mu.Unlock()
			}()
		}

		wg.Wait()

		// Should handle race conditions gracefully
		var successCount, rejectedCount int
		for _, err := range results {
			if err == nil {
				successCount++
			} else if err == ErrTooManyRequests || err == ErrOpenState {
				rejectedCount++
			}
		}

		t.Logf("Race condition test: %d successes, %d rejected", successCount, rejectedCount)
		assert.Greater(t, successCount, 0, "Should have some successful requests")
	})
}

// TestBitswapCircuitBreakerFaultTolerance tests the Bitswap-specific circuit breaker under faults
func TestBitswapCircuitBreakerFaultTolerance(t *testing.T) {
	t.Run("MultiPeerFailureIsolation", func(t *testing.T) {
		config := Config{
			MaxRequests: 3,
			Interval:    1 * time.Second,
			Timeout:     2 * time.Second,
			ReadyToTrip: func(counts Counts) bool {
				return counts.ConsecutiveFailures >= 3
			},
		}

		bcb := NewBitswapCircuitBreaker(config)

		// Create multiple test peers
		peerIDs := []string{
			"12D3KooWGzxzKZYveHXtpG6AsrUJBcWxHBFS2HsEoGTxrMLvKXtf",
			"12D3KooWH3uVF6wv47WnArKHbQBCgdjhCXCe5NkqJFTXNdVkjNwq",
			"12D3KooWBhMbLvQGgqByteXqKxKvJkXQhQvKZyYqBzWBKvNqKqhF",
			"12D3KooWQK1QRKAaZJcLhCCXFrGaEZoEJzrXJpvwJQQvQGKjjGzM",
			"12D3KooWFta2AqNqaVjEUrGXQhCXCe5NkqJFTXNdVkjNwqKqhF3B",
		}

		peers := make([]peer.ID, 5)
		for i, peerIDStr := range peerIDs {
			peerID, err := peer.Decode(peerIDStr)
			require.NoError(t, err)
			peers[i] = peerID
		}

		// Fail requests for some peers
		failingPeers := peers[:2]
		workingPeers := peers[2:]

		testError := errors.New("peer failure")

		// Trip circuit breakers for failing peers
		for _, peerID := range failingPeers {
			for i := 0; i < 3; i++ {
				err := bcb.ExecuteWithPeer(peerID, func() error { return testError })
				assert.Equal(t, testError, err)
			}
			assert.Equal(t, StateOpen, bcb.GetPeerState(peerID))
		}

		// Working peers should still function
		for _, peerID := range workingPeers {
			err := bcb.ExecuteWithPeer(peerID, func() error { return nil })
			assert.NoError(t, err)
			assert.Equal(t, StateClosed, bcb.GetPeerState(peerID))
		}

		// Failing peers should reject requests
		for _, peerID := range failingPeers {
			err := bcb.ExecuteWithPeer(peerID, func() error { return nil })
			assert.Equal(t, ErrOpenState, err)
		}
	})

	t.Run("PeerRecoveryAfterFailure", func(t *testing.T) {
		config := Config{
			MaxRequests: 2,
			Interval:    500 * time.Millisecond,
			Timeout:     1 * time.Second,
			ReadyToTrip: func(counts Counts) bool {
				return counts.ConsecutiveFailures >= 2
			},
		}

		bcb := NewBitswapCircuitBreaker(config)
		peerID, err := peer.Decode("12D3KooWGzxzKZYveHXtpG6AsrUJBcWxHBFS2HsEoGTxrMLvKXtf")
		require.NoError(t, err)

		// Trip the circuit breaker
		testError := errors.New("peer down")
		for i := 0; i < 2; i++ {
			bcb.ExecuteWithPeer(peerID, func() error { return testError })
		}
		assert.Equal(t, StateOpen, bcb.GetPeerState(peerID))

		// Wait for half-open
		time.Sleep(1200 * time.Millisecond)
		assert.Equal(t, StateHalfOpen, bcb.GetPeerState(peerID))

		// Successful recovery
		for i := 0; i < 2; i++ {
			err := bcb.ExecuteWithPeer(peerID, func() error { return nil })
			assert.NoError(t, err)
		}

		// Should be closed now
		assert.Equal(t, StateClosed, bcb.GetPeerState(peerID))
	})

	t.Run("HighLoadWithRandomFailures", func(t *testing.T) {
		config := Config{
			MaxRequests: 10,
			Interval:    2 * time.Second,
			Timeout:     3 * time.Second,
			ReadyToTrip: func(counts Counts) bool {
				failureRate := float64(counts.TotalFailures) / float64(counts.Requests)
				return counts.Requests >= 5 && failureRate >= 0.7
			},
		}

		bcb := NewBitswapCircuitBreaker(config)

		// Create multiple peers using simple test IDs
		peers := make([]peer.ID, 10)
		for i := 0; i < 10; i++ {
			// Generate a simple peer ID for testing
			peerID := peer.ID(fmt.Sprintf("test-peer-%d", i))
			peers[i] = peerID
		}

		var wg sync.WaitGroup
		var totalRequests, successfulRequests, rejectedRequests int64

		// High load with random failures
		for i := 0; i < 100; i++ {
			wg.Add(1)
			go func(requestID int) {
				defer wg.Done()

				peerID := peers[rand.Intn(len(peers))]
				atomic.AddInt64(&totalRequests, 1)

				err := bcb.ExecuteWithPeer(peerID, func() error {
					// 30% chance of failure
					if rand.Float32() < 0.3 {
						return errors.New("random failure")
					}
					return nil
				})

				if err == nil {
					atomic.AddInt64(&successfulRequests, 1)
				} else if err == ErrOpenState || err == ErrTooManyRequests {
					atomic.AddInt64(&rejectedRequests, 1)
				}
			}(i)
		}

		wg.Wait()

		total := atomic.LoadInt64(&totalRequests)
		successful := atomic.LoadInt64(&successfulRequests)
		rejected := atomic.LoadInt64(&rejectedRequests)

		t.Logf("High load test results:")
		t.Logf("  Total requests: %d", total)
		t.Logf("  Successful: %d", successful)
		t.Logf("  Rejected by circuit breaker: %d", rejected)

		// Should handle high load gracefully
		assert.Equal(t, int64(100), total)
		assert.Greater(t, successful, int64(30), "Should have substantial successful requests")
	})
}

// TestCircuitBreakerFailurePatterns tests various failure patterns
func TestCircuitBreakerFailurePatterns(t *testing.T) {
	t.Run("ExponentialBackoffPattern", func(t *testing.T) {
		config := Config{
			MaxRequests: 1,
			Interval:    100 * time.Millisecond,
			Timeout:     200 * time.Millisecond,
			ReadyToTrip: func(counts Counts) bool {
				return counts.ConsecutiveFailures >= 1
			},
		}

		cb := NewCircuitBreaker("backoff-test", config)
		testError := errors.New("backoff error")

		// Initial failure
		err := cb.Execute(func() error { return testError })
		assert.Equal(t, testError, err)
		assert.Equal(t, StateOpen, cb.State())

		// Test exponential backoff by checking state transitions
		timeouts := []time.Duration{200 * time.Millisecond, 400 * time.Millisecond, 800 * time.Millisecond}

		for i, timeout := range timeouts {
			// Create new circuit breaker with increasing timeout
			config.Timeout = timeout
			cb = NewCircuitBreaker(fmt.Sprintf("backoff-test-%d", i), config)

			// Trip it
			cb.Execute(func() error { return testError })
			assert.Equal(t, StateOpen, cb.State())

			// Wait less than timeout - should still be open
			time.Sleep(timeout / 2)
			assert.Equal(t, StateOpen, cb.State())

			// Wait for full timeout - should be half-open
			time.Sleep(timeout/2 + 50*time.Millisecond)
			assert.Equal(t, StateHalfOpen, cb.State())
		}
	})

	t.Run("FlappingServicePattern", func(t *testing.T) {
		config := Config{
			MaxRequests: 2,
			Interval:    500 * time.Millisecond,
			Timeout:     1 * time.Second,
			ReadyToTrip: func(counts Counts) bool {
				return counts.ConsecutiveFailures >= 2
			},
		}

		cb := NewCircuitBreaker("flapping-test", config)
		testError := errors.New("flapping error")

		// Simulate flapping service - circuit breaker should handle this gracefully
		for cycle := 0; cycle < 2; cycle++ {
			// Failure phase - trip the circuit breaker
			for i := 0; i < 2; i++ {
				err := cb.Execute(func() error { return testError })
				// First failures should return the original error
				if cb.State() != StateOpen {
					assert.Equal(t, testError, err)
				}
			}

			// Should be open after failures
			assert.Equal(t, StateOpen, cb.State())

			// Wait for half-open transition
			time.Sleep(1200 * time.Millisecond)
			assert.Equal(t, StateHalfOpen, cb.State())

			// One successful request
			err := cb.Execute(func() error { return nil })
			assert.NoError(t, err)

			// Then failure again - should go back to open
			err = cb.Execute(func() error { return testError })
			assert.Equal(t, testError, err)
			assert.Equal(t, StateOpen, cb.State())
		}

		// Verify the circuit breaker handled the flapping pattern
		assert.Equal(t, StateOpen, cb.State())
	})

	t.Run("CascadingFailurePattern", func(t *testing.T) {
		// Simulate cascading failures across multiple circuit breakers
		config := Config{
			MaxRequests: 3,
			Interval:    1 * time.Second,
			Timeout:     2 * time.Second,
			ReadyToTrip: func(counts Counts) bool {
				return counts.ConsecutiveFailures >= 2
			},
		}

		// Create a chain of circuit breakers
		cbs := make([]*CircuitBreaker, 5)
		for i := 0; i < 5; i++ {
			cbs[i] = NewCircuitBreaker(fmt.Sprintf("cascade-%d", i), config)
		}

		cascadeError := errors.New("cascade failure")

		// Simulate cascade - each failure triggers the next
		for i := 0; i < len(cbs); i++ {
			// Trip current circuit breaker
			for j := 0; j < 2; j++ {
				err := cbs[i].Execute(func() error { return cascadeError })
				assert.Equal(t, cascadeError, err)
			}
			assert.Equal(t, StateOpen, cbs[i].State())

			// Verify previous circuit breakers are still open
			for k := 0; k < i; k++ {
				assert.Equal(t, StateOpen, cbs[k].State())
			}

			// Small delay to simulate cascade timing
			time.Sleep(100 * time.Millisecond)
		}

		// All circuit breakers should be open
		for i, cb := range cbs {
			assert.Equal(t, StateOpen, cb.State(), "Circuit breaker %d should be open", i)
		}
	})
}

// TestCircuitBreakerRecoveryStrategies tests different recovery strategies
func TestCircuitBreakerRecoveryStrategies(t *testing.T) {
	t.Run("GradualRecoveryStrategy", func(t *testing.T) {
		config := Config{
			MaxRequests: 1, // Only allow one request in half-open
			Interval:    500 * time.Millisecond,
			Timeout:     1 * time.Second,
			ReadyToTrip: func(counts Counts) bool {
				return counts.ConsecutiveFailures >= 2
			},
		}

		cb := NewCircuitBreaker("gradual-recovery", config)
		testError := errors.New("recovery error")

		// Trip the circuit breaker
		for i := 0; i < 2; i++ {
			cb.Execute(func() error { return testError })
		}
		assert.Equal(t, StateOpen, cb.State())

		// Wait for half-open
		time.Sleep(1200 * time.Millisecond)
		assert.Equal(t, StateHalfOpen, cb.State())

		// First request succeeds
		err := cb.Execute(func() error { return nil })
		assert.NoError(t, err)
		assert.Equal(t, StateClosed, cb.State())

		// Verify it stays closed with more successful requests
		for i := 0; i < 5; i++ {
			err := cb.Execute(func() error { return nil })
			assert.NoError(t, err)
			assert.Equal(t, StateClosed, cb.State())
		}
	})

	t.Run("BulkRecoveryStrategy", func(t *testing.T) {
		config := Config{
			MaxRequests: 5, // Allow multiple requests in half-open
			Interval:    500 * time.Millisecond,
			Timeout:     1 * time.Second,
			ReadyToTrip: func(counts Counts) bool {
				return counts.ConsecutiveFailures >= 3
			},
		}

		cb := NewCircuitBreaker("bulk-recovery", config)
		testError := errors.New("bulk recovery error")

		// Trip the circuit breaker
		for i := 0; i < 3; i++ {
			cb.Execute(func() error { return testError })
		}
		assert.Equal(t, StateOpen, cb.State())

		// Wait for half-open
		time.Sleep(1200 * time.Millisecond)
		assert.Equal(t, StateHalfOpen, cb.State())

		// Multiple successful requests
		var wg sync.WaitGroup
		var successCount int32

		for i := 0; i < 5; i++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				err := cb.Execute(func() error { return nil })
				if err == nil {
					atomic.AddInt32(&successCount, 1)
				}
			}()
		}

		wg.Wait()

		// Should have processed multiple requests and closed
		assert.Greater(t, int(successCount), 0, "Should have successful requests")
		assert.Equal(t, StateClosed, cb.State())
	})

	t.Run("PartialRecoveryStrategy", func(t *testing.T) {
		config := Config{
			MaxRequests: 3,
			Interval:    500 * time.Millisecond,
			Timeout:     1 * time.Second,
			ReadyToTrip: func(counts Counts) bool {
				return counts.ConsecutiveFailures >= 2
			},
		}

		cb := NewCircuitBreaker("partial-recovery", config)
		testError := errors.New("partial recovery error")

		// Trip the circuit breaker
		for i := 0; i < 2; i++ {
			cb.Execute(func() error { return testError })
		}
		assert.Equal(t, StateOpen, cb.State())

		// Wait for half-open
		time.Sleep(1200 * time.Millisecond)
		assert.Equal(t, StateHalfOpen, cb.State())

		// Partial success - some requests succeed, some fail
		successCount := 0
		for i := 0; i < 3; i++ {
			var err error
			if i < 2 { // First two succeed
				err = cb.Execute(func() error { return nil })
				if err == nil {
					successCount++
				}
			} else { // Last one fails
				err = cb.Execute(func() error { return testError })
				if err == testError {
					// Should go back to open on failure in half-open
					assert.Equal(t, StateOpen, cb.State())
					break
				}
			}
		}

		assert.Equal(t, 2, successCount, "Should have 2 successful requests before failure")
		assert.Equal(t, StateOpen, cb.State(), "Should be open after failure in half-open")
	})
}

// BenchmarkCircuitBreakerUnderLoad benchmarks circuit breaker performance under load
func BenchmarkCircuitBreakerUnderLoad(b *testing.B) {
	config := DefaultBitswapCircuitBreakerConfig()
	cb := NewCircuitBreaker("benchmark", config)

	b.Run("SuccessfulRequests", func(b *testing.B) {
		b.ResetTimer()
		b.RunParallel(func(pb *testing.PB) {
			for pb.Next() {
				cb.Execute(func() error { return nil })
			}
		})
	})

	b.Run("FailedRequests", func(b *testing.B) {
		testError := errors.New("benchmark error")
		b.ResetTimer()
		b.RunParallel(func(pb *testing.PB) {
			for pb.Next() {
				cb.Execute(func() error { return testError })
			}
		})
	})

	b.Run("MixedRequests", func(b *testing.B) {
		testError := errors.New("benchmark error")
		b.ResetTimer()
		b.RunParallel(func(pb *testing.PB) {
			i := 0
			for pb.Next() {
				if i%4 == 0 { // 25% failure rate
					cb.Execute(func() error { return testError })
				} else {
					cb.Execute(func() error { return nil })
				}
				i++
			}
		})
	})
}

// BenchmarkBitswapCircuitBreakerUnderLoad benchmarks Bitswap circuit breaker under load
func BenchmarkBitswapCircuitBreakerUnderLoad(b *testing.B) {
	config := DefaultBitswapCircuitBreakerConfig()
	bcb := NewBitswapCircuitBreaker(config)

	// Create test peers
	peers := make([]peer.ID, 100)
	for i := 0; i < 100; i++ {
		peerID, _ := peer.Decode(fmt.Sprintf("12D3KooWGzxzKZYveHXtpG6AsrUJBcWxHBFS2HsEoGTxrMLvKXt%02d", i))
		peers[i] = peerID
	}

	b.Run("MultiPeerSuccessfulRequests", func(b *testing.B) {
		b.ResetTimer()
		b.RunParallel(func(pb *testing.PB) {
			for pb.Next() {
				peerID := peers[rand.Intn(len(peers))]
				bcb.ExecuteWithPeer(peerID, func() error { return nil })
			}
		})
	})

	b.Run("MultiPeerMixedRequests", func(b *testing.B) {
		testError := errors.New("benchmark error")
		b.ResetTimer()
		b.RunParallel(func(pb *testing.PB) {
			i := 0
			for pb.Next() {
				peerID := peers[rand.Intn(len(peers))]
				if i%5 == 0 { // 20% failure rate
					bcb.ExecuteWithPeer(peerID, func() error { return testError })
				} else {
					bcb.ExecuteWithPeer(peerID, func() error { return nil })
				}
				i++
			}
		})
	})
}
