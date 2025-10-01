package loadtest

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"testing"
	"time"

	delay "github.com/ipfs/go-ipfs-delay"
)

// TestBitswapHighConcurrency tests Bitswap with 10,000+ concurrent connections
// Requirements: 1.1, 1.2 - Handle thousands of concurrent requests with <100ms response time
func TestBitswapHighConcurrency(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping high concurrency test in short mode")
	}

	config := LoadTestConfig{
		NumNodes:           100,  // Simulate cluster environment
		NumBlocks:          1000, // Reasonable block count for concurrency test
		ConcurrentRequests: 10000,
		TestDuration:       5 * time.Minute,
		BlockSize:          8192, // 8KB blocks
		NetworkDelay:       delay.Fixed(10 * time.Millisecond),
		RequestRate:        2000, // 2K requests/sec per node
	}

	result := runLoadTest(t, config)

	// Validate requirements
	if result.AverageLatency > 100*time.Millisecond {
		t.Errorf("Average latency %v exceeds 100ms requirement", result.AverageLatency)
	}

	if result.P95Latency > 100*time.Millisecond {
		t.Errorf("P95 latency %v exceeds 100ms requirement", result.P95Latency)
	}

	if result.SuccessfulRequests < int64(float64(result.TotalRequests)*0.95) {
		t.Errorf("Success rate %.2f%% is below 95%% requirement",
			float64(result.SuccessfulRequests)/float64(result.TotalRequests)*100)
	}

	t.Logf("High Concurrency Test Results:")
	t.Logf("  Total Requests: %d", result.TotalRequests)
	t.Logf("  Success Rate: %.2f%%", float64(result.SuccessfulRequests)/float64(result.TotalRequests)*100)
	t.Logf("  Average Latency: %v", result.AverageLatency)
	t.Logf("  P95 Latency: %v", result.P95Latency)
	t.Logf("  Requests/sec: %.2f", result.RequestsPerSecond)
	t.Logf("  Memory Usage: %.2f MB", result.MemoryUsageMB)
	t.Logf("  Goroutines: %d", result.GoroutineCount)

	// Save test results
	if err := saveTestResults(result, "high_concurrency"); err != nil {
		t.Logf("Failed to save test results: %v", err)
	}
}

// TestBitswapHighThroughput tests handling 100,000 requests per second
// Requirements: 1.1, 1.2 - System should auto-scale connection pools for high load
func TestBitswapHighThroughput(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping high throughput test in short mode")
	}

	config := LoadTestConfig{
		NumNodes:           50,
		NumBlocks:          5000,
		ConcurrentRequests: 20000,
		TestDuration:       2 * time.Minute,
		BlockSize:          4096, // Smaller blocks for higher throughput
		NetworkDelay:       delay.Fixed(5 * time.Millisecond),
		RequestRate:        100000, // 100K requests/sec target
	}

	result := runLoadTest(t, config)

	// Validate throughput requirements
	if result.RequestsPerSecond < 50000 { // Allow some margin
		t.Errorf("Throughput %.2f req/sec is below 50K requirement", result.RequestsPerSecond)
	}

	if result.P95Latency > 200*time.Millisecond { // More lenient for high throughput
		t.Errorf("P95 latency %v exceeds 200ms under high load", result.P95Latency)
	}

	t.Logf("High Throughput Test Results:")
	t.Logf("  Throughput: %.2f req/sec", result.RequestsPerSecond)
	t.Logf("  Total Requests: %d", result.TotalRequests)
	t.Logf("  P95 Latency: %v", result.P95Latency)
	t.Logf("  Memory Usage: %.2f MB", result.MemoryUsageMB)

	// Save test results
	if err := saveTestResults(result, "high_throughput"); err != nil {
		t.Logf("Failed to save test results: %v", err)
	}
}

// TestBitswapStabilityLongRunning tests system stability over 24+ hours
// Requirements: 1.1, 1.2 - System should maintain performance over extended periods
func TestBitswapStabilityLongRunning(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping long-running stability test in short mode")
	}

	// For CI/testing, we'll run a shorter version (30 minutes)
	// In production, this should be configured for 24+ hours
	testDuration := 30 * time.Minute
	if duration := getEnvDuration("BITSWAP_STABILITY_DURATION"); duration > 0 {
		testDuration = duration
	}

	config := LoadTestConfig{
		NumNodes:           20,
		NumBlocks:          2000,
		ConcurrentRequests: 1000,
		TestDuration:       testDuration,
		BlockSize:          8192,
		NetworkDelay:       delay.Fixed(20 * time.Millisecond),
		RequestRate:        500, // Moderate sustained load
	}

	// Track memory usage over time
	memoryTracker := &MemoryTracker{
		interval: 1 * time.Minute,
		samples:  make([]float64, 0),
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go memoryTracker.Start(ctx)

	result := runLoadTest(t, config)

	memoryTracker.Stop()

	// Validate stability requirements
	memoryGrowth := memoryTracker.GetGrowthRate()
	if memoryGrowth > 0.05 { // More than 5% growth indicates potential leak
		t.Errorf("Memory growth rate %.2f%% indicates potential memory leak", memoryGrowth*100)
	}

	if result.SuccessfulRequests < int64(float64(result.TotalRequests)*0.98) {
		t.Errorf("Success rate %.2f%% degraded during long run",
			float64(result.SuccessfulRequests)/float64(result.TotalRequests)*100)
	}

	t.Logf("Stability Test Results (Duration: %v):", testDuration)
	t.Logf("  Memory Growth Rate: %.2f%%", memoryGrowth*100)
	t.Logf("  Final Memory Usage: %.2f MB", result.MemoryUsageMB)
	t.Logf("  Success Rate: %.2f%%", float64(result.SuccessfulRequests)/float64(result.TotalRequests)*100)
	t.Logf("  Average Latency: %v", result.AverageLatency)

	// Save test results
	if err := saveTestResults(result, "stability"); err != nil {
		t.Logf("Failed to save test results: %v", err)
	}
}

// TestBitswapExtremeLoad tests system behavior under extreme load conditions
// Requirements: 1.1, 1.2 - System should handle extreme load gracefully
func TestBitswapExtremeLoad(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping extreme load test in short mode")
	}

	scenarios := []struct {
		name   string
		config LoadTestConfig
	}{
		{
			name: "ExtremeConnections",
			config: LoadTestConfig{
				NumNodes:           200,
				NumBlocks:          1000,
				ConcurrentRequests: 50000, // 50K concurrent connections
				TestDuration:       3 * time.Minute,
				BlockSize:          4096,
				NetworkDelay:       delay.Fixed(5 * time.Millisecond),
				RequestRate:        10000,
			},
		},
		{
			name: "ExtremeThroughput",
			config: LoadTestConfig{
				NumNodes:           100,
				NumBlocks:          10000,
				ConcurrentRequests: 25000,
				TestDuration:       2 * time.Minute,
				BlockSize:          2048, // Very small blocks for max throughput
				NetworkDelay:       delay.Fixed(1 * time.Millisecond),
				RequestRate:        200000, // 200K requests/sec target
			},
		},
		{
			name: "ExtremeDuration",
			config: LoadTestConfig{
				NumNodes:           30,
				NumBlocks:          3000,
				ConcurrentRequests: 5000,
				TestDuration:       2 * time.Hour, // 2 hour test
				BlockSize:          8192,
				NetworkDelay:       delay.Fixed(15 * time.Millisecond),
				RequestRate:        1000,
			},
		},
	}

	for _, scenario := range scenarios {
		t.Run(scenario.name, func(t *testing.T) {
			// Skip extreme duration test unless explicitly enabled
			if scenario.name == "ExtremeDuration" && getEnvDuration("BITSWAP_EXTREME_DURATION") == 0 {
				t.Skip("Skipping extreme duration test - set BITSWAP_EXTREME_DURATION to enable")
			}

			result := runLoadTest(t, scenario.config)

			// Validate that system handled extreme load gracefully
			if result.SuccessfulRequests == 0 {
				t.Errorf("System completely failed under extreme load")
			}

			successRate := float64(result.SuccessfulRequests) / float64(result.TotalRequests)
			if successRate < 0.70 { // Allow lower success rate for extreme tests
				t.Errorf("Success rate %.2f%% is below 70%% minimum for extreme load", successRate*100)
			}

			t.Logf("Extreme Load Test %s Results:", scenario.name)
			t.Logf("  Total Requests: %d", result.TotalRequests)
			t.Logf("  Success Rate: %.2f%%", successRate*100)
			t.Logf("  Throughput: %.2f req/sec", result.RequestsPerSecond)
			t.Logf("  P95 Latency: %v", result.P95Latency)
			t.Logf("  Memory Usage: %.2f MB", result.MemoryUsageMB)
			t.Logf("  Goroutines: %d", result.GoroutineCount)

			// Save test results
			if err := saveTestResults(result, fmt.Sprintf("extreme_%s", scenario.name)); err != nil {
				t.Logf("Failed to save test results: %v", err)
			}
		})
	}
}

// saveTestResults saves test results to JSON file for analysis
func saveTestResults(result *LoadTestResult, testName string) error {
	outputDir := os.Getenv("OUTPUT_DIR")
	if outputDir == "" {
		outputDir = "test_results"
	}

	// Create output directory if it doesn't exist
	if err := os.MkdirAll(outputDir, 0755); err != nil {
		return err
	}

	// Create result with metadata
	resultWithMeta := struct {
		*LoadTestResult
		TestName   string     `json:"test_name"`
		Timestamp  time.Time  `json:"timestamp"`
		SystemInfo SystemInfo `json:"system_info"`
	}{
		LoadTestResult: result,
		TestName:       testName,
		Timestamp:      time.Now(),
		SystemInfo:     getSystemInfo(),
	}

	// Save to JSON file
	filename := fmt.Sprintf("%s/%s_results.json", outputDir, testName)
	file, err := os.Create(filename)
	if err != nil {
		return err
	}
	defer file.Close()

	encoder := json.NewEncoder(file)
	encoder.SetIndent("", "  ")
	return encoder.Encode(resultWithMeta)
}
