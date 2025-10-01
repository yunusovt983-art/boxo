package bitswap_test

import (
	"context"
	"fmt"
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

// BenchmarkBitswapConcurrentConnections benchmarks performance with varying connection counts
// Requirements: 1.1, 1.2 - Automated benchmarks for different connection scales
func BenchmarkBitswapConcurrentConnections(b *testing.B) {
	connectionCounts := []int{100, 500, 1000, 2000, 5000, 10000}

	for _, connCount := range connectionCounts {
		b.Run(fmt.Sprintf("Connections_%d", connCount), func(b *testing.B) {
			benchmarkConcurrentConnections(b, connCount)
		})
	}
}

// BenchmarkBitswapRequestThroughput benchmarks request throughput under different loads
// Requirements: 1.1, 1.2 - Measure throughput capabilities
func BenchmarkBitswapRequestThroughput(b *testing.B) {
	throughputTargets := []int{1000, 5000, 10000, 25000, 50000, 100000}

	for _, target := range throughputTargets {
		b.Run(fmt.Sprintf("RPS_%d", target), func(b *testing.B) {
			benchmarkRequestThroughput(b, target)
		})
	}
}

// BenchmarkBitswapLatencyUnderLoad benchmarks latency characteristics under load
// Requirements: 1.1, 1.2 - Ensure <100ms response time under load
func BenchmarkBitswapLatencyUnderLoad(b *testing.B) {
	loadLevels := []struct {
		name        string
		nodes       int
		concurrency int
		requestRate int
	}{
		{"Light", 10, 100, 1000},
		{"Medium", 25, 500, 5000},
		{"Heavy", 50, 1000, 10000},
		{"Extreme", 100, 2000, 25000},
	}

	for _, level := range loadLevels {
		b.Run(level.name, func(b *testing.B) {
			benchmarkLatencyUnderLoad(b, level.nodes, level.concurrency, level.requestRate)
		})
	}
}

// BenchmarkBitswapMemoryEfficiency benchmarks memory usage patterns
// Requirements: 1.1, 1.2 - Monitor memory efficiency under different scenarios
func BenchmarkBitswapMemoryEfficiency(b *testing.B) {
	scenarios := []struct {
		name      string
		blockSize int
		numBlocks int
		sessions  int
	}{
		{"SmallBlocks", 1024, 10000, 100},
		{"MediumBlocks", 8192, 5000, 200},
		{"LargeBlocks", 65536, 1000, 50},
		{"XLargeBlocks", 1048576, 100, 10},
	}

	for _, scenario := range scenarios {
		b.Run(scenario.name, func(b *testing.B) {
			benchmarkMemoryEfficiency(b, scenario.blockSize, scenario.numBlocks, scenario.sessions)
		})
	}
}

// BenchmarkBitswapNetworkConditions benchmarks performance under different network conditions
// Requirements: 1.1, 1.2 - Test performance under various network conditions
func BenchmarkBitswapNetworkConditions(b *testing.B) {
	conditions := []struct {
		name  string
		delay delay.D
	}{
		{"LowLatency", delay.Fixed(1 * time.Millisecond)},
		{"LANLatency", delay.Fixed(10 * time.Millisecond)},
		{"WANLatency", delay.Fixed(50 * time.Millisecond)},
		{"HighLatency", delay.Fixed(200 * time.Millisecond)},
		{"SatelliteLatency", delay.Fixed(600 * time.Millisecond)},
	}

	for _, condition := range conditions {
		b.Run(condition.name, func(b *testing.B) {
			benchmarkNetworkConditions(b, condition.delay)
		})
	}
}

// BenchmarkBitswapScalabilityLimits finds the scalability limits of the system
// Requirements: 1.1, 1.2 - Determine system limits and breaking points
func BenchmarkBitswapScalabilityLimits(b *testing.B) {
	b.Run("MaxNodes", func(b *testing.B) {
		benchmarkMaxNodes(b)
	})

	b.Run("MaxConcurrentRequests", func(b *testing.B) {
		benchmarkMaxConcurrentRequests(b)
	})

	b.Run("MaxBlockSize", func(b *testing.B) {
		benchmarkMaxBlockSize(b)
	})
}

// Implementation of benchmark functions

func benchmarkConcurrentConnections(b *testing.B, connectionCount int) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()

	net := tn.VirtualNetwork(delay.Fixed(10 * time.Millisecond))
	router := mockrouting.NewServer()
	ig := testinstance.NewTestInstanceGenerator(net, router, nil, nil)
	defer ig.Close()

	// Create instances to simulate connections
	nodeCount := min(connectionCount/10, 100) // Reasonable node count
	instances := ig.Instances(nodeCount)
	blocks := random.BlocksOfSize(1000, 8192)

	// Setup seeds and requesters
	seedCount := nodeCount / 2
	seeds := instances[:seedCount]
	requesters := instances[seedCount:]

	for _, seed := range seeds {
		if err := seed.Blockstore.PutMany(ctx, blocks); err != nil {
			b.Fatalf("Failed to put blocks: %v", err)
		}
	}

	var cids []cid.Cid
	for _, blk := range blocks {
		cids = append(cids, blk.Cid())
	}

	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		var requestCount, successCount int64
		var totalLatency time.Duration
		var wg sync.WaitGroup

		startTime := time.Now()

		// Create concurrent sessions to simulate connections
		sessionsPerRequester := connectionCount / len(requesters)
		for _, requester := range requesters {
			for j := 0; j < sessionsPerRequester; j++ {
				wg.Add(1)
				go func(bs *bitswap.Bitswap) {
					defer wg.Done()

					session := bs.NewSession(ctx)
					targetCID := cids[rand.Intn(len(cids))]

					requestStart := time.Now()
					atomic.AddInt64(&requestCount, 1)

					_, err := session.GetBlock(ctx, targetCID)
					latency := time.Since(requestStart)

					if err == nil {
						atomic.AddInt64(&successCount, 1)
						atomic.AddInt64((*int64)(&totalLatency), int64(latency))
					}
				}(requester.Exchange)
			}
		}

		wg.Wait()
		duration := time.Since(startTime)

		// Report metrics
		if requestCount > 0 {
			avgLatency := time.Duration(int64(totalLatency) / requestCount)
			successRate := float64(successCount) / float64(requestCount)
			throughput := float64(requestCount) / duration.Seconds()

			b.ReportMetric(throughput, "req/sec")
			b.ReportMetric(float64(avgLatency.Nanoseconds()), "avg-latency-ns")
			b.ReportMetric(successRate*100, "success-rate-%")
			b.ReportMetric(float64(connectionCount), "connections")
		}
	}
}

func benchmarkRequestThroughput(b *testing.B, targetRPS int) {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	net := tn.VirtualNetwork(delay.Fixed(5 * time.Millisecond))
	router := mockrouting.NewServer()
	ig := testinstance.NewTestInstanceGenerator(net, router, nil, nil)
	defer ig.Close()

	instances := ig.Instances(20)
	blocks := random.BlocksOfSize(2000, 4096) // Smaller blocks for higher throughput

	seeds := instances[:10]
	requesters := instances[10:]

	for _, seed := range seeds {
		if err := seed.Blockstore.PutMany(ctx, blocks); err != nil {
			b.Fatalf("Failed to put blocks: %v", err)
		}
	}

	var cids []cid.Cid
	for _, blk := range blocks {
		cids = append(cids, blk.Cid())
	}

	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		var requestCount, successCount int64
		var wg sync.WaitGroup

		startTime := time.Now()
		testDuration := 30 * time.Second
		requestInterval := time.Second / time.Duration(targetRPS)

		// Rate-limited request generation
		ticker := time.NewTicker(requestInterval)
		defer ticker.Stop()

		testCtx, testCancel := context.WithTimeout(ctx, testDuration)
		defer testCancel()

		// Request generators
		for _, requester := range requesters {
			wg.Add(1)
			go func(bs *bitswap.Bitswap) {
				defer wg.Done()

				session := bs.NewSession(testCtx)

				for {
					select {
					case <-testCtx.Done():
						return
					case <-ticker.C:
						go func() {
							targetCID := cids[rand.Intn(len(cids))]
							atomic.AddInt64(&requestCount, 1)

							_, err := session.GetBlock(testCtx, targetCID)
							if err == nil {
								atomic.AddInt64(&successCount, 1)
							}
						}()
					}
				}
			}(requester.Exchange)
		}

		wg.Wait()
		actualDuration := time.Since(startTime)

		actualRPS := float64(requestCount) / actualDuration.Seconds()
		successRate := float64(successCount) / float64(requestCount)

		b.ReportMetric(actualRPS, "actual-rps")
		b.ReportMetric(float64(targetRPS), "target-rps")
		b.ReportMetric(successRate*100, "success-rate-%")
		b.ReportMetric(actualRPS/float64(targetRPS)*100, "throughput-efficiency-%")
	}
}

func benchmarkLatencyUnderLoad(b *testing.B, nodeCount, concurrency, requestRate int) {
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
	defer cancel()

	net := tn.VirtualNetwork(delay.Fixed(10 * time.Millisecond))
	router := mockrouting.NewServer()
	ig := testinstance.NewTestInstanceGenerator(net, router, nil, nil)
	defer ig.Close()

	instances := ig.Instances(nodeCount)
	blocks := random.BlocksOfSize(1000, 8192)

	seedCount := nodeCount / 2
	seeds := instances[:seedCount]
	requesters := instances[seedCount:]

	for _, seed := range seeds {
		if err := seed.Blockstore.PutMany(ctx, blocks); err != nil {
			b.Fatalf("Failed to put blocks: %v", err)
		}
	}

	var cids []cid.Cid
	for _, blk := range blocks {
		cids = append(cids, blk.Cid())
	}

	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		latencies := make([]time.Duration, 0, requestRate*60) // Estimate for 1 minute
		var latencyMutex sync.Mutex
		var wg sync.WaitGroup

		testDuration := 60 * time.Second
		testCtx, testCancel := context.WithTimeout(ctx, testDuration)
		defer testCancel()

		// Generate load
		for _, requester := range requesters {
			for j := 0; j < concurrency/len(requesters); j++ {
				wg.Add(1)
				go func(bs *bitswap.Bitswap) {
					defer wg.Done()

					session := bs.NewSession(testCtx)
					ticker := time.NewTicker(time.Second / time.Duration(requestRate/concurrency))
					defer ticker.Stop()

					for {
						select {
						case <-testCtx.Done():
							return
						case <-ticker.C:
							targetCID := cids[rand.Intn(len(cids))]

							requestStart := time.Now()
							_, err := session.GetBlock(testCtx, targetCID)
							latency := time.Since(requestStart)

							if err == nil {
								latencyMutex.Lock()
								latencies = append(latencies, latency)
								latencyMutex.Unlock()
							}
						}
					}
				}(requester.Exchange)
			}
		}

		wg.Wait()

		// Calculate latency statistics
		if len(latencies) > 0 {
			avgLatency := calculateAverageLatency(latencies)
			p50Latency := calculatePercentileLatency(latencies, 0.50)
			p95Latency := calculatePercentileLatency(latencies, 0.95)
			p99Latency := calculatePercentileLatency(latencies, 0.99)
			maxLatency := calculateMaxLatency(latencies)

			b.ReportMetric(float64(avgLatency.Nanoseconds()), "avg-latency-ns")
			b.ReportMetric(float64(p50Latency.Nanoseconds()), "p50-latency-ns")
			b.ReportMetric(float64(p95Latency.Nanoseconds()), "p95-latency-ns")
			b.ReportMetric(float64(p99Latency.Nanoseconds()), "p99-latency-ns")
			b.ReportMetric(float64(maxLatency.Nanoseconds()), "max-latency-ns")
			b.ReportMetric(float64(len(latencies)), "successful-requests")

			// Check if P95 latency meets requirement
			p95RequirementMet := p95Latency <= 100*time.Millisecond
			if p95RequirementMet {
				b.ReportMetric(1, "p95-requirement-met")
			} else {
				b.ReportMetric(0, "p95-requirement-met")
			}
		}
	}
}

func benchmarkMemoryEfficiency(b *testing.B, blockSize, numBlocks, sessions int) {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	net := tn.VirtualNetwork(delay.Fixed(10 * time.Millisecond))
	router := mockrouting.NewServer()
	ig := testinstance.NewTestInstanceGenerator(net, router, nil, nil)
	defer ig.Close()

	instances := ig.Instances(10)
	blocks := random.BlocksOfSize(numBlocks, blockSize)

	seeds := instances[:5]
	requesters := instances[5:]

	for _, seed := range seeds {
		if err := seed.Blockstore.PutMany(ctx, blocks); err != nil {
			b.Fatalf("Failed to put blocks: %v", err)
		}
	}

	var cids []cid.Cid
	for _, blk := range blocks {
		cids = append(cids, blk.Cid())
	}

	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		// Measure initial memory
		var initialMemStats, finalMemStats runtime.MemStats
		runtime.GC()
		runtime.ReadMemStats(&initialMemStats)

		var requestCount, successCount int64
		var wg sync.WaitGroup

		// Create sessions and perform requests
		for _, requester := range requesters {
			for j := 0; j < sessions/len(requesters); j++ {
				wg.Add(1)
				go func(bs *bitswap.Bitswap) {
					defer wg.Done()

					session := bs.NewSession(ctx)

					// Perform multiple requests per session
					for k := 0; k < 10; k++ {
						targetCID := cids[rand.Intn(len(cids))]
						atomic.AddInt64(&requestCount, 1)

						_, err := session.GetBlock(ctx, targetCID)
						if err == nil {
							atomic.AddInt64(&successCount, 1)
						}
					}
				}(requester.Exchange)
			}
		}

		wg.Wait()

		// Measure final memory
		runtime.GC()
		runtime.ReadMemStats(&finalMemStats)

		// Calculate memory metrics
		initialMemMB := float64(initialMemStats.Alloc) / 1024 / 1024
		finalMemMB := float64(finalMemStats.Alloc) / 1024 / 1024
		memoryUsedMB := finalMemMB - initialMemMB

		if requestCount > 0 {
			memoryPerRequest := memoryUsedMB / float64(requestCount) * 1024 // KB per request
			successRate := float64(successCount) / float64(requestCount)

			b.ReportMetric(memoryUsedMB, "memory-used-mb")
			b.ReportMetric(memoryPerRequest, "memory-per-request-kb")
			b.ReportMetric(successRate*100, "success-rate-%")
			b.ReportMetric(float64(blockSize), "block-size-bytes")
			b.ReportMetric(float64(sessions), "concurrent-sessions")
		}
	}
}

func benchmarkNetworkConditions(b *testing.B, networkDelay delay.D) {
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
	defer cancel()

	net := tn.VirtualNetwork(networkDelay)
	router := mockrouting.NewServer()
	ig := testinstance.NewTestInstanceGenerator(net, router, nil, nil)
	defer ig.Close()

	instances := ig.Instances(15)
	blocks := random.BlocksOfSize(500, 8192)

	seeds := instances[:7]
	requesters := instances[7:]

	for _, seed := range seeds {
		if err := seed.Blockstore.PutMany(ctx, blocks); err != nil {
			b.Fatalf("Failed to put blocks: %v", err)
		}
	}

	var cids []cid.Cid
	for _, blk := range blocks {
		cids = append(cids, blk.Cid())
	}

	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		var requestCount, successCount int64
		var totalLatency time.Duration
		var wg sync.WaitGroup

		startTime := time.Now()
		testDuration := 60 * time.Second
		testCtx, testCancel := context.WithTimeout(ctx, testDuration)
		defer testCancel()

		// Generate requests under network conditions
		for _, requester := range requesters {
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

						requestStart := time.Now()
						atomic.AddInt64(&requestCount, 1)

						_, err := session.GetBlock(testCtx, targetCID)
						latency := time.Since(requestStart)

						if err == nil {
							atomic.AddInt64(&successCount, 1)
							atomic.AddInt64((*int64)(&totalLatency), int64(latency))
						}
					}
				}
			}(requester.Exchange)
		}

		wg.Wait()
		actualDuration := time.Since(startTime)

		if requestCount > 0 {
			avgLatency := time.Duration(int64(totalLatency) / requestCount)
			successRate := float64(successCount) / float64(requestCount)
			throughput := float64(requestCount) / actualDuration.Seconds()

			b.ReportMetric(throughput, "req/sec")
			b.ReportMetric(float64(avgLatency.Nanoseconds()), "avg-latency-ns")
			b.ReportMetric(successRate*100, "success-rate-%")
		}
	}
}

func benchmarkMaxNodes(b *testing.B) {
	nodeCounts := []int{50, 100, 200, 300, 500}

	for _, nodeCount := range nodeCounts {
		b.Run(fmt.Sprintf("Nodes_%d", nodeCount), func(b *testing.B) {
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
			defer cancel()

			net := tn.VirtualNetwork(delay.Fixed(10 * time.Millisecond))
			router := mockrouting.NewServer()
			ig := testinstance.NewTestInstanceGenerator(net, router, nil, nil)
			defer ig.Close()

			// Try to create the specified number of nodes
			startTime := time.Now()
			var instances []testinstance.Instance
			var creationError error

			func() {
				defer func() {
					if r := recover(); r != nil {
						creationError = fmt.Errorf("panic during node creation: %v", r)
					}
				}()
				instances = ig.Instances(nodeCount)
			}()

			creationDuration := time.Since(startTime)

			if creationError != nil {
				b.Logf("Failed to create %d nodes: %v", nodeCount, creationError)
				b.ReportMetric(0, "creation-success")
				return
			}

			b.ReportMetric(1, "creation-success")
			b.ReportMetric(creationDuration.Seconds(), "creation-time-sec")
			b.ReportMetric(float64(len(instances)), "nodes-created")

			// Test basic functionality
			if len(instances) > 0 {
				blocks := random.BlocksOfSize(100, 8192)
				seed := instances[0]

				if err := seed.Blockstore.PutMany(ctx, blocks); err == nil {
					b.ReportMetric(1, "basic-functionality")
				} else {
					b.ReportMetric(0, "basic-functionality")
				}
			}
		})
	}
}

func benchmarkMaxConcurrentRequests(b *testing.B) {
	requestCounts := []int{1000, 5000, 10000, 25000, 50000}

	for _, requestCount := range requestCounts {
		b.Run(fmt.Sprintf("Requests_%d", requestCount), func(b *testing.B) {
			ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
			defer cancel()

			net := tn.VirtualNetwork(delay.Fixed(5 * time.Millisecond))
			router := mockrouting.NewServer()
			ig := testinstance.NewTestInstanceGenerator(net, router, nil, nil)
			defer ig.Close()

			instances := ig.Instances(20)
			blocks := random.BlocksOfSize(1000, 4096)

			seeds := instances[:10]
			requesters := instances[10:]

			for _, seed := range seeds {
				if err := seed.Blockstore.PutMany(ctx, blocks); err != nil {
					b.Fatalf("Failed to put blocks: %v", err)
				}
			}

			var cids []cid.Cid
			for _, blk := range blocks {
				cids = append(cids, blk.Cid())
			}

			var successCount int64
			var wg sync.WaitGroup

			startTime := time.Now()

			// Launch concurrent requests
			for i := 0; i < requestCount; i++ {
				wg.Add(1)
				go func() {
					defer wg.Done()

					requester := requesters[rand.Intn(len(requesters))]
					session := requester.Exchange.NewSession(ctx)
					targetCID := cids[rand.Intn(len(cids))]

					_, err := session.GetBlock(ctx, targetCID)
					if err == nil {
						atomic.AddInt64(&successCount, 1)
					}
				}()
			}

			wg.Wait()
			duration := time.Since(startTime)

			successRate := float64(successCount) / float64(requestCount)
			throughput := float64(requestCount) / duration.Seconds()

			b.ReportMetric(throughput, "req/sec")
			b.ReportMetric(successRate*100, "success-rate-%")
			b.ReportMetric(float64(requestCount), "concurrent-requests")
			b.ReportMetric(duration.Seconds(), "completion-time-sec")
		})
	}
}

func benchmarkMaxBlockSize(b *testing.B) {
	blockSizes := []int{1024, 8192, 65536, 262144, 1048576, 4194304} // 1KB to 4MB

	for _, blockSize := range blockSizes {
		b.Run(fmt.Sprintf("BlockSize_%dKB", blockSize/1024), func(b *testing.B) {
			ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
			defer cancel()

			net := tn.VirtualNetwork(delay.Fixed(10 * time.Millisecond))
			router := mockrouting.NewServer()
			ig := testinstance.NewTestInstanceGenerator(net, router, nil, nil)
			defer ig.Close()

			instances := ig.Instances(10)
			blocks := random.BlocksOfSize(50, blockSize) // Fewer blocks for larger sizes

			seeds := instances[:5]
			requesters := instances[5:]

			for _, seed := range seeds {
				if err := seed.Blockstore.PutMany(ctx, blocks); err != nil {
					b.Logf("Failed to put blocks of size %d: %v", blockSize, err)
					b.ReportMetric(0, "put-success")
					return
				}
			}

			b.ReportMetric(1, "put-success")

			var cids []cid.Cid
			for _, blk := range blocks {
				cids = append(cids, blk.Cid())
			}

			var successCount int64
			var totalLatency time.Duration
			var wg sync.WaitGroup

			// Test retrieval performance
			for _, requester := range requesters {
				wg.Add(1)
				go func(bs *bitswap.Bitswap) {
					defer wg.Done()

					session := bs.NewSession(ctx)

					for _, targetCID := range cids {
						requestStart := time.Now()
						_, err := session.GetBlock(ctx, targetCID)
						latency := time.Since(requestStart)

						if err == nil {
							atomic.AddInt64(&successCount, 1)
							atomic.AddInt64((*int64)(&totalLatency), int64(latency))
						}
					}
				}(requester.Exchange)
			}

			wg.Wait()

			if successCount > 0 {
				avgLatency := time.Duration(int64(totalLatency) / successCount)
				successRate := float64(successCount) / float64(len(cids)*len(requesters))

				b.ReportMetric(float64(avgLatency.Nanoseconds()), "avg-latency-ns")
				b.ReportMetric(successRate*100, "success-rate-%")
				b.ReportMetric(float64(blockSize), "block-size-bytes")
				b.ReportMetric(float64(successCount), "successful-retrievals")
			}
		})
	}
}

// Helper function to find minimum of two integers
func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
