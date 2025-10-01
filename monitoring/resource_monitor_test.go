package monitoring

import (
	"context"
	"testing"
	"time"
)

func TestNewResourceMonitor(t *testing.T) {
	collector := NewResourceCollector()
	thresholds := DefaultResourceMonitorThresholds()

	monitor := NewResourceMonitor(collector, thresholds)

	if monitor == nil {
		t.Fatal("Expected non-nil ResourceMonitor")
	}

	if monitor.collector != collector {
		t.Error("Expected collector to be set correctly")
	}

	if monitor.thresholds != thresholds {
		t.Error("Expected thresholds to be set correctly")
	}

	if monitor.cpuHistory == nil || monitor.memoryHistory == nil ||
		monitor.diskHistory == nil || monitor.goroutineHistory == nil {
		t.Error("Expected all history trackers to be initialized")
	}
}

func TestDefaultResourceThresholds(t *testing.T) {
	thresholds := DefaultResourceMonitorThresholds()

	if thresholds.CPUWarning <= 0 || thresholds.CPUWarning >= 100 {
		t.Error("CPU warning threshold should be between 0 and 100")
	}

	if thresholds.CPUCritical <= thresholds.CPUWarning {
		t.Error("CPU critical threshold should be higher than warning")
	}

	if thresholds.MemoryWarning <= 0 || thresholds.MemoryWarning >= 1 {
		t.Error("Memory warning threshold should be between 0 and 1")
	}

	if thresholds.MemoryCritical <= thresholds.MemoryWarning {
		t.Error("Memory critical threshold should be higher than warning")
	}

	if thresholds.GoroutineWarning <= 0 {
		t.Error("Goroutine warning threshold should be positive")
	}

	if thresholds.GoroutineCritical <= thresholds.GoroutineWarning {
		t.Error("Goroutine critical threshold should be higher than warning")
	}
}

func TestResourceHistory(t *testing.T) {
	history := NewResourceHistory(3)

	if len(history.Values) != 0 {
		t.Error("Expected empty history initially")
	}

	// Add some data points
	now := time.Now()
	history.Add(now, 10.0)
	history.Add(now.Add(time.Second), 20.0)
	history.Add(now.Add(2*time.Second), 30.0)

	if len(history.Values) != 3 {
		t.Error("Expected 3 values in history")
	}

	// Add one more to test size limit
	history.Add(now.Add(3*time.Second), 40.0)

	if len(history.Values) != 3 {
		t.Error("Expected history to maintain max size of 3")
	}

	if history.Values[0] != 20.0 {
		t.Error("Expected oldest value to be removed")
	}

	if history.Values[2] != 40.0 {
		t.Error("Expected newest value to be at the end")
	}
}

func TestResourceHistoryTrend(t *testing.T) {
	history := NewResourceHistory(10)

	// Test stable trend
	for i := 0; i < 5; i++ {
		history.Add(time.Now().Add(time.Duration(i)*time.Second), 50.0)
	}

	direction, slope := history.GetTrend()
	if direction != "stable" {
		t.Errorf("Expected stable trend, got %s", direction)
	}

	// Test increasing trend
	history = NewResourceHistory(10)
	for i := 0; i < 5; i++ {
		history.Add(time.Now().Add(time.Duration(i)*time.Second), float64(i*10))
	}

	direction, slope = history.GetTrend()
	if direction != "increasing" {
		t.Errorf("Expected increasing trend, got %s", direction)
	}

	if slope <= 0 {
		t.Error("Expected positive slope for increasing trend")
	}

	// Test decreasing trend
	history = NewResourceHistory(10)
	for i := 0; i < 5; i++ {
		history.Add(time.Now().Add(time.Duration(i)*time.Second), float64(50-i*10))
	}

	direction, slope = history.GetTrend()
	if direction != "decreasing" {
		t.Errorf("Expected decreasing trend, got %s", direction)
	}

	if slope >= 0 {
		t.Error("Expected negative slope for decreasing trend")
	}
}

func TestResourceMonitorStartStop(t *testing.T) {
	collector := NewResourceCollector()
	thresholds := DefaultResourceMonitorThresholds()
	monitor := NewResourceMonitor(collector, thresholds)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Test start
	err := monitor.Start(ctx)
	if err != nil {
		t.Fatalf("Failed to start monitor: %v", err)
	}

	if !monitor.isRunning {
		t.Error("Expected monitor to be running")
	}

	// Test double start
	err = monitor.Start(ctx)
	if err == nil {
		t.Error("Expected error when starting already running monitor")
	}

	// Test stop
	monitor.Stop()

	if monitor.isRunning {
		t.Error("Expected monitor to be stopped")
	}
}

func TestResourceMonitorAlerts(t *testing.T) {
	collector := NewResourceCollector()
	thresholds := ResourceMonitorThresholds{
		CPUWarning:        50.0,
		CPUCritical:       80.0,
		MemoryWarning:     0.6,
		MemoryCritical:    0.8,
		DiskWarning:       0.7,
		DiskCritical:      0.9,
		GoroutineWarning:  1000,
		GoroutineCritical: 5000,
	}

	monitor := NewResourceMonitor(collector, thresholds)

	alertReceived := false
	var receivedAlert ResourceAlert

	monitor.SetAlertCallback(func(alert ResourceAlert) {
		alertReceived = true
		receivedAlert = alert
	})

	// Simulate high CPU usage
	collector.UpdateCPUUsage(85.0) // Above critical threshold

	monitor.collectAndAnalyze()

	if !alertReceived {
		t.Error("Expected alert to be received")
	}

	if receivedAlert.Type != "cpu" {
		t.Errorf("Expected CPU alert, got %s", receivedAlert.Type)
	}

	if receivedAlert.Level != "critical" {
		t.Errorf("Expected critical alert, got %s", receivedAlert.Level)
	}

	if receivedAlert.Value != 85.0 {
		t.Errorf("Expected alert value 85.0, got %f", receivedAlert.Value)
	}
}

func TestResourceMonitorPrediction(t *testing.T) {
	collector := NewResourceCollector()
	thresholds := DefaultResourceMonitorThresholds()
	monitor := NewResourceMonitor(collector, thresholds)
	monitor.SetMonitorInterval(time.Second) // Fast interval for testing

	// Add increasing trend data
	now := time.Now()
	for i := 0; i < 10; i++ {
		monitor.cpuHistory.Add(now.Add(time.Duration(i)*time.Second), float64(i*5))
	}

	prediction := monitor.getPrediction("cpu", 45.0, 90.0)

	if prediction == nil {
		t.Fatal("Expected non-nil prediction")
	}

	if prediction.TrendDirection != "increasing" {
		t.Errorf("Expected increasing trend, got %s", prediction.TrendDirection)
	}

	if prediction.TimeToThreshold <= 0 {
		t.Error("Expected positive time to threshold")
	}

	if prediction.Confidence <= 0 || prediction.Confidence > 1 {
		t.Errorf("Expected confidence between 0 and 1, got %f", prediction.Confidence)
	}

	if prediction.RecommendedAction == "" {
		t.Error("Expected non-empty recommended action")
	}
}

func TestResourceMonitorHealthStatus(t *testing.T) {
	collector := NewResourceCollector()
	thresholds := ResourceMonitorThresholds{
		CPUWarning:        50.0,
		CPUCritical:       80.0,
		MemoryWarning:     0.6,
		MemoryCritical:    0.8,
		DiskWarning:       0.7,
		DiskCritical:      0.9,
		GoroutineWarning:  1000,
		GoroutineCritical: 5000,
	}

	monitor := NewResourceMonitor(collector, thresholds)

	// Test healthy status
	collector.UpdateCPUUsage(30.0)
	collector.UpdateDiskUsage(100*1024*1024*1024, 1000*1024*1024*1024) // 10% disk usage

	health := monitor.GetHealthStatus()

	if health["health"] != "healthy" {
		t.Errorf("Expected healthy status, got %s", health["health"])
	}

	issues := health["issues"].([]string)
	if len(issues) != 0 {
		t.Errorf("Expected no issues, got %v", issues)
	}

	// Test warning status
	collector.UpdateCPUUsage(60.0) // Above warning threshold

	health = monitor.GetHealthStatus()

	if health["health"] != "warning" {
		t.Errorf("Expected warning status, got %s", health["health"])
	}

	issues = health["issues"].([]string)
	if len(issues) == 0 {
		t.Error("Expected at least one issue")
	}

	// Test critical status
	collector.UpdateCPUUsage(90.0) // Above critical threshold

	health = monitor.GetHealthStatus()

	if health["health"] != "critical" {
		t.Errorf("Expected critical status, got %s", health["health"])
	}
}

func TestResourceMonitorThresholdUpdates(t *testing.T) {
	collector := NewResourceCollector()
	thresholds := DefaultResourceMonitorThresholds()
	monitor := NewResourceMonitor(collector, thresholds)

	// Test getting thresholds
	currentThresholds := monitor.GetThresholds()
	if currentThresholds.CPUWarning != thresholds.CPUWarning {
		t.Error("Expected thresholds to match")
	}

	// Test updating thresholds
	newThresholds := ResourceMonitorThresholds{
		CPUWarning:        60.0,
		CPUCritical:       85.0,
		MemoryWarning:     0.7,
		MemoryCritical:    0.9,
		DiskWarning:       0.8,
		DiskCritical:      0.95,
		GoroutineWarning:  2000,
		GoroutineCritical: 10000,
	}

	monitor.UpdateThresholds(newThresholds)

	updatedThresholds := monitor.GetThresholds()
	if updatedThresholds.CPUWarning != 60.0 {
		t.Error("Expected thresholds to be updated")
	}
}

func TestResourceMonitorCurrentStatus(t *testing.T) {
	collector := NewResourceCollector()
	thresholds := DefaultResourceMonitorThresholds()
	monitor := NewResourceMonitor(collector, thresholds)

	// Add some trend data
	now := time.Now()
	for i := 0; i < 5; i++ {
		monitor.cpuHistory.Add(now.Add(time.Duration(i)*time.Second), float64(i*10))
	}

	status := monitor.GetCurrentStatus()

	if status == nil {
		t.Fatal("Expected non-nil status")
	}

	// Check that trend information is included
	if _, exists := status["cpu_trend"]; !exists {
		t.Error("Expected CPU trend information")
	}

	cpuTrend := status["cpu_trend"].(map[string]interface{})
	if cpuTrend["direction"] != "increasing" {
		t.Error("Expected increasing CPU trend")
	}
}

func TestResourceMonitorForceCollection(t *testing.T) {
	collector := NewResourceCollector()
	thresholds := DefaultResourceMonitorThresholds()
	monitor := NewResourceMonitor(collector, thresholds)

	// Force collection should not panic
	monitor.ForceCollection()

	// Check that data was collected
	if len(monitor.cpuHistory.Values) == 0 {
		t.Error("Expected data to be collected")
	}
}

func TestResourceMonitorGetResourceHistory(t *testing.T) {
	collector := NewResourceCollector()
	thresholds := DefaultResourceMonitorThresholds()
	monitor := NewResourceMonitor(collector, thresholds)

	// Test getting valid history
	cpuHistory := monitor.GetResourceHistory("cpu")
	if cpuHistory == nil {
		t.Error("Expected non-nil CPU history")
	}

	memoryHistory := monitor.GetResourceHistory("memory")
	if memoryHistory == nil {
		t.Error("Expected non-nil memory history")
	}

	diskHistory := monitor.GetResourceHistory("disk")
	if diskHistory == nil {
		t.Error("Expected non-nil disk history")
	}

	goroutineHistory := monitor.GetResourceHistory("goroutines")
	if goroutineHistory == nil {
		t.Error("Expected non-nil goroutine history")
	}

	// Test getting invalid history
	invalidHistory := monitor.GetResourceHistory("invalid")
	if invalidHistory != nil {
		t.Error("Expected nil for invalid metric type")
	}
}

func TestResourceMonitorAlertCooldown(t *testing.T) {
	collector := NewResourceCollector()
	thresholds := ResourceMonitorThresholds{
		CPUWarning:        50.0,
		CPUCritical:       80.0,
		MemoryWarning:     0.6,
		MemoryCritical:    0.8,
		DiskWarning:       0.7,
		DiskCritical:      0.9,
		GoroutineWarning:  1000,
		GoroutineCritical: 5000,
	}

	monitor := NewResourceMonitor(collector, thresholds)
	monitor.alertCooldown = 100 * time.Millisecond // Short cooldown for testing

	alertCount := 0
	monitor.SetAlertCallback(func(alert ResourceAlert) {
		alertCount++
	})

	// Simulate high CPU usage multiple times
	collector.UpdateCPUUsage(85.0)

	monitor.collectAndAnalyze()
	if alertCount != 1 {
		t.Errorf("Expected 1 alert, got %d", alertCount)
	}

	// Immediate second collection should not generate alert due to cooldown
	monitor.collectAndAnalyze()
	if alertCount != 1 {
		t.Errorf("Expected 1 alert due to cooldown, got %d", alertCount)
	}

	// Wait for cooldown to expire
	time.Sleep(150 * time.Millisecond)

	monitor.collectAndAnalyze()
	if alertCount != 2 {
		t.Errorf("Expected 2 alerts after cooldown, got %d", alertCount)
	}
}

// Benchmark tests
func BenchmarkResourceMonitorCollectAndAnalyze(b *testing.B) {
	collector := NewResourceCollector()
	thresholds := DefaultResourceMonitorThresholds()
	monitor := NewResourceMonitor(collector, thresholds)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		monitor.collectAndAnalyze()
	}
}

func BenchmarkResourceHistoryAdd(b *testing.B) {
	history := NewResourceHistory(1000)
	now := time.Now()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		history.Add(now.Add(time.Duration(i)*time.Second), float64(i))
	}
}

func BenchmarkResourceHistoryGetTrend(b *testing.B) {
	history := NewResourceHistory(100)
	now := time.Now()

	// Fill with data
	for i := 0; i < 100; i++ {
		history.Add(now.Add(time.Duration(i)*time.Second), float64(i))
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		history.GetTrend()
	}
}
