#!/bin/bash

# Boxo High-Load Basic Benchmark Script
# This script runs comprehensive performance tests against a Boxo cluster

set -e

# Configuration
CLUSTER_API_URL="${CLUSTER_API_URL:-http://localhost:9094}"
IPFS_API_URL="${IPFS_API_URL:-http://localhost:5001}"
TEST_DURATION="${TEST_DURATION:-300}"  # 5 minutes
CONCURRENT_USERS="${CONCURRENT_USERS:-100}"
RAMP_UP_TIME="${RAMP_UP_TIME:-60}"     # 1 minute
RESULTS_DIR="./results/$(date +%Y%m%d-%H%M%S)"

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

echo -e "${BLUE}=== Boxo High-Load Benchmark Suite ===${NC}"
echo "Cluster API: $CLUSTER_API_URL"
echo "IPFS API: $IPFS_API_URL"
echo "Test Duration: ${TEST_DURATION}s"
echo "Concurrent Users: $CONCURRENT_USERS"
echo "Results Directory: $RESULTS_DIR"
echo

# Create results directory
mkdir -p "$RESULTS_DIR"

# Function to log with timestamp
log() {
    echo -e "[$(date '+%Y-%m-%d %H:%M:%S')] $1"
}

# Function to check if service is available
check_service() {
    local url=$1
    local name=$2
    
    log "${YELLOW}Checking $name availability...${NC}"
    if curl -s --max-time 5 "$url/api/v0/version" > /dev/null 2>&1; then
        log "${GREEN}✓ $name is available${NC}"
        return 0
    else
        log "${RED}✗ $name is not available at $url${NC}"
        return 1
    fi
}

# Function to generate test data
generate_test_data() {
    log "${YELLOW}Generating test data...${NC}"
    
    # Create test files of various sizes
    mkdir -p "$RESULTS_DIR/test-data"
    
    # Small files (1KB - 10KB)
    for i in {1..10}; do
        dd if=/dev/urandom of="$RESULTS_DIR/test-data/small-$i.bin" bs=1024 count=$((RANDOM % 10 + 1)) 2>/dev/null
    done
    
    # Medium files (100KB - 1MB)
    for i in {1..5}; do
        dd if=/dev/urandom of="$RESULTS_DIR/test-data/medium-$i.bin" bs=1024 count=$((RANDOM % 900 + 100)) 2>/dev/null
    done
    
    # Large files (1MB - 10MB)
    for i in {1..3}; do
        dd if=/dev/urandom of="$RESULTS_DIR/test-data/large-$i.bin" bs=1048576 count=$((RANDOM % 9 + 1)) 2>/dev/null
    done
    
    log "${GREEN}✓ Test data generated${NC}"
}

# Function to add files to IPFS and get CIDs
add_test_files() {
    log "${YELLOW}Adding test files to IPFS...${NC}"
    
    local cids_file="$RESULTS_DIR/test-cids.txt"
    > "$cids_file"  # Clear file
    
    for file in "$RESULTS_DIR/test-data"/*; do
        if [ -f "$file" ]; then
            local cid=$(curl -s -X POST -F "file=@$file" "$IPFS_API_URL/api/v0/add?quiet=true")
            if [ -n "$cid" ]; then
                echo "$cid" >> "$cids_file"
                log "Added $(basename "$file"): $cid"
            fi
        fi
    done
    
    local cid_count=$(wc -l < "$cids_file")
    log "${GREEN}✓ Added $cid_count files to IPFS${NC}"
}

# Function to run pin performance test
run_pin_test() {
    log "${YELLOW}Running pin performance test...${NC}"
    
    local cids_file="$RESULTS_DIR/test-cids.txt"
    local results_file="$RESULTS_DIR/pin-test-results.json"
    
    # Create test script for concurrent pinning
    cat > "$RESULTS_DIR/pin-test.sh" << 'EOF'
#!/bin/bash
CLUSTER_API_URL=$1
CID=$2
RESULTS_FILE=$3

start_time=$(date +%s.%N)
response=$(curl -s -w "%{http_code},%{time_total}" -X POST "$CLUSTER_API_URL/api/v0/pins/$CID")
end_time=$(date +%s.%N)

http_code=$(echo "$response" | tail -c 4)
time_total=$(echo "$response" | sed 's/.*,//')
duration=$(echo "$end_time - $start_time" | bc -l)

echo "{\"cid\":\"$CID\",\"http_code\":$http_code,\"duration\":$duration,\"timestamp\":$(date +%s)}" >> "$RESULTS_FILE"
EOF
    chmod +x "$RESULTS_DIR/pin-test.sh"
    
    # Run concurrent pin operations
    > "$results_file"  # Clear results file
    
    local pids=()
    while IFS= read -r cid; do
        for i in $(seq 1 $((CONCURRENT_USERS / 10))); do
            "$RESULTS_DIR/pin-test.sh" "$CLUSTER_API_URL" "$cid" "$results_file" &
            pids+=($!)
        done
    done < "$cids_file"
    
    # Wait for all background processes
    for pid in "${pids[@]}"; do
        wait "$pid"
    done
    
    # Analyze results
    local total_requests=$(wc -l < "$results_file")
    local successful_requests=$(grep '"http_code":20[0-9]' "$results_file" | wc -l)
    local avg_duration=$(jq -r '.duration' "$results_file" | awk '{sum+=$1} END {print sum/NR}')
    
    log "${GREEN}✓ Pin test completed${NC}"
    log "  Total requests: $total_requests"
    log "  Successful requests: $successful_requests"
    log "  Success rate: $(echo "scale=2; $successful_requests * 100 / $total_requests" | bc -l)%"
    log "  Average duration: ${avg_duration}s"
}

# Function to run retrieval performance test
run_retrieval_test() {
    log "${YELLOW}Running retrieval performance test...${NC}"
    
    local cids_file="$RESULTS_DIR/test-cids.txt"
    local results_file="$RESULTS_DIR/retrieval-test-results.json"
    
    # Create test script for concurrent retrieval
    cat > "$RESULTS_DIR/retrieval-test.sh" << 'EOF'
#!/bin/bash
CLUSTER_API_URL=$1
CID=$2
RESULTS_FILE=$3

start_time=$(date +%s.%N)
response=$(curl -s -w "%{http_code},%{time_total},%{size_download}" -X GET "$CLUSTER_API_URL/api/v0/pins/$CID" -o /dev/null)
end_time=$(date +%s.%N)

http_code=$(echo "$response" | cut -d',' -f1)
time_total=$(echo "$response" | cut -d',' -f2)
size_download=$(echo "$response" | cut -d',' -f3)
duration=$(echo "$end_time - $start_time" | bc -l)

echo "{\"cid\":\"$CID\",\"http_code\":$http_code,\"duration\":$duration,\"size\":$size_download,\"timestamp\":$(date +%s)}" >> "$RESULTS_FILE"
EOF
    chmod +x "$RESULTS_DIR/retrieval-test.sh"
    
    # Run concurrent retrieval operations
    > "$results_file"  # Clear results file
    
    local pids=()
    for round in $(seq 1 5); do  # 5 rounds of testing
        while IFS= read -r cid; do
            for i in $(seq 1 $((CONCURRENT_USERS / 20))); do
                "$RESULTS_DIR/retrieval-test.sh" "$CLUSTER_API_URL" "$cid" "$results_file" &
                pids+=($!)
            done
        done < "$cids_file"
        
        # Wait a bit between rounds
        sleep 2
    done
    
    # Wait for all background processes
    for pid in "${pids[@]}"; do
        wait "$pid"
    done
    
    # Analyze results
    local total_requests=$(wc -l < "$results_file")
    local successful_requests=$(grep '"http_code":20[0-9]' "$results_file" | wc -l)
    local avg_duration=$(jq -r '.duration' "$results_file" | awk '{sum+=$1} END {print sum/NR}')
    local total_bytes=$(jq -r '.size' "$results_file" | awk '{sum+=$1} END {print sum}')
    
    log "${GREEN}✓ Retrieval test completed${NC}"
    log "  Total requests: $total_requests"
    log "  Successful requests: $successful_requests"
    log "  Success rate: $(echo "scale=2; $successful_requests * 100 / $total_requests" | bc -l)%"
    log "  Average duration: ${avg_duration}s"
    log "  Total bytes transferred: $(echo "$total_bytes" | numfmt --to=iec)"
}

# Function to run stress test
run_stress_test() {
    log "${YELLOW}Running stress test...${NC}"
    
    local results_file="$RESULTS_DIR/stress-test-results.json"
    
    # Create stress test script
    cat > "$RESULTS_DIR/stress-test.sh" << 'EOF'
#!/bin/bash
CLUSTER_API_URL=$1
DURATION=$2
RESULTS_FILE=$3
USER_ID=$4

end_time=$(($(date +%s) + DURATION))
request_count=0

while [ $(date +%s) -lt $end_time ]; do
    start_time=$(date +%s.%N)
    response=$(curl -s -w "%{http_code},%{time_total}" -X GET "$CLUSTER_API_URL/api/v0/peers" -o /dev/null)
    end_request_time=$(date +%s.%N)
    
    http_code=$(echo "$response" | cut -d',' -f1)
    time_total=$(echo "$response" | cut -d',' -f2)
    duration=$(echo "$end_request_time - $start_time" | bc -l)
    
    echo "{\"user_id\":$USER_ID,\"request_id\":$request_count,\"http_code\":$http_code,\"duration\":$duration,\"timestamp\":$(date +%s)}" >> "$RESULTS_FILE"
    
    request_count=$((request_count + 1))
    
    # Small delay to prevent overwhelming
    sleep 0.01
done

echo "User $USER_ID completed $request_count requests"
EOF
    chmod +x "$RESULTS_DIR/stress-test.sh"
    
    # Run stress test with multiple concurrent users
    > "$results_file"  # Clear results file
    
    local pids=()
    for user_id in $(seq 1 $CONCURRENT_USERS); do
        "$RESULTS_DIR/stress-test.sh" "$CLUSTER_API_URL" "$TEST_DURATION" "$results_file" "$user_id" &
        pids+=($!)
        
        # Gradual ramp-up
        if [ $((user_id % 10)) -eq 0 ]; then
            sleep $((RAMP_UP_TIME / (CONCURRENT_USERS / 10)))
        fi
    done
    
    log "Started $CONCURRENT_USERS concurrent users, waiting for completion..."
    
    # Wait for all users to complete
    for pid in "${pids[@]}"; do
        wait "$pid"
    done
    
    # Analyze stress test results
    local total_requests=$(wc -l < "$results_file")
    local successful_requests=$(grep '"http_code":20[0-9]' "$results_file" | wc -l)
    local avg_duration=$(jq -r '.duration' "$results_file" | awk '{sum+=$1} END {print sum/NR}')
    local p95_duration=$(jq -r '.duration' "$results_file" | sort -n | awk '{all[NR] = $0} END{print all[int(NR*0.95)]}')
    local requests_per_second=$(echo "scale=2; $total_requests / $TEST_DURATION" | bc -l)
    
    log "${GREEN}✓ Stress test completed${NC}"
    log "  Total requests: $total_requests"
    log "  Successful requests: $successful_requests"
    log "  Success rate: $(echo "scale=2; $successful_requests * 100 / $total_requests" | bc -l)%"
    log "  Requests per second: $requests_per_second"
    log "  Average response time: ${avg_duration}s"
    log "  95th percentile response time: ${p95_duration}s"
}

# Function to collect system metrics during test
collect_metrics() {
    log "${YELLOW}Collecting system metrics...${NC}"
    
    local metrics_file="$RESULTS_DIR/system-metrics.json"
    
    # Collect metrics from Prometheus if available
    if curl -s --max-time 5 "http://localhost:9090/api/v1/query?query=up" > /dev/null 2>&1; then
        # Collect key metrics
        local queries=(
            "rate(ipfs_cluster_api_requests_total[5m])"
            "histogram_quantile(0.95, rate(ipfs_cluster_api_request_duration_seconds_bucket[5m]))"
            "process_resident_memory_bytes"
            "ipfs_cluster_peers_connected"
            "rate(ipfs_bitswap_blocks_received_total[5m])"
        )
        
        for query in "${queries[@]}"; do
            curl -s "http://localhost:9090/api/v1/query?query=$(echo "$query" | sed 's/ /%20/g')" >> "$metrics_file"
            echo >> "$metrics_file"
        done
        
        log "${GREEN}✓ Metrics collected from Prometheus${NC}"
    else
        log "${YELLOW}⚠ Prometheus not available, skipping metrics collection${NC}"
    fi
}

# Function to generate performance report
generate_report() {
    log "${YELLOW}Generating performance report...${NC}"
    
    local report_file="$RESULTS_DIR/performance-report.md"
    
    cat > "$report_file" << EOF
# Boxo High-Load Performance Test Report

**Test Date:** $(date)
**Test Duration:** ${TEST_DURATION} seconds
**Concurrent Users:** ${CONCURRENT_USERS}
**Cluster API:** ${CLUSTER_API_URL}

## Test Configuration

- Ramp-up Time: ${RAMP_UP_TIME} seconds
- Test Data: Various file sizes (1KB - 10MB)
- Test Types: Pin operations, Retrieval operations, Stress testing

## Results Summary

### Pin Performance Test
EOF

    if [ -f "$RESULTS_DIR/pin-test-results.json" ]; then
        local pin_total=$(wc -l < "$RESULTS_DIR/pin-test-results.json")
        local pin_success=$(grep '"http_code":20[0-9]' "$RESULTS_DIR/pin-test-results.json" | wc -l)
        local pin_avg=$(jq -r '.duration' "$RESULTS_DIR/pin-test-results.json" | awk '{sum+=$1} END {print sum/NR}')
        
        cat >> "$report_file" << EOF

- Total Pin Requests: ${pin_total}
- Successful Requests: ${pin_success}
- Success Rate: $(echo "scale=2; $pin_success * 100 / $pin_total" | bc -l)%
- Average Response Time: ${pin_avg}s

EOF
    fi

    cat >> "$report_file" << EOF
### Retrieval Performance Test
EOF

    if [ -f "$RESULTS_DIR/retrieval-test-results.json" ]; then
        local ret_total=$(wc -l < "$RESULTS_DIR/retrieval-test-results.json")
        local ret_success=$(grep '"http_code":20[0-9]' "$RESULTS_DIR/retrieval-test-results.json" | wc -l)
        local ret_avg=$(jq -r '.duration' "$RESULTS_DIR/retrieval-test-results.json" | awk '{sum+=$1} END {print sum/NR}')
        
        cat >> "$report_file" << EOF

- Total Retrieval Requests: ${ret_total}
- Successful Requests: ${ret_success}
- Success Rate: $(echo "scale=2; $ret_success * 100 / $ret_total" | bc -l)%
- Average Response Time: ${ret_avg}s

EOF
    fi

    cat >> "$report_file" << EOF
### Stress Test Results
EOF

    if [ -f "$RESULTS_DIR/stress-test-results.json" ]; then
        local stress_total=$(wc -l < "$RESULTS_DIR/stress-test-results.json")
        local stress_success=$(grep '"http_code":20[0-9]' "$RESULTS_DIR/stress-test-results.json" | wc -l)
        local stress_avg=$(jq -r '.duration' "$RESULTS_DIR/stress-test-results.json" | awk '{sum+=$1} END {print sum/NR}')
        local stress_p95=$(jq -r '.duration' "$RESULTS_DIR/stress-test-results.json" | sort -n | awk '{all[NR] = $0} END{print all[int(NR*0.95)]}')
        local stress_rps=$(echo "scale=2; $stress_total / $TEST_DURATION" | bc -l)
        
        cat >> "$report_file" << EOF

- Total Requests: ${stress_total}
- Successful Requests: ${stress_success}
- Success Rate: $(echo "scale=2; $stress_success * 100 / $stress_total" | bc -l)%
- Requests per Second: ${stress_rps}
- Average Response Time: ${stress_avg}s
- 95th Percentile Response Time: ${stress_p95}s

EOF
    fi

    cat >> "$report_file" << EOF
## Performance Targets

| Metric | Target | Actual | Status |
|--------|--------|--------|--------|
| Response Time P95 | < 100ms | ${stress_p95:-N/A}s | $([ "${stress_p95:-1}" \< "0.1" ] && echo "✅ PASS" || echo "❌ FAIL") |
| Success Rate | > 99% | $(echo "scale=2; $stress_success * 100 / $stress_total" | bc -l)% | $([ "$(echo "$stress_success * 100 / $stress_total > 99" | bc -l)" = "1" ] && echo "✅ PASS" || echo "❌ FAIL") |
| Throughput | > 1000 req/s | ${stress_rps:-N/A} req/s | $([ "$(echo "${stress_rps:-0} > 1000" | bc -l)" = "1" ] && echo "✅ PASS" || echo "❌ FAIL") |

## Files Generated

- Test Data: \`test-data/\`
- Pin Test Results: \`pin-test-results.json\`
- Retrieval Test Results: \`retrieval-test-results.json\`
- Stress Test Results: \`stress-test-results.json\`
- System Metrics: \`system-metrics.json\`

## Recommendations

Based on the test results:

1. **If response times are high (> 100ms):** Consider increasing worker pool sizes or optimizing batch processing
2. **If success rate is low (< 99%):** Check error logs and consider implementing circuit breakers
3. **If throughput is low (< 1000 req/s):** Review connection limits and network configuration

For detailed optimization guidance, see the [Configuration Guide](../../configuration-guide.md).
EOF

    log "${GREEN}✓ Performance report generated: $report_file${NC}"
}

# Main execution
main() {
    log "${BLUE}Starting Boxo High-Load Benchmark${NC}"
    
    # Pre-flight checks
    if ! check_service "$CLUSTER_API_URL" "IPFS Cluster"; then
        log "${RED}Cannot proceed without IPFS Cluster${NC}"
        exit 1
    fi
    
    if ! check_service "$IPFS_API_URL" "IPFS Node"; then
        log "${RED}Cannot proceed without IPFS Node${NC}"
        exit 1
    fi
    
    # Check required tools
    for tool in curl jq bc numfmt; do
        if ! command -v "$tool" &> /dev/null; then
            log "${RED}Required tool '$tool' is not installed${NC}"
            exit 1
        fi
    done
    
    # Run benchmark tests
    generate_test_data
    add_test_files
    
    # Start metrics collection in background
    collect_metrics &
    local metrics_pid=$!
    
    # Run performance tests
    run_pin_test
    run_retrieval_test
    run_stress_test
    
    # Stop metrics collection
    kill $metrics_pid 2>/dev/null || true
    
    # Generate final report
    generate_report
    
    log "${GREEN}✅ Benchmark completed successfully!${NC}"
    log "${BLUE}Results available in: $RESULTS_DIR${NC}"
    log "${BLUE}Performance report: $RESULTS_DIR/performance-report.md${NC}"
}

# Run main function
main "$@"