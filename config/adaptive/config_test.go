package adaptive

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDefaultConfig(t *testing.T) {
	config := DefaultConfig()
	
	// Test that default config is valid
	err := config.Validate()
	assert.NoError(t, err, "Default configuration should be valid")
	
	// Test default values
	assert.NotNil(t, config.Bitswap)
	assert.NotNil(t, config.Blockstore)
	assert.NotNil(t, config.Network)
	assert.NotNil(t, config.Monitoring)
	assert.NotNil(t, config.Resources)
	assert.NotNil(t, config.AutoTune)
	
	// Test Bitswap defaults
	assert.Equal(t, int64(100<<20), config.Bitswap.MaxOutstandingBytesPerPeer)
	assert.Equal(t, int64(1<<20), config.Bitswap.MinOutstandingBytesPerPeer)
	assert.Equal(t, 128, config.Bitswap.MinWorkerCount)
	assert.Equal(t, 2048, config.Bitswap.MaxWorkerCount)
	assert.True(t, config.Bitswap.CircuitBreakerEnabled)
	
	// Test Blockstore defaults
	assert.Equal(t, int64(2<<30), config.Blockstore.MemoryCacheSize)
	assert.True(t, config.Blockstore.CompressionEnabled)
	assert.Equal(t, 6, config.Blockstore.CompressionLevel)
	
	// Test Network defaults
	assert.Equal(t, 2000, config.Network.HighWater)
	assert.Equal(t, 1000, config.Network.LowWater)
	assert.True(t, config.Network.AdaptiveBufferSizing)
	
	// Test Monitoring defaults
	assert.True(t, config.Monitoring.MetricsEnabled)
	assert.True(t, config.Monitoring.PrometheusEnabled)
	assert.Equal(t, 9090, config.Monitoring.PrometheusPort)
	
	// Test Resources defaults
	assert.True(t, config.Resources.DegradationEnabled)
	assert.Len(t, config.Resources.DegradationSteps, 4)
	
	// Test AutoTune defaults
	assert.True(t, config.AutoTune.Enabled)
	assert.True(t, config.AutoTune.SafetyMode)
	assert.False(t, config.AutoTune.LearningEnabled)
}

func TestBitswapConfigValidation(t *testing.T) {
	tests := []struct {
		name        string
		config      *BitswapConfig
		expectError bool
		errorMsg    string
	}{
		{
			name: "valid config",
			config: &BitswapConfig{
				MaxOutstandingBytesPerPeer: 10 << 20, // 10MB
				MinOutstandingBytesPerPeer: 1 << 20,  // 1MB
				MinWorkerCount:             10,
				MaxWorkerCount:             100,
				BatchSize:                  100,
				BatchTimeout:               10 * time.Millisecond,
				CircuitBreakerFailureRate:  0.5,
			},
			expectError: false,
		},
		{
			name: "max < min outstanding bytes",
			config: &BitswapConfig{
				MaxOutstandingBytesPerPeer: 1 << 20, // 1MB
				MinOutstandingBytesPerPeer: 2 << 20, // 2MB
				MinWorkerCount:             10,
				MaxWorkerCount:             100,
				BatchSize:                  100,
				BatchTimeout:               10 * time.Millisecond,
				CircuitBreakerFailureRate:  0.5,
			},
			expectError: true,
			errorMsg:    "max outstanding bytes must be greater than min outstanding bytes",
		},
		{
			name: "min outstanding bytes too small",
			config: &BitswapConfig{
				MaxOutstandingBytesPerPeer: 10 << 20, // 10MB
				MinOutstandingBytesPerPeer: 100 << 10, // 100KB (too small)
				MinWorkerCount:             10,
				MaxWorkerCount:             100,
				BatchSize:                  100,
				BatchTimeout:               10 * time.Millisecond,
				CircuitBreakerFailureRate:  0.5,
			},
			expectError: true,
			errorMsg:    "min outstanding bytes per peer too small",
		},
		{
			name: "max workers <= min workers",
			config: &BitswapConfig{
				MaxOutstandingBytesPerPeer: 10 << 20, // 10MB
				MinOutstandingBytesPerPeer: 1 << 20,  // 1MB
				MinWorkerCount:             100,
				MaxWorkerCount:             50, // Less than min
				BatchSize:                  100,
				BatchTimeout:               10 * time.Millisecond,
				CircuitBreakerFailureRate:  0.5,
			},
			expectError: true,
			errorMsg:    "max worker count must be greater than min worker count",
		},
		{
			name: "invalid batch size",
			config: &BitswapConfig{
				MaxOutstandingBytesPerPeer: 10 << 20, // 10MB
				MinOutstandingBytesPerPeer: 1 << 20,  // 1MB
				MinWorkerCount:             10,
				MaxWorkerCount:             100,
				BatchSize:                  0, // Invalid
				BatchTimeout:               10 * time.Millisecond,
				CircuitBreakerFailureRate:  0.5,
			},
			expectError: true,
			errorMsg:    "batch size must be between 1 and 10000",
		},
		{
			name: "invalid batch timeout",
			config: &BitswapConfig{
				MaxOutstandingBytesPerPeer: 10 << 20, // 10MB
				MinOutstandingBytesPerPeer: 1 << 20,  // 1MB
				MinWorkerCount:             10,
				MaxWorkerCount:             100,
				BatchSize:                  100,
				BatchTimeout:               0, // Invalid
				CircuitBreakerFailureRate:  0.5,
			},
			expectError: true,
			errorMsg:    "batch timeout must be between 1ms and 1s",
		},
		{
			name: "invalid circuit breaker failure rate",
			config: &BitswapConfig{
				MaxOutstandingBytesPerPeer: 10 << 20, // 10MB
				MinOutstandingBytesPerPeer: 1 << 20,  // 1MB
				MinWorkerCount:             10,
				MaxWorkerCount:             100,
				BatchSize:                  100,
				BatchTimeout:               10 * time.Millisecond,
				CircuitBreakerFailureRate:  1.5, // Invalid (> 1.0)
			},
			expectError: true,
			errorMsg:    "circuit breaker failure rate must be between 0 and 1",
		},
	}
	
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			config := &AdaptiveConfig{Bitswap: tt.config}
			err := config.validateBitswapConfig()
			
			if tt.expectError {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), tt.errorMsg)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestBlockstoreConfigValidation(t *testing.T) {
	tests := []struct {
		name        string
		config      *BlockstoreConfig
		expectError bool
		errorMsg    string
	}{
		{
			name: "valid config",
			config: &BlockstoreConfig{
				MemoryCacheSize:    1 << 30, // 1GB
				SSDCacheSize:       10 << 30, // 10GB
				HDDCacheSize:       100 << 30, // 100GB
				BatchSize:          1000,
				BatchTimeout:       100 * time.Millisecond,
				CompressionEnabled: true,
				CompressionLevel:   6,
				StreamingThreshold: 1 << 20, // 1MB
			},
			expectError: false,
		},
		{
			name: "negative cache size",
			config: &BlockstoreConfig{
				MemoryCacheSize:    -1, // Invalid
				SSDCacheSize:       10 << 30,
				HDDCacheSize:       100 << 30,
				BatchSize:          1000,
				CompressionLevel:   6,
				StreamingThreshold: 1 << 20,
			},
			expectError: true,
			errorMsg:    "cache sizes cannot be negative",
		},
		{
			name: "invalid batch size",
			config: &BlockstoreConfig{
				MemoryCacheSize:    1 << 30,
				SSDCacheSize:       10 << 30,
				HDDCacheSize:       100 << 30,
				BatchSize:          0, // Invalid
				CompressionLevel:   6,
				StreamingThreshold: 1 << 20,
			},
			expectError: true,
			errorMsg:    "batch size must be between 1 and 100000",
		},
		{
			name: "invalid compression level",
			config: &BlockstoreConfig{
				MemoryCacheSize:    1 << 30,
				SSDCacheSize:       10 << 30,
				HDDCacheSize:       100 << 30,
				BatchSize:          1000,
				CompressionLevel:   10, // Invalid (> 9)
				StreamingThreshold: 1 << 20,
			},
			expectError: true,
			errorMsg:    "compression level must be between 1 and 9",
		},
		{
			name: "negative streaming threshold",
			config: &BlockstoreConfig{
				MemoryCacheSize:    1 << 30,
				SSDCacheSize:       10 << 30,
				HDDCacheSize:       100 << 30,
				BatchSize:          1000,
				CompressionLevel:   6,
				StreamingThreshold: -1, // Invalid
			},
			expectError: true,
			errorMsg:    "streaming threshold cannot be negative",
		},
	}
	
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			config := &AdaptiveConfig{Blockstore: tt.config}
			err := config.validateBlockstoreConfig()
			
			if tt.expectError {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), tt.errorMsg)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestNetworkConfigValidation(t *testing.T) {
	tests := []struct {
		name        string
		config      *NetworkConfig
		expectError bool
		errorMsg    string
	}{
		{
			name: "valid config",
			config: &NetworkConfig{
				HighWater:            2000,
				LowWater:             1000,
				GracePeriod:          30 * time.Second,
				LatencyThreshold:     200 * time.Millisecond,
				BandwidthThreshold:   1 << 20, // 1MB/s
				ErrorRateThreshold:   0.05,    // 5%
				KeepAliveInterval:    30 * time.Second,
				KeepAliveTimeout:     10 * time.Second,
				AdaptiveBufferSizing: true,
				MinBufferSize:        4 << 10, // 4KB
				MaxBufferSize:        1 << 20, // 1MB
			},
			expectError: false,
		},
		{
			name: "high water <= low water",
			config: &NetworkConfig{
				HighWater:          1000,
				LowWater:           2000, // Greater than high water
				ErrorRateThreshold: 0.05,
				MinBufferSize:      4 << 10,
				MaxBufferSize:      1 << 20,
			},
			expectError: true,
			errorMsg:    "high water mark must be greater than low water mark",
		},
		{
			name: "low water too small",
			config: &NetworkConfig{
				HighWater:          2000,
				LowWater:           0, // Invalid
				ErrorRateThreshold: 0.05,
				MinBufferSize:      4 << 10,
				MaxBufferSize:      1 << 20,
			},
			expectError: true,
			errorMsg:    "low water mark must be at least 1",
		},
		{
			name: "invalid error rate threshold",
			config: &NetworkConfig{
				HighWater:          2000,
				LowWater:           1000,
				ErrorRateThreshold: 1.5, // Invalid (> 1.0)
				MinBufferSize:      4 << 10,
				MaxBufferSize:      1 << 20,
			},
			expectError: true,
			errorMsg:    "error rate threshold must be between 0 and 1",
		},
		{
			name: "invalid buffer sizes",
			config: &NetworkConfig{
				HighWater:          2000,
				LowWater:           1000,
				ErrorRateThreshold: 0.05,
				MinBufferSize:      1 << 20, // Larger than max
				MaxBufferSize:      4 << 10,
			},
			expectError: true,
			errorMsg:    "invalid buffer size configuration",
		},
	}
	
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			config := &AdaptiveConfig{Network: tt.config}
			err := config.validateNetworkConfig()
			
			if tt.expectError {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), tt.errorMsg)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestResourceConfigValidation(t *testing.T) {
	tests := []struct {
		name        string
		config      *ResourceConfig
		expectError bool
		errorMsg    string
	}{
		{
			name: "valid config",
			config: &ResourceConfig{
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
					{Threshold: 0.7, Action: "reduce_batch_size", Severity: "low"},
					{Threshold: 0.8, Action: "reduce_workers", Severity: "medium"},
				},
			},
			expectError: false,
		},
		{
			name: "invalid memory pressure level",
			config: &ResourceConfig{
				MemoryPressureLevel: 1.5, // Invalid (> 1.0)
				MaxCPUUsage:         0.8,
				MaxDiskUsage:        0.9,
				DegradationSteps: []DegradationStep{
					{Threshold: 0.7, Action: "reduce_batch_size", Severity: "low"},
				},
			},
			expectError: true,
			errorMsg:    "memory pressure level must be between 0 and 1",
		},
		{
			name: "invalid CPU usage",
			config: &ResourceConfig{
				MemoryPressureLevel: 0.8,
				MaxCPUUsage:         -0.1, // Invalid (< 0)
				MaxDiskUsage:        0.9,
				DegradationSteps: []DegradationStep{
					{Threshold: 0.7, Action: "reduce_batch_size", Severity: "low"},
				},
			},
			expectError: true,
			errorMsg:    "max CPU usage must be between 0 and 1",
		},
		{
			name: "invalid degradation step threshold",
			config: &ResourceConfig{
				MemoryPressureLevel: 0.8,
				MaxCPUUsage:         0.8,
				MaxDiskUsage:        0.9,
				DegradationSteps: []DegradationStep{
					{Threshold: 1.5, Action: "reduce_batch_size", Severity: "low"}, // Invalid threshold
				},
			},
			expectError: true,
			errorMsg:    "degradation step 0: threshold must be between 0 and 1",
		},
		{
			name: "empty degradation step action",
			config: &ResourceConfig{
				MemoryPressureLevel: 0.8,
				MaxCPUUsage:         0.8,
				MaxDiskUsage:        0.9,
				DegradationSteps: []DegradationStep{
					{Threshold: 0.7, Action: "", Severity: "low"}, // Empty action
				},
			},
			expectError: true,
			errorMsg:    "degradation step 0: action cannot be empty",
		},
	}
	
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			config := &AdaptiveConfig{Resources: tt.config}
			err := config.validateResourcesConfig()
			
			if tt.expectError {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), tt.errorMsg)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestAutoTuneConfigValidation(t *testing.T) {
	tests := []struct {
		name        string
		config      *AutoTuneConfig
		expectError bool
		errorMsg    string
	}{
		{
			name: "valid config",
			config: &AutoTuneConfig{
				Enabled:          true,
				TuningInterval:   5 * time.Minute,
				LearningEnabled:  false,
				SafetyMode:       true,
				MaxChangePercent: 0.1, // 10%
				MLModelPath:      "",
				TrainingEnabled:  false,
				TrainingInterval: 24 * time.Hour,
			},
			expectError: false,
		},
		{
			name: "invalid max change percent",
			config: &AutoTuneConfig{
				Enabled:          true,
				TuningInterval:   5 * time.Minute,
				MaxChangePercent: 1.5, // Invalid (> 1.0)
			},
			expectError: true,
			errorMsg:    "max change percent must be between 0 and 1",
		},
		{
			name: "tuning interval too short",
			config: &AutoTuneConfig{
				Enabled:          true,
				TuningInterval:   500 * time.Millisecond, // Too short
				MaxChangePercent: 0.1,
			},
			expectError: true,
			errorMsg:    "tuning interval must be at least 1 second",
		},
	}
	
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			config := &AdaptiveConfig{AutoTune: tt.config}
			err := config.validateAutoTuneConfig()
			
			if tt.expectError {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), tt.errorMsg)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestConfigClone(t *testing.T) {
	original := DefaultConfig()
	
	// Modify original
	original.Bitswap.BatchSize = 999
	original.Network.HighWater = 9999
	
	// Clone
	clone := original.Clone()
	
	// Verify clone has same values
	assert.Equal(t, original.Bitswap.BatchSize, clone.Bitswap.BatchSize)
	assert.Equal(t, original.Network.HighWater, clone.Network.HighWater)
	
	// Modify clone
	clone.Bitswap.BatchSize = 1111
	clone.Network.HighWater = 1111
	
	// Verify original is unchanged
	assert.Equal(t, 999, original.Bitswap.BatchSize)
	assert.Equal(t, 9999, original.Network.HighWater)
	
	// Verify clone has new values
	assert.Equal(t, 1111, clone.Bitswap.BatchSize)
	assert.Equal(t, 1111, clone.Network.HighWater)
}

func TestConfigUpdate(t *testing.T) {
	config := DefaultConfig()
	
	// Create update with new values
	update := &AdaptiveConfig{
		Bitswap: &BitswapConfig{
			MaxOutstandingBytesPerPeer: 200 << 20, // 200MB
			MinOutstandingBytesPerPeer: 2 << 20,   // 2MB
			MinWorkerCount:             256,
			MaxWorkerCount:             4096,
			BatchSize:                  1000,
			BatchTimeout:               20 * time.Millisecond,
			CircuitBreakerEnabled:      true,
			CircuitBreakerFailureRate:  0.6,
		},
	}
	
	// Apply update
	err := config.Update(update)
	require.NoError(t, err)
	
	// Verify values were updated
	assert.Equal(t, int64(200<<20), config.Bitswap.MaxOutstandingBytesPerPeer)
	assert.Equal(t, int64(2<<20), config.Bitswap.MinOutstandingBytesPerPeer)
	assert.Equal(t, 256, config.Bitswap.MinWorkerCount)
	assert.Equal(t, 4096, config.Bitswap.MaxWorkerCount)
	
	// Verify other sections remain unchanged (not nil)
	assert.NotNil(t, config.Blockstore)
	assert.NotNil(t, config.Network)
	assert.NotNil(t, config.Monitoring)
}

func TestConfigUpdateInvalid(t *testing.T) {
	config := DefaultConfig()
	
	// Create invalid update
	update := &AdaptiveConfig{
		Bitswap: &BitswapConfig{
			MaxOutstandingBytesPerPeer: 1 << 20, // 1MB
			MinOutstandingBytesPerPeer: 2 << 20, // 2MB (invalid: min > max)
			MinWorkerCount:             10,
			MaxWorkerCount:             100,
			BatchSize:                  100,
			BatchTimeout:               10 * time.Millisecond,
		},
	}
	
	// Apply update should fail
	err := config.Update(update)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "invalid configuration")
	
	// Verify original config is unchanged
	assert.Equal(t, int64(100<<20), config.Bitswap.MaxOutstandingBytesPerPeer)
	assert.Equal(t, int64(1<<20), config.Bitswap.MinOutstandingBytesPerPeer)
}

func TestDegradationLevelString(t *testing.T) {
	tests := []struct {
		level    DegradationLevel
		expected string
	}{
		{DegradationNone, "none"},
		{DegradationLow, "low"},
		{DegradationMedium, "medium"},
		{DegradationHigh, "high"},
		{DegradationCritical, "critical"},
		{DegradationLevel(999), "unknown"},
	}
	
	for _, tt := range tests {
		t.Run(tt.expected, func(t *testing.T) {
			assert.Equal(t, tt.expected, tt.level.String())
		})
	}
}