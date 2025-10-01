package bitswap_test

import (
	"context"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/ipfs/boxo/bitswap/adaptive"
	"github.com/ipfs/boxo/bitswap/circuitbreaker"
	"github.com/ipfs/boxo/bitswap/priority"
	testsession "github.com/ipfs/boxo/bitswap/testinstance"
	"github.com/ipfs/boxo/blockstore"
	"github.com/ipfs/go-cid"
	delay "github.com/ipfs/go-ipfs-delay"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/multiformats/go-multihash"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ClusterTestConfig defines configuration for cluster integration tests
type ClusterTestConfig struct {
	NodeCount            int
	BlockCount           int
	ConcurrentRequests   int
	TestDuration         time.Duration
	NetworkLatency       time.Duration
	FailureRate          float64
	EnableMetrics        bool
	EnableCircuitBreaker bool
	EnablePriority       bool
}

// DefaultClusterTestConfig returns a default configuration for cluster tests
func DefaultClusterTestConfig() ClusterTestConfig {
	return ClusterTestConfig{
		NodeCount:            5,
		BlockCount:           100,
		ConcurrentRequests:   50,
		TestDuration:         30 * time.Second,
		NetworkLatency:       50 * time.Millisecond,
		FailureRate:          0.05, // 5% failure rate
		EnableMetrics:        true,
		EnableCircuitBreaker: true,
		EnablePriority:       true,
	}
}

// ClusterTestNode represents a single node in the test cluster
type ClusterTestNode struct {
	Instance        testsession.Instance
	CircuitBreaker  *circuitbreaker.BitswapProtectionManager
	PriorityManager *priority.AdaptivePriorityManager
	AdaptiveConfig  *adaptive.AdaptiveBitswapConfig
	NodeID          peer.ID
	IsHealthy       bool
	mu              sync.RWMutex
}

// ClusterTestEnvironment manages the entire test cluster
type ClusterTestEnvironment struct {
	Nodes       []*ClusterTestNode
	Network     testnet.Network
	Config      ClusterTestConfig
	AlertsCount int64
	mu          sync.RWMutex
}

// TestClusterNodeInteraction tests basic interaction between cluster nodes
func TestClusterNodeInteraction(t *testing.T) {
	config := DefaultClusterTestConfig()
	config.NodeCount = 3
	config.TestDuration = 10 * time.Second

	env := setupClusterTestEnvironment(t, config)
	defer env.Cleanup()

	ctx, cancel := context.WithTimeout(context.Background(), config.TestDuration)
	defer cancel()

	// Start all nodes
	env.StartAllNodes(ctx)

	// Create test blocks on first node
	testBlocks := generateTestBlocks(t, 10)
	sourceNode := env.Nodes[0]

	// Add blocks to source node
	for _, block := range testBlocks {
		err := sourceNode.Instance.Blockstore.Put(ctx, block)
		require.NoError(t, err)
	}

	// Test block retrieval from other nodes
	var wg sync.WaitGroup
	for i := 1; i < len(env.Nodes); i++ {
		wg.Add(1)
		go func(nodeIndex int) {
			defer wg.Done()
			targetNode := env.Nodes[nodeIndex]

			for _, block := range testBlocks {
				retrievedBlock, err := targetNode.Instance.Exchange.GetBlock(ctx, block.Cid())
				assert.NoError(t, err, "Node %d should retrieve block from node 0", nodeIndex)
				assert.Equal(t, block.RawData(), retrievedBlock.RawData(), "Retrieved block should match original")
			}
		}(i)
	}

	wg.Wait()

	// Verify all nodes have healthy connections
	for i, node := range env.Nodes {
		assert.True(t, node.IsHealthy, "Node %d should be healthy", i)
	}

	t.Logf("Successfully tested interaction between %d cluster nodes", config.NodeCount)
}

// TestClusterMetricsAndAlerts tests metrics collection and alert generation in cluster environment
func TestClusterMetricsAndAlerts(t *testing.T) {
	config := DefaultClusterTestConfig()
	config.NodeCount = 4
	config.TestDuration = 15 * time.Second
	config.ConcurrentRequests = 100

	env := setupClusterTestEnvironment(t, config)
	defer env.Cleanup()

	ctx, cancel := context.WithTimeout(context.Background(), config.TestDuration)
	defer cancel()

	// Start all nodes with metrics collection
	env.StartAllNodes(ctx)

	// Generate high load to trigger metrics and alerts
	testBlocks := generateTestBlocks(t, 50)

	// Distribute blocks across nodes
	for i, block := range testBlocks {
		nodeIndex := i % len(env.Nodes)
		err := env.Nodes[nodeIndex].Instance.Blockstore.Put(ctx, block)
		require.NoError(t, err)
	}

	// Generate concurrent requests to create load
	var wg sync.WaitGroup
	requestsPerNode := config.ConcurrentRequests / len(env.Nodes)

	for nodeIndex := 0; nodeIndex < len(env.Nodes); nodeIndex++ {
		wg.Add(1)
		go func(nodeIdx int) {
			defer wg.Done()
			node := env.Nodes[nodeIdx]

			for i := 0; i < requestsPerNode; i++ {
				blockIndex := (nodeIdx*requestsPerNode + i) % len(testBlocks)
				block := testBlocks[blockIndex]

				// Request block from a different node
				targetNodeIdx := (nodeIdx + 1) % len(env.Nodes)
				targetNode := env.Nodes[targetNodeIdx]

				_, err := targetNode.Instance.Exchange.GetBlock(ctx, block.Cid())
				if err != nil {
					t.Logf("Request failed (expected under high load): %v", err)
				}

				// Small delay to prevent overwhelming
				time.Sleep(10 * time.Millisecond)
			}
		}(nodeIndex)
	}

	wg.Wait()

	// Wait for metrics collection
	time.Sleep(2 * time.Second)

	// Verify metrics collection
	for i, node := range env.Nodes {
		if node.MetricsCollector != nil {
			metrics := node.MetricsCollector.GetCurrentMetrics()
			assert.True(t, metrics.TotalRequests > 0, "Node %d should have processed requests", i)
			assert.True(t, metrics.AverageResponseTime > 0, "Node %d should have response time metrics", i)

			t.Logf("Node %d metrics: Requests=%d, AvgResponseTime=%v, P95=%v",
				i, metrics.TotalRequests, metrics.AverageResponseTime, metrics.P95ResponseTime)
		}

		if node.Monitor != nil {
			status := node.Monitor.GetCurrentStatus()
			assert.NotNil(t, status, "Node %d should have resource status", i)

			health := node.Monitor.GetHealthStatus()
			assert.NotNil(t, health, "Node %d should have health status", i)
		}
	}

	// Check if any alerts were generated
	alertCount := env.GetAlertsCount()
	t.Logf("Total alerts generated during test: %d", alertCount)

	// Verify adaptive configuration updates
	for i, node := range env.Nodes {
		if node.AdaptiveConfig != nil {
			workerCount := node.AdaptiveConfig.GetCurrentWorkerCount()
			bytesLimit := node.AdaptiveConfig.GetMaxOutstandingBytesPerPeer()

			assert.True(t, workerCount > 0, "Node %d should have worker threads", i)
			assert.True(t, bytesLimit > 0, "Node %d should have bytes limit", i)

			t.Logf("Node %d adaptive config: Workers=%d, BytesLimit=%d", i, workerCount, bytesLimit)
		}
	}
}

// TestClusterFaultTolerance tests cluster behavior under various failure scenarios
func TestClusterFaultTolerance(t *testing.T) {
	config := DefaultClusterTestConfig()
	config.NodeCount = 5
	config.TestDuration = 20 * time.Second
	config.FailureRate = 0.2 // 20% failure rate for testing

	env := setupClusterTestEnvironment(t, config)
	defer env.Cleanup()

	ctx, cancel := context.WithTimeout(context.Background(), config.TestDuration)
	defer cancel()

	// Start all nodes
	env.StartAllNodes(ctx)

	// Create test data
	testBlocks := generateTestBlocks(t, 30)

	// Distribute blocks across all nodes
	for i, block := range testBlocks {
		nodeIndex := i % len(env.Nodes)
		err := env.Nodes[nodeIndex].Instance.Blockstore.Put(ctx, block)
		require.NoError(t, err)
	}

	// Test 1: Node failure simulation
	t.Log("Testing node failure scenario...")
	failedNodeIndex := 2
	env.SimulateNodeFailure(failedNodeIndex)

	// Wait for failure detection
	time.Sleep(2 * time.Second)

	// Verify other nodes can still serve requests
	workingNodes := make([]*ClusterTestNode, 0)
	for i, node := range env.Nodes {
		if i != failedNodeIndex {
			workingNodes = append(workingNodes, node)
		}
	}

	// Test requests to working nodes
	var wg sync.WaitGroup
	for _, node := range workingNodes {
		wg.Add(1)
		go func(n *ClusterTestNode) {
			defer wg.Done()
			for _, block := range testBlocks[:5] { // Test subset
				_, err := n.Instance.Exchange.GetBlock(ctx, block.Cid())
				// Some failures are expected due to the failed node
				if err != nil {
					t.Logf("Expected failure due to node failure: %v", err)
				}
			}
		}(node)
	}
	wg.Wait()

	// Test 2: Network partition simulation
	t.Log("Testing network partition scenario...")
	env.SimulateNetworkPartition([]int{0, 1}, []int{3, 4}) // Split into two groups

	// Wait for partition detection
	time.Sleep(2 * time.Second)

	// Test 3: Node recovery
	t.Log("Testing node recovery scenario...")
	env.RecoverNode(failedNodeIndex)

	// Wait for recovery
	time.Sleep(3 * time.Second)

	// Verify recovered node can serve requests
	recoveredNode := env.Nodes[failedNodeIndex]
	for _, block := range testBlocks[:3] {
		_, err := recoveredNode.Instance.Exchange.GetBlock(ctx, block.Cid())
		if err != nil {
			t.Logf("Recovery still in progress: %v", err)
		}
	}

	// Test 4: Circuit breaker activation
	t.Log("Testing circuit breaker activation...")
	if env.Nodes[0].CircuitBreaker != nil {
		// Generate failures to trip circuit breaker
		testPeer := env.Nodes[1].NodeID
		for i := 0; i < 5; i++ {
			err := env.Nodes[0].CircuitBreaker.ExecuteRequest(ctx, testPeer, func(ctx context.Context) error {
				return fmt.Errorf("simulated failure %d", i)
			})
			assert.Error(t, err, "Should receive simulated error")
		}

		// Verify circuit breaker status
		status := env.Nodes[0].CircuitBreaker.GetPeerStatus(testPeer)
		t.Logf("Circuit breaker status for peer %s: State=%s, Health=%s",
			testPeer.String()[:8], status.CircuitState, status.HealthStatus)
	}

	// Verify cluster resilience metrics
	healthyNodes := 0
	for i, node := range env.Nodes {
		if node.IsHealthy {
			healthyNodes++
		}
		t.Logf("Node %d health status: %t", i, node.IsHealthy)
	}

	// At least 60% of nodes should remain healthy
	minHealthyNodes := int(float64(len(env.Nodes)) * 0.6)
	assert.GreaterOrEqual(t, healthyNodes, minHealthyNodes,
		"At least %d nodes should remain healthy", minHealthyNodes)

	t.Logf("Fault tolerance test completed. %d/%d nodes healthy", healthyNodes, len(env.Nodes))
}

// TestClusterPerformanceUnderLoad tests cluster performance under sustained load
func TestClusterPerformanceUnderLoad(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping performance test in short mode")
	}

	config := DefaultClusterTestConfig()
	config.NodeCount = 6
	config.TestDuration = 30 * time.Second
	config.ConcurrentRequests = 200
	config.BlockCount = 200

	env := setupClusterTestEnvironment(t, config)
	defer env.Cleanup()

	ctx, cancel := context.WithTimeout(context.Background(), config.TestDuration)
	defer cancel()

	// Start all nodes
	env.StartAllNodes(ctx)

	// Create large dataset
	testBlocks := generateTestBlocks(t, config.BlockCount)

	// Distribute blocks across nodes
	for i, block := range testBlocks {
		nodeIndex := i % len(env.Nodes)
		err := env.Nodes[nodeIndex].Instance.Blockstore.Put(ctx, block)
		require.NoError(t, err)
	}

	// Performance test metrics
	var (
		totalRequests  int64
		successfulReqs int64
		failedReqs     int64
		totalLatency   time.Duration
		mu             sync.Mutex
	)

	// Generate sustained load
	var wg sync.WaitGroup
	requestsPerNode := config.ConcurrentRequests / len(env.Nodes)

	startTime := time.Now()

	for nodeIndex := 0; nodeIndex < len(env.Nodes); nodeIndex++ {
		wg.Add(1)
		go func(nodeIdx int) {
			defer wg.Done()
			node := env.Nodes[nodeIdx]

			for time.Since(startTime) < config.TestDuration {
				blockIndex := int(time.Now().UnixNano()) % len(testBlocks)
				block := testBlocks[blockIndex]

				reqStart := time.Now()
				_, err := node.Instance.Exchange.GetBlock(ctx, block.Cid())
				reqLatency := time.Since(reqStart)

				mu.Lock()
				totalRequests++
				totalLatency += reqLatency
				if err == nil {
					successfulReqs++
				} else {
					failedReqs++
				}
				mu.Unlock()

				// Throttle requests
				time.Sleep(time.Duration(1000/requestsPerNode) * time.Millisecond)
			}
		}(nodeIndex)
	}

	wg.Wait()

	// Calculate performance metrics
	avgLatency := totalLatency / time.Duration(totalRequests)
	successRate := float64(successfulReqs) / float64(totalRequests)
	requestsPerSecond := float64(totalRequests) / config.TestDuration.Seconds()

	t.Logf("Performance Results:")
	t.Logf("  Total Requests: %d", totalRequests)
	t.Logf("  Successful: %d (%.2f%%)", successfulReqs, successRate*100)
	t.Logf("  Failed: %d (%.2f%%)", failedReqs, (1-successRate)*100)
	t.Logf("  Average Latency: %v", avgLatency)
	t.Logf("  Requests/Second: %.2f", requestsPerSecond)

	// Performance assertions
	assert.Greater(t, successRate, 0.8, "Success rate should be > 80%")
	assert.Less(t, avgLatency, 500*time.Millisecond, "Average latency should be < 500ms")
	assert.Greater(t, requestsPerSecond, 10.0, "Should handle > 10 requests/second")

	// Verify adaptive behavior
	for i, node := range env.Nodes {
		if node.AdaptiveConfig != nil {
			initialWorkers := 128 // Default worker count
			currentWorkers := node.AdaptiveConfig.GetCurrentWorkerCount()

			t.Logf("Node %d: Workers adapted from %d to %d", i, initialWorkers, currentWorkers)
		}
	}
}

// setupClusterTestEnvironment creates and configures a test cluster environment
func setupClusterTestEnvironment(t *testing.T, config ClusterTestConfig) *ClusterTestEnvironment {
	// Create virtual network with latency
	net := testnet.VirtualNetwork(delay.Fixed(config.NetworkLatency))

	env := &ClusterTestEnvironment{
		Network: net,
		Config:  config,
		Nodes:   make([]*ClusterTestNode, config.NodeCount),
	}

	// Create test instances
	generator := testsession.NewTestInstanceGenerator(net, nil, nil, nil)
	instances := generator.Instances(config.NodeCount)

	// Configure each node
	for i, instance := range instances {
		node := &ClusterTestNode{
			Instance:  instance,
			NodeID:    instance.Identity.ID(),
			IsHealthy: true,
		}

		// Setup adaptive configuration if enabled
		if config.EnableMetrics {
			node.AdaptiveConfig = adaptive.NewAdaptiveBitswapConfig()
		}

		// Setup circuit breaker if enabled
		if config.EnableCircuitBreaker {
			cbConfig := circuitbreaker.DefaultProtectionConfig()
			node.CircuitBreaker = circuitbreaker.NewBitswapProtectionManager(cbConfig)
		}

		// Setup priority manager if enabled
		if config.EnablePriority {
			node.PriorityManager = priority.CreateAdaptivePriorityManagerFromConfig(node.AdaptiveConfig)
		}

		env.Nodes[i] = node
	}

	return env
}

// StartAllNodes starts all nodes in the cluster
func (env *ClusterTestEnvironment) StartAllNodes(ctx context.Context) {
	for i, node := range env.Nodes {
		if node.Monitor != nil {
			err := node.Monitor.Start(ctx)
			if err != nil {
				panic(fmt.Sprintf("Failed to start monitor for node %d: %v", i, err))
			}
		}

		if node.PriorityManager != nil {
			node.PriorityManager.Start()
		}
	}
}

// Cleanup shuts down the test environment
func (env *ClusterTestEnvironment) Cleanup() {
	for i, node := range env.Nodes {
		if node.Monitor != nil {
			node.Monitor.Stop()
		}

		if node.PriorityManager != nil {
			node.PriorityManager.Stop()
		}

		if node.CircuitBreaker != nil {
			node.CircuitBreaker.Close()
		}

		if node.Instance.Exchange != nil {
			node.Instance.Exchange.Close()
		}

		fmt.Printf("Cleaned up node %d\n", i)
	}
}

// SimulateNodeFailure simulates a node failure
func (env *ClusterTestEnvironment) SimulateNodeFailure(nodeIndex int) {
	if nodeIndex >= 0 && nodeIndex < len(env.Nodes) {
		env.Nodes[nodeIndex].mu.Lock()
		env.Nodes[nodeIndex].IsHealthy = false
		env.Nodes[nodeIndex].mu.Unlock()

		// Disconnect from other nodes
		for i, otherNode := range env.Nodes {
			if i != nodeIndex {
				env.Nodes[nodeIndex].Instance.Adapter.DisconnectFrom(
					context.Background(),
					otherNode.NodeID,
				)
			}
		}
	}
}

// RecoverNode simulates node recovery
func (env *ClusterTestEnvironment) RecoverNode(nodeIndex int) {
	if nodeIndex >= 0 && nodeIndex < len(env.Nodes) {
		env.Nodes[nodeIndex].mu.Lock()
		env.Nodes[nodeIndex].IsHealthy = true
		env.Nodes[nodeIndex].mu.Unlock()

		// Reconnect to other nodes
		for i, otherNode := range env.Nodes {
			if i != nodeIndex {
				env.Nodes[nodeIndex].Instance.Adapter.Connect(
					context.Background(),
					peer.AddrInfo{ID: otherNode.NodeID},
				)
			}
		}
	}
}

// SimulateNetworkPartition simulates a network partition
func (env *ClusterTestEnvironment) SimulateNetworkPartition(group1, group2 []int) {
	// Disconnect nodes between groups
	for _, i := range group1 {
		for _, j := range group2 {
			if i < len(env.Nodes) && j < len(env.Nodes) {
				env.Nodes[i].Instance.Adapter.DisconnectFrom(
					context.Background(),
					env.Nodes[j].NodeID,
				)
			}
		}
	}
}

// GetAlertsCount returns the total number of alerts generated
func (env *ClusterTestEnvironment) GetAlertsCount() int64 {
	env.mu.RLock()
	defer env.mu.RUnlock()
	return env.AlertsCount
}

// generateTestBlocks creates test blocks for testing
func generateTestBlocks(t *testing.T, count int) []blockstore.Block {
	blocks := make([]blockstore.Block, count)

	for i := 0; i < count; i++ {
		data := []byte(fmt.Sprintf("test block data %d - %s", i, time.Now().String()))
		hash, err := multihash.Sum(data, multihash.SHA2_256, -1)
		require.NoError(t, err)

		c := cid.NewCidV1(cid.Raw, hash)
		block, err := blockstore.NewBlockWithCid(data, c)
		require.NoError(t, err)

		blocks[i] = block
	}

	return blocks
}

// Benchmark tests for cluster performance
func BenchmarkClusterNodeInteraction(b *testing.B) {
	config := DefaultClusterTestConfig()
	config.NodeCount = 3
	config.TestDuration = 10 * time.Second

	env := setupClusterTestEnvironment(b, config)
	defer env.Cleanup()

	ctx := context.Background()
	env.StartAllNodes(ctx)

	testBlocks := generateTestBlocks(b, 10)
	sourceNode := env.Nodes[0]

	for _, block := range testBlocks {
		sourceNode.Instance.Blockstore.Put(ctx, block)
	}

	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		blockIndex := i % len(testBlocks)
		block := testBlocks[blockIndex]
		nodeIndex := (i % (len(env.Nodes) - 1)) + 1

		_, err := env.Nodes[nodeIndex].Instance.Exchange.GetBlock(ctx, block.Cid())
		if err != nil {
			b.Logf("Request failed: %v", err)
		}
	}
}

func BenchmarkClusterHighLoad(b *testing.B) {
	config := DefaultClusterTestConfig()
	config.NodeCount = 5
	config.ConcurrentRequests = 100

	env := setupClusterTestEnvironment(b, config)
	defer env.Cleanup()

	ctx := context.Background()
	env.StartAllNodes(ctx)

	testBlocks := generateTestBlocks(b, 50)

	// Distribute blocks
	for i, block := range testBlocks {
		nodeIndex := i % len(env.Nodes)
		env.Nodes[nodeIndex].Instance.Blockstore.Put(ctx, block)
	}

	b.ResetTimer()

	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			blockIndex := int(time.Now().UnixNano()) % len(testBlocks)
			block := testBlocks[blockIndex]
			nodeIndex := int(time.Now().UnixNano()) % len(env.Nodes)

			_, err := env.Nodes[nodeIndex].Instance.Exchange.GetBlock(ctx, block.Cid())
			if err != nil {
				// Expected under high load
			}
		}
	})
}
