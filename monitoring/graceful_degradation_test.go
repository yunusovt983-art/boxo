package monitoring

import (
	"context"
	"log/slog"
	"testing"
	"time"
)

// Test degradation manager creation
func TestNewGracefulDegradationManager(t *testing.T) {
	mockPM := newMockPerformanceMonitor()
	mockRM := NewResourceMonitor(NewResourceCollector(), DefaultResourceMonitorThresholds())

	gdm := NewGracefulDegradationManager(mockPM, mockRM, slog.Default())

	if gdm == nil {
		t.Fatal("Expected non-nil graceful degradation manager")
	}

	if gdm.GetCurrentDegradationLevel() != DegradationNone {
		t.Errorf("Expected initial degradation level to be None, got %v", gdm.GetCurrentDegradationLevel())
	}
}

// Test degradation level progression
func TestDegradationLevelProgression(t *testing.T) {
	mockPM := newMockPerformanceMonitor()
	mockRM := NewResourceMonitor(NewResourceCollector(), DefaultResourceMonitorThresholds())

	gdm := NewGracefulDegradationManager(mockPM, mockRM, slog.Default())

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Start the degradation manager
	err := gdm.Start(ctx)
	if err != nil {
		t.Fatalf("Failed to start degradation manager: %v", err)
	}
	defer gdm.Stop()

	// Test light degradation - high CPU usage
	mockPM.setMetrics(&PerformanceMetrics{
		Timestamp: time.Now(),
		ResourceMetrics: ResourceStats{
			CPUUsage:       80.0, // Above light threshold (75%)
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

	// Wait for evaluation
	time.Sleep(100 * time.Millisecond)

	// Force evaluation
	gdm.(*gracefulDegradationManager).evaluateDegradation(ctx)

	if gdm.GetCurrentDegradationLevel() != DegradationLight {
		t.Errorf("Expected degradation level Light, got %v", gdm.GetCurrentDegradationLevel())
	}

	// Test severe degradation - very high CPU usage
	mockPM.setMetrics(&PerformanceMetrics{
		Timestamp: time.Now(),
		ResourceMetrics: ResourceStats{
			CPUUsage:       95.0, // Above severe threshold (90%)
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

	if gdm.GetCurrentDegradationLevel() != DegradationSevere {
		t.Errorf("Expected degradation level Severe, got %v", gdm.GetCurrentDegradationLevel())
	}
}

// Test memory pressure degradation
func TestMemoryPressureDegradation(t *testing.T) {
	mockPM := newMockPerformanceMonitor()
	mockRM := NewResourceMonitor(NewResourceCollector(), DefaultResourceMonitorThresholds())

	gdm := NewGracefulDegradationManager(mockPM, mockRM, slog.Default())

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	err := gdm.Start(ctx)
	if err != nil {
		t.Fatalf("Failed to start degradation manager: %v", err)
	}
	defer gdm.Stop()

	// Test critical memory pressure
	mockPM.setMetrics(&PerformanceMetrics{
		Timestamp: time.Now(),
		ResourceMetrics: ResourceStats{
			CPUUsage:       50.0,
			MemoryUsage:    7800 * 1024 * 1024, // ~97.5% of 8GB
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

	if gdm.GetCurrentDegradationLevel() != DegradationCritical {
		t.Errorf("Expected degradation level Critical, got %v", gdm.GetCurrentDegradationLevel())
	}

	status := gdm.GetDegradationStatus()
	if len(status.ActiveActions) == 0 {
		t.Error("Expected active actions during critical degradation")
	}
}

// Test network congestion degradation
func TestNetworkCongestionDegradation(t *testing.T) {
	mockPM := newMockPerformanceMonitor()
	mockRM := NewResourceMonitor(NewResourceCollector(), DefaultResourceMonitorThresholds())

	gdm := NewGracefulDegradationManager(mockPM, mockRM, slog.Default())

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	err := gdm.Start(ctx)
	if err != nil {
		t.Fatalf("Failed to start degradation manager: %v", err)
	}
	defer gdm.Stop()

	// Test network congestion
	mockPM.setMetrics(&PerformanceMetrics{
		Timestamp: time.Now(),
		ResourceMetrics: ResourceStats{
			CPUUsage:       50.0,
			MemoryUsage:    4 * 1024 * 1024 * 1024,
			MemoryTotal:    8 * 1024 * 1024 * 1024,
			GoroutineCount: 1000,
		},
		BitswapMetrics: BitswapStats{
			P95ResponseTime: 50 * time.Millisecond,
		},
		NetworkMetrics: NetworkStats{
			PacketLossRate: 0.08, // 8% packet loss - above threshold (5%)
		},
	})

	// Force evaluation
	gdm.(*gracefulDegradationManager).evaluateDegradation(ctx)

	if gdm.GetCurrentDegradationLevel() != DegradationModerate {
		t.Errorf("Expected degradation level Moderate, got %v", gdm.GetCurrentDegradationLevel())
	}
}

// Test Bitswap latency degradation
func TestBitswapLatencyDegradation(t *testing.T) {
	mockPM := newMockPerformanceMonitor()
	mockRM := NewResourceMonitor(NewResourceCollector(), DefaultResourceMonitorThresholds())

	gdm := NewGracefulDegradationManager(mockPM, mockRM, slog.Default())

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	err := gdm.Start(ctx)
	if err != nil {
		t.Fatalf("Failed to start degradation manager: %v", err)
	}
	defer gdm.Stop()

	// Test high Bitswap latency
	mockPM.setMetrics(&PerformanceMetrics{
		Timestamp: time.Now(),
		ResourceMetrics: ResourceStats{
			CPUUsage:       50.0,
			MemoryUsage:    4 * 1024 * 1024 * 1024,
			MemoryTotal:    8 * 1024 * 1024 * 1024,
			GoroutineCount: 1000,
		},
		BitswapMetrics: BitswapStats{
			P95ResponseTime: 250 * time.Millisecond, // Above threshold (200ms)
		},
		NetworkMetrics: NetworkStats{
			PacketLossRate: 0.001,
		},
	})

	// Force evaluation
	gdm.(*gracefulDegradationManager).evaluateDegradation(ctx)

	if gdm.GetCurrentDegradationLevel() != DegradationLight {
		t.Errorf("Expected degradation level Light, got %v", gdm.GetCurrentDegradationLevel())
	}
}

// Test recovery mechanism
func TestRecoveryMechanism(t *testing.T) {
	mockPM := newMockPerformanceMonitor()
	mockRM := NewResourceMonitor(NewResourceCollector(), DefaultResourceMonitorThresholds())

	gdm := NewGracefulDegradationManager(mockPM, mockRM, slog.Default())

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	err := gdm.Start(ctx)
	if err != nil {
		t.Fatalf("Failed to start degradation manager: %v", err)
	}
	defer gdm.Stop()

	// First, trigger degradation
	mockPM.setMetrics(&PerformanceMetrics{
		Timestamp: time.Now(),
		ResourceMetrics: ResourceStats{
			CPUUsage:       80.0, // High CPU
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

	gdm.(*gracefulDegradationManager).evaluateDegradation(ctx)

	if gdm.GetCurrentDegradationLevel() == DegradationNone {
		t.Error("Expected degradation to be triggered")
	}

	// Now improve metrics to trigger recovery
	mockPM.setMetrics(&PerformanceMetrics{
		Timestamp: time.Now(),
		ResourceMetrics: ResourceStats{
			CPUUsage:       60.0, // Improved CPU usage
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

	// Simulate time passing for stable metrics
	gdmImpl := gdm.(*gracefulDegradationManager)
	gdmImpl.mu.Lock()
	gdmImpl.lastLevelChange = time.Now().Add(-60 * time.Second) // Simulate 60 seconds ago
	gdmImpl.mu.Unlock()

	gdm.(*gracefulDegradationManager).evaluateDegradation(ctx)

	// Recovery should have occurred
	if gdm.GetCurrentDegradationLevel() == DegradationLight {
		t.Error("Expected recovery from degradation")
	}
}

// Test forced recovery
func TestForcedRecovery(t *testing.T) {
	mockPM := newMockPerformanceMonitor()
	mockRM := NewResourceMonitor(NewResourceCollector(), DefaultResourceMonitorThresholds())

	gdm := NewGracefulDegradationManager(mockPM, mockRM, slog.Default())

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	err := gdm.Start(ctx)
	if err != nil {
		t.Fatalf("Failed to start degradation manager: %v", err)
	}
	defer gdm.Stop()

	// Trigger degradation
	mockPM.setMetrics(&PerformanceMetrics{
		Timestamp: time.Now(),
		ResourceMetrics: ResourceStats{
			CPUUsage:       95.0, // Very high CPU
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

	gdm.(*gracefulDegradationManager).evaluateDegradation(ctx)

	if gdm.GetCurrentDegradationLevel() == DegradationNone {
		t.Error("Expected degradation to be triggered")
	}

	// Force recovery
	err = gdm.ForceRecovery()
	if err != nil {
		t.Fatalf("Failed to force recovery: %v", err)
	}

	if gdm.GetCurrentDegradationLevel() != DegradationNone {
		t.Errorf("Expected degradation level None after forced recovery, got %v", gdm.GetCurrentDegradationLevel())
	}
}

// Test custom degradation rules
func TestCustomDegradationRules(t *testing.T) {
	mockPM := newMockPerformanceMonitor()
	mockRM := NewResourceMonitor(NewResourceCollector(), DefaultResourceMonitorThresholds())

	gdm := NewGracefulDegradationManager(mockPM, mockRM, slog.Default())

	// Set custom rules
	customRules := []DegradationRule{
		{
			ID:          "custom-cpu-rule",
			Name:        "Custom CPU Rule",
			Description: "Custom CPU degradation rule",
			Component:   "resource",
			Metric:      "cpu_usage",
			Condition:   ConditionGreaterThan,
			Threshold:   60.0, // Lower threshold than default
			Level:       DegradationLight,
			Actions:     []string{"reduce_worker_threads"},
			Duration:    10 * time.Second,
			Priority:    500,
			Enabled:     true,
			CreatedAt:   time.Now(),
			UpdatedAt:   time.Now(),
		},
	}

	err := gdm.SetDegradationRules(customRules)
	if err != nil {
		t.Fatalf("Failed to set custom degradation rules: %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	err = gdm.Start(ctx)
	if err != nil {
		t.Fatalf("Failed to start degradation manager: %v", err)
	}
	defer gdm.Stop()

	// Test custom rule triggering
	mockPM.setMetrics(&PerformanceMetrics{
		Timestamp: time.Now(),
		ResourceMetrics: ResourceStats{
			CPUUsage:       65.0, // Above custom threshold (60%)
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

	gdm.(*gracefulDegradationManager).evaluateDegradation(ctx)

	if gdm.GetCurrentDegradationLevel() != DegradationLight {
		t.Errorf("Expected degradation level Light with custom rule, got %v", gdm.GetCurrentDegradationLevel())
	}
}

// Test degradation status reporting
func TestDegradationStatusReporting(t *testing.T) {
	mockPM := newMockPerformanceMonitor()
	mockRM := NewResourceMonitor(NewResourceCollector(), DefaultResourceMonitorThresholds())

	gdm := NewGracefulDegradationManager(mockPM, mockRM, slog.Default())

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	err := gdm.Start(ctx)
	if err != nil {
		t.Fatalf("Failed to start degradation manager: %v", err)
	}
	defer gdm.Stop()

	// Trigger degradation
	mockPM.setMetrics(&PerformanceMetrics{
		Timestamp: time.Now(),
		ResourceMetrics: ResourceStats{
			CPUUsage:       80.0,
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

	gdm.(*gracefulDegradationManager).evaluateDegradation(ctx)

	status := gdm.GetDegradationStatus()

	if status.Level == DegradationNone {
		t.Error("Expected non-none degradation level in status")
	}

	if len(status.ActiveRules) == 0 {
		t.Error("Expected active rules in status")
	}

	if len(status.ActiveActions) == 0 {
		t.Error("Expected active actions in status")
	}

	if status.Metrics == nil {
		t.Error("Expected metrics in status")
	}

	if status.DegradationStarted == nil {
		t.Error("Expected degradation start time in status")
	}
}

// Test recovery callback
func TestRecoveryCallback(t *testing.T) {
	mockPM := newMockPerformanceMonitor()
	mockRM := NewResourceMonitor(NewResourceCollector(), DefaultResourceMonitorThresholds())

	gdm := NewGracefulDegradationManager(mockPM, mockRM, slog.Default())

	callbackCalled := false
	var recoveryEvent RecoveryEvent

	err := gdm.SetRecoveryCallback(func(event RecoveryEvent) {
		callbackCalled = true
		recoveryEvent = event
	})
	if err != nil {
		t.Fatalf("Failed to set recovery callback: %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	err = gdm.Start(ctx)
	if err != nil {
		t.Fatalf("Failed to start degradation manager: %v", err)
	}
	defer gdm.Stop()

	// Trigger degradation
	mockPM.setMetrics(&PerformanceMetrics{
		Timestamp: time.Now(),
		ResourceMetrics: ResourceStats{
			CPUUsage:       80.0,
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

	gdm.(*gracefulDegradationManager).evaluateDegradation(ctx)

	// Force recovery
	err = gdm.ForceRecovery()
	if err != nil {
		t.Fatalf("Failed to force recovery: %v", err)
	}

	// Wait for callback
	time.Sleep(100 * time.Millisecond)

	if !callbackCalled {
		t.Error("Expected recovery callback to be called")
	}

	if recoveryEvent.Type != RecoveryEventForced {
		t.Errorf("Expected recovery event type Forced, got %v", recoveryEvent.Type)
	}

	if recoveryEvent.ToLevel != DegradationNone {
		t.Errorf("Expected recovery to level None, got %v", recoveryEvent.ToLevel)
	}
}

// Test multiple simultaneous degradation conditions
func TestMultipleDegradationConditions(t *testing.T) {
	mockPM := newMockPerformanceMonitor()
	mockRM := NewResourceMonitor(NewResourceCollector(), DefaultResourceMonitorThresholds())

	gdm := NewGracefulDegradationManager(mockPM, mockRM, slog.Default())

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	err := gdm.Start(ctx)
	if err != nil {
		t.Fatalf("Failed to start degradation manager: %v", err)
	}
	defer gdm.Stop()

	// Trigger multiple degradation conditions simultaneously
	mockPM.setMetrics(&PerformanceMetrics{
		Timestamp: time.Now(),
		ResourceMetrics: ResourceStats{
			CPUUsage:       95.0,               // Severe CPU degradation
			MemoryUsage:    7800 * 1024 * 1024, // Critical memory degradation (~97.5%)
			MemoryTotal:    8 * 1024 * 1024 * 1024,
			GoroutineCount: 1000,
		},
		BitswapMetrics: BitswapStats{
			P95ResponseTime: 250 * time.Millisecond, // Light Bitswap degradation
		},
		NetworkMetrics: NetworkStats{
			PacketLossRate: 0.08, // Moderate network degradation
		},
	})

	gdm.(*gracefulDegradationManager).evaluateDegradation(ctx)

	// Should choose the highest degradation level (Critical from memory)
	if gdm.GetCurrentDegradationLevel() != DegradationCritical {
		t.Errorf("Expected degradation level Critical with multiple conditions, got %v", gdm.GetCurrentDegradationLevel())
	}

	status := gdm.GetDegradationStatus()

	// Should have multiple active rules
	if len(status.ActiveRules) < 2 {
		t.Errorf("Expected multiple active rules, got %d", len(status.ActiveRules))
	}

	// Should have multiple active actions
	if len(status.ActiveActions) < 2 {
		t.Errorf("Expected multiple active actions, got %d", len(status.ActiveActions))
	}
}

// Test degradation action execution and recovery
func TestDegradationActionExecution(t *testing.T) {
	// Test individual degradation actions
	actions := []DegradationAction{
		&ReduceWorkerThreadsAction{},
		&IncreaseBatchTimeoutAction{},
		&ClearCachesAction{},
		&ReduceBufferSizesAction{},
		&LimitConcurrentOperationsAction{},
		&DisableNonCriticalFeaturesAction{},
		&ReduceConnectionLimitsAction{},
		&EmergencyCacheClearAction{},
		&DisableNonEssentialServicesAction{},
		&IncreaseBatchSizeAction{},
		&PrioritizeCriticalRequestsAction{},
		&EnableCompressionAction{},
		&PrioritizeCriticalTrafficAction{},
	}

	ctx := context.Background()
	metrics := &PerformanceMetrics{
		Timestamp: time.Now(),
		ResourceMetrics: ResourceStats{
			CPUUsage:       90.0,
			MemoryUsage:    7 * 1024 * 1024 * 1024,
			MemoryTotal:    8 * 1024 * 1024 * 1024,
			GoroutineCount: 5000,
		},
	}

	for _, action := range actions {
		t.Run(action.GetName(), func(t *testing.T) {
			// Test execution
			err := action.Execute(ctx, DegradationSevere, metrics)
			if err != nil {
				t.Errorf("Failed to execute action %s: %v", action.GetName(), err)
			}

			if !action.IsActive() {
				t.Errorf("Action %s should be active after execution", action.GetName())
			}

			// Test recovery
			err = action.Recover(ctx)
			if err != nil {
				t.Errorf("Failed to recover action %s: %v", action.GetName(), err)
			}

			if action.IsActive() {
				t.Errorf("Action %s should not be active after recovery", action.GetName())
			}
		})
	}
}

// Test degradation level string representation
func TestDegradationLevelString(t *testing.T) {
	tests := []struct {
		level    DegradationLevel
		expected string
	}{
		{DegradationNone, "none"},
		{DegradationLight, "light"},
		{DegradationModerate, "moderate"},
		{DegradationSevere, "severe"},
		{DegradationCritical, "critical"},
		{DegradationLevel(999), "unknown"},
	}

	for _, test := range tests {
		if test.level.String() != test.expected {
			t.Errorf("Expected %s, got %s", test.expected, test.level.String())
		}
	}
}

// Benchmark degradation evaluation
func BenchmarkDegradationEvaluation(b *testing.B) {
	mockPM := newMockPerformanceMonitor()
	mockRM := NewResourceMonitor(NewResourceCollector(), DefaultResourceMonitorThresholds())

	gdm := NewGracefulDegradationManager(mockPM, mockRM, slog.Default())

	ctx := context.Background()

	mockPM.setMetrics(&PerformanceMetrics{
		Timestamp: time.Now(),
		ResourceMetrics: ResourceStats{
			CPUUsage:       80.0,
			MemoryUsage:    6 * 1024 * 1024 * 1024,
			MemoryTotal:    8 * 1024 * 1024 * 1024,
			GoroutineCount: 2000,
		},
		BitswapMetrics: BitswapStats{
			P95ResponseTime: 150 * time.Millisecond,
		},
		NetworkMetrics: NetworkStats{
			PacketLossRate: 0.03,
		},
	})

	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		gdm.(*gracefulDegradationManager).evaluateDegradation(ctx)
	}
}
