package adaptive

import (
	"context"
	"fmt"
	"sync"
	"time"
)

// Manager implements the core adaptive configuration management
type Manager struct {
	mu     sync.RWMutex
	config *AdaptiveConfig
	
	// Component managers
	resourceManager ResourceManager
	autoTuner       AutoTuner
	perfMonitor     PerformanceMonitor
	
	// Event channels
	configChanges chan *ConfigChangeEvent
	alerts        chan *PerformanceAlert
	
	// State
	running         bool
	degradationLevel DegradationLevel
	lastTuning      time.Time
	
	// Subscribers
	subscribers []chan *ConfigChangeEvent
	subMu       sync.RWMutex
}

// NewManager creates a new adaptive configuration manager
func NewManager(config *AdaptiveConfig) (*Manager, error) {
	if config == nil {
		config = DefaultConfig()
	}
	
	if err := config.Validate(); err != nil {
		return nil, fmt.Errorf("invalid configuration: %w", err)
	}
	
	return &Manager{
		config:           config,
		configChanges:    make(chan *ConfigChangeEvent, 100),
		alerts:           make(chan *PerformanceAlert, 100),
		degradationLevel: DegradationNone,
		subscribers:      make([]chan *ConfigChangeEvent, 0),
	}, nil
}

// Start initializes and starts the adaptive configuration manager
func (m *Manager) Start(ctx context.Context) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	
	if m.running {
		return fmt.Errorf("manager already running")
	}
	
	// Initialize components if they're not set
	if m.resourceManager == nil {
		m.resourceManager = NewDefaultResourceManager(m.config.Resources)
	}
	
	if m.perfMonitor == nil {
		m.perfMonitor = NewDefaultPerformanceMonitor(m.config.Monitoring)
	}
	
	if m.autoTuner == nil && m.config.AutoTune.Enabled {
		m.autoTuner = NewDefaultAutoTuner(m.config.AutoTune)
	}
	
	// Start performance monitoring
	if err := m.perfMonitor.Start(ctx); err != nil {
		return fmt.Errorf("failed to start performance monitor: %w", err)
	}
	
	// Start auto-tuner if enabled
	if m.autoTuner != nil {
		if err := m.autoTuner.Start(ctx); err != nil {
			return fmt.Errorf("failed to start auto-tuner: %w", err)
		}
	}
	
	m.running = true
	
	// Start background monitoring goroutine
	go m.monitoringLoop(ctx)
	
	return nil
}

// Stop gracefully stops the adaptive configuration manager
func (m *Manager) Stop(ctx context.Context) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	
	if !m.running {
		return nil
	}
	
	m.running = false
	
	// Stop components
	if m.perfMonitor != nil {
		if err := m.perfMonitor.Stop(ctx); err != nil {
			return fmt.Errorf("failed to stop performance monitor: %w", err)
		}
	}
	
	if m.autoTuner != nil {
		if err := m.autoTuner.Stop(ctx); err != nil {
			return fmt.Errorf("failed to stop auto-tuner: %w", err)
		}
	}
	
	// Close channels
	close(m.configChanges)
	close(m.alerts)
	
	// Close subscriber channels
	m.subMu.Lock()
	for _, ch := range m.subscribers {
		close(ch)
	}
	m.subscribers = nil
	m.subMu.Unlock()
	
	return nil
}

// GetConfig returns the current configuration
func (m *Manager) GetConfig() *AdaptiveConfig {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.config.Clone()
}

// UpdateConfig updates the configuration with validation
func (m *Manager) UpdateConfig(ctx context.Context, newConfig *AdaptiveConfig) error {
	if err := newConfig.Validate(); err != nil {
		event := &ConfigChangeEvent{
			Type:      "validate",
			Timestamp: time.Now(),
			Success:   false,
			Error:     err.Error(),
		}
		m.notifySubscribers(event)
		return fmt.Errorf("configuration validation failed: %w", err)
	}
	
	m.mu.Lock()
	oldConfig := m.config.Clone()
	m.config = newConfig.Clone()
	m.mu.Unlock()
	
	// Apply configuration changes
	if err := m.applyConfigurationChanges(ctx, oldConfig, newConfig); err != nil {
		// Rollback on failure
		m.mu.Lock()
		m.config = oldConfig
		m.mu.Unlock()
		
		event := &ConfigChangeEvent{
			Type:      "apply",
			Timestamp: time.Now(),
			Success:   false,
			Error:     err.Error(),
		}
		m.notifySubscribers(event)
		return fmt.Errorf("failed to apply configuration changes: %w", err)
	}
	
	// Notify subscribers of successful update
	event := &ConfigChangeEvent{
		Type:      "update",
		Source:    "manual",
		Timestamp: time.Now(),
		Success:   true,
	}
	m.notifySubscribers(event)
	
	return nil
}

// ApplyConfigChanges applies specific configuration changes
func (m *Manager) ApplyConfigChanges(ctx context.Context, changes *ConfigChanges) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	
	// Create a new config with the changes applied
	newConfig := m.config.Clone()
	
	// Apply changes (simplified implementation)
	// In a real implementation, this would use reflection or a more sophisticated approach
	for key, value := range changes.Changes {
		if err := m.applyConfigChange(newConfig, key, value); err != nil {
			return fmt.Errorf("failed to apply change %s: %w", key, err)
		}
	}
	
	// Validate the new configuration
	if err := newConfig.Validate(); err != nil {
		return fmt.Errorf("configuration validation failed after changes: %w", err)
	}
	
	// Update the configuration
	oldConfig := m.config
	m.config = newConfig
	
	// Apply the changes
	if err := m.applyConfigurationChanges(ctx, oldConfig, newConfig); err != nil {
		// Rollback on failure
		m.config = oldConfig
		return fmt.Errorf("failed to apply configuration changes: %w", err)
	}
	
	// Notify subscribers
	event := &ConfigChangeEvent{
		Type:      "apply",
		Changes:   changes.Changes,
		Source:    changes.Source,
		Timestamp: changes.Timestamp,
		Success:   true,
	}
	m.notifySubscribers(event)
	
	return nil
}

// ValidateConfig validates a configuration without applying it
func (m *Manager) ValidateConfig(config *AdaptiveConfig) error {
	return config.Validate()
}

// GetConfigRecommendations returns configuration recommendations based on metrics
func (m *Manager) GetConfigRecommendations(ctx context.Context, metrics *PerformanceMetrics) (*ConfigRecommendations, error) {
	recommendations := &ConfigRecommendations{
		Recommendations: make([]ConfigRecommendation, 0),
		Timestamp:       time.Now(),
	}
	
	// Analyze Bitswap performance
	if metrics.Bitswap != nil {
		if recs := m.analyzeBitswapPerformance(metrics.Bitswap); len(recs) > 0 {
			recommendations.Recommendations = append(recommendations.Recommendations, recs...)
		}
	}
	
	// Analyze Blockstore performance
	if metrics.Blockstore != nil {
		if recs := m.analyzeBlockstorePerformance(metrics.Blockstore); len(recs) > 0 {
			recommendations.Recommendations = append(recommendations.Recommendations, recs...)
		}
	}
	
	// Analyze Network performance
	if metrics.Network != nil {
		if recs := m.analyzeNetworkPerformance(metrics.Network); len(recs) > 0 {
			recommendations.Recommendations = append(recommendations.Recommendations, recs...)
		}
	}
	
	// Calculate overall confidence based on data quality and consistency
	recommendations.Confidence = m.calculateRecommendationConfidence(metrics)
	
	return recommendations, nil
}

// Subscribe to configuration changes
func (m *Manager) Subscribe(ctx context.Context) (<-chan *ConfigChangeEvent, error) {
	m.subMu.Lock()
	defer m.subMu.Unlock()
	
	ch := make(chan *ConfigChangeEvent, 10)
	m.subscribers = append(m.subscribers, ch)
	
	return ch, nil
}

// GetResourceMetrics returns current resource usage metrics
func (m *Manager) GetResourceMetrics(ctx context.Context) (*ResourceMetrics, error) {
	if m.resourceManager == nil {
		return nil, fmt.Errorf("resource manager not initialized")
	}
	return m.resourceManager.GetResourceMetrics(ctx)
}

// GetPerformanceMetrics returns current performance metrics
func (m *Manager) GetPerformanceMetrics(ctx context.Context) (*PerformanceMetrics, error) {
	if m.perfMonitor == nil {
		return nil, fmt.Errorf("performance monitor not initialized")
	}
	return m.perfMonitor.GetMetrics(ctx)
}

// GetDegradationLevel returns the current degradation level
func (m *Manager) GetDegradationLevel() DegradationLevel {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.degradationLevel
}

// Private methods

func (m *Manager) monitoringLoop(ctx context.Context) {
	ticker := time.NewTicker(m.config.Monitoring.MetricsInterval)
	defer ticker.Stop()
	
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			if !m.running {
				return
			}
			
			// Check for degradation needs
			if err := m.checkDegradation(ctx); err != nil {
				// Log error but continue monitoring
				continue
			}
			
			// Run auto-tuning if enabled and due
			if m.autoTuner != nil && m.shouldRunAutoTuning() {
				if err := m.runAutoTuning(ctx); err != nil {
					// Log error but continue monitoring
					continue
				}
			}
		}
	}
}

func (m *Manager) checkDegradation(ctx context.Context) error {
	if m.resourceManager == nil {
		return nil
	}
	
	shouldDegrade, recommendation, err := m.resourceManager.ShouldDegrade(ctx)
	if err != nil {
		return err
	}
	
	if shouldDegrade && recommendation != nil {
		// Apply degradation if needed
		for _, step := range recommendation.Steps {
			if err := m.resourceManager.ApplyDegradation(ctx, &step); err != nil {
				return err
			}
		}
		
		m.mu.Lock()
		m.degradationLevel = recommendation.Level
		m.mu.Unlock()
		
		// Send alert
		alert := &PerformanceAlert{
			ID:        fmt.Sprintf("degradation-%d", time.Now().Unix()),
			Timestamp: time.Now(),
			Component: "resource_manager",
			Severity:  recommendation.Urgency,
			Message:   fmt.Sprintf("Applied degradation level %s: %s", recommendation.Level, recommendation.Reason),
		}
		
		select {
		case m.alerts <- alert:
		default:
			// Alert channel full, skip
		}
	}
	
	return nil
}

func (m *Manager) shouldRunAutoTuning() bool {
	if m.config.AutoTune == nil || !m.config.AutoTune.Enabled {
		return false
	}
	
	return time.Since(m.lastTuning) >= m.config.AutoTune.TuningInterval
}

func (m *Manager) runAutoTuning(ctx context.Context) error {
	if m.autoTuner == nil || m.perfMonitor == nil {
		return nil
	}
	
	// Get current performance metrics
	metrics, err := m.perfMonitor.GetMetrics(ctx)
	if err != nil {
		return err
	}
	
	// Get tuning recommendations
	recommendations, err := m.autoTuner.GetTuningRecommendations(ctx, metrics)
	if err != nil {
		return err
	}
	
	// Apply tuning if recommendations exist and are safe
	if len(recommendations.Recommendations) > 0 && recommendations.SafetyScore > 0.7 {
		if err := m.autoTuner.ApplyTuning(ctx, recommendations); err != nil {
			return err
		}
		
		m.lastTuning = time.Now()
		
		// Notify subscribers of auto-tuning changes
		event := &ConfigChangeEvent{
			Type:      "auto_tune",
			Source:    "auto_tuner",
			Timestamp: time.Now(),
			Success:   true,
		}
		m.notifySubscribers(event)
	}
	
	return nil
}

func (m *Manager) applyConfigurationChanges(ctx context.Context, oldConfig, newConfig *AdaptiveConfig) error {
	// In a real implementation, this would notify all registered components
	// about configuration changes and allow them to reconfigure themselves
	
	// For now, we'll just update the internal components
	if m.resourceManager != nil && configChanged(oldConfig.Resources, newConfig.Resources) {
		// Update resource manager configuration
		// This would be component-specific
	}
	
	if m.autoTuner != nil && configChanged(oldConfig.AutoTune, newConfig.AutoTune) {
		// Update auto-tuner configuration
		// This would be component-specific
	}
	
	return nil
}

func (m *Manager) applyConfigChange(config *AdaptiveConfig, key string, value interface{}) error {
	// Simplified implementation - in reality this would use reflection
	// or a more sophisticated configuration system
	switch key {
	case "bitswap.batch_size":
		if v, ok := value.(int); ok {
			config.Bitswap.BatchSize = v
		}
	case "blockstore.memory_cache_size":
		if v, ok := value.(int64); ok {
			config.Blockstore.MemoryCacheSize = v
		}
	// Add more cases as needed
	default:
		return fmt.Errorf("unknown configuration key: %s", key)
	}
	
	return nil
}

func (m *Manager) notifySubscribers(event *ConfigChangeEvent) {
	m.subMu.RLock()
	defer m.subMu.RUnlock()
	
	for _, ch := range m.subscribers {
		select {
		case ch <- event:
		default:
			// Channel full, skip this subscriber
		}
	}
}

func (m *Manager) analyzeBitswapPerformance(metrics *BitswapMetrics) []ConfigRecommendation {
	var recommendations []ConfigRecommendation
	
	// Check response time
	if metrics.P95ResponseTime > m.config.Monitoring.AlertThresholds.ResponseTimeP95 {
		recommendations = append(recommendations, ConfigRecommendation{
			Component:        "bitswap",
			Parameter:        "batch_size",
			CurrentValue:     m.config.Bitswap.BatchSize,
			RecommendedValue: m.config.Bitswap.BatchSize * 2, // Increase batch size
			Reason:           "High P95 response time detected",
			Priority:         "medium",
			Impact:           "Should reduce response time by batching more requests",
		})
	}
	
	// Check worker utilization
	if metrics.WorkerUtilization > 0.9 {
		recommendations = append(recommendations, ConfigRecommendation{
			Component:        "bitswap",
			Parameter:        "max_worker_count",
			CurrentValue:     m.config.Bitswap.MaxWorkerCount,
			RecommendedValue: int(float64(m.config.Bitswap.MaxWorkerCount) * 1.2), // Increase by 20%
			Reason:           "High worker utilization detected",
			Priority:         "high",
			Impact:           "Should improve throughput by adding more workers",
		})
	}
	
	return recommendations
}

func (m *Manager) analyzeBlockstorePerformance(metrics *BlockstoreMetrics) []ConfigRecommendation {
	var recommendations []ConfigRecommendation
	
	// Check cache hit rate
	if metrics.CacheHitRate < 0.8 {
		recommendations = append(recommendations, ConfigRecommendation{
			Component:        "blockstore",
			Parameter:        "memory_cache_size",
			CurrentValue:     m.config.Blockstore.MemoryCacheSize,
			RecommendedValue: int64(float64(m.config.Blockstore.MemoryCacheSize) * 1.5), // Increase by 50%
			Reason:           "Low cache hit rate detected",
			Priority:         "medium",
			Impact:           "Should improve read performance by caching more blocks",
		})
	}
	
	return recommendations
}

func (m *Manager) analyzeNetworkPerformance(metrics *NetworkMetrics) []ConfigRecommendation {
	var recommendations []ConfigRecommendation
	
	// Check connection count vs limits
	currentConfig := m.GetConfig()
	if float64(metrics.ActiveConnections) > float64(currentConfig.Network.HighWater)*0.9 {
		recommendations = append(recommendations, ConfigRecommendation{
			Component:        "network",
			Parameter:        "high_water",
			CurrentValue:     currentConfig.Network.HighWater,
			RecommendedValue: int(float64(currentConfig.Network.HighWater) * 1.2), // Increase by 20%
			Reason:           "Approaching connection limit",
			Priority:         "high",
			Impact:           "Should prevent connection throttling",
		})
	}
	
	return recommendations
}

func (m *Manager) calculateRecommendationConfidence(metrics *PerformanceMetrics) float64 {
	// Simple confidence calculation based on data availability
	confidence := 0.0
	factors := 0
	
	if metrics.Bitswap != nil {
		confidence += 0.25
		factors++
	}
	if metrics.Blockstore != nil {
		confidence += 0.25
		factors++
	}
	if metrics.Network != nil {
		confidence += 0.25
		factors++
	}
	if metrics.Resources != nil {
		confidence += 0.25
		factors++
	}
	
	if factors == 0 {
		return 0.0
	}
	
	return confidence
}

func configChanged(old, new interface{}) bool {
	// Simple comparison - in reality this would be more sophisticated
	return old != new
}