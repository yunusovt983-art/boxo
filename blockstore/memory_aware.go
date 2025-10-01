package blockstore

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"time"

	blocks "github.com/ipfs/go-block-format"
	cid "github.com/ipfs/go-cid"

	metrics "github.com/ipfs/go-metrics-interface"
)

var (
	ErrMemoryPressure = errors.New("operation rejected due to memory pressure")
	ErrCacheDisabled  = errors.New("cache temporarily disabled due to memory pressure")
)

// MemoryAwareConfig configures memory-aware blockstore behavior
type MemoryAwareConfig struct {
	// Memory monitoring
	MaxMemoryBytes      uint64        // Maximum memory usage (0 = unlimited)
	MonitorInterval     time.Duration // How often to check memory usage
	MemoryThresholds    MemoryThresholds
	
	// Graceful degradation settings
	DisableCacheOnHigh     bool // Disable caching when memory pressure is high
	RejectWritesOnCritical bool // Reject write operations when memory is critical
	ForceGCOnMedium        bool // Force GC when memory pressure is medium or higher
	
	// Cache cleanup settings
	CleanupBatchSize    int           // Number of cache entries to clean per batch
	CleanupInterval     time.Duration // How often to run cleanup
	AggressiveCleanup   bool          // More aggressive cleanup under pressure
}

// DefaultMemoryAwareConfig returns default configuration
func DefaultMemoryAwareConfig() MemoryAwareConfig {
	return MemoryAwareConfig{
		MaxMemoryBytes:         0, // Unlimited by default
		MonitorInterval:        time.Second * 5,
		MemoryThresholds:       DefaultMemoryThresholds(),
		DisableCacheOnHigh:     true,
		RejectWritesOnCritical: true,
		ForceGCOnMedium:        true,
		CleanupBatchSize:       1000,
		CleanupInterval:        time.Second * 30,
		AggressiveCleanup:      true,
	}
}

// MemoryAwareBlockstore wraps a blockstore with memory management capabilities
type MemoryAwareBlockstore struct {
	mu sync.RWMutex
	
	// Underlying blockstore
	blockstore Blockstore
	viewer     Viewer
	
	// Memory management
	memoryMonitor *MemoryMonitor
	config        MemoryAwareConfig
	
	// State management
	cacheEnabled    int32 // 1 if cache is enabled, 0 if disabled
	degradationMode int32 // Current degradation mode level
	
	// Cleanup management
	cleanupCtx    context.Context
	cleanupCancel context.CancelFunc
	
	// Metrics
	rejectedOpsCounter    metrics.Counter
	cacheDisabledCounter  metrics.Counter
	cleanupOpsCounter     metrics.Counter
	degradationGauge      metrics.Gauge
}

// NewMemoryAwareBlockstore creates a new memory-aware blockstore
func NewMemoryAwareBlockstore(ctx context.Context, bs Blockstore, config MemoryAwareConfig) (*MemoryAwareBlockstore, error) {
	cleanupCtx, cleanupCancel := context.WithCancel(ctx)
	
	mabs := &MemoryAwareBlockstore{
		blockstore:    bs,
		config:        config,
		cacheEnabled:  1, // Start with cache enabled
		cleanupCtx:    cleanupCtx,
		cleanupCancel: cleanupCancel,
	}
	
	// Check if underlying blockstore supports Viewer interface
	if v, ok := bs.(Viewer); ok {
		mabs.viewer = v
	}
	
	// Initialize memory monitor
	mabs.memoryMonitor = NewMemoryMonitor(ctx)
	if config.MaxMemoryBytes > 0 {
		mabs.memoryMonitor.SetMaxMemory(config.MaxMemoryBytes)
	}
	mabs.memoryMonitor.SetMonitorInterval(config.MonitorInterval)
	mabs.memoryMonitor.SetThresholds(config.MemoryThresholds)
	
	// Register pressure callbacks
	mabs.registerPressureCallbacks()
	
	// Initialize metrics
	if metrics.Active() {
		mabs.rejectedOpsCounter = metrics.NewCtx(ctx, "blockstore_rejected_ops_total",
			"Total number of operations rejected due to memory pressure").Counter()
		mabs.cacheDisabledCounter = metrics.NewCtx(ctx, "blockstore_cache_disabled_total",
			"Total number of times cache was disabled due to memory pressure").Counter()
		mabs.cleanupOpsCounter = metrics.NewCtx(ctx, "blockstore_cleanup_ops_total",
			"Total number of cache cleanup operations").Counter()
		mabs.degradationGauge = metrics.NewCtx(ctx, "blockstore_degradation_level",
			"Current degradation level").Gauge()
	}
	
	// Start memory monitoring and cleanup
	mabs.memoryMonitor.Start()
	go mabs.cleanupLoop()
	
	return mabs, nil
}

// registerPressureCallbacks sets up callbacks for different memory pressure levels
func (mabs *MemoryAwareBlockstore) registerPressureCallbacks() {
	// Low pressure: Start monitoring more closely
	mabs.memoryMonitor.RegisterPressureCallback(MemoryPressureLow, func(stats MemoryStats) {
		atomic.StoreInt32(&mabs.degradationMode, 1)
		if mabs.degradationGauge != nil {
			mabs.degradationGauge.Set(1)
		}
	})
	
	// Medium pressure: Force GC if configured
	mabs.memoryMonitor.RegisterPressureCallback(MemoryPressureMedium, func(stats MemoryStats) {
		atomic.StoreInt32(&mabs.degradationMode, 2)
		if mabs.degradationGauge != nil {
			mabs.degradationGauge.Set(2)
		}
		
		if mabs.config.ForceGCOnMedium {
			mabs.memoryMonitor.ForceGC()
		}
	})
	
	// High pressure: Disable cache if configured
	mabs.memoryMonitor.RegisterPressureCallback(MemoryPressureHigh, func(stats MemoryStats) {
		atomic.StoreInt32(&mabs.degradationMode, 3)
		if mabs.degradationGauge != nil {
			mabs.degradationGauge.Set(3)
		}
		
		if mabs.config.DisableCacheOnHigh {
			if atomic.CompareAndSwapInt32(&mabs.cacheEnabled, 1, 0) {
				memLogger.Warn("Cache disabled due to high memory pressure")
				if mabs.cacheDisabledCounter != nil {
					mabs.cacheDisabledCounter.Inc()
				}
			}
		}
		
		// Force GC
		mabs.memoryMonitor.ForceGC()
	})
	
	// Critical pressure: Reject writes if configured
	mabs.memoryMonitor.RegisterPressureCallback(MemoryPressureCritical, func(stats MemoryStats) {
		atomic.StoreInt32(&mabs.degradationMode, 4)
		if mabs.degradationGauge != nil {
			mabs.degradationGauge.Set(4)
		}
		
		memLogger.Error("Critical memory pressure detected")
		
		// Disable cache
		if atomic.CompareAndSwapInt32(&mabs.cacheEnabled, 1, 0) {
			if mabs.cacheDisabledCounter != nil {
				mabs.cacheDisabledCounter.Inc()
			}
		}
		
		// Force aggressive GC
		mabs.memoryMonitor.ForceGC()
	})
	
	// No pressure: Re-enable cache
	mabs.memoryMonitor.RegisterPressureCallback(MemoryPressureNone, func(stats MemoryStats) {
		atomic.StoreInt32(&mabs.degradationMode, 0)
		if mabs.degradationGauge != nil {
			mabs.degradationGauge.Set(0)
		}
		
		if atomic.CompareAndSwapInt32(&mabs.cacheEnabled, 0, 1) {
			memLogger.Info("Cache re-enabled - memory pressure resolved")
		}
	})
}

// cleanupLoop runs periodic cache cleanup
func (mabs *MemoryAwareBlockstore) cleanupLoop() {
	ticker := time.NewTicker(mabs.config.CleanupInterval)
	defer ticker.Stop()
	
	for {
		select {
		case <-mabs.cleanupCtx.Done():
			return
		case <-ticker.C:
			mabs.performCleanup()
		}
	}
}

// performCleanup performs cache cleanup based on current memory pressure
func (mabs *MemoryAwareBlockstore) performCleanup() {
	stats := mabs.memoryMonitor.GetStats()
	
	// Only perform cleanup if there's memory pressure
	if stats.PressureLevel == MemoryPressureNone {
		return
	}
	
	// Determine cleanup aggressiveness based on pressure level
	var cleanupRatio float64
	switch stats.PressureLevel {
	case MemoryPressureLow:
		cleanupRatio = 0.1 // Clean 10% of cache
	case MemoryPressureMedium:
		cleanupRatio = 0.25 // Clean 25% of cache
	case MemoryPressureHigh:
		cleanupRatio = 0.5 // Clean 50% of cache
	case MemoryPressureCritical:
		cleanupRatio = 0.8 // Clean 80% of cache
	}
	
	// Perform cleanup on underlying cache if it supports it
	if cleaner, ok := mabs.blockstore.(interface{ Cleanup(float64) }); ok {
		cleaner.Cleanup(cleanupRatio)
		if mabs.cleanupOpsCounter != nil {
			mabs.cleanupOpsCounter.Inc()
		}
	}
	
	memLogger.Debugf("Performed cache cleanup (ratio: %.2f) due to %v memory pressure", 
		cleanupRatio, stats.PressureLevel)
}

// checkMemoryPressure returns error if operation should be rejected due to memory pressure
func (mabs *MemoryAwareBlockstore) checkMemoryPressure(isWrite bool) error {
	stats := mabs.memoryMonitor.GetStats()
	
	// Reject writes on critical pressure if configured
	if isWrite && mabs.config.RejectWritesOnCritical && stats.PressureLevel == MemoryPressureCritical {
		if mabs.rejectedOpsCounter != nil {
			mabs.rejectedOpsCounter.Inc()
		}
		return ErrMemoryPressure
	}
	
	return nil
}

// isCacheEnabled returns whether caching is currently enabled
func (mabs *MemoryAwareBlockstore) isCacheEnabled() bool {
	return atomic.LoadInt32(&mabs.cacheEnabled) == 1
}

// Blockstore interface implementation

func (mabs *MemoryAwareBlockstore) Get(ctx context.Context, k cid.Cid) (blocks.Block, error) {
	if err := mabs.checkMemoryPressure(false); err != nil {
		return nil, err
	}
	
	return mabs.blockstore.Get(ctx, k)
}

func (mabs *MemoryAwareBlockstore) Put(ctx context.Context, block blocks.Block) error {
	if err := mabs.checkMemoryPressure(true); err != nil {
		return err
	}
	
	return mabs.blockstore.Put(ctx, block)
}

func (mabs *MemoryAwareBlockstore) PutMany(ctx context.Context, blocks []blocks.Block) error {
	if err := mabs.checkMemoryPressure(true); err != nil {
		return err
	}
	
	return mabs.blockstore.PutMany(ctx, blocks)
}

func (mabs *MemoryAwareBlockstore) Has(ctx context.Context, k cid.Cid) (bool, error) {
	if err := mabs.checkMemoryPressure(false); err != nil {
		return false, err
	}
	
	return mabs.blockstore.Has(ctx, k)
}

func (mabs *MemoryAwareBlockstore) GetSize(ctx context.Context, k cid.Cid) (int, error) {
	if err := mabs.checkMemoryPressure(false); err != nil {
		return -1, err
	}
	
	return mabs.blockstore.GetSize(ctx, k)
}

func (mabs *MemoryAwareBlockstore) DeleteBlock(ctx context.Context, k cid.Cid) error {
	if err := mabs.checkMemoryPressure(true); err != nil {
		return err
	}
	
	return mabs.blockstore.DeleteBlock(ctx, k)
}

func (mabs *MemoryAwareBlockstore) AllKeysChan(ctx context.Context) (<-chan cid.Cid, error) {
	return mabs.blockstore.AllKeysChan(ctx)
}

// Viewer interface implementation
func (mabs *MemoryAwareBlockstore) View(ctx context.Context, k cid.Cid, callback func([]byte) error) error {
	if mabs.viewer == nil {
		blk, err := mabs.Get(ctx, k)
		if err != nil {
			return err
		}
		return callback(blk.RawData())
	}
	
	if err := mabs.checkMemoryPressure(false); err != nil {
		return err
	}
	
	return mabs.viewer.View(ctx, k, callback)
}

// GCBlockstore interface implementation
func (mabs *MemoryAwareBlockstore) GCLock(ctx context.Context) Unlocker {
	if gcbs, ok := mabs.blockstore.(GCBlockstore); ok {
		return gcbs.GCLock(ctx)
	}
	return &unlocker{func() {}} // No-op if underlying doesn't support GC
}

func (mabs *MemoryAwareBlockstore) PinLock(ctx context.Context) Unlocker {
	if gcbs, ok := mabs.blockstore.(GCBlockstore); ok {
		return gcbs.PinLock(ctx)
	}
	return &unlocker{func() {}} // No-op if underlying doesn't support GC
}

func (mabs *MemoryAwareBlockstore) GCRequested(ctx context.Context) bool {
	if gcbs, ok := mabs.blockstore.(GCBlockstore); ok {
		return gcbs.GCRequested(ctx)
	}
	return false
}

// GetMemoryStats returns current memory statistics
func (mabs *MemoryAwareBlockstore) GetMemoryStats() MemoryStats {
	return mabs.memoryMonitor.GetStats()
}

// GetDegradationLevel returns current degradation level (0-4)
func (mabs *MemoryAwareBlockstore) GetDegradationLevel() int32 {
	return atomic.LoadInt32(&mabs.degradationMode)
}

// Close stops memory monitoring and cleanup
func (mabs *MemoryAwareBlockstore) Close() error {
	mabs.memoryMonitor.Stop()
	mabs.cleanupCancel()
	return nil
}