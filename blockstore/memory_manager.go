package blockstore

import (
	"context"
	"runtime"
	"sync"
	"sync/atomic"
	"time"

	logging "github.com/ipfs/go-log/v2"
	metrics "github.com/ipfs/go-metrics-interface"
)

var memLogger = logging.Logger("blockstore.memory")

// MemoryPressureLevel represents different levels of memory pressure
type MemoryPressureLevel int

const (
	MemoryPressureNone MemoryPressureLevel = iota
	MemoryPressureLow
	MemoryPressureMedium
	MemoryPressureHigh
	MemoryPressureCritical
)

// MemoryThresholds defines memory usage thresholds for different pressure levels
type MemoryThresholds struct {
	LowPressure      float64 // 0.6 (60% of available memory)
	MediumPressure   float64 // 0.75 (75% of available memory)
	HighPressure     float64 // 0.85 (85% of available memory)
	CriticalPressure float64 // 0.95 (95% of available memory)
}

// DefaultMemoryThresholds returns default memory pressure thresholds
func DefaultMemoryThresholds() MemoryThresholds {
	return MemoryThresholds{
		LowPressure:      0.6,
		MediumPressure:   0.75,
		HighPressure:     0.85,
		CriticalPressure: 0.95,
	}
}

// MemoryStats represents current memory usage statistics
type MemoryStats struct {
	AllocatedBytes     uint64  // Currently allocated bytes
	TotalAllocBytes    uint64  // Total allocated bytes (cumulative)
	SystemBytes        uint64  // System memory obtained from OS
	HeapBytes          uint64  // Heap memory in use
	StackBytes         uint64  // Stack memory in use
	GCCycles           uint32  // Number of GC cycles
	UsageRatio         float64 // Memory usage ratio (0.0 to 1.0)
	PressureLevel      MemoryPressureLevel
	LastGCTime         time.Time
	NextGCTarget       uint64
}

// MemoryMonitor provides real-time memory usage monitoring and management
type MemoryMonitor struct {
	mu         sync.RWMutex
	ctx        context.Context
	cancel     context.CancelFunc
	
	thresholds MemoryThresholds
	stats      MemoryStats
	
	// Monitoring configuration
	monitorInterval time.Duration
	maxMemoryBytes  uint64 // Maximum allowed memory usage (0 = unlimited)
	
	// Callbacks for memory pressure events
	pressureCallbacks map[MemoryPressureLevel][]func(MemoryStats)
	
	// Metrics
	memoryUsageGauge    metrics.Gauge
	pressureLevelGauge  metrics.Gauge
	gcCyclesCounter     metrics.Counter
	cleanupCounter      metrics.Counter
	
	// State
	running int32
}

// NewMemoryMonitor creates a new memory monitor with default settings
func NewMemoryMonitor(ctx context.Context) *MemoryMonitor {
	ctx, cancel := context.WithCancel(ctx)
	
	mm := &MemoryMonitor{
		ctx:               ctx,
		cancel:            cancel,
		thresholds:        DefaultMemoryThresholds(),
		monitorInterval:   time.Second * 5, // Monitor every 5 seconds
		pressureCallbacks: make(map[MemoryPressureLevel][]func(MemoryStats)),
	}
	
	// Initialize metrics if available
	if metrics.Active() {
		mm.memoryUsageGauge = metrics.NewCtx(ctx, "blockstore_memory_usage_ratio", 
			"Current memory usage ratio").Gauge()
		mm.pressureLevelGauge = metrics.NewCtx(ctx, "blockstore_memory_pressure_level", 
			"Current memory pressure level").Gauge()
		mm.gcCyclesCounter = metrics.NewCtx(ctx, "blockstore_gc_cycles_total", 
			"Total number of GC cycles").Counter()
		mm.cleanupCounter = metrics.NewCtx(ctx, "blockstore_memory_cleanup_total", 
			"Total number of memory cleanup operations").Counter()
	}
	
	return mm
}

// SetThresholds updates memory pressure thresholds
func (mm *MemoryMonitor) SetThresholds(thresholds MemoryThresholds) {
	mm.mu.Lock()
	defer mm.mu.Unlock()
	mm.thresholds = thresholds
}

// SetMaxMemory sets the maximum allowed memory usage in bytes (0 = unlimited)
func (mm *MemoryMonitor) SetMaxMemory(maxBytes uint64) {
	mm.mu.Lock()
	defer mm.mu.Unlock()
	mm.maxMemoryBytes = maxBytes
}

// SetMonitorInterval sets how frequently memory is monitored
func (mm *MemoryMonitor) SetMonitorInterval(interval time.Duration) {
	mm.mu.Lock()
	defer mm.mu.Unlock()
	mm.monitorInterval = interval
}

// RegisterPressureCallback registers a callback for specific memory pressure levels
func (mm *MemoryMonitor) RegisterPressureCallback(level MemoryPressureLevel, callback func(MemoryStats)) {
	mm.mu.Lock()
	defer mm.mu.Unlock()
	mm.pressureCallbacks[level] = append(mm.pressureCallbacks[level], callback)
}

// Start begins memory monitoring
func (mm *MemoryMonitor) Start() {
	if !atomic.CompareAndSwapInt32(&mm.running, 0, 1) {
		return // Already running
	}
	
	go mm.monitorLoop()
	memLogger.Info("Memory monitor started")
}

// Stop stops memory monitoring
func (mm *MemoryMonitor) Stop() {
	if !atomic.CompareAndSwapInt32(&mm.running, 1, 0) {
		return // Not running
	}
	
	mm.cancel()
	memLogger.Info("Memory monitor stopped")
}

// GetStats returns current memory statistics
func (mm *MemoryMonitor) GetStats() MemoryStats {
	mm.mu.RLock()
	defer mm.mu.RUnlock()
	return mm.stats
}

// ForceGC triggers garbage collection and updates statistics
func (mm *MemoryMonitor) ForceGC() {
	runtime.GC()
	mm.updateStats()
	
	if mm.cleanupCounter != nil {
		mm.cleanupCounter.Inc()
	}
	
	memLogger.Debug("Forced garbage collection completed")
}

// monitorLoop runs the main monitoring loop
func (mm *MemoryMonitor) monitorLoop() {
	ticker := time.NewTicker(mm.monitorInterval)
	defer ticker.Stop()
	
	// Initial stats update
	mm.updateStats()
	
	for {
		select {
		case <-mm.ctx.Done():
			return
		case <-ticker.C:
			mm.updateStats()
			mm.checkPressureLevel()
		}
	}
}

// updateStats updates current memory statistics
func (mm *MemoryMonitor) updateStats() {
	var m runtime.MemStats
	runtime.ReadMemStats(&m)
	
	mm.mu.Lock()
	defer mm.mu.Unlock()
	
	// Calculate usage ratio
	var usageRatio float64
	if mm.maxMemoryBytes > 0 {
		usageRatio = float64(m.Alloc) / float64(mm.maxMemoryBytes)
	} else {
		// Use system memory as baseline if no limit set
		usageRatio = float64(m.Alloc) / float64(m.Sys)
	}
	
	// Update stats
	mm.stats = MemoryStats{
		AllocatedBytes:  m.Alloc,
		TotalAllocBytes: m.TotalAlloc,
		SystemBytes:     m.Sys,
		HeapBytes:       m.HeapInuse,
		StackBytes:      m.StackInuse,
		GCCycles:        m.NumGC,
		UsageRatio:      usageRatio,
		LastGCTime:      time.Unix(0, int64(m.LastGC)),
		NextGCTarget:    m.NextGC,
	}
	
	// Determine pressure level
	mm.stats.PressureLevel = mm.calculatePressureLevel(usageRatio)
	
	// Update metrics
	if mm.memoryUsageGauge != nil {
		mm.memoryUsageGauge.Set(usageRatio)
	}
	if mm.pressureLevelGauge != nil {
		mm.pressureLevelGauge.Set(float64(mm.stats.PressureLevel))
	}
}

// calculatePressureLevel determines memory pressure level based on usage ratio
func (mm *MemoryMonitor) calculatePressureLevel(usageRatio float64) MemoryPressureLevel {
	switch {
	case usageRatio >= mm.thresholds.CriticalPressure:
		return MemoryPressureCritical
	case usageRatio >= mm.thresholds.HighPressure:
		return MemoryPressureHigh
	case usageRatio >= mm.thresholds.MediumPressure:
		return MemoryPressureMedium
	case usageRatio >= mm.thresholds.LowPressure:
		return MemoryPressureLow
	default:
		return MemoryPressureNone
	}
}

// checkPressureLevel checks if pressure level changed and triggers callbacks
func (mm *MemoryMonitor) checkPressureLevel() {
	mm.mu.RLock()
	currentLevel := mm.stats.PressureLevel
	stats := mm.stats
	callbacks := mm.pressureCallbacks[currentLevel]
	mm.mu.RUnlock()
	
	// Execute callbacks for current pressure level
	for _, callback := range callbacks {
		go func(cb func(MemoryStats)) {
			defer func() {
				if r := recover(); r != nil {
					memLogger.Errorf("Memory pressure callback panicked: %v", r)
				}
			}()
			cb(stats)
		}(callback)
	}
	
	// Log pressure level changes
	if currentLevel != MemoryPressureNone {
		memLogger.Warnf("Memory pressure level: %v (usage: %.2f%%)", 
			currentLevel, stats.UsageRatio*100)
	}
}

// String returns string representation of memory pressure level
func (level MemoryPressureLevel) String() string {
	switch level {
	case MemoryPressureNone:
		return "None"
	case MemoryPressureLow:
		return "Low"
	case MemoryPressureMedium:
		return "Medium"
	case MemoryPressureHigh:
		return "High"
	case MemoryPressureCritical:
		return "Critical"
	default:
		return "Unknown"
	}
}