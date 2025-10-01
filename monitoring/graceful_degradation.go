package monitoring

import (
	"context"
	"fmt"
	"log/slog"
	"sync"
	"time"
)

// GracefulDegradationManager manages automatic service quality reduction during overload
type GracefulDegradationManager interface {
	// Start begins monitoring and degradation management
	Start(ctx context.Context) error

	// Stop stops degradation management
	Stop() error

	// SetDegradationRules configures degradation rules
	SetDegradationRules(rules []DegradationRule) error

	// GetCurrentDegradationLevel returns current degradation level
	GetCurrentDegradationLevel() DegradationLevel

	// GetDegradationStatus returns detailed degradation status
	GetDegradationStatus() DegradationStatus

	// ForceRecovery forces recovery to normal operation
	ForceRecovery() error

	// RegisterDegradationAction registers a custom degradation action
	RegisterDegradationAction(name string, action DegradationAction) error

	// SetRecoveryCallback sets callback for recovery events
	SetRecoveryCallback(callback RecoveryCallback) error
}

// DegradationLevel represents the current level of service degradation
type DegradationLevel int

const (
	DegradationNone DegradationLevel = iota
	DegradationLight
	DegradationModerate
	DegradationSevere
	DegradationCritical
)

func (dl DegradationLevel) String() string {
	switch dl {
	case DegradationNone:
		return "none"
	case DegradationLight:
		return "light"
	case DegradationModerate:
		return "moderate"
	case DegradationSevere:
		return "severe"
	case DegradationCritical:
		return "critical"
	default:
		return "unknown"
	}
}

// DegradationRule defines conditions and actions for service degradation
type DegradationRule struct {
	ID          string           `json:"id"`
	Name        string           `json:"name"`
	Description string           `json:"description"`
	Component   string           `json:"component"`
	Metric      string           `json:"metric"`
	Condition   AlertCondition   `json:"condition"`
	Threshold   interface{}      `json:"threshold"`
	Level       DegradationLevel `json:"level"`
	Actions     []string         `json:"actions"`
	Duration    time.Duration    `json:"duration"`
	Priority    int              `json:"priority"` // Higher priority rules are evaluated first
	Enabled     bool             `json:"enabled"`
	CreatedAt   time.Time        `json:"created_at"`
	UpdatedAt   time.Time        `json:"updated_at"`
}

// DegradationAction defines an action to take during degradation
type DegradationAction interface {
	// Execute performs the degradation action
	Execute(ctx context.Context, level DegradationLevel, metrics *PerformanceMetrics) error

	// Recover reverses the degradation action
	Recover(ctx context.Context) error

	// GetName returns the action name
	GetName() string

	// IsActive returns whether the action is currently active
	IsActive() bool
}

// DegradationStatus contains current degradation status information
type DegradationStatus struct {
	Level              DegradationLevel    `json:"level"`
	ActiveRules        []string            `json:"active_rules"`
	ActiveActions      []string            `json:"active_actions"`
	DegradationStarted *time.Time          `json:"degradation_started,omitempty"`
	LastLevelChange    time.Time           `json:"last_level_change"`
	RecoveryProgress   float64             `json:"recovery_progress"` // 0-1
	Metrics            *PerformanceMetrics `json:"metrics"`
	History            []DegradationEvent  `json:"history"`
}

// DegradationEvent represents a degradation state change event
type DegradationEvent struct {
	Timestamp time.Time              `json:"timestamp"`
	FromLevel DegradationLevel       `json:"from_level"`
	ToLevel   DegradationLevel       `json:"to_level"`
	Trigger   string                 `json:"trigger"`
	Actions   []string               `json:"actions"`
	Metrics   map[string]interface{} `json:"metrics"`
}

// RecoveryCallback is called when recovery events occur
type RecoveryCallback func(event RecoveryEvent)

// RecoveryEvent contains information about recovery events
type RecoveryEvent struct {
	Type      RecoveryEventType   `json:"type"`
	FromLevel DegradationLevel    `json:"from_level"`
	ToLevel   DegradationLevel    `json:"to_level"`
	Timestamp time.Time           `json:"timestamp"`
	Duration  time.Duration       `json:"duration"`
	Metrics   *PerformanceMetrics `json:"metrics"`
}

// RecoveryEventType represents the type of recovery event
type RecoveryEventType string

const (
	RecoveryEventLevelReduced   RecoveryEventType = "level_reduced"
	RecoveryEventFullRecovery   RecoveryEventType = "full_recovery"
	RecoveryEventForced         RecoveryEventType = "forced_recovery"
	RecoveryEventPartialFailure RecoveryEventType = "partial_failure"
)

// gracefulDegradationManager implements GracefulDegradationManager
type gracefulDegradationManager struct {
	mu                 sync.RWMutex
	performanceMonitor PerformanceMonitor
	resourceMonitor    *ResourceMonitor
	logger             *slog.Logger

	// Degradation state
	currentLevel       DegradationLevel
	degradationStarted *time.Time
	lastLevelChange    time.Time
	activeRules        map[string]*DegradationRule
	activeActions      map[string]DegradationAction

	// Configuration
	rules            []DegradationRule
	actions          map[string]DegradationAction
	recoveryCallback RecoveryCallback

	// Monitoring
	isRunning       bool
	stopChan        chan struct{}
	monitorInterval time.Duration

	// Recovery management
	recoveryThresholds map[DegradationLevel]RecoveryThreshold
	recoveryProgress   float64

	// History
	eventHistory   []DegradationEvent
	maxHistorySize int
}

// RecoveryThreshold defines conditions for recovery from a degradation level
type RecoveryThreshold struct {
	StableMetricsDuration time.Duration          `json:"stable_metrics_duration"`
	RequiredMetrics       map[string]interface{} `json:"required_metrics"`
	GradualRecovery       bool                   `json:"gradual_recovery"`
	RecoverySteps         int                    `json:"recovery_steps"`
}

// NewGracefulDegradationManager creates a new graceful degradation manager
func NewGracefulDegradationManager(
	performanceMonitor PerformanceMonitor,
	resourceMonitor *ResourceMonitor,
	logger *slog.Logger,
) GracefulDegradationManager {
	if logger == nil {
		logger = slog.Default()
	}

	gdm := &gracefulDegradationManager{
		performanceMonitor: performanceMonitor,
		resourceMonitor:    resourceMonitor,
		logger:             logger,
		currentLevel:       DegradationNone,
		lastLevelChange:    time.Now(),
		activeRules:        make(map[string]*DegradationRule),
		activeActions:      make(map[string]DegradationAction),
		actions:            make(map[string]DegradationAction),
		stopChan:           make(chan struct{}),
		monitorInterval:    10 * time.Second,
		recoveryThresholds: getDefaultRecoveryThresholds(),
		eventHistory:       make([]DegradationEvent, 0),
		maxHistorySize:     1000,
	}

	// Register default degradation actions
	gdm.registerDefaultActions()

	// Set default degradation rules
	gdm.rules = getDefaultDegradationRules()

	return gdm
}

// getDefaultRecoveryThresholds returns default recovery thresholds for each degradation level
func getDefaultRecoveryThresholds() map[DegradationLevel]RecoveryThreshold {
	return map[DegradationLevel]RecoveryThreshold{
		DegradationLight: {
			StableMetricsDuration: 30 * time.Second,
			RequiredMetrics: map[string]interface{}{
				"cpu_usage":     70.0,
				"memory_usage":  0.75,
				"response_time": 100 * time.Millisecond,
			},
			GradualRecovery: true,
			RecoverySteps:   3,
		},
		DegradationModerate: {
			StableMetricsDuration: 60 * time.Second,
			RequiredMetrics: map[string]interface{}{
				"cpu_usage":     60.0,
				"memory_usage":  0.70,
				"response_time": 80 * time.Millisecond,
			},
			GradualRecovery: true,
			RecoverySteps:   5,
		},
		DegradationSevere: {
			StableMetricsDuration: 120 * time.Second,
			RequiredMetrics: map[string]interface{}{
				"cpu_usage":     50.0,
				"memory_usage":  0.65,
				"response_time": 60 * time.Millisecond,
			},
			GradualRecovery: true,
			RecoverySteps:   7,
		},
		DegradationCritical: {
			StableMetricsDuration: 300 * time.Second,
			RequiredMetrics: map[string]interface{}{
				"cpu_usage":     40.0,
				"memory_usage":  0.60,
				"response_time": 50 * time.Millisecond,
			},
			GradualRecovery: true,
			RecoverySteps:   10,
		},
	}
}

// getDefaultDegradationRules returns default degradation rules
func getDefaultDegradationRules() []DegradationRule {
	return []DegradationRule{
		{
			ID:          "cpu-overload-light",
			Name:        "CPU Overload Light",
			Description: "Light degradation when CPU usage is high",
			Component:   "resource",
			Metric:      "cpu_usage",
			Condition:   ConditionGreaterThan,
			Threshold:   75.0,
			Level:       DegradationLight,
			Actions:     []string{"reduce_worker_threads", "increase_batch_timeout"},
			Duration:    30 * time.Second,
			Priority:    100,
			Enabled:     true,
			CreatedAt:   time.Now(),
			UpdatedAt:   time.Now(),
		},
		{
			ID:          "cpu-overload-severe",
			Name:        "CPU Overload Severe",
			Description: "Severe degradation when CPU usage is critical",
			Component:   "resource",
			Metric:      "cpu_usage",
			Condition:   ConditionGreaterThan,
			Threshold:   90.0,
			Level:       DegradationSevere,
			Actions:     []string{"reduce_worker_threads", "disable_non_critical_features", "reduce_connection_limits"},
			Duration:    60 * time.Second,
			Priority:    200,
			Enabled:     true,
			CreatedAt:   time.Now(),
			UpdatedAt:   time.Now(),
		},
		{
			ID:          "memory-pressure-moderate",
			Name:        "Memory Pressure Moderate",
			Description: "Moderate degradation when memory usage is high",
			Component:   "resource",
			Metric:      "memory_usage_percent",
			Condition:   ConditionGreaterThan,
			Threshold:   85.0,
			Level:       DegradationModerate,
			Actions:     []string{"clear_caches", "reduce_buffer_sizes", "limit_concurrent_operations"},
			Duration:    45 * time.Second,
			Priority:    150,
			Enabled:     true,
			CreatedAt:   time.Now(),
			UpdatedAt:   time.Now(),
		},
		{
			ID:          "memory-pressure-critical",
			Name:        "Memory Pressure Critical",
			Description: "Critical degradation when memory usage is extremely high",
			Component:   "resource",
			Metric:      "memory_usage_percent",
			Condition:   ConditionGreaterThan,
			Threshold:   95.0,
			Level:       DegradationCritical,
			Actions:     []string{"emergency_cache_clear", "disable_non_essential_services", "reduce_connection_limits"},
			Duration:    90 * time.Second,
			Priority:    300,
			Enabled:     true,
			CreatedAt:   time.Now(),
			UpdatedAt:   time.Now(),
		},
		{
			ID:          "bitswap-latency-high",
			Name:        "Bitswap Latency High",
			Description: "Light degradation when Bitswap latency is high",
			Component:   "bitswap",
			Metric:      "p95_response_time",
			Condition:   ConditionGreaterThan,
			Threshold:   200 * time.Millisecond,
			Level:       DegradationLight,
			Actions:     []string{"increase_batch_size", "prioritize_critical_requests"},
			Duration:    30 * time.Second,
			Priority:    120,
			Enabled:     true,
			CreatedAt:   time.Now(),
			UpdatedAt:   time.Now(),
		},
		{
			ID:          "network-congestion",
			Name:        "Network Congestion",
			Description: "Moderate degradation when network is congested",
			Component:   "network",
			Metric:      "packet_loss_rate",
			Condition:   ConditionGreaterThan,
			Threshold:   0.05, // 5% packet loss
			Level:       DegradationModerate,
			Actions:     []string{"reduce_connection_limits", "enable_compression", "prioritize_critical_traffic"},
			Duration:    60 * time.Second,
			Priority:    140,
			Enabled:     true,
			CreatedAt:   time.Now(),
			UpdatedAt:   time.Now(),
		},
	}
}

// Start begins monitoring and degradation management
func (gdm *gracefulDegradationManager) Start(ctx context.Context) error {
	gdm.mu.Lock()
	defer gdm.mu.Unlock()

	if gdm.isRunning {
		return fmt.Errorf("graceful degradation manager is already running")
	}

	gdm.isRunning = true

	go gdm.monitorLoop(ctx)

	gdm.logger.Info("Graceful degradation manager started",
		slog.Int("rules_count", len(gdm.rules)),
		slog.Int("actions_count", len(gdm.actions)),
		slog.Duration("monitor_interval", gdm.monitorInterval),
	)

	return nil
}

// Stop stops degradation management
func (gdm *gracefulDegradationManager) Stop() error {
	gdm.mu.Lock()

	if !gdm.isRunning {
		gdm.mu.Unlock()
		return nil
	}

	gdm.isRunning = false
	close(gdm.stopChan)
	gdm.stopChan = make(chan struct{})

	// Check if we need to recover
	needsRecovery := gdm.currentLevel != DegradationNone
	gdm.mu.Unlock()

	// Recover from any active degradation without holding the lock
	if needsRecovery {
		gdm.recoverToLevel(context.Background(), DegradationNone)
	}

	gdm.logger.Info("Graceful degradation manager stopped")
	return nil
}

// monitorLoop is the main monitoring loop
func (gdm *gracefulDegradationManager) monitorLoop(ctx context.Context) {
	ticker := time.NewTicker(gdm.monitorInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-gdm.stopChan:
			return
		case <-ticker.C:
			gdm.evaluateDegradation(ctx)
		}
	}
}

// evaluateDegradation evaluates current metrics and applies degradation if needed
func (gdm *gracefulDegradationManager) evaluateDegradation(ctx context.Context) {
	metrics := gdm.performanceMonitor.CollectMetrics()
	if metrics == nil {
		gdm.logger.Warn("Failed to collect metrics for degradation evaluation")
		return
	}

	// Evaluate recovery first
	if gdm.currentLevel != DegradationNone {
		if gdm.shouldRecover(metrics) {
			gdm.attemptRecovery(ctx, metrics)
			return
		}
	}

	// Evaluate degradation rules
	highestLevel := DegradationNone
	var triggeredRules []*DegradationRule

	// Sort rules by priority (highest first)
	sortedRules := make([]DegradationRule, len(gdm.rules))
	copy(sortedRules, gdm.rules)

	for i := 0; i < len(sortedRules)-1; i++ {
		for j := i + 1; j < len(sortedRules); j++ {
			if sortedRules[i].Priority < sortedRules[j].Priority {
				sortedRules[i], sortedRules[j] = sortedRules[j], sortedRules[i]
			}
		}
	}

	for _, rule := range sortedRules {
		if !rule.Enabled {
			continue
		}

		if gdm.evaluateRule(rule, metrics) {
			triggeredRules = append(triggeredRules, &rule)
			if rule.Level > highestLevel {
				highestLevel = rule.Level
			}
		}
	}

	// Apply degradation if needed
	if highestLevel > gdm.currentLevel {
		gdm.applyDegradation(ctx, highestLevel, triggeredRules, metrics)
	}
}

// evaluateRule evaluates a single degradation rule
func (gdm *gracefulDegradationManager) evaluateRule(rule DegradationRule, metrics *PerformanceMetrics) bool {
	value := gdm.extractMetricValue(rule.Component, rule.Metric, metrics)
	if value == nil {
		return false
	}

	return gdm.compareValues(value, rule.Condition, rule.Threshold)
}

// extractMetricValue extracts metric value from performance metrics
func (gdm *gracefulDegradationManager) extractMetricValue(component, metric string, metrics *PerformanceMetrics) interface{} {
	switch component {
	case "bitswap":
		switch metric {
		case "p95_response_time":
			return metrics.BitswapMetrics.P95ResponseTime
		case "average_response_time":
			return metrics.BitswapMetrics.AverageResponseTime
		case "queued_requests":
			return metrics.BitswapMetrics.QueuedRequests
		case "active_connections":
			return metrics.BitswapMetrics.ActiveConnections
		case "error_rate":
			return metrics.BitswapMetrics.ErrorRate
		}
	case "blockstore":
		switch metric {
		case "cache_hit_rate":
			return metrics.BlockstoreMetrics.CacheHitRate
		case "average_read_latency":
			return metrics.BlockstoreMetrics.AverageReadLatency
		case "average_write_latency":
			return metrics.BlockstoreMetrics.AverageWriteLatency
		}
	case "network":
		switch metric {
		case "active_connections":
			return metrics.NetworkMetrics.ActiveConnections
		case "average_latency":
			return metrics.NetworkMetrics.AverageLatency
		case "packet_loss_rate":
			return metrics.NetworkMetrics.PacketLossRate
		}
	case "resource":
		switch metric {
		case "cpu_usage":
			return metrics.ResourceMetrics.CPUUsage
		case "memory_usage":
			return metrics.ResourceMetrics.MemoryUsage
		case "memory_usage_percent":
			if metrics.ResourceMetrics.MemoryTotal > 0 {
				return float64(metrics.ResourceMetrics.MemoryUsage) / float64(metrics.ResourceMetrics.MemoryTotal) * 100
			}
		case "disk_usage":
			return metrics.ResourceMetrics.DiskUsage
		case "goroutine_count":
			return metrics.ResourceMetrics.GoroutineCount
		}
	}

	return nil
}

// compareValues compares values using the specified condition
func (gdm *gracefulDegradationManager) compareValues(value interface{}, condition AlertCondition, threshold interface{}) bool {
	switch condition {
	case ConditionGreaterThan:
		return gdm.compareNumeric(value, threshold, func(a, b float64) bool { return a > b })
	case ConditionLessThan:
		return gdm.compareNumeric(value, threshold, func(a, b float64) bool { return a < b })
	case ConditionGreaterOrEqual:
		return gdm.compareNumeric(value, threshold, func(a, b float64) bool { return a >= b })
	case ConditionLessOrEqual:
		return gdm.compareNumeric(value, threshold, func(a, b float64) bool { return a <= b })
	case ConditionEquals:
		return gdm.compareEqual(value, threshold)
	case ConditionNotEquals:
		return !gdm.compareEqual(value, threshold)
	}

	return false
}

// compareNumeric compares numeric values
func (gdm *gracefulDegradationManager) compareNumeric(value, threshold interface{}, compareFn func(float64, float64) bool) bool {
	v1, ok1 := gdm.toFloat64(value)
	v2, ok2 := gdm.toFloat64(threshold)

	if !ok1 || !ok2 {
		return false
	}

	return compareFn(v1, v2)
}

// compareEqual compares values for equality
func (gdm *gracefulDegradationManager) compareEqual(value, threshold interface{}) bool {
	if v1, ok1 := gdm.toFloat64(value); ok1 {
		if v2, ok2 := gdm.toFloat64(threshold); ok2 {
			return v1 == v2
		}
	}

	return fmt.Sprintf("%v", value) == fmt.Sprintf("%v", threshold)
}

// toFloat64 converts various numeric types to float64
func (gdm *gracefulDegradationManager) toFloat64(value interface{}) (float64, bool) {
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

// applyDegradation applies the specified degradation level
func (gdm *gracefulDegradationManager) applyDegradation(ctx context.Context, level DegradationLevel, rules []*DegradationRule, metrics *PerformanceMetrics) {
	gdm.mu.Lock()
	defer gdm.mu.Unlock()

	previousLevel := gdm.currentLevel
	gdm.currentLevel = level
	gdm.lastLevelChange = time.Now()

	if gdm.degradationStarted == nil {
		now := time.Now()
		gdm.degradationStarted = &now
	}

	// Collect all actions to execute
	actionsToExecute := make(map[string]bool)
	ruleIDs := make([]string, 0, len(rules))

	for _, rule := range rules {
		gdm.activeRules[rule.ID] = rule
		ruleIDs = append(ruleIDs, rule.ID)

		for _, actionName := range rule.Actions {
			actionsToExecute[actionName] = true
		}
	}

	// Execute degradation actions
	executedActions := make([]string, 0, len(actionsToExecute))
	for actionName := range actionsToExecute {
		if action, exists := gdm.actions[actionName]; exists {
			if err := action.Execute(ctx, level, metrics); err != nil {
				gdm.logger.Error("Failed to execute degradation action",
					slog.String("action", actionName),
					slog.String("level", level.String()),
					slog.String("error", err.Error()),
				)
			} else {
				gdm.activeActions[actionName] = action
				executedActions = append(executedActions, actionName)
			}
		}
	}

	// Record degradation event
	event := DegradationEvent{
		Timestamp: time.Now(),
		FromLevel: previousLevel,
		ToLevel:   level,
		Trigger:   fmt.Sprintf("rules: %v", ruleIDs),
		Actions:   executedActions,
		Metrics: map[string]interface{}{
			"cpu_usage":     metrics.ResourceMetrics.CPUUsage,
			"memory_usage":  metrics.ResourceMetrics.MemoryUsage,
			"response_time": metrics.BitswapMetrics.P95ResponseTime,
		},
	}

	gdm.addToHistory(event)

	gdm.logger.Warn("Degradation level changed",
		slog.String("from_level", previousLevel.String()),
		slog.String("to_level", level.String()),
		slog.Any("triggered_rules", ruleIDs),
		slog.Any("executed_actions", executedActions),
	)
}

// shouldRecover determines if recovery should be attempted
func (gdm *gracefulDegradationManager) shouldRecover(metrics *PerformanceMetrics) bool {
	threshold, exists := gdm.recoveryThresholds[gdm.currentLevel]
	if !exists {
		return false
	}

	// Check if metrics meet recovery requirements
	for metricName, requiredValue := range threshold.RequiredMetrics {
		currentValue := gdm.extractMetricValue("resource", metricName, metrics)
		if currentValue == nil {
			continue
		}

		// For recovery, we want metrics to be BELOW thresholds
		if !gdm.compareNumeric(currentValue, requiredValue, func(a, b float64) bool { return a <= b }) {
			return false
		}
	}

	// Check if metrics have been stable for required duration
	timeSinceLastChange := time.Since(gdm.lastLevelChange)
	return timeSinceLastChange >= threshold.StableMetricsDuration
}

// attemptRecovery attempts to recover from current degradation level
func (gdm *gracefulDegradationManager) attemptRecovery(ctx context.Context, metrics *PerformanceMetrics) {
	threshold := gdm.recoveryThresholds[gdm.currentLevel]

	var targetLevel DegradationLevel
	if threshold.GradualRecovery {
		// Gradual recovery - reduce by one level
		targetLevel = gdm.currentLevel - 1
		if targetLevel < DegradationNone {
			targetLevel = DegradationNone
		}
	} else {
		// Full recovery
		targetLevel = DegradationNone
	}

	gdm.recoverToLevel(ctx, targetLevel)

	// Call recovery callback if set
	if gdm.recoveryCallback != nil {
		eventType := RecoveryEventLevelReduced
		if targetLevel == DegradationNone {
			eventType = RecoveryEventFullRecovery
		}

		event := RecoveryEvent{
			Type:      eventType,
			FromLevel: gdm.currentLevel,
			ToLevel:   targetLevel,
			Timestamp: time.Now(),
			Duration:  time.Since(gdm.lastLevelChange),
			Metrics:   metrics,
		}

		go gdm.recoveryCallback(event)
	}
}

// recoverToLevel recovers to the specified degradation level
func (gdm *gracefulDegradationManager) recoverToLevel(ctx context.Context, targetLevel DegradationLevel) {
	gdm.mu.Lock()
	defer gdm.mu.Unlock()

	previousLevel := gdm.currentLevel
	gdm.currentLevel = targetLevel
	gdm.lastLevelChange = time.Now()

	if targetLevel == DegradationNone {
		gdm.degradationStarted = nil
		gdm.recoveryProgress = 1.0
	} else {
		gdm.recoveryProgress = float64(int(previousLevel)-int(targetLevel)) / float64(int(previousLevel))
	}

	// Recover from actions that are no longer needed
	actionsToRecover := make([]string, 0)
	for actionName, action := range gdm.activeActions {
		// For simplicity, recover all actions when moving to a lower level
		// In a more sophisticated implementation, you might keep some actions active
		if err := action.Recover(ctx); err != nil {
			gdm.logger.Error("Failed to recover from degradation action",
				slog.String("action", actionName),
				slog.String("error", err.Error()),
			)
		} else {
			actionsToRecover = append(actionsToRecover, actionName)
		}
	}

	// Clear recovered actions
	for _, actionName := range actionsToRecover {
		delete(gdm.activeActions, actionName)
	}

	// Clear active rules if fully recovered
	if targetLevel == DegradationNone {
		gdm.activeRules = make(map[string]*DegradationRule)
	}

	// Record recovery event
	event := DegradationEvent{
		Timestamp: time.Now(),
		FromLevel: previousLevel,
		ToLevel:   targetLevel,
		Trigger:   "recovery",
		Actions:   actionsToRecover,
		Metrics:   make(map[string]interface{}),
	}

	gdm.addToHistory(event)

	gdm.logger.Info("Recovery completed",
		slog.String("from_level", previousLevel.String()),
		slog.String("to_level", targetLevel.String()),
		slog.Any("recovered_actions", actionsToRecover),
		slog.Float64("recovery_progress", gdm.recoveryProgress),
	)
}

// addToHistory adds an event to the degradation history
func (gdm *gracefulDegradationManager) addToHistory(event DegradationEvent) {
	gdm.eventHistory = append(gdm.eventHistory, event)

	if len(gdm.eventHistory) > gdm.maxHistorySize {
		gdm.eventHistory = gdm.eventHistory[len(gdm.eventHistory)-gdm.maxHistorySize:]
	}
}

// SetDegradationRules configures degradation rules
func (gdm *gracefulDegradationManager) SetDegradationRules(rules []DegradationRule) error {
	gdm.mu.Lock()
	defer gdm.mu.Unlock()

	// Validate rules
	for _, rule := range rules {
		if rule.ID == "" {
			return fmt.Errorf("degradation rule ID cannot be empty")
		}
		if rule.Component == "" {
			return fmt.Errorf("degradation rule component cannot be empty for rule %s", rule.ID)
		}
		if rule.Metric == "" {
			return fmt.Errorf("degradation rule metric cannot be empty for rule %s", rule.ID)
		}
	}

	gdm.rules = rules

	gdm.logger.Info("Degradation rules updated",
		slog.Int("rules_count", len(rules)),
	)

	return nil
}

// GetCurrentDegradationLevel returns current degradation level
func (gdm *gracefulDegradationManager) GetCurrentDegradationLevel() DegradationLevel {
	gdm.mu.RLock()
	defer gdm.mu.RUnlock()
	return gdm.currentLevel
}

// GetDegradationStatus returns detailed degradation status
func (gdm *gracefulDegradationManager) GetDegradationStatus() DegradationStatus {
	gdm.mu.RLock()
	defer gdm.mu.RUnlock()

	activeRules := make([]string, 0, len(gdm.activeRules))
	for ruleID := range gdm.activeRules {
		activeRules = append(activeRules, ruleID)
	}

	activeActions := make([]string, 0, len(gdm.activeActions))
	for actionName := range gdm.activeActions {
		activeActions = append(activeActions, actionName)
	}

	// Copy recent history
	historySize := 10
	if len(gdm.eventHistory) < historySize {
		historySize = len(gdm.eventHistory)
	}

	recentHistory := make([]DegradationEvent, historySize)
	if historySize > 0 {
		copy(recentHistory, gdm.eventHistory[len(gdm.eventHistory)-historySize:])
	}

	return DegradationStatus{
		Level:              gdm.currentLevel,
		ActiveRules:        activeRules,
		ActiveActions:      activeActions,
		DegradationStarted: gdm.degradationStarted,
		LastLevelChange:    gdm.lastLevelChange,
		RecoveryProgress:   gdm.recoveryProgress,
		Metrics:            gdm.performanceMonitor.CollectMetrics(),
		History:            recentHistory,
	}
}

// ForceRecovery forces recovery to normal operation
func (gdm *gracefulDegradationManager) ForceRecovery() error {
	gdm.mu.Lock()

	if gdm.currentLevel == DegradationNone {
		gdm.mu.Unlock()
		return nil // Already at normal level
	}

	previousLevel := gdm.currentLevel
	gdm.mu.Unlock()

	gdm.recoverToLevel(context.Background(), DegradationNone)

	// Call recovery callback if set
	gdm.mu.RLock()
	callback := gdm.recoveryCallback
	lastChange := gdm.lastLevelChange
	gdm.mu.RUnlock()

	if callback != nil {
		event := RecoveryEvent{
			Type:      RecoveryEventForced,
			FromLevel: previousLevel,
			ToLevel:   DegradationNone,
			Timestamp: time.Now(),
			Duration:  time.Since(lastChange),
			Metrics:   gdm.performanceMonitor.CollectMetrics(),
		}

		go callback(event)
	}

	gdm.logger.Info("Forced recovery to normal operation",
		slog.String("from_level", previousLevel.String()),
	)

	return nil
}

// RegisterDegradationAction registers a custom degradation action
func (gdm *gracefulDegradationManager) RegisterDegradationAction(name string, action DegradationAction) error {
	gdm.mu.Lock()
	defer gdm.mu.Unlock()

	gdm.actions[name] = action

	gdm.logger.Info("Degradation action registered",
		slog.String("name", name),
		slog.String("action_type", action.GetName()),
	)

	return nil
}

// SetRecoveryCallback sets callback for recovery events
func (gdm *gracefulDegradationManager) SetRecoveryCallback(callback RecoveryCallback) error {
	gdm.mu.Lock()
	defer gdm.mu.Unlock()

	gdm.recoveryCallback = callback
	return nil
}

// registerDefaultActions registers default degradation actions
func (gdm *gracefulDegradationManager) registerDefaultActions() {
	// Register built-in degradation actions
	gdm.actions["reduce_worker_threads"] = &ReduceWorkerThreadsAction{}
	gdm.actions["increase_batch_timeout"] = &IncreaseBatchTimeoutAction{}
	gdm.actions["clear_caches"] = &ClearCachesAction{}
	gdm.actions["reduce_buffer_sizes"] = &ReduceBufferSizesAction{}
	gdm.actions["limit_concurrent_operations"] = &LimitConcurrentOperationsAction{}
	gdm.actions["disable_non_critical_features"] = &DisableNonCriticalFeaturesAction{}
	gdm.actions["reduce_connection_limits"] = &ReduceConnectionLimitsAction{}
	gdm.actions["emergency_cache_clear"] = &EmergencyCacheClearAction{}
	gdm.actions["disable_non_essential_services"] = &DisableNonEssentialServicesAction{}
	gdm.actions["increase_batch_size"] = &IncreaseBatchSizeAction{}
	gdm.actions["prioritize_critical_requests"] = &PrioritizeCriticalRequestsAction{}
	gdm.actions["enable_compression"] = &EnableCompressionAction{}
	gdm.actions["prioritize_critical_traffic"] = &PrioritizeCriticalTrafficAction{}
}
