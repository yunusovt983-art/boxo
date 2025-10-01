package monitoring

import (
	"context"
	"testing"
	"time"
)

func TestNewBitswapCollector(t *testing.T) {
	collector := NewBitswapCollector()
	
	if collector == nil {
		t.Fatal("NewBitswapCollector returned nil")
	}
	
	names := collector.GetMetricNames()
	expectedNames := []string{
		"requests_per_second",
		"average_response_time",
		"p95_response_time",
		"p99_response_time",
		"outstanding_requests",
		"active_connections",
		"queued_requests",
		"blocks_received",
		"blocks_sent",
		"duplicate_blocks",
		"error_rate",
		"total_requests",
	}
	
	if len(names) != len(expectedNames) {
		t.Errorf("Expected %d metric names, got %d", len(expectedNames), len(names))
	}
	
	for i, expected := range expectedNames {
		if i >= len(names) || names[i] != expected {
			t.Errorf("Expected metric name %s at index %d, got %s", expected, i, names[i])
		}
	}
}

func TestBitswapCollectorRecordRequest(t *testing.T) {
	collector := NewBitswapCollector()
	
	// Record some requests
	collector.RecordRequest()
	collector.RecordRequest()
	collector.RecordRequest()
	
	metrics, err := collector.CollectMetrics(context.Background())
	if err != nil {
		t.Fatalf("CollectMetrics failed: %v", err)
	}
	
	totalRequests, ok := metrics["total_requests"].(int64)
	if !ok {
		t.Fatal("total_requests metric not found or wrong type")
	}
	
	if totalRequests != 3 {
		t.Errorf("Expected total_requests=3, got %d", totalRequests)
	}
	
	outstandingRequests, ok := metrics["outstanding_requests"].(int64)
	if !ok {
		t.Fatal("outstanding_requests metric not found or wrong type")
	}
	
	if outstandingRequests != 3 {
		t.Errorf("Expected outstanding_requests=3, got %d", outstandingRequests)
	}
}

func TestBitswapCollectorRecordResponse(t *testing.T) {
	collector := NewBitswapCollector()
	
	// Record requests and responses
	collector.RecordRequest()
	collector.RecordRequest()
	
	collector.RecordResponse(50*time.Millisecond, true)
	collector.RecordResponse(100*time.Millisecond, false) // Error response
	
	metrics, err := collector.CollectMetrics(context.Background())
	if err != nil {
		t.Fatalf("CollectMetrics failed: %v", err)
	}
	
	avgResponseTime, ok := metrics["average_response_time"].(time.Duration)
	if !ok {
		t.Fatal("average_response_time metric not found or wrong type")
	}
	
	expectedAvg := (50*time.Millisecond + 100*time.Millisecond) / 2
	if avgResponseTime != expectedAvg {
		t.Errorf("Expected average_response_time=%v, got %v", expectedAvg, avgResponseTime)
	}
	
	errorRate, ok := metrics["error_rate"].(float64)
	if !ok {
		t.Fatal("error_rate metric not found or wrong type")
	}
	
	expectedErrorRate := 1.0 / 2.0 // 1 error out of 2 total requests
	if errorRate != expectedErrorRate {
		t.Errorf("Expected error_rate=%f, got %f", expectedErrorRate, errorRate)
	}
	
	outstandingRequests, ok := metrics["outstanding_requests"].(int64)
	if !ok {
		t.Fatal("outstanding_requests metric not found or wrong type")
	}
	
	if outstandingRequests != 0 {
		t.Errorf("Expected outstanding_requests=0 after responses, got %d", outstandingRequests)
	}
}

func TestBitswapCollectorRecordBlocks(t *testing.T) {
	collector := NewBitswapCollector()
	
	// Record received blocks
	collector.RecordBlockReceived(false) // Not duplicate
	collector.RecordBlockReceived(true)  // Duplicate
	collector.RecordBlockReceived(false) // Not duplicate
	
	// Record sent blocks
	collector.RecordBlockSent()
	collector.RecordBlockSent()
	
	metrics, err := collector.CollectMetrics(context.Background())
	if err != nil {
		t.Fatalf("CollectMetrics failed: %v", err)
	}
	
	blocksReceived, ok := metrics["blocks_received"].(int64)
	if !ok {
		t.Fatal("blocks_received metric not found or wrong type")
	}
	
	if blocksReceived != 3 {
		t.Errorf("Expected blocks_received=3, got %d", blocksReceived)
	}
	
	duplicateBlocks, ok := metrics["duplicate_blocks"].(int64)
	if !ok {
		t.Fatal("duplicate_blocks metric not found or wrong type")
	}
	
	if duplicateBlocks != 1 {
		t.Errorf("Expected duplicate_blocks=1, got %d", duplicateBlocks)
	}
	
	blocksSent, ok := metrics["blocks_sent"].(int64)
	if !ok {
		t.Fatal("blocks_sent metric not found or wrong type")
	}
	
	if blocksSent != 2 {
		t.Errorf("Expected blocks_sent=2, got %d", blocksSent)
	}
}

func TestBitswapCollectorUpdateConnections(t *testing.T) {
	collector := NewBitswapCollector()
	
	collector.UpdateActiveConnections(15)
	collector.UpdateQueuedRequests(25)
	
	metrics, err := collector.CollectMetrics(context.Background())
	if err != nil {
		t.Fatalf("CollectMetrics failed: %v", err)
	}
	
	activeConnections, ok := metrics["active_connections"].(int)
	if !ok {
		t.Fatal("active_connections metric not found or wrong type")
	}
	
	if activeConnections != 15 {
		t.Errorf("Expected active_connections=15, got %d", activeConnections)
	}
	
	queuedRequests, ok := metrics["queued_requests"].(int64)
	if !ok {
		t.Fatal("queued_requests metric not found or wrong type")
	}
	
	if queuedRequests != 25 {
		t.Errorf("Expected queued_requests=25, got %d", queuedRequests)
	}
}

func TestBitswapCollectorPercentiles(t *testing.T) {
	collector := NewBitswapCollector()
	
	// Record requests and responses with varying durations
	durations := []time.Duration{
		10 * time.Millisecond,
		20 * time.Millisecond,
		30 * time.Millisecond,
		40 * time.Millisecond,
		50 * time.Millisecond,
		60 * time.Millisecond,
		70 * time.Millisecond,
		80 * time.Millisecond,
		90 * time.Millisecond,
		100 * time.Millisecond,
	}
	
	for _, duration := range durations {
		collector.RecordRequest()
		collector.RecordResponse(duration, true)
	}
	
	metrics, err := collector.CollectMetrics(context.Background())
	if err != nil {
		t.Fatalf("CollectMetrics failed: %v", err)
	}
	
	p95ResponseTime, ok := metrics["p95_response_time"].(time.Duration)
	if !ok {
		t.Fatal("p95_response_time metric not found or wrong type")
	}
	
	p99ResponseTime, ok := metrics["p99_response_time"].(time.Duration)
	if !ok {
		t.Fatal("p99_response_time metric not found or wrong type")
	}
	
	// P95 should be around 95ms (95th percentile of 10 values = index 9)
	// P99 should be around 99ms (99th percentile of 10 values = index 9)
	if p95ResponseTime != 100*time.Millisecond {
		t.Errorf("Expected p95_response_time=100ms, got %v", p95ResponseTime)
	}
	
	if p99ResponseTime != 100*time.Millisecond {
		t.Errorf("Expected p99_response_time=100ms, got %v", p99ResponseTime)
	}
}

func TestBitswapCollectorConcurrentAccess(t *testing.T) {
	collector := NewBitswapCollector()
	
	done := make(chan bool, 3)
	
	// Goroutine 1: Record requests
	go func() {
		for i := 0; i < 100; i++ {
			collector.RecordRequest()
			time.Sleep(time.Microsecond)
		}
		done <- true
	}()
	
	// Goroutine 2: Record responses
	go func() {
		for i := 0; i < 100; i++ {
			collector.RecordResponse(time.Duration(i)*time.Microsecond, i%10 != 0)
			time.Sleep(time.Microsecond)
		}
		done <- true
	}()
	
	// Goroutine 3: Collect metrics
	go func() {
		for i := 0; i < 50; i++ {
			collector.CollectMetrics(context.Background())
			time.Sleep(2 * time.Microsecond)
		}
		done <- true
	}()
	
	// Wait for all goroutines to complete
	for i := 0; i < 3; i++ {
		<-done
	}
	
	// Final metrics collection should not panic
	metrics, err := collector.CollectMetrics(context.Background())
	if err != nil {
		t.Fatalf("Final CollectMetrics failed: %v", err)
	}
	
	if metrics == nil {
		t.Fatal("Final metrics is nil")
	}
}

func TestBitswapCollectorResetCounters(t *testing.T) {
	collector := NewBitswapCollector()
	
	// Record some data
	collector.RecordRequest()
	collector.RecordResponse(50*time.Millisecond, true)
	
	// First collection
	metrics1, err := collector.CollectMetrics(context.Background())
	if err != nil {
		t.Fatalf("First CollectMetrics failed: %v", err)
	}
	
	avgResponseTime1, ok := metrics1["average_response_time"].(time.Duration)
	if !ok || avgResponseTime1 != 50*time.Millisecond {
		t.Errorf("Expected first average_response_time=50ms, got %v", avgResponseTime1)
	}
	
	// Second collection without new data should show reset counters
	metrics2, err := collector.CollectMetrics(context.Background())
	if err != nil {
		t.Fatalf("Second CollectMetrics failed: %v", err)
	}
	
	avgResponseTime2, ok := metrics2["average_response_time"].(time.Duration)
	if !ok || avgResponseTime2 != 0 {
		t.Errorf("Expected second average_response_time=0 (reset), got %v", avgResponseTime2)
	}
	
	// But total_requests should persist
	totalRequests2, ok := metrics2["total_requests"].(int64)
	if !ok || totalRequests2 != 1 {
		t.Errorf("Expected total_requests=1 (persistent), got %d", totalRequests2)
	}
}

func BenchmarkBitswapCollectorRecordRequest(b *testing.B) {
	collector := NewBitswapCollector()
	
	b.ResetTimer()
	
	for i := 0; i < b.N; i++ {
		collector.RecordRequest()
	}
}

func BenchmarkBitswapCollectorRecordResponse(b *testing.B) {
	collector := NewBitswapCollector()
	
	b.ResetTimer()
	
	for i := 0; i < b.N; i++ {
		collector.RecordResponse(time.Duration(i)*time.Nanosecond, true)
	}
}

func BenchmarkBitswapCollectorCollectMetrics(b *testing.B) {
	collector := NewBitswapCollector()
	
	// Pre-populate with some data
	for i := 0; i < 1000; i++ {
		collector.RecordRequest()
		collector.RecordResponse(time.Duration(i)*time.Microsecond, i%10 != 0)
	}
	
	b.ResetTimer()
	
	for i := 0; i < b.N; i++ {
		collector.CollectMetrics(context.Background())
	}
}