package monitoring

import (
	"context"
	"runtime"
	"sync"
	"time"
)

// ResourceCollector collects system resource usage metrics
type ResourceCollector struct {
	mu              sync.RWMutex
	cpuUsage        float64
	memoryUsage     int64
	memoryTotal     int64
	diskUsage       int64
	diskTotal       int64
	goroutineCount  int
	gcPauseTime     time.Duration
	
	// GC stats tracking
	lastGCStats     runtime.MemStats
	gcStatsInitialized bool
}

// NewResourceCollector creates a new resource metrics collector
func NewResourceCollector() *ResourceCollector {
	return &ResourceCollector{}
}

// CollectMetrics implements MetricCollector interface
func (rc *ResourceCollector) CollectMetrics(ctx context.Context) (map[string]interface{}, error) {
	rc.mu.Lock()
	defer rc.mu.Unlock()
	
	// Update runtime metrics
	rc.updateRuntimeMetrics()
	
	metrics := map[string]interface{}{
		"cpu_usage":       rc.cpuUsage,
		"memory_usage":    rc.memoryUsage,
		"memory_total":    rc.memoryTotal,
		"disk_usage":      rc.diskUsage,
		"disk_total":      rc.diskTotal,
		"goroutine_count": rc.goroutineCount,
		"gc_pause_time":   rc.gcPauseTime,
	}
	
	return metrics, nil
}

// GetMetricNames returns the list of metric names this collector provides
func (rc *ResourceCollector) GetMetricNames() []string {
	return []string{
		"cpu_usage",
		"memory_usage",
		"memory_total",
		"disk_usage",
		"disk_total",
		"goroutine_count",
		"gc_pause_time",
	}
}

// updateRuntimeMetrics updates Go runtime metrics
func (rc *ResourceCollector) updateRuntimeMetrics() {
	var memStats runtime.MemStats
	runtime.ReadMemStats(&memStats)
	
	// Update memory usage
	rc.memoryUsage = int64(memStats.Alloc)
	rc.memoryTotal = int64(memStats.Sys)
	
	// Update goroutine count
	rc.goroutineCount = runtime.NumGoroutine()
	
	// Calculate GC pause time
	if rc.gcStatsInitialized {
		// Calculate average GC pause time since last collection
		if memStats.NumGC > rc.lastGCStats.NumGC {
			totalPause := memStats.PauseTotalNs - rc.lastGCStats.PauseTotalNs
			numGCs := memStats.NumGC - rc.lastGCStats.NumGC
			if numGCs > 0 {
				rc.gcPauseTime = time.Duration(totalPause / uint64(numGCs))
			}
		}
	} else {
		rc.gcStatsInitialized = true
	}
	
	rc.lastGCStats = memStats
}

// UpdateCPUUsage updates the CPU usage percentage
func (rc *ResourceCollector) UpdateCPUUsage(usage float64) {
	rc.mu.Lock()
	defer rc.mu.Unlock()
	
	rc.cpuUsage = usage
}

// UpdateDiskUsage updates disk usage information
func (rc *ResourceCollector) UpdateDiskUsage(used, total int64) {
	rc.mu.Lock()
	defer rc.mu.Unlock()
	
	rc.diskUsage = used
	rc.diskTotal = total
}

// GetMemoryStats returns detailed memory statistics
func (rc *ResourceCollector) GetMemoryStats() runtime.MemStats {
	var memStats runtime.MemStats
	runtime.ReadMemStats(&memStats)
	return memStats
}

// GetGCStats returns garbage collection statistics
func (rc *ResourceCollector) GetGCStats() map[string]interface{} {
	var memStats runtime.MemStats
	runtime.ReadMemStats(&memStats)
	
	return map[string]interface{}{
		"num_gc":           memStats.NumGC,
		"pause_total_ns":   memStats.PauseTotalNs,
		"pause_ns":         memStats.PauseNs,
		"gc_cpu_fraction":  memStats.GCCPUFraction,
		"next_gc":          memStats.NextGC,
		"last_gc":          time.Unix(0, int64(memStats.LastGC)),
	}
}

// ForceGC forces a garbage collection and returns the pause time
func (rc *ResourceCollector) ForceGC() time.Duration {
	var before runtime.MemStats
	runtime.ReadMemStats(&before)
	
	start := time.Now()
	runtime.GC()
	duration := time.Since(start)
	
	var after runtime.MemStats
	runtime.ReadMemStats(&after)
	
	return duration
}

// GetMemoryPressure returns a memory pressure indicator (0.0 to 1.0)
func (rc *ResourceCollector) GetMemoryPressure() float64 {
	rc.mu.RLock()
	defer rc.mu.RUnlock()
	
	if rc.memoryTotal == 0 {
		return 0.0
	}
	
	return float64(rc.memoryUsage) / float64(rc.memoryTotal)
}

// GetDiskPressure returns a disk pressure indicator (0.0 to 1.0)
func (rc *ResourceCollector) GetDiskPressure() float64 {
	rc.mu.RLock()
	defer rc.mu.RUnlock()
	
	if rc.diskTotal == 0 {
		return 0.0
	}
	
	return float64(rc.diskUsage) / float64(rc.diskTotal)
}

// IsMemoryPressureHigh returns true if memory pressure is above threshold
func (rc *ResourceCollector) IsMemoryPressureHigh(threshold float64) bool {
	return rc.GetMemoryPressure() > threshold
}

// IsDiskPressureHigh returns true if disk pressure is above threshold
func (rc *ResourceCollector) IsDiskPressureHigh(threshold float64) bool {
	return rc.GetDiskPressure() > threshold
}

// GetResourceSummary returns a summary of current resource usage
func (rc *ResourceCollector) GetResourceSummary() map[string]interface{} {
	rc.mu.RLock()
	defer rc.mu.RUnlock()
	
	return map[string]interface{}{
		"cpu_usage_percent":    rc.cpuUsage,
		"memory_usage_mb":      rc.memoryUsage / (1024 * 1024),
		"memory_total_mb":      rc.memoryTotal / (1024 * 1024),
		"memory_pressure":      rc.GetMemoryPressure(),
		"disk_usage_gb":        rc.diskUsage / (1024 * 1024 * 1024),
		"disk_total_gb":        rc.diskTotal / (1024 * 1024 * 1024),
		"disk_pressure":        rc.GetDiskPressure(),
		"goroutines":           rc.goroutineCount,
		"gc_pause_ms":          float64(rc.gcPauseTime.Nanoseconds()) / 1e6,
	}
}