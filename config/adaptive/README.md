# Adaptive Configuration Package

This package provides adaptive configuration management for high-load optimization in Boxo. It implements a comprehensive system for dynamically managing configuration parameters based on real-time performance metrics and resource usage.

## Features

- **Dynamic Configuration**: Update configuration parameters without service restarts
- **Automatic Tuning**: AI-driven parameter optimization based on performance patterns
- **Graceful Degradation**: Automatic service degradation under resource pressure
- **Performance Monitoring**: Real-time metrics collection and bottleneck analysis
- **Resource Management**: Intelligent resource usage monitoring and control
- **Configuration Validation**: Comprehensive validation of all configuration parameters
- **Hot Reconfiguration**: Apply changes without interrupting service operation

## Quick Start

```go
package main

import (
    "context"
    "log"
    
    "github.com/ipfs/boxo/config/adaptive"
)

func main() {
    // Create manager with default configuration
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
    log.Printf("Current batch size: %d", config.Bitswap.BatchSize)
    
    // Update configuration
    newConfig := config.Clone()
    newConfig.Bitswap.BatchSize = 1000
    if err := manager.UpdateConfig(ctx, newConfig); err != nil {
        log.Printf("Failed to update: %v", err)
    }
}
```

## Configuration Structure

The adaptive configuration is organized into several sections:

### Bitswap Configuration
- `MaxOutstandingBytesPerPeer`: Dynamic limit for outstanding bytes (1MB-100MB)
- `MinWorkerCount`/`MaxWorkerCount`: Worker pool size limits
- `BatchSize`: Request batching size (100-1000)
- `CircuitBreakerEnabled`: Enable circuit breaker protection

### Blockstore Configuration
- `MemoryCacheSize`/`SSDCacheSize`/`HDDCacheSize`: Multi-tier cache sizes
- `BatchSize`: Batch I/O operation size
- `CompressionEnabled`: Enable data compression
- `StreamingThreshold`: Size threshold for streaming (e.g., 1MB)

### Network Configuration
- `HighWater`/`LowWater`: Connection pool limits
- `LatencyThreshold`: Maximum acceptable latency
- `AdaptiveBufferSizing`: Enable dynamic buffer sizing
- `KeepAliveInterval`: Keep-alive connection interval

### Monitoring Configuration
- `MetricsEnabled`: Enable metrics collection
- `PrometheusEnabled`: Export Prometheus metrics
- `AlertingEnabled`: Enable performance alerting
- `BottleneckAnalysisEnabled`: Enable bottleneck detection

### Resource Management
- `MaxMemoryUsage`: Memory usage limits
- `DegradationEnabled`: Enable graceful degradation
- `DegradationSteps`: Configurable degradation steps

### Auto-Tuning Configuration
- `Enabled`: Enable automatic parameter tuning
- `SafetyMode`: Prevent aggressive changes
- `LearningEnabled`: Enable machine learning mode
- `MaxChangePercent`: Maximum change per tuning cycle

## Usage Examples

### Configuration Validation

```go
config := adaptive.DefaultConfig()
config.Bitswap.BatchSize = 0 // Invalid value

if err := config.Validate(); err != nil {
    log.Printf("Invalid configuration: %v", err)
}
```

### Subscribing to Configuration Changes

```go
eventCh, err := manager.Subscribe(ctx)
if err != nil {
    log.Fatal(err)
}

go func() {
    for event := range eventCh {
        log.Printf("Config changed: %s from %s", event.Type, event.Source)
    }
}()
```

### Getting Performance Recommendations

```go
metrics, err := manager.GetPerformanceMetrics(ctx)
if err != nil {
    log.Fatal(err)
}

recommendations, err := manager.GetConfigRecommendations(ctx, metrics)
if err != nil {
    log.Fatal(err)
}

for _, rec := range recommendations.Recommendations {
    log.Printf("Recommendation: %s.%s %v -> %v", 
        rec.Component, rec.Parameter, rec.CurrentValue, rec.RecommendedValue)
}
```

### Resource Monitoring

```go
resourceMetrics, err := manager.GetResourceMetrics(ctx)
if err != nil {
    log.Fatal(err)
}

log.Printf("Memory usage: %.2f%%", resourceMetrics.MemoryPercent*100)
log.Printf("CPU usage: %.2f%%", resourceMetrics.CPUUsage*100)
```

## Default Configuration Values

The package provides sensible defaults optimized for high-load scenarios:

- **Bitswap**: 100MB max outstanding bytes, 128-2048 workers, 500 batch size
- **Blockstore**: 2GB memory cache, 50GB SSD cache, compression enabled
- **Network**: 2000 high water, 1000 low water, adaptive buffering
- **Monitoring**: Prometheus enabled on port 9090, 10s metrics interval
- **Auto-tuning**: Enabled with safety mode, 5-minute tuning interval

## Graceful Degradation

The system supports configurable degradation steps:

```go
steps := []adaptive.DegradationStep{
    {Threshold: 0.7, Action: "reduce_batch_size", Severity: "low"},
    {Threshold: 0.8, Action: "reduce_workers", Severity: "medium"},
    {Threshold: 0.9, Action: "disable_compression", Severity: "high"},
    {Threshold: 0.95, Action: "emergency_mode", Severity: "critical"},
}
```

## Performance Metrics

The system collects comprehensive performance metrics:

- **Bitswap**: Request rates, response times, worker utilization
- **Blockstore**: Cache hit rates, I/O latency, compression ratios
- **Network**: Connection counts, latency, bandwidth utilization
- **System**: Overall throughput, error rates, availability

## Thread Safety

All components are thread-safe and can be used concurrently from multiple goroutines. Configuration updates are atomic and use appropriate locking mechanisms.

## Requirements Mapping

This package addresses the following requirements:

- **Requirement 6.1**: Dynamic configuration without restart
- **Requirement 6.2**: Automatic parameter optimization  
- **Requirement 6.3**: Configuration validation and safety

## Testing

Run the comprehensive test suite:

```bash
go test ./config/adaptive -v
```

Run example tests:

```bash
go test ./config/adaptive -run Example
```

## Architecture

The package consists of several key components:

- **Manager**: Central coordinator for configuration management
- **ResourceManager**: Monitors and manages system resources
- **PerformanceMonitor**: Collects and analyzes performance metrics
- **AutoTuner**: Automatically optimizes configuration parameters
- **AdaptiveConfig**: Main configuration structure with validation

All components implement well-defined interfaces for extensibility and testing.