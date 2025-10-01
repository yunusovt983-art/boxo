package monitoring

import (
	"context"
	"testing"
	"time"
)

func TestNewBottleneckAnalyzer(t *testing.T) {
	analyzer := NewBottleneckAnalyzer()
	
	if analyzer == nil {
		t.Fatal("NewBottleneckAnalyzer returned nil")
	}
	
	// Test that default thresholds are set
	ba := analyzer.(*bottleneckAnalyzer)
	if ba.thresholds.Bitswap.MaxResponseTime == 0 {
		t.Error("Default Bitswap thresholds not set")
	}
}

func TestAnalyzeBottlenecks(t *testing.T) {
	analyzer := NewBottleneckAnalyzer()
	ctx := context.Background()
	
	// Test with normal metrics (should not detect bottlenecks)
	normalMetrics := createNormalMetrics()
	bottlenecks, err := analyzer.AnalyzeBottlenecks(ctx, normalMetrics)
	if err != nil {
		t.Fatalf("AnalyzeBottlenecks failed: %v", err)
	}
	if len(bottlenecks) != 0 {
		t.Errorf("Expected no bottlenecks for normal metrics, got %d", len(bottlenecks))
	}
	
	// Test with problematic metrics (should detect bottlenecks)
	problematicMetrics := createProblematicMetrics()
	bottlenecks, err = analyzer.AnalyzeBottlenecks(ctx, problematicMetrics)
	if err != nil {
		t.Fatalf("AnalyzeBottlenecks failed: %v", err)
	}
	if len(bottlenecks) == 0 {
		t.Error("Expected bottlenecks for problematic metrics, got none")
	}
}

func TestAnalyzeBitswapBottlenecks(t *testing.T) {
	analyzer := NewBottleneckAnalyzer().(*bottleneckAnalyzer)
	
	// Test high latency detection
	metrics := createNormalMetrics()
	metrics.BitswapMetrics.P95ResponseTime = 200 * time.Millisecond // Above threshold
	
	bottlenecks := analyzer.analyzeBitswapBottlenecks(metrics)
	if len(bottlenecks) == 0 {
		t.Error("Expected latency bottleneck, got none")
	}
	
	found := false
	for _, b := range bottlenecks {
		if b.Type == BottleneckTypeLatency {
			found = true
			break
		}
	}
	if !found {
		t.Error("Expected latency bottleneck type")
	}
}

func TestAnalyzeTrends(t *testing.T) {
	analyzer := NewBottleneckAnalyzer()
	ctx := context.Background()
	
	// Test with insufficient data
	history := []PerformanceMetrics{*createNormalMetrics()}
	trends, err := analyzer.AnalyzeTrends(ctx, history)
	if err == nil {
		t.Error("Expected error for insufficient data")
	}
	
	// Test with sufficient data
	history = createTrendHistory()
	trends, err = analyzer.AnalyzeTrends(ctx, history)
	if err != nil {
		t.Fatalf("AnalyzeTrends failed: %v", err)
	}
	if len(trends) == 0 {
		t.Error("Expected trends, got none")
	}
}

func TestDetectAnomalies(t *testing.T) {
	analyzer := NewBottleneckAnalyzer()
	ctx := context.Background()
	
	// Create baseline
	baseline := createTestBaseline()
	
	// Test with normal metrics (should not detect anomalies)
	normalMetrics := createNormalMetrics()
	anomalies, err := analyzer.DetectAnomalies(ctx, normalMetrics, baseline)
	if err != nil {
		t.Fatalf("DetectAnomalies failed: %v", err)
	}
	if len(anomalies) != 0 {
		t.Errorf("Expected no anomalies for normal metrics, got %d", len(anomalies))
	}
	
	// Test without baseline
	_, err = analyzer.DetectAnomalies(ctx, normalMetrics, nil)
	if err == nil {
		t.Error("Expected error when baseline is nil")
	}
}

func TestGenerateRecommendations(t *testing.T) {
	analyzer := NewBottleneckAnalyzer()
	ctx := context.Background()
	
	// Create bottlenecks
	bottlenecks := []Bottleneck{
		{
			Component: "bitswap",
			Type:      BottleneckTypeLatency,
			Severity:  SeverityHigh,
		},
	}
	
	recommendations, err := analyzer.GenerateRecommendations(ctx, bottlenecks)
	if err != nil {
		t.Fatalf("GenerateRecommendations failed: %v", err)
	}
	
	if len(recommendations) == 0 {
		t.Error("Expected recommendations, got none")
	}
}

func TestUpdateBaseline(t *testing.T) {
	analyzer := NewBottleneckAnalyzer()
	
	// Test updating baseline with metrics
	metrics := createNormalMetrics()
	err := analyzer.UpdateBaseline(metrics)
	if err != nil {
		t.Fatalf("UpdateBaseline failed: %v", err)
	}
	
	// Add more metrics to build baseline
	for i := 0; i < 15; i++ {
		metrics.Timestamp = time.Now().Add(time.Duration(i) * time.Minute)
		err = analyzer.UpdateBaseline(metrics)
		if err != nil {
			t.Fatalf("UpdateBaseline failed: %v", err)
		}
	}
	
	// Verify baseline was calculated
	baseline := analyzer.GetBaseline()
	if baseline == nil {
		t.Fatal("GetBaseline returned nil")
	}
}

func TestSetThresholds(t *testing.T) {
	analyzer := NewBottleneckAnalyzer()
	
	// Test valid thresholds
	validThresholds := PerformanceThresholds{
		Bitswap: BitswapThresholds{
			MaxResponseTime:      50 * time.Millisecond,
			MaxQueuedRequests:    500,
			MaxErrorRate:         0.02,
			MinRequestsPerSecond: 20,
		},
		Blockstore: BlockstoreThresholds{
			MinCacheHitRate:          0.85,
			MaxReadLatency:           25 * time.Millisecond,
			MaxWriteLatency:          50 * time.Millisecond,
			MinBatchOperationsPerSec: 200,
		},
		Network: NetworkThresholds{
			MaxLatency:          100 * time.Millisecond,
			MinBandwidth:        2 * 1024 * 1024,
			MaxPacketLossRate:   0.005,
			MaxConnectionErrors: 5,
		},
		Resource: ResourceThresholds{
			MaxCPUUsage:       70.0,
			MaxMemoryUsage:    80.0,
			MaxDiskUsage:      85.0,
			MaxGoroutineCount: 8000,
		},
	}
	
	err := analyzer.SetThresholds(validThresholds)
	if err != nil {
		t.Fatalf("SetThresholds failed with valid thresholds: %v", err)
	}
	
	// Test invalid thresholds
	invalidThresholds := validThresholds
	invalidThresholds.Bitswap.MaxResponseTime = 0 // Invalid
	
	err = analyzer.SetThresholds(invalidThresholds)
	if err == nil {
		t.Error("Expected error for invalid thresholds")
	}
}

// Helper functions for creating test data

func createNormalMetrics() *PerformanceMetrics {
	return &PerformanceMetrics{
		Timestamp: time.Now(),
		BitswapMetrics: BitswapStats{
			RequestsPerSecond:   50.0,
			AverageResponseTime: 30 * time.Millisecond,
			P95ResponseTime:     80 * time.Millisecond,
			P99ResponseTime:     120 * time.Millisecond,
			OutstandingRequests: 100,
			ActiveConnections:   10,
			QueuedRequests:      50,
			BlocksReceived:      1000,
			BlocksSent:          800,
			DuplicateBlocks:     10,
			ErrorRate:           0.01,
		},
		BlockstoreMetrics: BlockstoreStats{
			CacheHitRate:          0.85,
			AverageReadLatency:    10 * time.Millisecond,
			AverageWriteLatency:   20 * time.Millisecond,
			BatchOperationsPerSec: 200,
			CompressionRatio:      0.7,
			TotalBlocks:           10000,
			CacheSize:             1024 * 1024 * 1024,
			DiskUsage:             5 * 1024 * 1024 * 1024,
		},
		NetworkMetrics: NetworkStats{
			ActiveConnections: 15,
			TotalBandwidth:    2 * 1024 * 1024,
			AverageLatency:    50 * time.Millisecond,
			PacketLossRate:    0.005,
			ConnectionErrors:  2,
			BytesSent:         1024 * 1024 * 100,
			BytesReceived:     1024 * 1024 * 120,
		},
		ResourceMetrics: ResourceStats{
			CPUUsage:       45.0,
			MemoryUsage:    4 * 1024 * 1024 * 1024,
			MemoryTotal:    8 * 1024 * 1024 * 1024,
			DiskUsage:      50 * 1024 * 1024 * 1024,
			DiskTotal:      100 * 1024 * 1024 * 1024,
			GoroutineCount: 500,
			GCPauseTime:    5 * time.Millisecond,
		},
	}
}

func createProblematicMetrics() *PerformanceMetrics {
	metrics := createNormalMetrics()
	
	// Make metrics problematic
	metrics.BitswapMetrics.P95ResponseTime = 200 * time.Millisecond // High latency
	metrics.BitswapMetrics.QueuedRequests = 2000                    // High queue depth
	metrics.BitswapMetrics.ErrorRate = 0.1                          // High error rate
	
	metrics.BlockstoreMetrics.CacheHitRate = 0.5                    // Low cache hit rate
	metrics.NetworkMetrics.AverageLatency = 300 * time.Millisecond  // High network latency
	metrics.ResourceMetrics.CPUUsage = 90.0                         // High CPU usage
	
	return metrics
}

func createTrendHistory() []PerformanceMetrics {
	history := make([]PerformanceMetrics, 10)
	baseTime := time.Now().Add(-10 * time.Minute)
	
	for i := 0; i < 10; i++ {
		metrics := createNormalMetrics()
		metrics.Timestamp = baseTime.Add(time.Duration(i) * time.Minute)
		
		// Create increasing trend in response time
		metrics.BitswapMetrics.AverageResponseTime = time.Duration(20+i*5) * time.Millisecond
		
		history[i] = *metrics
	}
	
	return history
}

func createTestBaseline() *MetricBaseline {
	return &MetricBaseline{
		UpdatedAt: time.Now(),
		Bitswap: BitswapBaseline{
			ResponseTime: StatisticalBaseline{
				Mean:        float64(30 * time.Millisecond),
				StdDev:      float64(10 * time.Millisecond),
				SampleCount: 100,
			},
		},
	}
}

// Benchmark tests

func BenchmarkAnalyzeBottlenecks(b *testing.B) {
	analyzer := NewBottleneckAnalyzer()
	ctx := context.Background()
	metrics := createProblematicMetrics()
	
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := analyzer.AnalyzeBottlenecks(ctx, metrics)
		if err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkAnalyzeTrends(b *testing.B) {
	analyzer := NewBottleneckAnalyzer()
	ctx := context.Background()
	history := createTrendHistory()
	
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := analyzer.AnalyzeTrends(ctx, history)
		if err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkDetectAnomalies(b *testing.B) {
	analyzer := NewBottleneckAnalyzer()
	ctx := context.Background()
	metrics := createNormalMetrics()
	baseline := createTestBaseline()
	
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := analyzer.DetectAnomalies(ctx, metrics, baseline)
		if err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkUpdateBaseline(b *testing.B) {
	analyzer := NewBottleneckAnalyzer()
	metrics := createNormalMetrics()
	
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		err := analyzer.UpdateBaseline(metrics)
		if err != nil {
			b.Fatal(err)
		}
	}
}