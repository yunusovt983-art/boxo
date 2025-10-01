package blockstore

import (
	"context"
	"runtime"
	"testing"
	"time"

	blocks "github.com/ipfs/go-block-format"
)

func BenchmarkMemoryAwareBlockstore_NormalOperations(b *testing.B) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	
	// Create memory-aware blockstore
	baseBS := NewBlockstore(newTestDatastore())
	config := DefaultMemoryAwareConfig()
	config.MonitorInterval = time.Second // Slower monitoring for benchmarks
	
	mabs, err := NewMemoryAwareBlockstore(ctx, baseBS, config)
	if err != nil {
		b.Fatal(err)
	}
	defer mabs.Close()
	
	// Pre-create test blocks
	blocks := make([]blocks.Block, 1000)
	for i := 0; i < 1000; i++ {
		blocks[i] = newTestBlock("benchmark data " + string(rune(i)))
	}
	
	b.ResetTimer()
	
	b.Run("Put", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			block := blocks[i%len(blocks)]
			if err := mabs.Put(ctx, block); err != nil {
				b.Fatal(err)
			}
		}
	})
	
	b.Run("Get", func(b *testing.B) {
		// First put some blocks
		for i := 0; i < 100; i++ {
			mabs.Put(ctx, blocks[i])
		}
		
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			block := blocks[i%100]
			if _, err := mabs.Get(ctx, block.Cid()); err != nil {
				b.Fatal(err)
			}
		}
	})
	
	b.Run("Has", func(b *testing.B) {
		// First put some blocks
		for i := 0; i < 100; i++ {
			mabs.Put(ctx, blocks[i])
		}
		
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			block := blocks[i%100]
			if _, err := mabs.Has(ctx, block.Cid()); err != nil {
				b.Fatal(err)
			}
		}
	})
}

func BenchmarkMemoryAwareBlockstore_UnderPressure(b *testing.B) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	
	// Create memory-aware blockstore with low memory limit to trigger pressure
	baseBS := NewBlockstore(newTestDatastore())
	config := DefaultMemoryAwareConfig()
	config.MaxMemoryBytes = 1024 * 1024 // 1MB limit
	config.MonitorInterval = 100 * time.Millisecond
	
	mabs, err := NewMemoryAwareBlockstore(ctx, baseBS, config)
	if err != nil {
		b.Fatal(err)
	}
	defer mabs.Close()
	
	// Wait for memory pressure to be detected
	time.Sleep(200 * time.Millisecond)
	
	// Pre-create test blocks
	blocks := make([]blocks.Block, 100)
	for i := 0; i < 100; i++ {
		blocks[i] = newTestBlock("pressure test data " + string(rune(i)))
	}
	
	b.ResetTimer()
	
	b.Run("PutUnderPressure", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			block := blocks[i%len(blocks)]
			// Some operations might be rejected under pressure
			mabs.Put(ctx, block)
		}
	})
}

func BenchmarkMemoryMonitor_Overhead(b *testing.B) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	
	mm := NewMemoryMonitor(ctx)
	mm.SetMonitorInterval(10 * time.Millisecond) // Fast monitoring
	mm.Start()
	defer mm.Stop()
	
	b.ResetTimer()
	
	b.Run("GetStats", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			_ = mm.GetStats()
		}
	})
	
	b.Run("ForceGC", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			mm.ForceGC()
		}
	})
}

func BenchmarkMemoryUsage_Comparison(b *testing.B) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	
	// Benchmark regular blockstore vs memory-aware blockstore
	baseDS := newTestDatastore()
	
	b.Run("RegularBlockstore", func(b *testing.B) {
		bs := NewBlockstore(baseDS)
		
		// Force GC before measurement
		runtime.GC()
		var m1 runtime.MemStats
		runtime.ReadMemStats(&m1)
		
		b.ResetTimer()
		
		for i := 0; i < b.N; i++ {
			block := newTestBlock("regular test " + string(rune(i)))
			bs.Put(ctx, block)
			bs.Get(ctx, block.Cid())
		}
		
		b.StopTimer()
		
		// Measure memory after operations
		runtime.GC()
		var m2 runtime.MemStats
		runtime.ReadMemStats(&m2)
		
		b.ReportMetric(float64(m2.Alloc-m1.Alloc), "bytes/op")
	})
	
	b.Run("MemoryAwareBlockstore", func(b *testing.B) {
		bs := NewBlockstore(baseDS)
		config := DefaultMemoryAwareConfig()
		config.MonitorInterval = time.Second // Slow monitoring for fair comparison
		
		mabs, err := NewMemoryAwareBlockstore(ctx, bs, config)
		if err != nil {
			b.Fatal(err)
		}
		defer mabs.Close()
		
		// Force GC before measurement
		runtime.GC()
		var m1 runtime.MemStats
		runtime.ReadMemStats(&m1)
		
		b.ResetTimer()
		
		for i := 0; i < b.N; i++ {
			block := newTestBlock("memory-aware test " + string(rune(i)))
			mabs.Put(ctx, block)
			mabs.Get(ctx, block.Cid())
		}
		
		b.StopTimer()
		
		// Measure memory after operations
		runtime.GC()
		var m2 runtime.MemStats
		runtime.ReadMemStats(&m2)
		
		b.ReportMetric(float64(m2.Alloc-m1.Alloc), "bytes/op")
	})
}