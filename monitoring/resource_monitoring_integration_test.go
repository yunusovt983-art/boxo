package monitoring

import (
	"context"
	"testing"
	"time"
)

// TestResourceMonitoringIntegration tests the complete resource monitoring system
func TestResourceMonitoringIntegration(t *testing.T) {
	// Create resource collector
	collector := NewResourceCollector()

	// Set up custom thresholds for testing
	thresholds := ResourceMonitorThresholds{
		CPUWarning:        60.0,
		CPUCritical:       85.0,
		MemoryWarning:     0.7,  // 70%
		MemoryCritical:    0.9,  // 90%
		DiskWarning:       0.8,  // 80%
		DiskCritical:      0.95, // 95%
		GoroutineWarning:  5000,
		GoroutineCritical: 10000,
	}

	// Create resource monitor
	monitor := NewResourceMonitor(collector, thresholds)

	// Set short cooldown for testing
	monitor.alertCooldown = 100 * time.Millisecond

	// Set up alert tracking
	alertsReceived := make([]ResourceAlert, 0)
	monitor.SetAlertCallback(func(alert ResourceAlert) {
		alertsReceived = append(alertsReceived, alert)
		t.Logf("Alert received: %s - %s", alert.Level, alert.Message)
	})

	// Start monitoring
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	err := monitor.Start(ctx)
	if err != nil {
		t.Fatalf("Failed to start resource monitor: %v", err)
	}
	defer monitor.Stop()

	// Test 1: Normal resource usage - should not trigger alerts
	t.Log("Testing normal resource usage...")
	collector.UpdateCPUUsage(45.0)                                   // Below warning threshold
	collector.UpdateDiskUsage(50*1024*1024*1024, 100*1024*1024*1024) // 50% disk usage

	monitor.ForceCollection()
	time.Sleep(100 * time.Millisecond) // Allow processing

	if len(alertsReceived) > 0 {
		t.Errorf("Expected no alerts for normal usage, got %d", len(alertsReceived))
	}

	// Test 2: High CPU usage - should trigger warning alert
	t.Log("Testing high CPU usage...")
	collector.UpdateCPUUsage(75.0) // Above warning threshold

	monitor.ForceCollection()
	time.Sleep(100 * time.Millisecond) // Allow processing

	if len(alertsReceived) == 0 {
		t.Error("Expected CPU warning alert")
	} else {
		lastAlert := alertsReceived[len(alertsReceived)-1]
		if lastAlert.Type != "cpu" || lastAlert.Level != "warning" {
			t.Errorf("Expected CPU warning alert, got %s %s", lastAlert.Type, lastAlert.Level)
		}
		if lastAlert.Prediction == nil {
			t.Error("Expected prediction data in alert")
		}
	}

	// Test 3: Critical CPU usage - should trigger critical alert
	t.Log("Testing critical CPU usage...")
	collector.UpdateCPUUsage(90.0) // Above critical threshold

	// Wait for cooldown to expire
	time.Sleep(200 * time.Millisecond)

	monitor.ForceCollection()
	time.Sleep(100 * time.Millisecond) // Allow processing

	foundCritical := false
	for _, alert := range alertsReceived {
		if alert.Type == "cpu" && alert.Level == "critical" {
			foundCritical = true
			break
		}
	}

	if !foundCritical {
		t.Error("Expected CPU critical alert")
	}

	// Test 4: Check health status
	t.Log("Testing health status...")
	health := monitor.GetHealthStatus()

	if health["health"] != "critical" {
		t.Errorf("Expected critical health status, got %s", health["health"])
	}

	issues := health["issues"].([]string)
	if len(issues) == 0 {
		t.Error("Expected health issues to be reported")
	}

	// Test 5: Check current status with trends
	t.Log("Testing current status with trends...")
	status := monitor.GetCurrentStatus()

	if status == nil {
		t.Fatal("Expected non-nil status")
	}

	// Should include trend information
	if _, exists := status["cpu_trend"]; !exists {
		t.Error("Expected CPU trend information")
	}

	if _, exists := status["memory_trend"]; !exists {
		t.Error("Expected memory trend information")
	}

	// Test 6: Check resource history
	t.Log("Testing resource history...")
	cpuHistory := monitor.GetResourceHistory("cpu")
	if cpuHistory == nil {
		t.Error("Expected CPU history")
	}

	if len(cpuHistory.Values) == 0 {
		t.Error("Expected CPU history data")
	}

	// Test 7: Test predictive analysis
	t.Log("Testing predictive analysis...")

	// Add trend data to trigger prediction
	now := time.Now()
	for i := 0; i < 10; i++ {
		monitor.cpuHistory.Add(now.Add(time.Duration(i)*time.Second), float64(70+i*2))
	}

	prediction := monitor.getPrediction("cpu", 88.0, 85.0)
	if prediction == nil {
		t.Error("Expected prediction for trending CPU usage")
	} else {
		if prediction.TrendDirection != "increasing" {
			t.Errorf("Expected increasing trend, got %s", prediction.TrendDirection)
		}
		if prediction.RecommendedAction == "" {
			t.Error("Expected recommended action")
		}
	}

	// Test 8: Test threshold updates
	t.Log("Testing threshold updates...")
	newThresholds := ResourceMonitorThresholds{
		CPUWarning:        80.0, // Higher threshold
		CPUCritical:       95.0,
		MemoryWarning:     0.8,
		MemoryCritical:    0.95,
		DiskWarning:       0.9,
		DiskCritical:      0.98,
		GoroutineWarning:  8000,
		GoroutineCritical: 15000,
	}

	monitor.UpdateThresholds(newThresholds)

	updatedThresholds := monitor.GetThresholds()
	if updatedThresholds.CPUWarning != 80.0 {
		t.Error("Expected thresholds to be updated")
	}

	t.Logf("Integration test completed successfully. Received %d alerts total.", len(alertsReceived))
}

// TestResourceMonitoringPerformance tests the performance of the monitoring system
func TestResourceMonitoringPerformance(t *testing.T) {
	collector := NewResourceCollector()
	thresholds := DefaultResourceMonitorThresholds()
	monitor := NewResourceMonitor(collector, thresholds)

	// Set fast monitoring interval for testing
	monitor.SetMonitorInterval(10 * time.Millisecond)

	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
	defer cancel()

	// Start monitoring
	err := monitor.Start(ctx)
	if err != nil {
		t.Fatalf("Failed to start resource monitor: %v", err)
	}
	defer monitor.Stop()

	// Simulate changing resource usage
	go func() {
		for i := 0; i < 100; i++ {
			collector.UpdateCPUUsage(float64(30 + i%40)) // Vary between 30-70%
			collector.UpdateDiskUsage(
				int64((50+i%30)*1024*1024*1024), // Vary between 50-80GB
				100*1024*1024*1024,              // 100GB total
			)
			time.Sleep(5 * time.Millisecond)
		}
	}()

	// Wait for monitoring to complete
	<-ctx.Done()

	// Check that data was collected
	cpuHistory := monitor.GetResourceHistory("cpu")
	if len(cpuHistory.Values) == 0 {
		t.Error("Expected CPU history data to be collected")
	}

	diskHistory := monitor.GetResourceHistory("disk")
	if len(diskHistory.Values) == 0 {
		t.Error("Expected disk history data to be collected")
	}

	t.Logf("Performance test completed. Collected %d CPU data points and %d disk data points",
		len(cpuHistory.Values), len(diskHistory.Values))
}

// TestResourceMonitoringMemoryLeaks tests for memory leaks in long-running monitoring
func TestResourceMonitoringMemoryLeaks(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping memory leak test in short mode")
	}

	collector := NewResourceCollector()
	thresholds := DefaultResourceMonitorThresholds()
	monitor := NewResourceMonitor(collector, thresholds)

	// Set fast monitoring interval
	monitor.SetMonitorInterval(1 * time.Millisecond)

	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	// Start monitoring
	err := monitor.Start(ctx)
	if err != nil {
		t.Fatalf("Failed to start resource monitor: %v", err)
	}
	defer monitor.Stop()

	// Simulate continuous resource updates
	go func() {
		for {
			select {
			case <-ctx.Done():
				return
			default:
				collector.UpdateCPUUsage(float64(50 + (time.Now().UnixNano() % 40)))
				time.Sleep(100 * time.Microsecond)
			}
		}
	}()

	// Wait for monitoring to complete
	<-ctx.Done()

	// Check that history doesn't grow unbounded
	cpuHistory := monitor.GetResourceHistory("cpu")
	if len(cpuHistory.Values) > cpuHistory.MaxSize {
		t.Errorf("CPU history exceeded max size: %d > %d", len(cpuHistory.Values), cpuHistory.MaxSize)
	}

	memoryHistory := monitor.GetResourceHistory("memory")
	if len(memoryHistory.Values) > memoryHistory.MaxSize {
		t.Errorf("Memory history exceeded max size: %d > %d", len(memoryHistory.Values), memoryHistory.MaxSize)
	}

	t.Log("Memory leak test completed successfully")
}
