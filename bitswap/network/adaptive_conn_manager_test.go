package network

import (
	"context"
	"testing"
	"time"

	"github.com/libp2p/go-libp2p"
	"github.com/libp2p/go-libp2p/core/host"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/libp2p/go-libp2p/core/peerstore"
	"github.com/libp2p/go-libp2p/p2p/net/connmgr"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewAdaptiveConnManager(t *testing.T) {
	h := createTestHost(t)
	defer h.Close()
	
	config := &AdaptiveConnConfig{
		BaseHighWater:      1000,
		BaseLowWater:       500,
		BaseGracePeriod:    30 * time.Second,
		MaxHighWater:       2000,
		MinLowWater:        100,
		HighLoadThreshold:  0.8,
		LowLoadThreshold:   0.3,
		AdaptationInterval: 10 * time.Second,
	}
	
	acm := NewAdaptiveConnManager(h, config)
	
	assert.NotNil(t, acm)
	assert.Equal(t, config.BaseHighWater, acm.currentHighWater)
	assert.Equal(t, config.BaseLowWater, acm.currentLowWater)
	assert.Equal(t, config.BaseGracePeriod, acm.currentGracePeriod)
	assert.NotNil(t, acm.loadMonitor)
	assert.NotNil(t, acm.metrics)
}

func TestDefaultAdaptiveConnConfig(t *testing.T) {
	config := DefaultAdaptiveConnConfig()
	
	assert.Equal(t, 2000, config.BaseHighWater)
	assert.Equal(t, 1000, config.BaseLowWater)
	assert.Equal(t, 30*time.Second, config.BaseGracePeriod)
	assert.Equal(t, 5000, config.MaxHighWater)
	assert.Equal(t, 500, config.MinLowWater)
	assert.Equal(t, 0.8, config.HighLoadThreshold)
	assert.Equal(t, 0.3, config.LowLoadThreshold)
}

func TestSetDynamicLimits(t *testing.T) {
	h := createTestHost(t)
	defer h.Close()
	
	acm := NewAdaptiveConnManager(h, nil)
	
	newHigh := 1500
	newLow := 750
	newGrace := 45 * time.Second
	
	acm.SetDynamicLimits(newHigh, newLow, newGrace)
	
	assert.Equal(t, newHigh, acm.currentHighWater)
	assert.Equal(t, newLow, acm.currentLowWater)
	assert.Equal(t, newGrace, acm.currentGracePeriod)
}

func TestClusterPeerManagement(t *testing.T) {
	h := createTestHost(t)
	defer h.Close()
	
	// Create test peer IDs
	peerID1, _ := peer.Decode("12D3KooWGzxzKZYveHXtpG6AsrUJBcWxHBFS2HsEoGTxrMLvKXtf")
	peerID2, _ := peer.Decode("12D3KooWH3uVF6wv47WnArKHk5p6cvgCJEb74UTmxztmQDc298L3")
	
	config := &AdaptiveConnConfig{
		ClusterPeers: []peer.ID{peerID1},
	}
	
	acm := NewAdaptiveConnManager(h, config)
	
	// Test initial cluster peer
	assert.True(t, acm.IsClusterPeer(peerID1))
	assert.False(t, acm.IsClusterPeer(peerID2))
	
	// Test adding cluster peer
	acm.AddClusterPeer(peerID2)
	assert.True(t, acm.IsClusterPeer(peerID2))
	
	// Test removing cluster peer
	acm.RemoveClusterPeer(peerID1)
	assert.False(t, acm.IsClusterPeer(peerID1))
}

func TestConnectionTracking(t *testing.T) {
	h1 := createTestHost(t)
	h2 := createTestHost(t)
	defer h1.Close()
	defer h2.Close()
	
	acm := NewAdaptiveConnManager(h1, nil)
	
	ctx := context.Background()
	err := acm.Start(ctx)
	require.NoError(t, err)
	defer acm.Stop()
	
	// Connect to peer
	h1.Peerstore().AddAddrs(h2.ID(), h2.Addrs(), peerstore.PermanentAddrTTL)
	err = h1.Connect(ctx, peer.AddrInfo{ID: h2.ID(), Addrs: h2.Addrs()})
	require.NoError(t, err)
	
	// Give some time for the connection event to be processed
	time.Sleep(100 * time.Millisecond)
	
	// Check connection is tracked
	state := acm.GetConnectionState(h2.ID())
	assert.Equal(t, StateConnected, state)
	
	// Check metrics
	metrics := acm.GetMetrics()
	assert.Equal(t, int64(1), metrics.TotalConnections)
}

func TestLoadMonitoring(t *testing.T) {
	lm := NewLoadMonitor()
	
	// Test initial state
	load := lm.GetCurrentLoad()
	assert.Equal(t, 0.0, load)
	
	// Record some samples
	sample1 := LoadSample{
		Timestamp:         time.Now(),
		ActiveConnections: 500,
		RequestsPerSecond: 100,
		AverageLatency:    50 * time.Millisecond,
		ErrorRate:         0.01,
	}
	lm.RecordSample(sample1)
	
	load = lm.GetCurrentLoad()
	assert.Greater(t, load, 0.0)
	assert.Less(t, load, 1.0)
	
	// Test high load scenario
	sample2 := LoadSample{
		Timestamp:         time.Now(),
		ActiveConnections: 1800,
		RequestsPerSecond: 900,
		AverageLatency:    90 * time.Millisecond,
		ErrorRate:         0.04,
	}
	lm.RecordSample(sample2)
	
	highLoad := lm.GetCurrentLoad()
	assert.Greater(t, highLoad, load)
}

func TestAdaptationToLoad(t *testing.T) {
	h := createTestHost(t)
	defer h.Close()
	
	config := &AdaptiveConnConfig{
		BaseHighWater:      1000,
		BaseLowWater:       500,
		BaseGracePeriod:    30 * time.Second,
		MaxHighWater:       2000,
		MinLowWater:        200,
		MaxGracePeriod:     60 * time.Second,
		MinGracePeriod:     10 * time.Second,
		HighLoadThreshold:  0.7,
		LowLoadThreshold:   0.3,
		AdaptationInterval: 100 * time.Millisecond, // Fast for testing
	}
	
	acm := NewAdaptiveConnManager(h, config)
	
	ctx := context.Background()
	err := acm.Start(ctx)
	require.NoError(t, err)
	defer acm.Stop()
	
	// Simulate high load
	highLoadSample := LoadSample{
		Timestamp:         time.Now(),
		ActiveConnections: 1500,
		RequestsPerSecond: 800,
		AverageLatency:    80 * time.Millisecond,
		ErrorRate:         0.03,
	}
	acm.loadMonitor.RecordSample(highLoadSample)
	
	// Wait for adaptation
	time.Sleep(200 * time.Millisecond)
	
	// Check that limits were increased
	assert.Greater(t, acm.currentHighWater, config.BaseHighWater)
	assert.Less(t, acm.currentGracePeriod, config.BaseGracePeriod)
	
	// Simulate low load
	lowLoadSample := LoadSample{
		Timestamp:         time.Now(),
		ActiveConnections: 100,
		RequestsPerSecond: 50,
		AverageLatency:    20 * time.Millisecond,
		ErrorRate:         0.001,
	}
	acm.loadMonitor.RecordSample(lowLoadSample)
	
	// Wait for adaptation
	time.Sleep(200 * time.Millisecond)
	
	// Check that limits were decreased or returned to base
	assert.LessOrEqual(t, acm.currentHighWater, config.BaseHighWater+100) // Allow some tolerance
}






func TestMetrics(t *testing.T) {
	h := createTestHost(t)
	defer h.Close()
	
	acm := NewAdaptiveConnManager(h, nil)
	
	peerID1, _ := peer.Decode("12D3KooWGzxzKZYveHXtpG6AsrUJBcWxHBFS2HsEoGTxrMLvKXtf")
	peerID2, _ := peer.Decode("12D3KooWH3uVF6wv47WnArKHk5p6cvgCJEb74UTmxztmQDc298L3")
	
	// Add cluster peer
	acm.AddClusterPeer(peerID1)
	
	// Simulate connections
	acm.mu.Lock()
	acm.connections[peerID1] = &ConnectionInfo{
		PeerID:    peerID1,
		IsCluster: true,
		State:     StateConnected,
	}
	acm.connections[peerID2] = &ConnectionInfo{
		PeerID:    peerID2,
		IsCluster: false,
		State:     StateConnected,
	}
	acm.mu.Unlock()
	
	metrics := acm.GetMetrics()
	assert.Equal(t, int64(2), metrics.TotalConnections)
	assert.Equal(t, int64(1), metrics.ClusterConnections)
}

// Helper function to create a test host
func createTestHost(t testing.TB) host.Host {
	cm, err := connmgr.NewConnManager(10, 100, connmgr.WithGracePeriod(time.Minute))
	require.NoError(t, err)
	
	h, err := libp2p.New(
		libp2p.ConnectionManager(cm),
		libp2p.ListenAddrStrings("/ip4/127.0.0.1/tcp/0"),
	)
	require.NoError(t, err)
	
	return h
}

// Benchmark tests
func BenchmarkAdaptToLoad(b *testing.B) {
	h := createTestHost(b)
	defer h.Close()
	
	acm := NewAdaptiveConnManager(h, nil)
	
	// Simulate some load samples
	for i := 0; i < 10; i++ {
		sample := LoadSample{
			Timestamp:         time.Now(),
			ActiveConnections: 500 + i*100,
			RequestsPerSecond: float64(100 + i*50),
			AverageLatency:    time.Duration(50+i*10) * time.Millisecond,
			ErrorRate:         float64(i) * 0.01,
		}
		acm.loadMonitor.RecordSample(sample)
	}
	
	b.ResetTimer()
	
	for i := 0; i < b.N; i++ {
		acm.adaptToLoad()
	}
}

func BenchmarkUpdateConnectionQuality(b *testing.B) {
	h := createTestHost(b)
	defer h.Close()
	
	acm := NewAdaptiveConnManager(h, nil)
	
	peerID, _ := peer.Decode("12D3KooWGzxzKZYveHXtpG6AsrUJBcWxHBFS2HsEoGTxrMLvKXtf")
	
	// Setup connection
	acm.mu.Lock()
	acm.connections[peerID] = &ConnectionInfo{
		PeerID:      peerID,
		ConnectedAt: time.Now(),
		Quality:     &ConnectionQuality{},
		State:       StateConnected,
	}
	acm.mu.Unlock()
	
	b.ResetTimer()
	
	for i := 0; i < b.N; i++ {
		latency := time.Duration(50+i%50) * time.Millisecond
		bandwidth := int64(1024*1024 + i%1024)
		acm.UpdateConnectionQuality(peerID, latency, bandwidth, i%10 == 0)
	}
}