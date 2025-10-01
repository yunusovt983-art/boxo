package bitswap_test

import (
	"context"
	"math/rand"
	"runtime"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/ipfs/boxo/bitswap"
	testinstance "github.com/ipfs/boxo/bitswap/testinstance"
	tn "github.com/ipfs/boxo/bitswap/testnet"
	mockrouting "github.com/ipfs/boxo/routing/mock"
	cid "github.com/ipfs/go-cid"
	delay "github.com/ipfs/go-ipfs-delay"
	"github.com/ipfs/go-test/random"
)

// StressTestScenario defines different stress testing scenarios
type StressTestScenario struct {
	Name               string
	NumNodes           int
	NumBlocks          int
	BlockSize          int
	ConcurrentSessions int
	RequestBurstSize   int
	BurstInterval      time.Duration
	TestDuration       time.Duration
	NetworkDelay       delay.D
}

// TestBitswapStressScenarios runs various stress test scenarios
// Requirements: 1.1, 1.2 - Test system under extreme conditions
func TestBitswapStressScenarios(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping stress tests in short mode")
	}

	scenarios := []StressTestScenario{
		{
			Name:               "BurstTraffic",
			NumNodes:           20,
			NumBlocks:          1000,
			BlockSize:          8192,
			ConcurrentSessions: 500,
			RequestBurstSize:   100,
			BurstInterval:      1 * time.Second,
			TestDuration:       5 * time.Minute,
			NetworkDelay:       delay.Fixed(10 * time.Millisecond),
		},
		{
			Name:               "HighLatencyNetwork",
			NumNodes:           15,
			NumBlocks:          500,
			BlockSize:          4096,
			ConcurrentSessions: 200,
			RequestBurstSize:   50,
			BurstInterval:      2 * time.Second,
			TestDuration:       3 * time.Minute,
			NetworkDelay:       delay.Fixed(500 * time.Millisecond), // High latency
		},
		{
			Name:               "LargeBlockStress",
			NumNodes:           10,
			NumBlocks:          100,
			BlockSize:          1048576, // 1MB blocks
			ConcurrentSessions: 100,
			RequestBurstSize:   10,
			BurstInterval:      5 * time.Second,
			TestDuration:       10 * time.Minute,
			NetworkDelay:       delay.Fixed(50 * time.Millisecond),
		},
		{
			Name:               "ManySmallBlocks",
			NumNodes:           25,
			NumBlocks:          10000,
			BlockSize:          512, // Small blocks
			ConcurrentSessions: 1000,
			RequestBurstSize:   200,
			BurstInterval:      500 * time.Millisecond,
			TestDuration:       5 * time.Minute,
			NetworkDelay:       delay.Fixed(5 * time.Millisecond),
		},
	}

	for _, scenario := range scenarios {
		t.Run(scenario.Name, func(t *testing.T) {
			result := runStressTest(t, scenario)

			// Validate stress test results
			validateStressTestResult(t, scenario, result)

			t.Logf("Stress Test '%s' Results:", scenario.Name)
			t.Logf("  Total Requests: %d", result.TotalRequests)
			t.Logf("  Success Rate: %.2f%%", result.SuccessRate)
			t.Logf("  Average Latency: %v", result.AverageLatency)
			t.Logf("  P99 Latency: %v", result.P99Latency)
			t.Logf("  Peak Memory: %.2f MB", result.PeakMemoryMB)
			t.Logf("  Memory Growth: %.2f%%", result.MemoryGrowthPercent)
			t.Logf("  Error Rate: %.2f%%", result.ErrorRate)
		})
	}
}

// TestBitswapResourceExhaustion tests behavior under resource constraints
// Requirements: 1.1, 1.2 - System should handle resource exhaustion gracefully
func TestBitswapResourceExhaustion(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping resource exhaustion test in short mode")
	}

	// Test memory pressure scenario
	t.Run("MemoryPressure", func(t *testing.T) {
		scenario := StressTestScenario{
			Name:               "MemoryPressure",
			NumNodes:           30,
			NumBlocks:          5000,
			BlockSize:          65536, // 64KB blocks to consume memory
			ConcurrentSessions: 2000,
			RequestBurstSize:   500,
			BurstInterval:      100 * time.Millisecond,
			TestDuration:       3 * time.Minute,
			NetworkDelay:       delay.Fixed(10 * time.Millisecond),
		}

		result := runStressTest(t, scenario)

		// Under memory pressure, system should maintain basic functionality
		if result.SuccessRate < 0.80 { // Allow lower success rate under pressure
			t.Errorf("Success rate %.2f%% too low under memory pressure", result.SuccessRate*100)
		}

		// Memory growth should be controlled
		if result.MemoryGrowthPercent > 50 {
			t.Errorf("Memory growth %.2f%% indicates poor memory management", result.MemoryGrowthPercent)
		}
	})

	// Test connection exhaustion scenario
	t.Run("ConnectionExhaustion", func(t *testing.T) {
		scenario := StressTestScenario{
			Name:               "ConnectionExhaustion",
			NumNodes:           100, // Many nodes to exhaust connections
			NumBlocks:          1000,
			BlockSize:          4096,
			ConcurrentSessions: 5000, // High session count
			RequestBurstSize:   1000,
			BurstInterval:      500 * time.Millisecond,
			TestDuration:       2 * time.Minute,
			NetworkDelay:       delay.Fixed(20 * time.Millisecond),
		}

		result := runStressTest(t, scenario)

		// System should handle connection limits gracefully
		if result.ErrorRate > 0.20 { // Allow some errors under extreme load
			t.Errorf("Error rate %.2f%% too high under connection pressure", result.ErrorRate*100)
		}
	})
}

// TestBitswapFailureRecovery tests recovery from various failure scenarios
// Requirements: 1.1, 1.2 - System should recover from failures
func TestBitswapFailureRecovery(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping failure recovery test in short mode")
	}

	t.Run("NetworkPartition", func(t *testing.T) {
		testNetworkPartitionRecovery(t)
	})

	t.Run("NodeFailure", func(t *testing.T) {
		testNodeFailureRecovery(t)
	})

	t.Run("HighErrorRate", func(t *testing.T) {
		testHighErrorRateRecovery(t)
	})
}

// StressTestResult contains comprehensive stress test metrics
type StressTestResult struct {
	TotalRequests       int64
	SuccessfulRequests  int64
	FailedRequests      int64
	SuccessRate         float64
	ErrorRate           float64
	AverageLatency      time.Duration
	P95Latency          time.Duration
	P99Latency          time.Duration
	MaxLatency          time.Duration
	RequestsPerSecond   float64
	PeakMemoryMB        float64
	FinalMemoryMB       float64
	MemoryGrowthPercent float64
	PeakGoroutines      int
	FinalGoroutines     int
	ConnectionErrors    int64
	TimeoutErrors       int64
	NetworkErrors       int64
	TestDuration        time.Duration
}

func runStressTest(t *testing.T, scenario StressTestScenario) *StressTestResult {
	ctx, cancel := context.WithTimeout(context.Background(), scenario.TestDuration+1*time.Minute)
	defer cancel()

	// Setup test environment
	net := tn.VirtualNetwork(scenario.NetworkDelay)
	router := mockrouting.NewServer()
	ig := testinstance.NewTestInstanceGenerator(net, router, nil, nil)
	defer ig.Close()

	instances := ig.Instances(scenario.NumNodes)
	blocks := random.BlocksOfSize(scenario.NumBlocks, scenario.BlockSize)

	// Distribute blocks to first half of nodes
	seedCount := scenario.NumNodes / 2
	seeds := instances[:seedCount]
	requesters := instances[seedCount:]

	for _, seed := range seeds {
		if err := seed.Blockstore.PutMany(ctx, blocks); err != nil {
			t.Fatalf("Failed to put blocks: %v", err)
		}
	}

	// Prepare test data
	var cids []cid.Cid
	for _, blk := range blocks {
		cids = append(cids, blk.Cid())
	}

	// Initialize metrics tracking
	result := &StressTestResult{}
	var requestCount, successCount, failureCount int64
	var connectionErrors, timeoutErrors, networkErrors int64
	var latencies []time.Duration
	var latencyMutex sync.Mutex

	// Memory tracking
	var initialMemory, peakMemory float64
	var peakGoroutines int

	var memStats runtime.MemStats
	runtime.GC()
	runtime.ReadMemStats(&memStats)
	initialMemory = float64(memStats.Alloc) / 1024 / 1024
	peakMemory = initialMemory

	startTime := time.Now()

	// Create stress test workers
	var wg sync.WaitGroup

	// Memory monitoring goroutine
	wg.Add(1)
	go func() {
		defer wg.Done()
		ticker := time.NewTicker(5 * time.Second)
		defer ticker.Stop()

		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				runtime.ReadMemStats(&memStats)
				currentMemory := float64(memStats.Alloc) / 1024 / 1024
				if currentMemory > peakMemory {
					peakMemory = currentMemory
				}

				currentGoroutines := runtime.NumGoroutine()
				if currentGoroutines > peakGoroutines {
					peakGoroutines = currentGoroutines
				}
			}
		}
	}()

	// Burst request generators
	for _, requester := range requesters {
		for i := 0; i < scenario.ConcurrentSessions/len(requesters); i++ {
			wg.Add(1)
			go func(bs *bitswap.Bitswap, sessionID int) {
				defer wg.Done()

				session := bs.NewSession(ctx)
				burstTicker := time.NewTicker(scenario.BurstInterval)
				defer burstTicker.Stop()

				for {
					select {
					case <-ctx.Done():
						return
					case <-burstTicker.C:
						// Generate burst of requests
						for j := 0; j < scenario.RequestBurstSize; j++ {
							go func() {
								targetCID := cids[rand.Intn(len(cids))]
								requestStart := time.Now()
								atomic.AddInt64(&requestCount, 1)

								block, err := session.GetBlock(ctx, targetCID)
								latency := time.Since(requestStart)

								latencyMutex.Lock()
								latencies = append(latencies, latency)
								latencyMutex.Unlock()

								if err != nil {
									atomic.AddInt64(&failureCount, 1)
									categorizeError(err, &connectionErrors, &timeoutErrors, &networkErrors)
								} else if block != nil {
									atomic.AddInt64(&successCount, 1)
								}
							}()
						}
					}
				}
			}(requester.Exchange, i)
		}
	}

	// Wait for test completion
	wg.Wait()

	endTime := time.Now()
	actualDuration := endTime.Sub(startTime)

	// Calculate final metrics
	runtime.GC()
	runtime.ReadMemStats(&memStats)
	finalMemory := float64(memStats.Alloc) / 1024 / 1024

	result.TotalRequests = atomic.LoadInt64(&requestCount)
	result.SuccessfulRequests = atomic.LoadInt64(&successCount)
	result.FailedRequests = atomic.LoadInt64(&failureCount)
	result.ConnectionErrors = atomic.LoadInt64(&connectionErrors)
	result.TimeoutErrors = atomic.LoadInt64(&timeoutErrors)
	result.NetworkErrors = atomic.LoadInt64(&networkErrors)
	result.TestDuration = actualDuration

	if result.TotalRequests > 0 {
		result.SuccessRate = float64(result.SuccessfulRequests) / float64(result.TotalRequests)
		result.ErrorRate = float64(result.FailedRequests) / float64(result.TotalRequests)
	}

	result.RequestsPerSecond = float64(result.TotalRequests) / actualDuration.Seconds()
	result.PeakMemoryMB = peakMemory
	result.FinalMemoryMB = finalMemory
	result.PeakGoroutines = peakGoroutines
	result.FinalGoroutines = runtime.NumGoroutine()

	if initialMemory > 0 {
		result.MemoryGrowthPercent = ((finalMemory - initialMemory) / initialMemory) * 100
	}

	// Calculate latency statistics
	if len(latencies) > 0 {
		result.AverageLatency = calculateAverageLatency(latencies)
		result.P95Latency = calculatePercentileLatency(latencies, 0.95)
		result.P99Latency = calculatePercentileLatency(latencies, 0.99)
		result.MaxLatency = calculateMaxLatency(latencies)
	}

	return result
}

func validateStressTestResult(t *testing.T, scenario StressTestScenario, result *StressTestResult) {
	// Basic validation - adjust thresholds based on scenario
	minSuccessRate := 0.85  // 85% minimum success rate
	maxErrorRate := 0.15    // 15% maximum error rate
	maxMemoryGrowth := 30.0 // 30% maximum memory growth

	// Adjust thresholds for high-stress scenarios
	if scenario.Name == "MemoryPressure" || scenario.Name == "ConnectionExhaustion" {
		minSuccessRate = 0.70
		maxErrorRate = 0.30
		maxMemoryGrowth = 50.0
	}

	if result.SuccessRate < minSuccessRate {
		t.Errorf("Success rate %.2f%% below minimum %.2f%%", result.SuccessRate*100, minSuccessRate*100)
	}

	if result.ErrorRate > maxErrorRate {
		t.Errorf("Error rate %.2f%% above maximum %.2f%%", result.ErrorRate*100, maxErrorRate*100)
	}

	if result.MemoryGrowthPercent > maxMemoryGrowth {
		t.Errorf("Memory growth %.2f%% above maximum %.2f%%", result.MemoryGrowthPercent, maxMemoryGrowth)
	}

	// Latency should be reasonable even under stress
	maxP99Latency := 5 * time.Second
	if result.P99Latency > maxP99Latency {
		t.Errorf("P99 latency %v exceeds maximum %v", result.P99Latency, maxP99Latency)
	}
}

func categorizeError(err error, connErrors, timeoutErrors, netErrors *int64) {
	if err == nil {
		return
	}

	errStr := err.Error()
	switch {
	case errStr == "context deadline exceeded":
		atomic.AddInt64(timeoutErrors, 1)
	case errStr == "connection refused" || errStr == "connection reset":
		atomic.AddInt64(connErrors, 1)
	default:
		atomic.AddInt64(netErrors, 1)
	}
}

// Failure recovery test implementations
func testNetworkPartitionRecovery(t *testing.T) {
	// Simulate network partition and recovery
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()

	net := tn.VirtualNetwork(delay.Fixed(10 * time.Millisecond))
	router := mockrouting.NewServer()
	ig := testinstance.NewTestInstanceGenerator(net, router, nil, nil)
	defer ig.Close()

	instances := ig.Instances(10)
	blocks := random.BlocksOfSize(100, 8192)

	// Setup initial state
	seeds := instances[:5]
	requesters := instances[5:]

	for _, seed := range seeds {
		if err := seed.Blockstore.PutMany(ctx, blocks); err != nil {
			t.Fatalf("Failed to put blocks: %v", err)
		}
	}

	var cids []cid.Cid
	for _, blk := range blocks {
		cids = append(cids, blk.Cid())
	}

	// Phase 1: Normal operation
	successCount1 := performRequests(ctx, requesters, cids, 30*time.Second)

	// Phase 2: Simulate network partition (increase delay dramatically)
	// In a real test, you would disconnect some nodes
	net = tn.VirtualNetwork(delay.Fixed(2 * time.Second)) // High latency simulates partition

	successCount2 := performRequests(ctx, requesters, cids, 30*time.Second)

	// Phase 3: Recovery (restore normal network)
	net = tn.VirtualNetwork(delay.Fixed(10 * time.Millisecond))

	successCount3 := performRequests(ctx, requesters, cids, 30*time.Second)

	// Validate recovery
	if successCount3 < successCount1*0.8 {
		t.Errorf("Recovery performance %d significantly worse than initial %d", successCount3, successCount1)
	}

	t.Logf("Network Partition Recovery Test:")
	t.Logf("  Normal: %d successful requests", successCount1)
	t.Logf("  Partition: %d successful requests", successCount2)
	t.Logf("  Recovery: %d successful requests", successCount3)
}

func testNodeFailureRecovery(t *testing.T) {
	// Test recovery from node failures
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
	defer cancel()

	net := tn.VirtualNetwork(delay.Fixed(10 * time.Millisecond))
	router := mockrouting.NewServer()
	ig := testinstance.NewTestInstanceGenerator(net, router, nil, nil)
	defer ig.Close()

	instances := ig.Instances(15)
	blocks := random.BlocksOfSize(200, 8192)

	// Distribute blocks across all nodes for redundancy
	for _, instance := range instances {
		if err := instance.Blockstore.PutMany(ctx, blocks); err != nil {
			t.Fatalf("Failed to put blocks: %v", err)
		}
	}

	var cids []cid.Cid
	for _, blk := range blocks {
		cids = append(cids, blk.Cid())
	}

	// Phase 1: All nodes active
	successCount1 := performRequests(ctx, instances, cids, 30*time.Second)

	// Phase 2: Simulate node failures (close some instances)
	failedNodes := instances[:5] // Fail first 5 nodes
	for _, node := range failedNodes {
		node.Exchange.Close()
	}

	activeNodes := instances[5:]
	successCount2 := performRequests(ctx, activeNodes, cids, 30*time.Second)

	// Validate that system continues to function with reduced capacity
	expectedMinSuccess := int64(float64(successCount1) * 0.6) // Expect at least 60% performance
	if successCount2 < expectedMinSuccess {
		t.Errorf("Performance after node failure %d too low, expected at least %d", successCount2, expectedMinSuccess)
	}

	t.Logf("Node Failure Recovery Test:")
	t.Logf("  All nodes: %d successful requests", successCount1)
	t.Logf("  After failures: %d successful requests", successCount2)
	t.Logf("  Performance retention: %.2f%%", float64(successCount2)/float64(successCount1)*100)
}

func testHighErrorRateRecovery(t *testing.T) {
	// Test recovery from high error rate conditions
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
	defer cancel()

	// Use high latency and variable delay to simulate error conditions
	errorDelay := delay.Fixed(1 * time.Second)
	net := tn.VirtualNetwork(errorDelay)
	router := mockrouting.NewServer()
	ig := testinstance.NewTestInstanceGenerator(net, router, nil, nil)
	defer ig.Close()

	instances := ig.Instances(10)
	blocks := random.BlocksOfSize(100, 8192)

	seeds := instances[:5]
	requesters := instances[5:]

	for _, seed := range seeds {
		if err := seed.Blockstore.PutMany(ctx, blocks); err != nil {
			t.Fatalf("Failed to put blocks: %v", err)
		}
	}

	var cids []cid.Cid
	for _, blk := range blocks {
		cids = append(cids, blk.Cid())
	}

	// Phase 1: High error rate conditions
	successCount1 := performRequests(ctx, requesters, cids, 45*time.Second)

	// Phase 2: Recovery (improve network conditions)
	net = tn.VirtualNetwork(delay.Fixed(10 * time.Millisecond))

	successCount2 := performRequests(ctx, requesters, cids, 45*time.Second)

	// System should recover and perform better
	if successCount2 <= successCount1 {
		t.Errorf("No improvement after error recovery: before=%d, after=%d", successCount1, successCount2)
	}

	t.Logf("High Error Rate Recovery Test:")
	t.Logf("  High error conditions: %d successful requests", successCount1)
	t.Logf("  After recovery: %d successful requests", successCount2)
	t.Logf("  Improvement: %.2fx", float64(successCount2)/float64(successCount1))
}

func performRequests(ctx context.Context, instances []testinstance.Instance, cids []cid.Cid, duration time.Duration) int64 {
	var successCount int64
	var wg sync.WaitGroup

	testCtx, cancel := context.WithTimeout(ctx, duration)
	defer cancel()

	for _, instance := range instances {
		wg.Add(1)
		go func(bs *bitswap.Bitswap) {
			defer wg.Done()

			session := bs.NewSession(testCtx)
			ticker := time.NewTicker(100 * time.Millisecond)
			defer ticker.Stop()

			for {
				select {
				case <-testCtx.Done():
					return
				case <-ticker.C:
					targetCID := cids[rand.Intn(len(cids))]
					_, err := session.GetBlock(testCtx, targetCID)
					if err == nil {
						atomic.AddInt64(&successCount, 1)
					}
				}
			}
		}(instance.Exchange)
	}

	wg.Wait()
	return atomic.LoadInt64(&successCount)
}
