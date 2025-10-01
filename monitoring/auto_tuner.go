package monitoring

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"os"
	"sync"
	"time"

	"github.com/ipfs/boxo/config/adaptive"
)

// AutoTuner defines the interface for automatic parameter tuning
type AutoTuner interface {
	// StartAutoTuning begins the automatic tuning process
	StartAutoTuning(ctx context.Context) error
	
	// StopAutoTuning stops the automatic tuning process
	StopAutoTuning() error
	
	// TuneParameters performs a single tuning cycle
	TuneParameters(ctx context.Context, metrics *PerformanceMetrics) (*adaptive.AdaptiveConfig, error)
	
	// PredictOptimalParameters uses ML to predict optimal parameters
	PredictOptimalParameters(ctx context.Context, metrics *PerformanceMetrics, history []PerformanceMetrics) (*ParameterPrediction, error)
	
	// ApplyConfigurationSafely applies configuration changes with safety checks
	ApplyConfigurationSafely(ctx context.Context, newConfig *adaptive.AdaptiveConfig) (*ConfigurationChangeResult, error)
	
	// GetTuningHistory returns the history of tuning operations
	GetTuningHistory() []TuningOperation
	
	// SetTuningStrategy configures the tuning strategy
	SetTuningStrategy(strategy TuningStrategy) error
	
	// TrainModel trains the ML model with historical data
	TrainModel(ctx context.Context, trainingData []TrainingDataPoint) error
	
	// SaveModel saves the trained model to disk
	SaveModel(path string) error
	
	// LoadModel loads a trained model from disk
	LoadModel(path string) error
}

// ParameterPrediction contains ML-based parameter predictions
type ParameterPrediction struct {
	Timestamp   time.Time                  `json:"timestamp"`
	Confidence  float64                    `json:"confidence"`
	Predictions map[string]ParameterValue  `json:"predictions"`
	Reasoning   string                     `json:"reasoning"`
	ModelUsed   string                     `json:"model_used"`
}

// ParameterValue represents a predicted parameter value with confidence
type ParameterValue struct {
	Value      interface{} `json:"value"`
	Confidence float64     `json:"confidence"`
	Change     float64     `json:"change_percent"`
	Impact     string      `json:"expected_impact"`
}

// ConfigurationChangeResult contains the result of applying configuration changes
type ConfigurationChangeResult struct {
	Success         bool                       `json:"success"`
	AppliedChanges  map[string]interface{}     `json:"applied_changes"`
	RejectedChanges map[string]string          `json:"rejected_changes"`
	SafetyChecks    []SafetyCheckResult        `json:"safety_checks"`
	RollbackPlan    *RollbackPlan              `json:"rollback_plan"`
	Timestamp       time.Time                  `json:"timestamp"`
}

// SafetyCheckResult represents the result of a safety check
type SafetyCheckResult struct {
	CheckName   string      `json:"check_name"`
	Passed      bool        `json:"passed"`
	Message     string      `json:"message"`
	Severity    Severity    `json:"severity"`
	Details     interface{} `json:"details"`
}

// RollbackPlan contains information needed to rollback configuration changes
type RollbackPlan struct {
	PreviousConfig *adaptive.AdaptiveConfig `json:"previous_config"`
	ChangeID       string                   `json:"change_id"`
	Timestamp      time.Time                `json:"timestamp"`
	AutoRollback   bool                     `json:"auto_rollback"`
	RollbackDelay  time.Duration            `json:"rollback_delay"`
}

// TuningOperation represents a single tuning operation
type TuningOperation struct {
	ID              string                      `json:"id"`
	Timestamp       time.Time                   `json:"timestamp"`
	TriggerReason   string                      `json:"trigger_reason"`
	MetricsBefore   *PerformanceMetrics         `json:"metrics_before"`
	MetricsAfter    *PerformanceMetrics         `json:"metrics_after"`
	ConfigChanges   map[string]interface{}      `json:"config_changes"`
	Result          ConfigurationChangeResult   `json:"result"`
	PerformanceGain float64                     `json:"performance_gain"`
	Success         bool                        `json:"success"`
}

// TuningStrategy defines the strategy for parameter tuning
type TuningStrategy struct {
	Algorithm           TuningAlgorithm `json:"algorithm"`
	AggressivenessLevel float64         `json:"aggressiveness_level"` // 0.0 (conservative) to 1.0 (aggressive)
	FocusAreas          []string        `json:"focus_areas"`          // e.g., ["latency", "throughput", "memory"]
	SafetyMode          bool            `json:"safety_mode"`
	MaxChangePercent    float64         `json:"max_change_percent"`
	LearningRate        float64         `json:"learning_rate"`
}

// TuningAlgorithm represents different tuning algorithms
type TuningAlgorithm string

const (
	AlgorithmGradientDescent TuningAlgorithm = "gradient_descent"
	AlgorithmBayesianOpt     TuningAlgorithm = "bayesian_optimization"
	AlgorithmGeneticAlg      TuningAlgorithm = "genetic_algorithm"
	AlgorithmRulesBased      TuningAlgorithm = "rules_based"
	AlgorithmHybrid          TuningAlgorithm = "hybrid"
)

// TrainingDataPoint represents a single training data point for ML
type TrainingDataPoint struct {
	Timestamp     time.Time                `json:"timestamp"`
	Metrics       PerformanceMetrics       `json:"metrics"`
	Configuration adaptive.AdaptiveConfig  `json:"configuration"`
	Outcome       PerformanceOutcome       `json:"outcome"`
}

// PerformanceOutcome represents the outcome of a configuration change
type PerformanceOutcome struct {
	LatencyImprovement    float64 `json:"latency_improvement"`
	ThroughputImprovement float64 `json:"throughput_improvement"`
	ResourceEfficiency    float64 `json:"resource_efficiency"`
	OverallScore          float64 `json:"overall_score"`
}

// MLModel represents a machine learning model for parameter prediction
type MLModel struct {
	ModelType    string                 `json:"model_type"`
	Version      string                 `json:"version"`
	TrainedAt    time.Time              `json:"trained_at"`
	Accuracy     float64                `json:"accuracy"`
	Parameters   map[string]interface{} `json:"parameters"`
	FeatureNames []string               `json:"feature_names"`
}

// autoTuner implements the AutoTuner interface
type autoTuner struct {
	mu                sync.RWMutex
	config            *adaptive.AdaptiveConfig
	performanceMonitor PerformanceMonitor
	bottleneckAnalyzer BottleneckAnalyzer
	
	// Tuning state
	isRunning       bool
	stopChan        chan struct{}
	strategy        TuningStrategy
	history         []TuningOperation
	maxHistory      int
	
	// ML components
	model           *MLModel
	trainingData    []TrainingDataPoint
	maxTrainingData int
	
	// Safety mechanisms
	rollbackPlans   map[string]*RollbackPlan
	safetyChecks    []SafetyCheck
	
	// Configuration tracking
	currentConfig   *adaptive.AdaptiveConfig
	previousConfigs []*adaptive.AdaptiveConfig
	maxConfigHistory int
}

// SafetyCheck defines a safety check function
type SafetyCheck func(current, proposed *adaptive.AdaptiveConfig, metrics *PerformanceMetrics) SafetyCheckResult

// NewAutoTuner creates a new auto-tuner instance
func NewAutoTuner(config *adaptive.AdaptiveConfig, monitor PerformanceMonitor, analyzer BottleneckAnalyzer) AutoTuner {
	at := &autoTuner{
		config:             config,
		performanceMonitor: monitor,
		bottleneckAnalyzer: analyzer,
		stopChan:           make(chan struct{}),
		strategy:           getDefaultTuningStrategy(),
		history:            make([]TuningOperation, 0),
		maxHistory:         1000,
		trainingData:       make([]TrainingDataPoint, 0),
		maxTrainingData:    10000,
		rollbackPlans:      make(map[string]*RollbackPlan),
		safetyChecks:       getDefaultSafetyChecks(),
		currentConfig:      config.Clone(),
		previousConfigs:    make([]*adaptive.AdaptiveConfig, 0),
		maxConfigHistory:   100,
	}
	
	return at
}

// getDefaultTuningStrategy returns a conservative default tuning strategy
func getDefaultTuningStrategy() TuningStrategy {
	return TuningStrategy{
		Algorithm:           AlgorithmRulesBased,
		AggressivenessLevel: 0.3, // Conservative
		FocusAreas:          []string{"latency", "throughput"},
		SafetyMode:          true,
		MaxChangePercent:    0.1, // 10% max change
		LearningRate:        0.01,
	}
}

// getDefaultSafetyChecks returns default safety check functions
func getDefaultSafetyChecks() []SafetyCheck {
	return []SafetyCheck{
		checkMemoryLimits,
		checkCPULimits,
		checkNetworkLimits,
		checkConfigurationBounds,
		checkPerformanceRegression,
	}
}

// StartAutoTuning begins the automatic tuning process
func (at *autoTuner) StartAutoTuning(ctx context.Context) error {
	at.mu.Lock()
	defer at.mu.Unlock()
	
	if at.isRunning {
		return fmt.Errorf("auto-tuning is already running")
	}
	
	if !at.config.AutoTune.Enabled {
		return fmt.Errorf("auto-tuning is disabled in configuration")
	}
	
	at.isRunning = true
	
	go at.tuningLoop(ctx)
	
	return nil
}

// StopAutoTuning stops the automatic tuning process
func (at *autoTuner) StopAutoTuning() error {
	at.mu.Lock()
	defer at.mu.Unlock()
	
	if !at.isRunning {
		return nil
	}
	
	at.isRunning = false
	close(at.stopChan)
	at.stopChan = make(chan struct{})
	
	return nil
}

// tuningLoop runs the main tuning loop
func (at *autoTuner) tuningLoop(ctx context.Context) {
	ticker := time.NewTicker(at.config.AutoTune.TuningInterval)
	defer ticker.Stop()
	
	for {
		select {
		case <-ctx.Done():
			return
		case <-at.stopChan:
			return
		case <-ticker.C:
			metrics := at.performanceMonitor.CollectMetrics()
			if metrics != nil {
				if newConfig, err := at.TuneParameters(ctx, metrics); err == nil {
					at.ApplyConfigurationSafely(ctx, newConfig)
				}
			}
		}
	}
}

// TuneParameters performs a single tuning cycle
func (at *autoTuner) TuneParameters(ctx context.Context, metrics *PerformanceMetrics) (*adaptive.AdaptiveConfig, error) {
	at.mu.RLock()
	strategy := at.strategy
	currentConfig := at.currentConfig.Clone()
	at.mu.RUnlock()
	
	// Analyze current performance
	bottlenecks, err := at.bottleneckAnalyzer.AnalyzeBottlenecks(ctx, metrics)
	if err != nil {
		return nil, fmt.Errorf("failed to analyze bottlenecks: %w", err)
	}
	
	// Generate recommendations
	recommendations, err := at.bottleneckAnalyzer.GenerateRecommendations(ctx, bottlenecks)
	if err != nil {
		return nil, fmt.Errorf("failed to generate recommendations: %w", err)
	}
	
	// Apply tuning algorithm
	newConfig := currentConfig.Clone()
	
	switch strategy.Algorithm {
	case AlgorithmRulesBased:
		newConfig = at.applyRulesBasedTuning(newConfig, metrics, bottlenecks, recommendations)
	case AlgorithmGradientDescent:
		newConfig = at.applyGradientDescentTuning(newConfig, metrics, bottlenecks)
	case AlgorithmBayesianOpt:
		newConfig = at.applyBayesianOptimization(newConfig, metrics, bottlenecks)
	case AlgorithmHybrid:
		newConfig = at.applyHybridTuning(newConfig, metrics, bottlenecks, recommendations)
	default:
		return nil, fmt.Errorf("unsupported tuning algorithm: %s", strategy.Algorithm)
	}
	
	return newConfig, nil
}

// applyRulesBasedTuning applies rule-based tuning logic
func (at *autoTuner) applyRulesBasedTuning(config *adaptive.AdaptiveConfig, metrics *PerformanceMetrics, bottlenecks []Bottleneck, recommendations []OptimizationRecommendation) *adaptive.AdaptiveConfig {
	maxChange := at.strategy.MaxChangePercent
	
	// Apply recommendations based on bottlenecks
	for _, bottleneck := range bottlenecks {
		switch bottleneck.Component {
		case "bitswap":
			at.tuneBitswapParameters(config, bottleneck, maxChange)
		case "blockstore":
			at.tuneBlockstoreParameters(config, bottleneck, maxChange)
		case "network":
			at.tuneNetworkParameters(config, bottleneck, maxChange)
		case "resource":
			at.tuneResourceParameters(config, bottleneck, maxChange)
		}
	}
	
	// Apply specific recommendations
	for _, rec := range recommendations {
		at.applyRecommendation(config, rec, maxChange)
	}
	
	return config
}

// tuneBitswapParameters tunes Bitswap-specific parameters
func (at *autoTuner) tuneBitswapParameters(config *adaptive.AdaptiveConfig, bottleneck Bottleneck, maxChange float64) {
	switch bottleneck.Type {
	case BottleneckTypeLatency:
		// Increase outstanding bytes per peer to improve throughput
		current := config.Bitswap.MaxOutstandingBytesPerPeer
		increase := int64(float64(current) * maxChange)
		newValue := current + increase
		
		// Ensure we don't exceed reasonable limits
		if newValue <= 500<<20 { // 500MB max
			config.Bitswap.MaxOutstandingBytesPerPeer = newValue
		}
		
		// Increase worker count if needed
		if config.Bitswap.MaxWorkerCount < 4096 {
			currentWorkers := config.Bitswap.MaxWorkerCount
			increaseWorkers := int(float64(currentWorkers) * maxChange)
			config.Bitswap.MaxWorkerCount = currentWorkers + increaseWorkers
		}
		
	case BottleneckTypeQueue:
		// Increase batch size to process more requests at once
		current := config.Bitswap.BatchSize
		increase := int(float64(current) * maxChange)
		newValue := current + increase
		
		if newValue <= 2000 {
			config.Bitswap.BatchSize = newValue
		}
		
		// Reduce batch timeout to process queues faster
		currentTimeout := config.Bitswap.BatchTimeout
		reduction := time.Duration(float64(currentTimeout) * maxChange)
		newTimeout := currentTimeout - reduction
		
		if newTimeout >= time.Millisecond {
			config.Bitswap.BatchTimeout = newTimeout
		}
	}
}

// tuneBlockstoreParameters tunes Blockstore-specific parameters
func (at *autoTuner) tuneBlockstoreParameters(config *adaptive.AdaptiveConfig, bottleneck Bottleneck, maxChange float64) {
	switch bottleneck.Type {
	case BottleneckTypeCache:
		// Increase memory cache size
		current := config.Blockstore.MemoryCacheSize
		increase := int64(float64(current) * maxChange)
		newValue := current + increase
		
		// Don't exceed 16GB for memory cache
		if newValue <= 16<<30 {
			config.Blockstore.MemoryCacheSize = newValue
		}
		
	case BottleneckTypeLatency:
		// Increase batch size for better I/O efficiency
		current := config.Blockstore.BatchSize
		increase := int(float64(current) * maxChange)
		newValue := current + increase
		
		if newValue <= 5000 {
			config.Blockstore.BatchSize = newValue
		}
	}
}

// tuneNetworkParameters tunes Network-specific parameters
func (at *autoTuner) tuneNetworkParameters(config *adaptive.AdaptiveConfig, bottleneck Bottleneck, maxChange float64) {
	switch bottleneck.Type {
	case BottleneckTypeConnection:
		// Increase connection limits
		currentHigh := config.Network.HighWater
		increaseHigh := int(float64(currentHigh) * maxChange)
		newHigh := currentHigh + increaseHigh
		
		if newHigh <= 10000 {
			config.Network.HighWater = newHigh
			config.Network.LowWater = int(float64(newHigh) * 0.7) // Maintain 70% ratio
		}
		
	case BottleneckTypeLatency:
		// Increase buffer sizes for better throughput
		if config.Network.AdaptiveBufferSizing {
			current := config.Network.MaxBufferSize
			increase := int64(float64(current) * maxChange)
			newValue := current + increase
			
			if newValue <= 10<<20 { // 10MB max
				config.Network.MaxBufferSize = newValue
			}
		}
	}
}

// tuneResourceParameters tunes Resource-specific parameters
func (at *autoTuner) tuneResourceParameters(config *adaptive.AdaptiveConfig, bottleneck Bottleneck, maxChange float64) {
	switch bottleneck.Type {
	case BottleneckTypeResource:
		// Enable more aggressive resource management
		if !config.Resources.DegradationEnabled {
			config.Resources.DegradationEnabled = true
		}
		
		// Adjust memory pressure levels
		current := config.Resources.MemoryPressureLevel
		reduction := current * maxChange * 0.5 // Be conservative with resource limits
		newValue := current - reduction
		
		if newValue >= 0.5 { // Don't go below 50%
			config.Resources.MemoryPressureLevel = newValue
		}
	}
}

// applyRecommendation applies a specific optimization recommendation
func (at *autoTuner) applyRecommendation(config *adaptive.AdaptiveConfig, rec OptimizationRecommendation, maxChange float64) {
	for _, action := range rec.Actions {
		switch action.Type {
		case ActionTypeConfigChange:
			at.applyConfigChange(config, action.Parameters, maxChange)
		case ActionTypeScaleUp:
			at.applyScaleUp(config, action.Parameters, maxChange)
		case ActionTypeOptimize:
			at.applyOptimization(config, action.Parameters, maxChange)
		}
	}
}

// applyConfigChange applies a configuration change
func (at *autoTuner) applyConfigChange(config *adaptive.AdaptiveConfig, params map[string]interface{}, maxChange float64) {
	if configKey, ok := params["config_key"].(string); ok {
		if suggestedValue, ok := params["suggested_value"]; ok {
			at.applyParameterChange(config, configKey, suggestedValue, maxChange)
		}
	}
}

// applyParameterChange applies a specific parameter change
func (at *autoTuner) applyParameterChange(config *adaptive.AdaptiveConfig, key string, value interface{}, maxChange float64) {
	switch key {
	case "BitswapMaxOutstandingBytesPerPeer":
		if newValue, ok := at.parseByteSize(value); ok {
			current := config.Bitswap.MaxOutstandingBytesPerPeer
			maxIncrease := int64(float64(current) * maxChange)
			if newValue <= current+maxIncrease && newValue <= 500<<20 {
				config.Bitswap.MaxOutstandingBytesPerPeer = newValue
			}
		}
	case "BitswapWorkerCount":
		if newValue, ok := value.(int); ok {
			current := config.Bitswap.MaxWorkerCount
			maxIncrease := int(float64(current) * maxChange)
			if newValue <= current+maxIncrease && newValue <= 4096 {
				config.Bitswap.MaxWorkerCount = newValue
			}
		}
	}
}

// parseByteSize parses byte size strings like "5MB" to int64
func (at *autoTuner) parseByteSize(value interface{}) (int64, bool) {
	switch v := value.(type) {
	case string:
		// Simple parsing for common units
		if v == "5MB" {
			return 5 << 20, true
		}
		if v == "10MB" {
			return 10 << 20, true
		}
		if v == "100MB" {
			return 100 << 20, true
		}
	case int64:
		return v, true
	case int:
		return int64(v), true
	}
	return 0, false
}

// applyScaleUp applies scaling up operations
func (at *autoTuner) applyScaleUp(config *adaptive.AdaptiveConfig, params map[string]interface{}, maxChange float64) {
	// Implementation for scaling up resources
	if component, ok := params["component"].(string); ok {
		switch component {
		case "workers":
			current := config.Bitswap.MaxWorkerCount
			increase := int(float64(current) * maxChange)
			if current+increase <= 4096 {
				config.Bitswap.MaxWorkerCount = current + increase
			}
		case "connections":
			current := config.Network.HighWater
			increase := int(float64(current) * maxChange)
			if current+increase <= 10000 {
				config.Network.HighWater = current + increase
				config.Network.LowWater = int(float64(config.Network.HighWater) * 0.7)
			}
		}
	}
}

// applyOptimization applies optimization operations
func (at *autoTuner) applyOptimization(config *adaptive.AdaptiveConfig, params map[string]interface{}, maxChange float64) {
	// Implementation for optimization operations
	if optimizationType, ok := params["type"].(string); ok {
		switch optimizationType {
		case "compression":
			if !config.Blockstore.CompressionEnabled {
				config.Blockstore.CompressionEnabled = true
			}
		case "batching":
			current := config.Bitswap.BatchSize
			increase := int(float64(current) * maxChange)
			if current+increase <= 2000 {
				config.Bitswap.BatchSize = current + increase
			}
		}
	}
}

// applyGradientDescentTuning applies gradient descent-based tuning
func (at *autoTuner) applyGradientDescentTuning(config *adaptive.AdaptiveConfig, metrics *PerformanceMetrics, bottlenecks []Bottleneck) *adaptive.AdaptiveConfig {
	// Simplified gradient descent implementation
	learningRate := at.strategy.LearningRate
	
	// Calculate gradients based on performance metrics
	latencyGradient := at.calculateLatencyGradient(metrics)
	throughputGradient := at.calculateThroughputGradient(metrics)
	
	// Apply gradients to parameters
	if latencyGradient > 0 {
		// Increase parameters that reduce latency
		current := config.Bitswap.MaxOutstandingBytesPerPeer
		adjustment := int64(float64(current) * learningRate * latencyGradient)
		newValue := current + adjustment
		if newValue <= 500<<20 {
			config.Bitswap.MaxOutstandingBytesPerPeer = newValue
		}
	}
	
	if throughputGradient > 0 {
		// Increase parameters that improve throughput
		current := config.Bitswap.BatchSize
		adjustment := int(float64(current) * learningRate * throughputGradient)
		newValue := current + adjustment
		if newValue <= 2000 {
			config.Bitswap.BatchSize = newValue
		}
	}
	
	return config
}

// calculateLatencyGradient calculates the gradient for latency optimization
func (at *autoTuner) calculateLatencyGradient(metrics *PerformanceMetrics) float64 {
	// Simple gradient calculation based on response time
	targetLatency := 50 * time.Millisecond
	currentLatency := metrics.BitswapMetrics.P95ResponseTime
	
	if currentLatency > targetLatency {
		return float64(currentLatency-targetLatency) / float64(targetLatency)
	}
	return 0
}

// calculateThroughputGradient calculates the gradient for throughput optimization
func (at *autoTuner) calculateThroughputGradient(metrics *PerformanceMetrics) float64 {
	// Simple gradient calculation based on requests per second
	targetThroughput := 1000.0
	currentThroughput := metrics.BitswapMetrics.RequestsPerSecond
	
	if currentThroughput < targetThroughput {
		return (targetThroughput - currentThroughput) / targetThroughput
	}
	return 0
}

// applyBayesianOptimization applies Bayesian optimization (simplified)
func (at *autoTuner) applyBayesianOptimization(config *adaptive.AdaptiveConfig, metrics *PerformanceMetrics, bottlenecks []Bottleneck) *adaptive.AdaptiveConfig {
	// Simplified Bayesian optimization - in practice, this would use a proper BO library
	// For now, we'll use a heuristic approach
	
	// Explore parameter space based on uncertainty
	explorationFactor := 0.1
	
	// Adjust parameters with some exploration
	if len(at.history) > 10 {
		// Use historical data to guide exploration
		bestConfig := at.findBestHistoricalConfig()
		if bestConfig != nil {
			// Move towards best configuration with some exploration
			at.interpolateConfigs(config, bestConfig, explorationFactor)
		}
	}
	
	return config
}

// findBestHistoricalConfig finds the configuration that achieved the best performance
func (at *autoTuner) findBestHistoricalConfig() *adaptive.AdaptiveConfig {
	at.mu.RLock()
	defer at.mu.RUnlock()
	
	var bestConfig *adaptive.AdaptiveConfig
	bestScore := -1.0
	
	for _, op := range at.history {
		if op.Success && op.PerformanceGain > bestScore {
			bestScore = op.PerformanceGain
			// In practice, we'd store the full config in the history
			bestConfig = at.currentConfig // Simplified
		}
	}
	
	return bestConfig
}

// interpolateConfigs interpolates between two configurations
func (at *autoTuner) interpolateConfigs(target, source *adaptive.AdaptiveConfig, factor float64) {
	// Simplified interpolation for key parameters
	if source.Bitswap != nil && target.Bitswap != nil {
		current := target.Bitswap.MaxOutstandingBytesPerPeer
		desired := source.Bitswap.MaxOutstandingBytesPerPeer
		adjustment := int64(float64(desired-current) * factor)
		target.Bitswap.MaxOutstandingBytesPerPeer = current + adjustment
	}
}

// applyHybridTuning applies a hybrid approach combining multiple algorithms
func (at *autoTuner) applyHybridTuning(config *adaptive.AdaptiveConfig, metrics *PerformanceMetrics, bottlenecks []Bottleneck, recommendations []OptimizationRecommendation) *adaptive.AdaptiveConfig {
	// Start with rules-based tuning
	config = at.applyRulesBasedTuning(config, metrics, bottlenecks, recommendations)
	
	// Apply gradient descent for fine-tuning
	config = at.applyGradientDescentTuning(config, metrics, bottlenecks)
	
	// Add Bayesian exploration if we have enough history
	if len(at.history) > 20 {
		config = at.applyBayesianOptimization(config, metrics, bottlenecks)
	}
	
	return config
}

// PredictOptimalParameters uses ML to predict optimal parameters
func (at *autoTuner) PredictOptimalParameters(ctx context.Context, metrics *PerformanceMetrics, history []PerformanceMetrics) (*ParameterPrediction, error) {
	at.mu.RLock()
	model := at.model
	at.mu.RUnlock()
	
	if model == nil {
		return nil, fmt.Errorf("no trained model available")
	}
	
	// Extract features from metrics
	features := at.extractFeatures(metrics, history)
	
	// Make predictions using the model
	predictions := make(map[string]ParameterValue)
	
	// Simplified ML prediction - in practice, this would use a real ML library
	confidence := at.calculatePredictionConfidence(features)
	
	// Predict key parameters
	predictions["max_outstanding_bytes"] = ParameterValue{
		Value:      at.predictMaxOutstandingBytes(features),
		Confidence: confidence,
		Change:     0.1,
		Impact:     "Improved throughput",
	}
	
	predictions["worker_count"] = ParameterValue{
		Value:      at.predictWorkerCount(features),
		Confidence: confidence,
		Change:     0.05,
		Impact:     "Better parallelism",
	}
	
	return &ParameterPrediction{
		Timestamp:   time.Now(),
		Confidence:  confidence,
		Predictions: predictions,
		Reasoning:   "Based on historical performance patterns",
		ModelUsed:   model.ModelType,
	}, nil
}

// extractFeatures extracts features from metrics for ML prediction
func (at *autoTuner) extractFeatures(metrics *PerformanceMetrics, history []PerformanceMetrics) []float64 {
	features := make([]float64, 0, 20)
	
	// Current metrics features
	features = append(features, float64(metrics.BitswapMetrics.RequestsPerSecond))
	features = append(features, float64(metrics.BitswapMetrics.P95ResponseTime.Nanoseconds()))
	features = append(features, float64(metrics.BitswapMetrics.ActiveConnections))
	features = append(features, float64(metrics.BitswapMetrics.QueuedRequests))
	features = append(features, metrics.BitswapMetrics.ErrorRate)
	
	features = append(features, metrics.BlockstoreMetrics.CacheHitRate)
	features = append(features, float64(metrics.BlockstoreMetrics.AverageReadLatency.Nanoseconds()))
	features = append(features, float64(metrics.BlockstoreMetrics.BatchOperationsPerSec))
	
	features = append(features, float64(metrics.NetworkMetrics.ActiveConnections))
	features = append(features, float64(metrics.NetworkMetrics.TotalBandwidth))
	features = append(features, float64(metrics.NetworkMetrics.AverageLatency.Nanoseconds()))
	features = append(features, metrics.NetworkMetrics.PacketLossRate)
	
	features = append(features, metrics.ResourceMetrics.CPUUsage)
	features = append(features, float64(metrics.ResourceMetrics.MemoryUsage))
	features = append(features, float64(metrics.ResourceMetrics.GoroutineCount))
	
	// Historical trend features
	if len(history) > 0 {
		// Calculate trends
		latencyTrend := at.calculateTrend(history, func(m PerformanceMetrics) float64 {
			return float64(m.BitswapMetrics.P95ResponseTime.Nanoseconds())
		})
		throughputTrend := at.calculateTrend(history, func(m PerformanceMetrics) float64 {
			return m.BitswapMetrics.RequestsPerSecond
		})
		
		features = append(features, latencyTrend)
		features = append(features, throughputTrend)
	} else {
		features = append(features, 0, 0)
	}
	
	return features
}

// calculateTrend calculates the trend of a metric over time
func (at *autoTuner) calculateTrend(history []PerformanceMetrics, extractor func(PerformanceMetrics) float64) float64 {
	if len(history) < 2 {
		return 0
	}
	
	values := make([]float64, len(history))
	for i, metrics := range history {
		values[i] = extractor(metrics)
	}
	
	// Simple linear trend calculation
	n := len(values)
	sumX, sumY, sumXY, sumX2 := 0.0, 0.0, 0.0, 0.0
	
	for i, y := range values {
		x := float64(i)
		sumX += x
		sumY += y
		sumXY += x * y
		sumX2 += x * x
	}
	
	// Calculate slope
	slope := (float64(n)*sumXY - sumX*sumY) / (float64(n)*sumX2 - sumX*sumX)
	return slope
}

// calculatePredictionConfidence calculates confidence in predictions
func (at *autoTuner) calculatePredictionConfidence(features []float64) float64 {
	// Simplified confidence calculation
	// In practice, this would be based on model uncertainty
	
	// Base confidence
	confidence := 0.7
	
	// Adjust based on feature stability
	variance := at.calculateVariance(features)
	if variance < 0.1 {
		confidence += 0.2 // More confident with stable features
	} else if variance > 0.5 {
		confidence -= 0.2 // Less confident with volatile features
	}
	
	// Ensure confidence is in valid range
	if confidence > 1.0 {
		confidence = 1.0
	} else if confidence < 0.1 {
		confidence = 0.1
	}
	
	return confidence
}

// calculateVariance calculates the variance of features
func (at *autoTuner) calculateVariance(features []float64) float64 {
	if len(features) == 0 {
		return 0
	}
	
	mean := 0.0
	for _, f := range features {
		mean += f
	}
	mean /= float64(len(features))
	
	variance := 0.0
	for _, f := range features {
		diff := f - mean
		variance += diff * diff
	}
	variance /= float64(len(features))
	
	return math.Sqrt(variance)
}

// predictMaxOutstandingBytes predicts optimal MaxOutstandingBytesPerPeer
func (at *autoTuner) predictMaxOutstandingBytes(features []float64) int64 {
	// Simplified prediction logic
	// In practice, this would use the trained ML model
	
	requestsPerSecond := features[0]
	responseTime := features[1]
	
	// Base value
	baseValue := int64(5 << 20) // 5MB
	
	// Adjust based on load
	if requestsPerSecond > 1000 {
		baseValue *= 2 // Increase for high load
	}
	
	// Adjust based on latency
	if responseTime > float64(100*time.Millisecond.Nanoseconds()) {
		baseValue = int64(float64(baseValue) * 1.5) // Increase for high latency
	}
	
	// Cap at reasonable limits
	if baseValue > 500<<20 {
		baseValue = 500 << 20 // 500MB max
	}
	
	return baseValue
}

// predictWorkerCount predicts optimal worker count
func (at *autoTuner) predictWorkerCount(features []float64) int {
	// Simplified prediction logic
	requestsPerSecond := features[0]
	cpuUsage := features[12]
	
	// Base value
	baseValue := 256
	
	// Adjust based on load
	if requestsPerSecond > 1000 {
		baseValue = int(requestsPerSecond / 4) // Rough heuristic
	}
	
	// Adjust based on CPU usage
	if cpuUsage > 0.8 {
		baseValue = int(float64(baseValue) * 0.8) // Reduce if CPU is stressed
	}
	
	// Cap at reasonable limits
	if baseValue > 4096 {
		baseValue = 4096
	} else if baseValue < 128 {
		baseValue = 128
	}
	
	return baseValue
}

// ApplyConfigurationSafely applies configuration changes with safety checks
func (at *autoTuner) ApplyConfigurationSafely(ctx context.Context, newConfig *adaptive.AdaptiveConfig) (*ConfigurationChangeResult, error) {
	at.mu.Lock()
	defer at.mu.Unlock()
	
	currentMetrics := at.performanceMonitor.CollectMetrics()
	
	// Perform safety checks
	safetyResults := make([]SafetyCheckResult, 0, len(at.safetyChecks))
	allChecksPassed := true
	
	for _, check := range at.safetyChecks {
		result := check(at.currentConfig, newConfig, currentMetrics)
		safetyResults = append(safetyResults, result)
		if !result.Passed && result.Severity == SeverityCritical {
			allChecksPassed = false
		}
	}
	
	result := &ConfigurationChangeResult{
		Success:         false,
		AppliedChanges:  make(map[string]interface{}),
		RejectedChanges: make(map[string]string),
		SafetyChecks:    safetyResults,
		Timestamp:       time.Now(),
	}
	
	if !allChecksPassed {
		result.Success = false
		for _, check := range safetyResults {
			if !check.Passed {
				result.RejectedChanges[check.CheckName] = check.Message
			}
		}
		return result, fmt.Errorf("safety checks failed")
	}
	
	// Create rollback plan
	changeID := fmt.Sprintf("change-%d", time.Now().Unix())
	rollbackPlan := &RollbackPlan{
		PreviousConfig: at.currentConfig.Clone(),
		ChangeID:       changeID,
		Timestamp:      time.Now(),
		AutoRollback:   at.config.AutoTune.SafetyMode,
		RollbackDelay:  5 * time.Minute,
	}
	
	// Apply changes
	changes := at.calculateConfigChanges(at.currentConfig, newConfig)
	
	// Store previous config
	at.previousConfigs = append(at.previousConfigs, at.currentConfig.Clone())
	if len(at.previousConfigs) > at.maxConfigHistory {
		at.previousConfigs = at.previousConfigs[1:]
	}
	
	// Update current config
	if err := at.currentConfig.Update(newConfig); err != nil {
		return result, fmt.Errorf("failed to update configuration: %w", err)
	}
	
	// Store rollback plan
	at.rollbackPlans[changeID] = rollbackPlan
	
	result.Success = true
	result.AppliedChanges = changes
	result.RollbackPlan = rollbackPlan
	
	// Schedule automatic rollback if enabled
	if rollbackPlan.AutoRollback {
		go at.scheduleRollback(ctx, changeID, rollbackPlan.RollbackDelay)
	}
	
	return result, nil
}

// calculateConfigChanges calculates the differences between configurations
func (at *autoTuner) calculateConfigChanges(old, new *adaptive.AdaptiveConfig) map[string]interface{} {
	changes := make(map[string]interface{})
	
	// Compare Bitswap config
	if old.Bitswap != nil && new.Bitswap != nil {
		if old.Bitswap.MaxOutstandingBytesPerPeer != new.Bitswap.MaxOutstandingBytesPerPeer {
			changes["bitswap.max_outstanding_bytes_per_peer"] = map[string]interface{}{
				"old": old.Bitswap.MaxOutstandingBytesPerPeer,
				"new": new.Bitswap.MaxOutstandingBytesPerPeer,
			}
		}
		if old.Bitswap.MaxWorkerCount != new.Bitswap.MaxWorkerCount {
			changes["bitswap.max_worker_count"] = map[string]interface{}{
				"old": old.Bitswap.MaxWorkerCount,
				"new": new.Bitswap.MaxWorkerCount,
			}
		}
		if old.Bitswap.BatchSize != new.Bitswap.BatchSize {
			changes["bitswap.batch_size"] = map[string]interface{}{
				"old": old.Bitswap.BatchSize,
				"new": new.Bitswap.BatchSize,
			}
		}
	}
	
	// Compare Blockstore config
	if old.Blockstore != nil && new.Blockstore != nil {
		if old.Blockstore.MemoryCacheSize != new.Blockstore.MemoryCacheSize {
			changes["blockstore.memory_cache_size"] = map[string]interface{}{
				"old": old.Blockstore.MemoryCacheSize,
				"new": new.Blockstore.MemoryCacheSize,
			}
		}
		if old.Blockstore.BatchSize != new.Blockstore.BatchSize {
			changes["blockstore.batch_size"] = map[string]interface{}{
				"old": old.Blockstore.BatchSize,
				"new": new.Blockstore.BatchSize,
			}
		}
	}
	
	// Compare Network config
	if old.Network != nil && new.Network != nil {
		if old.Network.HighWater != new.Network.HighWater {
			changes["network.high_water"] = map[string]interface{}{
				"old": old.Network.HighWater,
				"new": new.Network.HighWater,
			}
		}
		if old.Network.MaxBufferSize != new.Network.MaxBufferSize {
			changes["network.max_buffer_size"] = map[string]interface{}{
				"old": old.Network.MaxBufferSize,
				"new": new.Network.MaxBufferSize,
			}
		}
	}
	
	return changes
}

// scheduleRollback schedules an automatic rollback after a delay
func (at *autoTuner) scheduleRollback(ctx context.Context, changeID string, delay time.Duration) {
	timer := time.NewTimer(delay)
	defer timer.Stop()
	
	select {
	case <-ctx.Done():
		return
	case <-timer.C:
		at.performRollback(changeID)
	}
}

// performRollback performs a rollback to a previous configuration
func (at *autoTuner) performRollback(changeID string) error {
	at.mu.Lock()
	defer at.mu.Unlock()
	
	rollbackPlan, exists := at.rollbackPlans[changeID]
	if !exists {
		return fmt.Errorf("rollback plan not found for change ID: %s", changeID)
	}
	
	// Check if rollback is still needed by comparing current performance
	currentMetrics := at.performanceMonitor.CollectMetrics()
	if at.isPerformanceAcceptable(currentMetrics) {
		// Performance is acceptable, cancel rollback
		delete(at.rollbackPlans, changeID)
		return nil
	}
	
	// Perform rollback
	if err := at.currentConfig.Update(rollbackPlan.PreviousConfig); err != nil {
		return fmt.Errorf("failed to rollback configuration: %w", err)
	}
	
	// Clean up rollback plan
	delete(at.rollbackPlans, changeID)
	
	return nil
}

// isPerformanceAcceptable checks if current performance is acceptable
func (at *autoTuner) isPerformanceAcceptable(metrics *PerformanceMetrics) bool {
	// Simple performance check
	if metrics.BitswapMetrics.P95ResponseTime > 200*time.Millisecond {
		return false
	}
	if metrics.BitswapMetrics.ErrorRate > 0.1 {
		return false
	}
	if metrics.ResourceMetrics.CPUUsage > 0.9 {
		return false
	}
	if float64(metrics.ResourceMetrics.MemoryUsage)/float64(metrics.ResourceMetrics.MemoryTotal) > 0.9 {
		return false
	}
	
	return true
}

// GetTuningHistory returns the history of tuning operations
func (at *autoTuner) GetTuningHistory() []TuningOperation {
	at.mu.RLock()
	defer at.mu.RUnlock()
	
	// Return a copy of the history
	history := make([]TuningOperation, len(at.history))
	copy(history, at.history)
	return history
}

// SetTuningStrategy configures the tuning strategy
func (at *autoTuner) SetTuningStrategy(strategy TuningStrategy) error {
	// Validate strategy
	if strategy.AggressivenessLevel < 0 || strategy.AggressivenessLevel > 1 {
		return fmt.Errorf("aggressiveness level must be between 0 and 1")
	}
	if strategy.MaxChangePercent < 0 || strategy.MaxChangePercent > 1 {
		return fmt.Errorf("max change percent must be between 0 and 1")
	}
	if strategy.LearningRate < 0 || strategy.LearningRate > 1 {
		return fmt.Errorf("learning rate must be between 0 and 1")
	}
	
	at.mu.Lock()
	defer at.mu.Unlock()
	
	at.strategy = strategy
	return nil
}

// TrainModel trains the ML model with historical data
func (at *autoTuner) TrainModel(ctx context.Context, trainingData []TrainingDataPoint) error {
	at.mu.Lock()
	defer at.mu.Unlock()
	
	if len(trainingData) < 10 {
		return fmt.Errorf("insufficient training data (need at least 10 samples, got %d)", len(trainingData))
	}
	
	// Store training data
	at.trainingData = append(at.trainingData, trainingData...)
	if len(at.trainingData) > at.maxTrainingData {
		at.trainingData = at.trainingData[len(at.trainingData)-at.maxTrainingData:]
	}
	
	// Train a simple model (in practice, this would use a real ML library)
	model := &MLModel{
		ModelType:    "linear_regression",
		Version:      "1.0",
		TrainedAt:    time.Now(),
		Accuracy:     0.75, // Simulated accuracy
		Parameters:   make(map[string]interface{}),
		FeatureNames: []string{"requests_per_second", "response_time", "cpu_usage", "memory_usage"},
	}
	
	// Simulate training process
	model.Parameters["weights"] = []float64{0.1, -0.2, -0.3, -0.1}
	model.Parameters["bias"] = 0.05
	
	at.model = model
	
	return nil
}

// SaveModel saves the trained model to disk
func (at *autoTuner) SaveModel(path string) error {
	at.mu.RLock()
	model := at.model
	at.mu.RUnlock()
	
	if model == nil {
		return fmt.Errorf("no model to save")
	}
	
	data, err := json.MarshalIndent(model, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal model: %w", err)
	}
	
	if err := os.WriteFile(path, data, 0644); err != nil {
		return fmt.Errorf("failed to write model file: %w", err)
	}
	
	return nil
}

// LoadModel loads a trained model from disk
func (at *autoTuner) LoadModel(path string) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("failed to read model file: %w", err)
	}
	
	var model MLModel
	if err := json.Unmarshal(data, &model); err != nil {
		return fmt.Errorf("failed to unmarshal model: %w", err)
	}
	
	at.mu.Lock()
	defer at.mu.Unlock()
	
	at.model = &model
	
	return nil
}

// Safety check implementations

// checkMemoryLimits checks if proposed configuration respects memory limits
func checkMemoryLimits(current, proposed *adaptive.AdaptiveConfig, metrics *PerformanceMetrics) SafetyCheckResult {
	result := SafetyCheckResult{
		CheckName: "memory_limits",
		Passed:    true,
		Severity:  SeverityMedium,
	}
	
	// Check if memory cache size increase is reasonable
	if proposed.Blockstore != nil && current.Blockstore != nil {
		currentCache := current.Blockstore.MemoryCacheSize
		proposedCache := proposed.Blockstore.MemoryCacheSize
		
		if proposedCache > currentCache*2 {
			result.Passed = false
			result.Severity = SeverityHigh
			result.Message = "Proposed memory cache size increase is too aggressive"
			result.Details = map[string]interface{}{
				"current":  currentCache,
				"proposed": proposedCache,
				"increase": float64(proposedCache-currentCache) / float64(currentCache),
			}
		}
		
		// Check against available memory
		if metrics != nil {
			availableMemory := metrics.ResourceMetrics.MemoryTotal - metrics.ResourceMetrics.MemoryUsage
			if proposedCache > availableMemory/2 {
				result.Passed = false
				result.Severity = SeverityCritical
				result.Message = "Proposed memory cache size exceeds available memory"
			}
		}
	}
	
	if result.Passed {
		result.Message = "Memory limits check passed"
	}
	
	return result
}

// checkCPULimits checks if proposed configuration respects CPU limits
func checkCPULimits(current, proposed *adaptive.AdaptiveConfig, metrics *PerformanceMetrics) SafetyCheckResult {
	result := SafetyCheckResult{
		CheckName: "cpu_limits",
		Passed:    true,
		Severity:  SeverityMedium,
	}
	
	// Check worker count increases
	if proposed.Bitswap != nil && current.Bitswap != nil {
		currentWorkers := current.Bitswap.MaxWorkerCount
		proposedWorkers := proposed.Bitswap.MaxWorkerCount
		
		if proposedWorkers > currentWorkers*2 {
			result.Passed = false
			result.Severity = SeverityHigh
			result.Message = "Proposed worker count increase is too aggressive"
			result.Details = map[string]interface{}{
				"current":  currentWorkers,
				"proposed": proposedWorkers,
				"increase": float64(proposedWorkers-currentWorkers) / float64(currentWorkers),
			}
		}
		
		// Check against current CPU usage
		if metrics != nil && metrics.ResourceMetrics.CPUUsage > 0.8 {
			if proposedWorkers > currentWorkers {
				result.Passed = false
				result.Severity = SeverityCritical
				result.Message = "Cannot increase worker count when CPU usage is already high"
			}
		}
	}
	
	if result.Passed {
		result.Message = "CPU limits check passed"
	}
	
	return result
}

// checkNetworkLimits checks if proposed configuration respects network limits
func checkNetworkLimits(current, proposed *adaptive.AdaptiveConfig, metrics *PerformanceMetrics) SafetyCheckResult {
	result := SafetyCheckResult{
		CheckName: "network_limits",
		Passed:    true,
		Severity:  SeverityMedium,
	}
	
	// Check connection limits
	if proposed.Network != nil && current.Network != nil {
		currentHigh := current.Network.HighWater
		proposedHigh := proposed.Network.HighWater
		
		if proposedHigh > currentHigh*3 {
			result.Passed = false
			result.Severity = SeverityHigh
			result.Message = "Proposed connection limit increase is too aggressive"
			result.Details = map[string]interface{}{
				"current":  currentHigh,
				"proposed": proposedHigh,
				"increase": float64(proposedHigh-currentHigh) / float64(currentHigh),
			}
		}
		
		// Check buffer size limits
		currentBuffer := current.Network.MaxBufferSize
		proposedBuffer := proposed.Network.MaxBufferSize
		
		if proposedBuffer > 50<<20 { // 50MB max
			result.Passed = false
			result.Severity = SeverityHigh
			result.Message = "Proposed buffer size exceeds reasonable limits"
			result.Details = map[string]interface{}{
				"current":  currentBuffer,
				"proposed": proposedBuffer,
			}
		}
	}
	
	if result.Passed {
		result.Message = "Network limits check passed"
	}
	
	return result
}

// checkConfigurationBounds checks if proposed values are within acceptable bounds
func checkConfigurationBounds(current, proposed *adaptive.AdaptiveConfig, metrics *PerformanceMetrics) SafetyCheckResult {
	result := SafetyCheckResult{
		CheckName: "configuration_bounds",
		Passed:    true,
		Severity:  SeverityHigh,
	}
	
	// Validate proposed configuration
	if err := proposed.Validate(); err != nil {
		result.Passed = false
		result.Severity = SeverityCritical
		result.Message = fmt.Sprintf("Proposed configuration is invalid: %v", err)
		return result
	}
	
	result.Message = "Configuration bounds check passed"
	return result
}

// checkPerformanceRegression checks if changes might cause performance regression
func checkPerformanceRegression(current, proposed *adaptive.AdaptiveConfig, metrics *PerformanceMetrics) SafetyCheckResult {
	result := SafetyCheckResult{
		CheckName: "performance_regression",
		Passed:    true,
		Severity:  SeverityMedium,
	}
	
	// Check if we're reducing critical parameters when performance is already poor
	if metrics != nil {
		if metrics.BitswapMetrics.P95ResponseTime > 200*time.Millisecond {
			// Performance is already poor, be careful about reductions
			if proposed.Bitswap != nil && current.Bitswap != nil {
				if proposed.Bitswap.MaxOutstandingBytesPerPeer < current.Bitswap.MaxOutstandingBytesPerPeer {
					result.Passed = false
					result.Severity = SeverityHigh
					result.Message = "Cannot reduce outstanding bytes when latency is already high"
				}
				if proposed.Bitswap.MaxWorkerCount < current.Bitswap.MaxWorkerCount {
					result.Passed = false
					result.Severity = SeverityHigh
					result.Message = "Cannot reduce worker count when latency is already high"
				}
			}
		}
	}
	
	if result.Passed {
		result.Message = "Performance regression check passed"
	}
	
	return result
}