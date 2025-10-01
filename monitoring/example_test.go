package monitoring

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"time"

	"github.com/prometheus/client_golang/prometheus/promhttp"
)

// ExamplePerformanceMonitor demonstrates how to use the performance monitoring system
func ExamplePerformanceMonitor() {
	// Create a new performance monitor
	monitor := NewPerformanceMonitor()
	
	// Create and register collectors for different components
	bitswapCollector := NewBitswapCollector()
	blockstoreCollector := NewBlockstoreCollector()
	networkCollector := NewNetworkCollector()
	resourceCollector := NewResourceCollector()
	
	monitor.RegisterCollector("bitswap", bitswapCollector)
	monitor.RegisterCollector("blockstore", blockstoreCollector)
	monitor.RegisterCollector("network", networkCollector)
	monitor.RegisterCollector("resource", resourceCollector)
	
	// Start continuous metric collection every 5 seconds
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	
	err := monitor.StartCollection(ctx, 5*time.Second)
	if err != nil {
		log.Printf("Failed to start metric collection: %v", err)
		return
	}
	
	// Simulate some Bitswap activity
	go func() {
		for i := 0; i < 10; i++ {
			bitswapCollector.RecordRequest()
			
			// Simulate processing time
			processingTime := time.Duration(50+i*10) * time.Millisecond
			time.Sleep(processingTime)
			
			// Record response (90% success rate)
			success := i%10 != 0
			bitswapCollector.RecordResponse(processingTime, success)
			
			// Update connection counts
			bitswapCollector.UpdateActiveConnections(10 + i)
			bitswapCollector.UpdateQueuedRequests(int64(5 + i))
			
			// Record block activity
			bitswapCollector.RecordBlockReceived(i%5 == 0) // 20% duplicates
			bitswapCollector.RecordBlockSent()
		}
	}()
	
	// Simulate some blockstore activity
	go func() {
		for i := 0; i < 10; i++ {
			// Simulate cache hits/misses (80% hit rate)
			if i%5 != 0 {
				blockstoreCollector.RecordCacheHit()
			} else {
				blockstoreCollector.RecordCacheMiss()
			}
			
			// Record read/write operations
			readLatency := time.Duration(1+i) * time.Millisecond
			writeLatency := time.Duration(2+i) * time.Millisecond
			
			blockstoreCollector.RecordReadOperation(readLatency)
			blockstoreCollector.RecordWriteOperation(writeLatency)
			blockstoreCollector.RecordBatchOperation()
			
			// Update storage metrics
			blockstoreCollector.UpdateTotalBlocks(int64(1000 + i*100))
			blockstoreCollector.UpdateCacheSize(int64(1024*1024 + i*1024))
			blockstoreCollector.UpdateCompressionRatio(0.7 + float64(i)*0.01)
			
			time.Sleep(100 * time.Millisecond)
		}
	}()
	
	// Simulate network activity
	go func() {
		for i := 0; i < 10; i++ {
			networkCollector.UpdateActiveConnections(5 + i)
			networkCollector.RecordLatency(time.Duration(20+i*5) * time.Millisecond)
			networkCollector.RecordBandwidth(int64(1024*1024 + i*1024*512)) // Increasing bandwidth
			
			// Record peer metrics
			peerID := fmt.Sprintf("peer_%d", i%3)
			networkCollector.RecordPeerLatency(peerID, time.Duration(15+i*3)*time.Millisecond)
			networkCollector.RecordPeerBandwidth(peerID, int64(512*1024+i*1024*256))
			networkCollector.RecordPeerBytes(peerID, int64(1024+i*512), int64(2048+i*1024))
			
			if i%7 == 0 { // Occasional errors
				networkCollector.RecordConnectionError()
				networkCollector.RecordPeerError(peerID)
			}
			
			time.Sleep(150 * time.Millisecond)
		}
	}()
	
	// Simulate resource monitoring
	go func() {
		for i := 0; i < 10; i++ {
			// Simulate increasing resource usage
			cpuUsage := 20.0 + float64(i)*5.0
			diskUsed := int64(10*1024*1024*1024 + i*1024*1024*1024) // 10GB + 1GB per iteration
			diskTotal := int64(100 * 1024 * 1024 * 1024)           // 100GB total
			
			resourceCollector.UpdateCPUUsage(cpuUsage)
			resourceCollector.UpdateDiskUsage(diskUsed, diskTotal)
			
			time.Sleep(200 * time.Millisecond)
		}
	}()
	
	// Set up Prometheus metrics endpoint
	http.Handle("/metrics", promhttp.HandlerFor(monitor.GetPrometheusRegistry(), promhttp.HandlerOpts{}))
	
	// Start HTTP server for metrics in a separate goroutine
	go func() {
		log.Println("Starting metrics server on :8080/metrics")
		if err := http.ListenAndServe(":8080", nil); err != nil {
			log.Printf("Metrics server error: %v", err)
		}
	}()
	
	// Collect and display metrics periodically
	ticker := time.NewTicker(2 * time.Second)
	defer ticker.Stop()
	
	for i := 0; i < 5; i++ {
		select {
		case <-ticker.C:
			metrics := monitor.CollectMetrics()
			fmt.Printf("\n=== Metrics Collection %d ===\n", i+1)
			fmt.Printf("Timestamp: %s\n", metrics.Timestamp.Format(time.RFC3339))
			
			// Display Bitswap metrics
			if bitswapMetrics, ok := metrics.CustomMetrics["bitswap"].(map[string]interface{}); ok {
				fmt.Printf("Bitswap - Active Connections: %v, Total Requests: %v, Error Rate: %.2f%%\n",
					bitswapMetrics["active_connections"],
					bitswapMetrics["total_requests"],
					bitswapMetrics["error_rate"].(float64)*100)
			}
			
			// Display Blockstore metrics
			if blockstoreMetrics, ok := metrics.CustomMetrics["blockstore"].(map[string]interface{}); ok {
				fmt.Printf("Blockstore - Cache Hit Rate: %.2f%%, Total Blocks: %v, Compression Ratio: %.2f\n",
					blockstoreMetrics["cache_hit_rate"].(float64)*100,
					blockstoreMetrics["total_blocks"],
					blockstoreMetrics["compression_ratio"])
			}
			
			// Display Network metrics
			if networkMetrics, ok := metrics.CustomMetrics["network"].(map[string]interface{}); ok {
				fmt.Printf("Network - Active Connections: %v, Bandwidth: %v bytes/s, Avg Latency: %v\n",
					networkMetrics["active_connections"],
					networkMetrics["total_bandwidth"],
					networkMetrics["average_latency"])
			}
			
			// Display Resource metrics
			if resourceMetrics, ok := metrics.CustomMetrics["resource"].(map[string]interface{}); ok {
				fmt.Printf("Resources - CPU: %.1f%%, Memory: %d MB, Goroutines: %v\n",
					resourceMetrics["cpu_usage"],
					resourceMetrics["memory_usage"].(int64)/(1024*1024),
					resourceMetrics["goroutine_count"])
			}
		}
	}
	
	// Stop collection
	err = monitor.StopCollection()
	if err != nil {
		log.Printf("Failed to stop metric collection: %v", err)
	}
	
	fmt.Println("\nMetrics collection stopped. Check http://localhost:8080/metrics for Prometheus format.")
}

// ExampleBitswapCollector demonstrates how to use the Bitswap collector
func ExampleBitswapCollector() {
	collector := NewBitswapCollector()
	
	// Simulate Bitswap operations
	for i := 0; i < 5; i++ {
		// Record a request
		collector.RecordRequest()
		
		// Simulate processing time
		processingTime := time.Duration(50+i*10) * time.Millisecond
		
		// Record the response (success)
		collector.RecordResponse(processingTime, true)
		
		// Record block activity
		collector.RecordBlockReceived(i%3 == 0) // Some duplicates
		collector.RecordBlockSent()
	}
	
	// Update connection status
	collector.UpdateActiveConnections(15)
	collector.UpdateQueuedRequests(25)
	
	// Collect metrics
	metrics, err := collector.CollectMetrics(context.Background())
	if err != nil {
		log.Printf("Error collecting metrics: %v", err)
		return
	}
	
	fmt.Printf("Total Requests: %v\n", metrics["total_requests"])
	fmt.Printf("Average Response Time: %v\n", metrics["average_response_time"])
	fmt.Printf("Active Connections: %v\n", metrics["active_connections"])
	fmt.Printf("Blocks Received: %v\n", metrics["blocks_received"])
	fmt.Printf("Blocks Sent: %v\n", metrics["blocks_sent"])
	fmt.Printf("Error Rate: %.2f%%\n", metrics["error_rate"].(float64)*100)
}

// Example_integration demonstrates a complete integration test
func Example_integration() {
	// Create monitor and collectors
	monitor := NewPerformanceMonitor()
	bitswapCollector := NewBitswapCollector()
	blockstoreCollector := NewBlockstoreCollector()
	
	// Register collectors
	monitor.RegisterCollector("bitswap", bitswapCollector)
	monitor.RegisterCollector("blockstore", blockstoreCollector)
	
	// Start collection
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	
	err := monitor.StartCollection(ctx, 100*time.Millisecond)
	if err != nil {
		log.Printf("Failed to start collection: %v", err)
		return
	}
	
	// Simulate high load scenario
	for i := 0; i < 100; i++ {
		// Bitswap activity
		bitswapCollector.RecordRequest()
		bitswapCollector.RecordResponse(time.Duration(i%50)*time.Millisecond, i%10 != 0)
		bitswapCollector.UpdateActiveConnections(50 + i%20)
		
		// Blockstore activity
		if i%4 == 0 {
			blockstoreCollector.RecordCacheMiss()
		} else {
			blockstoreCollector.RecordCacheHit()
		}
		blockstoreCollector.RecordReadOperation(time.Duration(i%10) * time.Millisecond)
		
		if i%10 == 0 {
			// Collect metrics periodically
			metrics := monitor.CollectMetrics()
			if bitswapMetrics, ok := metrics.CustomMetrics["bitswap"].(map[string]interface{}); ok {
				fmt.Printf("Iteration %d - Active Connections: %v, Error Rate: %.1f%%\n",
					i, bitswapMetrics["active_connections"], bitswapMetrics["error_rate"].(float64)*100)
			}
		}
		
		time.Sleep(10 * time.Millisecond)
	}
	
	// Final metrics collection
	finalMetrics := monitor.CollectMetrics()
	
	// Verify metrics are reasonable
	if bitswapMetrics, ok := finalMetrics.CustomMetrics["bitswap"].(map[string]interface{}); ok {
		totalRequests := bitswapMetrics["total_requests"].(int64)
		errorRate := bitswapMetrics["error_rate"].(float64)
		
		fmt.Printf("Final Results - Total Requests: %d, Error Rate: %.1f%%\n", totalRequests, errorRate*100)
		
		if totalRequests != 100 {
			fmt.Printf("Warning: Expected 100 total requests, got %d\n", totalRequests)
		}
		
		if errorRate < 0.05 || errorRate > 0.15 {
			fmt.Printf("Warning: Error rate %.1f%% is outside expected range (5-15%%)\n", errorRate*100)
		}
	}
	
	if blockstoreMetrics, ok := finalMetrics.CustomMetrics["blockstore"].(map[string]interface{}); ok {
		cacheHitRate := blockstoreMetrics["cache_hit_rate"].(float64)
		fmt.Printf("Blockstore Cache Hit Rate: %.1f%%\n", cacheHitRate*100)
		
		if cacheHitRate < 0.7 || cacheHitRate > 0.8 {
			fmt.Printf("Warning: Cache hit rate %.1f%% is outside expected range (70-80%%)\n", cacheHitRate*100)
		}
	}
	
	monitor.StopCollection()
	fmt.Println("Integration test completed successfully")
}