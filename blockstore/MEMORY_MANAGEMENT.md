# Blockstore Memory Management

This document describes the memory management features added to the Boxo blockstore to optimize performance under high load conditions.

## Overview

The memory management system provides:

1. **Real-time memory monitoring** - Continuous tracking of memory usage and pressure levels
2. **Graceful degradation** - Automatic reduction of service quality under memory pressure
3. **Automatic cache cleanup** - Proactive cleanup of cache entries when memory limits are approached
4. **Memory leak prevention** - Built-in safeguards to prevent memory leaks during long-running operations

## Components

### MemoryMonitor

The `MemoryMonitor` provides real-time memory usage tracking and pressure level detection.

```go
// Create a memory monitor
ctx := context.Background()
mm := NewMemoryMonitor(ctx)

// Configure thresholds
mm.SetThresholds(MemoryThresholds{
    LowPressure:      0.6,  // 60% memory usage
    MediumPressure:   0.75, // 75% memory usage  
    HighPressure:     0.85, // 85% memory usage
    CriticalPressure: 0.95, // 95% memory usage
})

// Register callbacks for pressure events
mm.RegisterPressureCallback(MemoryPressureHigh, func(stats MemoryStats) {
    log.Printf("High memory pressure: %.1f%% usage", stats.UsageRatio*100)
})

// Start monitoring
mm.Start()
defer mm.Stop()
```

### MemoryAwareBlockstore

The `MemoryAwareBlockstore` wraps any blockstore with memory management capabilities.

```go
// Create base blockstore
baseBS := NewBlockstore(datastore)

// Configure memory-aware behavior
config := DefaultMemoryAwareConfig()
config.MaxMemoryBytes = 1024 * 1024 * 1024 // 1GB limit
config.DisableCacheOnHigh = true
config.RejectWritesOnCritical = true

// Create memory-aware blockstore
mabs, err := NewMemoryAwareBlockstore(ctx, baseBS, config)
if err != nil {
    return err
}
defer mabs.Close()

// Use normally - memory management is automatic
err = mabs.Put(ctx, block)
```

## Configuration Options

### MemoryAwareConfig

- `MaxMemoryBytes` - Maximum memory usage limit (0 = unlimited)
- `MonitorInterval` - How often to check memory usage (default: 5s)
- `DisableCacheOnHigh` - Disable caching when memory pressure is high
- `RejectWritesOnCritical` - Reject write operations when memory is critical
- `ForceGCOnMedium` - Force garbage collection on medium pressure or higher
- `CleanupBatchSize` - Number of cache entries to clean per batch
- `CleanupInterval` - How often to run cleanup operations
- `AggressiveCleanup` - More aggressive cleanup under pressure

### Memory Thresholds

- `LowPressure` - Start monitoring more closely (default: 60%)
- `MediumPressure` - Begin proactive measures (default: 75%)
- `HighPressure` - Disable non-essential features (default: 85%)
- `CriticalPressure` - Reject non-critical operations (default: 95%)

## Graceful Degradation Behavior

The system automatically adjusts behavior based on memory pressure:

### Low Pressure (60-75%)
- Increased monitoring frequency
- Degradation level: 1

### Medium Pressure (75-85%)
- Force garbage collection if configured
- Begin cache cleanup operations
- Degradation level: 2

### High Pressure (85-95%)
- Disable caching if configured
- Aggressive cache cleanup
- Force garbage collection
- Degradation level: 3

### Critical Pressure (95%+)
- Reject write operations if configured
- Disable all caching
- Maximum cleanup aggressiveness
- Degradation level: 4

## Monitoring and Metrics

The system exports Prometheus metrics when available:

- `blockstore_memory_usage_ratio` - Current memory usage ratio
- `blockstore_memory_pressure_level` - Current pressure level (0-4)
- `blockstore_gc_cycles_total` - Total GC cycles triggered
- `blockstore_rejected_ops_total` - Operations rejected due to memory pressure
- `blockstore_cache_disabled_total` - Times cache was disabled
- `blockstore_cleanup_ops_total` - Cache cleanup operations performed
- `blockstore_degradation_level` - Current degradation level

## Usage Examples

### Basic Usage

```go
// Create memory-aware blockstore with default settings
config := DefaultMemoryAwareConfig()
mabs, err := NewMemoryAwareBlockstore(ctx, baseBlockstore, config)
if err != nil {
    return err
}
defer mabs.Close()

// Operations work normally
block := blocks.NewBlock(data)
err = mabs.Put(ctx, block)
if err == ErrMemoryPressure {
    // Handle memory pressure rejection
    log.Println("Operation rejected due to memory pressure")
}
```

### High-Load Configuration

```go
config := MemoryAwareConfig{
    MaxMemoryBytes:         2 * 1024 * 1024 * 1024, // 2GB limit
    MonitorInterval:        time.Second,             // Monitor every second
    DisableCacheOnHigh:     true,                    // Disable cache under pressure
    RejectWritesOnCritical: true,                    // Reject writes when critical
    ForceGCOnMedium:        true,                    // Aggressive GC
    CleanupInterval:        30 * time.Second,        // Cleanup every 30s
    AggressiveCleanup:      true,                    // Aggressive cleanup
}
```

### Custom Pressure Callbacks

```go
mm := NewMemoryMonitor(ctx)

// Log all pressure level changes
for level := MemoryPressureLow; level <= MemoryPressureCritical; level++ {
    mm.RegisterPressureCallback(level, func(stats MemoryStats) {
        log.Printf("Memory pressure %v: %.1f%% usage, %d GC cycles", 
            stats.PressureLevel, stats.UsageRatio*100, stats.GCCycles)
    })
}
```

## Best Practices

1. **Set appropriate memory limits** - Configure `MaxMemoryBytes` based on available system memory
2. **Monitor pressure levels** - Use callbacks or metrics to track memory pressure
3. **Handle rejections gracefully** - Check for `ErrMemoryPressure` and implement retry logic
4. **Tune thresholds** - Adjust pressure thresholds based on your application's needs
5. **Use with caching** - Combine with cached blockstores for maximum benefit
6. **Test under load** - Verify behavior under realistic load conditions

## Troubleshooting

### High Memory Usage
- Check if `MaxMemoryBytes` is set appropriately
- Verify cache cleanup is working (`blockstore_cleanup_ops_total` metric)
- Monitor GC frequency (`blockstore_gc_cycles_total` metric)

### Frequent Operation Rejections
- Lower the `CriticalPressure` threshold
- Increase `MaxMemoryBytes` if possible
- Implement retry logic with backoff

### Poor Performance
- Ensure `MonitorInterval` isn't too frequent
- Check if cache is being disabled too aggressively
- Verify cleanup operations aren't too frequent

## Requirements Satisfied

This implementation satisfies the following requirements from the specification:

- **5.1**: Real-time memory usage monitoring with configurable thresholds
- **5.2**: Graceful degradation mechanisms that activate under memory pressure
- **5.4**: Automatic cache cleanup and memory management to prevent leaks

The system provides comprehensive memory management for blockstore operations under high load conditions while maintaining system stability and performance.