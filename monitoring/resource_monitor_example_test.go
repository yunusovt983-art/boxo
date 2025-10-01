package monitoring

import (
	"context"
	"fmt"
	"log"
	"time"
)

// Example_resourceMonitorBasicUsage demonstrates basic ResourceMonitor usage
func Example_resourceMonitorBasicUsage() {
	// Create a resource collector
	collector := NewResourceCollector()

	// Set up thresholds
	thresholds := ResourceMonitorThresholds{
		CPUWarning:        70.0,
		CPUCritical:       90.0,
		MemoryWarning:     0.8,  // 80%
		MemoryCritical:    0.95, // 95%
		DiskWarning:       0.85, // 85%
		DiskCritical:      0.95, // 95%
		GoroutineWarning:  10000,
		GoroutineCritical: 50000,
	}

	// Create the resource monitor
	monitor := NewResourceMonitor(collector, thresholds)

	// Set up alert callback
	monitor.SetAlertCallback(func(alert ResourceAlert) {
		fmt.Printf("ALERT: %s - %s\n", alert.Level, alert.Message)
		if alert.Prediction != nil {
			fmt.Printf("Prediction: %s trend, confidence: %.2f\n",
				alert.Prediction.TrendDirection, alert.Prediction.Confidence)
		}
	})

	// Start monitoring
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	monitor.SetMonitorInterval(100 * time.Millisecond) // Fast for demo
	err := monitor.Start(ctx)
	if err != nil {
		log.Fatal(err)
	}
	defer monitor.Stop()

	// Simulate some resource usage
	collector.UpdateCPUUsage(75.0)                                     // Above warning threshold
	collector.UpdateDiskUsage(900*1024*1024*1024, 1000*1024*1024*1024) // 90% disk usage

	// Wait a bit for monitoring to detect the issues
	time.Sleep(200 * time.Millisecond)

	// Get current status
	status := monitor.GetCurrentStatus()
	fmt.Printf("CPU Usage: %.1f%%\n", status["cpu_usage_percent"])
	fmt.Printf("Disk Pressure: %.1f%%\n", status["disk_pressure"].(float64)*100)

	// Get health status
	health := monitor.GetHealthStatus()
	fmt.Printf("System Health: %s\n", health["health"])

	// Output:
	// ALERT: warning - cpu usage is high: 75.00% (threshold: 70.00%)
	// Prediction: stable trend, confidence: 0.30
	// ALERT: warning - disk usage is high: 90.00% (threshold: 85.00%)
	// Prediction: stable trend, confidence: 0.30
	// CPU Usage: 75.0%
	// Disk Pressure: 90.0%
	// System Health: warning
}

// Example_resourceMonitorPredictiveAnalysis demonstrates predictive analysis features
func Example_resourceMonitorPredictiveAnalysis() {
	collector := NewResourceCollector()
	thresholds := DefaultResourceMonitorThresholds()
	monitor := NewResourceMonitor(collector, thresholds)

	// Add historical data to simulate an increasing trend
	now := time.Now()
	for i := 0; i < 20; i++ {
		monitor.cpuHistory.Add(now.Add(time.Duration(i)*time.Second), float64(i*3)) // Increasing CPU usage
	}

	// Get prediction for current CPU usage
	prediction := monitor.getPrediction("cpu", 60.0, 90.0)
	if prediction != nil {
		fmt.Printf("CPU Trend: %s\n", prediction.TrendDirection)
		fmt.Printf("Confidence: %.2f\n", prediction.Confidence)
		fmt.Printf("Time to critical threshold: %v\n", prediction.TimeToThreshold)
		fmt.Printf("Recommended action: %s\n", prediction.RecommendedAction)
	}

	// Output:
	// CPU Trend: increasing
	// Confidence: 0.25
	// Time to critical threshold: 5m0s
	// Recommended action: Immediate action required - critical threshold will be reached soon
}

// Example_resourceMonitorCustomThresholds demonstrates custom threshold configuration
func Example_resourceMonitorCustomThresholds() {
	collector := NewResourceCollector()

	// Custom thresholds for a high-performance environment
	customThresholds := ResourceMonitorThresholds{
		CPUWarning:        80.0, // Higher CPU tolerance
		CPUCritical:       95.0,
		MemoryWarning:     0.9, // Higher memory tolerance
		MemoryCritical:    0.98,
		DiskWarning:       0.9, // Higher disk tolerance
		DiskCritical:      0.98,
		GoroutineWarning:  50000, // More goroutines allowed
		GoroutineCritical: 100000,
	}

	monitor := NewResourceMonitor(collector, customThresholds)

	// Update thresholds at runtime
	newThresholds := customThresholds
	newThresholds.CPUWarning = 85.0
	monitor.UpdateThresholds(newThresholds)

	currentThresholds := monitor.GetThresholds()
	fmt.Printf("CPU Warning Threshold: %.1f%%\n", currentThresholds.CPUWarning)
	fmt.Printf("Memory Critical Threshold: %.1f%%\n", currentThresholds.MemoryCritical*100)

	// Output:
	// CPU Warning Threshold: 85.0%
	// Memory Critical Threshold: 98.0%
}

// Example_resourceMonitorHistoricalData demonstrates working with historical data
func Example_resourceMonitorHistoricalData() {
	collector := NewResourceCollector()
	thresholds := DefaultResourceMonitorThresholds()
	monitor := NewResourceMonitor(collector, thresholds)

	// Simulate some data collection
	now := time.Now()
	for i := 0; i < 10; i++ {
		monitor.cpuHistory.Add(now.Add(time.Duration(i)*time.Minute), float64(50+i*2))
		monitor.memoryHistory.Add(now.Add(time.Duration(i)*time.Minute), float64(0.6+float64(i)*0.02))
	}

	// Get historical data
	cpuHistory := monitor.GetResourceHistory("cpu")
	if cpuHistory != nil && len(cpuHistory.Values) > 0 {
		fmt.Printf("CPU History Points: %d\n", len(cpuHistory.Values))
		fmt.Printf("Latest CPU Value: %.1f\n", cpuHistory.Values[len(cpuHistory.Values)-1])

		direction, slope := cpuHistory.GetTrend()
		fmt.Printf("CPU Trend: %s (slope: %.2f)\n", direction, slope)
	}

	memoryHistory := monitor.GetResourceHistory("memory")
	if memoryHistory != nil && len(memoryHistory.Values) > 0 {
		fmt.Printf("Memory History Points: %d\n", len(memoryHistory.Values))
		direction, slope := memoryHistory.GetTrend()
		fmt.Printf("Memory Trend: %s (slope: %.4f)\n", direction, slope)
	}

	// Output:
	// CPU History Points: 10
	// Latest CPU Value: 68.0
	// CPU Trend: increasing (slope: 2.00)
	// Memory History Points: 10
	// Memory Trend: increasing (slope: 0.0200)
}
