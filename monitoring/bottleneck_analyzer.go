package monitoring

import (
	"context"
	"fmt"
	"math"
	"sort"
	"sync"
	"time"
)

// BottleneckAnalyzer defines the interface for automatic performance bottleneck detection
type BottleneckAnalyzer interface {
	AnalyzeBottlenecks(ctx context.Context, metrics *PerformanceMetrics) ([]Bottleneck, error)
	AnalyzeTrends(ctx context.Context, history []PerformanceMetrics) ([]TrendAnalysis, error)
	DetectAnomalies(ctx context.Context, metrics *PerformanceMetrics, baseline *MetricBaseline) ([]Anomaly, error)
	GenerateRecommendations(ctx context.Context, bottlenecks []Bottleneck) ([]OptimizationRecommendation, error)
	UpdateBaseline(metrics *PerformanceMetrics) error
	GetBaseline() *MetricBaseline
	SetThresholds(thresholds PerformanceThresholds) error
}

// Bottleneck represents a detected performance bottleneck
type Bottleneck struct {
	ID          string                 `json:"id"`
	Component   string                 `json:"component"`
	Type        BottleneckType         `json:"type"`
	Severity    Severity               `json:"severity"`
	Description string                 `json:"description"`
	Metrics     map[string]interface{} `json:"metrics"`
	DetectedAt  time.Time              `json:"detected_at"`
	Impact      ImpactAssessment       `json:"impact"`
}

// TrendAnalysis represents trend analysis results
type TrendAnalysis struct {
	Metric    string        `json:"metric"`
	Component string        `json:"component"`
	Trend     TrendType     `json:"trend"`
	Slope     float64       `json:"slope"`
	R2        float64       `json:"r_squared"`
	Period    time.Duration `json:"period"`
	Forecast  []float64     `json:"forecast"`
}

// Anomaly represents detected anomalous behavior
type Anomaly struct {
	ID          string                 `json:"id"`
	Metric      string                 `json:"metric"`
	Component   string                 `json:"component"`
	Type        AnomalyType            `json:"type"`
	Severity    Severity               `json:"severity"`
	Value       float64                `json:"value"`
	Expected    float64                `json:"expected"`
	Deviation   float64                `json:"deviation"`
	DetectedAt  time.Time              `json:"detected_at"`
	Context     map[string]interface{} `json:"context"`
}

// OptimizationRecommendation provides actionable optimization advice
type OptimizationRecommendation struct {
	ID          string                 `json:"id"`
	Category    RecommendationCategory `json:"category"`
	Priority    Priority               `json:"priority"`
	Title       string                 `json:"title"`
	Description string                 `json:"description"`
	Actions     []RecommendedAction    `json:"actions"`
	Impact      ExpectedImpact         `json:"expected_impact"`
	Effort      ImplementationEffort   `json:"implementation_effort"`
}

// RecommendedAction represents a specific action to take
type RecommendedAction struct {
	Type        ActionType             `json:"type"`
	Description string                 `json:"description"`
	Parameters  map[string]interface{} `json:"parameters"`
	Validation  string                 `json:"validation"`
}

// ComponentBaseline represents baseline metrics for a component
type ComponentBaseline struct {
	Mean        map[string]float64         `json:"mean"`
	StdDev      map[string]float64         `json:"std_dev"`
	Min         map[string]float64         `json:"min"`
	Max         map[string]float64         `json:"max"`
	Percentiles map[string]map[int]float64 `json:"percentiles"`
}

// MetricBaseline contains baseline metrics for anomaly detection
type MetricBaseline struct {
	UpdatedAt time.Time                      `json:"updated_at"`
	Bitswap   BitswapBaseline               `json:"bitswap"`
	Blockstore BlockstoreBaseline           `json:"blockstore"`
	Network   NetworkBaseline               `json:"network"`
	Resource  ResourceBaseline              `json:"resource"`
	Custom    map[string]ComponentBaseline  `json:"custom"`
}

// StatisticalBaseline contains statistical measures for a metric
type StatisticalBaseline struct {
	Mean       float64            `json:"mean"`
	StdDev     float64            `json:"std_dev"`
	Min        float64            `json:"min"`
	Max        float64            `json:"max"`
	Percentiles map[int]float64   `json:"percentiles"`
	SampleCount int               `json:"sample_count"`
	LastUpdated time.Time         `json:"last_updated"`
}

// Baseline types for each component
type BitswapBaseline struct {
	ResponseTime      StatisticalBaseline `json:"response_time"`
	RequestsPerSecond StatisticalBaseline `json:"requests_per_second"`
	ErrorRate         StatisticalBaseline `json:"error_rate"`
	QueuedRequests    StatisticalBaseline `json:"queued_requests"`
	ActiveConnections StatisticalBaseline `json:"active_connections"`
}

type BlockstoreBaseline struct {
	CacheHitRate          StatisticalBaseline `json:"cache_hit_rate"`
	ReadLatency           StatisticalBaseline `json:"read_latency"`
	WriteLatency          StatisticalBaseline `json:"write_latency"`
	BatchOperationsPerSec StatisticalBaseline `json:"batch_operations_per_sec"`
}

type NetworkBaseline struct {
	ActiveConnections StatisticalBaseline `json:"active_connections"`
	Latency          StatisticalBaseline `json:"latency"`
	Bandwidth        StatisticalBaseline `json:"bandwidth"`
	PacketLossRate   StatisticalBaseline `json:"packet_loss_rate"`
}

type ResourceBaseline struct {
	CPUUsage       StatisticalBaseline `json:"cpu_usage"`
	MemoryUsage    StatisticalBaseline `json:"memory_usage"`
	DiskUsage      StatisticalBaseline `json:"disk_usage"`
	GoroutineCount StatisticalBaseline `json:"goroutine_count"`
}

// PerformanceThresholds defines thresholds for performance analysis
type PerformanceThresholds struct {
	Bitswap    BitswapThresholds    `json:"bitswap"`
	Blockstore BlockstoreThresholds `json:"blockstore"`
	Network    NetworkThresholds    `json:"network"`
	Resource   ResourceThresholds   `json:"resource"`
}

type BitswapThresholds struct {
	MaxResponseTime       time.Duration `json:"max_response_time"`
	MaxQueuedRequests     int64         `json:"max_queued_requests"`
	MaxErrorRate          float64       `json:"max_error_rate"`
	MinRequestsPerSecond  float64       `json:"min_requests_per_second"`
}

type BlockstoreThresholds struct {
	MinCacheHitRate          float64       `json:"min_cache_hit_rate"`
	MaxReadLatency           time.Duration `json:"max_read_latency"`
	MaxWriteLatency          time.Duration `json:"max_write_latency"`
	MinBatchOperationsPerSec float64       `json:"min_batch_operations_per_sec"`
}

type NetworkThresholds struct {
	MaxLatency          time.Duration `json:"max_latency"`
	MinBandwidth        int64         `json:"min_bandwidth"`
	MaxPacketLossRate   float64       `json:"max_packet_loss_rate"`
	MaxConnectionErrors int64         `json:"max_connection_errors"`
}

type ResourceThresholds struct {
	MaxCPUUsage       float64 `json:"max_cpu_usage"`
	MaxMemoryUsage    float64 `json:"max_memory_usage"`
	MaxDiskUsage      float64 `json:"max_disk_usage"`
	MaxGoroutineCount int     `json:"max_goroutine_count"`
}

// Enums for categorization
type BottleneckType string
const (
	BottleneckTypeLatency     BottleneckType = "latency"
	BottleneckTypeThroughput  BottleneckType = "throughput"
	BottleneckTypeResource    BottleneckType = "resource"
	BottleneckTypeConnection  BottleneckType = "connection"
	BottleneckTypeCache       BottleneckType = "cache"
	BottleneckTypeQueue       BottleneckType = "queue"
)

type TrendType string
const (
	TrendTypeIncreasing TrendType = "increasing"
	TrendTypeDecreasing TrendType = "decreasing"
	TrendTypeStable     TrendType = "stable"
	TrendTypeVolatile   TrendType = "volatile"
)

type AnomalyType string
const (
	AnomalyTypeSpike     AnomalyType = "spike"
	AnomalyTypeDrop      AnomalyType = "drop"
	AnomalyTypeOutlier   AnomalyType = "outlier"
	AnomalyTypePattern   AnomalyType = "pattern"
)

type Severity string
const (
	SeverityCritical Severity = "critical"
	SeverityHigh     Severity = "high"
	SeverityMedium   Severity = "medium"
	SeverityLow      Severity = "low"
)

type RecommendationCategory string
const (
	CategoryConfiguration RecommendationCategory = "configuration"
	CategoryScaling       RecommendationCategory = "scaling"
	CategoryOptimization  RecommendationCategory = "optimization"
	CategoryMaintenance   RecommendationCategory = "maintenance"
)

type Priority string
const (
	PriorityImmediate Priority = "immediate"
	PriorityHigh      Priority = "high"
	PriorityMedium    Priority = "medium"
	PriorityLow       Priority = "low"
)

type ActionType string
const (
	ActionTypeConfigChange ActionType = "config_change"
	ActionTypeScaleUp      ActionType = "scale_up"
	ActionTypeScaleDown    ActionType = "scale_down"
	ActionTypeOptimize     ActionType = "optimize"
	ActionTypeMonitor      ActionType = "monitor"
)

type ImpactAssessment struct {
	PerformanceImpact string  `json:"performance_impact"`
	UserImpact        string  `json:"user_impact"`
	SystemImpact      string  `json:"system_impact"`
	Confidence        float64 `json:"confidence"`
}

type ExpectedImpact struct {
	PerformanceGain   string  `json:"performance_gain"`
	ResourceSavings   string  `json:"resource_savings"`
	Confidence        float64 `json:"confidence"`
}

type ImplementationEffort struct {
	Complexity   string        `json:"complexity"`
	TimeEstimate time.Duration `json:"time_estimate"`
	RiskLevel    string        `json:"risk_level"`
}

// bottleneckAnalyzer implements the BottleneckAnalyzer interface
type bottleneckAnalyzer struct {
	mu         sync.RWMutex
	baseline   *MetricBaseline
	thresholds PerformanceThresholds
	history    []PerformanceMetrics
	maxHistory int
}

// NewBottleneckAnalyzer creates a new bottleneck analyzer instance
func NewBottleneckAnalyzer() BottleneckAnalyzer {
	return &bottleneckAnalyzer{
		baseline:   &MetricBaseline{},
		thresholds: getDefaultThresholds(),
		history:    make([]PerformanceMetrics, 0),
		maxHistory: 1000,
	}
}

// getDefaultThresholds returns default performance thresholds
func getDefaultThresholds() PerformanceThresholds {
	return PerformanceThresholds{
		Bitswap: BitswapThresholds{
			MaxResponseTime:      100 * time.Millisecond,
			MaxQueuedRequests:    1000,
			MaxErrorRate:         0.05,
			MinRequestsPerSecond: 10,
		},
		Blockstore: BlockstoreThresholds{
			MinCacheHitRate:          0.80,
			MaxReadLatency:           50 * time.Millisecond,
			MaxWriteLatency:          100 * time.Millisecond,
			MinBatchOperationsPerSec: 100,
		},
		Network: NetworkThresholds{
			MaxLatency:          200 * time.Millisecond,
			MinBandwidth:        1024 * 1024,
			MaxPacketLossRate:   0.01,
			MaxConnectionErrors: 10,
		},
		Resource: ResourceThresholds{
			MaxCPUUsage:       80.0,
			MaxMemoryUsage:    85.0,
			MaxDiskUsage:      90.0,
			MaxGoroutineCount: 10000,
		},
	}
}

// AnalyzeBottlenecks performs comprehensive analysis and returns detected issues
func (ba *bottleneckAnalyzer) AnalyzeBottlenecks(ctx context.Context, metrics *PerformanceMetrics) ([]Bottleneck, error) {
	ba.mu.RLock()
	defer ba.mu.RUnlock()
	
	var bottlenecks []Bottleneck
	
	// Analyze Bitswap bottlenecks
	bitswapBottlenecks := ba.analyzeBitswapBottlenecks(metrics)
	bottlenecks = append(bottlenecks, bitswapBottlenecks...)
	
	// Analyze Blockstore bottlenecks
	blockstoreBottlenecks := ba.analyzeBlockstoreBottlenecks(metrics)
	bottlenecks = append(bottlenecks, blockstoreBottlenecks...)
	
	// Analyze Network bottlenecks
	networkBottlenecks := ba.analyzeNetworkBottlenecks(metrics)
	bottlenecks = append(bottlenecks, networkBottlenecks...)
	
	// Analyze Resource bottlenecks
	resourceBottlenecks := ba.analyzeResourceBottlenecks(metrics)
	bottlenecks = append(bottlenecks, resourceBottlenecks...)
	
	// Sort by severity
	sort.Slice(bottlenecks, func(i, j int) bool {
		return getSeverityWeight(bottlenecks[i].Severity) > getSeverityWeight(bottlenecks[j].Severity)
	})
	
	return bottlenecks, nil
}

// analyzeBitswapBottlenecks analyzes Bitswap-specific performance issues
func (ba *bottleneckAnalyzer) analyzeBitswapBottlenecks(metrics *PerformanceMetrics) []Bottleneck {
	var bottlenecks []Bottleneck
	
	// High response time bottleneck
	if metrics.BitswapMetrics.P95ResponseTime > ba.thresholds.Bitswap.MaxResponseTime {
		bottlenecks = append(bottlenecks, Bottleneck{
			ID:          fmt.Sprintf("bitswap-high-latency-%d", time.Now().Unix()),
			Component:   "bitswap",
			Type:        BottleneckTypeLatency,
			Severity:    ba.calculateLatencySeverity(metrics.BitswapMetrics.P95ResponseTime, ba.thresholds.Bitswap.MaxResponseTime),
			Description: fmt.Sprintf("Bitswap P95 response time (%v) exceeds threshold (%v)", metrics.BitswapMetrics.P95ResponseTime, ba.thresholds.Bitswap.MaxResponseTime),
			Metrics: map[string]interface{}{
				"p95_response_time": metrics.BitswapMetrics.P95ResponseTime,
				"threshold":         ba.thresholds.Bitswap.MaxResponseTime,
			},
			DetectedAt: time.Now(),
			Impact: ImpactAssessment{
				PerformanceImpact: "High latency affects user experience and throughput",
				UserImpact:        "Slower block retrieval and content access",
				SystemImpact:      "Reduced overall system throughput",
				Confidence:        0.9,
			},
		})
	}
	
	// High queue depth bottleneck
	if metrics.BitswapMetrics.QueuedRequests > ba.thresholds.Bitswap.MaxQueuedRequests {
		bottlenecks = append(bottlenecks, Bottleneck{
			ID:          fmt.Sprintf("bitswap-queue-depth-%d", time.Now().Unix()),
			Component:   "bitswap",
			Type:        BottleneckTypeQueue,
			Severity:    ba.calculateQueueSeverity(metrics.BitswapMetrics.QueuedRequests, ba.thresholds.Bitswap.MaxQueuedRequests),
			Description: fmt.Sprintf("Bitswap queue depth (%d) exceeds threshold (%d)", metrics.BitswapMetrics.QueuedRequests, ba.thresholds.Bitswap.MaxQueuedRequests),
			Metrics: map[string]interface{}{
				"queued_requests": metrics.BitswapMetrics.QueuedRequests,
				"threshold":       ba.thresholds.Bitswap.MaxQueuedRequests,
			},
			DetectedAt: time.Now(),
			Impact: ImpactAssessment{
				PerformanceImpact: "Request backlog indicates processing bottleneck",
				UserImpact:        "Increased wait times for block requests",
				SystemImpact:      "Memory pressure from queued requests",
				Confidence:        0.85,
			},
		})
	}
	
	return bottlenecks
}

// analyzeBlockstoreBottlenecks analyzes Blockstore-specific performance issues
func (ba *bottleneckAnalyzer) analyzeBlockstoreBottlenecks(metrics *PerformanceMetrics) []Bottleneck {
	var bottlenecks []Bottleneck
	
	// Low cache hit rate bottleneck
	if metrics.BlockstoreMetrics.CacheHitRate < ba.thresholds.Blockstore.MinCacheHitRate {
		bottlenecks = append(bottlenecks, Bottleneck{
			ID:          fmt.Sprintf("blockstore-cache-miss-%d", time.Now().Unix()),
			Component:   "blockstore",
			Type:        BottleneckTypeCache,
			Severity:    ba.calculateCacheSeverity(metrics.BlockstoreMetrics.CacheHitRate, ba.thresholds.Blockstore.MinCacheHitRate),
			Description: fmt.Sprintf("Blockstore cache hit rate (%.2f%%) below threshold (%.2f%%)", metrics.BlockstoreMetrics.CacheHitRate*100, ba.thresholds.Blockstore.MinCacheHitRate*100),
			Metrics: map[string]interface{}{
				"cache_hit_rate": metrics.BlockstoreMetrics.CacheHitRate,
				"threshold":      ba.thresholds.Blockstore.MinCacheHitRate,
			},
			DetectedAt: time.Now(),
			Impact: ImpactAssessment{
				PerformanceImpact: "Low cache hit rate increases I/O operations",
				UserImpact:        "Slower block access and higher latency",
				SystemImpact:      "Increased disk I/O and CPU usage",
				Confidence:        0.9,
			},
		})
	}
	
	return bottlenecks
}

// analyzeNetworkBottlenecks analyzes Network-specific performance issues
func (ba *bottleneckAnalyzer) analyzeNetworkBottlenecks(metrics *PerformanceMetrics) []Bottleneck {
	var bottlenecks []Bottleneck
	
	// High latency bottleneck
	if metrics.NetworkMetrics.AverageLatency > ba.thresholds.Network.MaxLatency {
		bottlenecks = append(bottlenecks, Bottleneck{
			ID:          fmt.Sprintf("network-latency-%d", time.Now().Unix()),
			Component:   "network",
			Type:        BottleneckTypeLatency,
			Severity:    ba.calculateLatencySeverity(metrics.NetworkMetrics.AverageLatency, ba.thresholds.Network.MaxLatency),
			Description: fmt.Sprintf("Network latency (%v) exceeds threshold (%v)", metrics.NetworkMetrics.AverageLatency, ba.thresholds.Network.MaxLatency),
			Metrics: map[string]interface{}{
				"latency":   metrics.NetworkMetrics.AverageLatency,
				"threshold": ba.thresholds.Network.MaxLatency,
			},
			DetectedAt: time.Now(),
			Impact: ImpactAssessment{
				PerformanceImpact: "High network latency affects all operations",
				UserImpact:        "Slower response times across the system",
				SystemImpact:      "Reduced cluster efficiency",
				Confidence:        0.9,
			},
		})
	}
	
	return bottlenecks
}

// analyzeResourceBottlenecks analyzes Resource-specific performance issues
func (ba *bottleneckAnalyzer) analyzeResourceBottlenecks(metrics *PerformanceMetrics) []Bottleneck {
	var bottlenecks []Bottleneck
	
	// High CPU usage bottleneck
	if metrics.ResourceMetrics.CPUUsage > ba.thresholds.Resource.MaxCPUUsage {
		bottlenecks = append(bottlenecks, Bottleneck{
			ID:          fmt.Sprintf("resource-cpu-%d", time.Now().Unix()),
			Component:   "resource",
			Type:        BottleneckTypeResource,
			Severity:    ba.calculateResourceSeverity(metrics.ResourceMetrics.CPUUsage, ba.thresholds.Resource.MaxCPUUsage),
			Description: fmt.Sprintf("CPU usage (%.1f%%) exceeds threshold (%.1f%%)", metrics.ResourceMetrics.CPUUsage, ba.thresholds.Resource.MaxCPUUsage),
			Metrics: map[string]interface{}{
				"cpu_usage": metrics.ResourceMetrics.CPUUsage,
				"threshold": ba.thresholds.Resource.MaxCPUUsage,
			},
			DetectedAt: time.Now(),
			Impact: ImpactAssessment{
				PerformanceImpact: "High CPU usage affects all operations",
				UserImpact:        "System slowdown and increased latency",
				SystemImpact:      "Potential system instability",
				Confidence:        0.9,
			},
		})
	}
	
	return bottlenecks
}

// Helper functions for severity calculation
func (ba *bottleneckAnalyzer) calculateLatencySeverity(actual, threshold time.Duration) Severity {
	ratio := float64(actual) / float64(threshold)
	switch {
	case ratio >= 3.0:
		return SeverityCritical
	case ratio >= 2.0:
		return SeverityHigh
	case ratio >= 1.5:
		return SeverityMedium
	default:
		return SeverityLow
	}
}

func (ba *bottleneckAnalyzer) calculateQueueSeverity(actual, threshold int64) Severity {
	ratio := float64(actual) / float64(threshold)
	switch {
	case ratio >= 5.0:
		return SeverityCritical
	case ratio >= 3.0:
		return SeverityHigh
	case ratio >= 2.0:
		return SeverityMedium
	default:
		return SeverityLow
	}
}

func (ba *bottleneckAnalyzer) calculateCacheSeverity(actual, threshold float64) Severity {
	diff := threshold - actual
	switch {
	case diff >= 0.3:
		return SeverityCritical
	case diff >= 0.2:
		return SeverityHigh
	case diff >= 0.1:
		return SeverityMedium
	default:
		return SeverityLow
	}
}

func (ba *bottleneckAnalyzer) calculateResourceSeverity(actual, threshold float64) Severity {
	ratio := actual / threshold
	switch {
	case ratio >= 1.2:
		return SeverityCritical
	case ratio >= 1.1:
		return SeverityHigh
	case ratio >= 1.05:
		return SeverityMedium
	default:
		return SeverityLow
	}
}

func getSeverityWeight(severity Severity) int {
	switch severity {
	case SeverityCritical:
		return 4
	case SeverityHigh:
		return 3
	case SeverityMedium:
		return 2
	case SeverityLow:
		return 1
	default:
		return 0
	}
}

// AnalyzeTrends analyzes metric trends over time
func (ba *bottleneckAnalyzer) AnalyzeTrends(ctx context.Context, history []PerformanceMetrics) ([]TrendAnalysis, error) {
	if len(history) < 3 {
		return nil, fmt.Errorf("insufficient data for trend analysis (need at least 3 samples, got %d)", len(history))
	}
	
	var trends []TrendAnalysis
	
	// Simple trend analysis for response time
	responseTimes := make([]float64, len(history))
	for i, metrics := range history {
		responseTimes[i] = float64(metrics.BitswapMetrics.AverageResponseTime.Nanoseconds())
	}
	
	if trend := ba.calculateTrend("response_time", "bitswap", responseTimes, history); trend != nil {
		trends = append(trends, *trend)
	}
	
	return trends, nil
}

// calculateTrend performs simple trend analysis
func (ba *bottleneckAnalyzer) calculateTrend(metric, component string, values []float64, history []PerformanceMetrics) *TrendAnalysis {
	if len(values) < 3 {
		return nil
	}
	
	// Simple slope calculation
	n := len(values)
	x := make([]float64, n)
	for i := 0; i < n; i++ {
		x[i] = float64(i)
	}
	
	slope, _, r2 := ba.linearRegression(x, values)
	
	// Determine trend type
	trendType := TrendTypeStable
	if math.Abs(slope) > 0.1 {
		if slope > 0 {
			trendType = TrendTypeIncreasing
		} else {
			trendType = TrendTypeDecreasing
		}
	}
	
	period := time.Duration(0)
	if len(history) > 1 {
		period = history[len(history)-1].Timestamp.Sub(history[0].Timestamp)
	}
	
	return &TrendAnalysis{
		Metric:    metric,
		Component: component,
		Trend:     trendType,
		Slope:     slope,
		R2:        r2,
		Period:    period,
		Forecast:  []float64{},
	}
}

// linearRegression calculates simple linear regression
func (ba *bottleneckAnalyzer) linearRegression(x, y []float64) (slope, intercept, r2 float64) {
	if len(x) != len(y) || len(x) < 2 {
		return 0, 0, 0
	}
	
	n := float64(len(x))
	
	var sumX, sumY float64
	for i := 0; i < len(x); i++ {
		sumX += x[i]
		sumY += y[i]
	}
	meanX := sumX / n
	meanY := sumY / n
	
	var numerator, denominator float64
	for i := 0; i < len(x); i++ {
		numerator += (x[i] - meanX) * (y[i] - meanY)
		denominator += (x[i] - meanX) * (x[i] - meanX)
	}
	
	if denominator == 0 {
		return 0, meanY, 0
	}
	
	slope = numerator / denominator
	intercept = meanY - slope*meanX
	
	// Calculate R-squared
	var ssRes, ssTot float64
	for i := 0; i < len(y); i++ {
		predicted := slope*x[i] + intercept
		ssRes += (y[i] - predicted) * (y[i] - predicted)
		ssTot += (y[i] - meanY) * (y[i] - meanY)
	}
	
	if ssTot == 0 {
		r2 = 1.0
	} else {
		r2 = 1.0 - (ssRes / ssTot)
	}
	
	return slope, intercept, r2
}

// DetectAnomalies identifies anomalous behavior in metrics
func (ba *bottleneckAnalyzer) DetectAnomalies(ctx context.Context, metrics *PerformanceMetrics, baseline *MetricBaseline) ([]Anomaly, error) {
	if baseline == nil {
		return nil, fmt.Errorf("baseline is required for anomaly detection")
	}
	
	var anomalies []Anomaly
	
	// Simple anomaly detection for response time
	responseTimeNs := float64(metrics.BitswapMetrics.AverageResponseTime.Nanoseconds())
	if anomaly := ba.detectStatisticalAnomaly("response_time", "bitswap", responseTimeNs, baseline.Bitswap.ResponseTime, 3.0); anomaly != nil {
		anomalies = append(anomalies, *anomaly)
	}
	
	return anomalies, nil
}

// detectStatisticalAnomaly detects anomalies using Z-score
func (ba *bottleneckAnalyzer) detectStatisticalAnomaly(metric, component string, value float64, baseline StatisticalBaseline, threshold float64) *Anomaly {
	if baseline.SampleCount < 10 || baseline.StdDev == 0 {
		return nil
	}
	
	zScore := math.Abs(value - baseline.Mean) / baseline.StdDev
	
	if zScore < threshold {
		return nil
	}
	
	anomalyType := AnomalyTypeOutlier
	severity := SeverityLow
	
	if zScore >= 4.0 {
		anomalyType = AnomalyTypeSpike
		severity = SeverityCritical
	} else if zScore >= 3.5 {
		severity = SeverityHigh
	} else if zScore >= 3.0 {
		severity = SeverityMedium
	}
	
	if value < baseline.Mean {
		anomalyType = AnomalyTypeDrop
	}
	
	return &Anomaly{
		ID:        fmt.Sprintf("anomaly-%s-%s-%d", component, metric, time.Now().Unix()),
		Metric:    metric,
		Component: component,
		Type:      anomalyType,
		Severity:  severity,
		Value:     value,
		Expected:  baseline.Mean,
		Deviation: zScore,
		DetectedAt: time.Now(),
		Context: map[string]interface{}{
			"z_score":   zScore,
			"threshold": threshold,
		},
	}
}

// GenerateRecommendations provides optimization recommendations
func (ba *bottleneckAnalyzer) GenerateRecommendations(ctx context.Context, bottlenecks []Bottleneck) ([]OptimizationRecommendation, error) {
	var recommendations []OptimizationRecommendation
	
	for _, bottleneck := range bottlenecks {
		if bottleneck.Component == "bitswap" && bottleneck.Type == BottleneckTypeLatency {
			recommendations = append(recommendations, OptimizationRecommendation{
				ID:       fmt.Sprintf("bitswap-latency-opt-%d", time.Now().Unix()),
				Category: CategoryOptimization,
				Priority: ba.severityToPriority(bottleneck.Severity),
				Title:    "Optimize Bitswap Response Time",
				Description: "High Bitswap response times detected. Consider optimizing connection management.",
				Actions: []RecommendedAction{
					{
						Type:        ActionTypeConfigChange,
						Description: "Increase MaxOutstandingBytesPerPeer",
						Parameters: map[string]interface{}{
							"config_key": "BitswapMaxOutstandingBytesPerPeer",
							"suggested_value": "5MB",
						},
						Validation: "Monitor P95 response time should decrease",
					},
				},
				Impact: ExpectedImpact{
					PerformanceGain: "20-40% reduction in response time",
					ResourceSavings: "Better CPU utilization",
					Confidence:      0.8,
				},
				Effort: ImplementationEffort{
					Complexity:   "Low",
					TimeEstimate: 30 * time.Minute,
					RiskLevel:    "Low",
				},
			})
		}
	}
	
	return recommendations, nil
}

func (ba *bottleneckAnalyzer) severityToPriority(severity Severity) Priority {
	switch severity {
	case SeverityCritical:
		return PriorityImmediate
	case SeverityHigh:
		return PriorityHigh
	case SeverityMedium:
		return PriorityMedium
	default:
		return PriorityLow
	}
}

// UpdateBaseline updates the baseline metrics
func (ba *bottleneckAnalyzer) UpdateBaseline(metrics *PerformanceMetrics) error {
	ba.mu.Lock()
	defer ba.mu.Unlock()
	
	ba.history = append(ba.history, *metrics)
	
	if len(ba.history) > ba.maxHistory {
		ba.history = ba.history[len(ba.history)-ba.maxHistory:]
	}
	
	if len(ba.history) >= 10 {
		ba.baseline = ba.calculateBaseline(ba.history)
		ba.baseline.UpdatedAt = time.Now()
	}
	
	return nil
}

// GetBaseline returns the current baseline
func (ba *bottleneckAnalyzer) GetBaseline() *MetricBaseline {
	ba.mu.RLock()
	defer ba.mu.RUnlock()
	
	baselineCopy := *ba.baseline
	return &baselineCopy
}

// SetThresholds configures performance thresholds
func (ba *bottleneckAnalyzer) SetThresholds(thresholds PerformanceThresholds) error {
	ba.mu.Lock()
	defer ba.mu.Unlock()
	
	if thresholds.Bitswap.MaxResponseTime <= 0 {
		return fmt.Errorf("bitswap max response time must be positive")
	}
	
	ba.thresholds = thresholds
	return nil
}

// calculateBaseline calculates baseline from history
func (ba *bottleneckAnalyzer) calculateBaseline(history []PerformanceMetrics) *MetricBaseline {
	baseline := &MetricBaseline{
		Custom: make(map[string]ComponentBaseline),
	}
	
	// Calculate Bitswap baseline
	responseTimes := make([]float64, len(history))
	for i, metrics := range history {
		responseTimes[i] = float64(metrics.BitswapMetrics.AverageResponseTime.Nanoseconds())
	}
	
	baseline.Bitswap = BitswapBaseline{
		ResponseTime: ba.calculateStatisticalBaseline(responseTimes),
	}
	
	return baseline
}

// calculateStatisticalBaseline calculates statistical baseline
func (ba *bottleneckAnalyzer) calculateStatisticalBaseline(values []float64) StatisticalBaseline {
	if len(values) == 0 {
		return StatisticalBaseline{}
	}
	
	mean := ba.calculateMean(values)
	stdDev := ba.calculateStdDev(values, mean)
	
	return StatisticalBaseline{
		Mean:        mean,
		StdDev:      stdDev,
		SampleCount: len(values),
		LastUpdated: time.Now(),
	}
}

// calculateMean calculates arithmetic mean
func (ba *bottleneckAnalyzer) calculateMean(values []float64) float64 {
	if len(values) == 0 {
		return 0
	}
	
	sum := 0.0
	for _, v := range values {
		sum += v
	}
	return sum / float64(len(values))
}

// calculateStdDev calculates standard deviation
func (ba *bottleneckAnalyzer) calculateStdDev(values []float64, mean float64) float64 {
	if len(values) <= 1 {
		return 0
	}
	
	sumSquares := 0.0
	for _, v := range values {
		diff := v - mean
		sumSquares += diff * diff
	}
	
	return math.Sqrt(sumSquares / float64(len(values)-1))
}