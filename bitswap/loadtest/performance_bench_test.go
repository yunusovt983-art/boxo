package loadtest

import (
	"context"
	"runtime"
	"testing"
	"time"

	delay "github.com/ipfs/go-ipfs-delay"
)

// BenchmarkBitswapMemoryEfficiency tests memory usage under various loads
// Requirements: 1.1, 1.2 - Memory efficiency validation
func BenchmarkBitswapMemoryEfficiency(b *testing.B) {
	scenarios := []struct {
		name      string
		blockSize int
		numBlocks int
	}{
		{"SmallBlocks_1KB", 1024, 10000},
		{"MediumBlocks_8KB", 8192, 5000},
		{"LargeBlocks_64KB", 65536, 1000},
		{"XLargeBlocks_1MB", 1048576, 100},
	}

	for _, scenario := range scenarios {
		b.Run(scenario.name, func(b *testing.B) {
			config := LoadTestConfig{
				NumNodes:           10,
				NumBlocks:          scenario.numBlocks,
				ConcurrentRequests: 1000,
				TestDuration:       30 * time.Second,
				BlockSize:          scenario.blockSize,
				NetworkDelay:       delay.Fixed(10 * time.Millisecond),
				RequestRate:        500,
			}

			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				result := runLoadTest(b, config)

				// Calculate memory efficiency (requests per MB)
				efficiency := float64(result.SuccessfulRequests) / result.MemoryUsageMB
				b.ReportMetric(efficiency, "req/mb")
				b.ReportMetric(result.MemoryUsageMB, "memory-mb")
				b.ReportMetric(float64(result.TotalRequests), "total-requests")
				b.ReportMetric(float64(result.SuccessfulRequests), "successful-requests")
			}
		})
	}
}

// BenchmarkBitswapConnectionScaling tests performance scaling with connection count
// Requirements: 1.1, 1.2 - Connection scaling validation
func BenchmarkBitswapConnectionScaling(b *testing.B) {
	scenarios := []struct {
		name        string
		connections int
		nodes       int
		expectedRPS float64
	}{
		{"Scale_100Conn", 100, 10, 500},
		{"Scale_500Conn", 500, 15, 1000},
		{"Scale_1000Conn", 1000, 20, 1500},
		{"Scale_2500Conn", 2500, 30, 2000},
		{"Scale_5000Conn", 5000, 50, 2500},
		{"Scale_10000Conn", 10000, 100, 3000},
	}

	for _, scenario := range scenarios {
		b.Run(scenario.name, func(b *testing.B) {
			config := LoadTestConfig{
				NumNodes:           scenario.nodes,
				NumBlocks:          1000,
				ConcurrentRequests: scenario.connections,
				TestDuration:       45 * time.Second,
				BlockSize:          8192,
				NetworkDelay:       delay.Fixed(10 * time.Millisecond),
				RequestRate:        scenario.connections / 5, // 20% of connections active
			}

			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				result := runLoadTest(b, config)

				// Report scaling metrics
				b.ReportMetric(float64(result.RequestsPerSecond), "req/sec")
				b.ReportMetric(float64(scenario.connections), "connections")
				b.ReportMetric(result.RequestsPerSecond/float64(scenario.connections), "req/sec/conn")
				b.ReportMetric(result.MemoryUsageMB/float64(scenario.connections)*1000, "mb/1000conn")
				b.ReportMetric(float64(result.GoroutineCount), "goroutines")

				// Validate scaling efficiency
				scalingEfficiency := result.RequestsPerSecond / scenario.expectedRPS
				b.ReportMetric(scalingEfficiency, "scaling-efficiency")

				if scalingEfficiency < 0.7 { // At least 70% of expected performance
					b.Errorf("Scaling efficiency %.2f is below 70%% for %d connections",
						scalingEfficiency, scenario.connections)
				}
			}
		})
	}
}

// BenchmarkBitswapThroughputScaling tests throughput scaling capabilities
// Requirements: 1.1, 1.2 - Throughput scaling validation
func BenchmarkBitswapThroughputScaling(b *testing.B) {
	scenarios := []struct {
		name           string
		targetRPS      int
		nodes          int
		concurrency    int
		duration       time.Duration
		minAchievement float64 // Minimum percentage of target to achieve
	}{
		{"Throughput_1K", 1000, 20, 200, 30 * time.Second, 0.8},
		{"Throughput_5K", 5000, 30, 500, 30 * time.Second, 0.7},
		{"Throughput_10K", 10000, 40, 1000, 25 * time.Second, 0.6},
		{"Throughput_25K", 25000, 50, 2500, 20 * time.Second, 0.5},
		{"Throughput_50K", 50000, 60, 5000, 15 * time.Second, 0.4},
		{"Throughput_100K", 100000, 80, 10000, 10 * time.Second, 0.3},
	}

	for _, scenario := range scenarios {
		b.Run(scenario.name, func(b *testing.B) {
			config := LoadTestConfig{
				NumNodes:           scenario.nodes,
				NumBlocks:          5000,
				ConcurrentRequests: scenario.concurrency,
				TestDuration:       scenario.duration,
				BlockSize:          4096, // Smaller blocks for higher throughput
				NetworkDelay:       delay.Fixed(5 * time.Millisecond),
				RequestRate:        scenario.targetRPS,
			}

			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				result := runLoadTest(b, config)

				// Report throughput metrics
				b.ReportMetric(float64(result.RequestsPerSecond), "req/sec")
				b.ReportMetric(float64(scenario.targetRPS), "target-req/sec")

				achievement := result.RequestsPerSecond / float64(scenario.targetRPS)
				b.ReportMetric(achievement, "achievement-ratio")

				b.ReportMetric(float64(result.P95Latency.Nanoseconds()), "p95-latency-ns")
				b.ReportMetric(result.MemoryUsageMB, "memory-mb")
				b.ReportMetric(float64(result.GoroutineCount), "goroutines")

				// Validate minimum achievement
				if achievement < scenario.minAchievement {
					b.Errorf("Throughput achievement %.2f%% is below minimum %.2f%% for target %d req/sec",
						achievement*100, scenario.minAchievement*100, scenario.targetRPS)
				}
			}
		})
	}
}

// BenchmarkBitswapLatencyDistribution analyzes latency distribution under load
// Requirements: 1.1, 1.2 - Latency distribution analysis
func BenchmarkBitswapLatencyDistribution(b *testing.B) {
	scenarios := []struct {
		name     string
		load     int
		duration time.Duration
	}{
		{"Latency_LightLoad", 500, 60 * time.Second},
		{"Latency_ModerateLoad", 2000, 45 * time.Second},
		{"Latency_HighLoad", 5000, 30 * time.Second},
		{"Latency_VeryHighLoad", 10000, 20 * time.Second},
	}

	for _, scenario := range scenarios {
		b.Run(scenario.name, func(b *testing.B) {
			config := LoadTestConfig{
				NumNodes:           30,
				NumBlocks:          2000,
				ConcurrentRequests: scenario.load / 5,
				TestDuration:       scenario.duration,
				BlockSize:          8192,
				NetworkDelay:       delay.Fixed(10 * time.Millisecond),
				RequestRate:        scenario.load,
			}

			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				result := runDetailedLatencyTest(b, config)

				// Report detailed latency metrics
				b.ReportMetric(float64(result.AverageLatency.Nanoseconds()), "avg-latency-ns")
				b.ReportMetric(float64(result.P50Latency.Nanoseconds()), "p50-latency-ns")
				b.ReportMetric(float64(result.P95Latency.Nanoseconds()), "p95-latency-ns")
				b.ReportMetric(float64(result.P99Latency.Nanoseconds()), "p99-latency-ns")
				b.ReportMetric(float64(result.MaxLatency.Nanoseconds()), "max-latency-ns")

				// Calculate latency spread
				latencySpread := float64(result.P99Latency.Nanoseconds() - result.P50Latency.Nanoseconds())
				b.ReportMetric(latencySpread, "latency-spread-ns")

				b.ReportMetric(float64(result.RequestsPerSecond), "req/sec")
			}
		})
	}
}

// BenchmarkBitswapResourceUtilization measures resource utilization efficiency
// Requirements: 1.1, 1.2 - Resource utilization analysis
func BenchmarkBitswapResourceUtilization(b *testing.B) {
	scenarios := []struct {
		name        string
		nodes       int
		concurrency int
		duration    time.Duration
	}{
		{"Resource_Small", 10, 500, 30 * time.Second},
		{"Resource_Medium", 25, 1500, 30 * time.Second},
		{"Resource_Large", 50, 3000, 30 * time.Second},
		{"Resource_XLarge", 100, 6000, 30 * time.Second},
	}

	for _, scenario := range scenarios {
		b.Run(scenario.name, func(b *testing.B) {
			config := LoadTestConfig{
				NumNodes:           scenario.nodes,
				NumBlocks:          2000,
				ConcurrentRequests: scenario.concurrency,
				TestDuration:       scenario.duration,
				BlockSize:          8192,
				NetworkDelay:       delay.Fixed(10 * time.Millisecond),
				RequestRate:        scenario.concurrency / 2,
			}

			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				result := runResourceUtilizationTest(b, config)

				// Report resource utilization metrics
				b.ReportMetric(float64(result.RequestsPerSecond), "req/sec")
				b.ReportMetric(result.MemoryUsageMB, "memory-mb")
				b.ReportMetric(float64(result.GoroutineCount), "goroutines")
				b.ReportMetric(float64(result.ConnectionCount), "connections")

				// Calculate efficiency metrics
				reqPerMB := result.RequestsPerSecond / result.MemoryUsageMB
				reqPerGoroutine := result.RequestsPerSecond / float64(result.GoroutineCount)
				reqPerConnection := result.RequestsPerSecond / float64(result.ConnectionCount)

				b.ReportMetric(reqPerMB, "req/sec/mb")
				b.ReportMetric(reqPerGoroutine, "req/sec/goroutine")
				b.ReportMetric(reqPerConnection, "req/sec/connection")

				// CPU utilization approximation
				cpuEfficiency := result.RequestsPerSecond / float64(runtime.NumCPU()) / 1000
				b.ReportMetric(cpuEfficiency, "kreq/sec/cpu")
			}
		})
	}
}

// BenchmarkBitswapNetworkEfficiency tests network utilization efficiency
// Requirements: 1.1, 1.2 - Network efficiency analysis
func BenchmarkBitswapNetworkEfficiency(b *testing.B) {
	scenarios := []struct {
		name         string
		networkDelay delay.D
		bandwidth    int // Simulated bandwidth limit
		blockSize    int
	}{
		{"Network_LowLatency_HighBW", delay.Fixed(1 * time.Millisecond), 1000000, 8192},
		{"Network_LAN_MediumBW", delay.Fixed(10 * time.Millisecond), 100000, 8192},
		{"Network_WAN_LowBW", delay.Fixed(50 * time.Millisecond), 10000, 4096},
		{"Network_HighLatency_VeryLowBW", delay.Fixed(200 * time.Millisecond), 1000, 2048},
	}

	for _, scenario := range scenarios {
		b.Run(scenario.name, func(b *testing.B) {
			config := LoadTestConfig{
				NumNodes:           20,
				NumBlocks:          1000,
				ConcurrentRequests: 1000,
				TestDuration:       45 * time.Second,
				BlockSize:          scenario.blockSize,
				NetworkDelay:       scenario.networkDelay,
				RequestRate:        500,
			}

			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				result := runLoadTest(b, config)

				// Calculate network efficiency metrics
				totalBytes := float64(result.SuccessfulRequests * int64(scenario.blockSize))
				bytesPerSecond := totalBytes / result.TestDuration.Seconds()
				networkUtilization := bytesPerSecond / float64(scenario.bandwidth)

				b.ReportMetric(float64(result.RequestsPerSecond), "req/sec")
				b.ReportMetric(bytesPerSecond, "bytes/sec")
				b.ReportMetric(networkUtilization, "network-utilization")
				b.ReportMetric(float64(result.AverageLatency.Nanoseconds()), "avg-latency-ns")
				b.ReportMetric(float64(result.P95Latency.Nanoseconds()), "p95-latency-ns")

				// Efficiency relative to network conditions
				latencyEfficiency := 1.0 / (float64(result.AverageLatency.Nanoseconds()) / 1000000) // Inverse of latency in ms
				b.ReportMetric(latencyEfficiency, "latency-efficiency")
			}
		})
	}
}

// Extended LoadTestResult for detailed analysis
type DetailedLoadTestResult struct {
	*LoadTestResult
	P50Latency        time.Duration
	LatencyStdDev     time.Duration
	ThroughputStdDev  float64
	ResourcePeakUsage ResourcePeakUsage
}

type ResourcePeakUsage struct {
	PeakMemoryMB    float64
	PeakGoroutines  int
	PeakConnections int
	CPUUtilization  float64
}

// runDetailedLatencyTest runs a test with detailed latency analysis
func runDetailedLatencyTest(b *testing.B, config LoadTestConfig) *DetailedLoadTestResult {
	// Run the base load test
	baseResult := runLoadTest(b, config)

	// For this implementation, we'll extend with P50 calculation
	// In a full implementation, you'd collect all latency samples
	result := &DetailedLoadTestResult{
		LoadTestResult: baseResult,
		P50Latency:     baseResult.AverageLatency, // Simplified - would calculate actual P50
	}

	return result
}

// runResourceUtilizationTest runs a test with detailed resource monitoring
func runResourceUtilizationTest(b *testing.B, config LoadTestConfig) *DetailedLoadTestResult {
	// Monitor resources during test
	resourceMonitor := &ResourceMonitor{
		interval: 5 * time.Second,
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go resourceMonitor.Start(ctx)

	// Run the base load test
	baseResult := runLoadTest(b, config)

	resourceMonitor.Stop()

	// Get peak resource usage
	finalStats := resourceMonitor.GetFinalStats()

	result := &DetailedLoadTestResult{
		LoadTestResult: baseResult,
		ResourcePeakUsage: ResourcePeakUsage{
			PeakMemoryMB:    finalStats.MemoryMB,
			PeakGoroutines:  finalStats.GoroutineCount,
			PeakConnections: baseResult.ConnectionCount,
			CPUUtilization:  0.0, // Would need actual CPU monitoring
		},
	}

	return result
}

// BenchmarkBitswapStabilityUnderLoad tests stability metrics under sustained load
// Requirements: 1.1, 1.2 - Stability under sustained load
func BenchmarkBitswapStabilityUnderLoad(b *testing.B) {
	scenarios := []struct {
		name     string
		duration time.Duration
		load     int
	}{
		{"Stability_5min", 5 * time.Minute, 1000},
		{"Stability_10min", 10 * time.Minute, 800},
		{"Stability_15min", 15 * time.Minute, 600},
		{"Stability_30min", 30 * time.Minute, 400},
	}

	for _, scenario := range scenarios {
		b.Run(scenario.name, func(b *testing.B) {
			config := LoadTestConfig{
				NumNodes:           25,
				NumBlocks:          2000,
				ConcurrentRequests: scenario.load,
				TestDuration:       scenario.duration,
				BlockSize:          8192,
				NetworkDelay:       delay.Fixed(15 * time.Millisecond),
				RequestRate:        scenario.load / 2,
			}

			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				result := runStabilityTest(b, config)

				// Report stability metrics
				b.ReportMetric(float64(result.RequestsPerSecond), "req/sec")
				b.ReportMetric(result.MemoryGrowthRate*100, "memory-growth-pct")
				b.ReportMetric(float64(result.P95Latency.Nanoseconds()), "p95-latency-ns")
				b.ReportMetric(float64(result.SuccessfulRequests)/float64(result.TotalRequests)*100, "success-rate-pct")

				// Stability score (higher is better)
				stabilityScore := (float64(result.SuccessfulRequests) / float64(result.TotalRequests)) *
					(1.0 - result.MemoryGrowthRate) *
					(100000000.0 / float64(result.P95Latency.Nanoseconds())) // Inverse of latency
				b.ReportMetric(stabilityScore, "stability-score")
			}
		})
	}
}

// runStabilityTest runs a test with stability monitoring
func runStabilityTest(b *testing.B, config LoadTestConfig) *StressTestResult {
	// Memory tracking for stability
	memoryTracker := &MemoryTracker{
		interval: 30 * time.Second,
		samples:  make([]float64, 0),
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go memoryTracker.Start(ctx)

	// Run the base load test
	baseResult := runLoadTest(b, config)

	memoryTracker.Stop()

	// Calculate stability metrics
	result := &StressTestResult{
		LoadTestResult:   baseResult,
		MemoryGrowthRate: memoryTracker.GetGrowthRate(),
		PeakMemoryMB:     memoryTracker.GetPeakMemory(),
	}

	return result
}
