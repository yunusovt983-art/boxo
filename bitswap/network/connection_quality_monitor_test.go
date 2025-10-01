package network

import (
	"context"
	"testing"
	"time"

	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/libp2p/go-libp2p/core/peerstore"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestConnectionQualityTracking(t *testing.T) {
	h := createTestHost(t)
	defer h.Close()

	acm := NewAdaptiveConnManager(h, nil)

	peerID, _ := peer.Decode("12D3KooWGzxzKZYveHXtpG6AsrUJBcWxHBFS2HsEoGTxrMLvKXtf")

	// Simulate connection
	acm.mu.Lock()
	acm.connections[peerID] = &ConnectionInfo{
		PeerID:      peerID,
		ConnectedAt: time.Now(),
		Quality:     &ConnectionQuality{},
		State:       StateConnected,
	}
	acm.mu.Unlock()

	// Test initial quality update
	latency := 50 * time.Millisecond
	bandwidth := int64(1024 * 1024) // 1MB/s

	acm.UpdateConnectionQuality(peerID, latency, bandwidth, false)

	quality := acm.GetConnectionQuality(peerID)
	require.NotNil(t, quality)
	assert.Equal(t, latency, quality.Latency)
	assert.Equal(t, bandwidth, quality.Bandwidth)
	assert.Equal(t, latency, quality.AvgLatency) // First measurement
	assert.Equal(t, int64(1), quality.TotalRequests)
	assert.Equal(t, int64(1), quality.SuccessfulRequests)
	assert.Equal(t, int64(0), quality.FailedRequests)
	assert.Equal(t, 0.0, quality.ErrorRate)

	// Test quality update with error
	acm.UpdateConnectionQuality(peerID, 100*time.Millisecond, bandwidth/2, true)

	quality = acm.GetConnectionQuality(peerID)
	assert.Equal(t, int64(2), quality.TotalRequests)
	assert.Equal(t, int64(1), quality.SuccessfulRequests)
	assert.Equal(t, int64(1), quality.FailedRequests)
	assert.Equal(t, 0.5, quality.ErrorRate) // 50% error rate
	assert.Greater(t, quality.AvgLatency, latency) // Should be higher due to averaging
}

func TestConnectionQualityScoring(t *testing.T) {
	h := createTestHost(t)
	defer h.Close()

	acm := NewAdaptiveConnManager(h, nil)

	peerID, _ := peer.Decode("12D3KooWGzxzKZYveHXtpG6AsrUJBcWxHBFS2HsEoGTxrMLvKXtf")

	// Simulate connection
	acm.mu.Lock()
	acm.connections[peerID] = &ConnectionInfo{
		PeerID:      peerID,
		ConnectedAt: time.Now(),
		Quality:     &ConnectionQuality{},
		State:       StateConnected,
	}
	acm.mu.Unlock()

	// Test high quality connection
	acm.UpdateConnectionQuality(peerID, 10*time.Millisecond, 10*1024*1024, false) // 10ms, 10MB/s
	quality := acm.GetConnectionQuality(peerID)
	assert.Greater(t, quality.QualityScore, 0.8) // Should be high quality

	// Test poor quality connection
	acm.UpdateConnectionQuality(peerID, 500*time.Millisecond, 100*1024, true) // 500ms, 100KB/s, with error
	quality = acm.GetConnectionQuality(peerID)
	assert.Less(t, quality.QualityScore, 0.5) // Should be poor quality
}

func TestAlternativeRouteDiscovery(t *testing.T) {
	h1 := createTestHost(t)
	h2 := createTestHost(t)
	h3 := createTestHost(t)
	defer h1.Close()
	defer h2.Close()
	defer h3.Close()

	acm := NewAdaptiveConnManager(h1, nil)

	ctx := context.Background()
	err := acm.Start(ctx)
	require.NoError(t, err)
	defer acm.Stop()

	// Connect to both peers
	h1.Peerstore().AddAddrs(h2.ID(), h2.Addrs(), peerstore.PermanentAddrTTL)
	h1.Peerstore().AddAddrs(h3.ID(), h3.Addrs(), peerstore.PermanentAddrTTL)

	err = h1.Connect(ctx, peer.AddrInfo{ID: h2.ID(), Addrs: h2.Addrs()})
	require.NoError(t, err)
	err = h1.Connect(ctx, peer.AddrInfo{ID: h3.ID(), Addrs: h3.Addrs()})
	require.NoError(t, err)

	// Give time for connections to be established
	time.Sleep(100 * time.Millisecond)

	// Set up quality metrics - h2 has poor quality, h3 has good quality
	acm.UpdateConnectionQuality(h2.ID(), 200*time.Millisecond, 100*1024, true)   // Poor
	acm.UpdateConnectionQuality(h3.ID(), 20*time.Millisecond, 5*1024*1024, false) // Good

	// Find alternatives for h2
	acm.findAlternativeRoutes(h2.ID())

	// Check that h3 is found as an alternative
	quality := acm.GetConnectionQuality(h2.ID())
	require.NotNil(t, quality)
	assert.Contains(t, quality.AlternativeRoutes, h3.ID())
	assert.Equal(t, h3.ID(), quality.PreferredRoute)
}

func TestRouteOptimization(t *testing.T) {
	h1 := createTestHost(t)
	h2 := createTestHost(t)
	h3 := createTestHost(t)
	defer h1.Close()
	defer h2.Close()
	defer h3.Close()

	acm := NewAdaptiveConnManager(h1, nil)

	ctx := context.Background()
	err := acm.Start(ctx)
	require.NoError(t, err)
	defer acm.Stop()

	// Connect to both peers
	h1.Peerstore().AddAddrs(h2.ID(), h2.Addrs(), peerstore.PermanentAddrTTL)
	h1.Peerstore().AddAddrs(h3.ID(), h3.Addrs(), peerstore.PermanentAddrTTL)

	err = h1.Connect(ctx, peer.AddrInfo{ID: h2.ID(), Addrs: h2.Addrs()})
	require.NoError(t, err)
	err = h1.Connect(ctx, peer.AddrInfo{ID: h3.ID(), Addrs: h3.Addrs()})
	require.NoError(t, err)

	// Give time for connections to be established
	time.Sleep(100 * time.Millisecond)

	// Set up quality metrics
	acm.UpdateConnectionQuality(h2.ID(), 200*time.Millisecond, 100*1024, true)   // Poor
	acm.UpdateConnectionQuality(h3.ID(), 20*time.Millisecond, 5*1024*1024, false) // Good

	// Run route optimization
	err = acm.OptimizeRoutes(ctx)
	require.NoError(t, err)

	// Check that poor quality connections have alternatives
	poorPeers := acm.GetPoorQualityConnections(0.5)
	assert.Contains(t, poorPeers, h2.ID())

	goodPeers := acm.GetHighQualityConnections(0.5)
	assert.Contains(t, goodPeers, h3.ID())
}

func TestRouteSwitching(t *testing.T) {
	h := createTestHost(t)
	defer h.Close()

	acm := NewAdaptiveConnManager(h, nil)

	peerID1, _ := peer.Decode("12D3KooWGzxzKZYveHXtpG6AsrUJBcWxHBFS2HsEoGTxrMLvKXtf")
	peerID2, _ := peer.Decode("12D3KooWH3uVF6wv47WnArKHk5p6cvgCJEb74UTmxztmQDc298L3")

	// Set up connections with quality data
	acm.mu.Lock()
	acm.connections[peerID1] = &ConnectionInfo{
		PeerID:  peerID1,
		Quality: &ConnectionQuality{
			PreferredRoute: peerID2,
		},
		State: StateConnected,
	}
	acm.mu.Unlock()

	// Test successful route switching
	newRoute, err := acm.SwitchToAlternativeRoute(peerID1)
	require.NoError(t, err)
	assert.Equal(t, peerID2, newRoute)

	// Check that route change was recorded
	quality := acm.GetConnectionQuality(peerID1)
	assert.Equal(t, int64(1), quality.RouteChanges)

	// Test switching when no alternative is available
	acm.mu.Lock()
	acm.connections[peerID1].Quality.PreferredRoute = ""
	acm.mu.Unlock()

	_, err = acm.SwitchToAlternativeRoute(peerID1)
	assert.Equal(t, ErrNoAlternativeRoute, err)

	// Test switching for non-existent peer
	nonExistentPeer, _ := peer.Decode("12D3KooWNonExistent1234567890123456789012345678901234")
	_, err = acm.SwitchToAlternativeRoute(nonExistentPeer)
	assert.Equal(t, ErrPeerNotFound, err)
}

func TestRequestRecording(t *testing.T) {
	h := createTestHost(t)
	defer h.Close()

	acm := NewAdaptiveConnManager(h, nil)

	peerID, _ := peer.Decode("12D3KooWGzxzKZYveHXtpG6AsrUJBcWxHBFS2HsEoGTxrMLvKXtf")

	// Simulate connection
	acm.mu.Lock()
	acm.connections[peerID] = &ConnectionInfo{
		PeerID:      peerID,
		ConnectedAt: time.Now(),
		Quality:     &ConnectionQuality{},
		State:       StateConnected,
	}
	acm.mu.Unlock()

	// Record successful request
	responseTime := 30 * time.Millisecond
	acm.RecordRequest(peerID, responseTime, true)

	// Check that activity was updated
	acm.mu.RLock()
	connInfo := acm.connections[peerID]
	acm.mu.RUnlock()

	assert.WithinDuration(t, time.Now(), connInfo.LastActivity, time.Second)
	assert.Equal(t, 0, connInfo.ErrorCount)

	// Record failed request
	acm.RecordRequest(peerID, 200*time.Millisecond, false)

	acm.mu.RLock()
	connInfo = acm.connections[peerID]
	acm.mu.RUnlock()

	assert.Greater(t, connInfo.ErrorCount, 0)
	assert.WithinDuration(t, time.Now(), connInfo.LastError, time.Second)
}

func TestUnresponsiveMarking(t *testing.T) {
	h := createTestHost(t)
	defer h.Close()

	acm := NewAdaptiveConnManager(h, nil)

	peerID, _ := peer.Decode("12D3KooWGzxzKZYveHXtpG6AsrUJBcWxHBFS2HsEoGTxrMLvKXtf")

	// Simulate connection
	acm.mu.Lock()
	acm.connections[peerID] = &ConnectionInfo{
		PeerID:      peerID,
		ConnectedAt: time.Now(),
		Quality:     &ConnectionQuality{},
		State:       StateConnected,
	}
	acm.mu.Unlock()

	// Test initial state
	state := acm.GetConnectionState(peerID)
	assert.Equal(t, StateConnected, state)

	// Mark as unresponsive
	acm.MarkUnresponsive(peerID)

	state = acm.GetConnectionState(peerID)
	assert.Equal(t, StateUnresponsive, state)

	// Check that error time was recorded
	acm.mu.RLock()
	connInfo := acm.connections[peerID]
	acm.mu.RUnlock()
	assert.WithinDuration(t, time.Now(), connInfo.LastError, time.Second)

	// Mark as responsive again
	acm.MarkResponsive(peerID)

	state = acm.GetConnectionState(peerID)
	assert.Equal(t, StateConnected, state)
}

func TestConnectionStateForNonExistentPeer(t *testing.T) {
	h := createTestHost(t)
	defer h.Close()

	acm := NewAdaptiveConnManager(h, nil)

	nonExistentPeer, _ := peer.Decode("12D3KooWNonExistent1234567890123456789012345678901234")

	// Test getting state for non-existent peer
	state := acm.GetConnectionState(nonExistentPeer)
	assert.Equal(t, StateDisconnected, state)

	// Test getting quality for non-existent peer
	quality := acm.GetConnectionQuality(nonExistentPeer)
	assert.Nil(t, quality)
}

func TestLatencyMeasurement(t *testing.T) {
	h1 := createTestHost(t)
	h2 := createTestHost(t)
	defer h1.Close()
	defer h2.Close()

	acm := NewAdaptiveConnManager(h1, nil)

	ctx := context.Background()

	// Connect peers
	h1.Peerstore().AddAddrs(h2.ID(), h2.Addrs(), peerstore.PermanentAddrTTL)
	err := h1.Connect(ctx, peer.AddrInfo{ID: h2.ID(), Addrs: h2.Addrs()})
	require.NoError(t, err)

	// Test latency measurement
	latency, err := acm.MeasureConnectionLatency(ctx, h2.ID())
	if err != nil {
		// If ping service is not available, we should get a fallback measurement
		assert.NotEqual(t, ErrPeerNotConnected, err)
	} else {
		assert.Greater(t, latency, time.Duration(0))
		assert.Less(t, latency, 5*time.Second) // Should be reasonable
	}

	// Test latency measurement for non-connected peer
	nonConnectedPeer, _ := peer.Decode("12D3KooWNonConnected123456789012345678901234567890")
	_, err = acm.MeasureConnectionLatency(ctx, nonConnectedPeer)
	assert.Error(t, err)
}

func TestBandwidthEstimation(t *testing.T) {
	h := createTestHost(t)
	defer h.Close()

	acm := NewAdaptiveConnManager(h, nil)

	peerID, _ := peer.Decode("12D3KooWGzxzKZYveHXtpG6AsrUJBcWxHBFS2HsEoGTxrMLvKXtf")

	// Test bandwidth estimation for non-existent peer
	bandwidth := acm.EstimateBandwidth(peerID)
	assert.Equal(t, int64(0), bandwidth)

	// Set up connection with quality data
	acm.mu.Lock()
	acm.connections[peerID] = &ConnectionInfo{
		PeerID: peerID,
		Quality: &ConnectionQuality{
			AvgBandwidth: 5 * 1024 * 1024, // 5MB/s
		},
		State: StateConnected,
	}
	acm.mu.Unlock()

	// Test bandwidth estimation
	bandwidth = acm.EstimateBandwidth(peerID)
	assert.Equal(t, int64(5*1024*1024), bandwidth)
}

func TestQualityMonitoringIntegration(t *testing.T) {
	h1 := createTestHost(t)
	h2 := createTestHost(t)
	defer h1.Close()
	defer h2.Close()

	config := &AdaptiveConnConfig{
		BaseHighWater:      1000,
		BaseLowWater:       500,
		BaseGracePeriod:    30 * time.Second,
		AdaptationInterval: 100 * time.Millisecond, // Fast for testing
	}

	acm := NewAdaptiveConnManager(h1, config)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	err := acm.Start(ctx)
	require.NoError(t, err)
	defer acm.Stop()

	// Connect to peer
	h1.Peerstore().AddAddrs(h2.ID(), h2.Addrs(), peerstore.PermanentAddrTTL)
	err = h1.Connect(ctx, peer.AddrInfo{ID: h2.ID(), Addrs: h2.Addrs()})
	require.NoError(t, err)

	// Give time for connection to be established and monitored
	time.Sleep(200 * time.Millisecond)

	// Check that connection is being tracked
	state := acm.GetConnectionState(h2.ID())
	assert.Equal(t, StateConnected, state)

	// The quality monitoring should have started measuring
	// We can't guarantee specific values, but we can check the structure exists
	quality := acm.GetConnectionQuality(h2.ID())
	assert.NotNil(t, quality)
}

// Benchmark tests for connection quality monitoring


func BenchmarkFindAlternativeRoutes(b *testing.B) {
	h := createTestHost(b)
	defer h.Close()

	acm := NewAdaptiveConnManager(h, nil)

	// Create multiple peer connections
	for i := 0; i < 100; i++ {
		peerID, _ := peer.Decode("12D3KooWGzxzKZYveHXtpG6AsrUJBcWxHBFS2HsEoGTxrMLvKXtf")
		acm.mu.Lock()
		acm.connections[peerID] = &ConnectionInfo{
			PeerID: peerID,
			Quality: &ConnectionQuality{
				QualityScore: float64(i) / 100.0, // Varying quality scores
			},
			State: StateConnected,
		}
		acm.mu.Unlock()
	}

	targetPeer, _ := peer.Decode("12D3KooWTarget123456789012345678901234567890123456")
	acm.mu.Lock()
	acm.connections[targetPeer] = &ConnectionInfo{
		PeerID: targetPeer,
		Quality: &ConnectionQuality{
			QualityScore: 0.1, // Poor quality
		},
		State: StateConnected,
	}
	acm.mu.Unlock()

	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		acm.findAlternativeRoutes(targetPeer)
	}
}

