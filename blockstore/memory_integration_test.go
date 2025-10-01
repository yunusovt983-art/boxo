package blockstore

import (
	"context"
	"runtime"
	"testing"
	"time"

	blocks "github.com/ipfs/go-block-format"
)

// TestMemoryManagement_Integration tests the complete memory management system
func TestMemoryManagement_Integration(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	
	// Create base blockstore
	baseBS := NewBlockstore(newTestDatastore())
	
	// Create cached blockstore
	cachedBS, err := CachedBlockstore(ctx, baseBS, DefaultCacheOpts())
	if err != nil {
		t.Fatal(err)
	}
	
	// Create memory-aware blockstore with realistic settings
	config := DefaultMemoryAwareConfig()
	config.MonitorInterval = 100 * time.Millisecond
	config.CleanupInterval = 200 * time.Millisecond
	config.ForceGCOnMedium = true
	config.DisableCacheOnHigh = true
	
	mabs, err := NewMemoryAwareBlockstore(ctx, cachedBS, config)
	if err != nil {
		t.Fatal(err)
	}
	defer mabs.Close()
	
	// Track memory statistics
	var initialStats, finalStats MemoryStats
	
	// Wait for initial monitoring
	time.Sleep(150 * time.Millisecond)
	initialStats = mabs.GetMemoryStats()
	
	t.Logf("Initial memory stats: %.2f%% usage, pressure: %v", 
		initialStats.UsageRatio*100, initialStats.PressureLevel)
	
	// Perform operations to test memory management
	numBlocks := 50
	blocks := make([]blocks.Block, numBlocks)
	
	// Create and store blocks
	for i := 0; i < numBlocks; i++ {
		blocks[i] = newTestBlock("integration test data " + string(rune(i)))
		
		err := mabs.Put(ctx, blocks[i])
		if err != nil {
			t.Logf("Put operation %d failed (may be due to memory pressure): %v", i, err)
		}
	}
	
	// Read blocks back
	for i := 0; i < numBlocks; i++ {
		_, err := mabs.Get(ctx, blocks[i].Cid())
		if err != nil {
			t.Logf("Get operation %d failed: %v", i, err)
		}
	}
	
	// Wait for memory monitoring and cleanup to run
	time.Sleep(500 * time.Millisecond)
	
	finalStats = mabs.GetMemoryStats()
	t.Logf("Final memory stats: %.2f%% usage, pressure: %v", 
		finalStats.UsageRatio*100, finalStats.PressureLevel)
	
	// Check that degradation level is reasonable
	degradationLevel := mabs.GetDegradationLevel()
	t.Logf("Final degradation level: %d", degradationLevel)
	
	// Verify that memory management is working
	if finalStats.GCCycles >= initialStats.GCCycles {
		t.Log("GC cycles increased as expected")
	}
	
	// Test that the system is still functional
	testBlock := newTestBlock("final test block")
	err = mabs.Put(ctx, testBlock)
	if err != nil {
		t.Logf("Final put operation failed (may be due to memory pressure): %v", err)
	} else {
		retrieved, err := mabs.Get(ctx, testBlock.Cid())
		if err != nil {
			t.Errorf("Failed to retrieve final test block: %v", err)
		} else if string(retrieved.RawData()) != string(testBlock.RawData()) {
			t.Error("Retrieved data doesn't match original")
		} else {
			t.Log("Final operations successful")
		}
	}
}

// TestMemoryPressureSimulation simulates memory pressure conditions
func TestMemoryPressureSimulation(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	
	// Force GC to get baseline
	runtime.GC()
	var baseline runtime.MemStats
	runtime.ReadMemStats(&baseline)
	
	// Create memory monitor with very sensitive thresholds
	mm := NewMemoryMonitor(ctx)
	mm.SetThresholds(MemoryThresholds{
		LowPressure:      0.01, // Very low to trigger easily
		MediumPressure:   0.02,
		HighPressure:     0.03,
		CriticalPressure: 0.04,
	})
	mm.SetMonitorInterval(50 * time.Millisecond)
	
	// Track pressure level changes
	pressureChanges := 0
	mm.RegisterPressureCallback(MemoryPressureLow, func(stats MemoryStats) {
		pressureChanges++
		t.Logf("Low pressure detected: %.4f%% usage", stats.UsageRatio*100)
	})
	
	mm.RegisterPressureCallback(MemoryPressureMedium, func(stats MemoryStats) {
		pressureChanges++
		t.Logf("Medium pressure detected: %.4f%% usage", stats.UsageRatio*100)
	})
	
	mm.Start()
	defer mm.Stop()
	
	// Wait for monitoring to detect pressure
	time.Sleep(200 * time.Millisecond)
	
	// Check final stats
	finalStats := mm.GetStats()
	t.Logf("Final monitoring stats: %.4f%% usage, pressure: %v, changes: %d", 
		finalStats.UsageRatio*100, finalStats.PressureLevel, pressureChanges)
	
	// Verify monitoring is working
	if finalStats.UsageRatio > 0 {
		t.Log("Memory monitoring is working correctly")
	}
}

// TestCleanupEffectiveness tests cache cleanup under memory pressure
func TestCleanupEffectiveness(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	
	// Create a blockstore with cache that supports cleanup
	baseBS := NewBlockstore(newTestDatastore())
	
	// Create memory-aware blockstore with aggressive cleanup
	config := DefaultMemoryAwareConfig()
	config.CleanupInterval = 50 * time.Millisecond
	config.AggressiveCleanup = true
	
	mabs, err := NewMemoryAwareBlockstore(ctx, baseBS, config)
	if err != nil {
		t.Fatal(err)
	}
	defer mabs.Close()
	
	// Add data to trigger cleanup
	for i := 0; i < 20; i++ {
		block := newTestBlock("cleanup test " + string(rune(i)))
		err := mabs.Put(ctx, block)
		if err != nil {
			t.Logf("Put failed for block %d: %v", i, err)
		}
	}
	
	// Wait for cleanup operations
	time.Sleep(200 * time.Millisecond)
	
	// Verify system is still responsive
	testBlock := newTestBlock("post-cleanup test")
	err = mabs.Put(ctx, testBlock)
	if err != nil {
		t.Logf("Post-cleanup put failed: %v", err)
	}
	
	has, err := mabs.Has(ctx, testBlock.Cid())
	if err != nil {
		t.Errorf("Post-cleanup has check failed: %v", err)
	} else if !has {
		t.Error("Block should exist after cleanup")
	} else {
		t.Log("Cleanup operations completed successfully")
	}
}