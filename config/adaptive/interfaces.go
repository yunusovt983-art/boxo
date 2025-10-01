package adaptive

import (
	"context"
	"time"
)

// ResourceManager defines the interface for adaptive resource management
type ResourceManager interface {
	// Monitor resource usage and return current metrics
	GetResourceMetrics(ctx context.Context) (*ResourceMetrics, error)
	
	// Apply resource constraints based on current usage
	ApplyResourceConstraints(ctx context.Context, constraints *ResourceConstraints) error
	
	// Check if graceful degradation should be triggered
	ShouldDegrade(ctx context.Context) (bool, *DegradationRecommendation, error)
	
	// Apply degradation step
	ApplyDegradation(ctx context.Context, step *DegradationStep) error
	
	// Recover from degradation when resources are available
	RecoverFromDegradation(ctx context.Context) error
	
	// Get current degradation level
	GetDegradationLevel() DegradationLevel
}

// ConfigManager defines the interface for dynamic configuration management
type ConfigManager interface {
	// Get current configuration
	GetConfig() *AdaptiveConfig
	
	// Update configuration with validation
	UpdateConfig(ctx context.Context, config *AdaptiveConfig) error
	
	// Apply configuration changes without restart
	ApplyConfigChanges(ctx context.Context, changes *ConfigChanges) error
	
	// Validate configuration before applying
	ValidateConfig(config *AdaptiveConfig) error
	
	// Get configuration recommendations based on current metrics
	GetConfigRecommendations(ctx context.Context, metrics *PerformanceMetrics) (*ConfigRecommendations, error)
	
	// Subscribe to configuration changes
	Subscribe(ctx context.Context) (<-chan *ConfigChangeEvent, error)
}

// AutoTuner defines the interface for automatic parameter tuning
type AutoTuner interface {
	// Start auto-tuning process
	Start(ctx context.Context) error
	
	// Stop auto-tuning process
	Stop(ctx context.Context) error
	
	// Get tuning recommendations based on performance metrics
	GetTuningRecommendations(ctx context.Context, metrics *PerformanceMetrics) (*TuningRecommendations, error)
	
	// Apply tuning recommendations safely
	ApplyTuning(ctx context.Context, recommendations *TuningRecommendations) error
	
	// Get current tuning status
	GetTuningStatus() *TuningStatus
	
	// Enable/disable machine learning mode
	SetMLMode(enabled bool) error
}

// PerformanceMonitor defines the interface for performance monitoring
type PerformanceMonitor interface {
	// Start monitoring
	Start(ctx context.Context) error
	
	// Stop monitoring
	Stop(ctx context.Context) error
	
	// Get current performance metrics
	GetMetrics(ctx context.Context) (*PerformanceMetrics, error)
	
	// Get historical metrics for a time range
	GetHistoricalMetrics(ctx context.Context, from, to time.Time) ([]*PerformanceMetrics, error)
	
	// Analyze performance bottlenecks
	AnalyzeBottlenecks(ctx context.Context) (*BottleneckAnalysis, error)
	
	// Subscribe to performance alerts
	SubscribeToAlerts(ctx context.Context) (<-chan *PerformanceAlert, error)
}

// AdaptiveComponent defines the interface for components that can be adaptively configured
type AdaptiveComponent interface {
	// Get component name
	GetName() string
	
	// Apply new configuration
	ApplyConfig(ctx context.Context, config interface{}) error
	
	// Get current performance metrics
	GetMetrics(ctx context.Context) (interface{}, error)
	
	// Get configuration recommendations
	GetConfigRecommendations(ctx context.Context, metrics interface{}) (interface{}, error)
	
	// Check if component supports hot reconfiguration
	SupportsHotReconfig() bool
}

// Data structures for the interfaces

// ResourceMetrics contains current resource usage information
type ResourceMetrics struct {
	Timestamp time.Time `json:"timestamp"`
	
	// Memory metrics
	MemoryUsed      int64   `json:"memory_used"`
	MemoryTotal     int64   `json:"memory_total"`
	MemoryPercent   float64 `json:"memory_percent"`
	GCPressure      float64 `json:"gc_pressure"`
	
	// CPU metrics
	CPUUsage        float64 `json:"cpu_usage"`
	CPUCores        int     `json:"cpu_cores"`
	LoadAverage     float64 `json:"load_average"`
	
	// Disk metrics
	DiskUsed        int64   `json:"disk_used"`
	DiskTotal       int64   `json:"disk_total"`
	DiskPercent     float64 `json:"disk_percent"`
	DiskIOPS        int64   `json:"disk_iops"`
	
	// Network metrics
	NetworkBandwidth int64   `json:"network_bandwidth"`
	NetworkLatency   time.Duration `json:"network_latency"`
	NetworkErrors    int64   `json:"network_errors"`
	
	// Application-specific metrics
	ActiveConnections int     `json:"active_connections"`
	QueuedRequests    int64   `json:"queued_requests"`
	ErrorRate         float64 `json:"error_rate"`
}

// ResourceConstraints defines resource usage limits
type ResourceConstraints struct {
	MaxMemoryUsage   int64   `json:"max_memory_usage"`
	MaxCPUUsage      float64 `json:"max_cpu_usage"`
	MaxDiskUsage     int64   `json:"max_disk_usage"`
	MaxConnections   int     `json:"max_connections"`
	MaxQueueSize     int64   `json:"max_queue_size"`
}

// DegradationRecommendation suggests what degradation steps to take
type DegradationRecommendation struct {
	Level       DegradationLevel `json:"level"`
	Steps       []DegradationStep `json:"steps"`
	Reason      string           `json:"reason"`
	Urgency     string           `json:"urgency"` // low, medium, high, critical
	EstimatedImpact string       `json:"estimated_impact"`
}

// DegradationLevel represents the current degradation state
type DegradationLevel int

const (
	DegradationNone DegradationLevel = iota
	DegradationLow
	DegradationMedium
	DegradationHigh
	DegradationCritical
)

func (d DegradationLevel) String() string {
	switch d {
	case DegradationNone:
		return "none"
	case DegradationLow:
		return "low"
	case DegradationMedium:
		return "medium"
	case DegradationHigh:
		return "high"
	case DegradationCritical:
		return "critical"
	default:
		return "unknown"
	}
}

// ConfigChanges represents a set of configuration changes
type ConfigChanges struct {
	Changes   map[string]interface{} `json:"changes"`
	Reason    string                 `json:"reason"`
	Source    string                 `json:"source"` // manual, auto-tune, alert
	Timestamp time.Time              `json:"timestamp"`
}

// ConfigRecommendations contains suggested configuration changes
type ConfigRecommendations struct {
	Recommendations []ConfigRecommendation `json:"recommendations"`
	Confidence      float64                `json:"confidence"`
	EstimatedImpact string                 `json:"estimated_impact"`
	Timestamp       time.Time              `json:"timestamp"`
}

// ConfigRecommendation represents a single configuration recommendation
type ConfigRecommendation struct {
	Component   string      `json:"component"`
	Parameter   string      `json:"parameter"`
	CurrentValue interface{} `json:"current_value"`
	RecommendedValue interface{} `json:"recommended_value"`
	Reason      string      `json:"reason"`
	Priority    string      `json:"priority"` // low, medium, high
	Impact      string      `json:"impact"`
}

// ConfigChangeEvent represents a configuration change event
type ConfigChangeEvent struct {
	Type      string                 `json:"type"` // update, validate, apply
	Changes   map[string]interface{} `json:"changes"`
	Source    string                 `json:"source"`
	Timestamp time.Time              `json:"timestamp"`
	Success   bool                   `json:"success"`
	Error     string                 `json:"error,omitempty"`
}

// TuningRecommendations contains auto-tuning suggestions
type TuningRecommendations struct {
	Recommendations []TuningRecommendation `json:"recommendations"`
	Confidence      float64                `json:"confidence"`
	SafetyScore     float64                `json:"safety_score"`
	Timestamp       time.Time              `json:"timestamp"`
}

// TuningRecommendation represents a single tuning recommendation
type TuningRecommendation struct {
	Component       string      `json:"component"`
	Parameter       string      `json:"parameter"`
	CurrentValue    interface{} `json:"current_value"`
	RecommendedValue interface{} `json:"recommended_value"`
	ChangePercent   float64     `json:"change_percent"`
	Confidence      float64     `json:"confidence"`
	Reason          string      `json:"reason"`
	RiskLevel       string      `json:"risk_level"` // low, medium, high
}

// TuningStatus represents the current state of auto-tuning
type TuningStatus struct {
	Enabled         bool      `json:"enabled"`
	LastTuning      time.Time `json:"last_tuning"`
	NextTuning      time.Time `json:"next_tuning"`
	TuningCycles    int64     `json:"tuning_cycles"`
	SuccessfulTunes int64     `json:"successful_tunes"`
	FailedTunes     int64     `json:"failed_tunes"`
	MLMode          bool      `json:"ml_mode"`
	SafetyMode      bool      `json:"safety_mode"`
}

// PerformanceMetrics contains comprehensive performance data
type PerformanceMetrics struct {
	Timestamp time.Time `json:"timestamp"`
	
	// Bitswap metrics
	Bitswap *BitswapMetrics `json:"bitswap"`
	
	// Blockstore metrics
	Blockstore *BlockstoreMetrics `json:"blockstore"`
	
	// Network metrics
	Network *NetworkMetrics `json:"network"`
	
	// Resource metrics
	Resources *ResourceMetrics `json:"resources"`
	
	// Overall system metrics
	System *SystemMetrics `json:"system"`
}

// BitswapMetrics contains Bitswap-specific performance data
type BitswapMetrics struct {
	RequestsPerSecond     float64       `json:"requests_per_second"`
	AverageResponseTime   time.Duration `json:"average_response_time"`
	P95ResponseTime       time.Duration `json:"p95_response_time"`
	P99ResponseTime       time.Duration `json:"p99_response_time"`
	OutstandingRequests   int64         `json:"outstanding_requests"`
	ActiveConnections     int           `json:"active_connections"`
	QueuedRequests        int64         `json:"queued_requests"`
	SuccessfulRequests    int64         `json:"successful_requests"`
	FailedRequests        int64         `json:"failed_requests"`
	BytesTransferred      int64         `json:"bytes_transferred"`
	WorkerUtilization     float64       `json:"worker_utilization"`
	CircuitBreakerState   string        `json:"circuit_breaker_state"`
}

// BlockstoreMetrics contains blockstore-specific performance data
type BlockstoreMetrics struct {
	CacheHitRate          float64       `json:"cache_hit_rate"`
	CacheMissRate         float64       `json:"cache_miss_rate"`
	AverageReadLatency    time.Duration `json:"average_read_latency"`
	AverageWriteLatency   time.Duration `json:"average_write_latency"`
	BatchOperationsPerSec float64       `json:"batch_operations_per_sec"`
	CompressionRatio      float64       `json:"compression_ratio"`
	StorageUtilization    float64       `json:"storage_utilization"`
	IOPSRead              int64         `json:"iops_read"`
	IOPSWrite             int64         `json:"iops_write"`
	QueueDepth            int           `json:"queue_depth"`
}

// NetworkMetrics contains network-specific performance data
type NetworkMetrics struct {
	ActiveConnections     int           `json:"active_connections"`
	ConnectionsPerSecond  float64       `json:"connections_per_second"`
	AverageLatency        time.Duration `json:"average_latency"`
	P95Latency            time.Duration `json:"p95_latency"`
	TotalBandwidth        int64         `json:"total_bandwidth"`
	InboundBandwidth      int64         `json:"inbound_bandwidth"`
	OutboundBandwidth     int64         `json:"outbound_bandwidth"`
	PacketLossRate        float64       `json:"packet_loss_rate"`
	ConnectionErrors      int64         `json:"connection_errors"`
	TimeoutErrors         int64         `json:"timeout_errors"`
	KeepAliveConnections  int           `json:"keep_alive_connections"`
}

// SystemMetrics contains overall system performance data
type SystemMetrics struct {
	Uptime              time.Duration `json:"uptime"`
	TotalRequests       int64         `json:"total_requests"`
	RequestsPerSecond   float64       `json:"requests_per_second"`
	AverageResponseTime time.Duration `json:"average_response_time"`
	ErrorRate           float64       `json:"error_rate"`
	Throughput          int64         `json:"throughput"`
	Availability        float64       `json:"availability"`
}

// BottleneckAnalysis contains analysis of performance bottlenecks
type BottleneckAnalysis struct {
	Timestamp   time.Time    `json:"timestamp"`
	Bottlenecks []Bottleneck `json:"bottlenecks"`
	Overall     string       `json:"overall"` // overall system health: good, degraded, poor, critical
}

// Bottleneck represents a detected performance bottleneck
type Bottleneck struct {
	Component   string  `json:"component"`
	Metric      string  `json:"metric"`
	Severity    string  `json:"severity"` // low, medium, high, critical
	Impact      string  `json:"impact"`
	Suggestion  string  `json:"suggestion"`
	Confidence  float64 `json:"confidence"`
}

// PerformanceAlert represents a performance-related alert
type PerformanceAlert struct {
	ID          string    `json:"id"`
	Timestamp   time.Time `json:"timestamp"`
	Component   string    `json:"component"`
	Metric      string    `json:"metric"`
	Threshold   float64   `json:"threshold"`
	CurrentValue float64  `json:"current_value"`
	Severity    string    `json:"severity"`
	Message     string    `json:"message"`
	Resolved    bool      `json:"resolved"`
	ResolvedAt  *time.Time `json:"resolved_at,omitempty"`
}