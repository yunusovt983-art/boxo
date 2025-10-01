package loadtest

import (
	"context"
	"fmt"
	"os"
	"runtime"
	"sort"
	"strconv"
	"sync"
	"time"
)

// Helper functions for statistics calculation
func calculateAverageLatency(latencies []time.Duration) time.Duration {
	if len(latencies) == 0 {
		return 0
	}

	var total time.Duration
	for _, lat := range latencies {
		total += lat
	}
	return total / time.Duration(len(latencies))
}

func calculatePercentileLatency(latencies []time.Duration, percentile float64) time.Duration {
	if len(latencies) == 0 {
		return 0
	}

	// Create a copy and sort it
	sorted := make([]time.Duration, len(latencies))
	copy(sorted, latencies)
	sort.Slice(sorted, func(i, j int) bool {
		return sorted[i] < sorted[j]
	})

	// Calculate percentile index
	index := int(float64(len(sorted)-1) * percentile)
	if index < 0 {
		index = 0
	}
	if index >= len(sorted) {
		index = len(sorted) - 1
	}

	return sorted[index]
}

func calculateMaxLatency(latencies []time.Duration) time.Duration {
	if len(latencies) == 0 {
		return 0
	}

	max := latencies[0]
	for _, lat := range latencies[1:] {
		if lat > max {
			max = lat
		}
	}
	return max
}

func isNetworkError(err error) bool {
	// Simple network error detection (could be more sophisticated)
	return err != nil && (err.Error() == "context deadline exceeded" ||
		err.Error() == "connection refused" ||
		err.Error() == "network unreachable")
}

// MemoryTracker tracks memory usage over time
type MemoryTracker struct {
	interval time.Duration
	samples  []float64
	mutex    sync.Mutex
	running  bool
}

func (mt *MemoryTracker) Start(ctx context.Context) {
	mt.mutex.Lock()
	mt.running = true
	mt.mutex.Unlock()

	ticker := time.NewTicker(mt.interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			var memStats runtime.MemStats
			runtime.ReadMemStats(&memStats)
			memoryMB := float64(memStats.Alloc) / 1024 / 1024

			mt.mutex.Lock()
			mt.samples = append(mt.samples, memoryMB)
			mt.mutex.Unlock()
		}
	}
}

func (mt *MemoryTracker) Stop() {
	mt.mutex.Lock()
	mt.running = false
	mt.mutex.Unlock()
}

func (mt *MemoryTracker) GetGrowthRate() float64 {
	mt.mutex.Lock()
	defer mt.mutex.Unlock()

	if len(mt.samples) < 2 {
		return 0
	}

	initial := mt.samples[0]
	final := mt.samples[len(mt.samples)-1]

	if initial == 0 {
		return 0
	}

	return (final - initial) / initial
}

func (mt *MemoryTracker) GetPeakMemory() float64 {
	mt.mutex.Lock()
	defer mt.mutex.Unlock()

	if len(mt.samples) == 0 {
		return 0
	}

	peak := mt.samples[0]
	for _, sample := range mt.samples[1:] {
		if sample > peak {
			peak = sample
		}
	}

	return peak
}

// ResourceMonitor tracks system resource usage during tests
type ResourceMonitor struct {
	interval         time.Duration
	memorySamples    []float64
	goroutineSamples []int
	mutex            sync.Mutex
	running          bool
}

func (rm *ResourceMonitor) Start(ctx context.Context) {
	rm.mutex.Lock()
	rm.running = true
	rm.mutex.Unlock()

	ticker := time.NewTicker(rm.interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			var memStats runtime.MemStats
			runtime.ReadMemStats(&memStats)
			memoryMB := float64(memStats.Alloc) / 1024 / 1024
			goroutines := runtime.NumGoroutine()

			rm.mutex.Lock()
			rm.memorySamples = append(rm.memorySamples, memoryMB)
			rm.goroutineSamples = append(rm.goroutineSamples, goroutines)
			rm.mutex.Unlock()
		}
	}
}

func (rm *ResourceMonitor) Stop() {
	rm.mutex.Lock()
	rm.running = false
	rm.mutex.Unlock()
}

func (rm *ResourceMonitor) GetFinalStats() struct {
	MemoryMB       float64
	GoroutineCount int
} {
	rm.mutex.Lock()
	defer rm.mutex.Unlock()

	var finalMemory float64
	var finalGoroutines int

	if len(rm.memorySamples) > 0 {
		finalMemory = rm.memorySamples[len(rm.memorySamples)-1]
	}

	if len(rm.goroutineSamples) > 0 {
		finalGoroutines = rm.goroutineSamples[len(rm.goroutineSamples)-1]
	}

	return struct {
		MemoryMB       float64
		GoroutineCount int
	}{
		MemoryMB:       finalMemory,
		GoroutineCount: finalGoroutines,
	}
}

// getSystemInfo captures current system information
func getSystemInfo() SystemInfo {
	var memStats runtime.MemStats
	runtime.ReadMemStats(&memStats)

	return SystemInfo{
		GoVersion:  runtime.Version(),
		GOMAXPROCS: runtime.GOMAXPROCS(0),
		NumCPU:     runtime.NumCPU(),
		MemoryMB:   int64(memStats.Sys / 1024 / 1024),
		OS:         runtime.GOOS,
		Arch:       runtime.GOARCH,
	}
}

// Helper function to get duration from environment variable
func getEnvDuration(key string) time.Duration {
	value := os.Getenv(key)
	if value == "" {
		return 0
	}

	duration, err := time.ParseDuration(value)
	if err != nil {
		return 0
	}

	return duration
}

// Helper function to get int from environment variable
func getEnvInt(key string, defaultValue int) int {
	value := os.Getenv(key)
	if value == "" {
		return defaultValue
	}

	intValue, err := strconv.Atoi(value)
	if err != nil {
		return defaultValue
	}

	return intValue
}

// ValidateEnvironment checks if the system has sufficient resources for load testing
func ValidateEnvironment() error {
	// Check GOMAXPROCS (this is a more reliable check than memory)
	if runtime.GOMAXPROCS(0) < 1 {
		return fmt.Errorf("insufficient CPU cores: %d available, need at least 1", runtime.GOMAXPROCS(0))
	}

	// For memory, we'll just warn if we can't get a good reading
	var memStats runtime.MemStats
	runtime.ReadMemStats(&memStats)

	// Basic memory check - just ensure we have some reasonable allocation
	if memStats.Sys < 1024*1024 { // 1MB minimum - very basic check
		return fmt.Errorf("very low memory allocation detected: %d bytes, this may indicate severe resource constraints", memStats.Sys)
	}

	return nil
}
