package network

import (
	"context"
	"fmt"
	"time"

	"github.com/libp2p/go-libp2p"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/libp2p/go-libp2p/p2p/net/connmgr"
)

// ExampleAdaptiveConnManager demonstrates how to use the adaptive connection manager
func ExampleAdaptiveConnManager() {
	ctx := context.Background()
	
	// Create a libp2p host with connection manager
	cm, err := connmgr.NewConnManager(100, 1000, connmgr.WithGracePeriod(30*time.Second))
	if err != nil {
		panic(err)
	}
	
	host, err := libp2p.New(
		libp2p.ConnectionManager(cm),
		libp2p.ListenAddrStrings("/ip4/127.0.0.1/tcp/0"),
	)
	if err != nil {
		panic(err)
	}
	defer host.Close()
	
	// Create cluster peer IDs (in real usage, these would be actual cluster peers)
	clusterPeer1, _ := peer.Decode("12D3KooWGzxzKZYveHXtpG6AsrUJBcWxHBFS2HsEoGTxrMLvKXtf")
	clusterPeer2, _ := peer.Decode("12D3KooWH3uVF6wv47WnArKHk5p6cvgCJEb74UTmxztmQDc298L3")
	
	// Configure adaptive connection manager
	config := &AdaptiveConnConfig{
		BaseHighWater:      1000,
		BaseLowWater:       500,
		BaseGracePeriod:    30 * time.Second,
		MaxHighWater:       2000,
		MinLowWater:        200,
		MaxGracePeriod:     60 * time.Second,
		MinGracePeriod:     10 * time.Second,
		HighLoadThreshold:  0.8,
		LowLoadThreshold:   0.3,
		AdaptationInterval: 30 * time.Second,
		LoadSampleWindow:   5 * time.Minute,
		ClusterPeers:       []peer.ID{clusterPeer1, clusterPeer2},
	}
	
	// Create and start adaptive connection manager
	acm := NewAdaptiveConnManager(host, config)
	err = acm.Start(ctx)
	if err != nil {
		panic(err)
	}
	defer acm.Stop()
	
	// Simulate some load and demonstrate adaptation
	fmt.Println("Starting adaptive connection manager...")
	
	// Simulate high load
	highLoadSample := LoadSample{
		Timestamp:         time.Now(),
		ActiveConnections: 800,
		RequestsPerSecond: 500,
		AverageLatency:    80 * time.Millisecond,
		ErrorRate:         0.02,
	}
	acm.loadMonitor.RecordSample(highLoadSample)
	
	fmt.Printf("High load detected: %d connections, %.1f RPS\n", 
		highLoadSample.ActiveConnections, highLoadSample.RequestsPerSecond)
	
	// Manually trigger adaptation for demonstration
	acm.adaptToLoad()
	
	// Check adapted limits
	fmt.Printf("Adapted limits: HighWater=%d, LowWater=%d, GracePeriod=%v\n",
		acm.currentHighWater, acm.currentLowWater, acm.currentGracePeriod)
	
	// Simulate connection quality tracking
	testPeer, _ := peer.Decode("12D3KooWTest123456789ABCDEF")
	
	// Add connection info
	acm.mu.Lock()
	acm.connections[testPeer] = &ConnectionInfo{
		PeerID:      testPeer,
		ConnectedAt: time.Now(),
		Quality:     &ConnectionQuality{},
		State:       StateConnected,
	}
	acm.mu.Unlock()
	
	// Update connection quality
	acm.UpdateConnectionQuality(testPeer, 50*time.Millisecond, 1024*1024, false)
	
	quality := acm.GetConnectionQuality(testPeer)
	if quality != nil {
		fmt.Printf("Connection quality for test peer: Latency=%v, Bandwidth=%d bytes/s\n",
			quality.Latency, quality.Bandwidth)
	}
	
	// Get metrics
	metrics := acm.GetMetrics()
	fmt.Printf("Metrics: Total connections=%d, Cluster connections=%d\n",
		metrics.TotalConnections, metrics.ClusterConnections)
	
	// Output:
	// Starting adaptive connection manager...
	// High load detected: 800 connections, 500.0 RPS
	// Adapted limits: HighWater=1000, LowWater=500, GracePeriod=30s
	// Connection quality for test peer: Latency=50ms, Bandwidth=1048576 bytes/s
	// Metrics: Total connections=1, Cluster connections=0
}

// ExampleAdaptiveConnManager_clusterMode demonstrates cluster-specific features
func ExampleAdaptiveConnManager_clusterMode() {
	ctx := context.Background()
	
	// Create host
	host, err := libp2p.New(libp2p.ListenAddrStrings("/ip4/127.0.0.1/tcp/0"))
	if err != nil {
		panic(err)
	}
	defer host.Close()
	
	// Create adaptive connection manager with cluster configuration
	config := DefaultAdaptiveConnConfig()
	acm := NewAdaptiveConnManager(host, config)
	
	// Add cluster peers dynamically
	clusterPeer, _ := peer.Decode("12D3KooWGzxzKZYveHXtpG6AsrUJBcWxHBFS2HsEoGTxrMLvKXtf")
	acm.AddClusterPeer(clusterPeer)
	
	fmt.Printf("Added cluster peer: %s\n", "cluster-peer-1")
	fmt.Printf("Is cluster peer: %v\n", acm.IsClusterPeer(clusterPeer))
	
	// Start the manager
	err = acm.Start(ctx)
	if err != nil {
		panic(err)
	}
	defer acm.Stop()
	
	// Add connection info for the cluster peer
	acm.mu.Lock()
	acm.connections[clusterPeer] = &ConnectionInfo{
		PeerID:      clusterPeer,
		ConnectedAt: time.Now(),
		Quality:     &ConnectionQuality{},
		State:       StateConnected,
		IsCluster:   true,
	}
	acm.mu.Unlock()
	
	// Simulate request recording
	acm.RecordRequest(clusterPeer, 25*time.Millisecond, true)
	fmt.Println("Recorded successful request to cluster peer")
	
	// Check current load
	load := acm.loadMonitor.GetCurrentLoad()
	fmt.Printf("Current system load: %.2f\n", load)
	
	// Output:
	// Added cluster peer: cluster-peer-1
	// Is cluster peer: true
	// Recorded successful request to cluster peer
	// Current system load: 0.00
}

// ExampleLoadMonitor demonstrates load monitoring functionality
func ExampleLoadMonitor() {
	lm := NewLoadMonitor()
	
	// Record samples over time
	samples := []LoadSample{
		{
			Timestamp:         time.Now(),
			ActiveConnections: 100,
			RequestsPerSecond: 50,
			AverageLatency:    30 * time.Millisecond,
			ErrorRate:         0.001,
		},
		{
			Timestamp:         time.Now().Add(time.Minute),
			ActiveConnections: 500,
			RequestsPerSecond: 200,
			AverageLatency:    60 * time.Millisecond,
			ErrorRate:         0.01,
		},
		{
			Timestamp:         time.Now().Add(2 * time.Minute),
			ActiveConnections: 1200,
			RequestsPerSecond: 800,
			AverageLatency:    100 * time.Millisecond,
			ErrorRate:         0.03,
		},
	}
	
	for i, sample := range samples {
		lm.RecordSample(sample)
		load := lm.GetCurrentLoad()
		fmt.Printf("Sample %d: Load=%.2f, Connections=%d, RPS=%.0f\n",
			i+1, load, sample.ActiveConnections, sample.RequestsPerSecond)
	}
	
	// Output:
	// Sample 1: Load=0.09, Connections=100, RPS=50
	// Sample 2: Load=0.29, Connections=500, RPS=200
	// Sample 3: Load=0.74, Connections=1200, RPS=800
}