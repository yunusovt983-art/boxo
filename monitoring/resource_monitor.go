package monitoring

import (
	"context"
	"fmt"
	"math"
	"sync"
	"time"
)

// ResourceMonitorThresholds defines warning and critical thresholds for resources
type ResourceMonitorThresholds struct {
	CPUWarning        float64 // CPU usage percentage (0-100)
	CPUCritical       float64
	MemoryWarning     float64 // Memory usage percentage (0-1)
	MemoryCritical    float64
	DiskWarning       float64 // Disk usage percentage (0-1)
	DiskCritical      float64
	GoroutineWarning  int // Number of goroutines
	GoroutineCritical int
}

// DefaultResourceMonitorThresholds returns sensible default thresholds
func DefaultResourceMonitorThresholds() ResourceMonitorThresholds {
	return ResourceMonitorThresholds{
		CPUWarning:        70.0,
		CPUCritical:       90.0,
		MemoryWarning:     0.8,  // 80%
		MemoryCritical:    0.95, // 95%
		DiskWarning:       0.85, // 85%
		DiskCritical:      0.95, // 95%
		GoroutineWarning:  10000,
		GoroutineCritical: 50000,
	}
}

// ResourceAlert represents a resource usage alert
type ResourceAlert struct {
	Type       string              `json:"type"`
	Level      string              `json:"level"` // "warning" or "critical"
	Message    string              `json:"message"`
	Value      float64             `json:"value"`
	Threshold  float64             `json:"threshold"`
	Timestamp  time.Time           `json:"timestamp"`
	Prediction *ResourcePrediction `json:"prediction,omitempty"`
}

// ResourcePrediction contains predictive analysis results
type ResourcePrediction struct {
	TrendDirection    string        `json:"trend_direction"` // "increasing", "decreasing", "stable"
	TimeToThreshold   time.Duration `json:"time_to_threshold"`
	Confidence        float64       `json:"confidence"` // 0-1
	RecommendedAction string        `json:"recommended_action"`
}

// ResourceHistory stores historical resource usage data
type ResourceHistory struct {
	Timestamps []time.Time `json:"timestamps"`
	Values     []float64   `json:"values"`
	MaxSize    int         `json:"max_size"`
}

// NewResourceHistory creates a new resource history tracker
func NewResourceHistory(maxSize int) *ResourceHistory {
	return &ResourceHistory{
		Timestamps: make([]time.Time, 0, maxSize),
		Values:     make([]float64, 0, maxSize),
		MaxSize:    maxSize,
	}
}

// Add adds a new data point to the history
func (rh *ResourceHistory) Add(timestamp time.Time, value float64) {
	rh.Timestamps = append(rh.Timestamps, timestamp)
	rh.Values = append(rh.Values, value)

	// Keep only the last MaxSize entries
	if len(rh.Values) > rh.MaxSize {
		rh.Timestamps = rh.Timestamps[1:]
		rh.Values = rh.Values[1:]
	}
}

// GetTrend calculates the trend direction and slope
func (rh *ResourceHistory) GetTrend() (direction string, slope float64) {
	if len(rh.Values) < 2 {
		return "stable", 0.0
	}

	// Simple linear regression to calculate trend
	n := float64(len(rh.Values))
	sumX, sumY, sumXY, sumX2 := 0.0, 0.0, 0.0, 0.0

	for i, value := range rh.Values {
		x := float64(i)
		sumX += x
		sumY += value
		sumXY += x * value
		sumX2 += x * x
	}

	slope = (n*sumXY - sumX*sumY) / (n*sumX2 - sumX*sumX)

	if math.Abs(slope) < 0.001 {
		direction = "stable"
	} else if slope > 0 {
		direction = "increasing"
	} else {
		direction = "decreasing"
	}

	return direction, slope
}

// ResourceMonitor provides comprehensive resource monitoring with predictive analysis
type ResourceMonitor struct {
	mu            sync.RWMutex
	collector     *ResourceCollector
	thresholds    ResourceMonitorThresholds
	alertCallback func(ResourceAlert)

	// Historical data for predictive analysis
	cpuHistory       *ResourceHistory
	memoryHistory    *ResourceHistory
	diskHistory      *ResourceHistory
	goroutineHistory *ResourceHistory

	// Monitoring state
	isRunning       bool
	stopChan        chan struct{}
	monitorInterval time.Duration

	// Alert state tracking
	lastAlerts    map[string]time.Time
	alertCooldown time.Duration
}

// NewResourceMonitor creates a new resource monitor
func NewResourceMonitor(collector *ResourceCollector, thresholds ResourceMonitorThresholds) *ResourceMonitor {
	return &ResourceMonitor{
		collector:        collector,
		thresholds:       thresholds,
		cpuHistory:       NewResourceHistory(100), // Keep last 100 data points
		memoryHistory:    NewResourceHistory(100),
		diskHistory:      NewResourceHistory(100),
		goroutineHistory: NewResourceHistory(100),
		lastAlerts:       make(map[string]time.Time),
		alertCooldown:    5 * time.Minute,  // Don't spam alerts
		monitorInterval:  30 * time.Second, // Monitor every 30 seconds
		stopChan:         make(chan struct{}),
	}
}

// SetAlertCallback sets the callback function for alerts
func (rm *ResourceMonitor) SetAlertCallback(callback func(ResourceAlert)) {
	rm.mu.Lock()
	defer rm.mu.Unlock()
	rm.alertCallback = callback
}

// SetMonitorInterval sets the monitoring interval
func (rm *ResourceMonitor) SetMonitorInterval(interval time.Duration) {
	rm.mu.Lock()
	defer rm.mu.Unlock()
	rm.monitorInterval = interval
}

// Start begins resource monitoring
func (rm *ResourceMonitor) Start(ctx context.Context) error {
	rm.mu.Lock()
	if rm.isRunning {
		rm.mu.Unlock()
		return fmt.Errorf("resource monitor is already running")
	}
	rm.isRunning = true
	rm.mu.Unlock()

	go rm.monitorLoop(ctx)
	return nil
}

// Stop stops resource monitoring
func (rm *ResourceMonitor) Stop() {
	rm.mu.Lock()
	defer rm.mu.Unlock()

	if rm.isRunning {
		close(rm.stopChan)
		rm.isRunning = false
	}
}

// monitorLoop is the main monitoring loop
func (rm *ResourceMonitor) monitorLoop(ctx context.Context) {
	ticker := time.NewTicker(rm.monitorInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-rm.stopChan:
			return
		case <-ticker.C:
			rm.collectAndAnalyze()
		}
	}
}

// collectAndAnalyze collects current metrics and performs analysis
func (rm *ResourceMonitor) collectAndAnalyze() {
	now := time.Now()

	// Collect current metrics
	metrics, err := rm.collector.CollectMetrics(context.Background())
	if err != nil {
		return
	}

	// Update historical data
	if cpuUsage, ok := metrics["cpu_usage"].(float64); ok {
		rm.cpuHistory.Add(now, cpuUsage)
		rm.checkThreshold("cpu", cpuUsage, rm.thresholds.CPUWarning, rm.thresholds.CPUCritical, "%")
	}

	memoryPressure := rm.collector.GetMemoryPressure()
	rm.memoryHistory.Add(now, memoryPressure)
	rm.checkThreshold("memory", memoryPressure*100, rm.thresholds.MemoryWarning*100, rm.thresholds.MemoryCritical*100, "%")

	diskPressure := rm.collector.GetDiskPressure()
	rm.diskHistory.Add(now, diskPressure)
	rm.checkThreshold("disk", diskPressure*100, rm.thresholds.DiskWarning*100, rm.thresholds.DiskCritical*100, "%")

	if goroutineCount, ok := metrics["goroutine_count"].(int); ok {
		rm.goroutineHistory.Add(now, float64(goroutineCount))
		rm.checkThreshold("goroutines", float64(goroutineCount), float64(rm.thresholds.GoroutineWarning), float64(rm.thresholds.GoroutineCritical), "")
	}
}

// checkThreshold checks if a metric exceeds thresholds and generates alerts
func (rm *ResourceMonitor) checkThreshold(metricType string, value, warningThreshold, criticalThreshold float64, unit string) {
	now := time.Now()

	// Check if we should suppress alerts due to cooldown
	if lastAlert, exists := rm.lastAlerts[metricType]; exists {
		if now.Sub(lastAlert) < rm.alertCooldown {
			return
		}
	}

	var alert *ResourceAlert

	if value >= criticalThreshold {
		alert = &ResourceAlert{
			Type:      metricType,
			Level:     "critical",
			Message:   fmt.Sprintf("%s usage is critical: %.2f%s (threshold: %.2f%s)", metricType, value, unit, criticalThreshold, unit),
			Value:     value,
			Threshold: criticalThreshold,
			Timestamp: now,
		}
	} else if value >= warningThreshold {
		alert = &ResourceAlert{
			Type:      metricType,
			Level:     "warning",
			Message:   fmt.Sprintf("%s usage is high: %.2f%s (threshold: %.2f%s)", metricType, value, unit, warningThreshold, unit),
			Value:     value,
			Threshold: warningThreshold,
			Timestamp: now,
		}
	}

	if alert != nil {
		// Add predictive analysis
		alert.Prediction = rm.getPrediction(metricType, value, criticalThreshold)

		// Send alert
		rm.mu.RLock()
		callback := rm.alertCallback
		rm.mu.RUnlock()

		if callback != nil {
			callback(*alert)
		}

		// Update last alert time
		rm.lastAlerts[metricType] = now
	}
}

// getPrediction generates predictive analysis for a metric
func (rm *ResourceMonitor) getPrediction(metricType string, currentValue, criticalThreshold float64) *ResourcePrediction {
	var history *ResourceHistory

	switch metricType {
	case "cpu":
		history = rm.cpuHistory
	case "memory":
		history = rm.memoryHistory
	case "disk":
		history = rm.diskHistory
	case "goroutines":
		history = rm.goroutineHistory
	default:
		return nil
	}

	direction, slope := history.GetTrend()

	prediction := &ResourcePrediction{
		TrendDirection: direction,
		Confidence:     rm.calculateConfidence(history),
	}

	// Calculate time to threshold if trend is increasing
	if direction == "increasing" && slope > 0 {
		timeToThreshold := time.Duration((criticalThreshold-currentValue)/slope) * rm.monitorInterval
		prediction.TimeToThreshold = timeToThreshold

		if timeToThreshold < time.Hour {
			prediction.RecommendedAction = "Immediate action required - critical threshold will be reached soon"
		} else if timeToThreshold < 24*time.Hour {
			prediction.RecommendedAction = "Monitor closely and prepare mitigation actions"
		} else {
			prediction.RecommendedAction = "Continue monitoring trend"
		}
	} else if direction == "decreasing" {
		prediction.RecommendedAction = "Resource usage is decreasing - continue monitoring"
	} else {
		prediction.RecommendedAction = "Resource usage is stable - maintain current monitoring"
	}

	return prediction
}

// calculateConfidence calculates confidence level for predictions
func (rm *ResourceMonitor) calculateConfidence(history *ResourceHistory) float64 {
	if len(history.Values) < 5 {
		return 0.3 // Low confidence with insufficient data
	}

	// Calculate variance to determine confidence
	mean := 0.0
	for _, value := range history.Values {
		mean += value
	}
	mean /= float64(len(history.Values))

	variance := 0.0
	for _, value := range history.Values {
		variance += math.Pow(value-mean, 2)
	}
	variance /= float64(len(history.Values))

	// Lower variance = higher confidence
	confidence := 1.0 / (1.0 + variance/100.0)
	if confidence > 1.0 {
		confidence = 1.0
	}

	return confidence
}

// GetCurrentStatus returns current resource status
func (rm *ResourceMonitor) GetCurrentStatus() map[string]interface{} {
	rm.mu.RLock()
	defer rm.mu.RUnlock()

	summary := rm.collector.GetResourceSummary()

	// Add trend information
	cpuDirection, cpuSlope := rm.cpuHistory.GetTrend()
	memoryDirection, memorySlope := rm.memoryHistory.GetTrend()
	diskDirection, diskSlope := rm.diskHistory.GetTrend()
	goroutineDirection, goroutineSlope := rm.goroutineHistory.GetTrend()

	summary["cpu_trend"] = map[string]interface{}{
		"direction": cpuDirection,
		"slope":     cpuSlope,
	}
	summary["memory_trend"] = map[string]interface{}{
		"direction": memoryDirection,
		"slope":     memorySlope,
	}
	summary["disk_trend"] = map[string]interface{}{
		"direction": diskDirection,
		"slope":     diskSlope,
	}
	summary["goroutine_trend"] = map[string]interface{}{
		"direction": goroutineDirection,
		"slope":     goroutineSlope,
	}

	return summary
}

// GetResourceHistory returns historical data for a specific metric
func (rm *ResourceMonitor) GetResourceHistory(metricType string) *ResourceHistory {
	rm.mu.RLock()
	defer rm.mu.RUnlock()

	switch metricType {
	case "cpu":
		return rm.cpuHistory
	case "memory":
		return rm.memoryHistory
	case "disk":
		return rm.diskHistory
	case "goroutines":
		return rm.goroutineHistory
	default:
		return nil
	}
}

// UpdateThresholds updates the resource thresholds
func (rm *ResourceMonitor) UpdateThresholds(thresholds ResourceMonitorThresholds) {
	rm.mu.Lock()
	defer rm.mu.Unlock()
	rm.thresholds = thresholds
}

// GetThresholds returns current thresholds
func (rm *ResourceMonitor) GetThresholds() ResourceMonitorThresholds {
	rm.mu.RLock()
	defer rm.mu.RUnlock()
	return rm.thresholds
}

// ForceCollection forces an immediate collection and analysis
func (rm *ResourceMonitor) ForceCollection() {
	rm.collectAndAnalyze()
}

// GetHealthStatus returns overall system health status
func (rm *ResourceMonitor) GetHealthStatus() map[string]interface{} {
	status := rm.GetCurrentStatus()

	cpuUsage := status["cpu_usage_percent"].(float64)
	memoryPressure := status["memory_pressure"].(float64) * 100
	diskPressure := status["disk_pressure"].(float64) * 100
	goroutines := status["goroutines"].(int)

	health := "healthy"
	issues := []string{}

	if cpuUsage >= rm.thresholds.CPUCritical {
		health = "critical"
		issues = append(issues, "CPU usage critical")
	} else if cpuUsage >= rm.thresholds.CPUWarning {
		if health == "healthy" {
			health = "warning"
		}
		issues = append(issues, "CPU usage high")
	}

	if memoryPressure >= rm.thresholds.MemoryCritical*100 {
		health = "critical"
		issues = append(issues, "Memory usage critical")
	} else if memoryPressure >= rm.thresholds.MemoryWarning*100 {
		if health == "healthy" {
			health = "warning"
		}
		issues = append(issues, "Memory usage high")
	}

	if diskPressure >= rm.thresholds.DiskCritical*100 {
		health = "critical"
		issues = append(issues, "Disk usage critical")
	} else if diskPressure >= rm.thresholds.DiskWarning*100 {
		if health == "healthy" {
			health = "warning"
		}
		issues = append(issues, "Disk usage high")
	}

	if goroutines >= rm.thresholds.GoroutineCritical {
		health = "critical"
		issues = append(issues, "Goroutine count critical")
	} else if goroutines >= rm.thresholds.GoroutineWarning {
		if health == "healthy" {
			health = "warning"
		}
		issues = append(issues, "Goroutine count high")
	}

	return map[string]interface{}{
		"health":    health,
		"issues":    issues,
		"timestamp": time.Now(),
	}
}
