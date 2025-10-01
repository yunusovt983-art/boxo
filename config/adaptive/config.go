package adaptive

import (
	"errors"
	"fmt"
	"sync"
	"time"
)

// AdaptiveConfig represents the main configuration structure for high-load optimization
type AdaptiveConfig struct {
	mu sync.RWMutex

	// Bitswap configuration
	Bitswap *BitswapConfig `json:"bitswap"`

	// Blockstore configuration
	Blockstore *BlockstoreConfig `json:"blockstore"`

	// Network configuration
	Network *NetworkConfig `json:"network"`

	// Monitoring configuration
	Monitoring *MonitoringConfig `json:"monitoring"`

	// Resource management
	Resources *ResourceConfig `json:"resources"`

	// Auto-tuning settings
	AutoTune *AutoTuneConfig `json:"auto_tune"`
}

// BitswapConfig contains adaptive Bitswap settings
type BitswapConfig struct {
	// Dynamic limits for outstanding bytes per peer
	MaxOutstandingBytesPerPeer int64 `json:"max_outstanding_bytes_per_peer"`
	MinOutstandingBytesPerPeer int64 `json:"min_outstanding_bytes_per_peer"`

	// Worker pool settings
	MinWorkerCount int `json:"min_worker_count"`
	MaxWorkerCount int `json:"max_worker_count"`

	// Priority thresholds
	HighPriorityThreshold     time.Duration `json:"high_priority_threshold"`
	CriticalPriorityThreshold time.Duration `json:"critical_priority_threshold"`

	// Batch processing
	BatchSize    int           `json:"batch_size"`
	BatchTimeout time.Duration `json:"batch_timeout"`

	// Circuit breaker settings
	CircuitBreakerEnabled         bool          `json:"circuit_breaker_enabled"`
	CircuitBreakerFailureRate     float64       `json:"circuit_breaker_failure_rate"`
	CircuitBreakerRecoveryTimeout time.Duration `json:"circuit_breaker_recovery_timeout"`
}

// BlockstoreConfig contains blockstore optimization settings
type BlockstoreConfig struct {
	// Multi-tier cache settings
	MemoryCacheSize int64 `json:"memory_cache_size"`
	SSDCacheSize    int64 `json:"ssd_cache_size"`
	HDDCacheSize    int64 `json:"hdd_cache_size"`

	// Batch I/O settings
	BatchSize    int           `json:"batch_size"`
	BatchTimeout time.Duration `json:"batch_timeout"`

	// Compression settings
	CompressionEnabled bool `json:"compression_enabled"`
	CompressionLevel   int  `json:"compression_level"`

	// Streaming settings for large blocks
	StreamingThreshold int64 `json:"streaming_threshold"` // Size threshold for streaming (e.g., 1MB)
}

// NetworkConfig contains network optimization settings
type NetworkConfig struct {
	// Connection manager settings
	HighWater   int           `json:"high_water"`
	LowWater    int           `json:"low_water"`
	GracePeriod time.Duration `json:"grace_period"`

	// Connection quality thresholds
	LatencyThreshold    time.Duration `json:"latency_threshold"`
	BandwidthThreshold  int64         `json:"bandwidth_threshold"`
	ErrorRateThreshold  float64       `json:"error_rate_threshold"`

	// Keep-alive settings
	KeepAliveInterval time.Duration `json:"keep_alive_interval"`
	KeepAliveTimeout  time.Duration `json:"keep_alive_timeout"`

	// Buffer optimization
	AdaptiveBufferSizing bool  `json:"adaptive_buffer_sizing"`
	MinBufferSize        int64 `json:"min_buffer_size"`
	MaxBufferSize        int64 `json:"max_buffer_size"`
}

// MonitoringConfig contains monitoring and metrics settings
type MonitoringConfig struct {
	// Metrics collection
	MetricsEnabled       bool          `json:"metrics_enabled"`
	MetricsInterval      time.Duration `json:"metrics_interval"`
	PrometheusEnabled    bool          `json:"prometheus_enabled"`
	PrometheusPort       int           `json:"prometheus_port"`

	// Performance analysis
	BottleneckAnalysisEnabled bool          `json:"bottleneck_analysis_enabled"`
	AnalysisInterval          time.Duration `json:"analysis_interval"`

	// Alerting
	AlertingEnabled    bool          `json:"alerting_enabled"`
	AlertThresholds    AlertConfig   `json:"alert_thresholds"`
	AlertCooldown      time.Duration `json:"alert_cooldown"`

	// Structured logging
	StructuredLogging bool   `json:"structured_logging"`
	LogLevel          string `json:"log_level"`
	TraceEnabled      bool   `json:"trace_enabled"`
}

// AlertConfig defines thresholds for various alerts
type AlertConfig struct {
	ResponseTimeP95    time.Duration `json:"response_time_p95"`
	ErrorRate          float64       `json:"error_rate"`
	MemoryUsage        float64       `json:"memory_usage"`
	CPUUsage           float64       `json:"cpu_usage"`
	DiskUsage          float64       `json:"disk_usage"`
	ConnectionFailures int           `json:"connection_failures"`
}

// ResourceConfig contains resource management settings
type ResourceConfig struct {
	// Memory management
	MaxMemoryUsage      int64   `json:"max_memory_usage"`
	MemoryPressureLevel float64 `json:"memory_pressure_level"`
	GCPressureThreshold float64 `json:"gc_pressure_threshold"`

	// CPU management
	MaxCPUUsage         float64 `json:"max_cpu_usage"`
	CPUThrottleEnabled  bool    `json:"cpu_throttle_enabled"`

	// Disk management
	MaxDiskUsage        float64 `json:"max_disk_usage"`
	DiskCleanupEnabled  bool    `json:"disk_cleanup_enabled"`
	DiskCleanupInterval time.Duration `json:"disk_cleanup_interval"`

	// Graceful degradation
	DegradationEnabled bool    `json:"degradation_enabled"`
	DegradationSteps   []DegradationStep `json:"degradation_steps"`
}

// DegradationStep defines a step in graceful degradation
type DegradationStep struct {
	Threshold   float64 `json:"threshold"`   // Resource usage threshold (0.0-1.0)
	Action      string  `json:"action"`     // Action to take (e.g., "reduce_workers", "disable_compression")
	Severity    string  `json:"severity"`   // Severity level (low, medium, high, critical)
	Description string  `json:"description"`
}

// AutoTuneConfig contains auto-tuning settings
type AutoTuneConfig struct {
	Enabled           bool          `json:"enabled"`
	TuningInterval    time.Duration `json:"tuning_interval"`
	LearningEnabled   bool          `json:"learning_enabled"`
	SafetyMode        bool          `json:"safety_mode"`        // Prevents aggressive changes
	MaxChangePercent  float64       `json:"max_change_percent"` // Maximum percentage change per tuning cycle
	
	// Machine learning settings
	MLModelPath       string        `json:"ml_model_path"`
	TrainingEnabled   bool          `json:"training_enabled"`
	TrainingInterval  time.Duration `json:"training_interval"`
}

// DefaultConfig returns a configuration with sensible defaults for high-load scenarios
func DefaultConfig() *AdaptiveConfig {
	return &AdaptiveConfig{
		Bitswap: &BitswapConfig{
			MaxOutstandingBytesPerPeer:    100 << 20, // 100MB
			MinOutstandingBytesPerPeer:    1 << 20,   // 1MB
			MinWorkerCount:                128,
			MaxWorkerCount:                2048,
			HighPriorityThreshold:         50 * time.Millisecond,
			CriticalPriorityThreshold:     10 * time.Millisecond,
			BatchSize:                     500,
			BatchTimeout:                  10 * time.Millisecond,
			CircuitBreakerEnabled:         true,
			CircuitBreakerFailureRate:     0.5,
			CircuitBreakerRecoveryTimeout: 30 * time.Second,
		},
		Blockstore: &BlockstoreConfig{
			MemoryCacheSize:    2 << 30,  // 2GB
			SSDCacheSize:       50 << 30, // 50GB
			HDDCacheSize:       500 << 30, // 500GB
			BatchSize:          1000,
			BatchTimeout:       100 * time.Millisecond,
			CompressionEnabled: true,
			CompressionLevel:   6,
			StreamingThreshold: 1 << 20, // 1MB
		},
		Network: &NetworkConfig{
			HighWater:              2000,
			LowWater:               1000,
			GracePeriod:            30 * time.Second,
			LatencyThreshold:       200 * time.Millisecond,
			BandwidthThreshold:     1 << 20, // 1MB/s
			ErrorRateThreshold:     0.05,    // 5%
			KeepAliveInterval:      30 * time.Second,
			KeepAliveTimeout:       10 * time.Second,
			AdaptiveBufferSizing:   true,
			MinBufferSize:          4 << 10,  // 4KB
			MaxBufferSize:          1 << 20,  // 1MB
		},
		Monitoring: &MonitoringConfig{
			MetricsEnabled:            true,
			MetricsInterval:           10 * time.Second,
			PrometheusEnabled:         true,
			PrometheusPort:            9090,
			BottleneckAnalysisEnabled: true,
			AnalysisInterval:          60 * time.Second,
			AlertingEnabled:           true,
			AlertThresholds: AlertConfig{
				ResponseTimeP95:    100 * time.Millisecond,
				ErrorRate:          0.05, // 5%
				MemoryUsage:        0.8,  // 80%
				CPUUsage:           0.8,  // 80%
				DiskUsage:          0.9,  // 90%
				ConnectionFailures: 10,
			},
			AlertCooldown:     5 * time.Minute,
			StructuredLogging: true,
			LogLevel:          "info",
			TraceEnabled:      false,
		},
		Resources: &ResourceConfig{
			MaxMemoryUsage:      8 << 30, // 8GB
			MemoryPressureLevel: 0.8,     // 80%
			GCPressureThreshold: 0.9,     // 90%
			MaxCPUUsage:         0.8,     // 80%
			CPUThrottleEnabled:  true,
			MaxDiskUsage:        0.9, // 90%
			DiskCleanupEnabled:  true,
			DiskCleanupInterval: 1 * time.Hour,
			DegradationEnabled:  true,
			DegradationSteps: []DegradationStep{
				{Threshold: 0.7, Action: "reduce_batch_size", Severity: "low", Description: "Reduce batch processing size"},
				{Threshold: 0.8, Action: "reduce_workers", Severity: "medium", Description: "Reduce worker pool size"},
				{Threshold: 0.9, Action: "disable_compression", Severity: "high", Description: "Disable compression to save CPU"},
				{Threshold: 0.95, Action: "emergency_mode", Severity: "critical", Description: "Enter emergency mode with minimal features"},
			},
		},
		AutoTune: &AutoTuneConfig{
			Enabled:          true,
			TuningInterval:   5 * time.Minute,
			LearningEnabled:  false, // Disabled by default for safety
			SafetyMode:       true,
			MaxChangePercent: 0.1, // 10% max change per cycle
			MLModelPath:      "",
			TrainingEnabled:  false,
			TrainingInterval: 24 * time.Hour,
		},
	}
}

// Validate checks if the configuration is valid
func (c *AdaptiveConfig) Validate() error {
	c.mu.RLock()
	defer c.mu.RUnlock()

	var errs []error

	// Validate Bitswap config
	if c.Bitswap != nil {
		if err := c.validateBitswapConfig(); err != nil {
			errs = append(errs, fmt.Errorf("bitswap config: %w", err))
		}
	}

	// Validate Blockstore config
	if c.Blockstore != nil {
		if err := c.validateBlockstoreConfig(); err != nil {
			errs = append(errs, fmt.Errorf("blockstore config: %w", err))
		}
	}

	// Validate Network config
	if c.Network != nil {
		if err := c.validateNetworkConfig(); err != nil {
			errs = append(errs, fmt.Errorf("network config: %w", err))
		}
	}

	// Validate Monitoring config
	if c.Monitoring != nil {
		if err := c.validateMonitoringConfig(); err != nil {
			errs = append(errs, fmt.Errorf("monitoring config: %w", err))
		}
	}

	// Validate Resources config
	if c.Resources != nil {
		if err := c.validateResourcesConfig(); err != nil {
			errs = append(errs, fmt.Errorf("resources config: %w", err))
		}
	}

	// Validate AutoTune config
	if c.AutoTune != nil {
		if err := c.validateAutoTuneConfig(); err != nil {
			errs = append(errs, fmt.Errorf("auto-tune config: %w", err))
		}
	}

	if len(errs) > 0 {
		return fmt.Errorf("configuration validation failed: %v", errs)
	}

	return nil
}

func (c *AdaptiveConfig) validateBitswapConfig() error {
	bc := c.Bitswap
	
	if bc.MaxOutstandingBytesPerPeer <= bc.MinOutstandingBytesPerPeer {
		return errors.New("max outstanding bytes must be greater than min outstanding bytes")
	}
	
	if bc.MinOutstandingBytesPerPeer < 256<<10 { // 256KB minimum
		return errors.New("min outstanding bytes per peer too small (minimum 256KB)")
	}
	
	if bc.MaxWorkerCount <= bc.MinWorkerCount {
		return errors.New("max worker count must be greater than min worker count")
	}
	
	if bc.MinWorkerCount < 1 {
		return errors.New("min worker count must be at least 1")
	}
	
	if bc.BatchSize < 1 || bc.BatchSize > 10000 {
		return errors.New("batch size must be between 1 and 10000")
	}
	
	if bc.BatchTimeout < time.Millisecond || bc.BatchTimeout > time.Second {
		return errors.New("batch timeout must be between 1ms and 1s")
	}
	
	if bc.CircuitBreakerFailureRate < 0 || bc.CircuitBreakerFailureRate > 1 {
		return errors.New("circuit breaker failure rate must be between 0 and 1")
	}
	
	return nil
}

func (c *AdaptiveConfig) validateBlockstoreConfig() error {
	bc := c.Blockstore
	
	if bc.MemoryCacheSize < 0 || bc.SSDCacheSize < 0 || bc.HDDCacheSize < 0 {
		return errors.New("cache sizes cannot be negative")
	}
	
	if bc.BatchSize < 1 || bc.BatchSize > 100000 {
		return errors.New("batch size must be between 1 and 100000")
	}
	
	if bc.CompressionLevel < 1 || bc.CompressionLevel > 9 {
		return errors.New("compression level must be between 1 and 9")
	}
	
	if bc.StreamingThreshold < 0 {
		return errors.New("streaming threshold cannot be negative")
	}
	
	return nil
}

func (c *AdaptiveConfig) validateNetworkConfig() error {
	nc := c.Network
	
	if nc.HighWater <= nc.LowWater {
		return errors.New("high water mark must be greater than low water mark")
	}
	
	if nc.LowWater < 1 {
		return errors.New("low water mark must be at least 1")
	}
	
	if nc.ErrorRateThreshold < 0 || nc.ErrorRateThreshold > 1 {
		return errors.New("error rate threshold must be between 0 and 1")
	}
	
	if nc.MinBufferSize <= 0 || nc.MaxBufferSize <= nc.MinBufferSize {
		return errors.New("invalid buffer size configuration")
	}
	
	return nil
}

func (c *AdaptiveConfig) validateMonitoringConfig() error {
	mc := c.Monitoring
	
	if mc.PrometheusPort < 1 || mc.PrometheusPort > 65535 {
		return errors.New("prometheus port must be between 1 and 65535")
	}
	
	if mc.AlertThresholds.ErrorRate < 0 || mc.AlertThresholds.ErrorRate > 1 {
		return errors.New("alert error rate threshold must be between 0 and 1")
	}
	
	if mc.AlertThresholds.MemoryUsage < 0 || mc.AlertThresholds.MemoryUsage > 1 {
		return errors.New("alert memory usage threshold must be between 0 and 1")
	}
	
	return nil
}

func (c *AdaptiveConfig) validateResourcesConfig() error {
	rc := c.Resources
	
	if rc.MemoryPressureLevel < 0 || rc.MemoryPressureLevel > 1 {
		return errors.New("memory pressure level must be between 0 and 1")
	}
	
	if rc.MaxCPUUsage < 0 || rc.MaxCPUUsage > 1 {
		return errors.New("max CPU usage must be between 0 and 1")
	}
	
	if rc.MaxDiskUsage < 0 || rc.MaxDiskUsage > 1 {
		return errors.New("max disk usage must be between 0 and 1")
	}
	
	// Validate degradation steps
	for i, step := range rc.DegradationSteps {
		if step.Threshold < 0 || step.Threshold > 1 {
			return fmt.Errorf("degradation step %d: threshold must be between 0 and 1", i)
		}
		if step.Action == "" {
			return fmt.Errorf("degradation step %d: action cannot be empty", i)
		}
	}
	
	return nil
}

func (c *AdaptiveConfig) validateAutoTuneConfig() error {
	ac := c.AutoTune
	
	if ac.MaxChangePercent < 0 || ac.MaxChangePercent > 1 {
		return errors.New("max change percent must be between 0 and 1")
	}
	
	if ac.TuningInterval < time.Second {
		return errors.New("tuning interval must be at least 1 second")
	}
	
	return nil
}

// Clone creates a deep copy of the configuration
func (c *AdaptiveConfig) Clone() *AdaptiveConfig {
	c.mu.RLock()
	defer c.mu.RUnlock()
	
	// Create a new config with copied values
	clone := &AdaptiveConfig{}
	
	if c.Bitswap != nil {
		bitswap := *c.Bitswap
		clone.Bitswap = &bitswap
	}
	
	if c.Blockstore != nil {
		blockstore := *c.Blockstore
		clone.Blockstore = &blockstore
	}
	
	if c.Network != nil {
		network := *c.Network
		clone.Network = &network
	}
	
	if c.Monitoring != nil {
		monitoring := *c.Monitoring
		monitoring.AlertThresholds = c.Monitoring.AlertThresholds // Copy struct
		clone.Monitoring = &monitoring
	}
	
	if c.Resources != nil {
		resources := *c.Resources
		// Deep copy degradation steps
		if len(c.Resources.DegradationSteps) > 0 {
			resources.DegradationSteps = make([]DegradationStep, len(c.Resources.DegradationSteps))
			copy(resources.DegradationSteps, c.Resources.DegradationSteps)
		}
		clone.Resources = &resources
	}
	
	if c.AutoTune != nil {
		autoTune := *c.AutoTune
		clone.AutoTune = &autoTune
	}
	
	return clone
}

// Update safely updates the configuration with new values
func (c *AdaptiveConfig) Update(newConfig *AdaptiveConfig) error {
	if err := newConfig.Validate(); err != nil {
		return fmt.Errorf("invalid configuration: %w", err)
	}
	
	c.mu.Lock()
	defer c.mu.Unlock()
	
	// Update each section if provided
	if newConfig.Bitswap != nil {
		c.Bitswap = newConfig.Bitswap
	}
	if newConfig.Blockstore != nil {
		c.Blockstore = newConfig.Blockstore
	}
	if newConfig.Network != nil {
		c.Network = newConfig.Network
	}
	if newConfig.Monitoring != nil {
		c.Monitoring = newConfig.Monitoring
	}
	if newConfig.Resources != nil {
		c.Resources = newConfig.Resources
	}
	if newConfig.AutoTune != nil {
		c.AutoTune = newConfig.AutoTune
	}
	
	return nil
}