# Boxo Performance Monitoring System

This package provides a comprehensive performance monitoring system for Boxo components, designed to collect, analyze, and export metrics for high-load optimization scenarios.

## Overview

The monitoring system consists of:

- **PerformanceMonitor**: Central coordinator for metric collection and Prometheus export
- **Component Collectors**: Specialized collectors for Bitswap, Blockstore, Network, and Resource metrics
- **Prometheus Integration**: Built-in support for Prometheus metrics export
- **Concurrent Safe**: All components are designed for concurrent access

## Quick Start

```go
package main

import (
    "context"
    "log"
    "net/http"
    "time"
    
    "github.com/ipfs/boxo/monitoring"
    "github.com/prometheus/client_golang/prometheus/promhttp"
)

func main() {
    // Create performance monitor
    monitor := monitoring.NewPerformanceMonitor()
    
    // Create and register collectors
    bitswapCollector := monitoring.NewBitswapCollector()
    blockstoreCollector := monitoring.NewBlockstoreCollector()
    networkCollector := monitoring.NewNetworkCollector()
    resourceCollector := monitoring.NewResourceCollector()
    
    monitor.RegisterCollector("bitswap", bitswapCollector)
    monitor.RegisterCollector("blockstore", blockstoreCollector)
    monitor.RegisterCollector("network", networkCollector)
    monitor.RegisterCollector("resource", resourceCollector)
    
    // Start continuous collection every 5 seconds
    ctx := context.Background()
    monitor.StartCollection(ctx, 5*time.Second)
    defer monitor.StopCollection()
    
    // Set up Prometheus endpoint
    http.Handle("/metrics", promhttp.HandlerFor(monitor.GetPrometheusRegistry(), promhttp.HandlerOpts{}))
    log.Fatal(http.ListenAndServe(":8080", nil))
}
```

## Components

### PerformanceMonitor

The central component that coordinates metric collection from all registered collectors.

**Key Features:**
- Automatic Prometheus metrics registration
- Concurrent metric collection
- Configurable collection intervals
- Thread-safe operations

**Methods:**
- `CollectMetrics()`: Collect metrics from all registered collectors
- `StartCollection(ctx, interval)`: Start automatic metric collection
- `StopCollection()`: Stop metric collection
- `RegisterCollector(name, collector)`: Register a component collector
- `GetPrometheusRegistry()`: Get Prometheus registry for HTTP export

### BitswapCollector

Collects Bitswap-specific performance metrics including request/response times, connection counts, and block transfer statistics.

**Metrics:**
- Requests per second
- Average/P95/P99 response times
- Active connections and queued requests
- Blocks sent/received/duplicated
- Error rates

**Usage:**
```go
collector := monitoring.NewBitswapCollector()

// Record operations
collector.RecordRequest()
collector.RecordResponse(50*time.Millisecond, true)
collector.RecordBlockReceived(false) // not duplicate
collector.RecordBlockSent()
collector.UpdateActiveConnections(10)
```

### BlockstoreCollector

Monitors blockstore performance including cache efficiency, I/O latencies, and storage utilization.

**Metrics:**
- Cache hit/miss rates
- Read/write operation latencies
- Batch operations per second
- Compression ratios
- Storage usage statistics

**Usage:**
```go
collector := monitoring.NewBlockstoreCollector()

// Record operations
collector.RecordCacheHit()
collector.RecordReadOperation(5*time.Millisecond)
collector.RecordWriteOperation(10*time.Millisecond)
collector.UpdateTotalBlocks(1000)
collector.UpdateCompressionRatio(0.75)
```

### NetworkCollector

Tracks network layer performance including connection quality, bandwidth utilization, and peer-specific metrics.

**Metrics:**
- Active connection counts
- Network latency and bandwidth
- Packet loss rates
- Per-peer connection quality
- Bytes sent/received

**Usage:**
```go
collector := monitoring.NewNetworkCollector()

// Record network activity
collector.UpdateActiveConnections(5)
collector.RecordLatency(20*time.Millisecond)
collector.RecordBandwidth(1024*1024) // 1MB/s
collector.RecordPeerLatency("peer1", 15*time.Millisecond)
collector.RecordConnectionError()
```

### ResourceCollector

Monitors system resource usage including CPU, memory, disk, and Go runtime metrics.

**Metrics:**
- CPU usage percentage
- Memory usage and pressure
- Disk usage and pressure
- Goroutine counts
- Garbage collection statistics

**Usage:**
```go
collector := monitoring.NewResourceCollector()

// Update resource metrics
collector.UpdateCPUUsage(45.5)
collector.UpdateDiskUsage(1024*1024*1024, 10*1024*1024*1024)

// Get runtime metrics (automatic)
metrics, _ := collector.CollectMetrics(context.Background())
```

## Prometheus Metrics

The system automatically exports metrics in Prometheus format with the following naming convention:

- `boxo_bitswap_*`: Bitswap metrics
- `boxo_blockstore_*`: Blockstore metrics  
- `boxo_network_*`: Network metrics
- `boxo_resource_*`: Resource metrics

### Example Prometheus Metrics

```
# HELP boxo_bitswap_requests_total Total number of Bitswap requests processed
# TYPE boxo_bitswap_requests_total counter
boxo_bitswap_requests_total 1234

# HELP boxo_bitswap_response_time_seconds Bitswap request response time in seconds
# TYPE boxo_bitswap_response_time_seconds histogram
boxo_bitswap_response_time_seconds_bucket{le="0.001"} 45
boxo_bitswap_response_time_seconds_bucket{le="0.005"} 123
boxo_bitswap_response_time_seconds_bucket{le="0.01"} 234

# HELP boxo_blockstore_cache_hit_rate Blockstore cache hit rate (0-1)
# TYPE boxo_blockstore_cache_hit_rate gauge
boxo_blockstore_cache_hit_rate 0.85

# HELP boxo_network_active_connections Number of active network connections
# TYPE boxo_network_active_connections gauge
boxo_network_active_connections 15
```

## Performance Considerations

### Memory Usage

- Collectors maintain limited history for percentile calculations (default: 1000 samples)
- Automatic cleanup of stale peer connections
- Efficient concurrent data structures

### CPU Impact

- Minimal overhead during metric recording
- Batch processing for Prometheus updates
- Configurable collection intervals

### Concurrency

- All collectors are thread-safe
- Lock-free operations where possible
- Separate read/write locks for optimal performance

## Configuration

### Collection Intervals

```go
// High-frequency collection (every 1 second)
monitor.StartCollection(ctx, 1*time.Second)

// Standard collection (every 5 seconds)
monitor.StartCollection(ctx, 5*time.Second)

// Low-frequency collection (every 30 seconds)
monitor.StartCollection(ctx, 30*time.Second)
```

### Memory Limits

```go
// Configure collector history limits
collector := monitoring.NewBitswapCollector()
// Default: keeps last 1000 response times for percentiles

collector := monitoring.NewNetworkCollector()
// Default: keeps last 1000 latency samples
// Default: keeps last 100 bandwidth samples
```

## Integration Examples

### With IPFS Cluster

```go
// In your IPFS cluster node
monitor := monitoring.NewPerformanceMonitor()
bitswapCollector := monitoring.NewBitswapCollector()

// Integrate with Bitswap operations
func (bs *Bitswap) GetBlock(ctx context.Context, cid cid.Cid) (blocks.Block, error) {
    bitswapCollector.RecordRequest()
    start := time.Now()
    
    block, err := bs.actualGetBlock(ctx, cid)
    
    duration := time.Since(start)
    bitswapCollector.RecordResponse(duration, err == nil)
    
    return block, err
}
```

### With Custom Applications

```go
// Custom metric collector
type MyAppCollector struct {
    requestCount int64
}

func (c *MyAppCollector) CollectMetrics(ctx context.Context) (map[string]interface{}, error) {
    return map[string]interface{}{
        "custom_requests": c.requestCount,
    }, nil
}

func (c *MyAppCollector) GetMetricNames() []string {
    return []string{"custom_requests"}
}

// Register with monitor
monitor.RegisterCollector("myapp", &MyAppCollector{})
```

## Testing

The package includes comprehensive tests:

```bash
# Run all tests
go test ./monitoring

# Run with race detection
go test -race ./monitoring

# Run benchmarks
go test -bench=. ./monitoring

# Run specific test
go test -run TestPerformanceMonitor ./monitoring
```

## Troubleshooting

### High Memory Usage

1. Check collector history limits
2. Verify collection intervals aren't too frequent
3. Monitor goroutine counts in resource metrics

### Missing Metrics

1. Ensure collectors are registered before starting collection
2. Check for errors in CollectMetrics() calls
3. Verify Prometheus endpoint is accessible

### Performance Impact

1. Increase collection intervals if CPU usage is high
2. Use sampling for high-frequency operations
3. Monitor GC pause times in resource metrics

## Requirements Compliance

This implementation satisfies requirement 4.1 from the high-load optimization specification:

- ✅ Implements `PerformanceMonitor` interface for metric collection
- ✅ Provides Prometheus format export
- ✅ Creates structures for Bitswap, Blockstore, and Network metrics
- ✅ Includes comprehensive tests for metric collection correctness
- ✅ Supports concurrent access and high-load scenarios
- ✅ Provides real-time performance monitoring capabilities

## Future Enhancements

- Automatic anomaly detection
- Machine learning-based performance prediction
- Integration with alerting systems
- Custom dashboard generation
- Historical metric storage