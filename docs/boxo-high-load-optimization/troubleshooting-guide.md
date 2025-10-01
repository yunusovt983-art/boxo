# Troubleshooting Guide

This guide helps diagnose and resolve common performance issues in Boxo high-load deployments.

## Quick Diagnostics

### Performance Health Check Script

```bash
#!/bin/bash
# boxo-health-check.sh - Quick performance diagnostics

echo "=== Boxo High-Load Health Check ==="
echo "Timestamp: $(date)"
echo

# Check system resources
echo "--- System Resources ---"
echo "Memory Usage:"
free -h
echo
echo "CPU Usage:"
top -bn1 | grep "Cpu(s)" | awk '{print $2}' | cut -d'%' -f1
echo
echo "Disk Usage:"
df -h | grep -E "(/$|/data)"
echo

# Check network connections
echo "--- Network Status ---"
echo "Active connections:"
netstat -an | grep :9096 | wc -l
echo "Connection states:"
netstat -an | grep :9096 | awk '{print $6}' | sort | uniq -c
echo

# Check IPFS cluster status
echo "--- Cluster Status ---"
if command -v ipfs-cluster-ctl &> /dev/null; then
    echo "Cluster peers:"
    ipfs-cluster-ctl peers ls 2>/dev/null | wc -l
    echo "Pin status:"
    ipfs-cluster-ctl status --local 2>/dev/null | head -5
fi
echo

# Check metrics endpoint
echo "--- Metrics Status ---"
if curl -s http://localhost:9090/metrics > /dev/null; then
    echo "Metrics endpoint: OK"
    echo "Recent error rate:"
    curl -s http://localhost:9090/metrics | grep "boxo_error_rate" | tail -1
else
    echo "Metrics endpoint: UNAVAILABLE"
fi
echo

echo "=== Health Check Complete ==="
```

## Common Issues and Solutions

### 1. High Response Times

**Symptoms:**
- API responses > 500ms
- Timeouts in client applications
- High P95 response times in metrics

**Diagnosis:**
```bash
# Check current response times
curl -w "@curl-format.txt" -o /dev/null -s "http://localhost:9094/api/v0/pins"

# Monitor real-time metrics
watch -n 1 'curl -s http://localhost:9090/metrics | grep response_time'

# Check system load
iostat -x 1 5
```

**Solutions:**

1. **Increase Worker Pool Size:**
```go
config.Bitswap.MaxWorkerCount = 2048  // Increase from default
config.Bitswap.MinWorkerCount = 512   // Raise minimum
```

2. **Optimize Batch Processing:**
```go
config.Bitswap.BatchSize = 1000       // Larger batches
config.Bitswap.BatchTimeout = time.Millisecond * 10  // Shorter timeout
```

3. **Tune Memory Cache:**
```go
config.Blockstore.MemoryCacheSize = 8 << 30  // 8GB cache
```

### 2. Memory Leaks

**Symptoms:**
- Continuously increasing memory usage
- Out of memory errors
- System becomes unresponsive

**Diagnosis:**
```bash
# Monitor memory usage over time
while true; do
    echo "$(date): $(ps -o pid,vsz,rss,comm -p $(pgrep ipfs-cluster))"
    sleep 60
done > memory-usage.log

# Check for memory leaks in Go
go tool pprof http://localhost:6060/debug/pprof/heap
```

**Solutions:**

1. **Enable Memory Monitoring:**
```go
config.Monitoring.ResourceMonitoring = true
config.Monitoring.MemoryThreshold = 0.85  // Alert at 85%
```

2. **Configure Garbage Collection:**
```bash
export GOGC=50  # More aggressive GC
export GOMEMLIMIT=8GiB  # Set memory limit
```

3. **Implement Memory Pressure Handling:**
```go
config.Blockstore.MemoryPressureThreshold = 0.8
config.Blockstore.AutoCleanupEnabled = true
```

### 3. Network Connection Issues

**Symptoms:**
- Connection timeouts
- High connection churn
- Peer discovery failures

**Diagnosis:**
```bash
# Check connection limits
ulimit -n

# Monitor connection states
watch -n 1 'netstat -an | grep :9096 | awk "{print \$6}" | sort | uniq -c'

# Check peer connectivity
ipfs-cluster-ctl peers ls --verbose
```

**Solutions:**

1. **Increase System Limits:**
```bash
# /etc/security/limits.conf
* soft nofile 65536
* hard nofile 65536

# /etc/systemd/system.conf
DefaultLimitNOFILE=65536
```

2. **Optimize Connection Manager:**
```go
config.Network.HighWater = 5000
config.Network.LowWater = 2500
config.Network.GracePeriod = time.Second * 15
```

3. **Configure Keep-Alive:**
```go
config.Network.KeepAliveInterval = time.Second * 30
config.Network.KeepAliveTimeout = time.Second * 10
```

### 4. Disk I/O Bottlenecks

**Symptoms:**
- High disk wait times
- Slow block retrieval
- Storage operations timing out

**Diagnosis:**
```bash
# Monitor disk I/O
iostat -x 1 10

# Check disk usage patterns
iotop -o -d 1

# Analyze storage performance
fio --name=random-write --ioengine=posixaio --rw=randwrite --bs=4k --size=4g --numjobs=1 --iodepth=1 --runtime=60 --time_based --end_fsync=1
```

**Solutions:**

1. **Enable Batch I/O:**
```go
config.Blockstore.BatchSize = 2000
config.Blockstore.BatchTimeout = time.Millisecond * 25
```

2. **Use SSD Caching:**
```go
config.Blockstore.SSDCacheSize = 100 << 30  // 100GB SSD cache
config.Blockstore.CacheTierEnabled = true
```

3. **Optimize File System:**
```bash
# Mount with optimized options
mount -o noatime,nodiratime /dev/sdb1 /data
```

### 5. Circuit Breaker Activation

**Symptoms:**
- Requests being rejected
- "Circuit breaker open" errors
- Degraded service availability

**Diagnosis:**
```bash
# Check circuit breaker status
curl -s http://localhost:9090/metrics | grep circuit_breaker

# Monitor error rates
curl -s http://localhost:9090/metrics | grep error_rate
```

**Solutions:**

1. **Adjust Circuit Breaker Thresholds:**
```go
config.CircuitBreaker.ErrorThreshold = 0.1      // 10% error rate
config.CircuitBreaker.TimeoutThreshold = time.Second * 5
config.CircuitBreaker.RecoveryTimeout = time.Second * 30
```

2. **Implement Graceful Degradation:**
```go
config.GracefulDegradation.Enabled = true
config.GracefulDegradation.PriorityLevels = 3
```

## Performance Tuning Checklist

### System Level
- [ ] Increase file descriptor limits (`ulimit -n 65536`)
- [ ] Optimize TCP settings (`net.core.somaxconn = 65535`)
- [ ] Configure memory overcommit (`vm.overcommit_memory = 1`)
- [ ] Set appropriate swappiness (`vm.swappiness = 10`)
- [ ] Use high-performance file system (ext4 with noatime)

### Application Level
- [ ] Enable metrics collection
- [ ] Configure appropriate log levels
- [ ] Set memory limits (`GOMEMLIMIT`)
- [ ] Tune garbage collection (`GOGC`)
- [ ] Enable profiling endpoints

### Boxo Configuration
- [ ] Set appropriate worker pool sizes
- [ ] Configure batch processing parameters
- [ ] Optimize cache sizes
- [ ] Tune network connection limits
- [ ] Enable auto-tuning features

## Monitoring and Alerting

### Key Metrics to Monitor

```yaml
# Prometheus alerting rules
groups:
- name: boxo-high-load
  rules:
  - alert: HighResponseTime
    expr: boxo_response_time_p95 > 100
    for: 2m
    labels:
      severity: warning
    annotations:
      summary: "High response time detected"
      
  - alert: MemoryUsageHigh
    expr: boxo_memory_usage_ratio > 0.85
    for: 5m
    labels:
      severity: critical
    annotations:
      summary: "Memory usage above 85%"
      
  - alert: ErrorRateHigh
    expr: boxo_error_rate > 0.05
    for: 1m
    labels:
      severity: critical
    annotations:
      summary: "Error rate above 5%"
```

### Grafana Dashboard Queries

```promql
# Response time percentiles
histogram_quantile(0.95, rate(boxo_request_duration_seconds_bucket[5m]))

# Memory usage trend
boxo_memory_usage_bytes / boxo_memory_limit_bytes

# Connection count
boxo_active_connections

# Error rate
rate(boxo_errors_total[5m]) / rate(boxo_requests_total[5m])
```

## Emergency Procedures

### High Load Emergency Response

1. **Immediate Actions:**
```bash
# Reduce connection limits
ipfs-cluster-ctl config set connection_manager.high_water 1000

# Enable circuit breaker
curl -X POST http://localhost:9094/api/v0/config/circuit-breaker/enable

# Scale down non-critical operations
systemctl stop ipfs-cluster-backup
```

2. **Load Shedding:**
```go
// Implement priority-based request handling
config.LoadShedding.Enabled = true
config.LoadShedding.DropThreshold = 0.9  // Drop 90% of low-priority requests
```

3. **Graceful Degradation:**
```bash
# Reduce cache sizes to free memory
curl -X POST http://localhost:9094/api/v0/config/cache/reduce

# Disable non-essential features
curl -X POST http://localhost:9094/api/v0/config/features/disable-compression
```

### Recovery Procedures

1. **Gradual Load Restoration:**
```bash
#!/bin/bash
# gradual-recovery.sh

echo "Starting gradual recovery..."

# Restore connection limits gradually
for limit in 500 1000 2000 5000; do
    echo "Setting connection limit to $limit"
    ipfs-cluster-ctl config set connection_manager.high_water $limit
    sleep 300  # Wait 5 minutes between increases
done

echo "Recovery complete"
```

2. **Health Verification:**
```bash
# Verify system health before full restoration
./boxo-health-check.sh

# Check error rates
if [ $(curl -s http://localhost:9090/metrics | grep error_rate | awk '{print $2}' | cut -d. -f1) -lt 1 ]; then
    echo "System healthy, proceeding with full restoration"
else
    echo "System still unhealthy, maintaining reduced capacity"
fi
```

## Getting Help

### Log Analysis

```bash
# Extract performance-related logs
grep -E "(timeout|error|slow|performance)" /var/log/ipfs-cluster/cluster.log | tail -100

# Analyze error patterns
awk '/ERROR/ {print $0}' /var/log/ipfs-cluster/cluster.log | sort | uniq -c | sort -nr
```

### Debug Information Collection

```bash
#!/bin/bash
# collect-debug-info.sh

DEBUG_DIR="/tmp/boxo-debug-$(date +%Y%m%d-%H%M%S)"
mkdir -p "$DEBUG_DIR"

# System information
uname -a > "$DEBUG_DIR/system-info.txt"
free -h > "$DEBUG_DIR/memory-info.txt"
df -h > "$DEBUG_DIR/disk-info.txt"

# Network information
netstat -an > "$DEBUG_DIR/network-connections.txt"
ss -tuln > "$DEBUG_DIR/listening-ports.txt"

# Application logs
cp /var/log/ipfs-cluster/cluster.log "$DEBUG_DIR/"

# Configuration
ipfs-cluster-ctl config show > "$DEBUG_DIR/cluster-config.json"

# Metrics snapshot
curl -s http://localhost:9090/metrics > "$DEBUG_DIR/metrics.txt"

# Create archive
tar -czf "boxo-debug-$(date +%Y%m%d-%H%M%S).tar.gz" -C /tmp "$(basename $DEBUG_DIR)"

echo "Debug information collected in: boxo-debug-$(date +%Y%m%d-%H%M%S).tar.gz"
```

### Support Channels

- **GitHub Issues**: Report bugs and performance issues
- **Community Forum**: General questions and discussions
- **Documentation**: Latest configuration guides and best practices
- **Professional Support**: Enterprise support options

## Next Steps

- Review [Configuration Guide](configuration-guide.md) for optimization opportunities
- Check [Migration Guide](migration-guide.md) for upgrade procedures
- See [Integration Examples](integration-examples/) for application-specific solutions