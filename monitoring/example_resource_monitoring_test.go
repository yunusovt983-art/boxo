package monitoring

import (
	"context"
	"fmt"
	"time"
)

// Example_resourceMonitoringComplete demonstrates a complete resource monitoring setup
func Example_resourceMonitoringComplete() {
	// Create a resource collector
	collector := NewResourceCollector()

	// Set up custom thresholds for a production environment
	thresholds := ResourceMonitorThresholds{
		CPUWarning:        70.0, // 70% CPU usage warning
		CPUCritical:       90.0, // 90% CPU usage critical
		MemoryWarning:     0.8,  // 80% memory usage warning
		MemoryCritical:    0.95, // 95% memory usage critical
		DiskWarning:       0.85, // 85% disk usage warning
		DiskCritical:      0.95, // 95% disk usage critical
		GoroutineWarning:  10000,
		GoroutineCritical: 50000,
	}

	// Create resource monitor
	monitor := NewResourceMonitor(collector, thresholds)

	// Set up alert handling
	monitor.SetAlertCallback(func(alert ResourceAlert) {
		fmt.Printf("ALERT [%s]: %s\n", alert.Level, alert.Message)
		if alert.Prediction != nil {
			fmt.Printf("  Trend: %s\n", alert.Prediction.TrendDirection)
			fmt.Printf("  Recommendation: %s\n", alert.Prediction.RecommendedAction)
		}
	})

	// Set monitoring interval
	monitor.SetMonitorInterval(5 * time.Second)

	// Start monitoring
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	err := monitor.Start(ctx)
	if err != nil {
		fmt.Printf("Failed to start monitor: %v\n", err)
		return
	}
	defer monitor.Stop()

	// Simulate resource usage changes
	collector.UpdateCPUUsage(45.0)                                   // Normal usage
	collector.UpdateDiskUsage(60*1024*1024*1024, 100*1024*1024*1024) // 60% disk usage

	// Force collection to see immediate results
	monitor.ForceCollection()

	// Get current status
	status := monitor.GetCurrentStatus()
	fmt.Printf("CPU Usage: %.1f%%\n", status["cpu_usage_percent"])
	fmt.Printf("Memory Pressure: %.1f%%\n", status["memory_pressure"].(float64)*100)
	fmt.Printf("Disk Pressure: %.1f%%\n", status["disk_pressure"].(float64)*100)

	// Get health status
	health := monitor.GetHealthStatus()
	fmt.Printf("System Health: %s\n", health["health"])

	// Simulate high resource usage
	collector.UpdateCPUUsage(85.0) // High CPU usage
	monitor.ForceCollection()

	// Wait a bit for processing
	time.Sleep(50 * time.Millisecond)

	// Example output:
	// CPU Usage: 45.0%
	// Memory Pressure: 7.7%
	// Disk Pressure: 60.0%
	// System Health: healthy
	// ALERT [warning]: cpu usage is high: 85.00% (threshold: 70.00%)
	//   Trend: increasing
	//   Recommendation: Immediate action required - critical threshold will be reached soon
}

// Example_resourceMonitoringPredictiveAnalysis demonstrates predictive analysis features
func Example_resourceMonitoringPredictiveAnalysis() {
	collector := NewResourceCollector()
	thresholds := DefaultResourceMonitorThresholds()
	monitor := NewResourceMonitor(collector, thresholds)

	// Add historical data to demonstrate trend analysis
	now := time.Now()
	cpuHistory := monitor.GetResourceHistory("cpu")

	// Simulate increasing CPU usage trend
	for i := 0; i < 10; i++ {
		cpuHistory.Add(now.Add(time.Duration(i)*time.Minute), float64(50+i*3))
	}

	// Get trend analysis
	direction, slope := cpuHistory.GetTrend()
	fmt.Printf("CPU Trend: %s (slope: %.2f)\n", direction, slope)

	// Get prediction for current usage
	prediction := monitor.getPrediction("cpu", 75.0, 90.0)
	if prediction != nil {
		fmt.Printf("Prediction:\n")
		fmt.Printf("  Direction: %s\n", prediction.TrendDirection)
		fmt.Printf("  Confidence: %.2f\n", prediction.Confidence)
		fmt.Printf("  Time to threshold: %v\n", prediction.TimeToThreshold)
		fmt.Printf("  Recommendation: %s\n", prediction.RecommendedAction)
	}

	// Example output:
	// CPU Trend: increasing (slope: 3.00)
	// Prediction:
	//   Direction: increasing
	//   Confidence: 0.57
	//   Time to threshold: 2m30s
	//   Recommendation: Immediate action required - critical threshold will be reached soon
}

// Example_resourceMonitoringCustomization demonstrates advanced customization
func Example_resourceMonitoringCustomization() {
	collector := NewResourceCollector()

	// Custom thresholds for different environments
	productionThresholds := ResourceMonitorThresholds{
		CPUWarning:        60.0,
		CPUCritical:       85.0,
		MemoryWarning:     0.75,
		MemoryCritical:    0.9,
		DiskWarning:       0.8,
		DiskCritical:      0.95,
		GoroutineWarning:  8000,
		GoroutineCritical: 20000,
	}

	monitor := NewResourceMonitor(collector, productionThresholds)

	// Custom alert handling with different actions based on severity
	monitor.SetAlertCallback(func(alert ResourceAlert) {
		switch alert.Level {
		case "warning":
			fmt.Printf("⚠️  WARNING: %s\n", alert.Message)
			// In real implementation: send notification, log to monitoring system
		case "critical":
			fmt.Printf("🚨 CRITICAL: %s\n", alert.Message)
			// In real implementation: page on-call engineer, trigger auto-scaling
		}

		if alert.Prediction != nil && alert.Prediction.TimeToThreshold < time.Hour {
			fmt.Printf("   ⏰ Urgent: Critical threshold in %v\n", alert.Prediction.TimeToThreshold)
		}
	})

	// Demonstrate threshold updates at runtime
	fmt.Println("Initial thresholds:")
	fmt.Printf("  CPU Warning: %.1f%%\n", monitor.GetThresholds().CPUWarning)

	// Update thresholds for different load conditions
	nightTimeThresholds := productionThresholds
	nightTimeThresholds.CPUWarning = 40.0 // Lower threshold during low-traffic hours

	monitor.UpdateThresholds(nightTimeThresholds)

	fmt.Println("Updated thresholds:")
	fmt.Printf("  CPU Warning: %.1f%%\n", monitor.GetThresholds().CPUWarning)

	// Output:
	// Initial thresholds:
	//   CPU Warning: 60.0%
	// Updated thresholds:
	//   CPU Warning: 40.0%
}
