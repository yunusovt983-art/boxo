package circuitbreaker

import (
	"testing"
	"time"

	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestHealthMonitor_RecordRequest(t *testing.T) {
	config := DefaultHealthMonitorConfig()
	config.CleanupInterval = time.Hour // Disable cleanup for test
	hm := NewHealthMonitor(config)
	defer hm.Close()

	peerID, err := peer.Decode("12D3KooWGzxzKZYveHXtpG6AsrUJBcWxHBFS2HsEoGTxrMLvKXtf")
	require.NoError(t, err)

	// Record enough successful requests to get meaningful health status
	for i := 0; i < 5; i++ {
		hm.RecordRequest(peerID, 50*time.Millisecond, nil)
	}

	health, exists := hm.GetHealth(peerID)
	assert.True(t, exists)
	assert.Equal(t, int64(5), health.TotalRequests)
	assert.Equal(t, int64(0), health.ErrorCount)
	assert.Equal(t, float64(1.0), health.SuccessRate)
	assert.Equal(t, int64(0), health.ConsecutiveErrors)
	assert.True(t, health.IsHealthy)
}

func TestHealthMonitor_RecordFailedRequest(t *testing.T) {
	config := DefaultHealthMonitorConfig()
	config.CleanupInterval = time.Hour // Disable cleanup for test
	hm := NewHealthMonitor(config)
	defer hm.Close()

	peerID, err := peer.Decode("12D3KooWGzxzKZYveHXtpG6AsrUJBcWxHBFS2HsEoGTxrMLvKXtf")
	require.NoError(t, err)

	// Record failed request
	testErr := assert.AnError
	hm.RecordRequest(peerID, 200*time.Millisecond, testErr)

	health, exists := hm.GetHealth(peerID)
	assert.True(t, exists)
	assert.Equal(t, int64(1), health.TotalRequests)
	assert.Equal(t, int64(1), health.ErrorCount)
	assert.Equal(t, float64(0.0), health.SuccessRate)
	assert.Equal(t, int64(1), health.ConsecutiveErrors)
	assert.Equal(t, testErr, health.LastError)
}

func TestHealthMonitor_HealthStatus(t *testing.T) {
	config := DefaultHealthMonitorConfig()
	config.HealthyThreshold = 0.9
	config.DegradedThreshold = 0.7
	config.MaxConsecutiveErrors = 3
	config.CleanupInterval = time.Hour // Disable cleanup for test
	hm := NewHealthMonitor(config)
	defer hm.Close()

	peerID, err := peer.Decode("12D3KooWGzxzKZYveHXtpG6AsrUJBcWxHBFS2HsEoGTxrMLvKXtf")
	require.NoError(t, err)

	// Record enough successful requests for healthy status
	for i := 0; i < 10; i++ {
		hm.RecordRequest(peerID, 50*time.Millisecond, nil)
	}

	status := hm.GetHealthStatus(peerID)
	assert.Equal(t, HealthStatusHealthy, status)
	assert.True(t, hm.IsHealthy(peerID))

	// Add some failures to make it degraded (need to balance success rate)
	for i := 0; i < 2; i++ {
		hm.RecordRequest(peerID, 100*time.Millisecond, assert.AnError)
	}

	status = hm.GetHealthStatus(peerID)
	// With 10 success + 2 failures = 83% success rate, should be degraded
	assert.Equal(t, HealthStatusDegraded, status)
	assert.False(t, hm.IsHealthy(peerID))

	// Add more failures to make it unhealthy
	for i := 0; i < 8; i++ {
		hm.RecordRequest(peerID, 150*time.Millisecond, assert.AnError)
	}

	status = hm.GetHealthStatus(peerID)
	// Now we have 10 success + 10 failures = 50% success rate, should be unhealthy
	assert.Equal(t, HealthStatusUnhealthy, status)
	assert.False(t, hm.IsHealthy(peerID))
}

func TestHealthMonitor_ConsecutiveErrors(t *testing.T) {
	config := DefaultHealthMonitorConfig()
	config.MaxConsecutiveErrors = 3
	config.CleanupInterval = time.Hour // Disable cleanup for test
	hm := NewHealthMonitor(config)
	defer hm.Close()

	peerID, err := peer.Decode("12D3KooWGzxzKZYveHXtpG6AsrUJBcWxHBFS2HsEoGTxrMLvKXtf")
	require.NoError(t, err)

	// Record some successful requests first
	for i := 0; i < 5; i++ {
		hm.RecordRequest(peerID, 50*time.Millisecond, nil)
	}

	// Record consecutive errors
	for i := 0; i < 3; i++ {
		hm.RecordRequest(peerID, 100*time.Millisecond, assert.AnError)
	}

	status := hm.GetHealthStatus(peerID)
	assert.Equal(t, HealthStatusUnhealthy, status)

	// One successful request should reset consecutive errors
	hm.RecordRequest(peerID, 50*time.Millisecond, nil)

	health, _ := hm.GetHealth(peerID)
	assert.Equal(t, int64(0), health.ConsecutiveErrors)
}

func TestHealthMonitor_ResponseTimeThreshold(t *testing.T) {
	config := DefaultHealthMonitorConfig()
	config.MaxResponseTime = 100 * time.Millisecond
	config.CleanupInterval = time.Hour // Disable cleanup for test
	hm := NewHealthMonitor(config)
	defer hm.Close()

	peerID, err := peer.Decode("12D3KooWGzxzKZYveHXtpG6AsrUJBcWxHBFS2HsEoGTxrMLvKXtf")
	require.NoError(t, err)

	// Record requests with high response time
	for i := 0; i < 5; i++ {
		hm.RecordRequest(peerID, 200*time.Millisecond, nil)
	}

	status := hm.GetHealthStatus(peerID)
	assert.Equal(t, HealthStatusDegraded, status)
}

func TestHealthMonitor_GetAllHealth(t *testing.T) {
	config := DefaultHealthMonitorConfig()
	config.CleanupInterval = time.Hour // Disable cleanup for test
	hm := NewHealthMonitor(config)
	defer hm.Close()

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

		// Record some requests for each peer
		for j := 0; j < 5; j++ {
			hm.RecordRequest(peerID, 50*time.Millisecond, nil)
		}
	}

	allHealth := hm.GetAllHealth()
	assert.Len(t, allHealth, 3)

	for _, peerID := range peers {
		health, exists := allHealth[peerID]
		assert.True(t, exists)
		assert.Equal(t, int64(5), health.TotalRequests)
		assert.Equal(t, float64(1.0), health.SuccessRate)
	}
}

func TestHealthMonitor_HealthChangeCallback(t *testing.T) {
	config := DefaultHealthMonitorConfig()
	config.MaxConsecutiveErrors = 3
	config.HealthyThreshold = 0.9
	config.DegradedThreshold = 0.7
	config.CleanupInterval = time.Hour // Disable cleanup for test
	hm := NewHealthMonitor(config)
	defer hm.Close()

	peerID, err := peer.Decode("12D3KooWGzxzKZYveHXtpG6AsrUJBcWxHBFS2HsEoGTxrMLvKXtf")
	require.NoError(t, err)

	var statusChanges []HealthStatus
	hm.SetHealthChangeCallback(func(p peer.ID, from, to HealthStatus) {
		assert.Equal(t, peerID, p)
		statusChanges = append(statusChanges, to)
	})

	// Start with enough successful requests to establish healthy status
	for i := 0; i < 10; i++ {
		hm.RecordRequest(peerID, 50*time.Millisecond, nil)
	}

	// Verify we're healthy
	status := hm.GetHealthStatus(peerID)
	assert.Equal(t, HealthStatusHealthy, status)

	// Add enough consecutive errors to trigger unhealthy status
	for i := 0; i < 4; i++ {
		hm.RecordRequest(peerID, 100*time.Millisecond, assert.AnError)
	}

	// Verify status changed to unhealthy
	status = hm.GetHealthStatus(peerID)
	assert.Equal(t, HealthStatusUnhealthy, status)

	// Should have recorded status changes
	assert.True(t, len(statusChanges) > 0, "Expected at least one status change")
	assert.Contains(t, statusChanges, HealthStatusHealthy)
	assert.Contains(t, statusChanges, HealthStatusUnhealthy)
}

func TestHealthMonitor_GetHealthStats(t *testing.T) {
	config := DefaultHealthMonitorConfig()
	config.HealthyThreshold = 0.9
	config.DegradedThreshold = 0.7
	config.MaxConsecutiveErrors = 10 // Set high to avoid consecutive error triggering
	config.CleanupInterval = time.Hour // Disable cleanup for test
	hm := NewHealthMonitor(config)
	defer hm.Close()

	// Create peers with different health statuses
	healthyPeer, _ := peer.Decode("12D3KooWGzxzKZYveHXtpG6AsrUJBcWxHBFS2HsEoGTxrMLvKXtf")
	degradedPeer, _ := peer.Decode("12D3KooWH3uVF6wv47WnArKHbQBCgdjhCXCe5NkqJFTXNdVkjNwq")
	unhealthyPeer, _ := peer.Decode("12D3KooWBhMbLvQGgqByteXqKxKvJkXQhQvKZyYqBzWBKvNqKqhF")

	// Healthy peer: 10 successful requests (100% success rate)
	for i := 0; i < 10; i++ {
		hm.RecordRequest(healthyPeer, 50*time.Millisecond, nil)
	}

	// Degraded peer: 8 successful, 2 failed (80% success rate - exactly at threshold)
	for i := 0; i < 8; i++ {
		hm.RecordRequest(degradedPeer, 50*time.Millisecond, nil)
	}
	for i := 0; i < 2; i++ {
		hm.RecordRequest(degradedPeer, 100*time.Millisecond, assert.AnError)
	}

	// Unhealthy peer: 3 successful, 7 failed (30% success rate)
	for i := 0; i < 3; i++ {
		hm.RecordRequest(unhealthyPeer, 50*time.Millisecond, nil)
	}
	for i := 0; i < 7; i++ {
		hm.RecordRequest(unhealthyPeer, 200*time.Millisecond, assert.AnError)
	}

	// Verify individual statuses first
	assert.Equal(t, HealthStatusHealthy, hm.GetHealthStatus(healthyPeer))
	assert.Equal(t, HealthStatusDegraded, hm.GetHealthStatus(degradedPeer))
	assert.Equal(t, HealthStatusUnhealthy, hm.GetHealthStatus(unhealthyPeer))

	stats := hm.GetHealthStats()
	assert.Equal(t, 3, stats.TotalPeers)
	assert.Equal(t, 1, stats.HealthyPeers)
	assert.Equal(t, 1, stats.DegradedPeers)
	assert.Equal(t, 1, stats.UnhealthyPeers)
	assert.Equal(t, 0, stats.UnknownPeers)

	// Check average success rate
	expectedAvgSuccessRate := (1.0 + 0.8 + 0.3) / 3.0
	assert.InDelta(t, expectedAvgSuccessRate, stats.AverageSuccessRate, 0.01)
}

func TestHealthMonitor_CleanupStaleData(t *testing.T) {
	config := DefaultHealthMonitorConfig()
	config.CleanupInterval = 10 * time.Millisecond
	config.MaxAge = 50 * time.Millisecond
	hm := NewHealthMonitor(config)
	defer hm.Close()

	peerID, err := peer.Decode("12D3KooWGzxzKZYveHXtpG6AsrUJBcWxHBFS2HsEoGTxrMLvKXtf")
	require.NoError(t, err)

	// Record a request
	hm.RecordRequest(peerID, 50*time.Millisecond, nil)

	// Verify data exists
	_, exists := hm.GetHealth(peerID)
	assert.True(t, exists)

	// Wait for cleanup
	time.Sleep(100 * time.Millisecond)

	// Data should be cleaned up
	_, exists = hm.GetHealth(peerID)
	assert.False(t, exists)
}

func TestHealthStatus_String(t *testing.T) {
	assert.Equal(t, "HEALTHY", HealthStatusHealthy.String())
	assert.Equal(t, "DEGRADED", HealthStatusDegraded.String())
	assert.Equal(t, "UNHEALTHY", HealthStatusUnhealthy.String())
	assert.Equal(t, "UNKNOWN", HealthStatusUnknown.String())
}

func TestDefaultHealthMonitorConfig(t *testing.T) {
	config := DefaultHealthMonitorConfig()

	assert.Equal(t, 30*time.Second, config.CheckInterval)
	assert.Equal(t, 0.95, config.HealthyThreshold)
	assert.Equal(t, 0.80, config.DegradedThreshold)
	assert.Equal(t, 5*time.Second, config.MaxResponseTime)
	assert.Equal(t, int64(5), config.MaxConsecutiveErrors)
	assert.Equal(t, 10*time.Second, config.HealthCheckTimeout)
	assert.Equal(t, 5*time.Minute, config.CleanupInterval)
	assert.Equal(t, 10*time.Minute, config.MaxAge)
}

// Benchmark tests
func BenchmarkHealthMonitor_RecordRequest(b *testing.B) {
	config := DefaultHealthMonitorConfig()
	config.CleanupInterval = time.Hour // Disable cleanup for benchmark
	hm := NewHealthMonitor(config)
	defer hm.Close()

	peerID, _ := peer.Decode("12D3KooWGzxzKZYveHXtpG6AsrUJBcWxHBFS2HsEoGTxrMLvKXtf")

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			hm.RecordRequest(peerID, 50*time.Millisecond, nil)
		}
	})
}

func BenchmarkHealthMonitor_GetHealthStatus(b *testing.B) {
	config := DefaultHealthMonitorConfig()
	config.CleanupInterval = time.Hour // Disable cleanup for benchmark
	hm := NewHealthMonitor(config)
	defer hm.Close()

	peerID, _ := peer.Decode("12D3KooWGzxzKZYveHXtpG6AsrUJBcWxHBFS2HsEoGTxrMLvKXtf")

	// Pre-populate with some data
	for i := 0; i < 100; i++ {
		hm.RecordRequest(peerID, 50*time.Millisecond, nil)
	}

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			hm.GetHealthStatus(peerID)
		}
	})
}