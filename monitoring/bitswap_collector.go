package monitoring

import (
	"context"
	"sync"
	"time"
)

// BitswapCollector collects Bitswap-specific performance metrics
type BitswapCollector struct {
	mu                    sync.RWMutex
	requestCount          int64
	responseTimeSum       time.Duration
	responseTimeCount     int64
	responseTimeP95       time.Duration
	responseTimeP99       time.Duration
	outstandingRequests   int64
	activeConnections     int
	queuedRequests        int64
	blocksReceived        int64
	blocksSent            int64
	duplicateBlocks       int64
	errorCount            int64
	totalRequests         int64
	
	// Response time tracking for percentiles
	responseTimes         []time.Duration
	maxResponseTimes      int
}

// NewBitswapCollector creates a new Bitswap metrics collector
func NewBitswapCollector() *BitswapCollector {
	return &BitswapCollector{
		maxResponseTimes: 1000, // Keep last 1000 response times for percentile calculation
		responseTimes:    make([]time.Duration, 0, 1000),
	}
}

// CollectMetrics implements MetricCollector interface
func (bc *BitswapCollector) CollectMetrics(ctx context.Context) (map[string]interface{}, error) {
	bc.mu.RLock()
	defer bc.mu.RUnlock()
	
	var avgResponseTime time.Duration
	if bc.responseTimeCount > 0 {
		avgResponseTime = time.Duration(int64(bc.responseTimeSum) / bc.responseTimeCount)
	}
	
	var requestsPerSecond float64
	if bc.totalRequests > 0 {
		// Calculate requests per second over the last collection period
		// This is a simplified calculation - in a real implementation,
		// you'd track time windows more precisely
		requestsPerSecond = float64(bc.requestCount)
	}
	
	var errorRate float64
	if bc.totalRequests > 0 {
		errorRate = float64(bc.errorCount) / float64(bc.totalRequests)
	}
	
	metrics := map[string]interface{}{
		"requests_per_second":     requestsPerSecond,
		"average_response_time":   avgResponseTime,
		"p95_response_time":       bc.responseTimeP95,
		"p99_response_time":       bc.responseTimeP99,
		"outstanding_requests":    bc.outstandingRequests,
		"active_connections":      bc.activeConnections,
		"queued_requests":         bc.queuedRequests,
		"blocks_received":         bc.blocksReceived,
		"blocks_sent":             bc.blocksSent,
		"duplicate_blocks":        bc.duplicateBlocks,
		"error_rate":              errorRate,
		"total_requests":          bc.totalRequests,
	}
	
	// Reset counters for next collection period
	bc.requestCount = 0
	bc.responseTimeSum = 0
	bc.responseTimeCount = 0
	
	return metrics, nil
}

// GetMetricNames returns the list of metric names this collector provides
func (bc *BitswapCollector) GetMetricNames() []string {
	return []string{
		"requests_per_second",
		"average_response_time",
		"p95_response_time",
		"p99_response_time",
		"outstanding_requests",
		"active_connections",
		"queued_requests",
		"blocks_received",
		"blocks_sent",
		"duplicate_blocks",
		"error_rate",
		"total_requests",
	}
}

// RecordRequest records a new Bitswap request
func (bc *BitswapCollector) RecordRequest() {
	bc.mu.Lock()
	defer bc.mu.Unlock()
	
	bc.requestCount++
	bc.totalRequests++
	bc.outstandingRequests++
}

// RecordResponse records a Bitswap response with its duration
func (bc *BitswapCollector) RecordResponse(duration time.Duration, success bool) {
	bc.mu.Lock()
	defer bc.mu.Unlock()
	
	bc.responseTimeSum += duration
	bc.responseTimeCount++
	bc.outstandingRequests--
	
	if !success {
		bc.errorCount++
	}
	
	// Add to response times for percentile calculation
	bc.responseTimes = append(bc.responseTimes, duration)
	if len(bc.responseTimes) > bc.maxResponseTimes {
		// Remove oldest entry
		bc.responseTimes = bc.responseTimes[1:]
	}
	
	// Update percentiles
	bc.updatePercentiles()
}

// RecordBlockReceived records a received block
func (bc *BitswapCollector) RecordBlockReceived(isDuplicate bool) {
	bc.mu.Lock()
	defer bc.mu.Unlock()
	
	bc.blocksReceived++
	if isDuplicate {
		bc.duplicateBlocks++
	}
}

// RecordBlockSent records a sent block
func (bc *BitswapCollector) RecordBlockSent() {
	bc.mu.Lock()
	defer bc.mu.Unlock()
	
	bc.blocksSent++
}

// UpdateActiveConnections updates the count of active connections
func (bc *BitswapCollector) UpdateActiveConnections(count int) {
	bc.mu.Lock()
	defer bc.mu.Unlock()
	
	bc.activeConnections = count
}

// UpdateQueuedRequests updates the count of queued requests
func (bc *BitswapCollector) UpdateQueuedRequests(count int64) {
	bc.mu.Lock()
	defer bc.mu.Unlock()
	
	bc.queuedRequests = count
}

// updatePercentiles calculates P95 and P99 response times
func (bc *BitswapCollector) updatePercentiles() {
	if len(bc.responseTimes) == 0 {
		return
	}
	
	// Create a copy and sort it
	times := make([]time.Duration, len(bc.responseTimes))
	copy(times, bc.responseTimes)
	
	// Simple bubble sort for small arrays
	for i := 0; i < len(times); i++ {
		for j := i + 1; j < len(times); j++ {
			if times[i] > times[j] {
				times[i], times[j] = times[j], times[i]
			}
		}
	}
	
	// Calculate percentiles
	p95Index := int(float64(len(times)) * 0.95)
	p99Index := int(float64(len(times)) * 0.99)
	
	if p95Index >= len(times) {
		p95Index = len(times) - 1
	}
	if p99Index >= len(times) {
		p99Index = len(times) - 1
	}
	
	bc.responseTimeP95 = times[p95Index]
	bc.responseTimeP99 = times[p99Index]
}