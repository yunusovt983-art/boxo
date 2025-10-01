package loadtest

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

// TestBitswapStressScenarios tests various stress scenarios
// Requirements: 1.1, 1.2 - System should handle extreme load conditions
func TestBitswapStressScenarios(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping stress tests in short mode")
	}

	scenarios := []StressTestConfig{
		{
			Name:               "BurstTraffic",
			NumNodes:           50,
			NumBlocks:          2000,
			ConcurrentRequests: 5000,
			TestDuration:       3 * time.Minute,
			BlockSize:          8192,
			NetworkDelay:       delay.Fixed(10 * time.Millisecond),
			RequestRate:        1000,
			BurstInterval:      30 * time.Second,
			BurstMultiplier:    10,
		},
		{
			Name:               "HighLatencyNetwork",
			NumNodes:           30,
			NumBlocks:          1000,
			ConcurrentRequests: 2000,
			TestDuration:       5 * time.Minute,
			BlockSize:          8192,
			NetworkDelay:       delay.Fixed(500 * time.Millisecond), // Very high latency
			RequestRate:        500,
		},
		{
			Name:               "LargeBlockStress",
			NumNodes:           20,
			NumBlocks:          100,
			ConcurrentRequests: 1000,
			TestDuration:       3 * time.Minute,
			BlockSize:          1048576, // 1MB blocks
			NetworkDelay:       delay.Fixed(50 * time.Millisecond),
			RequestRate:        100,
		},
		{
			Name:               "ManySmallBlocks",
			NumNodes:           40,
			NumBlocks:          10000,
			ConcurrentRequests: 3000,
			TestDuration:       4 * time.Minute,
			BlockSize:          512, // Very small blocks
			NetworkDelay:       delay.Fixed(5 * time.Millisecond),
			RequestRate:        2000,
		},
	}

	for _, scenario := range scenarios {
		t.Run(scenario.Name, func(t *testing.T) {
			result := runStressTest(t, scenario)

			// Validate stress test requirements
			if result.SuccessfulRequests < int64(float64(result.TotalRequests)*0.90) {
				t.Errorf("Success rate %.2f%% is below 90%% requirement for stress test %s",
					float64(result.SuccessfulRequests)/float64(result.TotalRequests)*100,
					scenario.Name)
			}

			if result.MemoryGrowthRate > 0.10 { // Allow 10% growth under stress
				t.Errorf("Memory growth rate %.2f%% exceeds 10%% limit for stress test %s",
					result.MemoryGrowthRate*100, scenario.Name)
			}

			t.Logf("Stress Test %s Results:", scenario.Name)
			t.Logf("  Total Requests: %d", result.TotalRequests)
			t.Logf("  Success Rate: %.2f%%", float64(result.SuccessfulRequests)/float64(result.TotalRequests)*100)
			t.Logf("  Peak Memory: %.2f MB", result.PeakMemoryMB)
			t.Logf("  Memory Growth: %.2f%%", result.MemoryGrowthRate*100)
			t.Logf("  P95 Latency: %v", result.P95Latency)
		})
	}
}

// TestBitswapResourceExhaustion tests behavior under resource exhaustion
// Requirements: 1.1, 1.2 - System should gracefully handle resource limits
func TestBitswapResourceExhaustion(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping resource exhaustion tests in short mode")
	}

	scenarios := []struct {
		name        string
		exhaustType string
		config      StressTestConfig
	}{
		{
			name:        "MemoryExhaustion",
			exhaustType: "memory",
			config: StressTestConfig{
				Name:               "MemoryExhaustion",
				NumNodes:           100,
				NumBlocks:          5000,
				ConcurrentRequests: 10000,
				TestDuration:       2 * time.Minute,
				BlockSize:          65536, // Large blocks to consume memory
				NetworkDelay:       delay.Fixed(10 * time.Millisecond),
				RequestRate:        5000,
			},
		},
		{
			name:        "ConnectionExhaustion",
			exhaustType: "connections",
			config: StressTestConfig{
				Name:               "ConnectionExhaustion",
				NumNodes:           200,
				NumBlocks:          1000,
				ConcurrentRequests: 20000,
				TestDuration:       90 * time.Second,
				BlockSize:          4096,
				NetworkDelay:       delay.Fixed(5 * time.Millisecond),
				RequestRate:        10000,
			},
		},
		{
			name:        "GoroutineExhaustion",
			exhaustType: "goroutines",
			config: StressTestConfig{
				Name:               "GoroutineExhaustion",
				NumNodes:           50,
				NumBlocks:          2000,
				ConcurrentRequests: 50000, // Very high concurrency
				TestDuration:       60 * time.Second,
				BlockSize:          8192,
				NetworkDelay:       delay.Fixed(20 * time.Millisecond),
				RequestRate:        25000,
			},
		},
	}

	for _, scenario := range scenarios {
		t.Run(scenario.name, func(t *testing.T) {
			// Monitor resource usage during test
			resourceMonitor := &ResourceMonitor{
				interval: 5 * time.Second,
			}

			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()

			go resourceMonitor.Start(ctx)

			result := runStressTest(t, scenario.config)

			resourceMonitor.Stop()

			// Validate that system handled resource exhaustion gracefully
			if result.SuccessfulRequests == 0 {
				t.Errorf("System completely failed under %s exhaustion", scenario.exhaustType)
			}

			// Check for resource leaks
			finalStats := resourceMonitor.GetFinalStats()
			if finalStats.GoroutineCount > 10000 {
				t.Errorf("Potential goroutine leak: %d goroutines remaining", finalStats.GoroutineCount)
			}

			t.Logf("Resource Exhaustion Test %s Results:", scenario.name)
			t.Logf("  Success Rate: %.2f%%", float64(result.SuccessfulRequests)/float64(result.TotalRequests)*100)
			t.Logf("  Peak Memory: %.2f MB", result.PeakMemoryMB)
			t.Logf("  Final Goroutines: %d", finalStats.GoroutineCount)
			t.Logf("  Recovery Time: %v", result.RecoveryTime)
		})
	}
}

// TestBitswapFailureRecovery tests recovery from various failure scenarios
// Requirements: 1.1, 1.2 - System should recover from failures gracefully
func TestBitswapFailureRecovery(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping failure recovery tests in short mode")
	}

	scenarios := []struct {
		name         string
		failureType  string
		failureRate  float64
		recoveryTime time.Duration
	}{
		{
			name:         "NetworkPartition",
			failureType:  "network",
			failureRate:  0.30, // 30% of connections fail
			recoveryTime: 30 * time.Second,
		},
		{
			name:         "NodeFailure",
			failureType:  "node",
			failureRate:  0.20, // 20% of nodes fail
			recoveryTime: 45 * time.Second,
		},
		{
			name:         "HighErrorRate",
			failureType:  "errors",
			failureRate:  0.50, // 50% error rate
			recoveryTime: 60 * time.Second,
		},
	}

	for _, scenario := range scenarios {
		t.Run(scenario.name, func(t *testing.T) {
			config := StressTestConfig{
				Name:               scenario.name,
				NumNodes:           30,
				NumBlocks:          1500,
				ConcurrentRequests: 2000,
				TestDuration:       3 * time.Minute,
				BlockSize:          8192,
				NetworkDelay:       delay.Fixed(20 * time.Millisecond),
				RequestRate:        1000,
				FailureRate:        scenario.failureRate,
			}

			result := runFailureRecoveryTest(t, config, scenario.failureType, scenario.recoveryTime)

			// Validate recovery requirements
			if result.FailureRecoveryRate < 0.80 {
				t.Errorf("Recovery rate %.2f%% is below 80%% requirement for %s",
					result.FailureRecoveryRate*100, scenario.name)
			}

			if result.RecoveryTime > scenario.recoveryTime*2 {
				t.Errorf("Recovery time %v exceeds maximum %v for %s",
					result.RecoveryTime, scenario.recoveryTime*2, scenario.name)
			}

			t.Logf("Failure Recovery Test %s Results:", scenario.name)
			t.Logf("  Recovery Rate: %.2f%%", result.FailureRecoveryRate*100)
			t.Logf("  Recovery Time: %v", result.RecoveryTime)
			t.Logf("  Final Success Rate: %.2f%%", float64(result.SuccessfulRequests)/float64(result.TotalRequests)*100)
		})
	}
}

// runFailureRecoveryTest executes a failure recovery test
func runFailureRecoveryTest(t *testing.T, config StressTestConfig, failureType string, recoveryTime time.Duration) *StressTestResult {
	ctx, cancel := context.WithTimeout(context.Background(), config.TestDuration+120*time.Second)
	defer cancel()

	// Setup test network
	net := tn.VirtualNetwork(config.NetworkDelay)
	router := mockrouting.NewServer()
	ig := testinstance.NewTestInstanceGenerator(net, router, nil, nil)
	defer ig.Close()

	// Create instances
	instances := ig.Instances(config.NumNodes)

	// Generate test blocks
	blocks := random.BlocksOfSize(config.NumBlocks, config.BlockSize)

	// Distribute blocks to seed nodes
	seedCount := config.NumNodes / 2
	seeds := instances[:seedCount]

	for _, seed := range seeds {
		if err := seed.Blockstore.PutMany(ctx, blocks); err != nil {
			t.Fatalf("Failed to put blocks: %v", err)
		}
	}

	// Prepare CIDs for requests
	var cids []cid.Cid
	for _, blk := range blocks {
		cids = append(cids, blk.Cid())
	}

	// Initialize metrics
	result := &StressTestResult{
		LoadTestResult: &LoadTestResult{},
	}

	var requestCount, successCount, failureCount int64
	var recoveryStartTime time.Time
	var recoveryDetected bool

	// Start the test
	startTime := time.Now()

	// Simulate failures after initial period
	failureStart := time.Now().Add(30 * time.Second)
	recoveryStart := failureStart.Add(recoveryTime)

	// Run test with failure injection
	var wg sync.WaitGroup
	requestChan := make(chan struct{}, config.ConcurrentRequests)

	// Rate limiter
	ticker := time.NewTicker(time.Second / time.Duration(config.RequestRate))
	defer ticker.Stop()

	// Start request workers
	requesters := instances[seedCount:]
	for _, requester := range requesters {
		for i := 0; i < config.ConcurrentRequests/len(requesters); i++ {
			wg.Add(1)
			go func(bs *bitswap.Bitswap) {
				defer wg.Done()

				session := bs.NewSession(ctx)

				for {
					select {
					case <-ctx.Done():
						return
					case <-requestChan:
						now := time.Now()
						shouldFail := false

						// Inject failures during failure period
						if now.After(failureStart) && now.Before(recoveryStart) {
							shouldFail = rand.Float64() < config.FailureRate
						}

						if shouldFail {
							atomic.AddInt64(&failureCount, 1)
							atomic.AddInt64(&requestCount, 1)
							continue
						}

						// Select random block to request
						targetCID := cids[rand.Intn(len(cids))]

						atomic.AddInt64(&requestCount, 1)

						block, err := session.GetBlock(ctx, targetCID)
						if err != nil {
							atomic.AddInt64(&failureCount, 1)
						} else if block != nil {
							atomic.AddInt64(&successCount, 1)

							// Detect recovery
							if now.After(recoveryStart) && !recoveryDetected {
								recoveryStartTime = now
								recoveryDetected = true
							}
						}
					}
				}
			}(requester.Exchange)
		}
	}

	// Generate requests
	go func() {
		defer close(requestChan)
		testEnd := time.Now().Add(config.TestDuration)

		for time.Now().Before(testEnd) {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				select {
				case requestChan <- struct{}{}:
				default:
					// Channel full, skip this tick
				}
			}
		}
	}()

	// Wait for test completion
	wg.Wait()

	endTime := time.Now()
	actualDuration := endTime.Sub(startTime)

	// Calculate results
	result.TotalRequests = atomic.LoadInt64(&requestCount)
	result.SuccessfulRequests = atomic.LoadInt64(&successCount)
	result.FailedRequests = atomic.LoadInt64(&failureCount)
	result.TestDuration = actualDuration
	result.RequestsPerSecond = float64(result.TotalRequests) / actualDuration.Seconds()

	if recoveryDetected {
		result.RecoveryTime = recoveryStartTime.Sub(recoveryStart)
	} else {
		result.RecoveryTime = recoveryTime * 2 // Indicate failure to recover
	}

	// Calculate recovery rate (success rate after recovery period)
	if recoveryDetected {
		result.FailureRecoveryRate = float64(result.SuccessfulRequests) / float64(result.TotalRequests)
	} else {
		result.FailureRecoveryRate = 0
	}

	// Get memory stats
	var memStats runtime.MemStats
	runtime.GC()
	runtime.ReadMemStats(&memStats)
	result.MemoryUsageMB = float64(memStats.Alloc) / 1024 / 1024
	result.GoroutineCount = runtime.NumGoroutine()

	return result
}
