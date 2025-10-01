package adaptive_test

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/ipfs/boxo/config/adaptive"
)

// ExampleManager demonstrates basic usage of the adaptive configuration manager
func ExampleManager() {
	// Create a manager with default configuration
	manager, err := adaptive.NewManager(nil)
	if err != nil {
		log.Fatal(err)
	}

	// Start the adaptive management system
	ctx := context.Background()
	if err := manager.Start(ctx); err != nil {
		log.Fatal(err)
	}
	defer manager.Stop(ctx)

	// Get current configuration
	config := manager.GetConfig()
	fmt.Printf("Current Bitswap batch size: %d\n", config.Bitswap.BatchSize)

	// Update configuration
	newConfig := config.Clone()
	newConfig.Bitswap.BatchSize = 1000
	if err := manager.UpdateConfig(ctx, newConfig); err != nil {
		log.Printf("Failed to update config: %v", err)
		return
	}

	// Verify the update
	updatedConfig := manager.GetConfig()
	fmt.Printf("Updated Bitswap batch size: %d\n", updatedConfig.Bitswap.BatchSize)

	// Output:
	// Current Bitswap batch size: 500
	// Updated Bitswap batch size: 1000
}

// ExampleAdaptiveConfig demonstrates configuration validation
func ExampleAdaptiveConfig_Validate() {
	// Create a configuration with invalid values
	config := adaptive.DefaultConfig()
	config.Bitswap.BatchSize = 0 // Invalid: must be > 0

	// Validate the configuration
	if err := config.Validate(); err != nil {
		fmt.Printf("Configuration validation failed: %v\n", err)
	}

	// Fix the configuration
	config.Bitswap.BatchSize = 100

	// Validate again
	if err := config.Validate(); err != nil {
		fmt.Printf("Still invalid: %v\n", err)
	} else {
		fmt.Println("Configuration is now valid")
	}

	// Output:
	// Configuration validation failed: configuration validation failed: [bitswap config: batch size must be between 1 and 10000]
	// Configuration is now valid
}

// ExampleManager_Subscribe demonstrates subscribing to configuration changes
func ExampleManager_Subscribe() {
	manager, err := adaptive.NewManager(nil)
	if err != nil {
		log.Fatal(err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	if err := manager.Start(ctx); err != nil {
		log.Fatal(err)
	}
	defer manager.Stop(ctx)

	// Subscribe to configuration changes
	eventCh, err := manager.Subscribe(ctx)
	if err != nil {
		log.Fatal(err)
	}

	// Update configuration in a goroutine
	go func() {
		time.Sleep(100 * time.Millisecond)
		config := manager.GetConfig()
		newConfig := config.Clone()
		newConfig.Bitswap.BatchSize = 1500
		manager.UpdateConfig(context.Background(), newConfig)
	}()

	// Wait for the configuration change event
	select {
	case event := <-eventCh:
		fmt.Printf("Configuration changed: %s from %s\n", event.Type, event.Source)
	case <-ctx.Done():
		fmt.Println("Timeout waiting for configuration change")
	}

	// Output:
	// Configuration changed: update from manual
}

// ExampleDefaultConfig demonstrates the default configuration values
func ExampleDefaultConfig() {
	config := adaptive.DefaultConfig()

	fmt.Printf("Bitswap max outstanding bytes per peer: %d MB\n", config.Bitswap.MaxOutstandingBytesPerPeer/(1<<20))
	fmt.Printf("Bitswap max worker count: %d\n", config.Bitswap.MaxWorkerCount)
	fmt.Printf("Blockstore memory cache size: %d GB\n", config.Blockstore.MemoryCacheSize/(1<<30))
	fmt.Printf("Network high water mark: %d\n", config.Network.HighWater)
	fmt.Printf("Auto-tuning enabled: %t\n", config.AutoTune.Enabled)
	fmt.Printf("Graceful degradation enabled: %t\n", config.Resources.DegradationEnabled)

	// Output:
	// Bitswap max outstanding bytes per peer: 100 MB
	// Bitswap max worker count: 2048
	// Blockstore memory cache size: 2 GB
	// Network high water mark: 2000
	// Auto-tuning enabled: true
	// Graceful degradation enabled: true
}

// ExampleManager_GetConfigRecommendations demonstrates getting configuration recommendations
func ExampleManager_GetConfigRecommendations() {
	manager, err := adaptive.NewManager(nil)
	if err != nil {
		log.Fatal(err)
	}

	ctx := context.Background()

	// Create performance metrics that would trigger recommendations
	metrics := &adaptive.PerformanceMetrics{
		Timestamp: time.Now(),
		Bitswap: &adaptive.BitswapMetrics{
			P95ResponseTime:   150 * time.Millisecond, // Above threshold
			WorkerUtilization: 0.95,                   // High utilization
		},
		Blockstore: &adaptive.BlockstoreMetrics{
			CacheHitRate: 0.7, // Below optimal
		},
		Network: &adaptive.NetworkMetrics{
			ActiveConnections: 1900, // Near limit
		},
	}

	// Get recommendations
	recommendations, err := manager.GetConfigRecommendations(ctx, metrics)
	if err != nil {
		log.Fatal(err)
	}

	fmt.Printf("Found %d recommendations\n", len(recommendations.Recommendations))
	for _, rec := range recommendations.Recommendations {
		fmt.Printf("- %s.%s: %v -> %v (%s)\n",
			rec.Component, rec.Parameter, rec.CurrentValue, rec.RecommendedValue, rec.Reason)
	}

	// Output:
	// Found 4 recommendations
	// - bitswap.batch_size: 500 -> 1000 (High P95 response time detected)
	// - bitswap.max_worker_count: 2048 -> 2457 (High worker utilization detected)
	// - blockstore.memory_cache_size: 2147483648 -> 3221225472 (Low cache hit rate detected)
	// - network.high_water: 2000 -> 2400 (Approaching connection limit)
}