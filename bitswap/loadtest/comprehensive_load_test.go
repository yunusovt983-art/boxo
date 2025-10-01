package loadtest

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"runtime"
	"testing"
	"time"

	delay "github.com/ipfs/go-ipfs-delay"
)

// TestBitswapComprehensiveLoadSuite runs all comprehensive load tests for task 7.1
// Requirements: 1.1, 1.2 - Complete load testing suite for Bitswap
func TestBitswapComprehensiveLoadSuite(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping comprehensive load test suite in short mode")
	}

	// Validate environment before starting tests
	if err := ValidateEnvironment(); err != nil {
		t.Fatalf("Environment validation failed: %v", err)
	}

	// Load configuration
	config := loadTestSuiteConfig()

	// Create output directory
	if err := os.MkdirAll(config.OutputDir, 0755); err != nil {
		t.Fatalf("Failed to create output directory: %v", err)
	}

	t.Logf("Starting Comprehensive Bitswap Load Test Suite")
	t.Logf("System Info: %d CPUs, %d MB RAM, Go %s",
		runtime.NumCPU(),
		getSystemInfo().MemoryMB,
		runtime.Version())

	// Test 1: 10,000+ Concurrent Connections Test
	t.Run("Test_10K_ConcurrentConnections", func(t *testing.T) {
		testHighConcurrencyConnections(t, config)
	})

	// Test 2: 100,000 Requests Per Second Test
	t.Run("Test_100K_RequestsPerSecond", func(t *testing.T) {
		testHighThroughputRequests(t, config)
	})

	// Test 3: 24+ Hour Stability Test
	t.Run("Test_24Hour_Stability", func(t *testing.T) {
		testLongRunningStability(t, config)
	})

	// Test 4: Automated Performance Benchmarks
	t.Run("Test_Automated_Benchmarks", func(t *testing.T) {
		testAutomatedBenchmarks(t, config)
	})

	// Test 5: Extreme Load Scenarios
	t.Run("Test_Extreme_Load_Scenarios", func(t *testing.T) {
		testExtremeLoadScenarios(t, config)
	})

	// Test 6: Memory Leak Detection
	t.Run("Test_Memory_Leak_Detection", func(t *testing.T) {
		testMemoryLeakDetection(t, config)
	})

	// Test 7: Network Condition Variations
	t.Run("Test_Network_Conditions", func(t *testing.T) {
		testNetworkConditionVariations(t, config)
	})

	t.Logf("Comprehensive Load Test Suite Completed")
}

// testHighConcurrencyConnections tests 10,000+ concurrent connections
// Requirements: 1.1, 1.2 - Handle thousands of concurrent requests with <100ms response time
func testHighConcurrencyConnections(t *testing.T, config TestSuiteConfig) {
	t.Logf("Testing 10,000+ concurrent connections...")

	scenarios := []struct {
		name        string
		connections int
		nodes       int
		duration    time.Duration
	}{
		{"10K_Connections", 10000, 100, 5 * time.Minute},
		{"15K_Connections", 15000, 120, 4 * time.Minute},
		{"20K_Connections", 20000, 150, 3 * time.Minute},
		{"25K_Connections", 25000, 200, 2 * time.Minute},
	}

	for _, scenario := range scenarios {
		t.Run(scenario.name, func(t *testing.T) {
			testConfig := LoadTestConfig{
				NumNodes:           scenario.nodes,
				NumBlocks:          2000,
				ConcurrentRequests: scenario.connections,
				TestDuration:       scenario.duration,
				BlockSize:          8192,
				NetworkDelay:       delay.Fixed(10 * time.Millisecond),
				RequestRate:        scenario.connections / 10, // 10% active at any time
			}

			result := runLoadTest(t, testConfig)

			// Validate requirements
			if result.AverageLatency > 100*time.Millisecond {
				t.Errorf("Average latency %v exceeds 100ms requirement for %s",
					result.AverageLatency, scenario.name)
			}

			if result.P95Latency > 100*time.Millisecond {
				t.Errorf("P95 latency %v exceeds 100ms requirement for %s",
					result.P95Latency, scenario.name)
			}

			successRate := float64(result.SuccessfulRequests) / float64(result.TotalRequests)
			if successRate < 0.95 {
				t.Errorf("Success rate %.2f%% is below 95%% requirement for %s",
					successRate*100, scenario.name)
			}

			t.Logf("%s Results:", scenario.name)
			t.Logf("  Concurrent Connections: %d", scenario.connections)
			t.Logf("  Total Requests: %d", result.TotalRequests)
			t.Logf("  Success Rate: %.2f%%", successRate*100)
			t.Logf("  Average Latency: %v", result.AverageLatency)
			t.Logf("  P95 Latency: %v", result.P95Latency)
			t.Logf("  Requests/sec: %.2f", result.RequestsPerSecond)
			t.Logf("  Memory Usage: %.2f MB", result.MemoryUsageMB)

			// Save results
			if err := saveTestResults(result, fmt.Sprintf("concurrent_%s", scenario.name)); err != nil {
				t.Logf("Failed to save test results: %v", err)
			}
		})
	}
}

// testHighThroughputRequests tests 100,000 requests per second
// Requirements: 1.1, 1.2 - System should auto-scale connection pools for high load
func testHighThroughputRequests(t *testing.T, config TestSuiteConfig) {
	t.Logf("Testing 100,000+ requests per second...")

	scenarios := []struct {
		name           string
		targetRPS      int
		nodes          int
		concurrency    int
		duration       time.Duration
		minAchievement float64
	}{
		{"50K_RPS", 50000, 60, 5000, 3 * time.Minute, 0.8},
		{"75K_RPS", 75000, 80, 7500, 2 * time.Minute, 0.7},
		{"100K_RPS", 100000, 100, 10000, 90 * time.Second, 0.6},
		{"125K_RPS", 125000, 120, 12500, 60 * time.Second, 0.5},
		{"150K_RPS", 150000, 150, 15000, 45 * time.Second, 0.4},
	}

	for _, scenario := range scenarios {
		t.Run(scenario.name, func(t *testing.T) {
			testConfig := LoadTestConfig{
				NumNodes:           scenario.nodes,
				NumBlocks:          5000,
				ConcurrentRequests: scenario.concurrency,
				TestDuration:       scenario.duration,
				BlockSize:          4096, // Smaller blocks for higher throughput
				NetworkDelay:       delay.Fixed(5 * time.Millisecond),
				RequestRate:        scenario.targetRPS,
			}

			result := runLoadTest(t, testConfig)

			// Validate throughput requirements
			achievement := result.RequestsPerSecond / float64(scenario.targetRPS)
			if achievement < scenario.minAchievement {
				t.Errorf("Throughput achievement %.2f%% is below minimum %.2f%% for %s",
					achievement*100, scenario.minAchievement*100, scenario.name)
			}

			// More lenient latency for high throughput
			if result.P95Latency > 200*time.Millisecond {
				t.Errorf("P95 latency %v exceeds 200ms under high load for %s",
					result.P95Latency, scenario.name)
			}

			t.Logf("%s Results:", scenario.name)
			t.Logf("  Target RPS: %d", scenario.targetRPS)
			t.Logf("  Achieved RPS: %.2f", result.RequestsPerSecond)
			t.Logf("  Achievement: %.2f%%", achievement*100)
			t.Logf("  P95 Latency: %v", result.P95Latency)
			t.Logf("  Memory Usage: %.2f MB", result.MemoryUsageMB)

			// Save results
			if err := saveTestResults(result, fmt.Sprintf("throughput_%s", scenario.name)); err != nil {
				t.Logf("Failed to save test results: %v", err)
			}
		})
	}
}

// testLongRunningStability tests system stability over 24+ hours
// Requirements: 1.1, 1.2 - System should maintain performance over extended periods
func testLongRunningStability(t *testing.T, config TestSuiteConfig) {
	// Check if long running tests are enabled
	testDuration := config.StabilityDuration
	if !config.EnableLongRunning {
		testDuration = 30 * time.Minute // Shorter version for CI
		t.Logf("Long running tests disabled, using %v duration", testDuration)
	}

	// Allow override via environment
	if envDuration := getEnvDuration("BITSWAP_STABILITY_DURATION"); envDuration > 0 {
		testDuration = envDuration
	}

	t.Logf("Testing stability over %v...", testDuration)

	scenarios := []struct {
		name     string
		duration time.Duration
		load     int
		nodes    int
	}{
		{"Stability_Light_Load", testDuration, 500, 20},
		{"Stability_Medium_Load", testDuration / 2, 1000, 30},
		{"Stability_High_Load", testDuration / 4, 2000, 40},
	}

	for _, scenario := range scenarios {
		t.Run(scenario.name, func(t *testing.T) {
			testConfig := LoadTestConfig{
				NumNodes:           scenario.nodes,
				NumBlocks:          3000,
				ConcurrentRequests: scenario.load,
				TestDuration:       scenario.duration,
				BlockSize:          8192,
				NetworkDelay:       delay.Fixed(15 * time.Millisecond),
				RequestRate:        scenario.load / 2, // Sustained load
			}

			// Track memory usage over time
			memoryTracker := &MemoryTracker{
				interval: 1 * time.Minute,
				samples:  make([]float64, 0),
			}

			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()

			go memoryTracker.Start(ctx)

			result := runLoadTest(t, testConfig)

			memoryTracker.Stop()

			// Validate stability requirements
			memoryGrowth := memoryTracker.GetGrowthRate()
			if memoryGrowth > 0.05 { // More than 5% growth indicates potential leak
				t.Errorf("Memory growth rate %.2f%% indicates potential memory leak for %s",
					memoryGrowth*100, scenario.name)
			}

			successRate := float64(result.SuccessfulRequests) / float64(result.TotalRequests)
			if successRate < 0.98 { // Higher requirement for stability
				t.Errorf("Success rate %.2f%% degraded during long run for %s",
					successRate*100, scenario.name)
			}

			t.Logf("%s Results (Duration: %v):", scenario.name, scenario.duration)
			t.Logf("  Memory Growth Rate: %.2f%%", memoryGrowth*100)
			t.Logf("  Peak Memory: %.2f MB", memoryTracker.GetPeakMemory())
			t.Logf("  Final Memory Usage: %.2f MB", result.MemoryUsageMB)
			t.Logf("  Success Rate: %.2f%%", successRate*100)
			t.Logf("  Average Latency: %v", result.AverageLatency)

			// Save results
			if err := saveTestResults(result, fmt.Sprintf("stability_%s", scenario.name)); err != nil {
				t.Logf("Failed to save test results: %v", err)
			}
		})
	}
}

// testAutomatedBenchmarks runs automated performance benchmarks
// Requirements: 1.1, 1.2 - Automated benchmarks for performance validation
func testAutomatedBenchmarks(t *testing.T, config TestSuiteConfig) {
	if !config.EnableBenchmarks {
		t.Skip("Benchmarks disabled in configuration")
	}

	t.Logf("Running automated performance benchmarks...")

	benchmarks := []struct {
		name        string
		nodes       int
		blocks      int
		concurrency int
		duration    time.Duration
		blockSize   int
	}{
		{"Benchmark_Small_Scale", 10, 500, 100, 30 * time.Second, 4096},
		{"Benchmark_Medium_Scale", 25, 1000, 500, 45 * time.Second, 8192},
		{"Benchmark_Large_Scale", 50, 2000, 1000, 60 * time.Second, 16384},
		{"Benchmark_XLarge_Scale", 100, 5000, 2000, 90 * time.Second, 32768},
	}

	for _, benchmark := range benchmarks {
		t.Run(benchmark.name, func(t *testing.T) {
			testConfig := LoadTestConfig{
				NumNodes:           benchmark.nodes,
				NumBlocks:          benchmark.blocks,
				ConcurrentRequests: benchmark.concurrency,
				TestDuration:       benchmark.duration,
				BlockSize:          benchmark.blockSize,
				NetworkDelay:       delay.Fixed(10 * time.Millisecond),
				RequestRate:        benchmark.concurrency / 2,
			}

			result := runLoadTest(t, testConfig)

			// Calculate performance metrics
			efficiency := result.RequestsPerSecond / float64(benchmark.nodes)
			memoryEfficiency := result.RequestsPerSecond / result.MemoryUsageMB
			latencyScore := 1000.0 / float64(result.AverageLatency.Milliseconds())

			t.Logf("%s Results:", benchmark.name)
			t.Logf("  Requests/sec: %.2f", result.RequestsPerSecond)
			t.Logf("  Efficiency (req/sec/node): %.2f", efficiency)
			t.Logf("  Memory Efficiency (req/sec/MB): %.2f", memoryEfficiency)
			t.Logf("  Latency Score: %.2f", latencyScore)
			t.Logf("  P95 Latency: %v", result.P95Latency)
			t.Logf("  Memory Usage: %.2f MB", result.MemoryUsageMB)
			t.Logf("  Goroutines: %d", result.GoroutineCount)

			// Save benchmark results
			benchmarkResult := map[string]interface{}{
				"requests_per_second": result.RequestsPerSecond,
				"efficiency":          efficiency,
				"memory_efficiency":   memoryEfficiency,
				"latency_score":       latencyScore,
				"p95_latency_ms":      result.P95Latency.Milliseconds(),
				"memory_usage_mb":     result.MemoryUsageMB,
				"goroutine_count":     result.GoroutineCount,
				"success_rate":        float64(result.SuccessfulRequests) / float64(result.TotalRequests),
			}

			if err := saveBenchmarkResults(benchmarkResult, benchmark.name); err != nil {
				t.Logf("Failed to save benchmark results: %v", err)
			}
		})
	}
}

// testExtremeLoadScenarios tests system behavior under extreme conditions
// Requirements: 1.1, 1.2 - System should handle extreme load gracefully
func testExtremeLoadScenarios(t *testing.T, config TestSuiteConfig) {
	t.Logf("Testing extreme load scenarios...")

	scenarios := []struct {
		name        string
		description string
		testConfig  LoadTestConfig
		minSuccess  float64
	}{
		{
			name:        "Extreme_Connections_50K",
			description: "50,000 concurrent connections",
			testConfig: LoadTestConfig{
				NumNodes:           300,
				NumBlocks:          1000,
				ConcurrentRequests: 50000,
				TestDuration:       2 * time.Minute,
				BlockSize:          4096,
				NetworkDelay:       delay.Fixed(5 * time.Millisecond),
				RequestRate:        25000,
			},
			minSuccess: 0.70,
		},
		{
			name:        "Extreme_Throughput_200K",
			description: "200,000 requests per second",
			testConfig: LoadTestConfig{
				NumNodes:           200,
				NumBlocks:          10000,
				ConcurrentRequests: 20000,
				TestDuration:       90 * time.Second,
				BlockSize:          2048,
				NetworkDelay:       delay.Fixed(1 * time.Millisecond),
				RequestRate:        200000,
			},
			minSuccess: 0.60,
		},
		{
			name:        "Extreme_Block_Size_10MB",
			description: "10MB block sizes",
			testConfig: LoadTestConfig{
				NumNodes:           50,
				NumBlocks:          100,
				ConcurrentRequests: 1000,
				TestDuration:       3 * time.Minute,
				BlockSize:          10 * 1024 * 1024, // 10MB
				NetworkDelay:       delay.Fixed(20 * time.Millisecond),
				RequestRate:        100,
			},
			minSuccess: 0.80,
		},
		{
			name:        "Extreme_Many_Small_Blocks",
			description: "100,000 tiny blocks",
			testConfig: LoadTestConfig{
				NumNodes:           100,
				NumBlocks:          100000,
				ConcurrentRequests: 5000,
				TestDuration:       2 * time.Minute,
				BlockSize:          256, // 256 bytes
				NetworkDelay:       delay.Fixed(5 * time.Millisecond),
				RequestRate:        10000,
			},
			minSuccess: 0.75,
		},
	}

	for _, scenario := range scenarios {
		t.Run(scenario.name, func(t *testing.T) {
			t.Logf("Testing %s: %s", scenario.name, scenario.description)

			result := runLoadTest(t, scenario.testConfig)

			// Validate that system handled extreme load gracefully
			if result.SuccessfulRequests == 0 {
				t.Errorf("System completely failed under extreme load: %s", scenario.name)
			}

			successRate := float64(result.SuccessfulRequests) / float64(result.TotalRequests)
			if successRate < scenario.minSuccess {
				t.Errorf("Success rate %.2f%% is below minimum %.2f%% for %s",
					successRate*100, scenario.minSuccess*100, scenario.name)
			}

			t.Logf("%s Results:", scenario.name)
			t.Logf("  Total Requests: %d", result.TotalRequests)
			t.Logf("  Success Rate: %.2f%%", successRate*100)
			t.Logf("  Throughput: %.2f req/sec", result.RequestsPerSecond)
			t.Logf("  P95 Latency: %v", result.P95Latency)
			t.Logf("  Memory Usage: %.2f MB", result.MemoryUsageMB)
			t.Logf("  Goroutines: %d", result.GoroutineCount)

			// Save results
			if err := saveTestResults(result, fmt.Sprintf("extreme_%s", scenario.name)); err != nil {
				t.Logf("Failed to save test results: %v", err)
			}
		})
	}
}

// testMemoryLeakDetection tests for memory leaks during extended operation
// Requirements: 1.1, 1.2 - System should not have memory leaks
func testMemoryLeakDetection(t *testing.T, config TestSuiteConfig) {
	t.Logf("Testing memory leak detection...")

	scenarios := []struct {
		name     string
		duration time.Duration
		cycles   int
	}{
		{"Memory_Leak_Short_Cycles", 15 * time.Minute, 10},
		{"Memory_Leak_Long_Cycles", 30 * time.Minute, 5},
	}

	for _, scenario := range scenarios {
		t.Run(scenario.name, func(t *testing.T) {
			cycleDuration := scenario.duration / time.Duration(scenario.cycles)

			var initialMemory, finalMemory float64
			var memoryGrowthRates []float64

			for cycle := 0; cycle < scenario.cycles; cycle++ {
				t.Logf("Running memory leak detection cycle %d/%d", cycle+1, scenario.cycles)

				testConfig := LoadTestConfig{
					NumNodes:           30,
					NumBlocks:          1000,
					ConcurrentRequests: 2000,
					TestDuration:       cycleDuration,
					BlockSize:          8192,
					NetworkDelay:       delay.Fixed(10 * time.Millisecond),
					RequestRate:        1000,
				}

				// Track memory during this cycle
				memoryTracker := &MemoryTracker{
					interval: 30 * time.Second,
					samples:  make([]float64, 0),
				}

				ctx, cancel := context.WithCancel(context.Background())
				go memoryTracker.Start(ctx)

				result := runLoadTest(t, testConfig)

				cancel()
				memoryTracker.Stop()

				if cycle == 0 {
					initialMemory = result.MemoryUsageMB
				}
				finalMemory = result.MemoryUsageMB

				cycleGrowth := memoryTracker.GetGrowthRate()
				memoryGrowthRates = append(memoryGrowthRates, cycleGrowth)

				t.Logf("  Cycle %d: Memory %.2f MB, Growth %.2f%%",
					cycle+1, result.MemoryUsageMB, cycleGrowth*100)

				// Force garbage collection between cycles
				runtime.GC()
				runtime.GC()
				time.Sleep(5 * time.Second)
			}

			// Calculate overall memory growth
			overallGrowth := (finalMemory - initialMemory) / initialMemory

			// Calculate average cycle growth
			var avgCycleGrowth float64
			for _, growth := range memoryGrowthRates {
				avgCycleGrowth += growth
			}
			avgCycleGrowth /= float64(len(memoryGrowthRates))

			t.Logf("%s Results:", scenario.name)
			t.Logf("  Initial Memory: %.2f MB", initialMemory)
			t.Logf("  Final Memory: %.2f MB", finalMemory)
			t.Logf("  Overall Growth: %.2f%%", overallGrowth*100)
			t.Logf("  Average Cycle Growth: %.2f%%", avgCycleGrowth*100)

			// Validate memory leak requirements
			if overallGrowth > 0.10 { // More than 10% overall growth indicates leak
				t.Errorf("Overall memory growth %.2f%% indicates potential memory leak",
					overallGrowth*100)
			}

			if avgCycleGrowth > 0.05 { // More than 5% per cycle indicates leak
				t.Errorf("Average cycle memory growth %.2f%% indicates potential memory leak",
					avgCycleGrowth*100)
			}
		})
	}
}

// testNetworkConditionVariations tests performance under various network conditions
// Requirements: 1.1, 1.2 - System should perform well under various network conditions
func testNetworkConditionVariations(t *testing.T, config TestSuiteConfig) {
	t.Logf("Testing network condition variations...")

	scenarios := []struct {
		name         string
		networkDelay delay.D
		expectedRPS  float64
		description  string
	}{
		{"Network_Perfect_0ms", delay.Fixed(0), 8000, "Perfect network conditions"},
		{"Network_LAN_1ms", delay.Fixed(1 * time.Millisecond), 6000, "LAN conditions"},
		{"Network_FastWAN_10ms", delay.Fixed(10 * time.Millisecond), 3000, "Fast WAN conditions"},
		{"Network_SlowWAN_50ms", delay.Fixed(50 * time.Millisecond), 1000, "Slow WAN conditions"},
		{"Network_Satellite_200ms", delay.Fixed(200 * time.Millisecond), 200, "Satellite conditions"},
		{"Network_VeryPoor_500ms", delay.Fixed(500 * time.Millisecond), 50, "Very poor conditions"},
	}

	for _, scenario := range scenarios {
		t.Run(scenario.name, func(t *testing.T) {
			t.Logf("Testing %s: %s", scenario.name, scenario.description)

			testConfig := LoadTestConfig{
				NumNodes:           30,
				NumBlocks:          1500,
				ConcurrentRequests: 3000,
				TestDuration:       2 * time.Minute,
				BlockSize:          8192,
				NetworkDelay:       scenario.networkDelay,
				RequestRate:        2000,
			}

			result := runLoadTest(t, testConfig)

			// Validate performance relative to network conditions
			if result.RequestsPerSecond < scenario.expectedRPS*0.7 {
				t.Errorf("Performance %.2f req/sec is below expected %.2f for %s",
					result.RequestsPerSecond, scenario.expectedRPS*0.7, scenario.name)
			}

			t.Logf("%s Results:", scenario.name)
			t.Logf("  Network Delay: %v", scenario.networkDelay)
			t.Logf("  Expected RPS: %.2f", scenario.expectedRPS)
			t.Logf("  Achieved RPS: %.2f", result.RequestsPerSecond)
			t.Logf("  Performance Ratio: %.2f", result.RequestsPerSecond/scenario.expectedRPS)
			t.Logf("  Average Latency: %v", result.AverageLatency)
			t.Logf("  P95 Latency: %v", result.P95Latency)

			// Save results
			if err := saveTestResults(result, fmt.Sprintf("network_%s", scenario.name)); err != nil {
				t.Logf("Failed to save test results: %v", err)
			}
		})
	}
}

// saveBenchmarkResults saves benchmark results to JSON file
func saveBenchmarkResults(result map[string]interface{}, benchmarkName string) error {
	outputDir := os.Getenv("OUTPUT_DIR")
	if outputDir == "" {
		outputDir = "test_results"
	}

	// Create output directory if it doesn't exist
	if err := os.MkdirAll(outputDir, 0755); err != nil {
		return err
	}

	// Add metadata
	resultWithMeta := map[string]interface{}{
		"benchmark_name": benchmarkName,
		"timestamp":      time.Now(),
		"system_info":    getSystemInfo(),
		"results":        result,
	}

	// Save to JSON file
	filename := fmt.Sprintf("%s/benchmark_%s.json", outputDir, benchmarkName)
	file, err := os.Create(filename)
	if err != nil {
		return err
	}
	defer file.Close()

	encoder := json.NewEncoder(file)
	encoder.SetIndent("", "  ")
	return encoder.Encode(resultWithMeta)
}
