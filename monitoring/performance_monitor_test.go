package monitoring

import (
	"context"
	"testing"
	"time"
)

func TestNewPerformanceMonitor(t *testing.T) {
	pm := NewPerformanceMonitor()
	
	if pm == nil {
		t.Fatal("NewPerformanceMonitor returned nil")
	}
	
	registry := pm.GetPrometheusRegistry()
	if registry == nil {
		t.Fatal("GetPrometheusRegistry returned nil")
	}
}

func TestPerformanceMonitorCollectMetrics(t *testing.T) {
	pm := NewPerformanceMonitor()
	
	// Register test collectors
	bitswapCollector := NewBitswapCollector()
	blockstoreCollector := NewBlockstoreCollector()
	networkCollector := NewNetworkCollector()
	resourceCollector := NewResourceCollector()
	
	err := pm.RegisterCollector("bitswap", bitswapCollector)
	if err != nil {
		t.Fatalf("Failed to register bitswap collector: %v", err)
	}
	
	err = pm.RegisterCollector("blockstore", blockstoreCollector)
	if err != nil {
		t.Fatalf("Failed to register blockstore collector: %v", err)
	}
	
	err = pm.RegisterCollector("network", networkCollector)
	if err != nil {
		t.Fatalf("Failed to register network collector: %v", err)
	}
	
	err = pm.RegisterCollector("resource", resourceCollector)
	if err != nil {
		t.Fatalf("Failed to register resource collector: %v", err)
	}
	
	// Generate some test data
	bitswapCollector.RecordRequest()
	bitswapCollector.RecordResponse(50*time.Millisecond, true)
	bitswapCollector.UpdateActiveConnections(10)
	
	blockstoreCollector.RecordCacheHit()
	blockstoreCollector.RecordReadOperation(5 * time.Millisecond)
	blockstoreCollector.UpdateTotalBlocks(1000)
	
	networkCollector.UpdateActiveConnections(5)
	networkCollector.RecordLatency(20 * time.Millisecond)
	networkCollector.RecordBandwidth(1024 * 1024) // 1MB/s
	
	resourceCollector.UpdateCPUUsage(45.5)
	resourceCollector.UpdateDiskUsage(1024*1024*1024, 10*1024*1024*1024) // 1GB used, 10GB total
	
	// Collect metrics
	metrics := pm.CollectMetrics()
	
	if metrics == nil {
		t.Fatal("CollectMetrics returned nil")
	}
	
	if metrics.Timestamp.IsZero() {
		t.Error("Metrics timestamp is zero")
	}
	
	if len(metrics.CustomMetrics) != 4 {
		t.Errorf("Expected 4 custom metrics, got %d", len(metrics.CustomMetrics))
	}
	
	// Verify bitswap metrics
	if bitswapMetrics, ok := metrics.CustomMetrics["bitswap"].(map[string]interface{}); ok {
		if activeConns, ok := bitswapMetrics["active_connections"].(int); !ok || activeConns != 10 {
			t.Errorf("Expected active_connections=10, got %v", bitswapMetrics["active_connections"])
		}
	} else {
		t.Error("Bitswap metrics not found or invalid type")
	}
	
	// Verify blockstore metrics
	if blockstoreMetrics, ok := metrics.CustomMetrics["blockstore"].(map[string]interface{}); ok {
		if totalBlocks, ok := blockstoreMetrics["total_blocks"].(int64); !ok || totalBlocks != 1000 {
			t.Errorf("Expected total_blocks=1000, got %v", blockstoreMetrics["total_blocks"])
		}
	} else {
		t.Error("Blockstore metrics not found or invalid type")
	}
	
	// Verify network metrics
	if networkMetrics, ok := metrics.CustomMetrics["network"].(map[string]interface{}); ok {
		if activeConns, ok := networkMetrics["active_connections"].(int); !ok || activeConns != 5 {
			t.Errorf("Expected active_connections=5, got %v", networkMetrics["active_connections"])
		}
	} else {
		t.Error("Network metrics not found or invalid type")
	}
	
	// Verify resource metrics
	if resourceMetrics, ok := metrics.CustomMetrics["resource"].(map[string]interface{}); ok {
		if cpuUsage, ok := resourceMetrics["cpu_usage"].(float64); !ok || cpuUsage != 45.5 {
			t.Errorf("Expected cpu_usage=45.5, got %v", resourceMetrics["cpu_usage"])
		}
	} else {
		t.Error("Resource metrics not found or invalid type")
	}
}

func TestPerformanceMonitorStartStopCollection(t *testing.T) {
	pm := NewPerformanceMonitor()
	
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	
	// Start collection
	err := pm.StartCollection(ctx, 100*time.Millisecond)
	if err != nil {
		t.Fatalf("Failed to start collection: %v", err)
	}
	
	// Wait a bit to ensure collection is running
	time.Sleep(150 * time.Millisecond)
	
	// Stop collection
	err = pm.StopCollection()
	if err != nil {
		t.Fatalf("Failed to stop collection: %v", err)
	}
	
	// Starting again should work
	err = pm.StartCollection(ctx, 100*time.Millisecond)
	if err != nil {
		t.Fatalf("Failed to restart collection: %v", err)
	}
	
	err = pm.StopCollection()
	if err != nil {
		t.Fatalf("Failed to stop collection again: %v", err)
	}
}

func TestPerformanceMonitorPrometheusMetrics(t *testing.T) {
	pm := NewPerformanceMonitor()
	registry := pm.GetPrometheusRegistry()
	
	// Register a bitswap collector and generate some data
	bitswapCollector := NewBitswapCollector()
	err := pm.RegisterCollector("bitswap", bitswapCollector)
	if err != nil {
		t.Fatalf("Failed to register bitswap collector: %v", err)
	}
	
	bitswapCollector.UpdateActiveConnections(15)
	bitswapCollector.UpdateQueuedRequests(25)
	
	// Collect metrics to update Prometheus metrics
	pm.CollectMetrics()
	
	// Check if Prometheus metrics are updated
	metricFamilies, err := registry.Gather()
	if err != nil {
		t.Fatalf("Failed to gather metrics: %v", err)
	}
	
	if len(metricFamilies) == 0 {
		t.Error("No metrics found in registry")
	}
	
	// Look for specific metrics
	foundActiveConnections := false
	foundQueuedRequests := false
	
	for _, mf := range metricFamilies {
		switch *mf.Name {
		case "boxo_bitswap_active_connections":
			foundActiveConnections = true
			if len(mf.Metric) > 0 && *mf.Metric[0].Gauge.Value != 15 {
				t.Errorf("Expected active_connections=15, got %f", *mf.Metric[0].Gauge.Value)
			}
		case "boxo_bitswap_queued_requests":
			foundQueuedRequests = true
			if len(mf.Metric) > 0 && *mf.Metric[0].Gauge.Value != 25 {
				t.Errorf("Expected queued_requests=25, got %f", *mf.Metric[0].Gauge.Value)
			}
		}
	}
	
	if !foundActiveConnections {
		t.Error("boxo_bitswap_active_connections metric not found")
	}
	
	if !foundQueuedRequests {
		t.Error("boxo_bitswap_queued_requests metric not found")
	}
}

func TestPerformanceMonitorRegisterCollector(t *testing.T) {
	pm := NewPerformanceMonitor()
	
	collector := NewBitswapCollector()
	
	// Register collector
	err := pm.RegisterCollector("test_collector", collector)
	if err != nil {
		t.Fatalf("Failed to register collector: %v", err)
	}
	
	// Registering the same name again should work (overwrite)
	err = pm.RegisterCollector("test_collector", collector)
	if err != nil {
		t.Fatalf("Failed to re-register collector: %v", err)
	}
}

func TestPerformanceMonitorConcurrentAccess(t *testing.T) {
	pm := NewPerformanceMonitor()
	
	// Register collectors
	bitswapCollector := NewBitswapCollector()
	err := pm.RegisterCollector("bitswap", bitswapCollector)
	if err != nil {
		t.Fatalf("Failed to register bitswap collector: %v", err)
	}
	
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	
	// Start collection
	err = pm.StartCollection(ctx, 10*time.Millisecond)
	if err != nil {
		t.Fatalf("Failed to start collection: %v", err)
	}
	
	// Simulate concurrent access
	done := make(chan bool, 3)
	
	// Goroutine 1: Continuously collect metrics
	go func() {
		for i := 0; i < 10; i++ {
			pm.CollectMetrics()
			time.Sleep(5 * time.Millisecond)
		}
		done <- true
	}()
	
	// Goroutine 2: Continuously update collector data
	go func() {
		for i := 0; i < 10; i++ {
			bitswapCollector.RecordRequest()
			bitswapCollector.RecordResponse(time.Duration(i)*time.Millisecond, true)
			bitswapCollector.UpdateActiveConnections(i)
			time.Sleep(3 * time.Millisecond)
		}
		done <- true
	}()
	
	// Goroutine 3: Register/unregister collectors
	go func() {
		for i := 0; i < 5; i++ {
			newCollector := NewNetworkCollector()
			pm.RegisterCollector("temp_collector", newCollector)
			time.Sleep(8 * time.Millisecond)
		}
		done <- true
	}()
	
	// Wait for all goroutines to complete
	for i := 0; i < 3; i++ {
		<-done
	}
	
	err = pm.StopCollection()
	if err != nil {
		t.Fatalf("Failed to stop collection: %v", err)
	}
}

func TestPerformanceMonitorMetricsValidation(t *testing.T) {
	pm := NewPerformanceMonitor()
	
	// Test with no collectors
	metrics := pm.CollectMetrics()
	if metrics == nil {
		t.Fatal("CollectMetrics returned nil with no collectors")
	}
	
	if len(metrics.CustomMetrics) != 0 {
		t.Errorf("Expected 0 custom metrics with no collectors, got %d", len(metrics.CustomMetrics))
	}
	
	// Test with empty collector
	emptyCollector := &testCollector{}
	err := pm.RegisterCollector("empty", emptyCollector)
	if err != nil {
		t.Fatalf("Failed to register empty collector: %v", err)
	}
	
	metrics = pm.CollectMetrics()
	if len(metrics.CustomMetrics) != 1 {
		t.Errorf("Expected 1 custom metric with empty collector, got %d", len(metrics.CustomMetrics))
	}
}

// testCollector is a simple test implementation of MetricCollector
type testCollector struct{}

func (tc *testCollector) CollectMetrics(ctx context.Context) (map[string]interface{}, error) {
	return map[string]interface{}{
		"test_metric": 42,
	}, nil
}

func (tc *testCollector) GetMetricNames() []string {
	return []string{"test_metric"}
}

func BenchmarkPerformanceMonitorCollectMetrics(b *testing.B) {
	pm := NewPerformanceMonitor()
	
	// Register all collectors
	pm.RegisterCollector("bitswap", NewBitswapCollector())
	pm.RegisterCollector("blockstore", NewBlockstoreCollector())
	pm.RegisterCollector("network", NewNetworkCollector())
	pm.RegisterCollector("resource", NewResourceCollector())
	
	b.ResetTimer()
	
	for i := 0; i < b.N; i++ {
		pm.CollectMetrics()
	}
}

func BenchmarkPerformanceMonitorConcurrentCollection(b *testing.B) {
	pm := NewPerformanceMonitor()
	
	// Register collectors
	bitswapCollector := NewBitswapCollector()
	pm.RegisterCollector("bitswap", bitswapCollector)
	
	b.ResetTimer()
	
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			// Simulate concurrent metric collection and data updates
			go pm.CollectMetrics()
			bitswapCollector.RecordRequest()
			bitswapCollector.RecordResponse(time.Millisecond, true)
		}
	})
}