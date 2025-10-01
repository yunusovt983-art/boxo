// Package adaptive provides adaptive configuration management for high-load optimization in Boxo.
//
// This package implements a comprehensive system for dynamically managing configuration
// parameters based on real-time performance metrics and resource usage. It supports
// automatic tuning, graceful degradation, and hot reconfiguration without service restarts.
//
// # Key Components
//
// The package consists of several key components:
//
//   - AdaptiveConfig: Main configuration structure with validation
//   - Manager: Central coordinator for configuration management
//   - ResourceManager: Monitors and manages system resources
//   - PerformanceMonitor: Collects and analyzes performance metrics
//   - AutoTuner: Automatically optimizes configuration parameters
//
// # Configuration Structure
//
// The adaptive configuration is organized into several sections:
//
//   - Bitswap: Settings for optimizing Bitswap performance under high load
//   - Blockstore: Configuration for multi-tier caching and batch operations
//   - Network: Network optimization settings for cluster environments
//   - Monitoring: Metrics collection and alerting configuration
//   - Resources: Resource management and graceful degradation settings
//   - AutoTune: Automatic parameter tuning configuration
//
// # Usage Example
//
//	// Create a manager with default configuration
//	manager, err := adaptive.NewManager(nil)
//	if err != nil {
//		log.Fatal(err)
//	}
//
//	// Start the adaptive management system
//	ctx := context.Background()
//	if err := manager.Start(ctx); err != nil {
//		log.Fatal(err)
//	}
//	defer manager.Stop(ctx)
//
//	// Subscribe to configuration changes
//	eventCh, err := manager.Subscribe(ctx)
//	if err != nil {
//		log.Fatal(err)
//	}
//
//	// Monitor configuration changes
//	go func() {
//		for event := range eventCh {
//			log.Printf("Config changed: %s from %s", event.Type, event.Source)
//		}
//	}()
//
//	// Get current configuration
//	config := manager.GetConfig()
//	log.Printf("Current batch size: %d", config.Bitswap.BatchSize)
//
//	// Update configuration
//	newConfig := config.Clone()
//	newConfig.Bitswap.BatchSize = 2000
//	if err := manager.UpdateConfig(ctx, newConfig); err != nil {
//		log.Printf("Failed to update config: %v", err)
//	}
//
// # Adaptive Features
//
// The system provides several adaptive features:
//
// ## Resource Monitoring
//
// Continuously monitors system resources (CPU, memory, disk, network) and can
// trigger graceful degradation when thresholds are exceeded.
//
// ## Performance Analysis
//
// Analyzes performance metrics to identify bottlenecks and suggest configuration
// optimizations.
//
// ## Auto-Tuning
//
// Automatically adjusts configuration parameters based on performance patterns
// and machine learning models (when enabled).
//
// ## Hot Reconfiguration
//
// Applies configuration changes without requiring service restarts, ensuring
// continuous operation.
//
// ## Circuit Breaker
//
// Implements circuit breaker patterns to protect against cascading failures
// during high load conditions.
//
// # Configuration Validation
//
// All configuration changes are validated before application:
//
//	config := adaptive.DefaultConfig()
//	config.Bitswap.BatchSize = 0 // Invalid value
//
//	if err := config.Validate(); err != nil {
//		log.Printf("Invalid configuration: %v", err)
//	}
//
// # Graceful Degradation
//
// The system supports graceful degradation through configurable steps:
//
//	steps := []adaptive.DegradationStep{
//		{Threshold: 0.7, Action: "reduce_batch_size", Severity: "low"},
//		{Threshold: 0.8, Action: "reduce_workers", Severity: "medium"},
//		{Threshold: 0.9, Action: "disable_compression", Severity: "high"},
//		{Threshold: 0.95, Action: "emergency_mode", Severity: "critical"},
//	}
//
// # Metrics and Monitoring
//
// The system exports metrics in Prometheus format and provides structured
// logging for observability:
//
//	// Get current performance metrics
//	metrics, err := manager.GetPerformanceMetrics(ctx)
//	if err != nil {
//		log.Fatal(err)
//	}
//
//	log.Printf("Bitswap P95 response time: %v", metrics.Bitswap.P95ResponseTime)
//	log.Printf("Cache hit rate: %.2f", metrics.Blockstore.CacheHitRate)
//
// # Thread Safety
//
// All components in this package are thread-safe and can be used concurrently
// from multiple goroutines. Configuration updates are atomic and use appropriate
// locking mechanisms.
//
// # Requirements Mapping
//
// This package addresses the following requirements from the specification:
//
//   - Requirement 6.1: Dynamic configuration without restart
//   - Requirement 6.2: Automatic parameter optimization
//   - Requirement 6.3: Configuration validation and safety
//
package adaptive