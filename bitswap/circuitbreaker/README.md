# Bitswap Circuit Breaker

This package provides circuit breaker pattern implementation for Bitswap operations to protect against overload and cascading failures in high-load environments.

## Features

- **Circuit Breaker Pattern**: Automatic failure detection and recovery
- **Health Monitoring**: Real-time connection health tracking
- **Gradual Recovery**: Smart recovery mechanisms after failures
- **Per-Peer Protection**: Individual circuit breakers for each peer
- **Comprehensive Metrics**: Detailed performance and health metrics
- **Auto-tuning**: Automatic configuration adjustment based on load

## Components

### 1. Circuit Breaker

The core circuit breaker implementation that protects operations from cascading failures.

```go
config := circuitbreaker.DefaultBitswapCircuitBreakerConfig()
cb := circuitbreaker.NewCircuitBreaker("my-service", config)

err := cb.Execute(func() error {
    // Your protected operation here
    return doSomething()
})
```

**States:**
- `CLOSED`: Normal operation, requests pass through
- `OPEN`: Circuit is tripped, requests are rejected immediately
- `HALF_OPEN`: Testing recovery, limited requests allowed

### 2. Health Monitor

Tracks connection health and provides real-time status updates.

```go
config := circuitbreaker.DefaultHealthMonitorConfig()
hm := circuitbreaker.NewHealthMonitor(config)

// Record request results
hm.RecordRequest(peerID, responseTime, err)

// Get health status
status := hm.GetHealthStatus(peerID)
```

**Health Statuses:**
- `HEALTHY`: Good performance, low error rate
- `DEGRADED`: Acceptable performance with some issues
- `UNHEALTHY`: Poor performance, high error rate
- `UNKNOWN`: Insufficient data to determine status

### 3. Recovery Manager

Manages gradual recovery after circuit breaker trips.

```go
config := circuitbreaker.DefaultRecoveryConfig()
rm := circuitbreaker.NewRecoveryManager(config)

// Start recovery for a peer
rm.StartRecovery(peerID)

// Check if request should be allowed during recovery
if rm.ShouldAllowRequest(peerID) {
    // Process request
}
```

**Recovery Strategies:**
- `LINEAR`: Steady increase in allowed traffic
- `EXPONENTIAL`: Gradual ramp-up (safer)
- `STEP`: Discrete steps in traffic increase

### 4. Protection Manager

Integrates all components for comprehensive Bitswap protection.

```go
config := circuitbreaker.DefaultProtectionConfig()
pm := circuitbreaker.NewBitswapProtectionManager(config)
defer pm.Close()

// Execute protected request
err := pm.ExecuteRequest(ctx, peerID, func(ctx context.Context) error {
    // Your Bitswap operation
    return bitswapOperation(ctx)
})

// Get comprehensive peer status
status := pm.GetPeerStatus(peerID)
fmt.Printf("Circuit: %s, Health: %s, Recovery: %v\n", 
    status.CircuitState, status.HealthStatus, status.IsRecovering)
```

## Configuration

### Circuit Breaker Configuration

```go
config := circuitbreaker.Config{
    MaxRequests: 10,                    // Max requests in half-open state
    Interval:    30 * time.Second,      // Reset interval for closed state
    Timeout:     60 * time.Second,      // Timeout before half-open
    ReadyToTrip: func(counts circuitbreaker.Counts) bool {
        // Custom trip condition
        return counts.ConsecutiveFailures >= 5
    },
    IsSuccessful: func(err error) bool {
        // Custom success condition
        return err == nil
    },
}
```

### Health Monitor Configuration

```go
config := circuitbreaker.HealthMonitorConfig{
    CheckInterval:        30 * time.Second,
    HealthyThreshold:     0.95,  // 95% success rate
    DegradedThreshold:    0.80,  // 80% success rate
    MaxResponseTime:      5 * time.Second,
    MaxConsecutiveErrors: 5,
    CleanupInterval:      5 * time.Minute,
    MaxAge:              10 * time.Minute,
}
```

### Recovery Configuration

```go
config := circuitbreaker.RecoveryConfig{
    Strategy:            circuitbreaker.RecoveryExponential,
    InitialRecoveryRate: 0.1,   // Start with 10% traffic
    MaxRecoveryRate:     1.0,   // Eventually allow 100%
    RecoveryInterval:    10 * time.Second,
    RecoveryIncrement:   0.1,   // Increase by 10% each interval
    SuccessThreshold:    10,    // Need 10 successes to complete
    FailureThreshold:    3,     // 3 failures reset recovery
    MaxRecoveryTime:     5 * time.Minute,
    CooldownPeriod:      30 * time.Second,
}
```

## Usage Patterns

### Basic Protection

```go
// Simple circuit breaker for a single operation
cb := circuitbreaker.NewCircuitBreaker("bitswap-get", 
    circuitbreaker.DefaultBitswapCircuitBreakerConfig())

err := cb.Execute(func() error {
    return bitswap.GetBlock(ctx, cid)
})
```

### Per-Peer Protection

```go
// Circuit breaker manager for multiple peers
bcb := circuitbreaker.NewBitswapCircuitBreaker(
    circuitbreaker.DefaultBitswapCircuitBreakerConfig())

err := bcb.ExecuteWithPeer(peerID, func() error {
    return sendMessageToPeer(peerID, message)
})
```

### Full Protection Suite

```go
// Complete protection with health monitoring and recovery
pm := circuitbreaker.NewBitswapProtectionManager(
    circuitbreaker.DefaultProtectionConfig())

err := pm.ExecuteRequest(ctx, peerID, func(ctx context.Context) error {
    return performBitswapOperation(ctx, peerID)
})

// Monitor system health
status := pm.GetOverallStatus()
if !status.IsHealthy() {
    log.Warnf("System health degraded: %s", status.GetSummary())
}
```

## Metrics and Monitoring

The circuit breaker provides comprehensive metrics for monitoring:

```go
// Circuit breaker metrics
counts := cb.Counts()
fmt.Printf("Requests: %d, Successes: %d, Failures: %d\n",
    counts.Requests, counts.TotalSuccesses, counts.TotalFailures)

// Health metrics
health, _ := hm.GetHealth(peerID)
fmt.Printf("Success rate: %.2f%%, Response time: %v\n",
    health.SuccessRate*100, health.ResponseTime)

// Overall system metrics
overallStatus := pm.GetOverallStatus()
fmt.Printf("Total peers: %d, Healthy: %d, Circuit breakers open: %d\n",
    overallStatus.TotalPeers, 
    overallStatus.HealthStats.HealthyPeers,
    len(overallStatus.CircuitStates))
```

## Integration with Bitswap

To integrate with existing Bitswap code:

1. **Wrap Bitswap operations** with circuit breaker protection
2. **Monitor peer connections** using health monitoring
3. **Enable auto-recovery** for failed peers
4. **Collect metrics** for performance analysis

```go
// Example integration
type ProtectedBitswap struct {
    bitswap *bitswap.Bitswap
    protection *circuitbreaker.BitswapProtectionManager
}

func (pb *ProtectedBitswap) GetBlock(ctx context.Context, c cid.Cid) (blocks.Block, error) {
    var block blocks.Block
    err := pb.protection.ExecuteRequest(ctx, peerID, func(ctx context.Context) error {
        var err error
        block, err = pb.bitswap.GetBlock(ctx, c)
        return err
    })
    return block, err
}
```

## Best Practices

1. **Configure thresholds** based on your network conditions
2. **Monitor metrics** regularly to tune parameters
3. **Use appropriate recovery strategies** for your use case
4. **Handle circuit breaker errors** gracefully in your application
5. **Test failure scenarios** to ensure proper behavior
6. **Adjust timeouts** based on network latency characteristics

## Testing

The package includes comprehensive tests covering:

- Circuit breaker state transitions
- Health monitoring accuracy
- Recovery mechanism behavior
- Integration between components
- Concurrent operation safety
- Performance benchmarks

Run tests with:
```bash
go test -v ./bitswap/circuitbreaker/
```

## Performance Considerations

- Circuit breakers add minimal overhead to successful operations
- Health monitoring uses efficient data structures
- Recovery mechanisms are designed for low resource usage
- Metrics collection is optimized for high-frequency operations
- Background cleanup prevents memory leaks

## Requirements Satisfied

This implementation satisfies the following requirements from the specification:

- **7.1**: Circuit breaker pattern for overload protection
- **7.3**: Automatic isolation of problematic components and gradual recovery

The circuit breaker provides robust protection against cascading failures while maintaining high performance under normal conditions.