package monitoring

import (
	"context"
	"fmt"
	"log/slog"
	"time"
)

// ExampleGracefulDegradationManager demonstrates how to use the graceful degradation system
func ExampleGracefulDegradationManager() {
	// Create performance monitor and resource monitor
	performanceMonitor := newMockPerformanceMonitor()
	resourceMonitor := NewResourceMonitor(NewResourceCollector(), DefaultResourceMonitorThresholds())

	// Create graceful degradation manager
	gdm := NewGracefulDegradationManager(performanceMonitor, resourceMonitor, slog.Default())

	// Set up recovery callback
	gdm.SetRecoveryCallback(func(event RecoveryEvent) {
		fmt.Printf("Recovery event: %s from %s to %s\n",
			event.Type, event.FromLevel.String(), event.ToLevel.String())
	})

	// Start the degradation manager
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	err := gdm.Start(ctx)
	if err != nil {
		fmt.Printf("Failed to start degradation manager: %v\n", err)
		return
	}
	defer gdm.Stop()

	// Simulate normal operation
	fmt.Printf("Initial degradation level: %s\n", gdm.GetCurrentDegradationLevel().String())

	// Simulate high CPU usage to trigger degradation
	performanceMonitor.setMetrics(&PerformanceMetrics{
		Timestamp: time.Now(),
		ResourceMetrics: ResourceStats{
			CPUUsage:       85.0, // High CPU usage
			MemoryUsage:    4 * 1024 * 1024 * 1024,
			MemoryTotal:    8 * 1024 * 1024 * 1024,
			GoroutineCount: 1000,
		},
		BitswapMetrics: BitswapStats{
			P95ResponseTime: 50 * time.Millisecond,
		},
		NetworkMetrics: NetworkStats{
			PacketLossRate: 0.001,
		},
	})

	// Force evaluation
	gdm.(*gracefulDegradationManager).evaluateDegradation(ctx)

	fmt.Printf("Degradation level after high CPU: %s\n", gdm.GetCurrentDegradationLevel().String())

	// Get degradation status
	status := gdm.GetDegradationStatus()
	fmt.Printf("Active actions: %d\n", len(status.ActiveActions))
	fmt.Printf("Active rules: %d\n", len(status.ActiveRules))

	// Simulate recovery by improving metrics
	performanceMonitor.setMetrics(&PerformanceMetrics{
		Timestamp: time.Now(),
		ResourceMetrics: ResourceStats{
			CPUUsage:       50.0, // Normal CPU usage
			MemoryUsage:    4 * 1024 * 1024 * 1024,
			MemoryTotal:    8 * 1024 * 1024 * 1024,
			GoroutineCount: 1000,
		},
		BitswapMetrics: BitswapStats{
			P95ResponseTime: 50 * time.Millisecond,
		},
		NetworkMetrics: NetworkStats{
			PacketLossRate: 0.001,
		},
	})

	// Force recovery
	err = gdm.ForceRecovery()
	if err != nil {
		fmt.Printf("Failed to force recovery: %v\n", err)
		return
	}

	fmt.Printf("Final degradation level: %s\n", gdm.GetCurrentDegradationLevel().String())

	// Output:
	// Initial degradation level: none
	// Degradation level after high CPU: light
	// Active actions: 2
	// Active rules: 1
	// Final degradation level: none
}

// ExampleDegradationAction demonstrates how to create a custom degradation action
func ExampleDegradationAction() {
	// Create a custom degradation action
	action := &ReduceWorkerThreadsAction{}

	ctx := context.Background()
	metrics := &PerformanceMetrics{
		ResourceMetrics: ResourceStats{
			CPUUsage: 90.0,
		},
	}

	// Execute the action
	err := action.Execute(ctx, DegradationSevere, metrics)
	if err != nil {
		fmt.Printf("Failed to execute action: %v\n", err)
		return
	}

	fmt.Printf("Action active: %t\n", action.IsActive())
	fmt.Printf("Action name: %s\n", action.GetName())

	// Recover from the action
	err = action.Recover(ctx)
	if err != nil {
		fmt.Printf("Failed to recover action: %v\n", err)
		return
	}

	fmt.Printf("Action active after recovery: %t\n", action.IsActive())

	// Output:
	// Action active: true
	// Action name: reduce_worker_threads
	// Action active after recovery: false
}

// ExampleGracefulDegradationManager_customRules demonstrates how to set custom degradation rules
func ExampleGracefulDegradationManager_customRules() {
	performanceMonitor := newMockPerformanceMonitor()
	resourceMonitor := NewResourceMonitor(NewResourceCollector(), DefaultResourceMonitorThresholds())

	gdm := NewGracefulDegradationManager(performanceMonitor, resourceMonitor, slog.Default())

	// Define custom degradation rules
	customRules := []DegradationRule{
		{
			ID:          "custom-memory-rule",
			Name:        "Custom Memory Rule",
			Description: "Trigger degradation at 70% memory usage",
			Component:   "resource",
			Metric:      "memory_usage_percent",
			Condition:   ConditionGreaterThan,
			Threshold:   70.0,
			Level:       DegradationModerate,
			Actions:     []string{"clear_caches", "reduce_buffer_sizes"},
			Duration:    30 * time.Second,
			Priority:    200,
			Enabled:     true,
			CreatedAt:   time.Now(),
			UpdatedAt:   time.Now(),
		},
		{
			ID:          "custom-latency-rule",
			Name:        "Custom Latency Rule",
			Description: "Trigger degradation at 100ms P95 latency",
			Component:   "bitswap",
			Metric:      "p95_response_time",
			Condition:   ConditionGreaterThan,
			Threshold:   100 * time.Millisecond,
			Level:       DegradationLight,
			Actions:     []string{"increase_batch_size"},
			Duration:    15 * time.Second,
			Priority:    100,
			Enabled:     true,
			CreatedAt:   time.Now(),
			UpdatedAt:   time.Now(),
		},
	}

	// Set the custom rules
	err := gdm.SetDegradationRules(customRules)
	if err != nil {
		fmt.Printf("Failed to set custom rules: %v\n", err)
		return
	}

	fmt.Printf("Custom rules set successfully\n")
	fmt.Printf("Number of rules: %d\n", len(customRules))

	// Output:
	// Custom rules set successfully
	// Number of rules: 2
}
