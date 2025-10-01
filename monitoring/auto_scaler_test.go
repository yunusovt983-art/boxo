package monitoring

import (
	"context"
	"fmt"
	"log/slog"
	"testing"
	"time"

	"github.com/prometheus/client_golang/prometheus"
)

// MockPerformanceMonitor for testing
type MockPerformanceMonitor struct {
	metrics *PerformanceMetrics
}

func (m *MockPerformanceMonitor) CollectMetrics() *PerformanceMetrics {
	return m.metrics
}

func (m *MockPerformanceMonitor) StartCollection(ctx context.Context, interval time.Duration) error {
	return nil
}

func (m *MockPerformanceMonitor) StopCollection() error {
	return nil
}

func (m *MockPerformanceMonitor) RegisterCollector(name string, collector MetricCollector) error {
	return nil
}

func (m *MockPerformanceMonitor) GetPrometheusRegistry() *prometheus.Registry {
	return prometheus.NewRegistry()
}

func (m *MockPerformanceMonitor) SetMetrics(metrics *PerformanceMetrics) {
	m.metrics = metrics
}

// MockResourceMonitor for testing
type MockResourceMonitor struct {
	thresholds ResourceMonitorThresholds
}

func (m *MockResourceMonitor) GetThresholds() ResourceMonitorThresholds {
	return m.thresholds
}

func (m *MockResourceMonitor) GetCurrentStatus() map[string]interface{} {
	return map[string]interface{}{
		"cpu_usage_percent": 50.0,
		"memory_pressure":   0.6,
		"disk_pressure":     0.4,
		"goroutines":        1000,
	}
}

// MockScalableComponent for testing
type MockScalableComponent struct {
	name         string
	currentScale int
	minScale     int
	maxScale     int
	isHealthy    bool
	scaleError   error
	metrics      map[string]interface{}
}

func (m *MockScalableComponent) GetCurrentScale() int {
	return m.currentScale
}

func (m *MockScalableComponent) SetScale(ctx context.Context, scale int) error {
	if m.scaleError != nil {
		return m.scaleError
	}
	m.currentScale = scale
	return nil
}

func (m *MockScalableComponent) GetScaleRange() (min, max int) {
	return m.minScale, m.maxScale
}

func (m *MockScalableComponent) GetComponentName() string {
	return m.name
}

func (m *MockScalableComponent) IsHealthy() bool {
	return m.isHealthy
}

func (m *MockScalableComponent) GetMetrics() map[string]interface{} {
	if m.metrics == nil {
		return map[string]interface{}{
			"current_scale": m.currentScale,
			"utilization":   0.5,
		}
	}
	return m.metrics
}

func createTestAutoScaler() (*autoScaler, *MockPerformanceMonitor, *MockResourceMonitor) {
	mockPerfMonitor := &MockPerformanceMonitor{
		metrics: &PerformanceMetrics{
			ResourceMetrics: ResourceStats{
				CPUUsage:       50.0,
				MemoryUsage:    1024 * 1024 * 1024, // 1GB
				MemoryTotal:    2048 * 1024 * 1024, // 2GB
				DiskUsage:      500 * 1024 * 1024,  // 500MB
				GoroutineCount: 1000,
			},
			BitswapMetrics: BitswapStats{
				RequestsPerSecond:   100.0,
				AverageResponseTime: 50 * time.Millisecond,
				P95ResponseTime:     100 * time.Millisecond,
				QueuedRequests:      500,
				ActiveConnections:   200,
			},
			NetworkMetrics: NetworkStats{
				ActiveConnections: 200,
				AverageLatency:    20 * time.Millisecond,
				PacketLossRate:    0.01,
			},
		},
	}

	mockResourceMonitor := &MockResourceMonitor{
		thresholds: DefaultResourceMonitorThresholds(),
	}

	logger := slog.New(slog.NewTextHandler(nil, &slog.HandlerOptions{Level: slog.LevelError}))

	as := NewAutoScaler(mockPerfMonitor, mockResourceMonitor, logger).(*autoScaler)
	return as, mockPerfMonitor, mockResourceMonitor
}

func TestAutoScaler_Start_Stop(t *testing.T) {
	as, _, _ := createTestAutoScaler()

	ctx := context.Background()

	// Test start
	err := as.Start(ctx)
	if err != nil {
		t.Fatalf("Failed to start auto-scaler: %v", err)
	}

	if !as.isRunning {
		t.Error("Auto-scaler should be running after start")
	}

	// Test double start
	err = as.Start(ctx)
	if err == nil {
		t.Error("Expected error when starting already running auto-scaler")
	}

	// Test stop
	err = as.Stop()
	if err != nil {
		t.Fatalf("Failed to stop auto-scaler: %v", err)
	}

	if as.isRunning {
		t.Error("Auto-scaler should not be running after stop")
	}

	// Test double stop
	err = as.Stop()
	if err != nil {
		t.Error("Stop should not return error when already stopped")
	}
}

func TestAutoScaler_RegisterComponent(t *testing.T) {
	as, _, _ := createTestAutoScaler()

	mockComponent := &MockScalableComponent{
		name:         "test_component",
		currentScale: 100,
		minScale:     10,
		maxScale:     1000,
		isHealthy:    true,
	}

	err := as.RegisterScalableComponent("test_component", mockComponent)
	if err != nil {
		t.Fatalf("Failed to register component: %v", err)
	}

	as.mu.RLock()
	registered, exists := as.components["test_component"]
	as.mu.RUnlock()

	if !exists {
		t.Error("Component should be registered")
	}

	if registered != mockComponent {
		t.Error("Registered component should match the original")
	}
}

func TestAutoScaler_ForceScale(t *testing.T) {
	as, _, _ := createTestAutoScaler()

	mockComponent := &MockScalableComponent{
		name:         "test_component",
		currentScale: 100,
		minScale:     10,
		maxScale:     1000,
		isHealthy:    true,
	}

	as.RegisterScalableComponent("test_component", mockComponent)

	// Test scale up
	err := as.ForceScale("test_component", ScaleUp, 1.5)
	if err != nil {
		t.Fatalf("Failed to force scale up: %v", err)
	}

	if mockComponent.currentScale != 150 {
		t.Errorf("Expected scale 150, got %d", mockComponent.currentScale)
	}

	// Test scale down
	err = as.ForceScale("test_component", ScaleDown, 0.5)
	if err != nil {
		t.Fatalf("Failed to force scale down: %v", err)
	}

	if mockComponent.currentScale != 75 {
		t.Errorf("Expected scale 75, got %d", mockComponent.currentScale)
	}

	// Test scale with constraints
	err = as.ForceScale("test_component", ScaleDown, 0.01) // Would go below min
	if err != nil {
		t.Fatalf("Failed to force scale down: %v", err)
	}

	if mockComponent.currentScale != 10 { // Should be clamped to min
		t.Errorf("Expected scale 10 (min), got %d", mockComponent.currentScale)
	}

	// Test non-existent component
	err = as.ForceScale("non_existent", ScaleUp, 2.0)
	if err == nil {
		t.Error("Expected error when scaling non-existent component")
	}
}

func TestAutoScaler_IsolateComponent(t *testing.T) {
	as, _, _ := createTestAutoScaler()

	mockComponent := &MockScalableComponent{
		name:         "test_component",
		currentScale: 100,
		minScale:     10,
		maxScale:     1000,
		isHealthy:    true,
	}

	as.RegisterScalableComponent("test_component", mockComponent)

	// Test isolation
	err := as.IsolateComponent("test_component", "test isolation")
	if err != nil {
		t.Fatalf("Failed to isolate component: %v", err)
	}

	// Check if component is isolated
	if !as.isComponentIsolated("test_component") {
		t.Error("Component should be isolated")
	}

	// Check if component was scaled down to minimum
	if mockComponent.currentScale != 10 {
		t.Errorf("Expected scale 10 (min), got %d", mockComponent.currentScale)
	}

	// Test double isolation
	err = as.IsolateComponent("test_component", "double isolation")
	if err == nil {
		t.Error("Expected error when isolating already isolated component")
	}

	// Test isolation of non-existent component
	err = as.IsolateComponent("non_existent", "test")
	if err == nil {
		t.Error("Expected error when isolating non-existent component")
	}
}

func TestAutoScaler_RestoreComponent(t *testing.T) {
	as, _, _ := createTestAutoScaler()

	mockComponent := &MockScalableComponent{
		name:         "test_component",
		currentScale: 100,
		minScale:     10,
		maxScale:     1000,
		isHealthy:    true,
	}

	as.RegisterScalableComponent("test_component", mockComponent)

	// First isolate the component
	err := as.IsolateComponent("test_component", "test isolation")
	if err != nil {
		t.Fatalf("Failed to isolate component: %v", err)
	}

	// Test restoration
	err = as.RestoreComponent("test_component")
	if err != nil {
		t.Fatalf("Failed to restore component: %v", err)
	}

	// Check if component is no longer isolated
	if as.isComponentIsolated("test_component") {
		t.Error("Component should not be isolated after restoration")
	}

	// Check if component was restored to original scale
	if mockComponent.currentScale != 100 {
		t.Errorf("Expected scale 100 (original), got %d", mockComponent.currentScale)
	}

	// Test restoration of non-isolated component
	err = as.RestoreComponent("test_component")
	if err == nil {
		t.Error("Expected error when restoring non-isolated component")
	}

	// Test restoration of non-existent component
	err = as.RestoreComponent("non_existent")
	if err == nil {
		t.Error("Expected error when restoring non-existent component")
	}
}

func TestAutoScaler_SetScalingRules(t *testing.T) {
	as, _, _ := createTestAutoScaler()

	rules := []ScalingRule{
		{
			ID:          "test-rule-1",
			Name:        "Test Rule 1",
			Component:   "test_component",
			Metric:      "cpu_usage",
			Condition:   ConditionGreaterThan,
			Threshold:   80.0,
			Direction:   ScaleUp,
			ScaleFactor: 1.5,
			MinValue:    10,
			MaxValue:    1000,
			Cooldown:    1 * time.Minute,
			Priority:    100,
			Enabled:     true,
		},
		{
			ID:          "test-rule-2",
			Name:        "Test Rule 2",
			Component:   "test_component",
			Metric:      "cpu_usage",
			Condition:   ConditionLessThan,
			Threshold:   20.0,
			Direction:   ScaleDown,
			ScaleFactor: 0.8,
			MinValue:    10,
			MaxValue:    1000,
			Cooldown:    2 * time.Minute,
			Priority:    90,
			Enabled:     true,
		},
	}

	err := as.SetScalingRules(rules)
	if err != nil {
		t.Fatalf("Failed to set scaling rules: %v", err)
	}

	as.mu.RLock()
	if len(as.rules) != 2 {
		t.Errorf("Expected 2 rules, got %d", len(as.rules))
	}
	as.mu.RUnlock()

	// Test invalid rules
	invalidRules := []ScalingRule{
		{
			ID:          "", // Empty ID
			Component:   "test_component",
			ScaleFactor: 1.5,
			MinValue:    10,
			MaxValue:    1000,
		},
	}

	err = as.SetScalingRules(invalidRules)
	if err == nil {
		t.Error("Expected error for invalid rules")
	}

	// Test rule with invalid scale factor
	invalidRules = []ScalingRule{
		{
			ID:          "invalid-rule",
			Component:   "test_component",
			ScaleFactor: 0, // Invalid scale factor
			MinValue:    10,
			MaxValue:    1000,
		},
	}

	err = as.SetScalingRules(invalidRules)
	if err == nil {
		t.Error("Expected error for invalid scale factor")
	}

	// Test rule with invalid min/max values
	invalidRules = []ScalingRule{
		{
			ID:          "invalid-rule",
			Component:   "test_component",
			ScaleFactor: 1.5,
			MinValue:    100,
			MaxValue:    50, // Max < Min
		},
	}

	err = as.SetScalingRules(invalidRules)
	if err == nil {
		t.Error("Expected error for invalid min/max values")
	}
}

func TestAutoScaler_GetScalingStatus(t *testing.T) {
	as, _, _ := createTestAutoScaler()

	mockComponent := &MockScalableComponent{
		name:         "test_component",
		currentScale: 100,
		minScale:     10,
		maxScale:     1000,
		isHealthy:    true,
	}

	as.RegisterScalableComponent("test_component", mockComponent)

	// Add some scaling history
	event := ScalingEvent{
		Timestamp:   time.Now(),
		Component:   "test_component",
		Direction:   ScaleUp,
		FromScale:   100,
		ToScale:     150,
		ScaleFactor: 1.5,
		Trigger:     "test",
		Success:     true,
	}

	as.mu.Lock()
	as.addToHistory(event)
	as.mu.Unlock()

	status := as.GetScalingStatus()

	if status.IsActive {
		t.Error("Expected scaling status to show as inactive (not started)")
	}

	if len(status.ComponentScales) < 1 {
		t.Errorf("Expected at least 1 component scale, got %d", len(status.ComponentScales))
	}

	if status.ComponentScales["test_component"] != 100 {
		t.Errorf("Expected component scale 100, got %d", status.ComponentScales["test_component"])
	}

	if len(status.ScalingHistory) != 1 {
		t.Errorf("Expected 1 scaling event in history, got %d", len(status.ScalingHistory))
	}

	if status.LastScalingEvent == nil {
		t.Error("Expected last scaling event to be set")
	}

	if status.Metrics == nil {
		t.Error("Expected metrics to be included in status")
	}
}

func TestAutoScaler_GetIsolatedComponents(t *testing.T) {
	as, _, _ := createTestAutoScaler()

	mockComponent1 := &MockScalableComponent{
		name:         "component1",
		currentScale: 100,
		minScale:     10,
		maxScale:     1000,
		isHealthy:    true,
	}

	mockComponent2 := &MockScalableComponent{
		name:         "component2",
		currentScale: 200,
		minScale:     20,
		maxScale:     2000,
		isHealthy:    true,
	}

	as.RegisterScalableComponent("component1", mockComponent1)
	as.RegisterScalableComponent("component2", mockComponent2)

	// Initially no isolated components
	isolated := as.GetIsolatedComponents()
	if len(isolated) != 0 {
		t.Errorf("Expected 0 isolated components, got %d", len(isolated))
	}

	// Isolate first component
	err := as.IsolateComponent("component1", "test isolation 1")
	if err != nil {
		t.Fatalf("Failed to isolate component1: %v", err)
	}

	isolated = as.GetIsolatedComponents()
	if len(isolated) != 1 {
		t.Errorf("Expected 1 isolated component, got %d", len(isolated))
	}

	if isolated[0].Name != "component1" {
		t.Errorf("Expected isolated component 'component1', got '%s'", isolated[0].Name)
	}

	if isolated[0].Reason != "test isolation 1" {
		t.Errorf("Expected isolation reason 'test isolation 1', got '%s'", isolated[0].Reason)
	}

	// Isolate second component
	err = as.IsolateComponent("component2", "test isolation 2")
	if err != nil {
		t.Fatalf("Failed to isolate component2: %v", err)
	}

	isolated = as.GetIsolatedComponents()
	if len(isolated) != 2 {
		t.Errorf("Expected 2 isolated components, got %d", len(isolated))
	}

	// Restore first component
	err = as.RestoreComponent("component1")
	if err != nil {
		t.Fatalf("Failed to restore component1: %v", err)
	}

	isolated = as.GetIsolatedComponents()
	if len(isolated) != 1 {
		t.Errorf("Expected 1 isolated component after restoration, got %d", len(isolated))
	}

	if isolated[0].Name != "component2" {
		t.Errorf("Expected remaining isolated component 'component2', got '%s'", isolated[0].Name)
	}
}

func TestAutoScaler_ScalingCallback(t *testing.T) {
	as, _, _ := createTestAutoScaler()

	mockComponent := &MockScalableComponent{
		name:         "test_component",
		currentScale: 100,
		minScale:     10,
		maxScale:     1000,
		isHealthy:    true,
	}

	as.RegisterScalableComponent("test_component", mockComponent)

	// Set up callback
	var callbackEvent *ScalingEvent
	callback := func(event ScalingEvent) {
		callbackEvent = &event
	}

	err := as.SetScalingCallback(callback)
	if err != nil {
		t.Fatalf("Failed to set scaling callback: %v", err)
	}

	// Trigger scaling
	err = as.ForceScale("test_component", ScaleUp, 1.5)
	if err != nil {
		t.Fatalf("Failed to force scale: %v", err)
	}

	// Wait for callback (it's called in a goroutine)
	time.Sleep(10 * time.Millisecond)

	if callbackEvent == nil {
		t.Error("Expected scaling callback to be called")
	}

	if callbackEvent.Component != "test_component" {
		t.Errorf("Expected callback component 'test_component', got '%s'", callbackEvent.Component)
	}

	if callbackEvent.Direction != ScaleUp {
		t.Errorf("Expected callback direction ScaleUp, got %s", callbackEvent.Direction.String())
	}

	if callbackEvent.FromScale != 100 {
		t.Errorf("Expected callback from scale 100, got %d", callbackEvent.FromScale)
	}

	if callbackEvent.ToScale != 150 {
		t.Errorf("Expected callback to scale 150, got %d", callbackEvent.ToScale)
	}
}

func TestAutoScaler_EvaluateScalingRule(t *testing.T) {
	as, mockPerfMonitor, _ := createTestAutoScaler()

	// Test CPU usage rule
	rule := ScalingRule{
		ID:          "cpu-scale-up",
		Component:   "worker_pool",
		Metric:      "cpu_usage",
		Condition:   ConditionGreaterThan,
		Threshold:   80.0,
		Direction:   ScaleUp,
		ScaleFactor: 1.5,
	}

	// Set CPU usage to 90% (should trigger)
	mockPerfMonitor.SetMetrics(&PerformanceMetrics{
		ResourceMetrics: ResourceStats{
			CPUUsage: 90.0,
		},
	})

	result := as.evaluateScalingRule(rule, mockPerfMonitor.CollectMetrics())
	if !result {
		t.Error("Expected rule to trigger with CPU usage 90% > 80%")
	}

	// Set CPU usage to 70% (should not trigger)
	mockPerfMonitor.SetMetrics(&PerformanceMetrics{
		ResourceMetrics: ResourceStats{
			CPUUsage: 70.0,
		},
	})

	result = as.evaluateScalingRule(rule, mockPerfMonitor.CollectMetrics())
	if result {
		t.Error("Expected rule not to trigger with CPU usage 70% < 80%")
	}

	// Test connection utilization rule
	mockComponent := &MockScalableComponent{
		name:         "connection_pool",
		currentScale: 1000,
		minScale:     100,
		maxScale:     5000,
		isHealthy:    true,
	}

	as.RegisterScalableComponent("connection_pool", mockComponent)

	rule = ScalingRule{
		ID:          "connection-scale-up",
		Component:   "connection_pool",
		Metric:      "connection_utilization",
		Condition:   ConditionGreaterThan,
		Threshold:   0.8,
		Direction:   ScaleUp,
		ScaleFactor: 1.25,
	}

	// Set active connections to 900 (90% utilization, should trigger)
	mockPerfMonitor.SetMetrics(&PerformanceMetrics{
		NetworkMetrics: NetworkStats{
			ActiveConnections: 900,
		},
	})

	result = as.evaluateScalingRule(rule, mockPerfMonitor.CollectMetrics())
	if !result {
		t.Error("Expected rule to trigger with connection utilization 90% > 80%")
	}

	// Set active connections to 700 (70% utilization, should not trigger)
	mockPerfMonitor.SetMetrics(&PerformanceMetrics{
		NetworkMetrics: NetworkStats{
			ActiveConnections: 700,
		},
	})

	result = as.evaluateScalingRule(rule, mockPerfMonitor.CollectMetrics())
	if result {
		t.Error("Expected rule not to trigger with connection utilization 70% < 80%")
	}
}

func TestAutoScaler_CooldownPeriod(t *testing.T) {
	as, _, _ := createTestAutoScaler()

	// Test cooldown functionality
	component := "test_component"
	cooldown := 1 * time.Minute

	// Initially not in cooldown
	if as.isInCooldown(component, cooldown) {
		t.Error("Component should not be in cooldown initially")
	}

	// Set last scaling time to now
	as.mu.Lock()
	as.lastScalingTime[component] = time.Now()
	as.mu.Unlock()

	// Should be in cooldown
	if !as.isInCooldown(component, cooldown) {
		t.Error("Component should be in cooldown after recent scaling")
	}

	// Set last scaling time to past
	as.mu.Lock()
	as.lastScalingTime[component] = time.Now().Add(-2 * time.Minute)
	as.mu.Unlock()

	// Should not be in cooldown
	if as.isInCooldown(component, cooldown) {
		t.Error("Component should not be in cooldown after cooldown period")
	}
}

func TestAutoScaler_ComponentHealth(t *testing.T) {
	as, mockPerfMonitor, _ := createTestAutoScaler()

	mockComponent := &MockScalableComponent{
		name:         "test_component",
		currentScale: 100,
		minScale:     10,
		maxScale:     1000,
		isHealthy:    true,
		metrics: map[string]interface{}{
			"utilization": 0.75,
			"error_rate":  0.02,
		},
	}

	as.RegisterScalableComponent("test_component", mockComponent)

	// Update component health
	as.updateComponentHealth(mockPerfMonitor.CollectMetrics())

	as.mu.RLock()
	health, exists := as.componentHealth["test_component"]
	as.mu.RUnlock()

	if !exists {
		t.Error("Component health should exist")
	}

	if !health.IsHealthy {
		t.Error("Component should be healthy")
	}

	if health.CurrentScale != 100 {
		t.Errorf("Expected current scale 100, got %d", health.CurrentScale)
	}

	if health.MinScale != 10 {
		t.Errorf("Expected min scale 10, got %d", health.MinScale)
	}

	if health.MaxScale != 1000 {
		t.Errorf("Expected max scale 1000, got %d", health.MaxScale)
	}

	if health.IsIsolated {
		t.Error("Component should not be isolated")
	}

	// Test with isolated component
	err := as.IsolateComponent("test_component", "test isolation")
	if err != nil {
		t.Fatalf("Failed to isolate component: %v", err)
	}

	as.updateComponentHealth(mockPerfMonitor.CollectMetrics())

	as.mu.RLock()
	health, exists = as.componentHealth["test_component"]
	as.mu.RUnlock()

	if !exists {
		t.Error("Component health should exist after isolation")
	}

	if !health.IsIsolated {
		t.Error("Component should be isolated")
	}

	if health.IsolationReason != "test isolation" {
		t.Errorf("Expected isolation reason 'test isolation', got '%s'", health.IsolationReason)
	}

	if health.IsolatedAt == nil {
		t.Error("Isolated at time should be set")
	}
}

// Benchmark tests
func BenchmarkAutoScaler_EvaluateScaling(b *testing.B) {
	as, _, _ := createTestAutoScaler()

	// Register multiple components
	for i := 0; i < 10; i++ {
		mockComponent := &MockScalableComponent{
			name:         fmt.Sprintf("component_%d", i),
			currentScale: 100,
			minScale:     10,
			maxScale:     1000,
			isHealthy:    true,
		}
		as.RegisterScalableComponent(mockComponent.name, mockComponent)
	}

	ctx := context.Background()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		as.evaluateScaling(ctx)
	}
}

func BenchmarkAutoScaler_ForceScale(b *testing.B) {
	as, _, _ := createTestAutoScaler()

	mockComponent := &MockScalableComponent{
		name:         "test_component",
		currentScale: 100,
		minScale:     10,
		maxScale:     1000,
		isHealthy:    true,
	}

	as.RegisterScalableComponent("test_component", mockComponent)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		direction := ScaleUp
		if i%2 == 0 {
			direction = ScaleDown
		}
		as.ForceScale("test_component", direction, 1.1)
	}
}

func BenchmarkAutoScaler_GetScalingStatus(b *testing.B) {
	as, _, _ := createTestAutoScaler()

	// Register multiple components and add history
	for i := 0; i < 100; i++ {
		mockComponent := &MockScalableComponent{
			name:         fmt.Sprintf("component_%d", i),
			currentScale: 100,
			minScale:     10,
			maxScale:     1000,
			isHealthy:    true,
		}
		as.RegisterScalableComponent(mockComponent.name, mockComponent)

		// Add scaling history
		event := ScalingEvent{
			Timestamp:   time.Now(),
			Component:   mockComponent.name,
			Direction:   ScaleUp,
			FromScale:   100,
			ToScale:     150,
			ScaleFactor: 1.5,
			Trigger:     "benchmark",
			Success:     true,
		}
		as.mu.Lock()
		as.addToHistory(event)
		as.mu.Unlock()
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		as.GetScalingStatus()
	}
}
