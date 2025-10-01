package blockstore

import (
	"context"
	"fmt"
	"log"
	"time"

	blocks "github.com/ipfs/go-block-format"
	cid "github.com/ipfs/go-cid"
	mh "github.com/multiformats/go-multihash"
)

func ExampleMemoryAwareBlockstore() {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	
	// Create a base blockstore
	baseBS := NewBlockstore(newTestDatastore())
	
	// Configure memory-aware behavior
	config := DefaultMemoryAwareConfig()
	config.MaxMemoryBytes = 10 * 1024 * 1024 // 10MB limit
	config.MonitorInterval = time.Second
	config.DisableCacheOnHigh = true
	config.RejectWritesOnCritical = true
	
	// Create memory-aware blockstore
	mabs, err := NewMemoryAwareBlockstore(ctx, baseBS, config)
	if err != nil {
		log.Fatal(err)
	}
	defer mabs.Close()
	
	// Register callback to monitor memory pressure
	mabs.memoryMonitor.RegisterPressureCallback(MemoryPressureHigh, func(stats MemoryStats) {
		fmt.Printf("High memory pressure detected: %.2f%% usage\n", stats.UsageRatio*100)
	})
	
	// Create and store some blocks
	for i := 0; i < 10; i++ {
		data := fmt.Sprintf("Example block data %d", i)
		hash, _ := mh.Sum([]byte(data), mh.SHA2_256, -1)
		c := cid.NewCidV1(cid.Raw, hash)
		block, _ := blocks.NewBlockWithCid([]byte(data), c)
		
		err := mabs.Put(ctx, block)
		if err != nil {
			fmt.Printf("Failed to store block %d: %v\n", i, err)
			continue
		}
		
		// Retrieve the block
		retrieved, err := mabs.Get(ctx, block.Cid())
		if err != nil {
			fmt.Printf("Failed to retrieve block %d: %v\n", i, err)
			continue
		}
		
		fmt.Printf("Stored and retrieved block %d: %s\n", i, string(retrieved.RawData()))
	}
	
	// Check memory statistics
	stats := mabs.GetMemoryStats()
	fmt.Printf("Final memory usage: %.2f%%, pressure level: %v\n", 
		stats.UsageRatio*100, stats.PressureLevel)
	
	// Output:
	// Stored and retrieved block 0: Example block data 0
	// Stored and retrieved block 1: Example block data 1
	// Stored and retrieved block 2: Example block data 2
	// Stored and retrieved block 3: Example block data 3
	// Stored and retrieved block 4: Example block data 4
	// Stored and retrieved block 5: Example block data 5
	// Stored and retrieved block 6: Example block data 6
	// Stored and retrieved block 7: Example block data 7
	// Stored and retrieved block 8: Example block data 8
	// Stored and retrieved block 9: Example block data 9
	// Final memory usage: 0.00%, pressure level: None
}

func ExampleMemoryMonitor() {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	
	// Create memory monitor
	mm := NewMemoryMonitor(ctx)
	
	// Set custom thresholds
	mm.SetThresholds(MemoryThresholds{
		LowPressure:      0.5,  // 50%
		MediumPressure:   0.7,  // 70%
		HighPressure:     0.85, // 85%
		CriticalPressure: 0.95, // 95%
	})
	
	// Register callbacks for different pressure levels
	mm.RegisterPressureCallback(MemoryPressureLow, func(stats MemoryStats) {
		fmt.Printf("Low memory pressure: %.1f%% usage\n", stats.UsageRatio*100)
	})
	
	mm.RegisterPressureCallback(MemoryPressureMedium, func(stats MemoryStats) {
		fmt.Printf("Medium memory pressure: %.1f%% usage\n", stats.UsageRatio*100)
	})
	
	mm.RegisterPressureCallback(MemoryPressureHigh, func(stats MemoryStats) {
		fmt.Printf("High memory pressure: %.1f%% usage - taking action!\n", stats.UsageRatio*100)
	})
	
	// Start monitoring
	mm.Start()
	defer mm.Stop()
	
	// Simulate some work and check memory periodically
	for i := 0; i < 3; i++ {
		time.Sleep(time.Second)
		
		stats := mm.GetStats()
		fmt.Printf("Memory check %d: %.1f%% usage, %d GC cycles\n", 
			i+1, stats.UsageRatio*100, stats.GCCycles)
		
		// Force GC to demonstrate the feature
		if i == 1 {
			fmt.Println("Forcing garbage collection...")
			mm.ForceGC()
		}
	}
	
	// Final statistics
	finalStats := mm.GetStats()
	fmt.Printf("Final stats: %.1f%% usage, pressure level: %v\n", 
		finalStats.UsageRatio*100, finalStats.PressureLevel)
}

func ExampleMemoryAwareBlockstore_gracefulDegradation() {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	
	// Create blockstore with very aggressive memory management
	baseBS := NewBlockstore(newTestDatastore())
	config := DefaultMemoryAwareConfig()
	config.MaxMemoryBytes = 1024 // Very low limit to trigger degradation
	config.MonitorInterval = 100 * time.Millisecond
	config.DisableCacheOnHigh = true
	config.RejectWritesOnCritical = true
	
	mabs, err := NewMemoryAwareBlockstore(ctx, baseBS, config)
	if err != nil {
		log.Fatal(err)
	}
	defer mabs.Close()
	
	// Wait for memory monitoring to detect pressure
	time.Sleep(200 * time.Millisecond)
	
	// Try to perform operations under memory pressure
	for i := 0; i < 5; i++ {
		data := fmt.Sprintf("Test data under pressure %d", i)
		hash, _ := mh.Sum([]byte(data), mh.SHA2_256, -1)
		c := cid.NewCidV1(cid.Raw, hash)
		block, _ := blocks.NewBlockWithCid([]byte(data), c)
		
		err := mabs.Put(ctx, block)
		if err != nil {
			fmt.Printf("Operation %d rejected due to memory pressure: %v\n", i, err)
		} else {
			fmt.Printf("Operation %d succeeded despite memory pressure\n", i)
		}
		
		// Check degradation level
		degradationLevel := mabs.GetDegradationLevel()
		fmt.Printf("Current degradation level: %d\n", degradationLevel)
	}
	
	// Show final memory statistics
	stats := mabs.GetMemoryStats()
	fmt.Printf("Final memory state: %.1f%% usage, pressure: %v\n", 
		stats.UsageRatio*100, stats.PressureLevel)
}