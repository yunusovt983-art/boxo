package adaptive

import (
	"context"
	"fmt"
	"sync"
	"testing"
	"time"

	blocks "github.com/ipfs/go-block-format"
	"github.com/ipfs/go-cid"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/multiformats/go-multihash"
)

// Benchmark helper functions
func createBenchCID(b *testing.B, data string) cid.Cid {
	hash, err := multihash.Sum([]byte(data), multihash.SHA2_256, -1)
	if err != nil {
		b.Fatal(err)
	}
	return cid.NewCidV1(cid.Raw, hash)
}

func createBenchPeerID(id string) peer.ID {
	return peer.ID(id)
}

// Fast mock handler for benchmarks
func benchmarkRequestHandler(ctx context.Context, requests []*BatchRequest) []BatchResult {
	results := make([]BatchResult, len(requests))
	
	for i, req := range requests {
		// Minimal processing - just create a simple block
		blockData := []byte("bench-data")
		block, _ := blocks.NewBlockWithCid(blockData, req.CID)
		
		results[i] = BatchResult{
			Block: block,
			Error: nil,
		}
	}
	
	return results
}

// Slow mock handler to simulate network latency
func slowRequestHandler(ctx context.Context, requests []*BatchRequest) []BatchResult {
	// Simulate network latency
	time.Sleep(10 * time.Millisecond)
	return benchmarkRequestHandler(ctx, requests)
}

func BenchmarkBatchProcessor_SingleRequest(b *testing.B) {
	ctx := context.Background()
	config := NewAdaptiveBitswapConfig()
	config.BatchSize = 1
	config.BatchTimeout = 1 * time.Millisecond
	
	processor := NewBatchRequestProcessor(ctx, config, benchmarkRequestHandler)
	defer processor.Close()
	
	peer1 := createBenchPeerID("peer1")
	
	b.ResetTimer()
	b.ReportAllocs()
	
	for i := 0; i < b.N; i++ {
		cid := createBenchCID(b, fmt.Sprintf("bench-%d", i))
		resultChan := processor.SubmitRequest(ctx, cid, peer1, NormalPriority)
		
		select {
		case <-resultChan:
			// Success
		case <-time.After(1 * time.Second):
			b.Fatal("Request timeout")
		}
	}
}

func BenchmarkBatchProcessor_BatchedRequests(b *testing.B) {
	ctx := context.Background()
	config := NewAdaptiveBitswapConfig()
	config.BatchSize = 10
	config.BatchTimeout = 5 * time.Millisecond
	
	processor := NewBatchRequestProcessor(ctx, config, benchmarkRequestHandler)
	defer processor.Close()
	
	peer1 := createBenchPeerID("peer1")
	
	b.ResetTimer()
	b.ReportAllocs()
	
	for i := 0; i < b.N; i++ {
		cid := createBenchCID(b, fmt.Sprintf("batch-%d", i))
		resultChan := processor.SubmitRequest(ctx, cid, peer1, NormalPriority)
		
		select {
		case <-resultChan:
			// Success
		case <-time.After(1 * time.Second):
			b.Fatal("Request timeout")
		}
	}
}

func BenchmarkBatchProcessor_ConcurrentRequests(b *testing.B) {
	ctx := context.Background()
	config := NewAdaptiveBitswapConfig()
	config.BatchSize = 50
	config.BatchTimeout = 10 * time.Millisecond
	
	processor := NewBatchRequestProcessor(ctx, config, benchmarkRequestHandler)
	defer processor.Close()
	
	peer1 := createBenchPeerID("peer1")
	
	b.ResetTimer()
	b.ReportAllocs()
	
	b.RunParallel(func(pb *testing.PB) {
		i := 0
		for pb.Next() {
			cid := createBenchCID(b, fmt.Sprintf("concurrent-%d", i))
			resultChan := processor.SubmitRequest(ctx, cid, peer1, NormalPriority)
			
			select {
			case <-resultChan:
				// Success
			case <-time.After(1 * time.Second):
				b.Fatal("Request timeout")
			}
			i++
		}
	})
}

func BenchmarkBatchProcessor_DifferentPriorities(b *testing.B) {
	ctx := context.Background()
	config := NewAdaptiveBitswapConfig()
	config.BatchSize = 20
	config.BatchTimeout = 5 * time.Millisecond
	
	processor := NewBatchRequestProcessor(ctx, config, benchmarkRequestHandler)
	defer processor.Close()
	
	peer1 := createBenchPeerID("peer1")
	priorities := []RequestPriority{LowPriority, NormalPriority, HighPriority, CriticalPriority}
	
	b.ResetTimer()
	b.ReportAllocs()
	
	for i := 0; i < b.N; i++ {
		priority := priorities[i%len(priorities)]
		cid := createBenchCID(b, fmt.Sprintf("priority-%d", i))
		resultChan := processor.SubmitRequest(ctx, cid, peer1, priority)
		
		select {
		case <-resultChan:
			// Success
		case <-time.After(1 * time.Second):
			b.Fatal("Request timeout")
		}
	}
}

func BenchmarkBatchProcessor_HighThroughput(b *testing.B) {
	ctx := context.Background()
	config := NewAdaptiveBitswapConfig()
	config.BatchSize = 100
	config.BatchTimeout = 1 * time.Millisecond
	config.CurrentWorkerCount = 32
	
	processor := NewBatchRequestProcessor(ctx, config, benchmarkRequestHandler)
	defer processor.Close()
	
	peer1 := createBenchPeerID("peer1")
	
	b.ResetTimer()
	b.ReportAllocs()
	
	var wg sync.WaitGroup
	
	for i := 0; i < b.N; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			
			cid := createBenchCID(b, fmt.Sprintf("throughput-%d", idx))
			resultChan := processor.SubmitRequest(ctx, cid, peer1, NormalPriority)
			
			select {
			case <-resultChan:
				// Success
			case <-time.After(5 * time.Second):
				b.Error("Request timeout")
			}
		}(i)
	}
	
	wg.Wait()
}

func BenchmarkBatchProcessor_NetworkLatencySimulation(b *testing.B) {
	ctx := context.Background()
	config := NewAdaptiveBitswapConfig()
	config.BatchSize = 25
	config.BatchTimeout = 20 * time.Millisecond
	
	processor := NewBatchRequestProcessor(ctx, config, slowRequestHandler)
	defer processor.Close()
	
	peer1 := createBenchPeerID("peer1")
	
	b.ResetTimer()
	b.ReportAllocs()
	
	for i := 0; i < b.N; i++ {
		cid := createBenchCID(b, fmt.Sprintf("latency-%d", i))
		resultChan := processor.SubmitRequest(ctx, cid, peer1, NormalPriority)
		
		select {
		case <-resultChan:
			// Success
		case <-time.After(5 * time.Second):
			b.Fatal("Request timeout")
		}
	}
}

func BenchmarkBatchProcessor_VariableBatchSizes(b *testing.B) {
	batchSizes := []int{1, 5, 10, 25, 50, 100, 200}
	
	for _, batchSize := range batchSizes {
		b.Run(fmt.Sprintf("BatchSize%d", batchSize), func(b *testing.B) {
			ctx := context.Background()
			config := NewAdaptiveBitswapConfig()
			config.BatchSize = batchSize
			config.BatchTimeout = 10 * time.Millisecond
			
			processor := NewBatchRequestProcessor(ctx, config, benchmarkRequestHandler)
			defer processor.Close()
			
			peer1 := createBenchPeerID("peer1")
			
			b.ResetTimer()
			b.ReportAllocs()
			
			for i := 0; i < b.N; i++ {
				cid := createBenchCID(b, fmt.Sprintf("var-batch-%d-%d", batchSize, i))
				resultChan := processor.SubmitRequest(ctx, cid, peer1, NormalPriority)
				
				select {
				case <-resultChan:
					// Success
				case <-time.After(2 * time.Second):
					b.Fatal("Request timeout")
				}
			}
		})
	}
}

func BenchmarkBatchProcessor_WorkerPoolSizes(b *testing.B) {
	workerCounts := []int{1, 2, 4, 8, 16, 32}
	
	for _, workerCount := range workerCounts {
		b.Run(fmt.Sprintf("Workers%d", workerCount), func(b *testing.B) {
			ctx := context.Background()
			config := NewAdaptiveBitswapConfig()
			config.BatchSize = 20
			config.BatchTimeout = 5 * time.Millisecond
			config.CurrentWorkerCount = workerCount
			
			processor := NewBatchRequestProcessor(ctx, config, benchmarkRequestHandler)
			defer processor.Close()
			
			peer1 := createBenchPeerID("peer1")
			
			b.ResetTimer()
			b.ReportAllocs()
			
			b.RunParallel(func(pb *testing.PB) {
				i := 0
				for pb.Next() {
					cid := createBenchCID(b, fmt.Sprintf("workers-%d-%d", workerCount, i))
					resultChan := processor.SubmitRequest(ctx, cid, peer1, NormalPriority)
					
					select {
					case <-resultChan:
						// Success
					case <-time.After(2 * time.Second):
						b.Fatal("Request timeout")
					}
					i++
				}
			})
		})
	}
}

func BenchmarkWorkerPool_TaskSubmission(b *testing.B) {
	ctx := context.Background()
	pool := NewWorkerPool(ctx, 8)
	defer pool.Close()
	
	b.ResetTimer()
	b.ReportAllocs()
	
	var wg sync.WaitGroup
	
	for i := 0; i < b.N; i++ {
		wg.Add(1)
		pool.Submit(func() {
			wg.Done()
			// Minimal work
		})
	}
	
	wg.Wait()
}

func BenchmarkWorkerPool_ConcurrentSubmission(b *testing.B) {
	ctx := context.Background()
	pool := NewWorkerPool(ctx, 16)
	defer pool.Close()
	
	b.ResetTimer()
	b.ReportAllocs()
	
	var wg sync.WaitGroup
	
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			wg.Add(1)
			pool.Submit(func() {
				wg.Done()
				// Minimal work
			})
		}
	})
	
	wg.Wait()
}

func BenchmarkWorkerPool_WorkerScaling(b *testing.B) {
	workerCounts := []int{1, 2, 4, 8, 16, 32, 64}
	
	for _, workerCount := range workerCounts {
		b.Run(fmt.Sprintf("Workers%d", workerCount), func(b *testing.B) {
			ctx := context.Background()
			pool := NewWorkerPool(ctx, workerCount)
			defer pool.Close()
			
			b.ResetTimer()
			b.ReportAllocs()
			
			var wg sync.WaitGroup
			
			for i := 0; i < b.N; i++ {
				wg.Add(1)
				pool.Submit(func() {
					wg.Done()
					// Simulate small amount of work
					time.Sleep(time.Microsecond)
				})
			}
			
			wg.Wait()
		})
	}
}

// Benchmark to measure memory allocation patterns
func BenchmarkBatchProcessor_MemoryAllocation(b *testing.B) {
	ctx := context.Background()
	config := NewAdaptiveBitswapConfig()
	config.BatchSize = 50
	config.BatchTimeout = 5 * time.Millisecond
	
	processor := NewBatchRequestProcessor(ctx, config, benchmarkRequestHandler)
	defer processor.Close()
	
	peer1 := createBenchPeerID("peer1")
	
	b.ResetTimer()
	b.ReportAllocs()
	
	for i := 0; i < b.N; i++ {
		cid := createBenchCID(b, fmt.Sprintf("memory-%d", i))
		resultChan := processor.SubmitRequest(ctx, cid, peer1, NormalPriority)
		
		select {
		case <-resultChan:
			// Success
		case <-time.After(1 * time.Second):
			b.Fatal("Request timeout")
		}
	}
}

// Benchmark to test performance under different timeout scenarios
func BenchmarkBatchProcessor_TimeoutScenarios(b *testing.B) {
	timeouts := []time.Duration{
		1 * time.Millisecond,
		5 * time.Millisecond,
		10 * time.Millisecond,
		50 * time.Millisecond,
		100 * time.Millisecond,
	}
	
	for _, timeout := range timeouts {
		b.Run(fmt.Sprintf("Timeout%dms", timeout.Milliseconds()), func(b *testing.B) {
			ctx := context.Background()
			config := NewAdaptiveBitswapConfig()
			config.BatchSize = 100 // Large batch size to test timeout triggering
			config.BatchTimeout = timeout
			
			processor := NewBatchRequestProcessor(ctx, config, benchmarkRequestHandler)
			defer processor.Close()
			
			peer1 := createBenchPeerID("peer1")
			
			b.ResetTimer()
			b.ReportAllocs()
			
			for i := 0; i < b.N; i++ {
				cid := createBenchCID(b, fmt.Sprintf("timeout-%d", i))
				resultChan := processor.SubmitRequest(ctx, cid, peer1, NormalPriority)
				
				select {
				case <-resultChan:
					// Success
				case <-time.After(2 * time.Second):
					b.Fatal("Request timeout")
				}
			}
		})
	}
}