package priority

import (
	"context"
	"sync"
	"time"

	"github.com/ipfs/boxo/bitswap/adaptive"
	logging "github.com/ipfs/go-log/v2"
)

var integrationLog = logging.Logger("bitswap/priority/integration")

// AdaptivePriorityManager integrates the priority request manager with adaptive configuration
type AdaptivePriorityManager struct {
	*PriorityRequestManager
	
	adaptiveConfig *adaptive.AdaptiveBitswapConfig
	
	mu                sync.RWMutex
	lastConfigUpdate  time.Time
	updateInterval    time.Duration
	
	// Metrics for adaptation
	metricsCollector *PriorityMetricsCollector
	
	// Context for background operations
	ctx    context.Context
	cancel context.CancelFunc
	wg     sync.WaitGroup
}

// PriorityMetricsCollector collects metrics for adaptive priority management
type PriorityMetricsCollector struct {
	mu sync.RWMutex
	
	// Request metrics
	requestsPerSecond     float64
	averageResponseTime   time.Duration
	p95ResponseTime       time.Duration
	outstandingRequests   int64
	activeConnections     int
	
	// Priority-specific metrics
	priorityDistribution  map[Priority]float64
	priorityWaitTimes     map[Priority]time.Duration
	resourceUtilization   float64
	
	// Time tracking
	lastMetricsUpdate     time.Time
	metricsWindow         time.Duration
	
	// Counters for rate calculation
	requestCounter        int64
	lastRequestCount      int64
	lastCounterReset      time.Time
}

// NewAdaptivePriorityManager creates a new adaptive priority manager
func NewAdaptivePriorityManager(
	adaptiveConfig *adaptive.AdaptiveBitswapConfig,
	updateInterval time.Duration,
) *AdaptivePriorityManager {
	// Get current configuration from adaptive config
	highThreshold, criticalThreshold := adaptiveConfig.GetPriorityThresholds()
	workerCount := adaptiveConfig.GetCurrentWorkerCount()
	maxBytes := adaptiveConfig.GetMaxOutstandingBytesPerPeer()
	
	// Create priority request manager with adaptive parameters
	prm := NewPriorityRequestManager(
		workerCount,
		maxBytes, // Use as bandwidth limit approximation
		highThreshold,
		criticalThreshold,
	)
	
	ctx, cancel := context.WithCancel(context.Background())
	
	apm := &AdaptivePriorityManager{
		PriorityRequestManager: prm,
		adaptiveConfig:         adaptiveConfig,
		updateInterval:         updateInterval,
		ctx:                    ctx,
		cancel:                 cancel,
		metricsCollector: &PriorityMetricsCollector{
			priorityDistribution: make(map[Priority]float64),
			priorityWaitTimes:    make(map[Priority]time.Duration),
			metricsWindow:        time.Minute,
			lastCounterReset:     time.Now(),
		},
	}
	
	// Set up callback to collect metrics
	prm.SetRequestProcessedCallback(apm.onRequestProcessed)
	
	return apm
}

// Start begins the adaptive priority manager
func (apm *AdaptivePriorityManager) Start() {
	apm.PriorityRequestManager.Start(apm.ctx)
	
	// Start background adaptation loop
	apm.wg.Add(1)
	go apm.adaptationLoop()
	
	integrationLog.Info("Adaptive priority manager started")
}

// Stop gracefully shuts down the adaptive priority manager
func (apm *AdaptivePriorityManager) Stop() {
	apm.cancel()
	apm.wg.Wait()
	apm.PriorityRequestManager.Stop()
	
	integrationLog.Info("Adaptive priority manager stopped")
}

// adaptationLoop runs the background adaptation process
func (apm *AdaptivePriorityManager) adaptationLoop() {
	defer apm.wg.Done()
	
	ticker := time.NewTicker(apm.updateInterval)
	defer ticker.Stop()
	
	for {
		select {
		case <-apm.ctx.Done():
			return
		case <-ticker.C:
			apm.adaptConfiguration()
		}
	}
}

// adaptConfiguration updates priority manager configuration based on adaptive config
func (apm *AdaptivePriorityManager) adaptConfiguration() {
	apm.mu.Lock()
	defer apm.mu.Unlock()
	
	// Update metrics in adaptive config
	metrics := apm.metricsCollector.GetCurrentMetrics()
	apm.adaptiveConfig.UpdateMetrics(
		metrics.RequestsPerSecond,
		metrics.AverageResponseTime,
		metrics.P95ResponseTime,
		metrics.OutstandingRequests,
		metrics.ActiveConnections,
	)
	
	// Trigger adaptation in the adaptive config
	adapted := apm.adaptiveConfig.AdaptConfiguration(apm.ctx)
	
	if adapted {
		// Get updated configuration
		highThreshold, criticalThreshold := apm.adaptiveConfig.GetPriorityThresholds()
		workerCount := apm.adaptiveConfig.GetCurrentWorkerCount()
		maxBytes := apm.adaptiveConfig.GetMaxOutstandingBytesPerPeer()
		
		// Update priority manager configuration
		apm.PriorityRequestManager.UpdateConfiguration(
			workerCount,
			maxBytes,
			highThreshold,
			criticalThreshold,
		)
		
		apm.lastConfigUpdate = time.Now()
		
		integrationLog.Infof("Adapted priority configuration: workers=%d, maxBytes=%d, highThreshold=%v, criticalThreshold=%v",
			workerCount, maxBytes, highThreshold, criticalThreshold)
	}
}

// onRequestProcessed is called when a request is processed to collect metrics
func (apm *AdaptivePriorityManager) onRequestProcessed(request *PriorityRequest, result *RequestResult) {
	apm.metricsCollector.RecordRequest(request, result)
}

// GetAdaptiveMetrics returns current adaptive metrics
func (apm *AdaptivePriorityManager) GetAdaptiveMetrics() AdaptiveMetrics {
	apm.mu.RLock()
	defer apm.mu.RUnlock()
	
	priorityStats := apm.GetStats()
	configSnapshot := apm.adaptiveConfig.GetSnapshot()
	metricsSnapshot := apm.metricsCollector.GetCurrentMetrics()
	
	return AdaptiveMetrics{
		PriorityStats:      priorityStats,
		ConfigSnapshot:     configSnapshot,
		MetricsSnapshot:    metricsSnapshot,
		LastConfigUpdate:   apm.lastConfigUpdate,
		AdaptationInterval: apm.updateInterval,
	}
}

// AdaptiveMetrics contains comprehensive metrics for the adaptive priority system
type AdaptiveMetrics struct {
	PriorityStats      ManagerStats
	ConfigSnapshot     adaptive.AdaptiveBitswapConfigSnapshot
	MetricsSnapshot    PriorityMetrics
	LastConfigUpdate   time.Time
	AdaptationInterval time.Duration
}

// PriorityMetrics contains priority-specific metrics
type PriorityMetrics struct {
	RequestsPerSecond     float64
	AverageResponseTime   time.Duration
	P95ResponseTime       time.Duration
	OutstandingRequests   int64
	ActiveConnections     int
	PriorityDistribution  map[Priority]float64
	PriorityWaitTimes     map[Priority]time.Duration
	ResourceUtilization   float64
	LastUpdate            time.Time
}

// RecordRequest records metrics for a processed request
func (pmc *PriorityMetricsCollector) RecordRequest(request *PriorityRequest, result *RequestResult) {
	pmc.mu.Lock()
	defer pmc.mu.Unlock()
	
	now := time.Now()
	
	// Update request counter
	pmc.requestCounter++
	
	// Calculate requests per second
	if now.Sub(pmc.lastCounterReset) >= time.Second {
		pmc.requestsPerSecond = float64(pmc.requestCounter-pmc.lastRequestCount) / now.Sub(pmc.lastCounterReset).Seconds()
		pmc.lastRequestCount = pmc.requestCounter
		pmc.lastCounterReset = now
	}
	
	// Update response times (simple moving average)
	if pmc.averageResponseTime == 0 {
		pmc.averageResponseTime = result.ProcessTime
	} else {
		pmc.averageResponseTime = time.Duration(
			(int64(pmc.averageResponseTime)*9 + int64(result.ProcessTime)) / 10,
		)
	}
	
	// Update P95 response time (simplified - in production would use proper percentile calculation)
	if result.ProcessTime > pmc.p95ResponseTime {
		pmc.p95ResponseTime = result.ProcessTime
	} else {
		// Decay P95 slowly
		pmc.p95ResponseTime = time.Duration(int64(pmc.p95ResponseTime) * 99 / 100)
	}
	
	// Update priority-specific metrics
	waitTime := result.Request.QueueTime.Sub(result.Request.Context.RequestTime)
	priority := request.Priority
	
	if currentWait, exists := pmc.priorityWaitTimes[priority]; exists {
		pmc.priorityWaitTimes[priority] = time.Duration(
			(int64(currentWait)*9 + int64(waitTime)) / 10,
		)
	} else {
		pmc.priorityWaitTimes[priority] = waitTime
	}
	
	// Update priority distribution (simplified)
	totalRequests := float64(pmc.requestCounter)
	for p := LowPriority; p <= CriticalPriority; p++ {
		if p == priority {
			pmc.priorityDistribution[p] = (pmc.priorityDistribution[p]*(totalRequests-1) + 1) / totalRequests
		} else {
			pmc.priorityDistribution[p] = pmc.priorityDistribution[p] * (totalRequests - 1) / totalRequests
		}
	}
	
	pmc.lastMetricsUpdate = now
}

// GetCurrentMetrics returns current metrics snapshot
func (pmc *PriorityMetricsCollector) GetCurrentMetrics() PriorityMetrics {
	pmc.mu.RLock()
	defer pmc.mu.RUnlock()
	
	// Copy maps to avoid race conditions
	priorityDist := make(map[Priority]float64)
	priorityWait := make(map[Priority]time.Duration)
	
	for k, v := range pmc.priorityDistribution {
		priorityDist[k] = v
	}
	
	for k, v := range pmc.priorityWaitTimes {
		priorityWait[k] = v
	}
	
	return PriorityMetrics{
		RequestsPerSecond:     pmc.requestsPerSecond,
		AverageResponseTime:   pmc.averageResponseTime,
		P95ResponseTime:       pmc.p95ResponseTime,
		OutstandingRequests:   pmc.outstandingRequests,
		ActiveConnections:     pmc.activeConnections,
		PriorityDistribution:  priorityDist,
		PriorityWaitTimes:     priorityWait,
		ResourceUtilization:   pmc.resourceUtilization,
		LastUpdate:            pmc.lastMetricsUpdate,
	}
}

// UpdateResourceMetrics updates resource-related metrics
func (pmc *PriorityMetricsCollector) UpdateResourceMetrics(outstanding int64, connections int, utilization float64) {
	pmc.mu.Lock()
	defer pmc.mu.Unlock()
	
	pmc.outstandingRequests = outstanding
	pmc.activeConnections = connections
	pmc.resourceUtilization = utilization
}

// CreateAdaptivePriorityManagerFromConfig creates an adaptive priority manager from existing adaptive config
func CreateAdaptivePriorityManagerFromConfig(adaptiveConfig *adaptive.AdaptiveBitswapConfig) *AdaptivePriorityManager {
	return NewAdaptivePriorityManager(adaptiveConfig, 30*time.Second)
}