# Cluster Integration Tests

This document describes the comprehensive cluster integration test suite for Boxo's high-load optimization features.

## Overview

The cluster integration tests validate the behavior of Boxo components in a multi-node cluster environment, focusing on:

- **Node Interaction**: Communication and data exchange between cluster nodes
- **Metrics and Monitoring**: Collection, aggregation, and alerting across the cluster
- **Fault Tolerance**: System behavior during failures and recovery scenarios
- **Performance**: Throughput, latency, and scalability under various load conditions

## Test Structure

### Core Test Files

- `cluster_integration_test.go` - Main cluster interaction tests
- `cluster_test_utils.go` - Utility functions and helpers
- `cluster_test_config.json` - Test configuration templates
- `../monitoring/cluster_metrics_integration_test.go` - Metrics validation tests

### Test Categories

#### 1. Basic Cluster Tests (`TestClusterNodeInteraction`)
- **Purpose**: Validate fundamental node-to-node communication
- **Scope**: Block exchange, connection management, basic health checks
- **Duration**: ~10 seconds
- **Node Count**: 3-5 nodes

```go
func TestClusterNodeInteraction(t *testing.T)
```

**What it tests:**
- Block retrieval across nodes
- Connection establishment and maintenance
- Basic health status verification
- Data consistency across the cluster

#### 2. Performance Tests (`TestClusterPerformanceUnderLoad`)
- **Purpose**: Validate system performance under sustained load
- **Scope**: Throughput, latency, adaptive scaling
- **Duration**: 30-60 seconds
- **Node Count**: 5-10 nodes

```go
func TestClusterPerformanceUnderLoad(t *testing.T)
```

**What it tests:**
- Request throughput (target: >10 RPS)
- Response latency (target: <500ms P95)
- Success rate (target: >80%)
- Adaptive configuration updates
- Resource utilization patterns

#### 3. Fault Tolerance Tests (`TestClusterFaultTolerance`)
- **Purpose**: Validate system resilience during failures
- **Scope**: Node failures, network partitions, recovery
- **Duration**: 20-30 seconds
- **Node Count**: 5-6 nodes

```go
func TestClusterFaultTolerance(t *testing.T)
```

**What it tests:**
- Node failure detection and isolation
- Network partition handling
- Circuit breaker activation
- Automatic recovery mechanisms
- Cluster health maintenance (target: >60% healthy nodes)

#### 4. Metrics Validation Tests (`TestClusterMetricsAndAlerts`)
- **Purpose**: Validate metrics collection and alerting
- **Scope**: Prometheus metrics, alert generation, aggregation
- **Duration**: 15-20 seconds
- **Node Count**: 4-5 nodes

```go
func TestClusterMetricsAndAlerts(t *testing.T)
```

**What it tests:**
- Metrics collection from all nodes
- Alert generation under load
- Prometheus format compliance
- Cluster-wide metric aggregation
- Alert propagation and handling

## Configuration

### Environment Variables

The tests can be configured using environment variables:

```bash
# Basic configuration
export CLUSTER_NODE_COUNT=5
export CLUSTER_TEST_DURATION=30s
export CLUSTER_CONCURRENT_REQUESTS=100
export CLUSTER_FAILURE_RATE=0.05

# Advanced configuration
export CLUSTER_NETWORK_LATENCY=50ms
export CLUSTER_TEST_TIMEOUT=300s
export CLUSTER_TEST_VERBOSE=true
```

### Configuration File

Tests can also be configured using `cluster_test_config.json`:

```json
{
  "test_configurations": {
    "basic": {
      "node_count": 3,
      "test_duration": "10s",
      "concurrent_requests": 25
    },
    "performance": {
      "node_count": 5,
      "test_duration": "30s",
      "concurrent_requests": 100
    }
  }
}
```

## Running Tests

### Using Make (Recommended)

```bash
# Run all cluster tests
make -f Makefile.cluster-tests cluster-test-all

# Run specific test categories
make -f Makefile.cluster-tests cluster-test-basic
make -f Makefile.cluster-tests cluster-test-performance
make -f Makefile.cluster-tests cluster-test-fault-tolerance
make -f Makefile.cluster-tests cluster-test-metrics

# Run with custom configuration
make -f Makefile.cluster-tests cluster-test-all \
  CLUSTER_NODE_COUNT=8 \
  CLUSTER_TEST_DURATION=60s \
  CLUSTER_CONCURRENT_REQUESTS=200
```

### Using Go Test Directly

```bash
# Basic cluster tests
go test -v -timeout=300s -run="TestClusterNodeInteraction" ./bitswap/...

# Performance tests with custom parameters
CLUSTER_NODE_COUNT=5 CLUSTER_CONCURRENT_REQUESTS=100 \
go test -v -timeout=300s -run="TestClusterPerformanceUnderLoad" ./bitswap/...

# All cluster tests
go test -v -timeout=300s -run="TestCluster" ./bitswap/... ./monitoring/...
```

### CI/CD Integration

The tests are integrated with GitHub Actions via `.github/workflows/cluster-integration-tests.yml`:

```yaml
# Trigger tests on push/PR
on:
  push:
    branches: [ main, develop ]
  pull_request:
    branches: [ main, develop ]
  schedule:
    - cron: '0 2 * * *'  # Nightly runs
```

## Test Results and Reporting

### Automated Reporting

Tests generate comprehensive reports including:

- **Performance Metrics**: Throughput, latency, success rates
- **Resource Usage**: CPU, memory, network utilization
- **Health Status**: Node health, cluster stability
- **Alert Summary**: Generated alerts and their resolution

### Coverage Analysis

```bash
# Generate coverage report
make -f Makefile.cluster-tests cluster-test-coverage

# View coverage in browser
open coverage/cluster-coverage.html
```

### Benchmark Analysis

```bash
# Run benchmarks
make -f Makefile.cluster-tests cluster-test-benchmark

# Analyze performance profiles
make -f Makefile.cluster-tests cluster-test-analyze-benchmarks
```

## Performance Targets

### Success Criteria

| Metric | Target | Critical Threshold |
|--------|--------|--------------------|
| Success Rate | >95% | >80% |
| Average Latency | <200ms | <500ms |
| P95 Latency | <500ms | <1000ms |
| Throughput | >10 RPS | >5 RPS |
| Cluster Health | >90% | >60% |
| Alert Rate | <5% | <15% |

### Resource Limits

| Resource | Recommended | Minimum |
|----------|-------------|---------|
| Memory | 8GB | 4GB |
| CPU Cores | 8 | 4 |
| Disk Space | 10GB | 2GB |
| File Descriptors | 65536 | 8192 |

## Troubleshooting

### Common Issues

#### 1. Test Timeouts
```bash
# Increase timeout
export CLUSTER_TEST_TIMEOUT=600s
```

#### 2. Resource Exhaustion
```bash
# Check system resources
make -f Makefile.cluster-tests cluster-test-validate-env

# Reduce test load
export CLUSTER_NODE_COUNT=3
export CLUSTER_CONCURRENT_REQUESTS=25
```

#### 3. Network Issues
```bash
# Check network configuration
netstat -an | grep LISTEN
ss -tuln

# Adjust network settings
echo 'net.core.somaxconn = 65536' | sudo tee -a /etc/sysctl.conf
sudo sysctl -p
```

#### 4. File Descriptor Limits
```bash
# Check current limits
ulimit -n

# Increase limits
ulimit -n 65536
echo '* soft nofile 65536' | sudo tee -a /etc/security/limits.conf
echo '* hard nofile 65536' | sudo tee -a /etc/security/limits.conf
```

### Debug Mode

Enable verbose logging for debugging:

```bash
export CLUSTER_TEST_VERBOSE=true
export GOMAXPROCS=8

# Run with debug output
make -f Makefile.cluster-tests cluster-test-basic CLUSTER_TEST_VERBOSE=true
```

### Log Analysis

Test logs are saved in `test-results/` directory:

```bash
# View test logs
tail -f test-results/cluster-all.log

# Search for errors
grep -i error test-results/*.log

# Analyze performance patterns
grep -E "(latency|throughput|rps)" test-results/*.log
```

## Development Guidelines

### Adding New Tests

1. **Follow naming convention**: `TestCluster*`
2. **Use test utilities**: Leverage `cluster_test_utils.go` functions
3. **Configure appropriately**: Set reasonable timeouts and resource limits
4. **Document thoroughly**: Add comments explaining test purpose and expectations
5. **Validate results**: Use `ClusterTestValidator` for consistent validation

### Test Structure Template

```go
func TestClusterNewFeature(t *testing.T) {
    // Load configuration
    config := DefaultClusterTestConfig()
    config.NodeCount = 5
    config.TestDuration = 30 * time.Second
    
    // Setup environment
    env := setupClusterTestEnvironment(t, config)
    defer env.Cleanup()
    
    // Create context with timeout
    ctx, cancel := context.WithTimeout(context.Background(), config.TestDuration)
    defer cancel()
    
    // Start monitoring
    env.StartAllNodes(ctx)
    
    // Test implementation
    // ...
    
    // Validate results
    validator := DefaultClusterTestValidator()
    validator.AssertValidResults(t, results)
}
```

### Best Practices

1. **Resource Management**: Always clean up resources in defer statements
2. **Timeout Handling**: Use contexts with appropriate timeouts
3. **Error Handling**: Provide meaningful error messages and context
4. **Concurrency Safety**: Use proper synchronization for shared resources
5. **Test Isolation**: Ensure tests don't interfere with each other
6. **Performance Awareness**: Monitor test resource usage and execution time

## Integration with IPFS Cluster

These tests are designed to validate Boxo's behavior in IPFS cluster environments:

### Cluster Scenarios Tested

1. **Multi-node IPFS cluster** with shared content
2. **Load balancing** across cluster nodes
3. **Failover scenarios** when nodes become unavailable
4. **Content replication** and consistency
5. **Network partition tolerance**
6. **Performance under cluster load**

### Real-world Validation

The tests simulate realistic cluster conditions:

- **Variable network latency** between nodes
- **Intermittent failures** and recovery
- **High concurrent load** from multiple clients
- **Resource constraints** and pressure
- **Mixed workload patterns**

## Continuous Improvement

### Metrics Collection

Tests collect detailed metrics for analysis:

- **Performance trends** over time
- **Failure patterns** and root causes
- **Resource utilization** patterns
- **Scalability characteristics**

### Automated Analysis

CI/CD pipeline includes:

- **Performance regression detection**
- **Failure rate monitoring**
- **Resource usage trending**
- **Alert threshold tuning**

### Feedback Loop

Test results inform:

- **Configuration optimization**
- **Performance tuning**
- **Failure handling improvements**
- **Monitoring enhancements**

## Support and Maintenance

### Regular Updates

- **Test configurations** updated for new features
- **Performance targets** adjusted based on improvements
- **Failure scenarios** expanded for better coverage
- **Monitoring capabilities** enhanced with new metrics

### Community Contributions

- **Test case contributions** welcome
- **Performance improvements** encouraged
- **Bug reports** and fixes appreciated
- **Documentation updates** valued

For questions or issues with cluster integration tests, please:

1. Check this documentation first
2. Review existing GitHub issues
3. Run tests with verbose logging
4. Create detailed bug reports with logs and configuration
5. Contribute improvements back to the project