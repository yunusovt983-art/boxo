package bitswap_test

import (
	"errors"
	"fmt"
	"math/rand"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/ipfs/boxo/bitswap/circuitbreaker"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestCircuitBreakerValidationScenarios tests comprehensive circuit breaker validation
func TestCircuitBreakerValidationScenarios(t *testing.T) {
	t.Run("StateTransitionValidation", func(t *testing.T) {
		config := circuitbreaker.Config{
			MaxRequests: 3,
			Interval:    500 * time.Millisecond,
			Timeout:     1 * time.Second,
			ReadyToTrip: func(counts circuitbreaker.Counts) bool {
				return counts.ConsecutiveFailures >= 2
			},
		}

		cb := circuitbreaker.NewCircuitBreaker("state-validation", config)
		testError := errors.New("validation error")

		// Initial state should be closed
		assert.Equal(t, circuitbreaker.StateClosed, cb.State())

		// First failure - should remain closed
		err := cb.Execute(func() error { return testError })
		assert.Equal(t, testError, err)
		assert.Equal(t, circuitbreaker.StateClosed, cb.State())

		// Second failure - should trip to open
		err = cb.Execute(func() error { return testError })
		assert.Equal(t, testError, err)
		assert.Equal(t, circuitbreaker.StateOpen, cb.State())

		// Subsequent requests should be rejected
		err = cb.Execute(func() error { return nil })
		assert.Equal(t, circuitbreaker.ErrOpenState, err)
		assert.Equal(t, circuitbreaker.StateOpen, cb.State())

		// Wait for timeout - should transition to half-open
		time.Sleep(1200 * time.Millisecond)
		assert.Equal(t, circuitbreaker.StateHalfOpen, cb.State())

		// Successful request in half-open should close circuit
		err = cb.Execute(func() error { return nil })
		assert.NoError(t, err)
		assert.Equal(t, circuitbreaker.StateClosed, cb.State())
	})

	t.Run("MaxRequestsValidation", func(t *testing.T) {
		config := circuitbreaker.Config{
			MaxRequests: 2, // Only allow 2 requests in half-open
			Interval:    500 * time.Millisecond,
			Timeout:     1 * time.Second,
			ReadyToTrip: func(counts circuitbreaker.Counts) bool {
				return counts.ConsecutiveFailures >= 1
			},
		}

		cb := circuitbreaker.NewCircuitBreaker("max-requests-validation", config)
		testError := errors.New("max requests error")

		// Trip the circuit breaker
		cb.Execute(func() error { return testError })
		assert.Equal(t, circuitbreaker.StateOpen, cb.State())

		// Wait for half-open
		time.Sleep(1200 * time.Millisecond)
		assert.Equal(t, circuitbreaker.StateHalfOpen, cb.State())

		// First request should succeed
		err := cb.Execute(func() error { return nil })
		assert.NoError(t, err)
		assert.Equal(t, circuitbreaker.StateHalfOpen, cb.State())

		// Second request should succeed
		err = cb.Execute(func() error { return nil })
		assert.NoError(t, err)
		assert.Equal(t, circuitbreaker.StateClosed, cb.State()) // Should close after MaxRequests

		// Third request should be allowed (circuit is closed)
		err = cb.Execute(func() error { return nil })
		assert.NoError(t, err)
		assert.Equal(t, circuitbreaker.StateClosed, cb.State())
	})

	t.Run("IntervalResetValidation", func(t *testing.T) {
		config := circuitbreaker.Config{
			MaxRequests: 5,
			Interval:    1 * time.Second, // Reset counts every second
			Timeout:     2 * time.Second,
			ReadyToTrip: func(counts circuitbreaker.Counts) bool {
				return counts.ConsecutiveFailures >= 3
			},
		}

		cb := circuitbreaker.NewCircuitBreaker("interval-validation", config)
		testError := errors.New("interval error")

		// Generate 2 failures
		for i := 0; i < 2; i++ {
			err := cb.Execute(func() error { return testError })
			assert.Equal(t, testError, err)
		}
		assert.Equal(t, circuitbreaker.StateClosed, cb.State()) // Should still be closed

		// Wait for interval to reset
		time.Sleep(1200 * time.Millisecond)

		// Generate 2 more failures (should be reset due to interval)
		for i := 0; i < 2; i++ {
			err := cb.Execute(func() error { return testError })
			assert.Equal(t, testError, err)
		}
		assert.Equal(t, circuitbreaker.StateClosed, cb.State()) // Should still be closed due to reset

		// One more failure should trip it
		err := cb.Execute(func() error { return testError })
		assert.Equal(t, testError, err)
		assert.Equal(t, circuitbreaker.StateOpen, cb.State()) // Should be open now
	})

	t.Run("ConcurrentRequestValidation", func(t *testing.T) {
		config := circuitbreaker.Config{
			MaxRequests: 10,
			Interval:    1 * time.Second,
			Timeout:     2 * time.Second,
			ReadyToTrip: func(counts circuitbreaker.Counts) bool {
				return counts.ConsecutiveFailures >= 5
			},
		}

		cb := circuitbreaker.NewCircuitBreaker("concurrent-validation", config)
		testError := errors.New("concurrent error")

		var wg sync.WaitGroup
		var successCount, failureCount, rejectedCount int64

		// Concurrent requests with mixed results
		for i := 0; i < 100; i++ {
			wg.Add(1)
			go func(requestID int) {
				defer wg.Done()

				var err error
				if requestID%3 == 0 { // Every third request fails
					err = cb.Execute(func() error { return testError })
					if err == testError {
						atomic.AddInt64(&failureCount, 1)
					} else if err == circuitbreaker.ErrOpenState || err == circuitbreaker.ErrTooManyRequests {
						atomic.AddInt64(&rejectedCount, 1)
					}
				} else {
					err = cb.Execute(func() error { return nil })
					if err == nil {
						atomic.AddInt64(&successCount, 1)
					} else if err == circuitbreaker.ErrOpenState || err == circuitbreaker.ErrTooManyRequests {
						atomic.AddInt64(&rejectedCount, 1)
					}
				}
			}(i)
		}

		wg.Wait()

		t.Logf("Concurrent validation results: %d successes, %d failures, %d rejected",
			successCount, failureCount, rejectedCount)

		// Should handle concurrent requests properly
		assert.Greater(t, successCount, int64(30), "Should have successful requests")
		assert.Greater(t, failureCount, int64(10), "Should have some failures")
		// May or may not have rejected requests depending on timing
	})
}

// TestBitswapCircuitBreakerValidation tests Bitswap-specific circuit breaker validation
func TestBitswapCircuitBreakerValidation(t *testing.T) {
	t.Run("PeerIsolationValidation", func(t *testing.T) {
		config := circuitbreaker.Config{
			MaxRequests: 3,
			Interval:    1 * time.Second,
			Timeout:     2 * time.Second,
			ReadyToTrip: func(counts circuitbreaker.Counts) bool {
				return counts.ConsecutiveFailures >= 2
			},
		}

		bcb := circuitbreaker.NewBitswapCircuitBreaker(config)

		// Create test peers
		peer1, err := peer.Decode("12D3KooWGzxzKZYveHXtpG6AsrUJBcWxHBFS2HsEoGTxrMLvKXtf")
		require.NoError(t, err)
		peer2, err := peer.Decode("12D3KooWH3uVF6wv47WnArKHbQBCgdjhCXCe5NkqJFTXNdVkjNwq")
		require.NoError(t, err)

		testError := errors.New("peer isolation error")

		// Trip circuit breaker for peer1
		for i := 0; i < 2; i++ {
			err := bcb.ExecuteWithPeer(peer1, func() error { return testError })
			assert.Equal(t, testError, err)
		}
		assert.Equal(t, circuitbreaker.StateOpen, bcb.GetPeerState(peer1))

		// Peer2 should still be functional
		err = bcb.ExecuteWithPeer(peer2, func() error { return nil })
		assert.NoError(t, err)
		assert.Equal(t, circuitbreaker.StateClosed, bcb.GetPeerState(peer2))

		// Peer1 should reject requests
		err = bcb.ExecuteWithPeer(peer1, func() error { return nil })
		assert.Equal(t, circuitbreaker.ErrOpenState, err)

		// Peer2 should continue working
		err = bcb.ExecuteWithPeer(peer2, func() error { return nil })
		assert.NoError(t, err)
	})

	t.Run("PeerRecoveryValidation", func(t *testing.T) {
		config := circuitbreaker.Config{
			MaxRequests: 2,
			Interval:    500 * time.Millisecond,
			Timeout:     1 * time.Second,
			ReadyToTrip: func(counts circuitbreaker.Counts) bool {
				return counts.ConsecutiveFailures >= 2
			},
		}

		bcb := circuitbreaker.NewBitswapCircuitBreaker(config)
		peerID, err := peer.Decode("12D3KooWGzxzKZYveHXtpG6AsrUJBcWxHBFS2HsEoGTxrMLvKXtf")
		require.NoError(t, err)

		testError := errors.New("peer recovery error")

		// Trip the circuit breaker
		for i := 0; i < 2; i++ {
			bcb.ExecuteWithPeer(peerID, func() error { return testError })
		}
		assert.Equal(t, circuitbreaker.StateOpen, bcb.GetPeerState(peerID))

		// Wait for half-open
		time.Sleep(1200 * time.Millisecond)
		assert.Equal(t, circuitbreaker.StateHalfOpen, bcb.GetPeerState(peerID))

		// Successful recovery
		for i := 0; i < 2; i++ {
			err := bcb.ExecuteWithPeer(peerID, func() error { return nil })
			assert.NoError(t, err)
		}

		// Should be closed now
		assert.Equal(t, circuitbreaker.StateClosed, bcb.GetPeerState(peerID))

		// Should continue working normally
		err = bcb.ExecuteWithPeer(peerID, func() error { return nil })
		assert.NoError(t, err)
		assert.Equal(t, circuitbreaker.StateClosed, bcb.GetPeerState(peerID))
	})

	t.Run("MultiPeerConcurrentValidation", func(t *testing.T) {
		config := circuitbreaker.Config{
			MaxRequests: 5,
			Interval:    1 * time.Second,
			Timeout:     2 * time.Second,
			ReadyToTrip: func(counts circuitbreaker.Counts) bool {
				return counts.ConsecutiveFailures >= 3
			},
		}

		bcb := circuitbreaker.NewBitswapCircuitBreaker(config)

		// Create multiple test peers
		peers := make([]peer.ID, 20)
		for i := 0; i < 20; i++ {
			peerID, err := peer.Decode(fmt.Sprintf("12D3KooWGzxzKZYveHXtpG6AsrUJBcWxHBFS2HsEoGTxrMLvKXt%02d", i))
			require.NoError(t, err)
			peers[i] = peerID
		}

		var wg sync.WaitGroup
		var totalRequests, successfulRequests, rejectedRequests int64

		// Concurrent requests across multiple peers
		for i := 0; i < 200; i++ {
			wg.Add(1)
			go func(requestID int) {
				defer wg.Done()
				atomic.AddInt64(&totalRequests, 1)

				peerID := peers[rand.Intn(len(peers))]

				err := bcb.ExecuteWithPeer(peerID, func() error {
					// 25% failure rate
					if rand.Float32() < 0.25 {
						return errors.New("random failure")
					}
					return nil
				})

				if err == nil {
					atomic.AddInt64(&successfulRequests, 1)
				} else if err == circuitbreaker.ErrOpenState || err == circuitbreaker.ErrTooManyRequests {
					atomic.AddInt64(&rejectedRequests, 1)
				}
			}(i)
		}

		wg.Wait()

		total := atomic.LoadInt64(&totalRequests)
		successful := atomic.LoadInt64(&successfulRequests)
		rejected := atomic.LoadInt64(&rejectedRequests)

		t.Logf("Multi-peer concurrent validation:")
		t.Logf("  Total requests: %d", total)
		t.Logf("  Successful: %d", successful)
		t.Logf("  Rejected: %d", rejected)

		assert.Equal(t, int64(200), total)
		assert.Greater(t, successful, int64(100), "Should have substantial successful requests")
	})
}

// TestCircuitBreakerEdgeCases tests edge cases and boundary conditions
func TestCircuitBreakerEdgeCases(t *testing.T) {
	t.Run("ZeroMaxRequestsEdgeCase", func(t *testing.T) {
		config := circuitbreaker.Config{
			MaxRequests: 0, // Edge case: zero max requests
			Interval:    1 * time.Second,
			Timeout:     1 * time.Second,
			ReadyToTrip: func(counts circuitbreaker.Counts) bool {
				return counts.ConsecutiveFailures >= 1
			},
		}

		cb := circuitbreaker.NewCircuitBreaker("zero-max-requests", config)
		testError := errors.New("zero max error")

		// Trip the circuit breaker
		err := cb.Execute(func() error { return testError })
		assert.Equal(t, testError, err)
		assert.Equal(t, circuitbreaker.StateOpen, cb.State())

		// Wait for half-open
		time.Sleep(1200 * time.Millisecond)
		assert.Equal(t, circuitbreaker.StateHalfOpen, cb.State())

		// With MaxRequests=0, any successful request should close the circuit
		err = cb.Execute(func() error { return nil })
		assert.NoError(t, err)
		assert.Equal(t, circuitbreaker.StateClosed, cb.State())
	})

	t.Run("VeryShortTimeoutEdgeCase", func(t *testing.T) {
		config := circuitbreaker.Config{
			MaxRequests: 3,
			Interval:    1 * time.Second,
			Timeout:     1 * time.Millisecond, // Very short timeout
			ReadyToTrip: func(counts circuitbreaker.Counts) bool {
				return counts.ConsecutiveFailures >= 1
			},
		}

		cb := circuitbreaker.NewCircuitBreaker("short-timeout", config)
		testError := errors.New("short timeout error")

		// Trip the circuit breaker
		err := cb.Execute(func() error { return testError })
		assert.Equal(t, testError, err)
		assert.Equal(t, circuitbreaker.StateOpen, cb.State())

		// Should transition to half-open very quickly
		time.Sleep(10 * time.Millisecond)
		assert.Equal(t, circuitbreaker.StateHalfOpen, cb.State())
	})

	t.Run("AlwaysTripReadyToTripEdgeCase", func(t *testing.T) {
		config := circuitbreaker.Config{
			MaxRequests: 3,
			Interval:    1 * time.Second,
			Timeout:     1 * time.Second,
			ReadyToTrip: func(counts circuitbreaker.Counts) bool {
				return true // Always trip
			},
		}

		cb := circuitbreaker.NewCircuitBreaker("always-trip", config)

		// Even successful requests should trip the circuit
		err := cb.Execute(func() error { return nil })
		assert.NoError(t, err)
		assert.Equal(t, circuitbreaker.StateOpen, cb.State())
	})

	t.Run("NeverTripReadyToTripEdgeCase", func(t *testing.T) {
		config := circuitbreaker.Config{
			MaxRequests: 3,
			Interval:    1 * time.Second,
			Timeout:     1 * time.Second,
			ReadyToTrip: func(counts circuitbreaker.Counts) bool {
				return false // Never trip
			},
		}

		cb := circuitbreaker.NewCircuitBreaker("never-trip", config)
		testError := errors.New("never trip error")

		// Even many failures shouldn't trip the circuit
		for i := 0; i < 10; i++ {
			err := cb.Execute(func() error { return testError })
			assert.Equal(t, testError, err)
			assert.Equal(t, circuitbreaker.StateClosed, cb.State())
		}
	})

	t.Run("RapidStateTransitionEdgeCase", func(t *testing.T) {
		config := circuitbreaker.Config{
			MaxRequests: 1,
			Interval:    100 * time.Millisecond,
			Timeout:     100 * time.Millisecond,
			ReadyToTrip: func(counts circuitbreaker.Counts) bool {
				return counts.ConsecutiveFailures >= 1
			},
		}

		cb := circuitbreaker.NewCircuitBreaker("rapid-transition", config)
		testError := errors.New("rapid error")

		// Rapid state transitions
		for cycle := 0; cycle < 5; cycle++ {
			// Trip
			err := cb.Execute(func() error { return testError })
			assert.Equal(t, testError, err)
			assert.Equal(t, circuitbreaker.StateOpen, cb.State())

			// Wait for half-open
			time.Sleep(150 * time.Millisecond)
			assert.Equal(t, circuitbreaker.StateHalfOpen, cb.State())

			// Recover
			err = cb.Execute(func() error { return nil })
			assert.NoError(t, err)
			assert.Equal(t, circuitbreaker.StateClosed, cb.State())
		}
	})
}

// TestCircuitBreakerMetricsValidation tests that circuit breaker metrics are accurate
func TestCircuitBreakerMetricsValidation(t *testing.T) {
	t.Run("CountsAccuracyValidation", func(t *testing.T) {
		var capturedCounts circuitbreaker.Counts
		config := circuitbreaker.Config{
			MaxRequests: 10,
			Interval:    2 * time.Second,
			Timeout:     1 * time.Second,
			ReadyToTrip: func(counts circuitbreaker.Counts) bool {
				capturedCounts = counts
				return counts.ConsecutiveFailures >= 5
			},
		}

		cb := circuitbreaker.NewCircuitBreaker("counts-validation", config)
		testError := errors.New("counts error")

		// Execute mixed requests
		expectedRequests := 0
		expectedFailures := 0
		expectedSuccesses := 0

		for i := 0; i < 10; i++ {
			expectedRequests++
			if i%3 == 0 { // Every third request fails
				err := cb.Execute(func() error { return testError })
				if err == testError {
					expectedFailures++
				}
			} else {
				err := cb.Execute(func() error { return nil })
				if err == nil {
					expectedSuccesses++
				}
			}
		}

		// Validate counts
		assert.Equal(t, uint32(expectedRequests), capturedCounts.Requests)
		assert.Equal(t, uint32(expectedFailures), capturedCounts.TotalFailures)
		assert.Equal(t, uint32(expectedSuccesses), capturedCounts.TotalSuccesses)
	})

	t.Run("ConsecutiveFailuresValidation", func(t *testing.T) {
		var maxConsecutiveFailures uint32
		config := circuitbreaker.Config{
			MaxRequests: 10,
			Interval:    2 * time.Second,
			Timeout:     1 * time.Second,
			ReadyToTrip: func(counts circuitbreaker.Counts) bool {
				if counts.ConsecutiveFailures > maxConsecutiveFailures {
					maxConsecutiveFailures = counts.ConsecutiveFailures
				}
				return counts.ConsecutiveFailures >= 10 // High threshold to capture all
			},
		}

		cb := circuitbreaker.NewCircuitBreaker("consecutive-validation", config)
		testError := errors.New("consecutive error")

		// Pattern: 3 failures, 1 success, 4 failures, 1 success
		pattern := []bool{false, false, false, true, false, false, false, false, true}

		for _, shouldSucceed := range pattern {
			if shouldSucceed {
				cb.Execute(func() error { return nil })
			} else {
				cb.Execute(func() error { return testError })
			}
		}

		// Maximum consecutive failures should be 4
		assert.Equal(t, uint32(4), maxConsecutiveFailures)
	})
}

// BenchmarkCircuitBreakerValidation benchmarks circuit breaker performance
func BenchmarkCircuitBreakerValidation(b *testing.B) {
	config := circuitbreaker.Config{
		MaxRequests: 100,
		Interval:    10 * time.Second,
		Timeout:     5 * time.Second,
		ReadyToTrip: func(counts circuitbreaker.Counts) bool {
			return counts.ConsecutiveFailures >= 50
		},
	}

	b.Run("ClosedStatePerformance", func(b *testing.B) {
		cb := circuitbreaker.NewCircuitBreaker("closed-perf", config)
		b.ResetTimer()
		b.RunParallel(func(pb *testing.PB) {
			for pb.Next() {
				cb.Execute(func() error { return nil })
			}
		})
	})

	b.Run("OpenStatePerformance", func(b *testing.B) {
		cb := circuitbreaker.NewCircuitBreaker("open-perf", config)
		// Trip the circuit breaker
		for i := 0; i < 50; i++ {
			cb.Execute(func() error { return errors.New("trip error") })
		}

		b.ResetTimer()
		b.RunParallel(func(pb *testing.PB) {
			for pb.Next() {
				cb.Execute(func() error { return nil })
			}
		})
	})

	b.Run("MixedStatePerformance", func(b *testing.B) {
		cb := circuitbreaker.NewCircuitBreaker("mixed-perf", config)
		b.ResetTimer()
		b.RunParallel(func(pb *testing.PB) {
			i := 0
			for pb.Next() {
				if i%10 == 0 {
					cb.Execute(func() error { return errors.New("occasional error") })
				} else {
					cb.Execute(func() error { return nil })
				}
				i++
			}
		})
	})
}
