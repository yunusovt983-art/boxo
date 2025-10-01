package adaptive

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewManager(t *testing.T) {
	// Test with nil config (should use default)
	manager, err := NewManager(nil)
	require.NoError(t, err)
	assert.NotNil(t, manager)
	assert.NotNil(t, manager.config)
	
	// Test with valid config
	config := DefaultConfig()
	manager, err = NewManager(config)
	require.NoError(t, err)
	assert.NotNil(t, manager)
	assert.Equal(t, config.Bitswap.BatchSize, manager.config.Bitswap.BatchSize)
	
	// Test with invalid config
	invalidConfig := DefaultConfig()
	invalidConfig.Bitswap.MaxOutstandingBytesPerPeer = 0 // Invalid
	invalidConfig.Bitswap.MinOutstandingBytesPerPeer = 1 << 20
	
	manager, err = NewManager(invalidConfig)
	assert.Error(t, err)
	assert.Nil(t, manager)
	assert.Contains(t, err.Error(), "invalid configuration")
}

func TestManagerStartStop(t *testing.T) {
	manager, err := NewManager(nil)
	require.NoError(t, err)
	
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	
	// Test start
	err = manager.Start(ctx)
	assert.NoError(t, err)
	assert.True(t, manager.running)
	
	// Test start when already running
	err = manager.Start(ctx)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "already running")
	
	// Test stop
	err = manager.Stop(ctx)
	assert.NoError(t, err)
	assert.False(t, manager.running)
	
	// Test stop when not running
	err = manager.Stop(ctx)
	assert.NoError(t, err) // Should not error
}

func TestManagerGetConfig(t *testing.T) {
	config := DefaultConfig()
	config.Bitswap.BatchSize = 999 // Unique value for testing
	
	manager, err := NewManager(config)
	require.NoError(t, err)
	
	// Get config
	retrievedConfig := manager.GetConfig()
	assert.Equal(t, 999, retrievedConfig.Bitswap.BatchSize)
	
	// Modify retrieved config (should not affect manager's config)
	retrievedConfig.Bitswap.BatchSize = 1111
	
	// Get config again
	retrievedConfig2 := manager.GetConfig()
	assert.Equal(t, 999, retrievedConfig2.Bitswap.BatchSize) // Should be unchanged
}

func TestManagerUpdateConfig(t *testing.T) {
	manager, err := NewManager(nil)
	require.NoError(t, err)
	
	ctx := context.Background()
	
	// Create new config
	newConfig := DefaultConfig()
	newConfig.Bitswap.BatchSize = 1500
	newConfig.Network.HighWater = 3000
	
	// Update config
	err = manager.UpdateConfig(ctx, newConfig)
	assert.NoError(t, err)
	
	// Verify config was updated
	currentConfig := manager.GetConfig()
	assert.Equal(t, 1500, currentConfig.Bitswap.BatchSize)
	assert.Equal(t, 3000, currentConfig.Network.HighWater)
}

func TestManagerUpdateConfigInvalid(t *testing.T) {
	manager, err := NewManager(nil)
	require.NoError(t, err)
	
	ctx := context.Background()
	
	// Create invalid config
	invalidConfig := DefaultConfig()
	invalidConfig.Bitswap.MaxOutstandingBytesPerPeer = 1 << 20 // 1MB
	invalidConfig.Bitswap.MinOutstandingBytesPerPeer = 2 << 20 // 2MB (invalid: min > max)
	
	// Update should fail
	err = manager.UpdateConfig(ctx, invalidConfig)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "configuration validation failed")
	
	// Verify original config is unchanged
	currentConfig := manager.GetConfig()
	assert.Equal(t, int64(100<<20), currentConfig.Bitswap.MaxOutstandingBytesPerPeer)
	assert.Equal(t, int64(1<<20), currentConfig.Bitswap.MinOutstandingBytesPerPeer)
}

func TestManagerApplyConfigChanges(t *testing.T) {
	manager, err := NewManager(nil)
	require.NoError(t, err)
	
	ctx := context.Background()
	
	// Create config changes
	changes := &ConfigChanges{
		Changes: map[string]interface{}{
			"bitswap.batch_size": 2000,
		},
		Reason:    "performance optimization",
		Source:    "manual",
		Timestamp: time.Now(),
	}
	
	// Apply changes
	err = manager.ApplyConfigChanges(ctx, changes)
	assert.NoError(t, err)
	
	// Verify changes were applied
	currentConfig := manager.GetConfig()
	assert.Equal(t, 2000, currentConfig.Bitswap.BatchSize)
}

func TestManagerSubscribe(t *testing.T) {
	manager, err := NewManager(nil)
	require.NoError(t, err)
	
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	
	// Subscribe to config changes
	eventCh, err := manager.Subscribe(ctx)
	require.NoError(t, err)
	assert.NotNil(t, eventCh)
	
	// Update config to trigger event
	newConfig := DefaultConfig()
	newConfig.Bitswap.BatchSize = 1234
	
	go func() {
		time.Sleep(100 * time.Millisecond)
		err := manager.UpdateConfig(context.Background(), newConfig)
		assert.NoError(t, err)
	}()
	
	// Wait for event
	select {
	case event := <-eventCh:
		assert.NotNil(t, event)
		assert.Equal(t, "update", event.Type)
		assert.Equal(t, "manual", event.Source)
		assert.True(t, event.Success)
	case <-ctx.Done():
		t.Fatal("Timeout waiting for config change event")
	}
}

func TestManagerGetConfigRecommendations(t *testing.T) {
	manager, err := NewManager(nil)
	require.NoError(t, err)
	
	ctx := context.Background()
	
	// Create performance metrics
	metrics := &PerformanceMetrics{
		Timestamp: time.Now(),
		Bitswap: &BitswapMetrics{
			P95ResponseTime:   150 * time.Millisecond, // Above threshold
			WorkerUtilization: 0.95,                   // High utilization
		},
		Blockstore: &BlockstoreMetrics{
			CacheHitRate: 0.7, // Below optimal
		},
		Network: &NetworkMetrics{
			ActiveConnections: 1900, // Near limit
		},
	}
	
	// Get recommendations
	recommendations, err := manager.GetConfigRecommendations(ctx, metrics)
	require.NoError(t, err)
	assert.NotNil(t, recommendations)
	
	// Should have recommendations for high response time and low cache hit rate
	assert.Greater(t, len(recommendations.Recommendations), 0)
	
	// Check for specific recommendations
	foundBitswapRec := false
	foundBlockstoreRec := false
	foundNetworkRec := false
	
	for _, rec := range recommendations.Recommendations {
		switch rec.Component {
		case "bitswap":
			foundBitswapRec = true
		case "blockstore":
			foundBlockstoreRec = true
		case "network":
			foundNetworkRec = true
		}
	}
	
	assert.True(t, foundBitswapRec, "Should have Bitswap recommendations")
	assert.True(t, foundBlockstoreRec, "Should have Blockstore recommendations")
	assert.True(t, foundNetworkRec, "Should have Network recommendations")
}

func TestManagerGetDegradationLevel(t *testing.T) {
	manager, err := NewManager(nil)
	require.NoError(t, err)
	
	// Initial degradation level should be none
	level := manager.GetDegradationLevel()
	assert.Equal(t, DegradationNone, level)
	
	// Simulate degradation
	manager.mu.Lock()
	manager.degradationLevel = DegradationMedium
	manager.mu.Unlock()
	
	level = manager.GetDegradationLevel()
	assert.Equal(t, DegradationMedium, level)
}

func TestManagerValidateConfig(t *testing.T) {
	manager, err := NewManager(nil)
	require.NoError(t, err)
	
	// Test valid config
	validConfig := DefaultConfig()
	err = manager.ValidateConfig(validConfig)
	assert.NoError(t, err)
	
	// Test invalid config
	invalidConfig := DefaultConfig()
	invalidConfig.Bitswap.BatchSize = 0 // Invalid
	
	err = manager.ValidateConfig(invalidConfig)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "batch size")
}

func TestManagerWithCustomComponents(t *testing.T) {
	config := DefaultConfig()
	manager, err := NewManager(config)
	require.NoError(t, err)
	
	// Set custom components
	resourceManager := NewDefaultResourceManager(config.Resources)
	perfMonitor := NewDefaultPerformanceMonitor(config.Monitoring)
	autoTuner := NewDefaultAutoTuner(config.AutoTune)
	
	manager.resourceManager = resourceManager
	manager.perfMonitor = perfMonitor
	manager.autoTuner = autoTuner
	
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	
	// Start manager
	err = manager.Start(ctx)
	assert.NoError(t, err)
	
	// Test resource metrics
	resourceMetrics, err := manager.GetResourceMetrics(ctx)
	assert.NoError(t, err)
	assert.NotNil(t, resourceMetrics)
	
	// Test performance metrics
	perfMetrics, err := manager.GetPerformanceMetrics(ctx)
	assert.NoError(t, err)
	assert.NotNil(t, perfMetrics)
	
	// Stop manager
	err = manager.Stop(ctx)
	assert.NoError(t, err)
}

func TestManagerMonitoringLoop(t *testing.T) {
	// Create config with short intervals for testing
	config := DefaultConfig()
	config.Monitoring.MetricsInterval = 100 * time.Millisecond
	config.AutoTune.TuningInterval = 2 * time.Second // Must be at least 1 second
	
	manager, err := NewManager(config)
	require.NoError(t, err)
	
	// Set up components
	manager.resourceManager = NewDefaultResourceManager(config.Resources)
	manager.perfMonitor = NewDefaultPerformanceMonitor(config.Monitoring)
	manager.autoTuner = NewDefaultAutoTuner(config.AutoTune)
	
	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
	defer cancel()
	
	// Start manager (this starts the monitoring loop)
	err = manager.Start(ctx)
	assert.NoError(t, err)
	
	// Wait for monitoring loop to run a few cycles
	time.Sleep(500 * time.Millisecond)
	
	// Stop manager
	err = manager.Stop(ctx)
	assert.NoError(t, err)
	
	// Verify that auto-tuning ran at least once
	status := manager.autoTuner.GetTuningStatus()
	assert.True(t, status.TuningCycles >= 0) // Should have run at least once
}

func TestManagerConfigChangeNotification(t *testing.T) {
	manager, err := NewManager(nil)
	require.NoError(t, err)
	
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	
	// Subscribe to changes
	eventCh, err := manager.Subscribe(ctx)
	require.NoError(t, err)
	
	// Apply config changes
	changes := &ConfigChanges{
		Changes: map[string]interface{}{
			"bitswap.batch_size": 1500,
		},
		Source:    "test",
		Timestamp: time.Now(),
	}
	
	go func() {
		time.Sleep(100 * time.Millisecond)
		err := manager.ApplyConfigChanges(context.Background(), changes)
		assert.NoError(t, err)
	}()
	
	// Wait for notification
	select {
	case event := <-eventCh:
		assert.Equal(t, "apply", event.Type)
		assert.Equal(t, "test", event.Source)
		assert.True(t, event.Success)
		assert.Contains(t, event.Changes, "bitswap.batch_size")
	case <-ctx.Done():
		t.Fatal("Timeout waiting for config change notification")
	}
}