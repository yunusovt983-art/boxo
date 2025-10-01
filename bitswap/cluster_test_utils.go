package bitswap

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"math/rand"
	"os"
	"path/filepath"
	"strconv"
	"sync"
	"testing"
	"time"

	blocks "github.com/ipfs/go-block-format"
	"github.com/ipfs/go-cid"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/multiformats/go-multihash"
	"github.com/stretchr/testify/require"
)

// ClusterTestConfig represents configuration for a specific cluster test
type ClusterTestConfig struct {
	Name                string        `json:"name"`
	NodeCount           int           `json:"node_count"`
	BlockCount          int           `json:"block_count"`
	ConcurrentRequests  int           `json:"concurrent_requests"`
	TestDuration        time.Duration `json:"test_duration"`
	FailureRate         float64       `json:"failure_rate"`
	NetworkLatency      time.Duration `json:"network_latency"`
	MaxOutstandingBytes int64         `json:"max_outstanding_bytes"`
}

// ClusterTestConfigFile represents the structure of the test configuration file
type ClusterTestConfigFile struct {
	TestConfigurations map[string]ClusterTestConfig `json:"test_configurations"`
	AlertThresholds    map[string]interface{}       `json:"alert_thresholds"`
	PrometheusMetrics  PrometheusConfig             `json:"prometheus_metrics"`
	CICDSettings       CICDConfig                   `json:"ci_cd_settings"`
}

// PrometheusConfig defines Prometheus configuration
type PrometheusConfig struct {
	Enabled        bool   `json:"enabled"`
	Port           int    `json:"port"`
	Path           string `json:"path"`
	ScrapeInterval string `json:"scrape_interval"`
}

// CICDConfig defines CI/CD pipeline configuration
type CICDConfig struct {
	Timeout               string   `json:"timeout"`
	ParallelJobs          int      `json:"parallel_jobs"`
	RetryAttempts         int      `json:"retry_attempts"`
	ArtifactRetentionDays int      `json:"artifact_retention_days"`
	NotificationChannels  []string `json:"notification_channels"`
}

// DefaultClusterTestConfig returns a default cluster test configuration
func DefaultClusterTestConfig() ClusterTestConfig {
	return ClusterTestConfig{
		Name:                "default",
		NodeCount:           5,
		BlockCount:          100,
		ConcurrentRequests:  50,
		TestDuration:        30 * time.Second,
		FailureRate:         0.1,
		NetworkLatency:      50 * time.Millisecond,
		MaxOutstandingBytes: 1 << 20, // 1MB
	}
}

// TestResultsCollector collects and aggregates test results
type TestResultsCollector struct {
	TestName     string
	StartTime    time.Time
	EndTime      time.Time
	NodeResults  map[string]*NodeTestResult
	ClusterStats *ClusterTestStats
	Errors       []string
	Warnings     []string
	mu           sync.RWMutex
}

// NodeTestResult represents test results for a single node
type NodeTestResult struct {
	NodeID           string
	RequestsHandled  int64
	SuccessfulReqs   int64
	FailedReqs       int64
	AverageLatency   time.Duration
	P95Latency       time.Duration
	BytesTransferred int64
	AlertsGenerated  int
	IsHealthy        bool
	Errors           []string
}

// ClusterTestStats represents overall cluster test statistics
type ClusterTestStats struct {
	TotalNodes     int
	HealthyNodes   int
	TotalRequests  int64
	SuccessfulReqs int64
	FailedReqs     int64
	AverageLatency time.Duration
	P95Latency     time.Duration
	TotalAlerts    int
	ClusterHealth  float64
	ThroughputRPS  float64
	TestDuration   time.Duration
}

// LoadClusterTestConfig loads test configuration from file or environment
func LoadClusterTestConfig(configType string) (ClusterTestConfig, error) {
	// First try to load from environment variables
	if config, ok := loadConfigFromEnv(); ok {
		return config, nil
	}

	// Fall back to configuration file
	configFile := "cluster_test_config.json"
	if envFile := os.Getenv("CLUSTER_TEST_CONFIG_FILE"); envFile != "" {
		configFile = envFile
	}

	// Find the config file
	configPath, err := findConfigFile(configFile)
	if err != nil {
		return DefaultClusterTestConfig(), fmt.Errorf("config file not found: %v", err)
	}

	data, err := ioutil.ReadFile(configPath)
	if err != nil {
		return DefaultClusterTestConfig(), fmt.Errorf("failed to read config file: %v", err)
	}

	var configData ClusterTestConfigFile
	if err := json.Unmarshal(data, &configData); err != nil {
		return DefaultClusterTestConfig(), fmt.Errorf("failed to parse config file: %v", err)
	}

	config, exists := configData.TestConfigurations[configType]
	if !exists {
		return DefaultClusterTestConfig(), fmt.Errorf("configuration type '%s' not found", configType)
	}

	return config, nil
}

// loadConfigFromEnv loads configuration from environment variables
func loadConfigFromEnv() (ClusterTestConfig, bool) {
	nodeCountStr := os.Getenv("CLUSTER_NODE_COUNT")
	if nodeCountStr == "" {
		return ClusterTestConfig{}, false
	}

	nodeCount, err := strconv.Atoi(nodeCountStr)
	if err != nil {
		return ClusterTestConfig{}, false
	}

	config := DefaultClusterTestConfig()
	config.NodeCount = nodeCount

	if blockCountStr := os.Getenv("CLUSTER_BLOCK_COUNT"); blockCountStr != "" {
		if blockCount, err := strconv.Atoi(blockCountStr); err == nil {
			config.BlockCount = blockCount
		}
	}

	if concurrentReqsStr := os.Getenv("CLUSTER_CONCURRENT_REQUESTS"); concurrentReqsStr != "" {
		if concurrentReqs, err := strconv.Atoi(concurrentReqsStr); err == nil {
			config.ConcurrentRequests = concurrentReqs
		}
	}

	if testDurationStr := os.Getenv("CLUSTER_TEST_DURATION"); testDurationStr != "" {
		if testDuration, err := time.ParseDuration(testDurationStr); err == nil {
			config.TestDuration = testDuration
		}
	}

	if networkLatencyStr := os.Getenv("CLUSTER_NETWORK_LATENCY"); networkLatencyStr != "" {
		if networkLatency, err := time.ParseDuration(networkLatencyStr); err == nil {
			config.NetworkLatency = networkLatency
		}
	}

	if failureRateStr := os.Getenv("CLUSTER_FAILURE_RATE"); failureRateStr != "" {
		if failureRate, err := strconv.ParseFloat(failureRateStr, 64); err == nil {
			config.FailureRate = failureRate
		}
	}

	return config, true
}

// findConfigFile searches for the configuration file in common locations
func findConfigFile(filename string) (string, error) {
	searchPaths := []string{
		filename,
		filepath.Join(".", filename),
		filepath.Join("bitswap", filename),
		filepath.Join("..", filename),
		filepath.Join("..", "bitswap", filename),
	}

	for _, path := range searchPaths {
		if _, err := os.Stat(path); err == nil {
			return path, nil
		}
	}

	return "", fmt.Errorf("configuration file %s not found in any search path", filename)
}

// NewTestResultsCollector creates a new test results collector
func NewTestResultsCollector(testName string) *TestResultsCollector {
	return &TestResultsCollector{
		TestName:     testName,
		StartTime:    time.Now(),
		NodeResults:  make(map[string]*NodeTestResult),
		ClusterStats: &ClusterTestStats{},
		Errors:       make([]string, 0),
		Warnings:     make([]string, 0),
	}
}

// RecordNodeResult records test results for a specific node
func (trc *TestResultsCollector) RecordNodeResult(nodeID string, result *NodeTestResult) {
	trc.mu.Lock()
	defer trc.mu.Unlock()

	result.NodeID = nodeID
	trc.NodeResults[nodeID] = result
}

// AddError adds an error to the test results
func (trc *TestResultsCollector) AddError(err string) {
	trc.mu.Lock()
	defer trc.mu.Unlock()

	trc.Errors = append(trc.Errors, err)
}

// AddWarning adds a warning to the test results
func (trc *TestResultsCollector) AddWarning(warning string) {
	trc.mu.Lock()
	defer trc.mu.Unlock()

	trc.Warnings = append(trc.Warnings, warning)
}

// Finalize calculates final statistics and closes the collection
func (trc *TestResultsCollector) Finalize() {
	trc.mu.Lock()
	defer trc.mu.Unlock()

	trc.EndTime = time.Now()
	trc.ClusterStats.TestDuration = trc.EndTime.Sub(trc.StartTime)

	// Aggregate node results
	trc.ClusterStats.TotalNodes = len(trc.NodeResults)

	var totalLatency time.Duration
	var p95Latencies []time.Duration

	for _, nodeResult := range trc.NodeResults {
		trc.ClusterStats.TotalRequests += nodeResult.RequestsHandled
		trc.ClusterStats.SuccessfulReqs += nodeResult.SuccessfulReqs
		trc.ClusterStats.FailedReqs += nodeResult.FailedReqs
		trc.ClusterStats.TotalAlerts += nodeResult.AlertsGenerated

		if nodeResult.IsHealthy {
			trc.ClusterStats.HealthyNodes++
		}

		totalLatency += nodeResult.AverageLatency
		p95Latencies = append(p95Latencies, nodeResult.P95Latency)
	}

	// Calculate averages
	if trc.ClusterStats.TotalNodes > 0 {
		trc.ClusterStats.AverageLatency = totalLatency / time.Duration(trc.ClusterStats.TotalNodes)
		trc.ClusterStats.ClusterHealth = float64(trc.ClusterStats.HealthyNodes) / float64(trc.ClusterStats.TotalNodes)
	}

	// Calculate P95 latency (simplified - take max of node P95s)
	for _, p95 := range p95Latencies {
		if p95 > trc.ClusterStats.P95Latency {
			trc.ClusterStats.P95Latency = p95
		}
	}

	// Calculate throughput
	if trc.ClusterStats.TestDuration > 0 {
		trc.ClusterStats.ThroughputRPS = float64(trc.ClusterStats.TotalRequests) / trc.ClusterStats.TestDuration.Seconds()
	}
}

// GenerateReport generates a detailed test report
func (trc *TestResultsCollector) GenerateReport() string {
	trc.mu.RLock()
	defer trc.mu.RUnlock()

	report := fmt.Sprintf("# Cluster Integration Test Report: %s\n\n", trc.TestName)
	report += fmt.Sprintf("**Test Duration:** %v\n", trc.ClusterStats.TestDuration)
	report += fmt.Sprintf("**Start Time:** %v\n", trc.StartTime.Format(time.RFC3339))
	report += fmt.Sprintf("**End Time:** %v\n\n", trc.EndTime.Format(time.RFC3339))

	// Cluster-wide statistics
	report += "## Cluster Statistics\n\n"
	report += fmt.Sprintf("- **Total Nodes:** %d\n", trc.ClusterStats.TotalNodes)
	report += fmt.Sprintf("- **Healthy Nodes:** %d (%.1f%%)\n",
		trc.ClusterStats.HealthyNodes, trc.ClusterStats.ClusterHealth*100)
	report += fmt.Sprintf("- **Total Requests:** %d\n", trc.ClusterStats.TotalRequests)
	report += fmt.Sprintf("- **Successful Requests:** %d (%.1f%%)\n",
		trc.ClusterStats.SuccessfulReqs,
		float64(trc.ClusterStats.SuccessfulReqs)/float64(trc.ClusterStats.TotalRequests)*100)
	report += fmt.Sprintf("- **Failed Requests:** %d (%.1f%%)\n",
		trc.ClusterStats.FailedReqs,
		float64(trc.ClusterStats.FailedReqs)/float64(trc.ClusterStats.TotalRequests)*100)
	report += fmt.Sprintf("- **Average Latency:** %v\n", trc.ClusterStats.AverageLatency)
	report += fmt.Sprintf("- **P95 Latency:** %v\n", trc.ClusterStats.P95Latency)
	report += fmt.Sprintf("- **Throughput:** %.2f requests/second\n", trc.ClusterStats.ThroughputRPS)
	report += fmt.Sprintf("- **Total Alerts:** %d\n\n", trc.ClusterStats.TotalAlerts)

	// Node-specific results
	report += "## Node Results\n\n"
	for nodeID, result := range trc.NodeResults {
		report += fmt.Sprintf("### Node: %s\n", nodeID)
		report += fmt.Sprintf("- **Status:** %s\n", func() string {
			if result.IsHealthy {
				return "Healthy"
			} else {
				return "Unhealthy"
			}
		}())
		report += fmt.Sprintf("- **Requests Handled:** %d\n", result.RequestsHandled)
		report += fmt.Sprintf("- **Success Rate:** %.1f%%\n",
			float64(result.SuccessfulReqs)/float64(result.RequestsHandled)*100)
		report += fmt.Sprintf("- **Average Latency:** %v\n", result.AverageLatency)
		report += fmt.Sprintf("- **P95 Latency:** %v\n", result.P95Latency)
		report += fmt.Sprintf("- **Bytes Transferred:** %d\n", result.BytesTransferred)
		report += fmt.Sprintf("- **Alerts Generated:** %d\n", result.AlertsGenerated)

		if len(result.Errors) > 0 {
			report += "- **Errors:**\n"
			for _, err := range result.Errors {
				report += fmt.Sprintf("  - %s\n", err)
			}
		}
		report += "\n"
	}

	// Errors and warnings
	if len(trc.Errors) > 0 {
		report += "## Errors\n\n"
		for _, err := range trc.Errors {
			report += fmt.Sprintf("- %s\n", err)
		}
		report += "\n"
	}

	if len(trc.Warnings) > 0 {
		report += "## Warnings\n\n"
		for _, warning := range trc.Warnings {
			report += fmt.Sprintf("- %s\n", warning)
		}
		report += "\n"
	}

	return report
}

// SaveReport saves the test report to a file
func (trc *TestResultsCollector) SaveReport(filename string) error {
	report := trc.GenerateReport()
	return ioutil.WriteFile(filename, []byte(report), 0644)
}

// GenerateTestBlocks creates a set of test blocks with specified characteristics
func GenerateTestBlocks(t *testing.T, count int, sizeRange [2]int) []blocks.Block {
	testBlocks := make([]blocks.Block, count)

	for i := 0; i < count; i++ {
		// Generate random size within range
		size := sizeRange[0]
		if sizeRange[1] > sizeRange[0] {
			size += rand.Intn(sizeRange[1] - sizeRange[0])
		}

		// Generate random data
		data := make([]byte, size)
		rand.Read(data)

		// Add some identifiable content
		prefix := fmt.Sprintf("test-block-%d-", i)
		copy(data[:len(prefix)], prefix)

		// Create CID
		hash, err := multihash.Sum(data, multihash.SHA2_256, -1)
		require.NoError(t, err)

		c := cid.NewCidV1(cid.Raw, hash)
		block, err := blocks.NewBlockWithCid(data, c)
		require.NoError(t, err)

		testBlocks[i] = block
	}

	return testBlocks
}

// WaitForCondition waits for a condition to be met or timeout
func WaitForCondition(condition func() bool, timeout time.Duration, checkInterval time.Duration) bool {
	deadline := time.Now().Add(timeout)

	for time.Now().Before(deadline) {
		if condition() {
			return true
		}
		time.Sleep(checkInterval)
	}

	return false
}

// GenerateRandomPeerID generates a random peer ID for testing
func GenerateRandomPeerID() (peer.ID, error) {
	// Generate random bytes
	randomBytes := make([]byte, 32)
	rand.Read(randomBytes)

	// Create multihash
	hash, err := multihash.Sum(randomBytes, multihash.SHA2_256, -1)
	if err != nil {
		return "", err
	}

	// Create peer ID
	return peer.ID(hash), nil
}

// SimulateNetworkConditions simulates various network conditions
type NetworkConditionSimulator struct {
	BaseLatency    time.Duration
	JitterRange    time.Duration
	PacketLossRate float64
	BandwidthLimit int64 // bytes per second
	mu             sync.RWMutex
}

// NewNetworkConditionSimulator creates a new network condition simulator
func NewNetworkConditionSimulator(baseLatency time.Duration, jitterRange time.Duration, packetLossRate float64, bandwidthLimit int64) *NetworkConditionSimulator {
	return &NetworkConditionSimulator{
		BaseLatency:    baseLatency,
		JitterRange:    jitterRange,
		PacketLossRate: packetLossRate,
		BandwidthLimit: bandwidthLimit,
	}
}

// SimulateLatency returns a simulated latency with jitter
func (ncs *NetworkConditionSimulator) SimulateLatency() time.Duration {
	ncs.mu.RLock()
	defer ncs.mu.RUnlock()

	jitter := time.Duration(rand.Int63n(int64(ncs.JitterRange)))
	return ncs.BaseLatency + jitter
}

// ShouldDropPacket returns true if packet should be dropped based on loss rate
func (ncs *NetworkConditionSimulator) ShouldDropPacket() bool {
	ncs.mu.RLock()
	defer ncs.mu.RUnlock()

	return rand.Float64() < ncs.PacketLossRate
}

// CalculateTransferTime calculates transfer time based on bandwidth limit
func (ncs *NetworkConditionSimulator) CalculateTransferTime(dataSize int64) time.Duration {
	ncs.mu.RLock()
	defer ncs.mu.RUnlock()

	if ncs.BandwidthLimit <= 0 {
		return 0
	}

	return time.Duration(dataSize * int64(time.Second) / ncs.BandwidthLimit)
}

// UpdateConditions updates network conditions
func (ncs *NetworkConditionSimulator) UpdateConditions(baseLatency time.Duration, jitterRange time.Duration, packetLossRate float64, bandwidthLimit int64) {
	ncs.mu.Lock()
	defer ncs.mu.Unlock()

	ncs.BaseLatency = baseLatency
	ncs.JitterRange = jitterRange
	ncs.PacketLossRate = packetLossRate
	ncs.BandwidthLimit = bandwidthLimit
}

// ClusterTestValidator validates test results against expected criteria
type ClusterTestValidator struct {
	MinSuccessRate    float64
	MaxAverageLatency time.Duration
	MaxP95Latency     time.Duration
	MinThroughput     float64
	MinClusterHealth  float64
	MaxAlertRate      float64
}

// DefaultClusterTestValidator returns a validator with default criteria
func DefaultClusterTestValidator() *ClusterTestValidator {
	return &ClusterTestValidator{
		MinSuccessRate:    0.95, // 95%
		MaxAverageLatency: 200 * time.Millisecond,
		MaxP95Latency:     500 * time.Millisecond,
		MinThroughput:     10.0, // requests per second
		MinClusterHealth:  0.8,  // 80%
		MaxAlertRate:      0.1,  // 10%
	}
}

// ValidateResults validates test results against criteria
func (ctv *ClusterTestValidator) ValidateResults(stats *ClusterTestStats) []string {
	var violations []string

	// Check success rate
	successRate := float64(stats.SuccessfulReqs) / float64(stats.TotalRequests)
	if successRate < ctv.MinSuccessRate {
		violations = append(violations, fmt.Sprintf("Success rate %.2f%% below minimum %.2f%%",
			successRate*100, ctv.MinSuccessRate*100))
	}

	// Check average latency
	if stats.AverageLatency > ctv.MaxAverageLatency {
		violations = append(violations, fmt.Sprintf("Average latency %v exceeds maximum %v",
			stats.AverageLatency, ctv.MaxAverageLatency))
	}

	// Check P95 latency
	if stats.P95Latency > ctv.MaxP95Latency {
		violations = append(violations, fmt.Sprintf("P95 latency %v exceeds maximum %v",
			stats.P95Latency, ctv.MaxP95Latency))
	}

	// Check throughput
	if stats.ThroughputRPS < ctv.MinThroughput {
		violations = append(violations, fmt.Sprintf("Throughput %.2f RPS below minimum %.2f RPS",
			stats.ThroughputRPS, ctv.MinThroughput))
	}

	// Check cluster health
	if stats.ClusterHealth < ctv.MinClusterHealth {
		violations = append(violations, fmt.Sprintf("Cluster health %.2f%% below minimum %.2f%%",
			stats.ClusterHealth*100, ctv.MinClusterHealth*100))
	}

	// Check alert rate
	alertRate := float64(stats.TotalAlerts) / float64(stats.TotalRequests)
	if alertRate > ctv.MaxAlertRate {
		violations = append(violations, fmt.Sprintf("Alert rate %.2f%% exceeds maximum %.2f%%",
			alertRate*100, ctv.MaxAlertRate*100))
	}

	return violations
}

// AssertValidResults asserts that test results meet validation criteria
func (ctv *ClusterTestValidator) AssertValidResults(t *testing.T, stats *ClusterTestStats) {
	violations := ctv.ValidateResults(stats)

	if len(violations) > 0 {
		t.Errorf("Test results validation failed:")
		for _, violation := range violations {
			t.Errorf("  - %s", violation)
		}
	}
}
