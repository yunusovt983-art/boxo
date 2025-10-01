package monitoring

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/ipfs/boxo/config/adaptive"
)

// Example_autoTunerBasicUsage demonstrates basic AutoTuner usage
func Example_autoTunerBasicUsage() {
	// Create configuration with auto-tuning enabled
	config := adaptive.DefaultConfig()
	config.AutoTune.Enabled = true
	config.AutoTune.TuningInterval = 30 * time.Second
	config.AutoTune.SafetyMode = true
	config.AutoTune.MaxChangePercent = 0.1 // 10% max change per cycle

	// Create performance monitor and bottleneck analyzer
	monitor := NewPerformanceMonitor()
	analyzer := NewBottleneckAnalyzer()

	// Create auto-tuner
	tuner := NewAutoTuner(config, monitor, analyzer)

	// Set tuning strategy
	strategy := TuningStrategy{
		Algorithm:           AlgorithmRulesBased,
		AggressivenessLevel: 0.3, // Conservative
		FocusAreas:          []string{"latency", "throughput"},
		SafetyMode:          true,
		MaxChangePercent:    0.1,
		LearningRate:        0.01,
	}

	err := tuner.SetTuningStrategy(strategy)
	if err != nil {
		log.Fatalf("Failed to set tuning strategy: %v", err)
	}

	// Start auto-tuning
	ctx := context.Background()
	err = tuner.StartAutoTuning(ctx)
	if err != nil {
		log.Fatalf("Failed to start auto-tuning: %v", err)
	}

	fmt.Println("AutoTuner started successfully")

	// Simulate running for a while
	time.Sleep(100 * time.Millisecond)

	// Stop auto-tuning
	err = tuner.StopAutoTuning()
	if err != nil {
		log.Fatalf("Failed to stop auto-tuning: %v", err)
	}

	fmt.Println("AutoTuner stopped successfully")

	// Output:
	// AutoTuner started successfully
	// AutoTuner stopped successfully
}

// Example_autoTunerManualTuning demonstrates manual parameter tuning
func Example_autoTunerManualTuning() {
	// Create configuration
	config := adaptive.DefaultConfig()
	monitor := createTestPerformanceMonitor(nil)
	analyzer := createTestBottleneckAnalyzer(nil, nil)

	// Create auto-tuner
	tuner := NewAutoTuner(config, monitor, analyzer)

	// Get current metrics
	metrics := monitor.CollectMetrics()

	// Perform manual tuning
	ctx := context.Background()
	newConfig, err := tuner.TuneParameters(ctx, metrics)
	if err != nil {
		log.Fatalf("Failed to tune parameters: %v", err)
	}

	// Apply configuration safely
	result, err := tuner.ApplyConfigurationSafely(ctx, newConfig)
	if err != nil {
		fmt.Printf("Configuration application failed: %v\n", err)
		return
	}

	if result.Success {
		fmt.Printf("Configuration applied successfully with %d changes\n", len(result.AppliedChanges))
	} else {
		fmt.Printf("Configuration application failed with %d rejected changes\n", len(result.RejectedChanges))
	}

	// Output:
	// Configuration application failed: safety checks failed
}

// Example_autoTunerMachineLearning demonstrates ML-based parameter prediction
func Example_autoTunerMachineLearning() {
	// Create configuration
	config := adaptive.DefaultConfig()
	config.AutoTune.LearningEnabled = true
	config.AutoTune.TrainingEnabled = true

	monitor := createTestPerformanceMonitor(nil)
	analyzer := createTestBottleneckAnalyzer(nil, nil)

	// Create auto-tuner
	tuner := NewAutoTuner(config, monitor, analyzer)

	// Generate training data
	trainingData := generateTestTrainingData(20)

	// Train the model
	ctx := context.Background()
	err := tuner.TrainModel(ctx, trainingData)
	if err != nil {
		log.Fatalf("Failed to train model: %v", err)
	}

	// Get current metrics and history
	metrics := monitor.CollectMetrics()
	history := []PerformanceMetrics{*metrics}

	// Predict optimal parameters
	prediction, err := tuner.PredictOptimalParameters(ctx, metrics, history)
	if err != nil {
		log.Fatalf("Failed to predict parameters: %v", err)
	}

	fmt.Printf("ML prediction confidence: %.2f\n", prediction.Confidence)
	fmt.Printf("Number of parameter predictions: %d\n", len(prediction.Predictions))

	// Output:
	// ML prediction confidence: 0.50
	// Number of parameter predictions: 2
}

// Example_autoTunerSafetyChecks demonstrates safety check mechanisms
func Example_autoTunerSafetyChecks() {
	// Create configuration
	config := adaptive.DefaultConfig()

	// Create monitor with high resource usage
	highResourceMetrics := &PerformanceMetrics{
		ResourceMetrics: ResourceStats{
			CPUUsage:    95.0, // Very high CPU
			MemoryUsage: 15 << 30, // 15GB
			MemoryTotal: 16 << 30, // 16GB total
		},
	}

	monitor := createTestPerformanceMonitor(highResourceMetrics)
	analyzer := createTestBottleneckAnalyzer(nil, nil)

	// Create auto-tuner
	tuner := NewAutoTuner(config, monitor, analyzer)

	// Try to apply aggressive configuration changes
	aggressiveConfig := config.Clone()
	aggressiveConfig.Bitswap.MaxWorkerCount = 8192 // Very high worker count
	aggressiveConfig.Blockstore.MemoryCacheSize = 20 << 30 // 20GB cache (more than available)

	// Apply configuration (should fail safety checks)
	ctx := context.Background()
	result, err := tuner.ApplyConfigurationSafely(ctx, aggressiveConfig)

	if err != nil {
		fmt.Printf("Configuration rejected due to safety checks: %v\n", err)
	}

	if !result.Success {
		fmt.Printf("Safety checks prevented %d dangerous changes\n", len(result.RejectedChanges))
		for checkName, reason := range result.RejectedChanges {
			fmt.Printf("- %s: %s\n", checkName, reason)
		}
	}

	// Output:
	// Configuration rejected due to safety checks: safety checks failed
	// Safety checks prevented 2 dangerous changes
	// - memory_limits: Proposed memory cache size exceeds available memory
	// - cpu_limits: Cannot increase worker count when CPU usage is already high
}

// Example_autoTunerTuningHistory demonstrates accessing tuning history
func Example_autoTunerTuningHistory() {
	// Create configuration
	config := adaptive.DefaultConfig()
	monitor := createTestPerformanceMonitor(nil)
	analyzer := createTestBottleneckAnalyzer(nil, nil)

	// Create auto-tuner
	tuner := NewAutoTuner(config, monitor, analyzer)

	// Simulate some tuning operations by adding to history
	concreteTuner := tuner.(*autoTuner)
	concreteTuner.mu.Lock()
	concreteTuner.history = append(concreteTuner.history, TuningOperation{
		ID:              "op-1",
		Timestamp:       time.Now(),
		TriggerReason:   "High latency detected",
		Success:         true,
		PerformanceGain: 0.15, // 15% improvement
	})
	concreteTuner.history = append(concreteTuner.history, TuningOperation{
		ID:              "op-2",
		Timestamp:       time.Now(),
		TriggerReason:   "Memory pressure",
		Success:         false,
		PerformanceGain: 0.0,
	})
	concreteTuner.mu.Unlock()

	// Get tuning history
	history := tuner.GetTuningHistory()

	fmt.Printf("Total tuning operations: %d\n", len(history))
	for _, op := range history {
		status := "failed"
		if op.Success {
			status = "succeeded"
		}
		fmt.Printf("- Operation %s: %s (%.1f%% gain)\n", op.ID, status, op.PerformanceGain*100)
	}

	// Output:
	// Total tuning operations: 2
	// - Operation op-1: succeeded (15.0% gain)
	// - Operation op-2: failed (0.0% gain)
}

// Example_autoTunerModelPersistence demonstrates saving and loading ML models
func Example_autoTunerModelPersistence() {
	// Create configuration
	config := adaptive.DefaultConfig()
	monitor := createTestPerformanceMonitor(nil)
	analyzer := createTestBottleneckAnalyzer(nil, nil)

	// Create auto-tuner and train model
	tuner := NewAutoTuner(config, monitor, analyzer)

	trainingData := generateTestTrainingData(15)
	ctx := context.Background()
	err := tuner.TrainModel(ctx, trainingData)
	if err != nil {
		log.Fatalf("Failed to train model: %v", err)
	}

	// Save model to temporary file
	modelPath := "/tmp/autotuner_model.json"
	err = tuner.SaveModel(modelPath)
	if err != nil {
		log.Fatalf("Failed to save model: %v", err)
	}

	fmt.Println("Model saved successfully")

	// Create new tuner and load model
	newTuner := NewAutoTuner(config, monitor, analyzer)
	err = newTuner.LoadModel(modelPath)
	if err != nil {
		log.Fatalf("Failed to load model: %v", err)
	}

	fmt.Println("Model loaded successfully")

	// Test prediction with loaded model
	metrics := monitor.CollectMetrics()
	history := []PerformanceMetrics{*metrics}

	prediction, err := newTuner.PredictOptimalParameters(ctx, metrics, history)
	if err != nil {
		log.Fatalf("Failed to predict with loaded model: %v", err)
	}

	fmt.Printf("Prediction with loaded model: %.2f confidence\n", prediction.Confidence)

	// Output:
	// Model saved successfully
	// Model loaded successfully
	// Prediction with loaded model: 0.50 confidence
}