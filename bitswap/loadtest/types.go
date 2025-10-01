package loadtest

import (
	"time"

	delay "github.com/ipfs/go-ipfs-delay"
)

// LoadTestConfig defines configuration for load tests
type LoadTestConfig struct {
	NumNodes           int
	NumBlocks          int
	ConcurrentRequests int
	TestDuration       time.Duration
	BlockSize          int
	NetworkDelay       delay.D
	RequestRate        int // requests per second
}

// LoadTestResult contains metrics from load test execution
type LoadTestResult struct {
	TotalRequests      int64
	SuccessfulRequests int64
	FailedRequests     int64
	AverageLatency     time.Duration
	P95Latency         time.Duration
	P99Latency         time.Duration
	MaxLatency         time.Duration
	RequestsPerSecond  float64
	TestDuration       time.Duration
	MemoryUsageMB      float64
	GoroutineCount     int
	ConnectionCount    int
	DuplicateBlocks    int64
	NetworkErrors      int64
}

// StressTestConfig defines configuration for stress tests
type StressTestConfig struct {
	Name               string
	NumNodes           int
	NumBlocks          int
	ConcurrentRequests int
	TestDuration       time.Duration
	BlockSize          int
	NetworkDelay       delay.D
	RequestRate        int
	FailureRate        float64 // Percentage of requests that should fail
	BurstInterval      time.Duration
	BurstMultiplier    int
}

// StressTestResult contains metrics from stress test execution
type StressTestResult struct {
	*LoadTestResult
	PeakMemoryMB        float64
	MemoryGrowthRate    float64
	GoroutineLeakCount  int
	RecoveryTime        time.Duration
	FailureRecoveryRate float64
}

// TestSuiteConfig defines configuration for the complete test suite
type TestSuiteConfig struct {
	TestTimeout              time.Duration `json:"test_timeout"`
	BenchmarkTimeout         time.Duration `json:"benchmark_timeout"`
	StabilityDuration        time.Duration `json:"stability_duration"`
	MaxP95Latency            time.Duration `json:"max_p95_latency"`
	MinSuccessRate           float64       `json:"min_success_rate"`
	MaxMemoryGrowth          float64       `json:"max_memory_growth"`
	MinThroughput            float64       `json:"min_throughput"`
	MaxConcurrentConnections int           `json:"max_concurrent_connections"`
	MaxRequestRate           int           `json:"max_request_rate"`
	EnableLongRunning        bool          `json:"enable_long_running"`
	EnableStressTests        bool          `json:"enable_stress_tests"`
	EnableBenchmarks         bool          `json:"enable_benchmarks"`
	OutputDir                string        `json:"output_dir"`
	GenerateReports          bool          `json:"generate_reports"`
}

// TestSuiteResult aggregates results from all tests
type TestSuiteResult struct {
	StartTime        time.Time                    `json:"start_time"`
	EndTime          time.Time                    `json:"end_time"`
	Duration         time.Duration                `json:"duration"`
	TestResults      map[string]*LoadTestResult   `json:"test_results"`
	StressResults    map[string]*StressTestResult `json:"stress_results"`
	BenchmarkResults map[string]interface{}       `json:"benchmark_results"`
	OverallSuccess   bool                         `json:"overall_success"`
	FailedTests      []string                     `json:"failed_tests"`
	SystemInfo       SystemInfo                   `json:"system_info"`
	Configuration    TestSuiteConfig              `json:"configuration"`
}

// SystemInfo captures system information for test context
type SystemInfo struct {
	GoVersion  string `json:"go_version"`
	GOMAXPROCS int    `json:"gomaxprocs"`
	NumCPU     int    `json:"num_cpu"`
	MemoryMB   int64  `json:"memory_mb"`
	OS         string `json:"os"`
	Arch       string `json:"arch"`
}
