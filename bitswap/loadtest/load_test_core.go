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
	"github.com/ipfs/go-test/random"
)

// runLoadTest executes a load test with the given configuration
func runLoadTest(t testing.TB, config LoadTestConfig) *LoadTestResult {
	ctx, cancel := context.WithTimeout(context.Background(), config.TestDuration+30*time.Second)
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

	// Distribute blocks across seed nodes (use half as seeds, half as requesters)
	seedCount := config.NumNodes / 2
	if seedCount == 0 {
		seedCount = 1
	}
	seeds := instances[:seedCount]
	requesters := instances[seedCount:]

	// Distribute blocks to seeds
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
	result := &LoadTestResult{}
	var latencies []time.Duration
	var latencyMutex sync.Mutex

	startTime := time.Now()
	var requestCount int64
	var successCount int64
	var failureCount int64
	var duplicateCount int64
	var networkErrorCount int64

	// Create request workers
	var wg sync.WaitGroup
	requestChan := make(chan struct{}, config.ConcurrentRequests)

	// Rate limiter
	ticker := time.NewTicker(time.Second / time.Duration(config.RequestRate))
	defer ticker.Stop()

	// Start request workers
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
						// Select random block to request
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
							if isNetworkError(err) {
								atomic.AddInt64(&networkErrorCount, 1)
							}
						} else if block != nil {
							atomic.AddInt64(&successCount, 1)
						}
					}
				}
			}(requester.Exchange)
		}
	}

	// Generate requests at specified rate
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

	// Calculate statistics
	result.TotalRequests = atomic.LoadInt64(&requestCount)
	result.SuccessfulRequests = atomic.LoadInt64(&successCount)
	result.FailedRequests = atomic.LoadInt64(&failureCount)
	result.DuplicateBlocks = atomic.LoadInt64(&duplicateCount)
	result.NetworkErrors = atomic.LoadInt64(&networkErrorCount)
	result.TestDuration = actualDuration
	result.RequestsPerSecond = float64(result.TotalRequests) / actualDuration.Seconds()

	// Calculate latency statistics
	if len(latencies) > 0 {
		result.AverageLatency = calculateAverageLatency(latencies)
		result.P95Latency = calculatePercentileLatency(latencies, 0.95)
		result.P99Latency = calculatePercentileLatency(latencies, 0.99)
		result.MaxLatency = calculateMaxLatency(latencies)
	}

	// Get memory and goroutine stats
	var memStats runtime.MemStats
	runtime.GC()
	runtime.ReadMemStats(&memStats)
	result.MemoryUsageMB = float64(memStats.Alloc) / 1024 / 1024
	result.GoroutineCount = runtime.NumGoroutine()

	// Count active connections
	result.ConnectionCount = countActiveConnections(instances)

	return result
}

func countActiveConnections(instances []testinstance.Instance) int {
	// This is a simplified connection count
	// In a real implementation, you'd query the network adapter
	return len(instances) * (len(instances) - 1) / 2
}

// runStressTest executes a stress test with the given configuration
func runStressTest(t testing.TB, config StressTestConfig) *StressTestResult {
	ctx, cancel := context.WithTimeout(context.Background(), config.TestDuration+60*time.Second)
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
	requesters := instances[seedCount:]

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

	// Memory tracking
	memoryTracker := &MemoryTracker{
		interval: 10 * time.Second,
		samples:  make([]float64, 0),
	}

	memCtx, memCancel := context.WithCancel(context.Background())
	defer memCancel()

	go memoryTracker.Start(memCtx)

	// Run the load test with stress conditions
	loadConfig := LoadTestConfig{
		NumNodes:           config.NumNodes,
		NumBlocks:          config.NumBlocks,
		ConcurrentRequests: config.ConcurrentRequests,
		TestDuration:       config.TestDuration,
		BlockSize:          config.BlockSize,
		NetworkDelay:       config.NetworkDelay,
		RequestRate:        config.RequestRate,
	}

	// Add burst traffic if configured
	if config.BurstInterval > 0 && config.BurstMultiplier > 1 {
		go simulateBurstTraffic(ctx, requesters, cids, config.BurstInterval, config.BurstMultiplier)
	}

	baseResult := runLoadTest(t, loadConfig)
	result.LoadTestResult = baseResult

	memoryTracker.Stop()

	// Calculate stress-specific metrics
	result.MemoryGrowthRate = memoryTracker.GetGrowthRate()
	result.PeakMemoryMB = memoryTracker.GetPeakMemory()
	result.GoroutineLeakCount = runtime.NumGoroutine() - 100 // Baseline goroutines

	return result
}

// simulateBurstTraffic creates periodic bursts of high traffic
func simulateBurstTraffic(ctx context.Context, requesters []testinstance.Instance, cids []cid.Cid, interval time.Duration, multiplier int) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			// Create burst of requests
			var wg sync.WaitGroup
			for i := 0; i < multiplier*100; i++ {
				wg.Add(1)
				go func() {
					defer wg.Done()
					requester := requesters[rand.Intn(len(requesters))]
					targetCID := cids[rand.Intn(len(cids))]

					session := requester.Exchange.NewSession(ctx)
					_, _ = session.GetBlock(ctx, targetCID)
				}()
			}
			wg.Wait()
		}
	}
}
