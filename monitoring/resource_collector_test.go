package monitoring

import (
	"context"
	"testing"
	"time"
)

func TestNewResourceCollector(t *testing.T) {
	collector := NewResourceCollector()

	if collector == nil {
		t.Fatal("Expected non-nil ResourceCollector")
	}

	// Test initial state
	if collector.cpuUsage != 0 {
		t.Error("Expected initial CPU usage to be 0")
	}

	if collector.goroutineCount != 0 {
		t.Error("Expected initial goroutine count to be 0")
	}
}

func TestResourceCollectorCollectMetrics(t *testing.T) {
	collector := NewResourceCollector()

	// Update some values
	collector.UpdateCPUUsage(75.5)
	collector.UpdateDiskUsage(50*1024*1024*1024, 100*1024*1024*1024) // 50GB used, 100GB total

	metrics, err := collector.CollectMetrics(context.Background())
	if err != nil {
		t.Fatalf("Failed to collect metrics: %v", err)
	}

	if metrics == nil {
		t.Fatal("Expected non-nil metrics")
	}

	// Check CPU usage
	if cpuUsage, ok := metrics["cpu_usage"].(float64); !ok || cpuUsage != 75.5 {
		t.Errorf("Expected CPU usage 75.5, got %v", cpuUsage)
	}

	// Check disk usage
	if diskUsage, ok := metrics["disk_usage"].(int64); !ok || diskUsage != 50*1024*1024*1024 {
		t.Errorf("Expected disk usage %d, got %v", 50*1024*1024*1024, diskUsage)
	}

	if diskTotal, ok := metrics["disk_total"].(int64); !ok || diskTotal != 100*1024*1024*1024 {
		t.Errorf("Expected disk total %d, got %v", 100*1024*1024*1024, diskTotal)
	}

	// Check that memory and goroutine metrics are present
	if _, ok := metrics["memory_usage"]; !ok {
		t.Error("Expected memory_usage metric")
	}

	if _, ok := metrics["goroutine_count"]; !ok {
		t.Error("Expected goroutine_count metric")
	}
}

func TestResourceCollectorGetMetricNames(t *testing.T) {
	collector := NewResourceCollector()

	names := collector.GetMetricNames()

	expectedNames := []string{
		"cpu_usage",
		"memory_usage",
		"memory_total",
		"disk_usage",
		"disk_total",
		"goroutine_count",
		"gc_pause_time",
	}

	if len(names) != len(expectedNames) {
		t.Errorf("Expected %d metric names, got %d", len(expectedNames), len(names))
	}

	nameMap := make(map[string]bool)
	for _, name := range names {
		nameMap[name] = true
	}

	for _, expected := range expectedNames {
		if !nameMap[expected] {
			t.Errorf("Expected metric name %s not found", expected)
		}
	}
}

func TestResourceCollectorUpdateCPUUsage(t *testing.T) {
	collector := NewResourceCollector()

	// Test updating CPU usage
	collector.UpdateCPUUsage(45.7)

	if collector.cpuUsage != 45.7 {
		t.Errorf("Expected CPU usage 45.7, got %f", collector.cpuUsage)
	}

	// Test updating again
	collector.UpdateCPUUsage(89.2)

	if collector.cpuUsage != 89.2 {
		t.Errorf("Expected CPU usage 89.2, got %f", collector.cpuUsage)
	}
}

func TestResourceCollectorUpdateDiskUsage(t *testing.T) {
	collector := NewResourceCollector()

	used := int64(25 * 1024 * 1024 * 1024)   // 25GB
	total := int64(100 * 1024 * 1024 * 1024) // 100GB

	collector.UpdateDiskUsage(used, total)

	if collector.diskUsage != used {
		t.Errorf("Expected disk usage %d, got %d", used, collector.diskUsage)
	}

	if collector.diskTotal != total {
		t.Errorf("Expected disk total %d, got %d", total, collector.diskTotal)
	}
}

func TestResourceCollectorMemoryPressure(t *testing.T) {
	collector := NewResourceCollector()

	// Test with no memory data
	pressure := collector.GetMemoryPressure()
	if pressure != 0.0 {
		t.Errorf("Expected memory pressure 0.0 with no data, got %f", pressure)
	}

	// Simulate memory usage by updating internal values
	collector.memoryUsage = 8 * 1024 * 1024 * 1024  // 8GB
	collector.memoryTotal = 16 * 1024 * 1024 * 1024 // 16GB

	pressure = collector.GetMemoryPressure()
	expected := 0.5 // 50%
	if pressure != expected {
		t.Errorf("Expected memory pressure %f, got %f", expected, pressure)
	}

	// Test high memory pressure
	isHigh := collector.IsMemoryPressureHigh(0.4) // 40% threshold
	if !isHigh {
		t.Error("Expected high memory pressure with 50% usage and 40% threshold")
	}

	isHigh = collector.IsMemoryPressureHigh(0.6) // 60% threshold
	if isHigh {
		t.Error("Expected normal memory pressure with 50% usage and 60% threshold")
	}
}

func TestResourceCollectorDiskPressure(t *testing.T) {
	collector := NewResourceCollector()

	// Test with no disk data
	pressure := collector.GetDiskPressure()
	if pressure != 0.0 {
		t.Errorf("Expected disk pressure 0.0 with no data, got %f", pressure)
	}

	used := int64(75 * 1024 * 1024 * 1024)   // 75GB
	total := int64(100 * 1024 * 1024 * 1024) // 100GB

	collector.UpdateDiskUsage(used, total)

	pressure = collector.GetDiskPressure()
	expected := 0.75 // 75%
	if pressure != expected {
		t.Errorf("Expected disk pressure %f, got %f", expected, pressure)
	}

	// Test high disk pressure
	isHigh := collector.IsDiskPressureHigh(0.7) // 70% threshold
	if !isHigh {
		t.Error("Expected high disk pressure with 75% usage and 70% threshold")
	}

	isHigh = collector.IsDiskPressureHigh(0.8) // 80% threshold
	if isHigh {
		t.Error("Expected normal disk pressure with 75% usage and 80% threshold")
	}
}

func TestResourceCollectorGetResourceSummary(t *testing.T) {
	collector := NewResourceCollector()

	// Set some test values
	collector.UpdateCPUUsage(65.3)
	collector.UpdateDiskUsage(30*1024*1024*1024, 100*1024*1024*1024) // 30GB used, 100GB total
	collector.memoryUsage = 4 * 1024 * 1024 * 1024                   // 4GB
	collector.memoryTotal = 16 * 1024 * 1024 * 1024                  // 16GB
	collector.goroutineCount = 1500
	collector.gcPauseTime = 2 * time.Millisecond

	summary := collector.GetResourceSummary()

	if summary == nil {
		t.Fatal("Expected non-nil resource summary")
	}

	// Check CPU usage
	if cpuUsage, ok := summary["cpu_usage_percent"].(float64); !ok || cpuUsage != 65.3 {
		t.Errorf("Expected CPU usage 65.3, got %v", cpuUsage)
	}

	// Check memory values
	if memoryUsageMB, ok := summary["memory_usage_mb"].(int64); !ok || memoryUsageMB != 4*1024 {
		t.Errorf("Expected memory usage 4096 MB, got %v", memoryUsageMB)
	}

	if memoryTotalMB, ok := summary["memory_total_mb"].(int64); !ok || memoryTotalMB != 16*1024 {
		t.Errorf("Expected memory total 16384 MB, got %v", memoryTotalMB)
	}

	// Check memory pressure
	if memoryPressure, ok := summary["memory_pressure"].(float64); !ok || memoryPressure != 0.25 {
		t.Errorf("Expected memory pressure 0.25, got %v", memoryPressure)
	}

	// Check disk values
	if diskUsageGB, ok := summary["disk_usage_gb"].(int64); !ok || diskUsageGB != 30 {
		t.Errorf("Expected disk usage 30 GB, got %v", diskUsageGB)
	}

	if diskTotalGB, ok := summary["disk_total_gb"].(int64); !ok || diskTotalGB != 100 {
		t.Errorf("Expected disk total 100 GB, got %v", diskTotalGB)
	}

	// Check disk pressure
	if diskPressure, ok := summary["disk_pressure"].(float64); !ok || diskPressure != 0.3 {
		t.Errorf("Expected disk pressure 0.3, got %v", diskPressure)
	}

	// Check goroutines
	if goroutines, ok := summary["goroutines"].(int); !ok || goroutines != 1500 {
		t.Errorf("Expected goroutines 1500, got %v", goroutines)
	}

	// Check GC pause time
	if gcPauseMS, ok := summary["gc_pause_ms"].(float64); !ok || gcPauseMS != 2.0 {
		t.Errorf("Expected GC pause 2.0 ms, got %v", gcPauseMS)
	}
}

func TestResourceCollectorGetMemoryStats(t *testing.T) {
	collector := NewResourceCollector()

	stats := collector.GetMemoryStats()

	// Check that we get valid memory stats
	if stats.Alloc == 0 {
		t.Error("Expected non-zero allocated memory")
	}

	if stats.Sys == 0 {
		t.Error("Expected non-zero system memory")
	}

	if stats.NumGC < 0 {
		t.Error("Expected non-negative GC count")
	}
}

func TestResourceCollectorGetGCStats(t *testing.T) {
	collector := NewResourceCollector()

	gcStats := collector.GetGCStats()

	if gcStats == nil {
		t.Fatal("Expected non-nil GC stats")
	}

	// Check that required fields are present
	requiredFields := []string{
		"num_gc",
		"pause_total_ns",
		"pause_ns",
		"gc_cpu_fraction",
		"next_gc",
		"last_gc",
	}

	for _, field := range requiredFields {
		if _, ok := gcStats[field]; !ok {
			t.Errorf("Expected GC stats field %s", field)
		}
	}
}

func TestResourceCollectorForceGC(t *testing.T) {
	collector := NewResourceCollector()

	// Force GC and measure duration
	duration := collector.ForceGC()

	if duration < 0 {
		t.Error("Expected non-negative GC duration")
	}

	// Duration should be reasonable (less than 1 second for tests)
	if duration > time.Second {
		t.Errorf("GC took too long: %v", duration)
	}
}

// Benchmark tests
func BenchmarkResourceCollectorCollectMetrics(b *testing.B) {
	collector := NewResourceCollector()
	ctx := context.Background()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		collector.CollectMetrics(ctx)
	}
}

func BenchmarkResourceCollectorUpdateCPUUsage(b *testing.B) {
	collector := NewResourceCollector()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		collector.UpdateCPUUsage(float64(i % 100))
	}
}

func BenchmarkResourceCollectorGetResourceSummary(b *testing.B) {
	collector := NewResourceCollector()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		collector.GetResourceSummary()
	}
}
