# Bitswap Load Testing Suite

This directory contains comprehensive load testing implementation for Bitswap, designed to validate performance under high-load conditions as specified in task 7.1.

## Overview

The load testing suite implements the following requirements:
- **10,000+ concurrent connections**: Tests system behavior with massive concurrent load
- **100,000+ requests per second**: Validates high-throughput performance
- **24+ hour stability tests**: Ensures system stability over extended periods
- **Automated performance benchmarks**: Provides consistent performance validation

## Quick Start

### Prerequisites

- Go 1.19+ 
- Minimum 8GB RAM (16GB+ recommended)
- 4+ CPU cores (8+ recommended)
- 10GB+ free disk space

### Basic Usage

```bash
# Run all load tests (may take several hours)
make -f Makefile.loadtest test-all

# Run individual test categories
make -f Makefile.loadtest test-concurrent    # 10K+ connections
make -f Makefile.loadtest test-throughput    # 100K+ req/sec
make -f Makefile.loadtest test-stability     # Long-running stability
make -f Makefile.loadtest test-benchmarks    # Automated benchmarks

# Run CI-friendly tests (shorter duration)
make -f Makefile.loadtest test-ci
```

### Environment Configuration

```bash
# Set test duration for stability tests
export BITSWAP_STABILITY_DURATION=24h

# Set output directory
export OUTPUT_DIR=./my_test_results

# Set CPU cores
export GOMAXPROCS=16
```

## Test Categories

### 1. Concurrent Connection Tests

Tests system behavior with 10,000+ simultaneous connections:

- **10K Connections**: 10,000 concurrent connections, 5-minute duration
- **15K Connections**: 15,000 concurrent connections, 4-minute duration  
- **20K Connections**: 20,000 concurrent connections, 3-minute duration
- **25K Connections**: 25,000 concurrent connections, 2-minute duration

**Requirements Validated**:
- Average latency < 100ms for 95% of requests
- P95 latency < 100ms
- Success rate > 95%

### 2. High Throughput Tests

Tests system ability to handle 100,000+ requests per second:

- **50K RPS**: 50,000 requests/sec, 80% achievement target
- **75K RPS**: 75,000 requests/sec, 70% achievement target
- **100K RPS**: 100,000 requests/sec, 60% achievement target
- **125K RPS**: 125,000 requests/sec, 50% achievement target
- **150K RPS**: 150,000 requests/sec, 40% achievement target

**Requirements Validated**:
- Throughput achievement above minimum thresholds
- P95 latency < 200ms under high load
- System auto-scaling behavior

### 3. Stability Tests

Tests system stability over extended periods (24+ hours configurable):

- **Light Load Stability**: 500 req/sec sustained load
- **Medium Load Stability**: 1,000 req/sec sustained load  
- **High Load Stability**: 2,000 req/sec sustained load

**Requirements Validated**:
- Memory growth rate < 5% over test duration
- Success rate > 98% throughout test
- No memory leaks detected
- Consistent performance over time

### 4. Automated Benchmarks

Provides consistent performance validation across different scales:

- **Small Scale**: 10 nodes, 500 blocks, 100 concurrent requests
- **Medium Scale**: 25 nodes, 1,000 blocks, 500 concurrent requests
- **Large Scale**: 50 nodes, 2,000 blocks, 1,000 concurrent requests
- **XLarge Scale**: 100 nodes, 5,000 blocks, 2,000 concurrent requests

**Metrics Collected**:
- Requests per second
- Efficiency (req/sec per node)
- Memory efficiency (req/sec per MB)
- Latency scores
- Resource utilization

### 5. Extreme Load Scenarios

Tests system behavior under extreme conditions:

- **50K Concurrent Connections**: Maximum connection stress test
- **200K Requests/Second**: Maximum throughput stress test
- **10MB Block Sizes**: Large block handling test
- **100K Tiny Blocks**: Many small blocks test

### 6. Memory Leak Detection

Validates system memory management over multiple test cycles:

- **Short Cycles**: 15-minute duration, 10 cycles
- **Long Cycles**: 30-minute duration, 5 cycles

**Validation**:
- Overall memory growth < 10%
- Per-cycle memory growth < 5%
- No goroutine leaks

### 7. Network Condition Tests

Tests performance under various network conditions:

- **Perfect Network**: 0ms latency
- **LAN Conditions**: 1ms latency
- **Fast WAN**: 10ms latency
- **Slow WAN**: 50ms latency
- **Satellite**: 200ms latency
- **Very Poor**: 500ms latency

## Test Infrastructure

### Core Components

- **`types.go`**: Test configuration and result structures
- **`load_test_core.go`**: Core load testing engine
- **`utils.go`**: Utility functions and helpers
- **`comprehensive_load_test.go`**: Main test suite implementation
- **`high_load_tests.go`**: High-load specific tests
- **`performance_bench_test.go`**: Go benchmark tests
- **`stress_test.go`**: Stress and failure recovery tests
- **`test_runner.go`**: Test suite orchestration

### Configuration

Tests can be configured via:

1. **JSON Configuration File**: `bitswap_load_test_config.json`
2. **Environment Variables**: `BITSWAP_STABILITY_DURATION`, `OUTPUT_DIR`, etc.
3. **Command Line Parameters**: Via Go test flags

Example configuration:
```json
{
  "test_timeout": "30m",
  "stability_duration": "24h",
  "max_concurrent_connections": 25000,
  "max_request_rate": 150000,
  "enable_long_running": true,
  "enable_stress_tests": true,
  "enable_benchmarks": true,
  "output_dir": "test_results",
  "generate_reports": true
}
```

### Results and Reporting

Test results are saved in multiple formats:

- **JSON Results**: Detailed metrics for each test
- **Summary Reports**: Human-readable test summaries
- **Benchmark Data**: Performance benchmark results
- **System Monitoring**: Resource usage during tests

Results include:
- Request counts and success rates
- Latency distributions (average, P95, P99, max)
- Throughput measurements
- Memory usage and growth rates
- Goroutine counts and connection metrics
- System resource utilization

## Advanced Usage

### Profiling

```bash
# CPU profiling
make -f Makefile.loadtest profile-cpu

# Memory profiling  
make -f Makefile.loadtest profile-memory

# Race detection
make -f Makefile.loadtest test-race
```

### Monitoring

```bash
# Monitor system resources during tests
make -f Makefile.loadtest monitor

# Generate coverage reports
make -f Makefile.loadtest test-coverage
```

### Docker Support

```bash
# Run tests in isolated Docker environment
make -f Makefile.loadtest docker-test
```

## Performance Requirements

The test suite validates the following performance requirements from the specification:

### Requirement 1.1: High Concurrency
- Handle 1,000+ concurrent requests with <100ms response time for 95% of requests
- Auto-scale connection pools when load exceeds 10,000 requests/sec
- Prioritize critical requests during peak load
- Activate cache shedding when memory exceeds 80%

### Requirement 1.2: Auto-scaling
- System should automatically scale connection pools for high load
- Maintain performance characteristics as load increases
- Graceful degradation under extreme load conditions

## Troubleshooting

### Common Issues

1. **Insufficient Memory**
   ```
   Error: insufficient memory: need at least 2GB
   ```
   Solution: Increase system memory or reduce test scale

2. **Too Few CPU Cores**
   ```
   Error: insufficient CPU cores: recommend at least 2
   ```
   Solution: Increase GOMAXPROCS or run on system with more cores

3. **Test Timeouts**
   ```
   Error: test exceeded timeout
   ```
   Solution: Increase test timeout or reduce test duration

4. **Port Exhaustion**
   ```
   Error: cannot assign requested address
   ```
   Solution: Increase system ulimits or reduce concurrent connections

### Performance Tuning

For optimal performance:

1. **System Configuration**:
   ```bash
   # Increase file descriptor limits
   ulimit -n 65536
   
   # Increase network buffer sizes
   echo 'net.core.rmem_max = 134217728' >> /etc/sysctl.conf
   echo 'net.core.wmem_max = 134217728' >> /etc/sysctl.conf
   ```

2. **Go Runtime Tuning**:
   ```bash
   # Set CPU cores
   export GOMAXPROCS=16
   
   # Tune garbage collector
   export GOGC=100
   ```

3. **Test Configuration**:
   - Start with smaller scales and increase gradually
   - Monitor system resources during tests
   - Use appropriate test durations for your environment

## Integration with CI/CD

### GitHub Actions Example

```yaml
name: Bitswap Load Tests
on: [push, pull_request]

jobs:
  load-tests:
    runs-on: ubuntu-latest
    steps:
    - uses: actions/checkout@v3
    - uses: actions/setup-go@v3
      with:
        go-version: '1.21'
    - name: Run Load Tests
      run: |
        cd bitswap
        make -f Makefile.loadtest test-ci
    - name: Upload Results
      uses: actions/upload-artifact@v3
      with:
        name: load-test-results
        path: bitswap/test_results/
```

### Jenkins Pipeline Example

```groovy
pipeline {
    agent any
    stages {
        stage('Load Tests') {
            steps {
                dir('bitswap') {
                    sh 'make -f Makefile.loadtest test-ci'
                }
            }
            post {
                always {
                    archiveArtifacts artifacts: 'bitswap/test_results/**/*'
                    publishHTML([
                        allowMissing: false,
                        alwaysLinkToLastBuild: true,
                        keepAll: true,
                        reportDir: 'bitswap/test_results',
                        reportFiles: 'load_test_summary.txt',
                        reportName: 'Load Test Report'
                    ])
                }
            }
        }
    }
}
```

## Contributing

When adding new load tests:

1. Follow the existing test structure and naming conventions
2. Include appropriate requirement validation
3. Add comprehensive logging and metrics collection
4. Update this README with new test descriptions
5. Ensure tests are configurable and can run in CI environments

## Support

For issues or questions about the load testing suite:

1. Check the troubleshooting section above
2. Review test logs in the output directory
3. Validate system requirements and configuration
4. Consider running smaller-scale tests first to isolate issues