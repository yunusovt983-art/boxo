package monitoring

import (
	"context"
	"fmt"
	"log/slog"
	"sync"
	"time"
)

// AlertManager defines the interface for managing performance alerts and notifications
type AlertManager interface {
	// StartMonitoring begins continuous monitoring and alert generation
	StartMonitoring(ctx context.Context, interval time.Duration) error
	
	// StopMonitoring stops alert monitoring
	StopMonitoring() error
	
	// RegisterNotifier adds a notification channel
	RegisterNotifier(name string, notifier AlertNotifier) error
	
	// UnregisterNotifier removes a notification channel
	UnregisterNotifier(name string) error
	
	// SetAlertRules configures alert rules
	SetAlertRules(rules []AlertRule) error
	
	// GetActiveAlerts returns currently active alerts
	GetActiveAlerts() []Alert
	
	// GetAlertHistory returns alert history
	GetAlertHistory(since time.Time) []Alert
	
	// AcknowledgeAlert acknowledges an alert
	AcknowledgeAlert(alertID string, acknowledgedBy string) error
	
	// ResolveAlert marks an alert as resolved
	ResolveAlert(alertID string, resolvedBy string) error
	
	// GetAlertStatistics returns alert statistics
	GetAlertStatistics() AlertStatistics
}

// AlertNotifier defines interface for alert notification channels
type AlertNotifier interface {
	SendAlert(ctx context.Context, alert Alert) error
	SendResolution(ctx context.Context, alert Alert) error
	GetName() string
	IsHealthy() bool
}

// Alert represents a performance alert
type Alert struct {
	ID              string                 `json:"id"`
	RuleID          string                 `json:"rule_id"`
	Title           string                 `json:"title"`
	Description     string                 `json:"description"`
	Severity        Severity               `json:"severity"`
	Component       string                 `json:"component"`
	Metric          string                 `json:"metric"`
	Value           interface{}            `json:"value"`
	Threshold       interface{}            `json:"threshold"`
	Labels          map[string]string      `json:"labels"`
	Annotations     map[string]string      `json:"annotations"`
	Context         map[string]interface{} `json:"context"`
	CreatedAt       time.Time              `json:"created_at"`
	UpdatedAt       time.Time              `json:"updated_at"`
	AcknowledgedAt  *time.Time             `json:"acknowledged_at,omitempty"`
	AcknowledgedBy  string                 `json:"acknowledged_by,omitempty"`
	ResolvedAt      *time.Time             `json:"resolved_at,omitempty"`
	ResolvedBy      string                 `json:"resolved_by,omitempty"`
	State           AlertState             `json:"state"`
	FireCount       int                    `json:"fire_count"`
	LastFiredAt     time.Time              `json:"last_fired_at"`
	TraceID         string                 `json:"trace_id,omitempty"`
	RequestID       string                 `json:"request_id,omitempty"`
}

// AlertRule defines conditions for triggering alerts
type AlertRule struct {
	ID          string                 `json:"id"`
	Name        string                 `json:"name"`
	Description string                 `json:"description"`
	Component   string                 `json:"component"`
	Metric      string                 `json:"metric"`
	Condition   AlertCondition         `json:"condition"`
	Threshold   interface{}            `json:"threshold"`
	Duration    time.Duration          `json:"duration"`
	Severity    Severity               `json:"severity"`
	Labels      map[string]string      `json:"labels"`
	Annotations map[string]string      `json:"annotations"`
	Enabled     bool                   `json:"enabled"`
	CreatedAt   time.Time              `json:"created_at"`
	UpdatedAt   time.Time              `json:"updated_at"`
}

// AlertCondition defines the type of condition for alert rules
type AlertCondition string

const (
	ConditionGreaterThan    AlertCondition = "gt"
	ConditionLessThan       AlertCondition = "lt"
	ConditionEquals         AlertCondition = "eq"
	ConditionNotEquals      AlertCondition = "ne"
	ConditionGreaterOrEqual AlertCondition = "gte"
	ConditionLessOrEqual    AlertCondition = "lte"
	ConditionContains       AlertCondition = "contains"
	ConditionRegex          AlertCondition = "regex"
)

// AlertState represents the current state of an alert
type AlertState string

const (
	AlertStateFiring      AlertState = "firing"
	AlertStateAcknowledged AlertState = "acknowledged"
	AlertStateResolved    AlertState = "resolved"
	AlertStateSuppressed  AlertState = "suppressed"
)

// AlertStatistics contains alert system statistics
type AlertStatistics struct {
	TotalAlerts       int                    `json:"total_alerts"`
	ActiveAlerts      int                    `json:"active_alerts"`
	AcknowledgedAlerts int                   `json:"acknowledged_alerts"`
	ResolvedAlerts    int                    `json:"resolved_alerts"`
	AlertsByComponent map[string]int         `json:"alerts_by_component"`
	AlertsBySeverity  map[Severity]int       `json:"alerts_by_severity"`
	MTTR              time.Duration          `json:"mttr"` // Mean Time To Resolution
	AlertFrequency    map[string]float64     `json:"alert_frequency"`
	LastUpdated       time.Time              `json:"last_updated"`
}

// RequestTracer defines interface for request tracing when SLA is exceeded
type RequestTracer interface {
	StartTrace(ctx context.Context, operation string) (TraceContext, error)
	EndTrace(traceCtx TraceContext, err error) error
	GetTrace(traceID string) (*TraceData, error)
	GetActiveTraces() []TraceData
}

// TraceContext contains tracing context information
type TraceContext struct {
	TraceID     string                 `json:"trace_id"`
	SpanID      string                 `json:"span_id"`
	Operation   string                 `json:"operation"`
	StartTime   time.Time              `json:"start_time"`
	Labels      map[string]string      `json:"labels"`
	Attributes  map[string]interface{} `json:"attributes"`
}

// TraceData contains complete trace information
type TraceData struct {
	TraceID     string                 `json:"trace_id"`
	Operation   string                 `json:"operation"`
	StartTime   time.Time              `json:"start_time"`
	EndTime     *time.Time             `json:"end_time,omitempty"`
	Duration    time.Duration          `json:"duration"`
	Success     bool                   `json:"success"`
	Error       string                 `json:"error,omitempty"`
	Spans       []SpanData             `json:"spans"`
	Labels      map[string]string      `json:"labels"`
	Attributes  map[string]interface{} `json:"attributes"`
}

// SpanData contains span information within a trace
type SpanData struct {
	SpanID      string                 `json:"span_id"`
	ParentID    string                 `json:"parent_id,omitempty"`
	Operation   string                 `json:"operation"`
	StartTime   time.Time              `json:"start_time"`
	EndTime     *time.Time             `json:"end_time,omitempty"`
	Duration    time.Duration          `json:"duration"`
	Success     bool                   `json:"success"`
	Error       string                 `json:"error,omitempty"`
	Attributes  map[string]interface{} `json:"attributes"`
}

// alertManager implements the AlertManager interface
type alertManager struct {
	mu                sync.RWMutex
	performanceMonitor PerformanceMonitor
	bottleneckAnalyzer BottleneckAnalyzer
	requestTracer      RequestTracer
	logger            *slog.Logger
	
	// Alert management
	rules         []AlertRule
	activeAlerts  map[string]*Alert
	alertHistory  []Alert
	maxHistory    int
	
	// Notification
	notifiers     map[string]AlertNotifier
	
	// Monitoring control
	isMonitoring  bool
	stopChan      chan struct{}
	
	// SLA thresholds for request tracing
	slaThresholds SLAThresholds
}

// SLAThresholds defines SLA thresholds for different operations
type SLAThresholds struct {
	BitswapRequestSLA  time.Duration `json:"bitswap_request_sla"`
	BlockstoreReadSLA  time.Duration `json:"blockstore_read_sla"`
	BlockstoreWriteSLA time.Duration `json:"blockstore_write_sla"`
	NetworkLatencySLA  time.Duration `json:"network_latency_sla"`
}

// NewAlertManager creates a new alert manager instance
func NewAlertManager(
	performanceMonitor PerformanceMonitor,
	bottleneckAnalyzer BottleneckAnalyzer,
	requestTracer RequestTracer,
	logger *slog.Logger,
) AlertManager {
	if logger == nil {
		logger = slog.Default()
	}
	
	am := &alertManager{
		performanceMonitor: performanceMonitor,
		bottleneckAnalyzer: bottleneckAnalyzer,
		requestTracer:      requestTracer,
		logger:            logger,
		rules:             getDefaultAlertRules(),
		activeAlerts:      make(map[string]*Alert),
		alertHistory:      make([]Alert, 0),
		maxHistory:        10000,
		notifiers:         make(map[string]AlertNotifier),
		stopChan:          make(chan struct{}),
		slaThresholds:     getDefaultSLAThresholds(),
	}
	
	return am
}

// getDefaultAlertRules returns default alert rules
func getDefaultAlertRules() []AlertRule {
	return []AlertRule{
		{
			ID:          "bitswap-high-latency",
			Name:        "Bitswap High Latency",
			Description: "Bitswap P95 response time exceeds threshold",
			Component:   "bitswap",
			Metric:      "p95_response_time",
			Condition:   ConditionGreaterThan,
			Threshold:   100 * time.Millisecond,
			Duration:    30 * time.Second,
			Severity:    SeverityHigh,
			Labels: map[string]string{
				"component": "bitswap",
				"type":      "latency",
			},
			Annotations: map[string]string{
				"summary":     "High Bitswap latency detected",
				"description": "Bitswap P95 response time is {{ .Value }} which exceeds threshold of {{ .Threshold }}",
				"runbook":     "https://docs.example.com/runbooks/bitswap-latency",
			},
			Enabled:   true,
			CreatedAt: time.Now(),
			UpdatedAt: time.Now(),
		},
		{
			ID:          "bitswap-queue-depth",
			Name:        "Bitswap Queue Depth High",
			Description: "Bitswap queue depth exceeds threshold",
			Component:   "bitswap",
			Metric:      "queued_requests",
			Condition:   ConditionGreaterThan,
			Threshold:   int64(1000),
			Duration:    60 * time.Second,
			Severity:    SeverityMedium,
			Labels: map[string]string{
				"component": "bitswap",
				"type":      "queue",
			},
			Annotations: map[string]string{
				"summary":     "High Bitswap queue depth",
				"description": "Bitswap queue depth is {{ .Value }} which exceeds threshold of {{ .Threshold }}",
			},
			Enabled:   true,
			CreatedAt: time.Now(),
			UpdatedAt: time.Now(),
		},
		{
			ID:          "blockstore-cache-hit-rate-low",
			Name:        "Blockstore Cache Hit Rate Low",
			Description: "Blockstore cache hit rate below threshold",
			Component:   "blockstore",
			Metric:      "cache_hit_rate",
			Condition:   ConditionLessThan,
			Threshold:   0.80,
			Duration:    120 * time.Second,
			Severity:    SeverityMedium,
			Labels: map[string]string{
				"component": "blockstore",
				"type":      "cache",
			},
			Annotations: map[string]string{
				"summary":     "Low blockstore cache hit rate",
				"description": "Blockstore cache hit rate is {{ .Value }} which is below threshold of {{ .Threshold }}",
			},
			Enabled:   true,
			CreatedAt: time.Now(),
			UpdatedAt: time.Now(),
		},
		{
			ID:          "resource-cpu-high",
			Name:        "High CPU Usage",
			Description: "CPU usage exceeds threshold",
			Component:   "resource",
			Metric:      "cpu_usage",
			Condition:   ConditionGreaterThan,
			Threshold:   80.0,
			Duration:    180 * time.Second,
			Severity:    SeverityHigh,
			Labels: map[string]string{
				"component": "resource",
				"type":      "cpu",
			},
			Annotations: map[string]string{
				"summary":     "High CPU usage detected",
				"description": "CPU usage is {{ .Value }}% which exceeds threshold of {{ .Threshold }}%",
			},
			Enabled:   true,
			CreatedAt: time.Now(),
			UpdatedAt: time.Now(),
		},
		{
			ID:          "resource-memory-high",
			Name:        "High Memory Usage",
			Description: "Memory usage exceeds threshold",
			Component:   "resource",
			Metric:      "memory_usage_percent",
			Condition:   ConditionGreaterThan,
			Threshold:   85.0,
			Duration:    120 * time.Second,
			Severity:    SeverityCritical,
			Labels: map[string]string{
				"component": "resource",
				"type":      "memory",
			},
			Annotations: map[string]string{
				"summary":     "High memory usage detected",
				"description": "Memory usage is {{ .Value }}% which exceeds threshold of {{ .Threshold }}%",
			},
			Enabled:   true,
			CreatedAt: time.Now(),
			UpdatedAt: time.Now(),
		},
	}
}

// getDefaultSLAThresholds returns default SLA thresholds
func getDefaultSLAThresholds() SLAThresholds {
	return SLAThresholds{
		BitswapRequestSLA:  100 * time.Millisecond,
		BlockstoreReadSLA:  50 * time.Millisecond,
		BlockstoreWriteSLA: 100 * time.Millisecond,
		NetworkLatencySLA:  200 * time.Millisecond,
	}
}

// StartMonitoring begins continuous monitoring and alert generation
func (am *alertManager) StartMonitoring(ctx context.Context, interval time.Duration) error {
	am.mu.Lock()
	defer am.mu.Unlock()
	
	if am.isMonitoring {
		return nil // Already monitoring
	}
	
	am.isMonitoring = true
	
	go func() {
		ticker := time.NewTicker(interval)
		defer ticker.Stop()
		
		am.logger.Info("Alert manager started monitoring",
			slog.Duration("interval", interval),
			slog.Int("rules_count", len(am.rules)),
		)
		
		for {
			select {
			case <-ctx.Done():
				am.logger.Info("Alert manager stopped due to context cancellation")
				return
			case <-am.stopChan:
				am.logger.Info("Alert manager stopped")
				return
			case <-ticker.C:
				if err := am.checkAlerts(ctx); err != nil {
					am.logger.Error("Error checking alerts",
						slog.String("error", err.Error()),
					)
				}
			}
		}
	}()
	
	return nil
}

// StopMonitoring stops alert monitoring
func (am *alertManager) StopMonitoring() error {
	am.mu.Lock()
	defer am.mu.Unlock()
	
	if !am.isMonitoring {
		return nil // Not monitoring
	}
	
	am.isMonitoring = false
	close(am.stopChan)
	am.stopChan = make(chan struct{})
	
	am.logger.Info("Alert manager monitoring stopped")
	return nil
}

// checkAlerts evaluates all alert rules against current metrics
func (am *alertManager) checkAlerts(ctx context.Context) error {
	// Collect current metrics
	metrics := am.performanceMonitor.CollectMetrics()
	if metrics == nil {
		return fmt.Errorf("failed to collect metrics")
	}
	
	// Check for SLA violations and start tracing if needed
	am.checkSLAViolations(ctx, metrics)
	
	// Evaluate each alert rule
	for _, rule := range am.rules {
		if !rule.Enabled {
			continue
		}
		
		if shouldFire, value := am.evaluateRule(rule, metrics); shouldFire {
			am.handleAlertFiring(ctx, rule, value, metrics)
		} else {
			am.handleAlertResolution(ctx, rule.ID)
		}
	}
	
	return nil
}

// checkSLAViolations checks for SLA violations and starts request tracing
func (am *alertManager) checkSLAViolations(ctx context.Context, metrics *PerformanceMetrics) {
	// Check Bitswap SLA
	if metrics.BitswapMetrics.P95ResponseTime > am.slaThresholds.BitswapRequestSLA {
		if am.requestTracer != nil {
			traceCtx, err := am.requestTracer.StartTrace(ctx, "bitswap_sla_violation")
			if err == nil {
				am.logger.Warn("SLA violation detected, starting trace",
					slog.String("component", "bitswap"),
					slog.Duration("response_time", metrics.BitswapMetrics.P95ResponseTime),
					slog.Duration("sla_threshold", am.slaThresholds.BitswapRequestSLA),
					slog.String("trace_id", traceCtx.TraceID),
				)
			}
		}
	}
	
	// Check Blockstore SLA
	if metrics.BlockstoreMetrics.AverageReadLatency > am.slaThresholds.BlockstoreReadSLA {
		if am.requestTracer != nil {
			traceCtx, err := am.requestTracer.StartTrace(ctx, "blockstore_read_sla_violation")
			if err == nil {
				am.logger.Warn("SLA violation detected, starting trace",
					slog.String("component", "blockstore"),
					slog.String("operation", "read"),
					slog.Duration("latency", metrics.BlockstoreMetrics.AverageReadLatency),
					slog.Duration("sla_threshold", am.slaThresholds.BlockstoreReadSLA),
					slog.String("trace_id", traceCtx.TraceID),
				)
			}
		}
	}
	
	// Check Network SLA
	if metrics.NetworkMetrics.AverageLatency > am.slaThresholds.NetworkLatencySLA {
		if am.requestTracer != nil {
			traceCtx, err := am.requestTracer.StartTrace(ctx, "network_sla_violation")
			if err == nil {
				am.logger.Warn("SLA violation detected, starting trace",
					slog.String("component", "network"),
					slog.Duration("latency", metrics.NetworkMetrics.AverageLatency),
					slog.Duration("sla_threshold", am.slaThresholds.NetworkLatencySLA),
					slog.String("trace_id", traceCtx.TraceID),
				)
			}
		}
	}
}

// evaluateRule evaluates a single alert rule against metrics
func (am *alertManager) evaluateRule(rule AlertRule, metrics *PerformanceMetrics) (bool, interface{}) {
	value := am.extractMetricValue(rule.Component, rule.Metric, metrics)
	if value == nil {
		return false, nil
	}
	
	return am.compareValues(value, rule.Condition, rule.Threshold), value
}

// extractMetricValue extracts the specified metric value from performance metrics
func (am *alertManager) extractMetricValue(component, metric string, metrics *PerformanceMetrics) interface{} {
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
		case "requests_per_second":
			return metrics.BitswapMetrics.RequestsPerSecond
		}
	case "blockstore":
		switch metric {
		case "cache_hit_rate":
			return metrics.BlockstoreMetrics.CacheHitRate
		case "average_read_latency":
			return metrics.BlockstoreMetrics.AverageReadLatency
		case "average_write_latency":
			return metrics.BlockstoreMetrics.AverageWriteLatency
		case "batch_operations_per_sec":
			return metrics.BlockstoreMetrics.BatchOperationsPerSec
		}
	case "network":
		switch metric {
		case "active_connections":
			return metrics.NetworkMetrics.ActiveConnections
		case "average_latency":
			return metrics.NetworkMetrics.AverageLatency
		case "total_bandwidth":
			return metrics.NetworkMetrics.TotalBandwidth
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

// compareValues compares two values based on the condition
func (am *alertManager) compareValues(value interface{}, condition AlertCondition, threshold interface{}) bool {
	switch condition {
	case ConditionGreaterThan:
		return am.compareNumeric(value, threshold, func(a, b float64) bool { return a > b })
	case ConditionLessThan:
		return am.compareNumeric(value, threshold, func(a, b float64) bool { return a < b })
	case ConditionGreaterOrEqual:
		return am.compareNumeric(value, threshold, func(a, b float64) bool { return a >= b })
	case ConditionLessOrEqual:
		return am.compareNumeric(value, threshold, func(a, b float64) bool { return a <= b })
	case ConditionEquals:
		return am.compareEqual(value, threshold)
	case ConditionNotEquals:
		return !am.compareEqual(value, threshold)
	}
	
	return false
}

// compareNumeric compares numeric values
func (am *alertManager) compareNumeric(value, threshold interface{}, compareFn func(float64, float64) bool) bool {
	v1, ok1 := am.toFloat64(value)
	v2, ok2 := am.toFloat64(threshold)
	
	if !ok1 || !ok2 {
		return false
	}
	
	return compareFn(v1, v2)
}

// compareEqual compares values for equality
func (am *alertManager) compareEqual(value, threshold interface{}) bool {
	// Try numeric comparison first
	if v1, ok1 := am.toFloat64(value); ok1 {
		if v2, ok2 := am.toFloat64(threshold); ok2 {
			return v1 == v2
		}
	}
	
	// Fall back to string comparison
	return fmt.Sprintf("%v", value) == fmt.Sprintf("%v", threshold)
}

// toFloat64 converts various numeric types to float64
func (am *alertManager) toFloat64(value interface{}) (float64, bool) {
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

// handleAlertFiring handles when an alert rule fires
func (am *alertManager) handleAlertFiring(ctx context.Context, rule AlertRule, value interface{}, metrics *PerformanceMetrics) {
	am.mu.Lock()
	defer am.mu.Unlock()
	
	alertID := rule.ID
	existingAlert, exists := am.activeAlerts[alertID]
	
	if exists {
		// Update existing alert
		existingAlert.FireCount++
		existingAlert.LastFiredAt = time.Now()
		existingAlert.UpdatedAt = time.Now()
		existingAlert.Value = value
		
		am.logger.Debug("Alert updated",
			slog.String("alert_id", alertID),
			slog.Int("fire_count", existingAlert.FireCount),
		)
	} else {
		// Create new alert
		alert := &Alert{
			ID:          alertID,
			RuleID:      rule.ID,
			Title:       rule.Name,
			Description: rule.Description,
			Severity:    rule.Severity,
			Component:   rule.Component,
			Metric:      rule.Metric,
			Value:       value,
			Threshold:   rule.Threshold,
			Labels:      copyMap(rule.Labels),
			Annotations: copyMap(rule.Annotations),
			Context: map[string]interface{}{
				"metrics_timestamp": metrics.Timestamp,
				"rule_duration":     rule.Duration,
			},
			CreatedAt:   time.Now(),
			UpdatedAt:   time.Now(),
			State:       AlertStateFiring,
			FireCount:   1,
			LastFiredAt: time.Now(),
		}
		
		// Add trace ID if available
		if am.requestTracer != nil {
			activeTraces := am.requestTracer.GetActiveTraces()
			if len(activeTraces) > 0 {
				alert.TraceID = activeTraces[0].TraceID
			}
		}
		
		am.activeAlerts[alertID] = alert
		am.addToHistory(*alert)
		
		am.logger.Warn("New alert fired",
			slog.String("alert_id", alertID),
			slog.String("title", alert.Title),
			slog.String("severity", string(alert.Severity)),
			slog.String("component", alert.Component),
			slog.Any("value", value),
			slog.Any("threshold", rule.Threshold),
		)
		
		// Send notifications
		am.sendNotifications(ctx, *alert)
	}
}

// handleAlertResolution handles when an alert condition is no longer met
func (am *alertManager) handleAlertResolution(ctx context.Context, alertID string) {
	am.mu.Lock()
	defer am.mu.Unlock()
	
	alert, exists := am.activeAlerts[alertID]
	if !exists || alert.State == AlertStateResolved {
		return
	}
	
	// Auto-resolve the alert
	now := time.Now()
	alert.ResolvedAt = &now
	alert.ResolvedBy = "system"
	alert.State = AlertStateResolved
	alert.UpdatedAt = now
	
	am.logger.Info("Alert auto-resolved",
		slog.String("alert_id", alertID),
		slog.String("title", alert.Title),
	)
	
	// Send resolution notifications
	am.sendResolutionNotifications(ctx, *alert)
	
	// Remove from active alerts
	delete(am.activeAlerts, alertID)
	
	// Update history
	am.addToHistory(*alert)
}

// sendNotifications sends alert notifications to all registered notifiers
func (am *alertManager) sendNotifications(ctx context.Context, alert Alert) {
	for name, notifier := range am.notifiers {
		if !notifier.IsHealthy() {
			am.logger.Warn("Skipping unhealthy notifier",
				slog.String("notifier", name),
			)
			continue
		}
		
		go func(n AlertNotifier, a Alert) {
			if err := n.SendAlert(ctx, a); err != nil {
				am.logger.Error("Failed to send alert notification",
					slog.String("notifier", n.GetName()),
					slog.String("alert_id", a.ID),
					slog.String("error", err.Error()),
				)
			}
		}(notifier, alert)
	}
}

// sendResolutionNotifications sends alert resolution notifications
func (am *alertManager) sendResolutionNotifications(ctx context.Context, alert Alert) {
	for name, notifier := range am.notifiers {
		if !notifier.IsHealthy() {
			am.logger.Warn("Skipping unhealthy notifier for resolution",
				slog.String("notifier", name),
			)
			continue
		}
		
		go func(n AlertNotifier, a Alert) {
			if err := n.SendResolution(ctx, a); err != nil {
				am.logger.Error("Failed to send resolution notification",
					slog.String("notifier", n.GetName()),
					slog.String("alert_id", a.ID),
					slog.String("error", err.Error()),
				)
			}
		}(notifier, alert)
	}
}

// addToHistory adds an alert to the history
func (am *alertManager) addToHistory(alert Alert) {
	am.alertHistory = append(am.alertHistory, alert)
	
	if len(am.alertHistory) > am.maxHistory {
		am.alertHistory = am.alertHistory[len(am.alertHistory)-am.maxHistory:]
	}
}

// copyMap creates a copy of a string map
func copyMap(original map[string]string) map[string]string {
	if original == nil {
		return nil
	}
	
	copy := make(map[string]string, len(original))
	for k, v := range original {
		copy[k] = v
	}
	return copy
}

// RegisterNotifier adds a notification channel
func (am *alertManager) RegisterNotifier(name string, notifier AlertNotifier) error {
	am.mu.Lock()
	defer am.mu.Unlock()
	
	am.notifiers[name] = notifier
	
	am.logger.Info("Alert notifier registered",
		slog.String("name", name),
		slog.String("type", notifier.GetName()),
	)
	
	return nil
}

// UnregisterNotifier removes a notification channel
func (am *alertManager) UnregisterNotifier(name string) error {
	am.mu.Lock()
	defer am.mu.Unlock()
	
	delete(am.notifiers, name)
	
	am.logger.Info("Alert notifier unregistered",
		slog.String("name", name),
	)
	
	return nil
}

// SetAlertRules configures alert rules
func (am *alertManager) SetAlertRules(rules []AlertRule) error {
	am.mu.Lock()
	defer am.mu.Unlock()
	
	// Validate rules
	for _, rule := range rules {
		if rule.ID == "" {
			return fmt.Errorf("alert rule ID cannot be empty")
		}
		if rule.Component == "" {
			return fmt.Errorf("alert rule component cannot be empty for rule %s", rule.ID)
		}
		if rule.Metric == "" {
			return fmt.Errorf("alert rule metric cannot be empty for rule %s", rule.ID)
		}
	}
	
	am.rules = rules
	
	am.logger.Info("Alert rules updated",
		slog.Int("rules_count", len(rules)),
	)
	
	return nil
}

// GetActiveAlerts returns currently active alerts
func (am *alertManager) GetActiveAlerts() []Alert {
	am.mu.RLock()
	defer am.mu.RUnlock()
	
	alerts := make([]Alert, 0, len(am.activeAlerts))
	for _, alert := range am.activeAlerts {
		alerts = append(alerts, *alert)
	}
	
	return alerts
}

// GetAlertHistory returns alert history
func (am *alertManager) GetAlertHistory(since time.Time) []Alert {
	am.mu.RLock()
	defer am.mu.RUnlock()
	
	var filteredHistory []Alert
	for _, alert := range am.alertHistory {
		if alert.CreatedAt.After(since) {
			filteredHistory = append(filteredHistory, alert)
		}
	}
	
	return filteredHistory
}

// AcknowledgeAlert acknowledges an alert
func (am *alertManager) AcknowledgeAlert(alertID string, acknowledgedBy string) error {
	am.mu.Lock()
	defer am.mu.Unlock()
	
	alert, exists := am.activeAlerts[alertID]
	if !exists {
		return fmt.Errorf("alert %s not found", alertID)
	}
	
	if alert.State == AlertStateResolved {
		return fmt.Errorf("cannot acknowledge resolved alert %s", alertID)
	}
	
	now := time.Now()
	alert.AcknowledgedAt = &now
	alert.AcknowledgedBy = acknowledgedBy
	alert.State = AlertStateAcknowledged
	alert.UpdatedAt = now
	
	am.logger.Info("Alert acknowledged",
		slog.String("alert_id", alertID),
		slog.String("acknowledged_by", acknowledgedBy),
	)
	
	return nil
}

// ResolveAlert marks an alert as resolved
func (am *alertManager) ResolveAlert(alertID string, resolvedBy string) error {
	am.mu.Lock()
	defer am.mu.Unlock()
	
	alert, exists := am.activeAlerts[alertID]
	if !exists {
		return fmt.Errorf("alert %s not found", alertID)
	}
	
	if alert.State == AlertStateResolved {
		return fmt.Errorf("alert %s is already resolved", alertID)
	}
	
	now := time.Now()
	alert.ResolvedAt = &now
	alert.ResolvedBy = resolvedBy
	alert.State = AlertStateResolved
	alert.UpdatedAt = now
	
	am.logger.Info("Alert manually resolved",
		slog.String("alert_id", alertID),
		slog.String("resolved_by", resolvedBy),
	)
	
	// Remove from active alerts
	delete(am.activeAlerts, alertID)
	
	// Update history
	am.addToHistory(*alert)
	
	return nil
}

// GetAlertStatistics returns alert statistics
func (am *alertManager) GetAlertStatistics() AlertStatistics {
	am.mu.RLock()
	defer am.mu.RUnlock()
	
	stats := AlertStatistics{
		TotalAlerts:       len(am.alertHistory),
		ActiveAlerts:      len(am.activeAlerts),
		AlertsByComponent: make(map[string]int),
		AlertsBySeverity:  make(map[Severity]int),
		AlertFrequency:    make(map[string]float64),
		LastUpdated:       time.Now(),
	}
	
	// Count acknowledged and resolved alerts
	for _, alert := range am.alertHistory {
		if alert.AcknowledgedAt != nil {
			stats.AcknowledgedAlerts++
		}
		if alert.ResolvedAt != nil {
			stats.ResolvedAlerts++
		}
		
		stats.AlertsByComponent[alert.Component]++
		stats.AlertsBySeverity[alert.Severity]++
	}
	
	// Calculate MTTR (Mean Time To Resolution)
	var totalResolutionTime time.Duration
	var resolvedCount int
	
	for _, alert := range am.alertHistory {
		if alert.ResolvedAt != nil {
			resolutionTime := alert.ResolvedAt.Sub(alert.CreatedAt)
			totalResolutionTime += resolutionTime
			resolvedCount++
		}
	}
	
	if resolvedCount > 0 {
		stats.MTTR = totalResolutionTime / time.Duration(resolvedCount)
	}
	
	return stats
}