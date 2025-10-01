package adaptive

import (
	"sync"
	"testing"
	"time"
)

func TestNewLoadMonitor(t *testing.T) {
	config := NewAdaptiveBitswapConfig()
	monitor := NewLoadMonitor(config)
	
	if monitor.config != config {
		t.Error("Expected monitor to reference the provided config")
	}
	
	if monitor.maxSamples != 1000 {
		t.Errorf("Expected maxSamples to be 1000, got %d", monitor.maxSamples)
	}
	
	if monitor.monitoringInterval != 10*time.Second {
		t.Errorf("Expected monitoringInterval to be 10s, got %v", monitor.monitoringInterval)
	}
}

func TestMonitorRecordRequest(t *testing.T) {
	config := NewAdaptiveBitswapConfig()
	monitor := NewLoadMonitor(config)
	
	initialRequests := monitor.requestCount
	initialOutstanding := monitor.outstandingRequests
	
	monitor.RecordRequest()
	
	if monitor.requestCount != initialRequests+1 {
		t.Errorf("Expected requestCount to increase by 1, got %d", monitor.requestCount)
	}
	
	if monitor.outstandingRequests != initialOutstanding+1 {
		t.Errorf("Expected outstandingRequests to increase by 1, got %d", monitor.outstandingRequests)
	}
}

func TestMonitorRecordResponse(t *testing.T) {
	config := NewAdaptiveBitswapConfig()
	monitor := NewLoadMonitor(config)
	
	// Record a request first
	monitor.RecordRequest()
	
	initialOutstanding := monitor.outstandingRequests
	responseTime := 50 * time.Millisecond
	
	monitor.RecordResponse(responseTime)
	
	if monitor.outstandingRequests != initialOutstanding-1 {
		t.Errorf("Expected outstandingRequests to decrease by 1, got %d", monitor.outstandingRequests)
	}
	
	if monitor.responseTimeCount != 1 {
		t.Errorf("Expected responseTimeCount to be 1, got %d", monitor.responseTimeCount)
	}
	
	if len(monitor.responseTimes) != 1 {
		t.Errorf("Expected responseTimes length to be 1, got %d", len(monitor.responseTimes))
	}
	
	if monitor.responseTimes[0] != responseTime {
		t.Errorf("Expected first response time to be %v, got %v", responseTime, monitor.responseTimes[0])
	}
}

func TestRecordConnectionChange(t *testing.T) {
	config := NewAdaptiveBitswapConfig()
	monitor := NewLoadMonitor(config)
	
	initialConnections := monitor.activeConnections
	
	monitor.RecordConnectionChange(5)
	
	if monitor.activeConnections != initialConnections+5 {
		t.Errorf("Expected activeConnections to increase by 5, got %d", monitor.activeConnections)
	}
	
	monitor.RecordConnectionChange(-2)
	
	if monitor.activeConnections != initialConnections+3 {
		t.Errorf("Expected activeConnections to be %d, got %d", initialConnections+3, monitor.activeConnections)
	}
}

func TestCalculateP95ResponseTime(t *testing.T) {
	config := NewAdaptiveBitswapConfig()
	monitor := NewLoadMonitor(config)
	
	// Test with no response times
	p95 := monitor.calculateP95ResponseTime()
	if p95 != 0 {
		t.Errorf("Expected P95 to be 0 with no data, got %v", p95)
	}
	
	// Add response times
	responseTimes := []time.Duration{
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
	
	for _, rt := range responseTimes {
		monitor.RecordResponse(rt)
	}
	
	p95 = monitor.calculateP95ResponseTime()
	
	// P95 of 10 values should be the 95th percentile
	// P95 index = 10 * 0.95 = 9.5, rounded down to 9 (0-indexed), which is the 10th value = 100ms
	expected := 100 * time.Millisecond
	if p95 != expected {
		t.Errorf("Expected P95 to be %v, got %v", expected, p95)
	}
}

func TestCalculateP95ResponseTimeWithLargeDataset(t *testing.T) {
	config := NewAdaptiveBitswapConfig()
	monitor := NewLoadMonitor(config)
	
	// Add 100 response times from 1ms to 100ms
	for i := 1; i <= 100; i++ {
		monitor.RecordResponse(time.Duration(i) * time.Millisecond)
	}
	
	p95 := monitor.calculateP95ResponseTime()
	
	// P95 of 100 values should be around the 95th percentile
	// P95 index = 100 * 0.95 = 95, so it's the 96th value (1-indexed) = 96ms
	expected := 96 * time.Millisecond
	if p95 != expected {
		t.Errorf("Expected P95 to be %v, got %v", expected, p95)
	}
}

func TestResponseTimeCircularBuffer(t *testing.T) {
	config := NewAdaptiveBitswapConfig()
	monitor := NewLoadMonitor(config)
	monitor.maxSamples = 5 // Small buffer for testing
	
	// Add more response times than buffer size
	for i := 1; i <= 10; i++ {
		monitor.RecordResponse(time.Duration(i) * time.Millisecond)
	}
	
	// Should only keep the last 5 samples
	if len(monitor.responseTimes) != 5 {
		t.Errorf("Expected responseTimes length to be 5, got %d", len(monitor.responseTimes))
	}
	
	// Should contain the last 5 values (6ms to 10ms)
	expectedTimes := []time.Duration{
		6 * time.Millisecond,
		7 * time.Millisecond,
		8 * time.Millisecond,
		9 * time.Millisecond,
		10 * time.Millisecond,
	}
	
	for i, expected := range expectedTimes {
		if monitor.responseTimes[i] != expected {
			t.Errorf("Expected responseTimes[%d] to be %v, got %v", i, expected, monitor.responseTimes[i])
		}
	}
}

func TestRequestTracker(t *testing.T) {
	config := NewAdaptiveBitswapConfig()
	monitor := NewLoadMonitor(config)
	
	tracker := monitor.NewRequestTracker()
	
	if tracker.monitor != monitor {
		t.Error("Expected tracker to reference the monitor")
	}
	
	if tracker.startTime.IsZero() {
		t.Error("Expected tracker to have a start time")
	}
	
	// Simulate some processing time
	time.Sleep(10 * time.Millisecond)
	
	initialOutstanding := monitor.outstandingRequests
	tracker.Complete()
	
	if monitor.outstandingRequests != initialOutstanding-1 {
		t.Error("Expected outstanding requests to decrease after completion")
	}
	
	if len(monitor.responseTimes) != 1 {
		t.Error("Expected one response time to be recorded")
	}
	
	if monitor.responseTimes[0] < 10*time.Millisecond {
		t.Errorf("Expected response time to be at least 10ms, got %v", monitor.responseTimes[0])
	}
}

func TestRequestTrackerWithError(t *testing.T) {
	config := NewAdaptiveBitswapConfig()
	monitor := NewLoadMonitor(config)
	
	tracker := monitor.NewRequestTracker()
	
	// Simulate some processing time
	time.Sleep(5 * time.Millisecond)
	
	initialOutstanding := monitor.outstandingRequests
	tracker.CompleteWithError()
	
	if monitor.outstandingRequests != initialOutstanding-1 {
		t.Error("Expected outstanding requests to decrease after error completion")
	}
	
	if len(monitor.responseTimes) != 1 {
		t.Error("Expected one response time to be recorded even for errors")
	}
}

func TestGetCurrentMetrics(t *testing.T) {
	config := NewAdaptiveBitswapConfig()
	monitor := NewLoadMonitor(config)
	
	// Set up some test data
	monitor.RecordRequest()
	monitor.RecordRequest()
	monitor.RecordConnectionChange(5)
	
	// Update config metrics
	config.UpdateMetrics(100.0, 50*time.Millisecond, 100*time.Millisecond, 2, 5)
	
	metrics := monitor.GetCurrentMetrics()
	
	if metrics.OutstandingRequests != 2 {
		t.Errorf("Expected OutstandingRequests to be 2, got %d", metrics.OutstandingRequests)
	}
	
	if metrics.ActiveConnections != 5 {
		t.Errorf("Expected ActiveConnections to be 5, got %d", metrics.ActiveConnections)
	}
	
	if metrics.RequestsPerSecond != 100.0 {
		t.Errorf("Expected RequestsPerSecond to be 100.0, got %f", metrics.RequestsPerSecond)
	}
	
	if metrics.AverageResponseTime != 50*time.Millisecond {
		t.Errorf("Expected AverageResponseTime to be 50ms, got %v", metrics.AverageResponseTime)
	}
}

func TestConcurrentAccess(t *testing.T) {
	config := NewAdaptiveBitswapConfig()
	monitor := NewLoadMonitor(config)
	
	var wg sync.WaitGroup
	
	// Concurrent request recording
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < 100; j++ {
				monitor.RecordRequest()
				monitor.RecordResponse(time.Duration(j) * time.Millisecond)
			}
		}()
	}
	
	// Concurrent connection changes
	for i := 0; i < 5; i++ {
		wg.Add(1)
		go func(delta int) {
			defer wg.Done()
			for j := 0; j < 50; j++ {
				monitor.RecordConnectionChange(delta)
				monitor.RecordConnectionChange(-delta)
			}
		}(i + 1)
	}
	
	// Concurrent metrics reading
	for i := 0; i < 5; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < 100; j++ {
				monitor.GetCurrentMetrics()
				monitor.calculateP95ResponseTime()
			}
		}()
	}
	
	wg.Wait()
	
	// Verify final state is consistent
	metrics := monitor.GetCurrentMetrics()
	if metrics.OutstandingRequests < 0 {
		t.Errorf("Outstanding requests should not be negative: %d", metrics.OutstandingRequests)
	}
	
	// Should have recorded many response times
	if len(monitor.responseTimes) == 0 {
		t.Error("Expected some response times to be recorded")
	}
}

func TestMonitorStartStop(t *testing.T) {
	config := NewAdaptiveBitswapConfig()
	monitor := NewLoadMonitor(config)
	monitor.monitoringInterval = 50 * time.Millisecond // Fast interval for testing
	
	// Start monitoring
	monitor.Start()
	
	// Let it run for a bit
	time.Sleep(100 * time.Millisecond)
	
	// Stop monitoring
	monitor.Stop()
	
	// Verify it stopped cleanly (no panic or deadlock)
	// If we reach here, the test passes
}