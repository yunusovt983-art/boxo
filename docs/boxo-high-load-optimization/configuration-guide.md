# Boxo High Load Configuration Guide

## Quick Start

For immediate high-load optimization, apply these minimal configuration changes:

```go
import (
    "github.com/ipfs/boxo/bitswap"
    "github.com/ipfs/boxo/blockstore"
    "github.com/ipfs/boxo/config/adaptive"
)

// Quick high-load configuration
config := &adaptive.HighLoadConfig{
    Bitswap: adaptive.AdaptiveBitswapConfig{
        MaxOutstandingBytesPerPeer: 10 << 20, // 10MB
        MaxWorkerCount:             1024,
        BatchSize:                  500,
        BatchTimeout:               time.Millisecond * 20,
    },
    Blockstore: adaptive.BlockstoreConfig{
        MemoryCacheSize:    2 << 30, // 2GB
        BatchSize:          1000,
        CompressionEnabled: true,
    },
    Network: adaptive.NetworkConfig{
        HighWater:   2000,
        LowWater:    1000,
        GracePeriod: time.Second * 30,
    },
}
```

## Configuration Scenarios

### 1. Small Cluster (3-5 nodes, < 1000 concurrent users)

**Characteristics:**
- Limited resources per node
- Moderate request volume
- Focus on stability over peak performance

```go
smallClusterConfig := &adaptive.HighLoadConfig{
    Bitswap: adaptive.AdaptiveBitswapConfig{
        MaxOutstandingBytesPerPeer: 5 << 20,  // 5MB
        MinOutstandingBytesPerPeer: 1 << 20,  // 1MB
        MinWorkerCount:             64,
        MaxWorkerCount:             256,
        BatchSize:                  100,
        BatchTimeout:               time.Millisecond * 50,
        HighPriorityThreshold:      time.Millisecond * 100,
        CriticalPriorityThreshold:  time.Millisecond * 20,
    },
    Blockstore: adaptive.BlockstoreConfig{
        MemoryCacheSize:    512 << 20, // 512MB
        SSDCacheSize:       10 << 30,  // 10GB
        BatchSize:          500,
        BatchTimeout:       time.Millisecond * 100,
        CompressionEnabled: true,
        CompressionLevel:   3,
    },
    Network: adaptive.NetworkConfig{
        HighWater:              500,
        LowWater:               250,
        GracePeriod:            time.Second * 60,
        LatencyThreshold:       time.Millisecond * 300,
        BandwidthThreshold:     1 << 20, // 1MB/s
        ErrorRateThreshold:     0.1,     // 10%
        KeepAliveInterval:      time.Second * 45,
        KeepAliveTimeout:       time.Second * 15,
    },
    Monitoring: adaptive.MonitoringConfig{
        MetricsInterval:    time.Second * 30,
        AlertThresholds: adaptive.AlertThresholds{
            ResponseTimeP95:    time.Millisecond * 200,
            ErrorRate:          0.05,
            MemoryUsage:        0.8,
            CPUUsage:           0.7,
        },
    },
}
```

### 2. Medium Cluster (5-20 nodes, 1000-10000 concurrent users)

**Characteristics:**
- Balanced resource allocation
- High request volume
- Need for adaptive scaling

```go
mediumClusterConfig := &adaptive.HighLoadConfig{
    Bitswap: adaptive.AdaptiveBitswapConfig{
        MaxOutstandingBytesPerPeer: 20 << 20, // 20MB
        MinOutstandingBytesPerPeer: 2 << 20,  // 2MB
        MinWorkerCount:             128,
        MaxWorkerCount:             1024,
        BatchSize:                  500,
        BatchTimeout:               time.Millisecond * 20,
        HighPriorityThreshold:      time.Millisecond * 50,
        CriticalPriorityThreshold:  time.Millisecond * 10,
    },
    Blockstore: adaptive.BlockstoreConfig{
        MemoryCacheSize:    2 << 30,  // 2GB
        SSDCacheSize:       50 << 30, // 50GB
        BatchSize:          1000,
        BatchTimeout:       time.Millisecond * 50,
        CompressionEnabled: true,
        CompressionLevel:   5,
    },
    Network: adaptive.NetworkConfig{
        HighWater:              2000,
        LowWater:               1000,
        GracePeriod:            time.Second * 30,
        LatencyThreshold:       time.Millisecond * 200,
        BandwidthThreshold:     5 << 20, // 5MB/s
        ErrorRateThreshold:     0.05,    // 5%
        KeepAliveInterval:      time.Second * 30,
        KeepAliveTimeout:       time.Second * 10,
    },
    Monitoring: adaptive.MonitoringConfig{
        MetricsInterval:    time.Second * 15,
        AlertThresholds: adaptive.AlertThresholds{
            ResponseTimeP95:    time.Millisecond * 100,
            ErrorRate:          0.03,
            MemoryUsage:        0.85,
            CPUUsage:           0.8,
        },
        AutoTuningEnabled: true,
        AutoTuningInterval: time.Minute * 5,
    },
}
```

### 3. Large Cluster (20+ nodes, 10000+ concurrent users)

**Characteristics:**
- Maximum performance optimization
- Aggressive resource utilization
- Advanced monitoring and auto-tuning

```go
largeClusterConfig := &adaptive.HighLoadConfig{
    Bitswap: adaptive.AdaptiveBitswapConfig{
        MaxOutstandingBytesPerPeer: 100 << 20, // 100MB
        MinOutstandingBytesPerPeer: 5 << 20,   // 5MB
        MinWorkerCount:             256,
        MaxWorkerCount:             2048,
        BatchSize:                  1000,
        BatchTimeout:               time.Millisecond * 10,
        HighPriorityThreshold:      time.Millisecond * 25,
        CriticalPriorityThreshold:  time.Millisecond * 5,
    },
    Blockstore: adaptive.BlockstoreConfig{
        MemoryCacheSize:    8 << 30,   // 8GB
        SSDCacheSize:       200 << 30, // 200GB
        BatchSize:          2000,
        BatchTimeout:       time.Millisecond * 25,
        CompressionEnabled: true,
        CompressionLevel:   7,
    },
    Network: adaptive.NetworkConfig{
        HighWater:              5000,
        LowWater:               2500,
        GracePeriod:            time.Second * 15,
        LatencyThreshold:       time.Millisecond * 100,
        BandwidthThreshold:     20 << 20, // 20MB/s
        ErrorRateThreshold:     0.02,     // 2%
        KeepAliveInterval:      time.Second * 15,
        KeepAliveTimeout:       time.Second * 5,
    },
    Monitoring: adaptive.MonitoringConfig{
        MetricsInterval:    time.Second * 5,
        AlertThresholds: adaptive.AlertThresholds{
            ResponseTimeP95:    time.Millisecond * 50,
            ErrorRate:          0.01,
            MemoryUsage:        0.9,
            CPUUsage:           0.85,
        },
        AutoTuningEnabled:  true,
        AutoTuningInterval: time.Minute * 2,
        PredictiveScaling:  true,
    },
}
```

## Parameter Reference

### Bitswap Configuration

| Parameter | Description | Small Cluster | Medium Cluster | Large Cluster |
|-----------|-------------|---------------|----------------|---------------|
| `MaxOutstandingBytesPerPeer` | Maximum bytes pending per peer | 5MB | 20MB | 100MB |
| `MinOutstandingBytesPerPeer` | Minimum bytes pending per peer | 1MB | 2MB | 5MB |
| `MaxWorkerCount` | Maximum worker threads | 256 | 1024 | 2048 |
| `BatchSize` | Requests per batch | 100 | 500 | 1000 |
| `BatchTimeout` | Batch processing timeout | 50ms | 20ms | 10ms |

### Blockstore Configuration

| Parameter | Description | Small Cluster | Medium Cluster | Large Cluster |
|-----------|-------------|---------------|----------------|---------------|
| `MemoryCacheSize` | In-memory cache size | 512MB | 2GB | 8GB |
| `SSDCacheSize` | SSD cache size | 10GB | 50GB | 200GB |
| `BatchSize` | Blocks per batch operation | 500 | 1000 | 2000 |
| `CompressionLevel` | Compression level (1-9) | 3 | 5 | 7 |

### Network Configuration

| Parameter | Description | Small Cluster | Medium Cluster | Large Cluster |
|-----------|-------------|---------------|----------------|---------------|
| `HighWater` | Maximum connections | 500 | 2000 | 5000 |
| `LowWater` | Target connections | 250 | 1000 | 2500 |
| `GracePeriod` | Connection cleanup delay | 60s | 30s | 15s |
| `LatencyThreshold` | Maximum acceptable latency | 300ms | 200ms | 100ms |

## Environment-Specific Tuning

### Development Environment
```go
devConfig := smallClusterConfig
devConfig.Monitoring.MetricsInterval = time.Second * 60
devConfig.Monitoring.AlertThresholds.ResponseTimeP95 = time.Millisecond * 500
```

### Staging Environment
```go
stagingConfig := mediumClusterConfig
stagingConfig.Monitoring.AutoTuningEnabled = false // Disable for predictable testing
```

### Production Environment
```go
prodConfig := largeClusterConfig
prodConfig.Monitoring.PredictiveScaling = true
prodConfig.Monitoring.AlertThresholds.ErrorRate = 0.005 // Stricter error tolerance
```

## Dynamic Configuration

Enable runtime configuration updates:

```go
import "github.com/ipfs/boxo/config/adaptive"

// Initialize with dynamic configuration support
manager, err := adaptive.NewConfigManager(config)
if err != nil {
    return err
}

// Update configuration at runtime
newConfig := &adaptive.HighLoadConfig{
    // Updated parameters
}

err = manager.UpdateConfig(context.Background(), newConfig)
if err != nil {
    return err
}
```

## Configuration Validation

Validate configuration before applying:

```go
validator := adaptive.NewConfigValidator()

if err := validator.Validate(config); err != nil {
    log.Fatalf("Invalid configuration: %v", err)
}

// Get optimization recommendations
recommendations := validator.GetRecommendations(config, currentMetrics)
for _, rec := range recommendations {
    log.Printf("Recommendation: %s", rec.Description)
}
```

## Best Practices

1. **Start Conservative**: Begin with small cluster configuration and scale up based on observed performance
2. **Monitor First**: Always enable monitoring before applying optimizations
3. **Gradual Changes**: Make incremental configuration changes and observe their impact
4. **Environment Consistency**: Use similar configurations across development, staging, and production
5. **Regular Review**: Review and adjust configurations based on usage patterns and performance metrics

## Next Steps

- Review [Cluster Deployment Examples](cluster-deployment-examples.md) for specific deployment scenarios
- Check [Troubleshooting Guide](troubleshooting-guide.md) if experiencing performance issues
- See [Migration Guide](migration-guide.md) for upgrading existing installations