package monitoring

import (
	"context"
	"sync"
	"time"
)

// BlockstoreCollector collects blockstore-specific performance metrics
type BlockstoreCollector struct {
	mu                    sync.RWMutex
	cacheHits             int64
	cacheMisses           int64
	readLatencySum        time.Duration
	readLatencyCount      int64
	writeLatencySum       time.Duration
	writeLatencyCount     int64
	batchOperations       int64
	totalBlocks           int64
	cacheSize             int64
	diskUsage             int64
	compressionRatio      float64
	
	// Latency tracking
	readLatencies         []time.Duration
	writeLatencies        []time.Duration
	maxLatencies          int
}

// NewBlockstoreCollector creates a new blockstore metrics collector
func NewBlockstoreCollector() *BlockstoreCollector {
	return &BlockstoreCollector{
		maxLatencies:   1000, // Keep last 1000 latencies for analysis
		readLatencies:  make([]time.Duration, 0, 1000),
		writeLatencies: make([]time.Duration, 0, 1000),
	}
}

// CollectMetrics implements MetricCollector interface
func (bc *BlockstoreCollector) CollectMetrics(ctx context.Context) (map[string]interface{}, error) {
	bc.mu.RLock()
	defer bc.mu.RUnlock()
	
	var cacheHitRate float64
	totalCacheAccess := bc.cacheHits + bc.cacheMisses
	if totalCacheAccess > 0 {
		cacheHitRate = float64(bc.cacheHits) / float64(totalCacheAccess)
	}
	
	var avgReadLatency time.Duration
	if bc.readLatencyCount > 0 {
		avgReadLatency = time.Duration(int64(bc.readLatencySum) / bc.readLatencyCount)
	}
	
	var avgWriteLatency time.Duration
	if bc.writeLatencyCount > 0 {
		avgWriteLatency = time.Duration(int64(bc.writeLatencySum) / bc.writeLatencyCount)
	}
	
	var batchOpsPerSec float64
	if bc.batchOperations > 0 {
		// Simplified calculation - in real implementation, track time windows
		batchOpsPerSec = float64(bc.batchOperations)
	}
	
	metrics := map[string]interface{}{
		"cache_hit_rate":           cacheHitRate,
		"average_read_latency":     avgReadLatency,
		"average_write_latency":    avgWriteLatency,
		"batch_operations_per_sec": batchOpsPerSec,
		"compression_ratio":        bc.compressionRatio,
		"total_blocks":             bc.totalBlocks,
		"cache_size":               bc.cacheSize,
		"disk_usage":               bc.diskUsage,
		"cache_hits":               bc.cacheHits,
		"cache_misses":             bc.cacheMisses,
	}
	
	// Reset counters for next collection period
	bc.cacheHits = 0
	bc.cacheMisses = 0
	bc.readLatencySum = 0
	bc.readLatencyCount = 0
	bc.writeLatencySum = 0
	bc.writeLatencyCount = 0
	bc.batchOperations = 0
	
	return metrics, nil
}

// GetMetricNames returns the list of metric names this collector provides
func (bc *BlockstoreCollector) GetMetricNames() []string {
	return []string{
		"cache_hit_rate",
		"average_read_latency",
		"average_write_latency",
		"batch_operations_per_sec",
		"compression_ratio",
		"total_blocks",
		"cache_size",
		"disk_usage",
		"cache_hits",
		"cache_misses",
	}
}

// RecordCacheHit records a cache hit
func (bc *BlockstoreCollector) RecordCacheHit() {
	bc.mu.Lock()
	defer bc.mu.Unlock()
	
	bc.cacheHits++
}

// RecordCacheMiss records a cache miss
func (bc *BlockstoreCollector) RecordCacheMiss() {
	bc.mu.Lock()
	defer bc.mu.Unlock()
	
	bc.cacheMisses++
}

// RecordReadOperation records a read operation with its latency
func (bc *BlockstoreCollector) RecordReadOperation(latency time.Duration) {
	bc.mu.Lock()
	defer bc.mu.Unlock()
	
	bc.readLatencySum += latency
	bc.readLatencyCount++
	
	// Add to latencies for analysis
	bc.readLatencies = append(bc.readLatencies, latency)
	if len(bc.readLatencies) > bc.maxLatencies {
		bc.readLatencies = bc.readLatencies[1:]
	}
}

// RecordWriteOperation records a write operation with its latency
func (bc *BlockstoreCollector) RecordWriteOperation(latency time.Duration) {
	bc.mu.Lock()
	defer bc.mu.Unlock()
	
	bc.writeLatencySum += latency
	bc.writeLatencyCount++
	
	// Add to latencies for analysis
	bc.writeLatencies = append(bc.writeLatencies, latency)
	if len(bc.writeLatencies) > bc.maxLatencies {
		bc.writeLatencies = bc.writeLatencies[1:]
	}
}

// RecordBatchOperation records a batch operation
func (bc *BlockstoreCollector) RecordBatchOperation() {
	bc.mu.Lock()
	defer bc.mu.Unlock()
	
	bc.batchOperations++
}

// UpdateTotalBlocks updates the total number of blocks
func (bc *BlockstoreCollector) UpdateTotalBlocks(count int64) {
	bc.mu.Lock()
	defer bc.mu.Unlock()
	
	bc.totalBlocks = count
}

// UpdateCacheSize updates the cache size
func (bc *BlockstoreCollector) UpdateCacheSize(size int64) {
	bc.mu.Lock()
	defer bc.mu.Unlock()
	
	bc.cacheSize = size
}

// UpdateDiskUsage updates the disk usage
func (bc *BlockstoreCollector) UpdateDiskUsage(usage int64) {
	bc.mu.Lock()
	defer bc.mu.Unlock()
	
	bc.diskUsage = usage
}

// UpdateCompressionRatio updates the compression ratio
func (bc *BlockstoreCollector) UpdateCompressionRatio(ratio float64) {
	bc.mu.Lock()
	defer bc.mu.Unlock()
	
	bc.compressionRatio = ratio
}

// GetReadLatencyPercentiles returns P95 and P99 read latencies
func (bc *BlockstoreCollector) GetReadLatencyPercentiles() (p95, p99 time.Duration) {
	bc.mu.RLock()
	defer bc.mu.RUnlock()
	
	return bc.calculatePercentiles(bc.readLatencies)
}

// GetWriteLatencyPercentiles returns P95 and P99 write latencies
func (bc *BlockstoreCollector) GetWriteLatencyPercentiles() (p95, p99 time.Duration) {
	bc.mu.RLock()
	defer bc.mu.RUnlock()
	
	return bc.calculatePercentiles(bc.writeLatencies)
}

// calculatePercentiles calculates P95 and P99 from a slice of durations
func (bc *BlockstoreCollector) calculatePercentiles(latencies []time.Duration) (p95, p99 time.Duration) {
	if len(latencies) == 0 {
		return 0, 0
	}
	
	// Create a copy and sort it
	times := make([]time.Duration, len(latencies))
	copy(times, latencies)
	
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
	
	return times[p95Index], times[p99Index]
}