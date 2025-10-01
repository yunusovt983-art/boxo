package monitoring

import (
	"context"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ClusterMetricsTestConfig defines configuration for cluster metrics testing
type ClusterMetricsTestConfig struct {
	NodeCount        int
	MetricsInterval  time.Duration
	AlertThresholds  map[string]float64
	TestDuration     time.Duration
	SimulateLoad     bool
	SimulateFailures bool
}

// DefaultClusterMetricsTestConfig returns default configuration for cluster metrics tests
func DefaultClusterMetricsTestConfig() ClusterMetricsTestConfig {
	return ClusterMetricsTestConfig{
		NodeCount:       5,
		MetricsInterval: 100 * time.Millisecond,
		AlertThresholds: map[string]float64{
			"cpu_warning":     70.0,
			"cpu_critical":    90.0,
			"memory_warning":  0.8,
			"memory_critical": 0.95,
		},
		TestDuration:     10 * time.Second,
		SimulateLoad:     true,
		SimulateFailures: true,
	}
}

// ClusterMetricsNode represents a single node in the metrics test cluster
type ClusterMetricsNode struct {
	ID                 string
	ResourceMonitor    *ResourceMonitor
	BitswapCollector   *BitswapCollector
	AlertManager       *AlertManager
	PerformanceMonitor *PerformanceMonitor
	MetricsRegistry    *prometheus.Registry
	AlertsReceived     []ResourceAlert
	mu                 sync.RWMutex
}

// ClusterMetricsEnvironment manages the metrics test cluster
type ClusterMetricsEnvironment struct {
	Nodes             []*ClusterMetricsNode
	Config            ClusterMetricsTestConfig
	GlobalAlerts      []ResourceAlert
	MetricsAggregator *ClusterMetricsAggregator
	mu                sync.RWMutex
}

// ClusterMetricsAggregator aggregates metrics from multiple nodes
type ClusterMetricsAggregator struct {
	nodes           []*ClusterMetricsNode
	aggregatedStats map[string]interface{}
	mu              sync.RWMutex
}

// TestClusterMetricsCollection tests metrics collection across cluster nodes
func TestClusterMetricsCollection(t *testing.T) {
	config := DefaultClusterMetricsTestConfig()
	config.TestDuration = 5 * time.Second

	env := setupClusterMetricsEnvironment(t, config)
	defer env.Cleanup()

	ctx, cancel := context.WithTimeout(context.Background(), config.TestDuration)
	defer cancel()

	// Start all monitoring components
	env.StartAllMonitoring(ctx)

	// Simulate different load patterns on each node
	var wg sync.WaitGroup
	for i, node := range env.Nodes {
		wg.Add(1)
		go func(nodeIndex int, n *ClusterMetricsNode) {
			defer wg.Done()
			env.simulateNodeActivity(ctx, nodeIndex, n)
		}(i, node)
	}

	// Wait for simulation to complete
	wg.Wait()

	// Allow time for final metrics collection
	time.Sleep(500 * time.Millisecond)

	// Verify metrics collection
	for i, node := range env.Nodes {
		t.Logf("Verifying metrics for node %d", i)

		// Check resource metrics
		if node.ResourceMonitor != nil {
			status := node.ResourceMonitor.GetCurrentStatus()
			assert.NotNil(t, status, "Node %d should have resource status", i)

			cpuHistory := node.ResourceMonitor.GetResourceHistory("cpu")
			assert.NotNil(t, cpuHistory, "Node %d should have CPU history", i)
			assert.Greater(t, len(cpuHistory.Values), 0, "Node %d should have CPU data points", i)

			memoryHistory := node.ResourceMonitor.GetResourceHistory("memory")
			assert.NotNil(t, memoryHistory, "Node %d should have memory history", i)
			assert.Greater(t, len(memoryHistory.Values), 0, "Node %d should have memory data points", i)
		}

		// Check Bitswap metrics
		if node.BitswapCollector != nil {
			metrics := node.BitswapCollector.GetCurrentMetrics()
			assert.NotNil(t, metrics, "Node %d should have Bitswap metrics", i)

			t.Logf("Node %d Bitswap metrics: Requests=%d, AvgLatency=%v, P95=%v",
				i, metrics.TotalRequests, metrics.AverageResponseTime, metrics.P95ResponseTime)
		}

		// Check performance metrics
		if node.PerformanceMonitor != nil {
			perfMetrics := node.PerformanceMonitor.GetCurrentMetrics()
			assert.NotNil(t, perfMetrics, "Node %d should have performance metrics", i)
		}

		// Verify Prometheus metrics
		if node.MetricsRegistry != nil {
			metricFamilies, err := node.MetricsRegistry.Gather()
			assert.NoError(t, err, "Node %d should gather Prometheus metrics", i)
			assert.Greater(t, len(metricFamilies), 0, "Node %d should have metric families", i)
		}
	}

	// Test cluster-wide metrics aggregation
	env.MetricsAggregator.AggregateMetrics()
	aggregatedStats := env.MetricsAggregator.GetAggregatedStats()

	assert.NotNil(t, aggregatedStats, "Should have aggregated cluster stats")
	assert.Contains(t, aggregatedStats, "total_nodes", "Should have total nodes count")
	assert.Contains(t, aggregatedStats, "healthy_nodes", "Should have healthy nodes count")
	assert.Contains(t, aggregatedStats, "average_cpu", "Should have average CPU usage")
	assert.Contains(t, aggregatedStats, "average_memory", "Should have average memory usage")

	t.Logf("Cluster metrics collection test completed successfully")
}

// TestClusterAlertsGeneration tests alert generation and propagation in cluster
func TestClusterAlertsGeneration(t *testing.T) {
	config := DefaultClusterMetricsTestConfig()
	config.TestDuration = 8 * time.Second
	config.AlertThresholds["cpu_warning"] = 60.0 // Lower threshold for testing

	env := setupClusterMetricsEnvironment(t, config)
	defer env.Cleanup()

	ctx, cancel := context.WithTimeout(context.Background(), config.TestDuration)
	defer cancel()

	// Start monitoring with alert generation
	env.StartAllMonitoring(ctx)

	// Simulate high load to trigger alerts
	var wg sync.WaitGroup
	for i, node := range env.Nodes {
		wg.Add(1)
		go func(nodeIndex int, n *ClusterMetricsNode) {
			defer wg.Done()

			// Simulate high CPU usage to trigger alerts
			if n.ResourceMonitor != nil {
				collector := n.ResourceMonitor.collector.(*ResourceCollector)

				// Gradually increase CPU usage
				for j := 0; j < 20; j++ {
					cpuUsage := 50.0 + float64(j)*2.0 // 50% to 88%
					collector.UpdateCPUUsage(cpuUsage)

					// Force collection to trigger alerts
					n.ResourceMonitor.ForceCollection()

					time.Sleep(200 * time.Millisecond)
				}
			}
		}(i, node)
	}

	wg.Wait()

	// Wait for alert processing
	time.Sleep(1 * time.Second)

	// Verify alerts were generated
	totalAlerts := 0
	for i, node := range env.Nodes {
		node.mu.RLock()
		nodeAlerts := len(node.AlertsReceived)
		node.mu.RUnlock()

		t.Logf("Node %d generated %d alerts", i, nodeAlerts)
		totalAlerts += nodeAlerts

		if nodeAlerts > 0 {
			node.mu.RLock()
			lastAlert := node.AlertsReceived[nodeAlerts-1]
			node.mu.RUnlock()

			assert.Equal(t, "cpu", lastAlert.Type, "Should have CPU alert")
			assert.Contains(t, []string{"warning", "critical"}, lastAlert.Level, "Should have warning or critical level")
			assert.NotEmpty(t, lastAlert.Message, "Alert should have message")
			assert.NotNil(t, lastAlert.Timestamp, "Alert should have timestamp")
		}
	}

	assert.Greater(t, totalAlerts, 0, "At least one alert should be generated")

	// Test global alert aggregation
	env.mu.RLock()
	globalAlerts := len(env.GlobalAlerts)
	env.mu.RUnlock()

	t.Logf("Total global alerts: %d", globalAlerts)
	assert.Greater(t, globalAlerts, 0, "Global alerts should be collected")

	t.Logf("Cluster alerts generation test completed with %d total alerts", totalAlerts)
}

// TestClusterMetricsResilience tests metrics collection resilience during failures
func TestClusterMetricsResilience(t *testing.T) {
	config := DefaultClusterMetricsTestConfig()
	config.TestDuration = 12 * time.Second
	config.NodeCount = 6

	env := setupClusterMetricsEnvironment(t, config)
	defer env.Cleanup()

	ctx, cancel := context.WithTimeout(context.Background(), config.TestDuration)
	defer cancel()

	// Start all monitoring
	env.StartAllMonitoring(ctx)

	// Phase 1: Normal operation
	t.Log("Phase 1: Normal operation")
	time.Sleep(3 * time.Second)

	// Verify initial metrics collection
	initialMetricsCount := 0
	for _, node := range env.Nodes {
		if node.ResourceMonitor != nil {
			cpuHistory := node.ResourceMonitor.GetResourceHistory("cpu")
			initialMetricsCount += len(cpuHistory.Values)
		}
	}
	assert.Greater(t, initialMetricsCount, 0, "Should collect metrics during normal operation")

	// Phase 2: Simulate node failures
	t.Log("Phase 2: Simulating node failures")
	failedNodes := []int{1, 3} // Fail 2 out of 6 nodes

	for _, nodeIndex := range failedNodes {
		env.SimulateNodeFailure(nodeIndex)
	}

	time.Sleep(3 * time.Second)

	// Verify remaining nodes continue collecting metrics
	workingNodes := 0
	for i, node := range env.Nodes {
		if !contains(failedNodes, i) {
			workingNodes++
			if node.ResourceMonitor != nil {
				status := node.ResourceMonitor.GetCurrentStatus()
				assert.NotNil(t, status, "Working node %d should still have status", i)
			}
		}
	}
	assert.Equal(t, config.NodeCount-len(failedNodes), workingNodes, "Should have correct number of working nodes")

	// Phase 3: Node recovery
	t.Log("Phase 3: Node recovery")
	for _, nodeIndex := range failedNodes {
		env.RecoverNode(nodeIndex)
	}

	time.Sleep(3 * time.Second)

	// Verify recovered nodes resume metrics collection
	recoveredNodes := 0
	for _, nodeIndex := range failedNodes {
		node := env.Nodes[nodeIndex]
		if node.ResourceMonitor != nil {
			status := node.ResourceMonitor.GetCurrentStatus()
			if status != nil {
				recoveredNodes++
			}
		}
	}
	assert.Greater(t, recoveredNodes, 0, "At least some nodes should recover")

	// Verify cluster-wide metrics aggregation still works
	env.MetricsAggregator.AggregateMetrics()
	aggregatedStats := env.MetricsAggregator.GetAggregatedStats()
	assert.NotNil(t, aggregatedStats, "Cluster aggregation should work after failures")

	t.Logf("Metrics resilience test completed. %d nodes recovered", recoveredNodes)
}

// TestPrometheusMetricsExport tests Prometheus metrics export from cluster
func TestPrometheusMetricsExport(t *testing.T) {
	config := DefaultClusterMetricsTestConfig()
	config.NodeCount = 3
	config.TestDuration = 5 * time.Second

	env := setupClusterMetricsEnvironment(t, config)
	defer env.Cleanup()

	ctx, cancel := context.WithTimeout(context.Background(), config.TestDuration)
	defer cancel()

	// Start monitoring
	env.StartAllMonitoring(ctx)

	// Generate some activity
	time.Sleep(2 * time.Second)

	// Test Prometheus metrics export for each node
	for i, node := range env.Nodes {
		if node.MetricsRegistry != nil {
			// Test that metrics can be gathered
			metricFamilies, err := node.MetricsRegistry.Gather()
			require.NoError(t, err, "Node %d should gather metrics", i)
			require.Greater(t, len(metricFamilies), 0, "Node %d should have metrics", i)

			// Test specific metric existence
			expectedMetrics := []string{
				"boxo_bitswap_requests_total",
				"boxo_bitswap_response_time_seconds",
				"boxo_resource_cpu_usage_percent",
				"boxo_resource_memory_usage_ratio",
			}

			for _, expectedMetric := range expectedMetrics {
				found := false
				for _, family := range metricFamilies {
					if family.GetName() == expectedMetric {
						found = true
						assert.Greater(t, len(family.GetMetric()), 0,
							"Metric %s should have values", expectedMetric)
						break
					}
				}
				if !found {
					t.Logf("Warning: Expected metric %s not found in node %d", expectedMetric, i)
				}
			}

			// Test metric format validation
			for _, family := range metricFamilies {
				assert.NotEmpty(t, family.GetName(), "Metric family should have name")
				assert.NotNil(t, family.GetType(), "Metric family should have type")

				for _, metric := range family.GetMetric() {
					// Validate metric has either counter, gauge, or histogram value
					hasValue := metric.GetCounter() != nil ||
						metric.GetGauge() != nil ||
						metric.GetHistogram() != nil
					assert.True(t, hasValue, "Metric should have a value")
				}
			}

			t.Logf("Node %d exported %d metric families", i, len(metricFamilies))
		}
	}

	// Test cluster-wide metrics aggregation export
	env.MetricsAggregator.AggregateMetrics()
	clusterMetrics := env.MetricsAggregator.ExportPrometheusMetrics()
	assert.NotEmpty(t, clusterMetrics, "Should export cluster-wide metrics")

	t.Log("Prometheus metrics export test completed successfully")
}

// setupClusterMetricsEnvironment creates a test environment for cluster metrics testing
func setupClusterMetricsEnvironment(t *testing.T, config ClusterMetricsTestConfig) *ClusterMetricsEnvironment {
	env := &ClusterMetricsEnvironment{
		Config:       config,
		Nodes:        make([]*ClusterMetricsNode, config.NodeCount),
		GlobalAlerts: make([]ResourceAlert, 0),
	}

	// Create nodes
	for i := 0; i < config.NodeCount; i++ {
		node := &ClusterMetricsNode{
			ID:              fmt.Sprintf("node-%d", i),
			AlertsReceived:  make([]ResourceAlert, 0),
			MetricsRegistry: prometheus.NewRegistry(),
		}

		// Setup resource monitoring
		collector := NewResourceCollector()
		thresholds := ResourceMonitorThresholds{
			CPUWarning:        config.AlertThresholds["cpu_warning"],
			CPUCritical:       config.AlertThresholds["cpu_critical"],
			MemoryWarning:     config.AlertThresholds["memory_warning"],
			MemoryCritical:    config.AlertThresholds["memory_critical"],
			DiskWarning:       0.8,
			DiskCritical:      0.95,
			GoroutineWarning:  5000,
			GoroutineCritical: 10000,
		}

		node.ResourceMonitor = NewResourceMonitor(collector, thresholds)
		node.ResourceMonitor.SetMonitorInterval(config.MetricsInterval)

		// Setup alert callback
		node.ResourceMonitor.SetAlertCallback(func(alert ResourceAlert) {
			node.mu.Lock()
			node.AlertsReceived = append(node.AlertsReceived, alert)
			node.mu.Unlock()

			env.mu.Lock()
			env.GlobalAlerts = append(env.GlobalAlerts, alert)
			env.mu.Unlock()

			t.Logf("Alert from %s: %s - %s", node.ID, alert.Level, alert.Message)
		})

		// Setup Bitswap collector
		node.BitswapCollector = NewBitswapCollector()

		// Setup performance monitor
		node.PerformanceMonitor = NewPerformanceMonitor()

		// Setup alert manager
		node.AlertManager = NewAlertManager()

		// Register metrics with Prometheus
		node.MetricsRegistry.MustRegister(collector)
		if node.BitswapCollector != nil {
			node.MetricsRegistry.MustRegister(node.BitswapCollector)
		}

		env.Nodes[i] = node
	}

	// Setup metrics aggregator
	env.MetricsAggregator = NewClusterMetricsAggregator(env.Nodes)

	return env
}

// StartAllMonitoring starts monitoring on all nodes
func (env *ClusterMetricsEnvironment) StartAllMonitoring(ctx context.Context) {
	for i, node := range env.Nodes {
		if node.ResourceMonitor != nil {
			err := node.ResourceMonitor.Start(ctx)
			if err != nil {
				panic(fmt.Sprintf("Failed to start monitoring on node %d: %v", i, err))
			}
		}
	}
}

// Cleanup shuts down the test environment
func (env *ClusterMetricsEnvironment) Cleanup() {
	for i, node := range env.Nodes {
		if node.ResourceMonitor != nil {
			node.ResourceMonitor.Stop()
		}
		fmt.Printf("Cleaned up metrics node %d\n", i)
	}
}

// simulateNodeActivity simulates various activities on a node
func (env *ClusterMetricsEnvironment) simulateNodeActivity(ctx context.Context, nodeIndex int, node *ClusterMetricsNode) {
	if node.ResourceMonitor == nil {
		return
	}

	collector := node.ResourceMonitor.collector.(*ResourceCollector)

	// Simulate varying CPU and memory usage
	for i := 0; i < 50; i++ {
		select {
		case <-ctx.Done():
			return
		default:
		}

		// Simulate CPU usage pattern
		cpuUsage := 30.0 + float64(nodeIndex)*10.0 + float64(i%10)*2.0
		collector.UpdateCPUUsage(cpuUsage)

		// Simulate memory usage
		memoryUsed := int64(1024*1024*1024) + int64(nodeIndex)*int64(512*1024*1024) + int64(i)*int64(10*1024*1024)
		memoryTotal := int64(8 * 1024 * 1024 * 1024) // 8GB
		collector.UpdateMemoryUsage(memoryUsed, memoryTotal)

		// Simulate disk usage
		diskUsed := int64(50*1024*1024*1024) + int64(i)*int64(100*1024*1024)
		diskTotal := int64(500 * 1024 * 1024 * 1024) // 500GB
		collector.UpdateDiskUsage(diskUsed, diskTotal)

		// Simulate Bitswap activity
		if node.BitswapCollector != nil {
			node.BitswapCollector.RecordRequest(time.Duration(10+i)*time.Millisecond, true)
		}

		time.Sleep(100 * time.Millisecond)
	}
}

// SimulateNodeFailure simulates a node failure
func (env *ClusterMetricsEnvironment) SimulateNodeFailure(nodeIndex int) {
	if nodeIndex >= 0 && nodeIndex < len(env.Nodes) {
		node := env.Nodes[nodeIndex]
		if node.ResourceMonitor != nil {
			node.ResourceMonitor.Stop()
		}
	}
}

// RecoverNode simulates node recovery
func (env *ClusterMetricsEnvironment) RecoverNode(nodeIndex int) {
	if nodeIndex >= 0 && nodeIndex < len(env.Nodes) {
		node := env.Nodes[nodeIndex]
		if node.ResourceMonitor != nil {
			ctx := context.Background()
			node.ResourceMonitor.Start(ctx)
		}
	}
}

// NewClusterMetricsAggregator creates a new cluster metrics aggregator
func NewClusterMetricsAggregator(nodes []*ClusterMetricsNode) *ClusterMetricsAggregator {
	return &ClusterMetricsAggregator{
		nodes:           nodes,
		aggregatedStats: make(map[string]interface{}),
	}
}

// AggregateMetrics aggregates metrics from all nodes
func (cma *ClusterMetricsAggregator) AggregateMetrics() {
	cma.mu.Lock()
	defer cma.mu.Unlock()

	totalNodes := len(cma.nodes)
	healthyNodes := 0
	totalCPU := 0.0
	totalMemory := 0.0
	totalRequests := int64(0)

	for _, node := range cma.nodes {
		if node.ResourceMonitor != nil {
			status := node.ResourceMonitor.GetCurrentStatus()
			if status != nil {
				healthyNodes++
				if cpuVal, ok := status["cpu_usage"].(float64); ok {
					totalCPU += cpuVal
				}
				if memVal, ok := status["memory_usage"].(float64); ok {
					totalMemory += memVal
				}
			}
		}

		if node.BitswapCollector != nil {
			metrics := node.BitswapCollector.GetCurrentMetrics()
			totalRequests += metrics.TotalRequests
		}
	}

	cma.aggregatedStats = map[string]interface{}{
		"total_nodes":    totalNodes,
		"healthy_nodes":  healthyNodes,
		"average_cpu":    totalCPU / float64(healthyNodes),
		"average_memory": totalMemory / float64(healthyNodes),
		"total_requests": totalRequests,
		"cluster_health": float64(healthyNodes) / float64(totalNodes),
	}
}

// GetAggregatedStats returns the aggregated cluster statistics
func (cma *ClusterMetricsAggregator) GetAggregatedStats() map[string]interface{} {
	cma.mu.RLock()
	defer cma.mu.RUnlock()

	result := make(map[string]interface{})
	for k, v := range cma.aggregatedStats {
		result[k] = v
	}
	return result
}

// ExportPrometheusMetrics exports cluster-wide metrics in Prometheus format
func (cma *ClusterMetricsAggregator) ExportPrometheusMetrics() string {
	cma.mu.RLock()
	defer cma.mu.RUnlock()

	metrics := ""
	for key, value := range cma.aggregatedStats {
		switch v := value.(type) {
		case int:
			metrics += fmt.Sprintf("boxo_cluster_%s %d\n", key, v)
		case int64:
			metrics += fmt.Sprintf("boxo_cluster_%s %d\n", key, v)
		case float64:
			metrics += fmt.Sprintf("boxo_cluster_%s %.2f\n", key, v)
		}
	}
	return metrics
}

// Helper function to check if slice contains value
func contains(slice []int, value int) bool {
	for _, v := range slice {
		if v == value {
			return true
		}
	}
	return false
}
