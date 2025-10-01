package circuitbreaker

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestBitswapProtectionManager_BasicFunctionality(t *testing.T) {
	config := DefaultProtectionConfig()
	config.CircuitBreaker.Timeout = 100 * time.Millisecond
	config.HealthMonitor.CleanupInterval = time.Hour // Disable cleanup
	config.Recovery.CooldownPeriod = 50 * time.Millisecond

	pm := NewBitswapProtectionManager(config)
	defer pm.Close()

	peerID, err := peer.Decode("12D3KooWGzxzKZYveHXtpG6AsrUJBcWxHBFS2HsEoGTxrMLvKXtf")
	require.NoError(t, err)

	ctx := context.Background()

	// Test successful request
	err = pm.ExecuteRequest(ctx, peerID, func(ctx context.Context) error {
		return nil
	})
	assert.NoError(t, err)

	// Verify peer status
	status := pm.GetPeerStatus(peerID)
	assert.Equal(t, peerID, status.PeerID)
	assert.Equal(t, StateClosed, status.CircuitState)
	assert.Equal(t, HealthStatusUnknown, status.HealthStatus) // Not enough data yet
	assert.False(t, status.IsRecovering)
}

func TestBitswapProtectionManager_CircuitBreakerIntegration(t *testing.T) {
	config := DefaultProtectionConfig()
	config.CircuitBreaker.ReadyToTrip = func(counts Counts) bool {
		return counts.ConsecutiveFailures >= 3
	}
	config.CircuitBreaker.Timeout = 100 * time.Millisecond
	config.HealthMonitor.CleanupInterval = time.Hour // Disable cleanup
	config.Recovery.CooldownPeriod = 50 * time.Millisecond

	pm := NewBitswapProtectionManager(config)
	defer pm.Close()

	peerID, err := peer.Decode("12D3KooWGzxzKZYveHXtpG6AsrUJBcWxHBFS2HsEoGTxrMLvKXtf")
	require.NoError(t, err)

	ctx := context.Background()
	testErr := errors.New("test error")

	// Generate failures to trip circuit breaker
	for i := 0; i < 3; i++ {
		err = pm.ExecuteRequest(ctx, peerID, func(ctx context.Context) error {
			return testErr
		})
		assert.Equal(t, testErr, err)
	}

	// Circuit breaker should be open
	status := pm.GetPeerStatus(peerID)
	assert.Equal(t, StateOpen, status.CircuitState)
	assert.Equal(t, HealthStatusUnhealthy, status.HealthStatus)

	// Requests should be rejected
	err = pm.ExecuteRequest(ctx, peerID, func(ctx context.Context) error {
		return nil
	})
	assert.Equal(t, ErrOpenState, err)
}

func TestBitswapProtectionManager_AutoRecovery(t *testing.T) {
	config := DefaultProtectionConfig()
	config.CircuitBreaker.ReadyToTrip = func(counts Counts) bool {
		return counts.ConsecutiveFailures >= 2
	}
	config.CircuitBreaker.Timeout = 50 * time.Millisecond
	config.HealthMonitor.CleanupInterval = time.Hour // Disable cleanup
	config.Recovery.CooldownPeriod = 10 * time.Millisecond
	config.Recovery.InitialRecoveryRate = 0.5
	config.EnableAutoRecovery = true

	pm := NewBitswapProtectionManager(config)
	defer pm.Close()

	peerID, err := peer.Decode("12D3KooWGzxzKZYveHXtpG6AsrUJBcWxHBFS2HsEoGTxrMLvKXtf")
	require.NoError(t, err)

	ctx := context.Background()
	testErr := errors.New("test error")

	// Trip circuit breaker
	for i := 0; i < 2; i++ {
		pm.ExecuteRequest(ctx, peerID, func(ctx context.Context) error {
			return testErr
		})
	}

	// Wait for circuit breaker to go to half-open
	time.Sleep(60 * time.Millisecond)

	// Check if recovery is started (this depends on the implementation details)
	status := pm.GetPeerStatus(peerID)
	assert.Equal(t, StateHalfOpen, status.CircuitState)
}

func TestBitswapProtectionManager_OverallStatus(t *testing.T) {
	config := DefaultProtectionConfig()
	config.HealthMonitor.CleanupInterval = time.Hour // Disable cleanup

	pm := NewBitswapProtectionManager(config)
	defer pm.Close()

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
	}

	ctx := context.Background()

	// Make some requests to generate data
	for _, peerID := range peers {
		for j := 0; j < 5; j++ {
			pm.ExecuteRequest(ctx, peerID, func(ctx context.Context) error {
				return nil
			})
		}
	}

	// Get overall status
	overallStatus := pm.GetOverallStatus()
	assert.Equal(t, 3, overallStatus.TotalPeers)
	assert.True(t, overallStatus.IsHealthy())

	summary := overallStatus.GetSummary()
	assert.Contains(t, summary, "3 peers")
}

func TestBitswapProtectionManager_ConcurrentRequests(t *testing.T) {
	config := DefaultProtectionConfig()
	config.HealthMonitor.CleanupInterval = time.Hour // Disable cleanup

	pm := NewBitswapProtectionManager(config)
	defer pm.Close()

	peerID, err := peer.Decode("12D3KooWGzxzKZYveHXtpG6AsrUJBcWxHBFS2HsEoGTxrMLvKXtf")
	require.NoError(t, err)

	ctx := context.Background()
	numGoroutines := 10
	requestsPerGoroutine := 100

	var wg sync.WaitGroup
	var successCount, errorCount int64
	var mu sync.Mutex

	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < requestsPerGoroutine; j++ {
				err := pm.ExecuteRequest(ctx, peerID, func(ctx context.Context) error {
					// Simulate some work
					time.Sleep(time.Microsecond)
					return nil
				})

				mu.Lock()
				if err == nil {
					successCount++
				} else {
					errorCount++
				}
				mu.Unlock()
			}
		}()
	}

	wg.Wait()

	// All requests should succeed since we're not generating errors
	assert.Equal(t, int64(numGoroutines*requestsPerGoroutine), successCount)
	assert.Equal(t, int64(0), errorCount)

	// Check final status
	status := pm.GetPeerStatus(peerID)
	assert.Equal(t, StateClosed, status.CircuitState)
	assert.True(t, status.Health.TotalRequests > 0)
}

func TestBitswapProtectionManager_FailureRecoveryScenario(t *testing.T) {
	config := DefaultProtectionConfig()
	config.CircuitBreaker.ReadyToTrip = func(counts Counts) bool {
		return counts.ConsecutiveFailures >= 3
	}
	config.CircuitBreaker.Timeout = 50 * time.Millisecond
	config.CircuitBreaker.MaxRequests = 2
	config.HealthMonitor.CleanupInterval = time.Hour // Disable cleanup
	config.Recovery.CooldownPeriod = 10 * time.Millisecond
	config.Recovery.SuccessThreshold = 2

	pm := NewBitswapProtectionManager(config)
	defer pm.Close()

	peerID, err := peer.Decode("12D3KooWGzxzKZYveHXtpG6AsrUJBcWxHBFS2HsEoGTxrMLvKXtf")
	require.NoError(t, err)

	ctx := context.Background()
	testErr := errors.New("test error")

	// Phase 1: Trip the circuit breaker
	for i := 0; i < 3; i++ {
		err = pm.ExecuteRequest(ctx, peerID, func(ctx context.Context) error {
			return testErr
		})
		assert.Equal(t, testErr, err)
	}

	status := pm.GetPeerStatus(peerID)
	assert.Equal(t, StateOpen, status.CircuitState)

	// Phase 2: Wait for half-open state
	time.Sleep(60 * time.Millisecond)

	status = pm.GetPeerStatus(peerID)
	assert.Equal(t, StateHalfOpen, status.CircuitState)

	// Phase 3: Successful requests should close the circuit
	for i := 0; i < 2; i++ {
		err = pm.ExecuteRequest(ctx, peerID, func(ctx context.Context) error {
			return nil
		})
		assert.NoError(t, err)
	}

	status = pm.GetPeerStatus(peerID)
	assert.Equal(t, StateClosed, status.CircuitState)
}

func TestBitswapProtectionManager_MetricsUpdates(t *testing.T) {
	config := DefaultProtectionConfig()
	config.MetricsUpdateInterval = 10 * time.Millisecond
	config.HealthMonitor.CleanupInterval = time.Hour // Disable cleanup

	pm := NewBitswapProtectionManager(config)
	defer pm.Close()

	peerID, err := peer.Decode("12D3KooWGzxzKZYveHXtpG6AsrUJBcWxHBFS2HsEoGTxrMLvKXtf")
	require.NoError(t, err)

	ctx := context.Background()

	// Make some requests
	for i := 0; i < 10; i++ {
		pm.ExecuteRequest(ctx, peerID, func(ctx context.Context) error {
			return nil
		})
	}

	// Wait for metrics update
	time.Sleep(20 * time.Millisecond)

	overallStatus := pm.GetOverallStatus()
	assert.True(t, overallStatus.Metrics.TotalRequests > 0)
	assert.True(t, overallStatus.Metrics.SuccessfulRequests > 0)
	assert.Equal(t, uint64(0), overallStatus.Metrics.FailedRequests)
}

func TestDefaultProtectionConfig(t *testing.T) {
	config := DefaultProtectionConfig()

	assert.NotNil(t, config.CircuitBreaker)
	assert.NotNil(t, config.HealthMonitor)
	assert.NotNil(t, config.Recovery)
	assert.True(t, config.EnableAutoRecovery)
	assert.True(t, config.EnableHealthBasedTripping)
	assert.Equal(t, 30*time.Second, config.MetricsUpdateInterval)
}

func TestPeerStatus_Comprehensive(t *testing.T) {
	config := DefaultProtectionConfig()
	config.HealthMonitor.CleanupInterval = time.Hour // Disable cleanup

	pm := NewBitswapProtectionManager(config)
	defer pm.Close()

	peerID, err := peer.Decode("12D3KooWGzxzKZYveHXtpG6AsrUJBcWxHBFS2HsEoGTxrMLvKXtf")
	require.NoError(t, err)

	ctx := context.Background()

	// Make enough requests to get meaningful health data
	for i := 0; i < 10; i++ {
		pm.ExecuteRequest(ctx, peerID, func(ctx context.Context) error {
			return nil
		})
	}

	status := pm.GetPeerStatus(peerID)
	assert.Equal(t, peerID, status.PeerID)
	assert.Equal(t, StateClosed, status.CircuitState)
	assert.Equal(t, HealthStatusHealthy, status.HealthStatus)
	assert.False(t, status.IsRecovering)
	assert.Equal(t, float64(1.0), status.RecoveryRate)
	assert.True(t, status.LastUpdated.After(time.Time{}))

	// Verify circuit counts
	assert.Equal(t, uint32(10), status.CircuitCounts.Requests)
	assert.Equal(t, uint32(10), status.CircuitCounts.TotalSuccesses)
	assert.Equal(t, uint32(0), status.CircuitCounts.TotalFailures)

	// Verify health data
	assert.Equal(t, int64(10), status.Health.TotalRequests)
	assert.Equal(t, float64(1.0), status.Health.SuccessRate)
	assert.True(t, status.Health.IsHealthy)
}

func TestOverallStatus_HealthySystem(t *testing.T) {
	config := DefaultProtectionConfig()
	config.HealthMonitor.CleanupInterval = time.Hour // Disable cleanup

	pm := NewBitswapProtectionManager(config)
	defer pm.Close()

	// Create multiple healthy peers with different IDs
	peerIDs := []string{
		"12D3KooWGzxzKZYveHXtpG6AsrUJBcWxHBFS2HsEoGTxrMLvKXtf",
		"12D3KooWH3uVF6wv47WnArKHbQBCgdjhCXCe5NkqJFTXNdVkjNwq",
		"12D3KooWBhMbLvQGgqByteXqKxKvJkXQhQvKZyYqBzWBKvNqKqhF",
		"12D3KooWQfGkPUkoLDEeJE3H3ZTmu9BZvAdbJpmhha8WpjeSLKMM",
		"12D3KooWRzCVDwHUkgdq7eRw9HcaKAtEzfnUBEfLjwj8ryq6V3GK",
	}
	
	peers := make([]peer.ID, 5)
	for i, peerIDStr := range peerIDs {
		peerID, err := peer.Decode(peerIDStr)
		require.NoError(t, err)
		peers[i] = peerID

		// Make requests to establish health
		for j := 0; j < 10; j++ {
			pm.ExecuteRequest(context.Background(), peerID, func(ctx context.Context) error {
				return nil
			})
		}
	}

	overallStatus := pm.GetOverallStatus()
	assert.True(t, overallStatus.IsHealthy())
	assert.Equal(t, 5, overallStatus.TotalPeers)
	assert.Equal(t, 5, overallStatus.HealthStats.HealthyPeers)
	assert.Equal(t, 0, overallStatus.HealthStats.UnhealthyPeers)

	summary := overallStatus.GetSummary()
	assert.Contains(t, summary, "5 peers")
	assert.Contains(t, summary, "5 healthy")
}

func TestOverallStatus_UnhealthySystem(t *testing.T) {
	config := DefaultProtectionConfig()
	config.CircuitBreaker.ReadyToTrip = func(counts Counts) bool {
		return counts.ConsecutiveFailures >= 2
	}
	config.HealthMonitor.CleanupInterval = time.Hour // Disable cleanup

	pm := NewBitswapProtectionManager(config)
	defer pm.Close()

	// Create peers with mixed health
	healthyPeer, _ := peer.Decode("12D3KooWGzxzKZYveHXtpG6AsrUJBcWxHBFS2HsEoGTxrMLvKXtf")
	unhealthyPeer, _ := peer.Decode("12D3KooWH3uVF6wv47WnArKHbQBCgdjhCXCe5NkqJFTXNdVkjNwq")

	ctx := context.Background()
	testErr := errors.New("test error")

	// Healthy peer
	for i := 0; i < 10; i++ {
		pm.ExecuteRequest(ctx, healthyPeer, func(ctx context.Context) error {
			return nil
		})
	}

	// Unhealthy peer
	for i := 0; i < 5; i++ {
		pm.ExecuteRequest(ctx, unhealthyPeer, func(ctx context.Context) error {
			return testErr
		})
	}

	overallStatus := pm.GetOverallStatus()
	assert.False(t, overallStatus.IsHealthy()) // Should be unhealthy due to circuit breakers being open
	assert.Equal(t, 2, overallStatus.TotalPeers)
}

// Benchmark tests
func BenchmarkBitswapProtectionManager_ExecuteRequest(b *testing.B) {
	config := DefaultProtectionConfig()
	config.HealthMonitor.CleanupInterval = time.Hour // Disable cleanup
	config.MetricsUpdateInterval = time.Hour         // Disable frequent updates

	pm := NewBitswapProtectionManager(config)
	defer pm.Close()

	peerID, _ := peer.Decode("12D3KooWGzxzKZYveHXtpG6AsrUJBcWxHBFS2HsEoGTxrMLvKXtf")
	ctx := context.Background()

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			pm.ExecuteRequest(ctx, peerID, func(ctx context.Context) error {
				return nil
			})
		}
	})
}

func BenchmarkBitswapProtectionManager_GetPeerStatus(b *testing.B) {
	config := DefaultProtectionConfig()
	config.HealthMonitor.CleanupInterval = time.Hour // Disable cleanup

	pm := NewBitswapProtectionManager(config)
	defer pm.Close()

	peerID, _ := peer.Decode("12D3KooWGzxzKZYveHXtpG6AsrUJBcWxHBFS2HsEoGTxrMLvKXtf")

	// Pre-populate with some data
	ctx := context.Background()
	for i := 0; i < 100; i++ {
		pm.ExecuteRequest(ctx, peerID, func(ctx context.Context) error {
			return nil
		})
	}

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			pm.GetPeerStatus(peerID)
		}
	})
}