package network

import (
	"context"
	"fmt"
	"math/rand"
	"sync"
	"testing"
	"time"

	"github.com/libp2p/go-libp2p/core/peer"
	libp2ptest "github.com/libp2p/go-libp2p/core/test"
)

// NetworkPerformanceBenchmark provides comprehensive network performance benchmarks
type NetworkPerformanceBenchmark struct {
	bkm           *BufferKeepAliveManager
	peers         []peer.ID
	config        *BufferKeepAliveConfig
	results       *BenchmarkResults
}

// BenchmarkResults stores benchmark results for analysis
type BenchmarkResults struct {
	TotalOperations     int64
	TotalDuration       time.Duration
	OperationsPerSecond float64
	AverageLatency      time.Duration
	P95Latency          time.Duration
	P99Latency          time.Duration
	
	BufferAdaptations   int64
	KeepAliveProbes     int64
	SlowConnDetections  int64
	
	MemoryUsage         int64
	CPUUsage            float64
	
	Errors              int64
	ErrorRate           float64
}

// LatencyMeasurement stores individual latency measurements
type LatencyMeasurement struct {
	Operation string
	Latency   time.Duration
	Success   bool
}

// BenchmarkBufferAdaptationThroughput measures buffer adaptation throughput
func BenchmarkBufferAdaptationThroughput(b *testing.B) {
	h := &mockHost{}
	
	config := DefaultBufferKeepAliveConfig()
	bkm := NewBufferKeepAliveManager(h, config)
	
	// Create test peers
	numPeers := 1000
	peers := make([]peer.ID, numPeers)
	for i := range peers {
		peers[i] = libp2ptest.RandPeerIDFatal(b)
	}
	
	b.ResetTimer()
	b.ReportAllocs()
	
	b.RunParallel(func(pb *testing.PB) {
		i := 0
		for pb.Next() {
			peerID := peers[i%numPeers]
			bandwidth := int64(rand.Intn(20*1024*1024) + 1024*1024) // 1MB-20MB/s
			latency := time.Duration(rand.Intn(200)+10) * time.Millisecond // 10-210ms
			
			bkm.AdaptBufferSize(peerID, bandwidth, latency)
			i++
		}
	})
	
	b.StopTimer()
	
	// Report custom metrics
	opsPerSec := float64(b.N) / b.Elapsed().Seconds()
	b.ReportMetric(opsPerSec, "ops/sec")
	b.ReportMetric(float64(b.N)/float64(numPeers), "adaptations/peer")
}

// BenchmarkKeepAliveScalability measures keep-alive scalability
func BenchmarkKeepAliveScalability(b *testing.B) {
	peerCounts := []int{100, 500, 1000, 5000, 10000}
	
	for _, peerCount := range peerCounts {
		b.Run(fmt.Sprintf("peers-%d", peerCount), func(b *testing.B) {
			h := &mockHost{}
			
			config := DefaultBufferKeepAliveConfig()
			config.KeepAliveCheckInterval = 100 * time.Millisecond
			
			bkm := NewBufferKeepAliveManager(h, config)
			
			// Create and enable keep-alive for peers
			peers := make([]peer.ID, peerCount)
			for i := range peers {
				peers[i] = libp2ptest.RandPeerIDFatal(b)
				bkm.EnableKeepAlive(peers[i], i%10 == 0) // 10% important peers
			}
			
			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer cancel()
			
			err := bkm.Start(ctx)
			if err != nil {
				b.Fatal(err)
			}
			defer bkm.Stop()
			
			b.ResetTimer()
			
			// Measure keep-alive check performance
			start := time.Now()
			for i := 0; i < b.N; i++ {
				bkm.keepAliveManager.PerformKeepAliveChecks(ctx)
			}
			duration := time.Since(start)
			
			b.StopTimer()
			
			checksPerSec := float64(b.N) / duration.Seconds()
			peersPerCheck := float64(peerCount)
			
			b.ReportMetric(checksPerSec, "checks/sec")
			b.ReportMetric(checksPerSec*peersPerCheck, "peer-checks/sec")
		})
	}
}

// BenchmarkBandwidthMonitoringOverhead measures bandwidth monitoring overhead
func BenchmarkBandwidthMonitoringOverhead(b *testing.B) {
	config := DefaultBufferKeepAliveConfig()
	bm := NewBandwidthMonitor(config)
	
	numPeers := 1000
	peers := make([]peer.ID, numPeers)
	for i := range peers {
		peers[i] = libp2ptest.RandPeerIDFatal(b)
	}
	
	b.ResetTimer()
	b.ReportAllocs()
	
	b.RunParallel(func(pb *testing.PB) {
		i := 0
		for pb.Next() {
			peerID := peers[i%numPeers]
			bytesSent := int64(rand.Intn(1024*1024) + 1024) // 1KB-1MB
			bytesReceived := int64(rand.Intn(1024*1024) + 1024)
			
			bm.RecordTransfer(peerID, bytesSent, bytesReceived)
			i++
		}
	})
	
	b.StopTimer()
	
	transfersPerSec := float64(b.N) / b.Elapsed().Seconds()
	b.ReportMetric(transfersPerSec, "transfers/sec")
}

// BenchmarkSlowConnectionDetectionLatency measures slow connection detection latency
func BenchmarkSlowConnectionDetectionLatency(b *testing.B) {
	config := DefaultBufferKeepAliveConfig()
	config.SlowConnectionSamples = 5
	
	scd := NewSlowConnectionDetector(config)
	
	numPeers := 100
	peers := make([]peer.ID, numPeers)
	for i := range peers {
		peers[i] = libp2ptest.RandPeerIDFatal(b)
	}
	
	// Pre-populate with some latency data
	for _, peerID := range peers {
		for j := 0; j < config.SlowConnectionSamples; j++ {
			latency := time.Duration(rand.Intn(1000)+10) * time.Millisecond
			scd.RecordLatency(peerID, latency)
		}
	}
	
	b.ResetTimer()
	
	for i := 0; i < b.N; i++ {
		scd.DetectSlowConnections()
	}
	
	b.StopTimer()
	
	detectionsPerSec := float64(b.N) / b.Elapsed().Seconds()
	peersPerDetection := float64(numPeers)
	
	b.ReportMetric(detectionsPerSec, "detections/sec")
	b.ReportMetric(detectionsPerSec*peersPerDetection, "peer-checks/sec")
}

// BenchmarkConcurrentNetworkOperations measures performance under concurrent load
func BenchmarkConcurrentNetworkOperations(b *testing.B) {
	h := &mockHost{}
	
	config := DefaultBufferKeepAliveConfig()
	bkm := NewBufferKeepAliveManager(h, config)
	
	numPeers := 1000
	peers := make([]peer.ID, numPeers)
	for i := range peers {
		peers[i] = libp2ptest.RandPeerIDFatal(b)
	}
	
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	
	err := bkm.Start(ctx)
	if err != nil {
		b.Fatal(err)
	}
	defer bkm.Stop()
	
	b.ResetTimer()
	b.ReportAllocs()
	
	// Run concurrent operations
	var wg sync.WaitGroup
	numWorkers := 10
	
	for w := 0; w < numWorkers; w++ {
		wg.Add(1)
		go func(workerID int) {
			defer wg.Done()
			
			for i := 0; i < b.N/numWorkers; i++ {
				peerID := peers[(workerID*1000+i)%numPeers]
				
				switch i % 4 {
				case 0:
					// Buffer adaptation
					bandwidth := int64(rand.Intn(10*1024*1024) + 1024*1024)
					latency := time.Duration(rand.Intn(100)+10) * time.Millisecond
					bkm.AdaptBufferSize(peerID, bandwidth, latency)
					
				case 1:
					// Keep-alive management
					bkm.EnableKeepAlive(peerID, i%20 == 0)
					
				case 2:
					// Latency recording
					latency := time.Duration(rand.Intn(500)+10) * time.Millisecond
					bkm.RecordLatency(peerID, latency)
					
				case 3:
					// Metrics retrieval
					bkm.GetConnectionMetrics(peerID)
				}
			}
		}(w)
	}
	
	wg.Wait()
	b.StopTimer()
	
	opsPerSec := float64(b.N) / b.Elapsed().Seconds()
	b.ReportMetric(opsPerSec, "ops/sec")
	b.ReportMetric(opsPerSec/float64(numWorkers), "ops/sec/worker")
}

// BenchmarkMemoryUsageUnderLoad measures memory usage under sustained load
func BenchmarkMemoryUsageUnderLoad(b *testing.B) {
	h := &mockHost{}
	
	config := DefaultBufferKeepAliveConfig()
	bkm := NewBufferKeepAliveManager(h, config)
	
	numPeers := 10000
	peers := make([]peer.ID, numPeers)
	for i := range peers {
		peers[i] = libp2ptest.RandPeerIDFatal(b)
	}
	
	b.ResetTimer()
	b.ReportAllocs()
	
	// Simulate sustained load
	for i := 0; i < b.N; i++ {
		peerID := peers[i%numPeers]
		
		// Perform various operations
		bandwidth := int64(rand.Intn(20*1024*1024) + 1024*1024)
		latency := time.Duration(rand.Intn(200)+10) * time.Millisecond
		
		bkm.AdaptBufferSize(peerID, bandwidth, latency)
		bkm.EnableKeepAlive(peerID, i%100 == 0)
		bkm.RecordLatency(peerID, latency)
		
		// Periodically clean up to simulate real usage
		if i%1000 == 0 {
			// Simulate some disconnections
			for j := 0; j < 10; j++ {
				disconnectPeer := peers[(i+j)%numPeers]
				bkm.DisableKeepAlive(disconnectPeer)
			}
		}
	}
}

// BenchmarkLatencyDistribution measures latency distribution for operations
func BenchmarkLatencyDistribution(b *testing.B) {
	h := &mockHost{}
	
	bkm := NewBufferKeepAliveManager(h, nil)
	peerID := libp2ptest.RandPeerIDFatal(b)
	
	latencies := make([]time.Duration, b.N)
	
	b.ResetTimer()
	
	for i := 0; i < b.N; i++ {
		start := time.Now()
		
		bandwidth := int64(rand.Intn(10*1024*1024) + 1024*1024)
		latency := time.Duration(rand.Intn(100)+10) * time.Millisecond
		bkm.AdaptBufferSize(peerID, bandwidth, latency)
		
		latencies[i] = time.Since(start)
	}
	
	b.StopTimer()
	
	// Calculate percentiles
	// Sort latencies for percentile calculation
	for i := 0; i < len(latencies)-1; i++ {
		for j := i + 1; j < len(latencies); j++ {
			if latencies[i] > latencies[j] {
				latencies[i], latencies[j] = latencies[j], latencies[i]
			}
		}
	}
	
	p50 := latencies[len(latencies)*50/100]
	p95 := latencies[len(latencies)*95/100]
	p99 := latencies[len(latencies)*99/100]
	
	b.ReportMetric(float64(p50.Nanoseconds()), "p50-latency-ns")
	b.ReportMetric(float64(p95.Nanoseconds()), "p95-latency-ns")
	b.ReportMetric(float64(p99.Nanoseconds()), "p99-latency-ns")
}

// BenchmarkHighConnectionChurn measures performance under high connection churn
func BenchmarkHighConnectionChurn(b *testing.B) {
	h := &mockHost{}
	
	config := DefaultBufferKeepAliveConfig()
	bkm := NewBufferKeepAliveManager(h, config)
	
	b.ResetTimer()
	
	for i := 0; i < b.N; i++ {
		peerID := libp2ptest.RandPeerIDFatal(b)
		
		// Simulate connection establishment
		bkm.EnableKeepAlive(peerID, i%10 == 0)
		bkm.AdaptBufferSize(peerID, int64(1024*1024), 50*time.Millisecond)
		
		// Simulate some activity
		bkm.RecordLatency(peerID, 30*time.Millisecond)
		
		// Simulate disconnection
		bkm.DisableKeepAlive(peerID)
	}
	
	b.StopTimer()
	
	connectionsPerSec := float64(b.N) / b.Elapsed().Seconds()
	b.ReportMetric(connectionsPerSec, "connections/sec")
}

// BenchmarkNetworkPerformanceProfile provides a comprehensive performance profile
func BenchmarkNetworkPerformanceProfile(b *testing.B) {
	scenarios := []struct {
		name      string
		peers     int
		duration  time.Duration
		workers   int
	}{
		{"small-cluster", 100, 5 * time.Second, 2},
		{"medium-cluster", 1000, 10 * time.Second, 5},
		{"large-cluster", 5000, 15 * time.Second, 10},
		{"enterprise-cluster", 10000, 20 * time.Second, 20},
	}
	
	for _, scenario := range scenarios {
		b.Run(scenario.name, func(b *testing.B) {
			h := &mockHost{}
			
			config := DefaultBufferKeepAliveConfig()
			config.BufferAdaptationInterval = 1 * time.Second
			config.KeepAliveCheckInterval = 2 * time.Second
			
			bkm := NewBufferKeepAliveManager(h, config)
			
			peers := make([]peer.ID, scenario.peers)
			for i := range peers {
				peers[i] = libp2ptest.RandPeerIDFatal(b)
			}
			
			ctx, cancel := context.WithTimeout(context.Background(), scenario.duration)
			defer cancel()
			
			err := bkm.Start(ctx)
			if err != nil {
				b.Fatal(err)
			}
			defer bkm.Stop()
			
			b.ResetTimer()
			
			var wg sync.WaitGroup
			operationCount := int64(0)
			
			for w := 0; w < scenario.workers; w++ {
				wg.Add(1)
				go func(workerID int) {
					defer wg.Done()
					
					ops := 0
					ticker := time.NewTicker(10 * time.Millisecond)
					defer ticker.Stop()
					
					for {
						select {
						case <-ctx.Done():
							return
						case <-ticker.C:
							peerID := peers[(workerID*1000+ops)%len(peers)]
							
							// Perform mixed operations
							switch ops % 6 {
							case 0:
								bkm.AdaptBufferSize(peerID, int64(rand.Intn(20*1024*1024)+1024*1024), 
									time.Duration(rand.Intn(200)+10)*time.Millisecond)
							case 1:
								bkm.EnableKeepAlive(peerID, ops%50 == 0)
							case 2:
								bkm.RecordLatency(peerID, time.Duration(rand.Intn(500)+10)*time.Millisecond)
							case 3:
								bkm.GetConnectionMetrics(peerID)
							case 4:
								bkm.IsSlowConnection(peerID)
							case 5:
								bkm.GetBufferSize(peerID)
							}
							
							ops++
							operationCount++
						}
					}
				}(w)
			}
			
			wg.Wait()
			b.StopTimer()
			
			opsPerSec := float64(operationCount) / scenario.duration.Seconds()
			opsPerPeer := float64(operationCount) / float64(scenario.peers)
			
			b.ReportMetric(opsPerSec, "ops/sec")
			b.ReportMetric(opsPerPeer, "ops/peer")
			b.ReportMetric(float64(scenario.peers), "total-peers")
		})
	}
}

// Helper function to create a realistic network load pattern
func createRealisticLoadPattern(bkm *BufferKeepAliveManager, peers []peer.ID, duration time.Duration) {
	ctx, cancel := context.WithTimeout(context.Background(), duration)
	defer cancel()
	
	ticker := time.NewTicker(100 * time.Millisecond)
	defer ticker.Stop()
	
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			// Simulate realistic network patterns
			for i, peerID := range peers {
				if rand.Float64() < 0.1 { // 10% activity rate per tick
					// Simulate data transfer
					bandwidth := int64(rand.Intn(10*1024*1024) + 100*1024) // 100KB-10MB/s
					latency := time.Duration(rand.Intn(200)+10) * time.Millisecond
					
					bkm.AdaptBufferSize(peerID, bandwidth, latency)
					bkm.RecordLatency(peerID, latency)
					
					// Occasionally enable/disable keep-alive
					if i%100 == 0 {
						bkm.EnableKeepAlive(peerID, i%1000 == 0)
					}
				}
			}
		}
	}
}