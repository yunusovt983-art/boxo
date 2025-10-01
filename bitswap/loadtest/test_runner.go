package loadtest

import (
	"encoding/json"
	"fmt"
	"os"
	"testing"
	"time"

	delay "github.com/ipfs/go-ipfs-delay"
)

// TestBitswapLoadTestSuite runs the complete load testing suite
// Requirements: 1.1, 1.2 - Comprehensive load testing validation
func TestBitswapLoadTestSuite(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping comprehensive load test suite in short mode")
	}

	// Load configuration
	config := loadTestSuiteConfig()

	// Validate environment
	if err := ValidateEnvironment(); err != nil {
		t.Fatalf("Environment validation failed: %v", err)
	}

	// Initialize test suite result
	result := &TestSuiteResult{
		StartTime:        time.Now(),
		TestResults:      make(map[string]*LoadTestResult),
		StressResults:    make(map[string]*StressTestResult),
		BenchmarkResults: make(map[string]interface{}),
		FailedTests:      make([]string, 0),
		SystemInfo:       getSystemInfo(),
		Configuration:    config,
	}

	// Create output directory
	if err := os.MkdirAll(config.OutputDir, 0755); err != nil {
		t.Fatalf("Failed to create output directory: %v", err)
	}

	// Run high concurrency test
	t.Run("HighConcurrency", func(t *testing.T) {
		testConfig := LoadTestConfig{
			NumNodes:           100,
			NumBlocks:          1000,
			ConcurrentRequests: config.MaxConcurrentConnections,
			TestDuration:       5 * time.Minute,
			BlockSize:          8192,
			NetworkDelay:       delay.Fixed(10 * time.Millisecond),
			RequestRate:        2000,
		}

		testResult := runLoadTest(t, testConfig)
		result.TestResults["HighConcurrency"] = testResult

		// Validate requirements
		if !validateHighConcurrencyRequirements(t, testResult, config) {
			result.FailedTests = append(result.FailedTests, "HighConcurrency")
		}

		// Save individual test result
		if err := saveTestResults(testResult, "high_concurrency"); err != nil {
			t.Logf("Failed to save test results: %v", err)
		}
	})

	// Run high throughput test
	t.Run("HighThroughput", func(t *testing.T) {
		testConfig := LoadTestConfig{
			NumNodes:           50,
			NumBlocks:          5000,
			ConcurrentRequests: config.MaxRequestRate / 10,
			TestDuration:       2 * time.Minute,
			BlockSize:          4096,
			NetworkDelay:       delay.Fixed(5 * time.Millisecond),
			RequestRate:        config.MaxRequestRate,
		}

		testResult := runLoadTest(t, testConfig)
		result.TestResults["HighThroughput"] = testResult

		// Validate requirements
		if !validateHighThroughputRequirements(t, testResult, config) {
			result.FailedTests = append(result.FailedTests, "HighThroughput")
		}

		// Save individual test result
		if err := saveTestResults(testResult, "high_throughput"); err != nil {
			t.Logf("Failed to save test results: %v", err)
		}
	})

	// Run stability test if enabled
	if config.EnableLongRunning {
		t.Run("LongRunningStability", func(t *testing.T) {
			testConfig := LoadTestConfig{
				NumNodes:           20,
				NumBlocks:          2000,
				ConcurrentRequests: 1000,
				TestDuration:       config.StabilityDuration,
				BlockSize:          8192,
				NetworkDelay:       delay.Fixed(20 * time.Millisecond),
				RequestRate:        500,
			}

			testResult := runLoadTest(t, testConfig)
			result.TestResults["LongRunningStability"] = testResult

			// Validate requirements
			if !validateStabilityRequirements(t, testResult, config) {
				result.FailedTests = append(result.FailedTests, "LongRunningStability")
			}

			// Save individual test result
			if err := saveTestResults(testResult, "stability"); err != nil {
				t.Logf("Failed to save test results: %v", err)
			}
		})
	}

	// Run stress tests if enabled
	if config.EnableStressTests {
		stressScenarios := []StressTestConfig{
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
				TestDuration:       3 * time.Minute,
				BlockSize:          8192,
				NetworkDelay:       delay.Fixed(500 * time.Millisecond),
				RequestRate:        500,
			},
		}

		for _, stressConfig := range stressScenarios {
			t.Run("Stress_"+stressConfig.Name, func(t *testing.T) {
				// Convert LoadTestConfig to StressTestConfig for compatibility
				stressTestConfig := StressTestConfig{
					Name:               stressConfig.Name,
					NumNodes:           stressConfig.NumNodes,
					NumBlocks:          stressConfig.NumBlocks,
					ConcurrentRequests: stressConfig.ConcurrentRequests,
					TestDuration:       stressConfig.TestDuration,
					BlockSize:          stressConfig.BlockSize,
					NetworkDelay:       stressConfig.NetworkDelay,
					RequestRate:        stressConfig.RequestRate,
					BurstInterval:      stressConfig.BurstInterval,
					BurstMultiplier:    stressConfig.BurstMultiplier,
				}

				stressResult := runStressTest(t, stressTestConfig)
				result.StressResults["Stress_"+stressConfig.Name] = stressResult

				// Validate stress test requirements
				if !validateStressTestRequirements(t, stressResult, config) {
					result.FailedTests = append(result.FailedTests, "Stress_"+stressConfig.Name)
				}
			})
		}
	}

	// Complete test suite
	result.EndTime = time.Now()
	result.Duration = result.EndTime.Sub(result.StartTime)
	result.OverallSuccess = len(result.FailedTests) == 0

	// Generate reports if enabled
	if config.GenerateReports {
		if err := generateTestReport(result, config.OutputDir); err != nil {
			t.Logf("Failed to generate test report: %v", err)
		}
	}

	// Save complete test suite results
	if err := saveTestSuiteResults(result, config.OutputDir); err != nil {
		t.Logf("Failed to save test suite results: %v", err)
	}

	// Final validation
	if !result.OverallSuccess {
		t.Errorf("Test suite failed. Failed tests: %v", result.FailedTests)
	}

	// Log summary
	t.Logf("Load Test Suite Completed:")
	t.Logf("  Duration: %v", result.Duration)
	t.Logf("  Tests Run: %d", len(result.TestResults)+len(result.StressResults))
	t.Logf("  Failed Tests: %d", len(result.FailedTests))
	t.Logf("  Overall Success: %v", result.OverallSuccess)
}

// loadTestSuiteConfig loads configuration from file or returns defaults
func loadTestSuiteConfig() TestSuiteConfig {
	config := TestSuiteConfig{
		TestTimeout:              30 * time.Minute,
		BenchmarkTimeout:         10 * time.Minute,
		StabilityDuration:        30 * time.Minute,
		MaxP95Latency:            100 * time.Millisecond,
		MinSuccessRate:           0.95,
		MaxMemoryGrowth:          0.05,
		MinThroughput:            1000,
		MaxConcurrentConnections: 10000,
		MaxRequestRate:           100000,
		EnableLongRunning:        false,
		EnableStressTests:        true,
		EnableBenchmarks:         true,
		OutputDir:                "test_results",
		GenerateReports:          true,
	}

	// Try to load from file
	if data, err := os.ReadFile("bitswap_load_test_config.json"); err == nil {
		if err := json.Unmarshal(data, &config); err != nil {
			// Use defaults if config file is invalid
		}
	}

	// Override with environment variables
	if duration := getEnvDuration("BITSWAP_STABILITY_DURATION"); duration > 0 {
		config.StabilityDuration = duration
		config.EnableLongRunning = true
	}

	if outputDir := os.Getenv("OUTPUT_DIR"); outputDir != "" {
		config.OutputDir = outputDir
	}

	return config
}

// Validation functions for different test types
func validateHighConcurrencyRequirements(t *testing.T, result *LoadTestResult, config TestSuiteConfig) bool {
	success := true

	if result.AverageLatency > config.MaxP95Latency {
		t.Errorf("Average latency %v exceeds %v requirement", result.AverageLatency, config.MaxP95Latency)
		success = false
	}

	if result.P95Latency > config.MaxP95Latency {
		t.Errorf("P95 latency %v exceeds %v requirement", result.P95Latency, config.MaxP95Latency)
		success = false
	}

	successRate := float64(result.SuccessfulRequests) / float64(result.TotalRequests)
	if successRate < config.MinSuccessRate {
		t.Errorf("Success rate %.2f%% is below %.2f%% requirement", successRate*100, config.MinSuccessRate*100)
		success = false
	}

	return success
}

func validateHighThroughputRequirements(t *testing.T, result *LoadTestResult, config TestSuiteConfig) bool {
	success := true

	if result.RequestsPerSecond < config.MinThroughput {
		t.Errorf("Throughput %.2f req/sec is below %.2f requirement", result.RequestsPerSecond, config.MinThroughput)
		success = false
	}

	// More lenient latency requirements for high throughput
	maxLatency := config.MaxP95Latency * 2
	if result.P95Latency > maxLatency {
		t.Errorf("P95 latency %v exceeds %v under high load", result.P95Latency, maxLatency)
		success = false
	}

	return success
}

func validateStabilityRequirements(t *testing.T, result *LoadTestResult, config TestSuiteConfig) bool {
	success := true

	// For stability tests, we need to check memory growth
	// This would require additional tracking during the test
	// For now, we validate basic success rate

	successRate := float64(result.SuccessfulRequests) / float64(result.TotalRequests)
	if successRate < 0.98 { // Higher requirement for stability
		t.Errorf("Stability success rate %.2f%% is below 98%% requirement", successRate*100)
		success = false
	}

	return success
}

func validateStressTestRequirements(t *testing.T, result *StressTestResult, config TestSuiteConfig) bool {
	success := true

	successRate := float64(result.SuccessfulRequests) / float64(result.TotalRequests)
	if successRate < 0.90 { // Lower requirement for stress tests
		t.Errorf("Stress test success rate %.2f%% is below 90%% requirement", successRate*100)
		success = false
	}

	if result.MemoryGrowthRate > 0.10 { // Allow 10% growth under stress
		t.Errorf("Memory growth rate %.2f%% exceeds 10%% limit", result.MemoryGrowthRate*100)
		success = false
	}

	return success
}

// generateTestReport generates a human-readable test report
func generateTestReport(result *TestSuiteResult, outputDir string) error {
	filename := fmt.Sprintf("%s/test_summary.txt", outputDir)
	file, err := os.Create(filename)
	if err != nil {
		return err
	}
	defer file.Close()

	fmt.Fprintf(file, "Bitswap Load Test Suite Report\n")
	fmt.Fprintf(file, "==============================\n\n")
	fmt.Fprintf(file, "Test Execution Summary:\n")
	fmt.Fprintf(file, "  Start Time: %s\n", result.StartTime.Format(time.RFC3339))
	fmt.Fprintf(file, "  End Time: %s\n", result.EndTime.Format(time.RFC3339))
	fmt.Fprintf(file, "  Duration: %v\n", result.Duration)
	fmt.Fprintf(file, "  Overall Success: %v\n\n", result.OverallSuccess)

	fmt.Fprintf(file, "System Information:\n")
	fmt.Fprintf(file, "  Go Version: %s\n", result.SystemInfo.GoVersion)
	fmt.Fprintf(file, "  GOMAXPROCS: %d\n", result.SystemInfo.GOMAXPROCS)
	fmt.Fprintf(file, "  CPU Cores: %d\n", result.SystemInfo.NumCPU)
	fmt.Fprintf(file, "  Memory: %d MB\n", result.SystemInfo.MemoryMB)
	fmt.Fprintf(file, "  OS/Arch: %s/%s\n\n", result.SystemInfo.OS, result.SystemInfo.Arch)

	fmt.Fprintf(file, "Test Results:\n")
	for testName, testResult := range result.TestResults {
		fmt.Fprintf(file, "  %s:\n", testName)
		fmt.Fprintf(file, "    Total Requests: %d\n", testResult.TotalRequests)
		fmt.Fprintf(file, "    Success Rate: %.2f%%\n", float64(testResult.SuccessfulRequests)/float64(testResult.TotalRequests)*100)
		fmt.Fprintf(file, "    Requests/sec: %.2f\n", testResult.RequestsPerSecond)
		fmt.Fprintf(file, "    Average Latency: %v\n", testResult.AverageLatency)
		fmt.Fprintf(file, "    P95 Latency: %v\n", testResult.P95Latency)
		fmt.Fprintf(file, "    Memory Usage: %.2f MB\n", testResult.MemoryUsageMB)
		fmt.Fprintf(file, "    Goroutines: %d\n\n", testResult.GoroutineCount)
	}

	if len(result.StressResults) > 0 {
		fmt.Fprintf(file, "Stress Test Results:\n")
		for testName, stressResult := range result.StressResults {
			fmt.Fprintf(file, "  %s:\n", testName)
			fmt.Fprintf(file, "    Success Rate: %.2f%%\n", float64(stressResult.SuccessfulRequests)/float64(stressResult.TotalRequests)*100)
			fmt.Fprintf(file, "    Peak Memory: %.2f MB\n", stressResult.PeakMemoryMB)
			fmt.Fprintf(file, "    Memory Growth: %.2f%%\n", stressResult.MemoryGrowthRate*100)
			fmt.Fprintf(file, "    Recovery Time: %v\n\n", stressResult.RecoveryTime)
		}
	}

	if len(result.FailedTests) > 0 {
		fmt.Fprintf(file, "Failed Tests:\n")
		for _, failedTest := range result.FailedTests {
			fmt.Fprintf(file, "  - %s\n", failedTest)
		}
		fmt.Fprintf(file, "\n")
	}

	fmt.Fprintf(file, "Configuration Used:\n")
	fmt.Fprintf(file, "  Max P95 Latency: %v\n", result.Configuration.MaxP95Latency)
	fmt.Fprintf(file, "  Min Success Rate: %.2f%%\n", result.Configuration.MinSuccessRate*100)
	fmt.Fprintf(file, "  Max Memory Growth: %.2f%%\n", result.Configuration.MaxMemoryGrowth*100)
	fmt.Fprintf(file, "  Min Throughput: %.2f req/sec\n", result.Configuration.MinThroughput)
	fmt.Fprintf(file, "  Max Concurrent Connections: %d\n", result.Configuration.MaxConcurrentConnections)
	fmt.Fprintf(file, "  Max Request Rate: %d req/sec\n", result.Configuration.MaxRequestRate)

	return nil
}

// saveTestSuiteResults saves the complete test suite results
func saveTestSuiteResults(result *TestSuiteResult, outputDir string) error {
	filename := fmt.Sprintf("%s/load_test_suite_results.json", outputDir)
	file, err := os.Create(filename)
	if err != nil {
		return err
	}
	defer file.Close()

	encoder := json.NewEncoder(file)
	encoder.SetIndent("", "  ")
	return encoder.Encode(result)
}
