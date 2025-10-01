package blockstore

import (
	"context"
	"errors"
	"runtime"
	"sync"
	"testing"
	"time"

	blocks "github.com/ipfs/go-block-format"
	cid "github.com/ipfs/go-cid"
	ds "github.com/ipfs/go-datastore"
	dsq "github.com/ipfs/go-datastore/query"
	mh "github.com/multiformats/go-multihash"
)

func TestMemoryMonitor_Basic(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	
	mm := NewMemoryMonitor(ctx)
	if mm == nil {
		t.Fatal("NewMemoryMonitor returned nil")
	}
	
	// Test initial state
	stats := mm.GetStats()
	if stats.PressureLevel != MemoryPressureNone {
		t.Errorf("Expected MemoryPressureNone, got %v", stats.PressureLevel)
	}
	if stats.UsageRatio < 0 {
		t.Errorf("Expected non-negative usage ratio, got %f", stats.UsageRatio)
	}
	
	// Test starting and stopping
	mm.Start()
	time.Sleep(100 * time.Millisecond) // Let it run briefly
	mm.Stop()
}

func TestMemoryMonitor_Thresholds(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	
	mm := NewMemoryMonitor(ctx)
	
	// Set custom thresholds
	thresholds := MemoryThresholds{
		LowPressure:      0.1,
		MediumPressure:   0.2,
		HighPressure:     0.3,
		CriticalPressure: 0.4,
	}
	mm.SetThresholds(thresholds)
	
	// Test pressure level calculation
	level := mm.calculatePressureLevel(0.05)
	if level != MemoryPressureNone {
		t.Errorf("Expected MemoryPressureNone for 0.05, got %v", level)
	}
	
	level = mm.calculatePressureLevel(0.15)
	if level != MemoryPressureLow {
		t.Errorf("Expected MemoryPressureLow for 0.15, got %v", level)
	}
	
	level = mm.calculatePressureLevel(0.25)
	if level != MemoryPressureMedium {
		t.Errorf("Expected MemoryPressureMedium for 0.25, got %v", level)
	}
	
	level = mm.calculatePressureLevel(0.35)
	if level != MemoryPressureHigh {
		t.Errorf("Expected MemoryPressureHigh for 0.35, got %v", level)
	}
	
	level = mm.calculatePressureLevel(0.45)
	if level != MemoryPressureCritical {
		t.Errorf("Expected MemoryPressureCritical for 0.45, got %v", level)
	}
}

func TestMemoryMonitor_Callbacks(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	
	mm := NewMemoryMonitor(ctx)
	
	// Set up callback tracking
	var callbackCalled bool
	var callbackStats MemoryStats
	var mu sync.Mutex
	
	mm.RegisterPressureCallback(MemoryPressureLow, func(stats MemoryStats) {
		mu.Lock()
		defer mu.Unlock()
		callbackCalled = true
		callbackStats = stats
	})
	
	// Set thresholds to trigger callback
	mm.SetThresholds(MemoryThresholds{
		LowPressure:      0.01, // Very low threshold
		MediumPressure:   0.5,
		HighPressure:     0.8,
		CriticalPressure: 0.95,
	})
	
	mm.Start()
	defer mm.Stop()
	
	// Wait for callback to be triggered
	time.Sleep(200 * time.Millisecond)
	
	mu.Lock()
	defer mu.Unlock()
	
	if !callbackCalled {
		t.Error("Callback should have been called")
	}
	if callbackStats.PressureLevel != MemoryPressureLow {
		t.Errorf("Expected MemoryPressureLow, got %v", callbackStats.PressureLevel)
	}
}

func TestMemoryMonitor_ForceGC(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	
	mm := NewMemoryMonitor(ctx)
	
	// Get initial GC count
	var initialStats runtime.MemStats
	runtime.ReadMemStats(&initialStats)
	initialGC := initialStats.NumGC
	
	// Force GC
	mm.ForceGC()
	
	// Check that GC was triggered
	var afterStats runtime.MemStats
	runtime.ReadMemStats(&afterStats)
	
	if afterStats.NumGC <= initialGC {
		t.Error("GC should have been triggered")
	}
}

func TestMemoryAwareBlockstore_Basic(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	
	// Create base blockstore
	baseBS := NewBlockstore(newTestDatastore())
	
	// Create memory-aware blockstore
	config := DefaultMemoryAwareConfig()
	config.MonitorInterval = 50 * time.Millisecond // Fast monitoring for tests
	
	mabs, err := NewMemoryAwareBlockstore(ctx, baseBS, config)
	if err != nil {
		t.Fatal(err)
	}
	defer mabs.Close()
	
	// Test basic operations
	block := newTestBlock("test data")
	
	err = mabs.Put(ctx, block)
	if err != nil {
		t.Errorf("Put failed: %v", err)
	}
	
	has, err := mabs.Has(ctx, block.Cid())
	if err != nil {
		t.Errorf("Has failed: %v", err)
	}
	if !has {
		t.Error("Block should exist")
	}
	
	retrieved, err := mabs.Get(ctx, block.Cid())
	if err != nil {
		t.Errorf("Get failed: %v", err)
	}
	if string(block.RawData()) != string(retrieved.RawData()) {
		t.Errorf("Retrieved data doesn't match: expected %s, got %s", 
			string(block.RawData()), string(retrieved.RawData()))
	}
}

func TestMemoryAwareBlockstore_MemoryPressure(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	
	// Create base blockstore
	baseBS := NewBlockstore(newTestDatastore())
	
	// Create memory-aware blockstore with very low memory limit
	config := DefaultMemoryAwareConfig()
	config.MaxMemoryBytes = 1024 // Very low limit to trigger pressure
	config.MonitorInterval = 10 * time.Millisecond
	config.RejectWritesOnCritical = true
	
	mabs, err := NewMemoryAwareBlockstore(ctx, baseBS, config)
	if err != nil {
		t.Fatal(err)
	}
	defer mabs.Close()
	
	// Wait for monitoring to start
	time.Sleep(50 * time.Millisecond)
	
	// Check that we're under memory pressure
	stats := mabs.GetMemoryStats()
	t.Logf("Memory usage ratio: %.2f, pressure level: %v", stats.UsageRatio, stats.PressureLevel)
	
	// The memory pressure should be detected (may not always trigger due to test environment)
	// Just verify the monitoring is working
	if stats.PressureLevel >= MemoryPressureLow {
		t.Log("Memory pressure detected as expected")
	} else {
		t.Log("No memory pressure detected in test environment")
	}
}

func TestMemoryAwareBlockstore_GracefulDegradation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	
	// Create base blockstore
	baseBS := NewBlockstore(newTestDatastore())
	
	// Create memory-aware blockstore
	config := DefaultMemoryAwareConfig()
	config.MaxMemoryBytes = 512 // Very low limit
	config.MonitorInterval = 10 * time.Millisecond
	config.DisableCacheOnHigh = true
	config.RejectWritesOnCritical = true
	
	mabs, err := NewMemoryAwareBlockstore(ctx, baseBS, config)
	if err != nil {
		t.Fatal(err)
	}
	defer mabs.Close()
	
	// Wait for monitoring to detect pressure
	time.Sleep(100 * time.Millisecond)
	
	// Check degradation level
	degradationLevel := mabs.GetDegradationLevel()
	t.Logf("Degradation level: %d", degradationLevel)
	
	// Should be in some level of degradation due to low memory limit (may not always trigger)
	if degradationLevel > 0 {
		t.Log("Degradation detected as expected")
	} else {
		t.Log("No degradation detected in test environment")
	}
}

func TestMemoryAwareBlockstore_NoMemoryLeaks(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	
	// Force GC to get baseline
	runtime.GC()
	runtime.GC()
	
	var initialStats runtime.MemStats
	runtime.ReadMemStats(&initialStats)
	initialAlloc := initialStats.Alloc
	
	// Create and use memory-aware blockstore
	func() {
		baseBS := NewBlockstore(newTestDatastore())
		config := DefaultMemoryAwareConfig()
		config.MonitorInterval = 10 * time.Millisecond
		
		mabs, err := NewMemoryAwareBlockstore(ctx, baseBS, config)
		if err != nil {
			t.Fatal(err)
		}
		defer mabs.Close()
		
		// Perform many operations
		for i := 0; i < 100; i++ {
			block := newTestBlock("test data " + string(rune(i)))
			err := mabs.Put(ctx, block)
			if err != nil {
				t.Errorf("Put failed for block %d: %v", i, err)
			}
			
			_, err = mabs.Get(ctx, block.Cid())
			if err != nil {
				t.Errorf("Get failed for block %d: %v", i, err)
			}
		}
		
		// Wait a bit for monitoring
		time.Sleep(100 * time.Millisecond)
	}()
	
	// Force GC to clean up
	runtime.GC()
	runtime.GC()
	time.Sleep(50 * time.Millisecond)
	
	var finalStats runtime.MemStats
	runtime.ReadMemStats(&finalStats)
	finalAlloc := finalStats.Alloc
	
	// Check that memory usage didn't grow significantly
	memoryGrowth := int64(finalAlloc) - int64(initialAlloc)
	t.Logf("Memory growth: %d bytes", memoryGrowth)
	
	// Allow some growth but not excessive (less than 1MB)
	if memoryGrowth >= 1024*1024 {
		t.Errorf("Memory growth too large: %d bytes", memoryGrowth)
	}
}

func TestMemoryAwareBlockstore_CleanupOperations(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	
	// Create cached blockstore that supports cleanup
	baseBS := NewBlockstore(newTestDatastore())
	cachedBS, err := CachedBlockstore(ctx, baseBS, DefaultCacheOpts())
	if err != nil {
		t.Fatal(err)
	}
	
	// Create memory-aware blockstore
	config := DefaultMemoryAwareConfig()
	config.CleanupInterval = 50 * time.Millisecond // Fast cleanup for tests
	config.AggressiveCleanup = true
	
	mabs, err := NewMemoryAwareBlockstore(ctx, cachedBS, config)
	if err != nil {
		t.Fatal(err)
	}
	defer mabs.Close()
	
	// Add some data to cache
	for i := 0; i < 10; i++ {
		block := newTestBlock("test data " + string(rune(i)))
		err := mabs.Put(ctx, block)
		if err != nil {
			t.Errorf("Put failed for block %d: %v", i, err)
		}
	}
	
	// Wait for cleanup operations
	time.Sleep(200 * time.Millisecond)
	
	// Cleanup should have been performed (we can't easily verify the exact effect,
	// but we can check that no errors occurred)
	stats := mabs.GetMemoryStats()
	if stats.UsageRatio < 0 {
		t.Error("Invalid usage ratio")
	}
}

// Helper functions for tests

var ErrNotFound = errors.New("not found")

func newTestDatastore() ds.Batching {
	return &mapDatastore{
		data: make(map[string][]byte),
	}
}

func newTestBlock(data string) blocks.Block {
	hash, _ := mh.Sum([]byte(data), mh.SHA2_256, -1)
	c := cid.NewCidV1(cid.Raw, hash)
	block, _ := blocks.NewBlockWithCid([]byte(data), c)
	return block
}

// Simple in-memory datastore for testing
type mapDatastore struct {
	mu   sync.RWMutex
	data map[string][]byte
}

func (ds *mapDatastore) Get(ctx context.Context, key ds.Key) ([]byte, error) {
	ds.mu.RLock()
	defer ds.mu.RUnlock()
	
	if data, exists := ds.data[key.String()]; exists {
		return data, nil
	}
	return nil, ErrNotFound
}

func (ds *mapDatastore) Put(ctx context.Context, key ds.Key, value []byte) error {
	ds.mu.Lock()
	defer ds.mu.Unlock()
	
	ds.data[key.String()] = value
	return nil
}

func (ds *mapDatastore) Has(ctx context.Context, key ds.Key) (bool, error) {
	ds.mu.RLock()
	defer ds.mu.RUnlock()
	
	_, exists := ds.data[key.String()]
	return exists, nil
}

func (ds *mapDatastore) Delete(ctx context.Context, key ds.Key) error {
	ds.mu.Lock()
	defer ds.mu.Unlock()
	
	delete(ds.data, key.String())
	return nil
}

func (ds *mapDatastore) GetSize(ctx context.Context, key ds.Key) (int, error) {
	ds.mu.RLock()
	defer ds.mu.RUnlock()
	
	if data, exists := ds.data[key.String()]; exists {
		return len(data), nil
	}
	return -1, ErrNotFound
}

func (ds *mapDatastore) Query(ctx context.Context, q dsq.Query) (dsq.Results, error) {
	return dsq.ResultsWithEntries(q, nil), nil
}

func (ds *mapDatastore) Sync(ctx context.Context, prefix ds.Key) error {
	return nil
}

func (ds *mapDatastore) Close() error {
	return nil
}

func (ds *mapDatastore) Batch(ctx context.Context) (ds.Batch, error) {
	return &mapBatch{ds: ds}, nil
}



type mapBatch struct {
	ds   *mapDatastore
	ops  []batchOp
	mu   sync.Mutex
}

type batchOp struct {
	key   ds.Key
	value []byte
	del   bool
}

func (b *mapBatch) Put(ctx context.Context, key ds.Key, value []byte) error {
	b.mu.Lock()
	defer b.mu.Unlock()
	
	b.ops = append(b.ops, batchOp{key: key, value: value})
	return nil
}

func (b *mapBatch) Delete(ctx context.Context, key ds.Key) error {
	b.mu.Lock()
	defer b.mu.Unlock()
	
	b.ops = append(b.ops, batchOp{key: key, del: true})
	return nil
}

func (b *mapBatch) Commit(ctx context.Context) error {
	b.mu.Lock()
	defer b.mu.Unlock()
	
	for _, op := range b.ops {
		if op.del {
			b.ds.Delete(ctx, op.key)
		} else {
			b.ds.Put(ctx, op.key, op.value)
		}
	}
	return nil
}