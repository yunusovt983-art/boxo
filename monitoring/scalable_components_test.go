package monitoring

import (
	"context"
	"testing"
	"time"
)

func TestWorkerPoolComponent(t *testing.T) {
	component := NewWorkerPoolComponent("test_worker_pool", 128, 64, 2048)

	// Test initial state
	if component.GetCurrentScale() != 128 {
		t.Errorf("Expected initial scale 128, got %d", component.GetCurrentScale())
	}

	if component.GetComponentName() != "test_worker_pool" {
		t.Errorf("Expected component name 'test_worker_pool', got '%s'", component.GetComponentName())
	}

	if !component.IsHealthy() {
		t.Error("Component should be healthy initially")
	}

	min, max := component.GetScaleRange()
	if min != 64 || max != 2048 {
		t.Errorf("Expected scale range [64, 2048], got [%d, %d]", min, max)
	}

	// Test scaling up
	ctx := context.Background()
	err := component.SetScale(ctx, 256)
	if err != nil {
		t.Fatalf("Failed to scale up: %v", err)
	}

	if component.GetCurrentScale() != 256 {
		t.Errorf("Expected scale 256 after scaling up, got %d", component.GetCurrentScale())
	}

	// Test scaling down
	err = component.SetScale(ctx, 100)
	if err != nil {
		t.Fatalf("Failed to scale down: %v", err)
	}

	if component.GetCurrentScale() != 100 {
		t.Errorf("Expected scale 100 after scaling down, got %d", component.GetCurrentScale())
	}

	// Test scaling outside range
	err = component.SetScale(ctx, 32) // Below min
	if err == nil {
		t.Error("Expected error when scaling below minimum")
	}

	err = component.SetScale(ctx, 3000) // Above max
	if err == nil {
		t.Error("Expected error when scaling above maximum")
	}

	// Test metrics
	component.UpdateMetrics(500, 1000, 50*time.Millisecond)
	metrics := component.GetMetrics()

	if metrics["queued_tasks"] != int64(500) {
		t.Errorf("Expected queued_tasks 500, got %v", metrics["queued_tasks"])
	}

	if metrics["processed_tasks"] != int64(1000) {
		t.Errorf("Expected processed_tasks 1000, got %v", metrics["processed_tasks"])
	}

	if metrics["average_latency"] != 50*time.Millisecond {
		t.Errorf("Expected average_latency 50ms, got %v", metrics["average_latency"])
	}

	// Test health status
	component.SetHealthy(false)
	if component.IsHealthy() {
		t.Error("Component should not be healthy after setting to false")
	}

	component.SetHealthy(true)
	if !component.IsHealthy() {
		t.Error("Component should be healthy after setting to true")
	}
}

func TestConnectionPoolComponent(t *testing.T) {
	component := NewConnectionPoolComponent("test_connection_pool", 1000, 500, 5000)

	// Test initial state
	if component.GetCurrentScale() != 1000 {
		t.Errorf("Expected initial scale 1000, got %d", component.GetCurrentScale())
	}

	if component.GetComponentName() != "test_connection_pool" {
		t.Errorf("Expected component name 'test_connection_pool', got '%s'", component.GetComponentName())
	}

	if !component.IsHealthy() {
		t.Error("Component should be healthy initially")
	}

	min, max := component.GetScaleRange()
	if min != 500 || max != 5000 {
		t.Errorf("Expected scale range [500, 5000], got [%d, %d]", min, max)
	}

	// Test scaling up
	ctx := context.Background()
	err := component.SetScale(ctx, 1500)
	if err != nil {
		t.Fatalf("Failed to scale up: %v", err)
	}

	if component.GetCurrentScale() != 1500 {
		t.Errorf("Expected scale 1500 after scaling up, got %d", component.GetCurrentScale())
	}

	// Test scaling down
	err = component.SetScale(ctx, 800)
	if err != nil {
		t.Fatalf("Failed to scale down: %v", err)
	}

	if component.GetCurrentScale() != 800 {
		t.Errorf("Expected scale 800 after scaling down, got %d", component.GetCurrentScale())
	}

	// Test scaling outside range
	err = component.SetScale(ctx, 400) // Below min
	if err == nil {
		t.Error("Expected error when scaling below minimum")
	}

	err = component.SetScale(ctx, 6000) // Above max
	if err == nil {
		t.Error("Expected error when scaling above maximum")
	}

	// Test metrics
	component.UpdateMetrics(600, 200, 10000, 100, 25*time.Millisecond)
	metrics := component.GetMetrics()

	if metrics["active_connections"] != 600 {
		t.Errorf("Expected active_connections 600, got %v", metrics["active_connections"])
	}

	if metrics["idle_connections"] != 200 {
		t.Errorf("Expected idle_connections 200, got %v", metrics["idle_connections"])
	}

	if metrics["total_requests"] != int64(10000) {
		t.Errorf("Expected total_requests 10000, got %v", metrics["total_requests"])
	}

	if metrics["failed_requests"] != int64(100) {
		t.Errorf("Expected failed_requests 100, got %v", metrics["failed_requests"])
	}

	expectedUtilization := float64(600) / float64(800) // 0.75
	if metrics["utilization"] != expectedUtilization {
		t.Errorf("Expected utilization %f, got %v", expectedUtilization, metrics["utilization"])
	}

	expectedErrorRate := float64(100) / float64(10000) // 0.01
	if metrics["error_rate"] != expectedErrorRate {
		t.Errorf("Expected error_rate %f, got %v", expectedErrorRate, metrics["error_rate"])
	}
}

func TestBatchProcessorComponent(t *testing.T) {
	component := NewBatchProcessorComponent("test_batch_processor", 50, 10, 200)

	// Test initial state
	if component.GetCurrentScale() != 50 {
		t.Errorf("Expected initial scale 50, got %d", component.GetCurrentScale())
	}

	if component.GetComponentName() != "test_batch_processor" {
		t.Errorf("Expected component name 'test_batch_processor', got '%s'", component.GetComponentName())
	}

	if !component.IsHealthy() {
		t.Error("Component should be healthy initially")
	}

	min, max := component.GetScaleRange()
	if min != 10 || max != 200 {
		t.Errorf("Expected scale range [10, 200], got [%d, %d]", min, max)
	}

	// Test scaling up
	ctx := context.Background()
	err := component.SetScale(ctx, 75)
	if err != nil {
		t.Fatalf("Failed to scale up: %v", err)
	}

	if component.GetCurrentScale() != 75 {
		t.Errorf("Expected scale 75 after scaling up, got %d", component.GetCurrentScale())
	}

	// Test scaling down
	err = component.SetScale(ctx, 30)
	if err != nil {
		t.Fatalf("Failed to scale down: %v", err)
	}

	if component.GetCurrentScale() != 30 {
		t.Errorf("Expected scale 30 after scaling down, got %d", component.GetCurrentScale())
	}

	// Test scaling outside range
	err = component.SetScale(ctx, 5) // Below min
	if err == nil {
		t.Error("Expected error when scaling below minimum")
	}

	err = component.SetScale(ctx, 250) // Above max
	if err == nil {
		t.Error("Expected error when scaling above maximum")
	}

	// Test metrics
	component.UpdateMetrics(20, 100, 5000, 50, 100, 200*time.Millisecond)
	metrics := component.GetMetrics()

	if metrics["active_batches"] != 20 {
		t.Errorf("Expected active_batches 20, got %v", metrics["active_batches"])
	}

	if metrics["queued_batches"] != 100 {
		t.Errorf("Expected queued_batches 100, got %v", metrics["queued_batches"])
	}

	if metrics["processed_batches"] != int64(5000) {
		t.Errorf("Expected processed_batches 5000, got %v", metrics["processed_batches"])
	}

	if metrics["failed_batches"] != int64(50) {
		t.Errorf("Expected failed_batches 50, got %v", metrics["failed_batches"])
	}

	if metrics["average_batch_size"] != 100 {
		t.Errorf("Expected average_batch_size 100, got %v", metrics["average_batch_size"])
	}

	expectedUtilization := float64(20) / float64(30) // ~0.67
	if metrics["utilization"] != expectedUtilization {
		t.Errorf("Expected utilization %f, got %v", expectedUtilization, metrics["utilization"])
	}

	expectedErrorRate := float64(50) / float64(5050) // ~0.0099
	if metrics["error_rate"] != expectedErrorRate {
		t.Errorf("Expected error_rate %f, got %v", expectedErrorRate, metrics["error_rate"])
	}

	expectedThroughput := float64(100) / (200 * time.Millisecond).Seconds() // 500 items/sec
	if metrics["throughput"] != expectedThroughput {
		t.Errorf("Expected throughput %f, got %v", expectedThroughput, metrics["throughput"])
	}
}

func TestCacheComponent(t *testing.T) {
	component := NewCacheComponent("test_cache", 1024, 256, 4096) // 1GB initial, 256MB min, 4GB max

	// Test initial state
	if component.GetCurrentScale() != 1024 {
		t.Errorf("Expected initial scale 1024, got %d", component.GetCurrentScale())
	}

	if component.GetComponentName() != "test_cache" {
		t.Errorf("Expected component name 'test_cache', got '%s'", component.GetComponentName())
	}

	// Initially not healthy due to 0 hit rate
	if component.IsHealthy() {
		t.Error("Component should not be healthy initially (0 hit rate)")
	}

	min, max := component.GetScaleRange()
	if min != 256 || max != 4096 {
		t.Errorf("Expected scale range [256, 4096], got [%d, %d]", min, max)
	}

	// Test scaling up
	ctx := context.Background()
	err := component.SetScale(ctx, 2048)
	if err != nil {
		t.Fatalf("Failed to scale up: %v", err)
	}

	if component.GetCurrentScale() != 2048 {
		t.Errorf("Expected scale 2048 after scaling up, got %d", component.GetCurrentScale())
	}

	// Test scaling down
	err = component.SetScale(ctx, 512)
	if err != nil {
		t.Fatalf("Failed to scale down: %v", err)
	}

	if component.GetCurrentScale() != 512 {
		t.Errorf("Expected scale 512 after scaling down, got %d", component.GetCurrentScale())
	}

	// Test scaling outside range
	err = component.SetScale(ctx, 128) // Below min
	if err == nil {
		t.Error("Expected error when scaling below minimum")
	}

	err = component.SetScale(ctx, 8192) // Above max
	if err == nil {
		t.Error("Expected error when scaling above maximum")
	}

	// Test metrics
	memoryUsage := int64(400 * 1024 * 1024) // 400MB
	component.UpdateMetrics(0.8, 0.2, 0.05, memoryUsage, 100000)
	metrics := component.GetMetrics()

	if metrics["hit_rate"] != 0.8 {
		t.Errorf("Expected hit_rate 0.8, got %v", metrics["hit_rate"])
	}

	if metrics["miss_rate"] != 0.2 {
		t.Errorf("Expected miss_rate 0.2, got %v", metrics["miss_rate"])
	}

	if metrics["eviction_rate"] != 0.05 {
		t.Errorf("Expected eviction_rate 0.05, got %v", metrics["eviction_rate"])
	}

	if metrics["memory_usage_bytes"] != memoryUsage {
		t.Errorf("Expected memory_usage_bytes %d, got %v", memoryUsage, metrics["memory_usage_bytes"])
	}

	if metrics["total_requests"] != int64(100000) {
		t.Errorf("Expected total_requests 100000, got %v", metrics["total_requests"])
	}

	expectedUtilizationMB := float64(memoryUsage) / (1024 * 1024) // 400MB
	if metrics["memory_usage_mb"] != expectedUtilizationMB {
		t.Errorf("Expected memory_usage_mb %f, got %v", expectedUtilizationMB, metrics["memory_usage_mb"])
	}

	expectedUtilization := expectedUtilizationMB / float64(512) // 400/512 = ~0.78
	if metrics["utilization"] != expectedUtilization {
		t.Errorf("Expected utilization %f, got %v", expectedUtilization, metrics["utilization"])
	}

	// Test health based on hit rate
	component.UpdateMetrics(0.3, 0.7, 0.1, memoryUsage, 100000) // Low hit rate
	if component.IsHealthy() {
		t.Error("Component should not be healthy with low hit rate")
	}

	component.UpdateMetrics(0.9, 0.1, 0.01, memoryUsage, 100000) // High hit rate
	if !component.IsHealthy() {
		t.Error("Component should be healthy with high hit rate")
	}
}

func TestNetworkBufferComponent(t *testing.T) {
	component := NewNetworkBufferComponent("test_network_buffer", 64, 16, 256) // 64KB initial, 16KB min, 256KB max

	// Test initial state
	if component.GetCurrentScale() != 64 {
		t.Errorf("Expected initial scale 64, got %d", component.GetCurrentScale())
	}

	if component.GetComponentName() != "test_network_buffer" {
		t.Errorf("Expected component name 'test_network_buffer', got '%s'", component.GetComponentName())
	}

	if !component.IsHealthy() {
		t.Error("Component should be healthy initially")
	}

	min, max := component.GetScaleRange()
	if min != 16 || max != 256 {
		t.Errorf("Expected scale range [16, 256], got [%d, %d]", min, max)
	}

	// Test scaling up
	ctx := context.Background()
	err := component.SetScale(ctx, 128)
	if err != nil {
		t.Fatalf("Failed to scale up: %v", err)
	}

	if component.GetCurrentScale() != 128 {
		t.Errorf("Expected scale 128 after scaling up, got %d", component.GetCurrentScale())
	}

	// Test scaling down
	err = component.SetScale(ctx, 32)
	if err != nil {
		t.Fatalf("Failed to scale down: %v", err)
	}

	if component.GetCurrentScale() != 32 {
		t.Errorf("Expected scale 32 after scaling down, got %d", component.GetCurrentScale())
	}

	// Test scaling outside range
	err = component.SetScale(ctx, 8) // Below min
	if err == nil {
		t.Error("Expected error when scaling below minimum")
	}

	err = component.SetScale(ctx, 512) // Above max
	if err == nil {
		t.Error("Expected error when scaling above maximum")
	}

	// Test metrics
	component.UpdateMetrics(0.75, 10, 2, 15*time.Millisecond, 50.0)
	metrics := component.GetMetrics()

	if metrics["buffer_utilization"] != 0.75 {
		t.Errorf("Expected buffer_utilization 0.75, got %v", metrics["buffer_utilization"])
	}

	if metrics["overflow_count"] != int64(10) {
		t.Errorf("Expected overflow_count 10, got %v", metrics["overflow_count"])
	}

	if metrics["underflow_count"] != int64(2) {
		t.Errorf("Expected underflow_count 2, got %v", metrics["underflow_count"])
	}

	if metrics["average_latency"] != 15*time.Millisecond {
		t.Errorf("Expected average_latency 15ms, got %v", metrics["average_latency"])
	}

	if metrics["throughput_mbps"] != 50.0 {
		t.Errorf("Expected throughput_mbps 50.0, got %v", metrics["throughput_mbps"])
	}

	// Test health based on utilization and overflows
	component.UpdateMetrics(0.95, 150, 5, 20*time.Millisecond, 40.0) // High utilization and overflows
	if component.IsHealthy() {
		t.Error("Component should not be healthy with high utilization and overflows")
	}

	component.UpdateMetrics(0.6, 5, 1, 10*time.Millisecond, 60.0) // Good utilization and low overflows
	if !component.IsHealthy() {
		t.Error("Component should be healthy with good utilization and low overflows")
	}
}

// Benchmark tests for scalable components
func BenchmarkWorkerPoolComponent_SetScale(b *testing.B) {
	component := NewWorkerPoolComponent("bench_worker_pool", 128, 64, 2048)
	ctx := context.Background()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		scale := 128 + (i % 100) // Vary scale between 128-227
		component.SetScale(ctx, scale)
	}
}

func BenchmarkWorkerPoolComponent_GetMetrics(b *testing.B) {
	component := NewWorkerPoolComponent("bench_worker_pool", 128, 64, 2048)
	component.UpdateMetrics(500, 1000, 50*time.Millisecond)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		component.GetMetrics()
	}
}

func BenchmarkConnectionPoolComponent_SetScale(b *testing.B) {
	component := NewConnectionPoolComponent("bench_connection_pool", 1000, 500, 5000)
	ctx := context.Background()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		scale := 1000 + (i % 1000) // Vary scale between 1000-1999
		component.SetScale(ctx, scale)
	}
}

func BenchmarkConnectionPoolComponent_GetMetrics(b *testing.B) {
	component := NewConnectionPoolComponent("bench_connection_pool", 1000, 500, 5000)
	component.UpdateMetrics(600, 200, 10000, 100, 25*time.Millisecond)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		component.GetMetrics()
	}
}

func BenchmarkBatchProcessorComponent_SetScale(b *testing.B) {
	component := NewBatchProcessorComponent("bench_batch_processor", 50, 10, 200)
	ctx := context.Background()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		scale := 50 + (i % 50) // Vary scale between 50-99
		component.SetScale(ctx, scale)
	}
}

func BenchmarkBatchProcessorComponent_GetMetrics(b *testing.B) {
	component := NewBatchProcessorComponent("bench_batch_processor", 50, 10, 200)
	component.UpdateMetrics(20, 100, 5000, 50, 100, 200*time.Millisecond)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		component.GetMetrics()
	}
}

func BenchmarkCacheComponent_SetScale(b *testing.B) {
	component := NewCacheComponent("bench_cache", 1024, 256, 4096)
	ctx := context.Background()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		scale := 1024 + (i % 1024) // Vary scale between 1024-2047
		component.SetScale(ctx, scale)
	}
}

func BenchmarkCacheComponent_GetMetrics(b *testing.B) {
	component := NewCacheComponent("bench_cache", 1024, 256, 4096)
	component.UpdateMetrics(0.8, 0.2, 0.05, 400*1024*1024, 100000)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		component.GetMetrics()
	}
}

func BenchmarkNetworkBufferComponent_SetScale(b *testing.B) {
	component := NewNetworkBufferComponent("bench_network_buffer", 64, 16, 256)
	ctx := context.Background()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		scale := 64 + (i % 64) // Vary scale between 64-127
		component.SetScale(ctx, scale)
	}
}

func BenchmarkNetworkBufferComponent_GetMetrics(b *testing.B) {
	component := NewNetworkBufferComponent("bench_network_buffer", 64, 16, 256)
	component.UpdateMetrics(0.75, 10, 2, 15*time.Millisecond, 50.0)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		component.GetMetrics()
	}
}
