package adaptive

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDefaultResourceManager(t *testing.T) {
	config := &ResourceConfig{
		MaxMemoryUsage:      8 << 30, // 8GB
		MemoryPressureLevel: 0.8,     // 80%
		MaxCPUUsage:         0.8,     // 80%
		MaxDiskUsage:        0.9,     // 90%
		DegradationEnabled:  true,
		DegradationSteps: []DegradationStep{
			{Threshold: 0.7, Action: "reduce_batch_size", Severity: "low"},
			{Threshold: 0.8, Action: "reduce_workers", Severity: "medium"},
			{Threshold: 0.9, Action: "disable_compression", Severity: "high"},
		},
	}
	
	rm := NewDefaultResourceManager(config)
	assert.NotNil(t, rm)
	assert.Equal(t, DegradationNone, rm.GetDegradationLevel())
	
	ctx := context.Background()
	
	// Test GetResourceMetrics
	metrics, err := rm.GetResourceMetrics(ctx)
	require.NoError(t, err)
	assert.NotNil(t, metrics)
	assert.NotZero(t, metrics.Timestamp)
	assert.Greater(t, metrics.CPUCores, 0)
	
	// Test ApplyResourceConstraints
	constraints := &ResourceConstraints{
		MaxMemoryUsage: 4 << 30, // 4GB
		MaxCPUUsage:    0.7,     // 70%
		MaxConnections: 1000,
	}
	
	err = rm.ApplyResourceConstraints(ctx, constraints)
	assert.NoError(t, err)
	
	// Test ShouldDegrade (should not degrade with normal metrics)
	shouldDegrade, recommendation, err := rm.ShouldDegrade(ctx)
	require.NoError(t, err)
	assert.False(t, shouldDegrade)
	assert.Nil(t, recommendation)
	
	// Test ApplyDegradation
	step := &DegradationStep{
		Threshold: 0.8,
		Action:    "reduce_workers",
		Severity:  "medium",
	}
	
	err = rm.ApplyDegradation(ctx, step)
	assert.NoError(t, err)
	assert.Equal(t, DegradationMedium, rm.GetDegradationLevel())
	
	// Test RecoverFromDegradation
	err = rm.RecoverFromDegradation(ctx)
	assert.NoError(t, err)
	// Note: Recovery depends on current resource usage, so level might not change
}

func TestDefaultResourceManagerSeverityMapping(t *testing.T) {
	rm := &DefaultResourceManager{}
	
	tests := []struct {
		severity string
		expected DegradationLevel
	}{
		{"low", DegradationLow},
		{"medium", DegradationMedium},
		{"high", DegradationHigh},
		{"critical", DegradationCritical},
		{"unknown", DegradationNone},
		{"", DegradationNone},
	}
	
	for _, tt := range tests {
		t.Run(tt.severity, func(t *testing.T) {
			level := rm.severityToDegradationLevel(tt.severity)
			assert.Equal(t, tt.expected, level)
		})
	}
}

func TestDefaultPerformanceMonitor(t *testing.T) {
	config := &MonitoringConfig{
		MetricsEnabled:    true,
		MetricsInterval:   100 * time.Millisecond,
		PrometheusEnabled: true,
		PrometheusPort:    9090,
		AlertThresholds: AlertConfig{
			ResponseTimeP95: 100 * time.Millisecond,
			ErrorRate:       0.05,
			MemoryUsage:     0.8,
		},
		AlertingEnabled: true,
		LogLevel:        "info",
	}
	
	pm := NewDefaultPerformanceMonitor(config)
	assert.NotNil(t, pm)
	
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	
	// Test Start
	err := pm.Start(ctx)
	assert.NoError(t, err)
	
	// Test GetMetrics
	metrics, err := pm.GetMetrics(ctx)
	require.NoError(t, err)
	assert.NotNil(t, metrics)
	assert.NotZero(t, metrics.Timestamp)
	assert.NotNil(t, metrics.Bitswap)
	assert.NotNil(t, metrics.Blockstore)
	assert.NotNil(t, metrics.Network)
	assert.NotNil(t, metrics.System)
	
	// Verify metric values are reasonable
	assert.Greater(t, metrics.Bitswap.RequestsPerSecond, 0.0)
	assert.Greater(t, metrics.Blockstore.CacheHitRate, 0.0)
	assert.Greater(t, metrics.Network.ActiveConnections, 0)
	assert.Greater(t, metrics.System.RequestsPerSecond, 0.0)
	
	// Test GetHistoricalMetrics
	from := time.Now().Add(-1 * time.Hour)
	to := time.Now()
	historical, err := pm.GetHistoricalMetrics(ctx, from, to)
	assert.NoError(t, err)
	assert.NotNil(t, historical)
	// Historical metrics are empty in this implementation
	assert.Len(t, historical, 0)
	
	// Test AnalyzeBottlenecks
	analysis, err := pm.AnalyzeBottlenecks(ctx)
	require.NoError(t, err)
	assert.NotNil(t, analysis)
	assert.NotZero(t, analysis.Timestamp)
	assert.NotEmpty(t, analysis.Overall)
	
	// Test SubscribeToAlerts
	alertCh, err := pm.SubscribeToAlerts(ctx)
	assert.NoError(t, err)
	assert.NotNil(t, alertCh)
	
	// Wait for metrics collection to run
	time.Sleep(200 * time.Millisecond)
	
	// Test Stop
	err = pm.Stop(ctx)
	assert.NoError(t, err)
}

func TestDefaultPerformanceMonitorBottleneckAnalysis(t *testing.T) {
	config := &MonitoringConfig{
		MetricsEnabled:  true,
		MetricsInterval: 1 * time.Second,
		AlertThresholds: AlertConfig{
			ResponseTimeP95: 50 * time.Millisecond, // Lower threshold for testing
			ErrorRate:       0.05,
		},
	}
	
	pm := NewDefaultPerformanceMonitor(config)
	
	ctx := context.Background()
	
	// Start monitor
	err := pm.Start(ctx)
	require.NoError(t, err)
	defer pm.Stop(ctx)
	
	// Get metrics (which will have P95 response time > threshold)
	_, err = pm.GetMetrics(ctx)
	require.NoError(t, err)
	
	// Analyze bottlenecks
	analysis, err := pm.AnalyzeBottlenecks(ctx)
	require.NoError(t, err)
	
	// Should detect bottlenecks due to high response time (cache hit rate is 0.85 > 0.8 threshold)
	assert.Greater(t, len(analysis.Bottlenecks), 0)
	
	// Check for specific bottlenecks
	foundBitswapBottleneck := false
	
	for _, bottleneck := range analysis.Bottlenecks {
		if bottleneck.Component == "bitswap" && bottleneck.Metric == "p95_response_time" {
			foundBitswapBottleneck = true
		}
	}
	
	assert.True(t, foundBitswapBottleneck, "Should detect Bitswap response time bottleneck")
	
	// Overall health should be degraded due to bottlenecks
	assert.Equal(t, "degraded", analysis.Overall)
}

func TestDefaultAutoTuner(t *testing.T) {
	config := &AutoTuneConfig{
		Enabled:          true,
		TuningInterval:   1 * time.Minute,
		LearningEnabled:  false,
		SafetyMode:       true,
		MaxChangePercent: 0.1,
		TrainingEnabled:  false,
	}
	
	at := NewDefaultAutoTuner(config)
	assert.NotNil(t, at)
	
	ctx := context.Background()
	
	// Test initial status
	status := at.GetTuningStatus()
	assert.NotNil(t, status)
	assert.True(t, status.Enabled)
	assert.True(t, status.SafetyMode)
	assert.False(t, status.MLMode)
	
	// Test Start
	err := at.Start(ctx)
	assert.NoError(t, err)
	
	// Test GetTuningRecommendations
	metrics := &PerformanceMetrics{
		Bitswap: &BitswapMetrics{
			WorkerUtilization: 0.95, // High utilization should trigger recommendation
		},
	}
	
	recommendations, err := at.GetTuningRecommendations(ctx, metrics)
	require.NoError(t, err)
	assert.NotNil(t, recommendations)
	
	// Should have recommendations for high worker utilization
	assert.Greater(t, len(recommendations.Recommendations), 0)
	
	rec := recommendations.Recommendations[0]
	assert.Equal(t, "bitswap", rec.Component)
	assert.Equal(t, "max_worker_count", rec.Parameter)
	assert.Equal(t, "low", rec.RiskLevel)
	
	// Test ApplyTuning
	err = at.ApplyTuning(ctx, recommendations)
	assert.NoError(t, err)
	
	// Check status after tuning
	status = at.GetTuningStatus()
	assert.Equal(t, int64(1), status.TuningCycles)
	assert.Equal(t, int64(1), status.SuccessfulTunes)
	assert.Equal(t, int64(0), status.FailedTunes)
	
	// Test SetMLMode
	err = at.SetMLMode(true)
	assert.NoError(t, err)
	
	status = at.GetTuningStatus()
	assert.True(t, status.MLMode)
	
	// Test Stop
	err = at.Stop(ctx)
	assert.NoError(t, err)
	
	status = at.GetTuningStatus()
	assert.False(t, status.Enabled)
}

func TestDefaultAutoTunerNoRecommendations(t *testing.T) {
	config := &AutoTuneConfig{
		Enabled:          true,
		TuningInterval:   1 * time.Minute,
		SafetyMode:       true,
		MaxChangePercent: 0.1,
	}
	
	at := NewDefaultAutoTuner(config)
	ctx := context.Background()
	
	// Test with metrics that don't trigger recommendations
	metrics := &PerformanceMetrics{
		Bitswap: &BitswapMetrics{
			WorkerUtilization: 0.5, // Normal utilization
		},
	}
	
	recommendations, err := at.GetTuningRecommendations(ctx, metrics)
	require.NoError(t, err)
	assert.NotNil(t, recommendations)
	
	// Should have no recommendations
	assert.Len(t, recommendations.Recommendations, 0)
	assert.Equal(t, 0.0, recommendations.Confidence)
}

func TestDefaultPerformanceMonitorMetricsCollection(t *testing.T) {
	config := &MonitoringConfig{
		MetricsEnabled:  true,
		MetricsInterval: 50 * time.Millisecond, // Very short for testing
		AlertThresholds: AlertConfig{
			ResponseTimeP95: 100 * time.Millisecond,
		},
	}
	
	pm := NewDefaultPerformanceMonitor(config)
	
	ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
	defer cancel()
	
	// Start monitoring
	err := pm.Start(ctx)
	require.NoError(t, err)
	
	// Wait for several collection cycles
	time.Sleep(200 * time.Millisecond)
	
	// Get metrics
	metrics, err := pm.GetMetrics(ctx)
	require.NoError(t, err)
	assert.NotNil(t, metrics)
	
	// Stop monitoring
	err = pm.Stop(ctx)
	assert.NoError(t, err)
	
	// Verify metrics were collected
	assert.NotZero(t, metrics.Timestamp)
	assert.NotNil(t, metrics.Bitswap)
	assert.NotNil(t, metrics.Blockstore)
	assert.NotNil(t, metrics.Network)
	assert.NotNil(t, metrics.System)
}

func TestDefaultResourceManagerDegradationSteps(t *testing.T) {
	config := &ResourceConfig{
		DegradationEnabled: true,
		DegradationSteps: []DegradationStep{
			{Threshold: 0.7, Action: "reduce_batch_size", Severity: "low"},
			{Threshold: 0.8, Action: "reduce_workers", Severity: "medium"},
			{Threshold: 0.9, Action: "disable_compression", Severity: "high"},
			{Threshold: 0.95, Action: "emergency_mode", Severity: "critical"},
		},
	}
	
	rm := NewDefaultResourceManager(config)
	ctx := context.Background()
	
	// Apply degradation steps in order
	steps := []struct {
		step     DegradationStep
		expected DegradationLevel
	}{
		{config.DegradationSteps[0], DegradationLow},
		{config.DegradationSteps[1], DegradationMedium},
		{config.DegradationSteps[2], DegradationHigh},
		{config.DegradationSteps[3], DegradationCritical},
	}
	
	for _, test := range steps {
		err := rm.ApplyDegradation(ctx, &test.step)
		assert.NoError(t, err)
		assert.Equal(t, test.expected, rm.GetDegradationLevel())
	}
	
	// Verify all steps are marked as applied
	assert.Len(t, rm.appliedSteps, 4)
	assert.True(t, rm.appliedSteps["reduce_batch_size"])
	assert.True(t, rm.appliedSteps["reduce_workers"])
	assert.True(t, rm.appliedSteps["disable_compression"])
	assert.True(t, rm.appliedSteps["emergency_mode"])
}