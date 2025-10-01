package monitoring

import (
	"context"
	"fmt"
	"log/slog"
	"math"
	"sync"
	"time"
)

// AutoScaler manages automatic scaling of system components
type AutoScaler interface {
	// Start begins auto-scaling operations
	Start(ctx context.Context) error

	// Stop stops auto-scaling operations
	Stop() error

	// SetScalingRules configures scaling rules
	SetScalingRules(rules []ScalingRule) error

	// GetScalingStatus returns current scaling status
	GetScalingStatus() ScalingStatus

	// ForceScale forces scaling of a specific component
	ForceScale(component string, direction ScalingDirection, factor float64) error

	// RegisterScalableComponent registers a component that can be scaled
	RegisterScalableComponent(name string, component ScalableComponent) error

	// IsolateComponent isolates a problematic component
	IsolateComponent(component string, reason string) error

	// RestoreComponent restores an isolated component
	RestoreComponent(component string) error

	// GetIsolatedComponents returns list of isolated components
	GetIsolatedComponents() []IsolatedComponent

	// SetScalingCallback sets callback for scaling events
	SetScalingCallback(callback ScalingCallback) error
}

// ScalingDirection represents the direction of scaling
type ScalingDirection int

const (
	ScaleUp ScalingDirection = iota
	ScaleDown
	ScaleStable
)

func (sd ScalingDirection) String() string {
	switch sd {
	case ScaleUp:
		return "up"
	case ScaleDown:
		return "down"
	case ScaleStable:
		return "stable"
	default:
		return "unknown"
	}
}

// ScalingRule defines conditions and actions for auto-scaling
type ScalingRule struct {
	ID          string           `json:"id"`
	Name        string           `json:"name"`
	Description string           `json:"description"`
	Component   string           `json:"component"`
	Metric      string           `json:"metric"`
	Condition   AlertCondition   `json:"condition"`
	Threshold   interface{}      `json:"threshold"`
	Direction   ScalingDirection `json:"direction"`
	ScaleFactor float64          `json:"scale_factor"` // 0.5 = scale down by 50%, 2.0 = scale up by 100%
	MinValue    int              `json:"min_value"`
	MaxValue    int              `json:"max_value"`
	Cooldown    time.Duration    `json:"cooldown"`
	Priority    int              `json:"priority"`
	Enabled     bool             `json:"enabled"`
	CreatedAt   time.Time        `json:"created_at"`
	UpdatedAt   time.Time        `json:"updated_at"`
}

// ScalableComponent represents a component that can be scaled
type ScalableComponent interface {
	// GetCurrentScale returns current scale value
	GetCurrentScale() int

	// SetScale sets the scale to the specified value
	SetScale(ctx context.Context, scale int) error

	// GetScaleRange returns min and max scale values
	GetScaleRange() (min, max int)

	// GetComponentName returns the component name
	GetComponentName() string

	// IsHealthy returns whether the component is healthy
	IsHealthy() bool

	// GetMetrics returns component-specific metrics
	GetMetrics() map[string]interface{}
}

// ScalingStatus contains current auto-scaling status
type ScalingStatus struct {
	IsActive         bool                       `json:"is_active"`
	ActiveRules      []string                   `json:"active_rules"`
	ComponentScales  map[string]int             `json:"component_scales"`
	IsolatedCount    int                        `json:"isolated_count"`
	LastScalingEvent *ScalingEvent              `json:"last_scaling_event,omitempty"`
	ScalingHistory   []ScalingEvent             `json:"scaling_history"`
	ComponentHealth  map[string]ComponentHealth `json:"component_health"`
	Metrics          *PerformanceMetrics        `json:"metrics"`
}

// ScalingEvent represents a scaling event
type ScalingEvent struct {
	Timestamp   time.Time              `json:"timestamp"`
	Component   string                 `json:"component"`
	Direction   ScalingDirection       `json:"direction"`
	FromScale   int                    `json:"from_scale"`
	ToScale     int                    `json:"to_scale"`
	ScaleFactor float64                `json:"scale_factor"`
	Trigger     string                 `json:"trigger"`
	Success     bool                   `json:"success"`
	Error       string                 `json:"error,omitempty"`
	Metrics     map[string]interface{} `json:"metrics"`
}

// ComponentHealth represents the health status of a component
type ComponentHealth struct {
	IsHealthy       bool                   `json:"is_healthy"`
	IsIsolated      bool                   `json:"is_isolated"`
	IsolationReason string                 `json:"isolation_reason,omitempty"`
	IsolatedAt      *time.Time             `json:"isolated_at,omitempty"`
	CurrentScale    int                    `json:"current_scale"`
	MinScale        int                    `json:"min_scale"`
	MaxScale        int                    `json:"max_scale"`
	LastScaled      *time.Time             `json:"last_scaled,omitempty"`
	Metrics         map[string]interface{} `json:"metrics"`
}

// IsolatedComponent represents an isolated component
type IsolatedComponent struct {
	Name          string        `json:"name"`
	Reason        string        `json:"reason"`
	IsolatedAt    time.Time     `json:"isolated_at"`
	OriginalScale int           `json:"original_scale"`
	IsolatedScale int           `json:"isolated_scale"`
	AutoRestore   bool          `json:"auto_restore"`
	RestoreAfter  time.Duration `json:"restore_after"`
}

// ScalingCallback is called when scaling events occur
type ScalingCallback func(event ScalingEvent)

// autoScaler implements AutoScaler
type autoScaler struct {
	mu                 sync.RWMutex
	performanceMonitor PerformanceMonitor
	resourceMonitor    ResourceMonitorInterface
	logger             *slog.Logger

	// Scaling state
	isRunning       bool
	stopChan        chan struct{}
	monitorInterval time.Duration

	// Configuration
	rules           []ScalingRule
	components      map[string]ScalableComponent
	scalingCallback ScalingCallback

	// Scaling management
	lastScalingTime map[string]time.Time
	scalingHistory  []ScalingEvent
	maxHistorySize  int

	// Component isolation
	isolatedComponents map[string]*IsolatedComponent
	isolationTimeout   time.Duration

	// Health monitoring
	componentHealth map[string]*ComponentHealth
}

// ResourceMonitorInterface defines the interface for resource monitoring
type ResourceMonitorInterface interface {
	GetThresholds() ResourceMonitorThresholds
	GetCurrentStatus() map[string]interface{}
}

// NewAutoScaler creates a new auto-scaler
func NewAutoScaler(
	performanceMonitor PerformanceMonitor,
	resourceMonitor ResourceMonitorInterface,
	logger *slog.Logger,
) AutoScaler {
	if logger == nil {
		logger = slog.Default()
	}

	as := &autoScaler{
		performanceMonitor: performanceMonitor,
		resourceMonitor:    resourceMonitor,
		logger:             logger,
		stopChan:           make(chan struct{}),
		monitorInterval:    30 * time.Second,
		components:         make(map[string]ScalableComponent),
		lastScalingTime:    make(map[string]time.Time),
		scalingHistory:     make([]ScalingEvent, 0),
		maxHistorySize:     1000,
		isolatedComponents: make(map[string]*IsolatedComponent),
		isolationTimeout:   5 * time.Minute,
		componentHealth:    make(map[string]*ComponentHealth),
	}

	// Set default scaling rules
	as.rules = getDefaultScalingRules()

	// Register default scalable components
	as.registerDefaultComponents()

	return as
}

// getDefaultScalingRules returns default scaling rules
func getDefaultScalingRules() []ScalingRule {
	return []ScalingRule{
		{
			ID:          "worker-threads-scale-up",
			Name:        "Worker Threads Scale Up",
			Description: "Scale up worker threads when CPU usage is high but response time is good",
			Component:   "worker_pool",
			Metric:      "cpu_usage",
			Condition:   ConditionGreaterThan,
			Threshold:   70.0,
			Direction:   ScaleUp,
			ScaleFactor: 1.5,
			MinValue:    64,
			MaxValue:    2048,
			Cooldown:    2 * time.Minute,
			Priority:    100,
			Enabled:     true,
			CreatedAt:   time.Now(),
			UpdatedAt:   time.Now(),
		},
		{
			ID:          "worker-threads-scale-down",
			Name:        "Worker Threads Scale Down",
			Description: "Scale down worker threads when CPU usage is low",
			Component:   "worker_pool",
			Metric:      "cpu_usage",
			Condition:   ConditionLessThan,
			Threshold:   30.0,
			Direction:   ScaleDown,
			ScaleFactor: 0.75,
			MinValue:    64,
			MaxValue:    2048,
			Cooldown:    5 * time.Minute,
			Priority:    90,
			Enabled:     true,
			CreatedAt:   time.Now(),
			UpdatedAt:   time.Now(),
		},
		{
			ID:          "connection-pool-scale-up",
			Name:        "Connection Pool Scale Up",
			Description: "Scale up connection pool when active connections are high",
			Component:   "connection_pool",
			Metric:      "connection_utilization",
			Condition:   ConditionGreaterThan,
			Threshold:   0.8, // 80% utilization
			Direction:   ScaleUp,
			ScaleFactor: 1.25,
			MinValue:    500,
			MaxValue:    5000,
			Cooldown:    1 * time.Minute,
			Priority:    120,
			Enabled:     true,
			CreatedAt:   time.Now(),
			UpdatedAt:   time.Now(),
		},
		{
			ID:          "connection-pool-scale-down",
			Name:        "Connection Pool Scale Down",
			Description: "Scale down connection pool when utilization is low",
			Component:   "connection_pool",
			Metric:      "connection_utilization",
			Condition:   ConditionLessThan,
			Threshold:   0.3, // 30% utilization
			Direction:   ScaleDown,
			ScaleFactor: 0.8,
			MinValue:    500,
			MaxValue:    5000,
			Cooldown:    3 * time.Minute,
			Priority:    80,
			Enabled:     true,
			CreatedAt:   time.Now(),
			UpdatedAt:   time.Now(),
		},
		{
			ID:          "batch-processor-scale-up",
			Name:        "Batch Processor Scale Up",
			Description: "Scale up batch processors when queue length is high",
			Component:   "batch_processor",
			Metric:      "queue_length",
			Condition:   ConditionGreaterThan,
			Threshold:   1000,
			Direction:   ScaleUp,
			ScaleFactor: 1.3,
			MinValue:    10,
			MaxValue:    200,
			Cooldown:    1 * time.Minute,
			Priority:    110,
			Enabled:     true,
			CreatedAt:   time.Now(),
			UpdatedAt:   time.Now(),
		},
		{
			ID:          "memory-pressure-scale-down",
			Name:        "Memory Pressure Scale Down",
			Description: "Scale down components when memory pressure is high",
			Component:   "worker_pool",
			Metric:      "memory_usage_percent",
			Condition:   ConditionGreaterThan,
			Threshold:   85.0,
			Direction:   ScaleDown,
			ScaleFactor: 0.7,
			MinValue:    64,
			MaxValue:    2048,
			Cooldown:    30 * time.Second,
			Priority:    200,
			Enabled:     true,
			CreatedAt:   time.Now(),
			UpdatedAt:   time.Now(),
		},
	}
}

// registerDefaultComponents registers default scalable components
func (as *autoScaler) registerDefaultComponents() {
	// Register default components (these would be implemented elsewhere)
	as.components["worker_pool"] = &WorkerPoolComponent{}
	as.components["connection_pool"] = &ConnectionPoolComponent{}
	as.components["batch_processor"] = &BatchProcessorComponent{}
}

// Start begins auto-scaling operations
func (as *autoScaler) Start(ctx context.Context) error {
	as.mu.Lock()
	defer as.mu.Unlock()

	if as.isRunning {
		return fmt.Errorf("auto-scaler is already running")
	}

	as.isRunning = true

	go as.monitorLoop(ctx)

	as.logger.Info("Auto-scaler started",
		slog.Int("rules_count", len(as.rules)),
		slog.Int("components_count", len(as.components)),
		slog.Duration("monitor_interval", as.monitorInterval),
	)

	return nil
}

// Stop stops auto-scaling operations
func (as *autoScaler) Stop() error {
	as.mu.Lock()
	defer as.mu.Unlock()

	if !as.isRunning {
		return nil
	}

	as.isRunning = false
	close(as.stopChan)
	as.stopChan = make(chan struct{})

	as.logger.Info("Auto-scaler stopped")
	return nil
}

// monitorLoop is the main monitoring loop
func (as *autoScaler) monitorLoop(ctx context.Context) {
	ticker := time.NewTicker(as.monitorInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-as.stopChan:
			return
		case <-ticker.C:
			as.evaluateScaling(ctx)
			as.checkIsolatedComponents(ctx)
		}
	}
}

// evaluateScaling evaluates current metrics and applies scaling if needed
func (as *autoScaler) evaluateScaling(ctx context.Context) {
	metrics := as.performanceMonitor.CollectMetrics()
	if metrics == nil {
		as.logger.Warn("Failed to collect metrics for scaling evaluation")
		return
	}

	// Update component health
	as.updateComponentHealth(metrics)

	// Sort rules by priority (highest first)
	sortedRules := make([]ScalingRule, len(as.rules))
	copy(sortedRules, as.rules)

	for i := 0; i < len(sortedRules)-1; i++ {
		for j := i + 1; j < len(sortedRules); j++ {
			if sortedRules[i].Priority < sortedRules[j].Priority {
				sortedRules[i], sortedRules[j] = sortedRules[j], sortedRules[i]
			}
		}
	}

	// Evaluate scaling rules
	for _, rule := range sortedRules {
		if !rule.Enabled {
			continue
		}

		// Check if component is isolated
		if as.isComponentIsolated(rule.Component) {
			continue
		}

		// Check cooldown
		if as.isInCooldown(rule.Component, rule.Cooldown) {
			continue
		}

		if as.evaluateScalingRule(rule, metrics) {
			as.applyScaling(ctx, rule, metrics)
		}
	}
}

// evaluateScalingRule evaluates a single scaling rule
func (as *autoScaler) evaluateScalingRule(rule ScalingRule, metrics *PerformanceMetrics) bool {
	value := as.extractMetricValue(rule.Component, rule.Metric, metrics)
	if value == nil {
		return false
	}

	return as.compareValues(value, rule.Condition, rule.Threshold)
}

// extractMetricValue extracts metric value from performance metrics
func (as *autoScaler) extractMetricValue(component, metric string, metrics *PerformanceMetrics) interface{} {
	switch component {
	case "worker_pool":
		switch metric {
		case "cpu_usage":
			return metrics.ResourceMetrics.CPUUsage
		case "queue_length":
			return metrics.BitswapMetrics.QueuedRequests
		case "memory_usage_percent":
			if metrics.ResourceMetrics.MemoryTotal > 0 {
				return float64(metrics.ResourceMetrics.MemoryUsage) / float64(metrics.ResourceMetrics.MemoryTotal) * 100
			}
		}
	case "connection_pool":
		switch metric {
		case "connection_utilization":
			// Calculate utilization based on active vs max connections
			if comp, exists := as.components[component]; exists {
				currentScale := comp.GetCurrentScale()
				activeConnections := metrics.NetworkMetrics.ActiveConnections
				if currentScale > 0 {
					return float64(activeConnections) / float64(currentScale)
				}
			}
		case "active_connections":
			return metrics.NetworkMetrics.ActiveConnections
		}
	case "batch_processor":
		switch metric {
		case "queue_length":
			return metrics.BitswapMetrics.QueuedRequests
		case "processing_rate":
			return metrics.BitswapMetrics.RequestsPerSecond
		}
	}

	return nil
}

// compareValues compares values using the specified condition
func (as *autoScaler) compareValues(value interface{}, condition AlertCondition, threshold interface{}) bool {
	switch condition {
	case ConditionGreaterThan:
		return as.compareNumeric(value, threshold, func(a, b float64) bool { return a > b })
	case ConditionLessThan:
		return as.compareNumeric(value, threshold, func(a, b float64) bool { return a < b })
	case ConditionGreaterOrEqual:
		return as.compareNumeric(value, threshold, func(a, b float64) bool { return a >= b })
	case ConditionLessOrEqual:
		return as.compareNumeric(value, threshold, func(a, b float64) bool { return a <= b })
	case ConditionEquals:
		return as.compareEqual(value, threshold)
	case ConditionNotEquals:
		return !as.compareEqual(value, threshold)
	}

	return false
}

// compareNumeric compares numeric values
func (as *autoScaler) compareNumeric(value, threshold interface{}, compareFn func(float64, float64) bool) bool {
	v1, ok1 := as.toFloat64(value)
	v2, ok2 := as.toFloat64(threshold)

	if !ok1 || !ok2 {
		return false
	}

	return compareFn(v1, v2)
}

// compareEqual compares values for equality
func (as *autoScaler) compareEqual(value, threshold interface{}) bool {
	if v1, ok1 := as.toFloat64(value); ok1 {
		if v2, ok2 := as.toFloat64(threshold); ok2 {
			return v1 == v2
		}
	}

	return fmt.Sprintf("%v", value) == fmt.Sprintf("%v", threshold)
}

// toFloat64 converts various numeric types to float64
func (as *autoScaler) toFloat64(value interface{}) (float64, bool) {
	switch v := value.(type) {
	case float64:
		return v, true
	case float32:
		return float64(v), true
	case int:
		return float64(v), true
	case int32:
		return float64(v), true
	case int64:
		return float64(v), true
	case uint:
		return float64(v), true
	case uint32:
		return float64(v), true
	case uint64:
		return float64(v), true
	case time.Duration:
		return float64(v.Nanoseconds()), true
	default:
		return 0, false
	}
}

// applyScaling applies scaling based on the rule
func (as *autoScaler) applyScaling(ctx context.Context, rule ScalingRule, metrics *PerformanceMetrics) {
	component, exists := as.components[rule.Component]
	if !exists {
		as.logger.Warn("Component not found for scaling",
			slog.String("component", rule.Component),
			slog.String("rule", rule.ID),
		)
		return
	}

	currentScale := component.GetCurrentScale()
	var newScale int

	switch rule.Direction {
	case ScaleUp:
		newScale = int(math.Ceil(float64(currentScale) * rule.ScaleFactor))
	case ScaleDown:
		newScale = int(math.Floor(float64(currentScale) * rule.ScaleFactor))
	default:
		return
	}

	// Apply min/max constraints
	if newScale < rule.MinValue {
		newScale = rule.MinValue
	}
	if newScale > rule.MaxValue {
		newScale = rule.MaxValue
	}

	// Don't scale if no change needed
	if newScale == currentScale {
		return
	}

	// Apply scaling
	event := ScalingEvent{
		Timestamp:   time.Now(),
		Component:   rule.Component,
		Direction:   rule.Direction,
		FromScale:   currentScale,
		ToScale:     newScale,
		ScaleFactor: rule.ScaleFactor,
		Trigger:     fmt.Sprintf("rule: %s", rule.ID),
		Metrics: map[string]interface{}{
			"cpu_usage":     metrics.ResourceMetrics.CPUUsage,
			"memory_usage":  metrics.ResourceMetrics.MemoryUsage,
			"response_time": metrics.BitswapMetrics.P95ResponseTime,
		},
	}

	if err := component.SetScale(ctx, newScale); err != nil {
		event.Success = false
		event.Error = err.Error()

		as.logger.Error("Failed to scale component",
			slog.String("component", rule.Component),
			slog.String("rule", rule.ID),
			slog.Int("from_scale", currentScale),
			slog.Int("to_scale", newScale),
			slog.String("error", err.Error()),
		)
	} else {
		event.Success = true

		as.logger.Info("Component scaled successfully",
			slog.String("component", rule.Component),
			slog.String("rule", rule.ID),
			slog.String("direction", rule.Direction.String()),
			slog.Int("from_scale", currentScale),
			slog.Int("to_scale", newScale),
			slog.Float64("scale_factor", rule.ScaleFactor),
		)
	}

	// Record scaling event
	as.mu.Lock()
	as.addToHistory(event)
	as.lastScalingTime[rule.Component] = time.Now()
	as.mu.Unlock()

	// Call scaling callback if set
	if as.scalingCallback != nil {
		go as.scalingCallback(event)
	}
}

// isInCooldown checks if a component is in cooldown period
func (as *autoScaler) isInCooldown(component string, cooldown time.Duration) bool {
	as.mu.RLock()
	defer as.mu.RUnlock()

	lastScaling, exists := as.lastScalingTime[component]
	if !exists {
		return false
	}

	return time.Since(lastScaling) < cooldown
}

// isComponentIsolated checks if a component is isolated
func (as *autoScaler) isComponentIsolated(component string) bool {
	as.mu.RLock()
	defer as.mu.RUnlock()

	_, isolated := as.isolatedComponents[component]
	return isolated
}

// updateComponentHealth updates the health status of all components
func (as *autoScaler) updateComponentHealth(metrics *PerformanceMetrics) {
	as.mu.Lock()
	defer as.mu.Unlock()

	for name, component := range as.components {
		health := &ComponentHealth{
			IsHealthy:    component.IsHealthy(),
			CurrentScale: component.GetCurrentScale(),
			Metrics:      component.GetMetrics(),
		}

		health.MinScale, health.MaxScale = component.GetScaleRange()

		if isolated, exists := as.isolatedComponents[name]; exists {
			health.IsIsolated = true
			health.IsolationReason = isolated.Reason
			health.IsolatedAt = &isolated.IsolatedAt
		}

		if lastScaling, exists := as.lastScalingTime[name]; exists {
			health.LastScaled = &lastScaling
		}

		as.componentHealth[name] = health
	}
}

// checkIsolatedComponents checks if isolated components can be restored
func (as *autoScaler) checkIsolatedComponents(ctx context.Context) {
	as.mu.Lock()
	componentsToRestore := make([]string, 0)

	for name, isolated := range as.isolatedComponents {
		if isolated.AutoRestore && time.Since(isolated.IsolatedAt) >= isolated.RestoreAfter {
			componentsToRestore = append(componentsToRestore, name)
		}
	}
	as.mu.Unlock()

	// Restore components outside the lock
	for _, name := range componentsToRestore {
		if err := as.RestoreComponent(name); err != nil {
			as.logger.Error("Failed to auto-restore isolated component",
				slog.String("component", name),
				slog.String("error", err.Error()),
			)
		}
	}
}

// addToHistory adds a scaling event to the history
func (as *autoScaler) addToHistory(event ScalingEvent) {
	as.scalingHistory = append(as.scalingHistory, event)

	if len(as.scalingHistory) > as.maxHistorySize {
		as.scalingHistory = as.scalingHistory[len(as.scalingHistory)-as.maxHistorySize:]
	}
}

// SetScalingRules configures scaling rules
func (as *autoScaler) SetScalingRules(rules []ScalingRule) error {
	as.mu.Lock()
	defer as.mu.Unlock()

	// Validate rules
	for _, rule := range rules {
		if rule.ID == "" {
			return fmt.Errorf("scaling rule ID cannot be empty")
		}
		if rule.Component == "" {
			return fmt.Errorf("scaling rule component cannot be empty for rule %s", rule.ID)
		}
		if rule.ScaleFactor <= 0 {
			return fmt.Errorf("scaling rule scale factor must be positive for rule %s", rule.ID)
		}
		if rule.MinValue < 0 {
			return fmt.Errorf("scaling rule min value cannot be negative for rule %s", rule.ID)
		}
		if rule.MaxValue <= rule.MinValue {
			return fmt.Errorf("scaling rule max value must be greater than min value for rule %s", rule.ID)
		}
	}

	as.rules = rules

	as.logger.Info("Scaling rules updated",
		slog.Int("rules_count", len(rules)),
	)

	return nil
}

// GetScalingStatus returns current scaling status
func (as *autoScaler) GetScalingStatus() ScalingStatus {
	as.mu.RLock()
	defer as.mu.RUnlock()

	activeRules := make([]string, 0)
	for _, rule := range as.rules {
		if rule.Enabled {
			activeRules = append(activeRules, rule.ID)
		}
	}

	componentScales := make(map[string]int)
	for name, component := range as.components {
		componentScales[name] = component.GetCurrentScale()
	}

	// Copy recent history
	historySize := 10
	if len(as.scalingHistory) < historySize {
		historySize = len(as.scalingHistory)
	}

	recentHistory := make([]ScalingEvent, historySize)
	if historySize > 0 {
		copy(recentHistory, as.scalingHistory[len(as.scalingHistory)-historySize:])
	}

	var lastEvent *ScalingEvent
	if len(as.scalingHistory) > 0 {
		lastEvent = &as.scalingHistory[len(as.scalingHistory)-1]
	}

	// Copy component health
	healthCopy := make(map[string]ComponentHealth)
	for name, health := range as.componentHealth {
		healthCopy[name] = *health
	}

	return ScalingStatus{
		IsActive:         as.isRunning,
		ActiveRules:      activeRules,
		ComponentScales:  componentScales,
		IsolatedCount:    len(as.isolatedComponents),
		LastScalingEvent: lastEvent,
		ScalingHistory:   recentHistory,
		ComponentHealth:  healthCopy,
		Metrics:          as.performanceMonitor.CollectMetrics(),
	}
}

// ForceScale forces scaling of a specific component
func (as *autoScaler) ForceScale(component string, direction ScalingDirection, factor float64) error {
	as.mu.RLock()
	comp, exists := as.components[component]
	as.mu.RUnlock()

	if !exists {
		return fmt.Errorf("component %s not found", component)
	}

	if as.isComponentIsolated(component) {
		return fmt.Errorf("component %s is isolated and cannot be scaled", component)
	}

	currentScale := comp.GetCurrentScale()
	var newScale int

	switch direction {
	case ScaleUp:
		newScale = int(math.Ceil(float64(currentScale) * factor))
	case ScaleDown:
		newScale = int(math.Floor(float64(currentScale) * factor))
	default:
		return fmt.Errorf("invalid scaling direction: %s", direction.String())
	}

	// Apply component constraints
	minScale, maxScale := comp.GetScaleRange()
	if newScale < minScale {
		newScale = minScale
	}
	if newScale > maxScale {
		newScale = maxScale
	}

	if newScale == currentScale {
		return fmt.Errorf("no scaling needed, current scale is already %d", currentScale)
	}

	// Apply scaling
	ctx := context.Background()
	event := ScalingEvent{
		Timestamp:   time.Now(),
		Component:   component,
		Direction:   direction,
		FromScale:   currentScale,
		ToScale:     newScale,
		ScaleFactor: factor,
		Trigger:     "manual",
		Metrics:     comp.GetMetrics(),
	}

	if err := comp.SetScale(ctx, newScale); err != nil {
		event.Success = false
		event.Error = err.Error()
		as.addToHistory(event)
		return fmt.Errorf("failed to scale component %s: %w", component, err)
	}

	event.Success = true

	as.mu.Lock()
	as.addToHistory(event)
	as.lastScalingTime[component] = time.Now()
	as.mu.Unlock()

	as.logger.Info("Component manually scaled",
		slog.String("component", component),
		slog.String("direction", direction.String()),
		slog.Int("from_scale", currentScale),
		slog.Int("to_scale", newScale),
		slog.Float64("scale_factor", factor),
	)

	// Call scaling callback if set
	if as.scalingCallback != nil {
		go as.scalingCallback(event)
	}

	return nil
}

// RegisterScalableComponent registers a component that can be scaled
func (as *autoScaler) RegisterScalableComponent(name string, component ScalableComponent) error {
	as.mu.Lock()
	defer as.mu.Unlock()

	as.components[name] = component

	as.logger.Info("Scalable component registered",
		slog.String("name", name),
		slog.String("component_type", component.GetComponentName()),
		slog.Int("current_scale", component.GetCurrentScale()),
	)

	return nil
}

// IsolateComponent isolates a problematic component
func (as *autoScaler) IsolateComponent(component string, reason string) error {
	as.mu.Lock()
	defer as.mu.Unlock()

	comp, exists := as.components[component]
	if !exists {
		return fmt.Errorf("component %s not found", component)
	}

	if _, isolated := as.isolatedComponents[component]; isolated {
		return fmt.Errorf("component %s is already isolated", component)
	}

	originalScale := comp.GetCurrentScale()
	minScale, _ := comp.GetScaleRange()

	// Scale down to minimum to isolate
	isolatedComponent := &IsolatedComponent{
		Name:          component,
		Reason:        reason,
		IsolatedAt:    time.Now(),
		OriginalScale: originalScale,
		IsolatedScale: minScale,
		AutoRestore:   true,
		RestoreAfter:  as.isolationTimeout,
	}

	as.isolatedComponents[component] = isolatedComponent

	// Scale down to minimum
	ctx := context.Background()
	if err := comp.SetScale(ctx, minScale); err != nil {
		delete(as.isolatedComponents, component)
		return fmt.Errorf("failed to isolate component %s: %w", component, err)
	}

	as.logger.Warn("Component isolated",
		slog.String("component", component),
		slog.String("reason", reason),
		slog.Int("original_scale", originalScale),
		slog.Int("isolated_scale", minScale),
		slog.Duration("auto_restore_after", as.isolationTimeout),
	)

	return nil
}

// RestoreComponent restores an isolated component
func (as *autoScaler) RestoreComponent(component string) error {
	as.mu.Lock()
	isolated, exists := as.isolatedComponents[component]
	if !exists {
		as.mu.Unlock()
		return fmt.Errorf("component %s is not isolated", component)
	}

	comp, compExists := as.components[component]
	if !compExists {
		as.mu.Unlock()
		return fmt.Errorf("component %s not found", component)
	}

	delete(as.isolatedComponents, component)
	as.mu.Unlock()

	// Restore to original scale
	ctx := context.Background()
	if err := comp.SetScale(ctx, isolated.OriginalScale); err != nil {
		// Re-add to isolated components if restore fails
		as.mu.Lock()
		as.isolatedComponents[component] = isolated
		as.mu.Unlock()
		return fmt.Errorf("failed to restore component %s: %w", component, err)
	}

	as.logger.Info("Component restored from isolation",
		slog.String("component", component),
		slog.String("isolation_reason", isolated.Reason),
		slog.Duration("isolation_duration", time.Since(isolated.IsolatedAt)),
		slog.Int("restored_scale", isolated.OriginalScale),
	)

	return nil
}

// GetIsolatedComponents returns list of isolated components
func (as *autoScaler) GetIsolatedComponents() []IsolatedComponent {
	as.mu.RLock()
	defer as.mu.RUnlock()

	isolated := make([]IsolatedComponent, 0, len(as.isolatedComponents))
	for _, component := range as.isolatedComponents {
		isolated = append(isolated, *component)
	}

	return isolated
}

// SetScalingCallback sets callback for scaling events
func (as *autoScaler) SetScalingCallback(callback ScalingCallback) error {
	as.mu.Lock()
	defer as.mu.Unlock()

	as.scalingCallback = callback
	return nil
}
