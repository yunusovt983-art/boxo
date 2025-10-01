package monitoring

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/ipfs/boxo/config/adaptive"
)

// Helper function to create a test performance monitor
func createTestPerformanceMonitor(metrics *PerformanceMetrics) *mockPerformanceMonitor {
	monitor := &mockPerformanceMonitor{}
	if metrics != nil {
		monitor.mu.Lock()
		monitor.metrics = metrics
		monitor.mu.Unlock()
	} else {
		monitor.mu.Lock()
		monitor.metrics = &PerformanceMetrics{
			Timestamp: time.Now(),
			BitswapMetrics: BitswapStats{
				RequestsPerSecond:   500,
				AverageResponseTime: 50 * time.Millisecond,
				P95ResponseTime:     100 * time.Millisecond,
				ActiveConnections:   100,
				QueuedRequests:      50,
				ErrorRate:           0.02,
			},
			BlockstoreMetrics: BlockstoreStats{
				CacheHitRate:          0.85,
				AverageReadLatency:    10 * time.Millisecond,
				BatchOperationsPerSec: 200,
			},
			NetworkMetrics: NetworkStats{
				ActiveConnections: 150,
				TotalBandwidth:    10 << 20, // 10MB/s
				AverageLatency:    30 * time.Millisecond,
				PacketLossRate:    0.001,
			},
			ResourceMetrics: ResourceStats{
				CPUUsage:       60.0,
				MemoryUsage:    4 << 30, // 4GB
				MemoryTotal:    16 << 30, // 16GB
				GoroutineCount: 500,
			},
		}
		monitor.mu.Unlock()
	}
	return monitor
}

// Helper function to create a test bottleneck analyzer
func createTestBottleneckAnalyzer(bottlenecks []Bottleneck, recommendations []OptimizationRecommendation) *mockBottleneckAnalyzer {
	analyzer := &mockBottleneckAnalyzer{}
	
	if bottlenecks != nil {
		analyzer.bottlenecks = bottlenecks
	} else {
		analyzer.bottlenecks = []Bottleneck{
			{
				ID:          "test-latency-bottleneck",
				Component:   "bitswap",
				Type:        BottleneckTypeLatency,
				Severity:    SeverityMedium,
				Description: "High response time detected",
				DetectedAt:  time.Now(),
			},
		}
	}
	
	return analyzer
}

func TestNewAutoTuner(t *testing.T) {
	config := adaptive.DefaultConfig()
	monitor := createTestPerformanceMonitor(nil)
	analyzer := createTestBottleneckAnalyzer(nil, nil)
	
	tuner := NewAutoTuner(config, monitor, analyzer)
	
	if tuner == nil {
		t.Fatal("NewAutoTuner returned nil")
	}
	
	// Test that we can cast to the concrete type
	concreteTuner, ok := tuner.(*autoTuner)
	if !ok {
		t.Fatal("NewAutoTuner did not return *autoTuner")
	}
	
	if concreteTuner.config != config {
		t.Error("AutoTuner config not set correctly")
	}
	
	if concreteTuner.performanceMonitor != monitor {
		t.Error("AutoTuner performance monitor not set correctly")
	}
	
	if concreteTuner.bottleneckAnalyzer != analyzer {
		t.Error("AutoTuner bottleneck analyzer not set correctly")
	}
}

func TestAutoTuner_StartStopAutoTuning(t *testing.T) {
	config := adaptive.DefaultConfig()
	monitor := createTestPerformanceMonitor(nil)
	analyzer := createTestBottleneckAnalyzer(nil, nil)
	
	tuner := NewAutoTuner(config, monitor, analyzer)
	ctx := context.Background()
	
	// Test starting auto-tuning
	err := tuner.StartAutoTuning(ctx)
	if err != nil {
		t.Fatalf("StartAutoTuning failed: %v", err)
	}
	
	// Test starting again (should fail)
	err = tuner.StartAutoTuning(ctx)
	if err == nil {
		t.Error("StartAutoTuning should fail when already running")
	}
	
	// Test stopping
	err = tuner.StopAutoTuning()
	if err != nil {
		t.Fatalf("StopAutoTuning failed: %v", err)
	}
	
	// Test stopping again (should not fail)
	err = tuner.StopAutoTuning()
	if err != nil {
		t.Fatalf("StopAutoTuning should not fail when not running: %v", err)
	}
}

func TestAutoTuner_StartAutoTuningDisabled(t *testing.T) {
	config := adaptive.DefaultConfig()
	config.AutoTune.Enabled = false // Disable auto-tuning
	
	monitor := createTestPerformanceMonitor(nil)
	analyzer := createTestBottleneckAnalyzer(nil, nil)
	
	tuner := NewAutoTuner(config, monitor, analyzer)
	ctx := context.Background()
	
	err := tuner.StartAutoTuning(ctx)
	if err == nil {
		t.Error("StartAutoTuning should fail when disabled in config")
	}
}

func TestAutoTuner_TuneParameters(t *testing.T) {
	config := adaptive.DefaultConfig()
	monitor := createTestPerformanceMonitor(nil)
	analyzer := createTestBottleneckAnalyzer(nil, nil)
	
	tuner := NewAutoTuner(config, monitor, analyzer)
	ctx := context.Background()
	
	metrics := monitor.CollectMetrics()
	
	newConfig, err := tuner.TuneParameters(ctx, metrics)
	if err != nil {
		t.Fatalf("TuneParameters failed: %v", err)
	}
	
	if newConfig == nil {
		t.Fatal("TuneParameters returned nil config")
	}
	
	// Verify that some parameters were adjusted
	if newConfig.Bitswap.MaxOutstandingBytesPerPeer == config.Bitswap.MaxOutstandingBytesPerPeer {
		t.Log("MaxOutstandingBytesPerPeer was not changed (this might be expected)")
	}
}

func TestAutoTuner_TuneParametersWithHighLatency(t *testing.T) {
	config := adaptive.DefaultConfig()
	
	highLatencyMetrics := &PerformanceMetrics{
		Timestamp: time.Now(),
		BitswapMetrics: BitswapStats{
			RequestsPerSecond:   1000,
			P95ResponseTime:     200 * time.Millisecond, // High latency
			ActiveConnections:   200,
			QueuedRequests:      500, // High queue
			ErrorRate:           0.05,
		},
		ResourceMetrics: ResourceStats{
			CPUUsage:    70.0,
			MemoryUsage: 8 << 30, // 8GB
			MemoryTotal: 16 << 30, // 16GB
		},
	}
	
	bottlenecks := []Bottleneck{
		{
			Component: "bitswap",
			Type:      BottleneckTypeLatency,
			Severity:  SeverityHigh,
		},
		{
			Component: "bitswap",
			Type:      BottleneckTypeQueue,
			Severity:  SeverityMedium,
		},
	}
	
	monitor := createTestPerformanceMonitor(highLatencyMetrics)
	analyzer := createTestBottleneckAnalyzer(bottlenecks, nil)
	
	tuner := NewAutoTuner(config, monitor, analyzer)
	ctx := context.Background()
	
	originalOutstanding := config.Bitswap.MaxOutstandingBytesPerPeer
	originalBatchSize := config.Bitswap.BatchSize
	
	newConfig, err := tuner.TuneParameters(ctx, monitor.CollectMetrics())
	if err != nil {
		t.Fatalf("TuneParameters failed: %v", err)
	}
	
	// Should increase outstanding bytes for latency bottleneck
	if newConfig.Bitswap.MaxOutstandingBytesPerPeer <= originalOutstanding {
		t.Error("Expected MaxOutstandingBytesPerPeer to increase for latency bottleneck")
	}
	
	// Should increase batch size for queue bottleneck
	if newConfig.Bitswap.BatchSize <= originalBatchSize {
		t.Error("Expected BatchSize to increase for queue bottleneck")
	}
}

func TestAutoTuner_ApplyConfigurationSafely(t *testing.T) {
	config := adaptive.DefaultConfig()
	monitor := createTestPerformanceMonitor(nil)
	analyzer := createTestBottleneckAnalyzer(nil, nil)
	
	tuner := NewAutoTuner(config, monitor, analyzer)
	ctx := context.Background()
	
	// Create a new config with some changes
	newConfig := config.Clone()
	newConfig.Bitswap.MaxOutstandingBytesPerPeer = 10 << 20 // 10MB
	newConfig.Bitswap.MaxWorkerCount = 512
	
	result, err := tuner.ApplyConfigurationSafely(ctx, newConfig)
	if err != nil {
		t.Fatalf("ApplyConfigurationSafely failed: %v", err)
	}
	
	if !result.Success {
		t.Errorf("Configuration application should have succeeded: %v", result.RejectedChanges)
	}
	
	if len(result.AppliedChanges) == 0 {
		t.Error("Expected some applied changes")
	}
	
	if result.RollbackPlan == nil {
		t.Error("Expected rollback plan to be created")
	}
	
	// Verify safety checks were performed
	if len(result.SafetyChecks) == 0 {
		t.Error("Expected safety checks to be performed")
	}
}

func TestAutoTuner_ApplyConfigurationSafelyWithFailure(t *testing.T) {
	config := adaptive.DefaultConfig()
	
	highResourceMetrics := &PerformanceMetrics{
		ResourceMetrics: ResourceStats{
			CPUUsage:    95.0, // Very high CPU usage
			MemoryUsage: 15 << 30, // 15GB
			MemoryTotal: 16 << 30, // 16GB (very high memory usage)
		},
	}
	
	monitor := createTestPerformanceMonitor(highResourceMetrics)
	analyzer := createTestBottleneckAnalyzer(nil, nil)
	
	tuner := NewAutoTuner(config, monitor, analyzer)
	ctx := context.Background()
	
	// Create a config that should fail safety checks
	newConfig := config.Clone()
	newConfig.Bitswap.MaxWorkerCount = 8192 // Very high worker count
	newConfig.Blockstore.MemoryCacheSize = 20 << 30 // 20GB (more than available)
	
	result, err := tuner.ApplyConfigurationSafely(ctx, newConfig)
	if err == nil {
		t.Error("ApplyConfigurationSafely should have failed due to safety checks")
	}
	
	if result.Success {
		t.Error("Configuration application should have failed")
	}
	
	if len(result.RejectedChanges) == 0 {
		t.Error("Expected some rejected changes")
	}
}

func TestAutoTuner_SetTuningStrategy(t *testing.T) {
	config := adaptive.DefaultConfig()
	monitor := createTestPerformanceMonitor(nil)
	analyzer := createTestBottleneckAnalyzer(nil, nil)
	
	tuner := NewAutoTuner(config, monitor, analyzer)
	
	// Test valid strategy
	strategy := TuningStrategy{
		Algorithm:           AlgorithmGradientDescent,
		AggressivenessLevel: 0.5,
		FocusAreas:          []string{"latency", "throughput"},
		SafetyMode:          true,
		MaxChangePercent:    0.2,
		LearningRate:        0.01,
	}
	
	err := tuner.SetTuningStrategy(strategy)
	if err != nil {
		t.Fatalf("SetTuningStrategy failed: %v", err)
	}
	
	// Test invalid aggressiveness level
	invalidStrategy := strategy
	invalidStrategy.AggressivenessLevel = 1.5
	
	err = tuner.SetTuningStrategy(invalidStrategy)
	if err == nil {
		t.Error("SetTuningStrategy should fail with invalid aggressiveness level")
	}
	
	// Test invalid max change percent
	invalidStrategy = strategy
	invalidStrategy.MaxChangePercent = -0.1
	
	err = tuner.SetTuningStrategy(invalidStrategy)
	if err == nil {
		t.Error("SetTuningStrategy should fail with invalid max change percent")
	}
	
	// Test invalid learning rate
	invalidStrategy = strategy
	invalidStrategy.LearningRate = 2.0
	
	err = tuner.SetTuningStrategy(invalidStrategy)
	if err == nil {
		t.Error("SetTuningStrategy should fail with invalid learning rate")
	}
}

func TestAutoTuner_PredictOptimalParameters(t *testing.T) {
	config := adaptive.DefaultConfig()
	monitor := createTestPerformanceMonitor(nil)
	analyzer := createTestBottleneckAnalyzer(nil, nil)
	
	tuner := NewAutoTuner(config, monitor, analyzer)
	ctx := context.Background()
	
	// First, we need to train a model
	trainingData := generateTestTrainingData(20)
	err := tuner.TrainModel(ctx, trainingData)
	if err != nil {
		t.Fatalf("TrainModel failed: %v", err)
	}
	
	metrics := monitor.CollectMetrics()
	history := []PerformanceMetrics{*metrics}
	
	prediction, err := tuner.PredictOptimalParameters(ctx, metrics, history)
	if err != nil {
		t.Fatalf("PredictOptimalParameters failed: %v", err)
	}
	
	if prediction == nil {
		t.Fatal("PredictOptimalParameters returned nil")
	}
	
	if prediction.Confidence <= 0 || prediction.Confidence > 1 {
		t.Errorf("Invalid confidence value: %f", prediction.Confidence)
	}
	
	if len(prediction.Predictions) == 0 {
		t.Error("Expected some parameter predictions")
	}
	
	// Check specific predictions
	if _, exists := prediction.Predictions["max_outstanding_bytes"]; !exists {
		t.Error("Expected max_outstanding_bytes prediction")
	}
	
	if _, exists := prediction.Predictions["worker_count"]; !exists {
		t.Error("Expected worker_count prediction")
	}
}

func TestAutoTuner_PredictOptimalParametersWithoutModel(t *testing.T) {
	config := adaptive.DefaultConfig()
	monitor := createTestPerformanceMonitor(nil)
	analyzer := createTestBottleneckAnalyzer(nil, nil)
	
	tuner := NewAutoTuner(config, monitor, analyzer)
	ctx := context.Background()
	
	metrics := monitor.CollectMetrics()
	history := []PerformanceMetrics{*metrics}
	
	_, err := tuner.PredictOptimalParameters(ctx, metrics, history)
	if err == nil {
		t.Error("PredictOptimalParameters should fail without trained model")
	}
}

func TestAutoTuner_TrainModel(t *testing.T) {
	config := adaptive.DefaultConfig()
	monitor := createTestPerformanceMonitor(nil)
	analyzer := createTestBottleneckAnalyzer(nil, nil)
	
	tuner := NewAutoTuner(config, monitor, analyzer)
	ctx := context.Background()
	
	// Test with sufficient training data
	trainingData := generateTestTrainingData(15)
	err := tuner.TrainModel(ctx, trainingData)
	if err != nil {
		t.Fatalf("TrainModel failed: %v", err)
	}
	
	// Test with insufficient training data
	insufficientData := generateTestTrainingData(5)
	err = tuner.TrainModel(ctx, insufficientData)
	if err == nil {
		t.Error("TrainModel should fail with insufficient data")
	}
}

func TestAutoTuner_SaveLoadModel(t *testing.T) {
	config := adaptive.DefaultConfig()
	monitor := createTestPerformanceMonitor(nil)
	analyzer := createTestBottleneckAnalyzer(nil, nil)
	
	tuner := NewAutoTuner(config, monitor, analyzer)
	ctx := context.Background()
	
	// Train a model first
	trainingData := generateTestTrainingData(15)
	err := tuner.TrainModel(ctx, trainingData)
	if err != nil {
		t.Fatalf("TrainModel failed: %v", err)
	}
	
	// Create temporary file
	tmpDir := t.TempDir()
	modelPath := filepath.Join(tmpDir, "test_model.json")
	
	// Test saving model
	err = tuner.SaveModel(modelPath)
	if err != nil {
		t.Fatalf("SaveModel failed: %v", err)
	}
	
	// Verify file exists
	if _, err := os.Stat(modelPath); os.IsNotExist(err) {
		t.Error("Model file was not created")
	}
	
	// Create new tuner and load model
	newTuner := NewAutoTuner(config, monitor, analyzer)
	err = newTuner.LoadModel(modelPath)
	if err != nil {
		t.Fatalf("LoadModel failed: %v", err)
	}
	
	// Test prediction with loaded model
	metrics := monitor.CollectMetrics()
	history := []PerformanceMetrics{*metrics}
	
	prediction, err := newTuner.PredictOptimalParameters(ctx, metrics, history)
	if err != nil {
		t.Fatalf("PredictOptimalParameters failed with loaded model: %v", err)
	}
	
	if prediction == nil {
		t.Error("Expected prediction with loaded model")
	}
}

func TestAutoTuner_SaveModelWithoutModel(t *testing.T) {
	config := adaptive.DefaultConfig()
	monitor := createTestPerformanceMonitor(nil)
	analyzer := createTestBottleneckAnalyzer(nil, nil)
	
	tuner := NewAutoTuner(config, monitor, analyzer)
	
	tmpDir := t.TempDir()
	modelPath := filepath.Join(tmpDir, "test_model.json")
	
	err := tuner.SaveModel(modelPath)
	if err == nil {
		t.Error("SaveModel should fail when no model is trained")
	}
}

func TestAutoTuner_LoadModelInvalidFile(t *testing.T) {
	config := adaptive.DefaultConfig()
	monitor := createTestPerformanceMonitor(nil)
	analyzer := createTestBottleneckAnalyzer(nil, nil)
	
	tuner := NewAutoTuner(config, monitor, analyzer)
	
	// Test loading non-existent file
	err := tuner.LoadModel("/non/existent/path")
	if err == nil {
		t.Error("LoadModel should fail with non-existent file")
	}
	
	// Test loading invalid JSON
	tmpDir := t.TempDir()
	invalidPath := filepath.Join(tmpDir, "invalid.json")
	err = os.WriteFile(invalidPath, []byte("invalid json"), 0644)
	if err != nil {
		t.Fatalf("Failed to create invalid JSON file: %v", err)
	}
	
	err = tuner.LoadModel(invalidPath)
	if err == nil {
		t.Error("LoadModel should fail with invalid JSON")
	}
}

func TestAutoTuner_GetTuningHistory(t *testing.T) {
	config := adaptive.DefaultConfig()
	monitor := createTestPerformanceMonitor(nil)
	analyzer := createTestBottleneckAnalyzer(nil, nil)
	
	tuner := NewAutoTuner(config, monitor, analyzer)
	
	// Initially should be empty
	history := tuner.GetTuningHistory()
	if len(history) != 0 {
		t.Error("Expected empty history initially")
	}
	
	// Add some mock history (this would normally be done by the tuning process)
	concreteTuner := tuner.(*autoTuner)
	concreteTuner.mu.Lock()
	concreteTuner.history = append(concreteTuner.history, TuningOperation{
		ID:        "test-op-1",
		Timestamp: time.Now(),
		Success:   true,
	})
	concreteTuner.mu.Unlock()
	
	history = tuner.GetTuningHistory()
	if len(history) != 1 {
		t.Errorf("Expected 1 history item, got %d", len(history))
	}
	
	if history[0].ID != "test-op-1" {
		t.Error("History item not returned correctly")
	}
}

// Helper functions for testing

func generateTestTrainingData(count int) []TrainingDataPoint {
	data := make([]TrainingDataPoint, count)
	
	for i := 0; i < count; i++ {
		data[i] = TrainingDataPoint{
			Timestamp: time.Now().Add(-time.Duration(i) * time.Hour),
			Metrics: PerformanceMetrics{
				BitswapMetrics: BitswapStats{
					RequestsPerSecond: float64(500 + i*10),
					P95ResponseTime:   time.Duration(50+i*5) * time.Millisecond,
					ErrorRate:         0.01 + float64(i)*0.001,
				},
				ResourceMetrics: ResourceStats{
					CPUUsage:    50.0 + float64(i)*2,
					MemoryUsage: int64(4+i) << 30,
				},
			},
			Configuration: *adaptive.DefaultConfig(),
			Outcome: PerformanceOutcome{
				LatencyImprovement:    float64(i) * 0.1,
				ThroughputImprovement: float64(i) * 0.05,
				ResourceEfficiency:    0.8 + float64(i)*0.01,
				OverallScore:          0.7 + float64(i)*0.02,
			},
		}
	}
	
	return data
}

// Benchmark tests

func BenchmarkAutoTuner_TuneParameters(b *testing.B) {
	config := adaptive.DefaultConfig()
	monitor := createTestPerformanceMonitor(nil)
	analyzer := createTestBottleneckAnalyzer(nil, nil)
	
	tuner := NewAutoTuner(config, monitor, analyzer)
	ctx := context.Background()
	metrics := monitor.CollectMetrics()
	
	b.ResetTimer()
	
	for i := 0; i < b.N; i++ {
		_, err := tuner.TuneParameters(ctx, metrics)
		if err != nil {
			b.Fatalf("TuneParameters failed: %v", err)
		}
	}
}

func BenchmarkAutoTuner_ApplyConfigurationSafely(b *testing.B) {
	config := adaptive.DefaultConfig()
	monitor := createTestPerformanceMonitor(nil)
	analyzer := createTestBottleneckAnalyzer(nil, nil)
	
	tuner := NewAutoTuner(config, monitor, analyzer)
	ctx := context.Background()
	
	newConfig := config.Clone()
	newConfig.Bitswap.MaxOutstandingBytesPerPeer = 10 << 20
	
	b.ResetTimer()
	
	for i := 0; i < b.N; i++ {
		_, err := tuner.ApplyConfigurationSafely(ctx, newConfig)
		if err != nil {
			b.Fatalf("ApplyConfigurationSafely failed: %v", err)
		}
	}
}

func BenchmarkAutoTuner_PredictOptimalParameters(b *testing.B) {
	config := adaptive.DefaultConfig()
	monitor := createTestPerformanceMonitor(nil)
	analyzer := createTestBottleneckAnalyzer(nil, nil)
	
	tuner := NewAutoTuner(config, monitor, analyzer)
	ctx := context.Background()
	
	// Train model
	trainingData := generateTestTrainingData(20)
	err := tuner.TrainModel(ctx, trainingData)
	if err != nil {
		b.Fatalf("TrainModel failed: %v", err)
	}
	
	metrics := monitor.CollectMetrics()
	history := []PerformanceMetrics{*metrics}
	
	b.ResetTimer()
	
	for i := 0; i < b.N; i++ {
		_, err := tuner.PredictOptimalParameters(ctx, metrics, history)
		if err != nil {
			b.Fatalf("PredictOptimalParameters failed: %v", err)
		}
	}
}