package adaptive

import (
	"context"
	"sync"
	"sync/atomic"
	"time"

	logging "github.com/ipfs/go-log/v2"
)

// LoadMonitor tracks performance metrics and triggers adaptive configuration changes
type LoadMonitor struct {
	config *AdaptiveBitswapConfig
	
	// Metrics tracking
	requestCount      int64
	responseTimeSum   int64 // nanoseconds
	responseTimeCount int64
	
	// Response time percentile tracking
	responseTimes     []time.Duration
	responseTimesMu   sync.RWMutex
	maxSamples        int
	
	// Outstanding requests tracking
	outstandingRequests int64
	activeConnections   int64
	
	// Monitoring control
	ctx        context.Context
	cancel     context.CancelFunc
	ticker     *time.Ticker
	wg         sync.WaitGroup
	
	// Configuration
	monitoringInterval time.Duration
}

// NewLoadMonitor creates a new load monitor for the given adaptive configuration
func NewLoadMonitor(config *AdaptiveBitswapConfig) *LoadMonitor {
	ctx, cancel := context.WithCancel(context.Background())
	
	return &LoadMonitor{
		config:             config,
		maxSamples:         1000, // Keep last 1000 response times for percentile calculation
		responseTimes:      make([]time.Duration, 0, 1000),
		ctx:                ctx,
		cancel:             cancel,
		monitoringInterval: 10 * time.Second, // Update metrics every 10 seconds
	}
}

// Start begins monitoring performance metrics and triggering adaptations
func (m *LoadMonitor) Start() {
	m.ticker = time.NewTicker(m.monitoringInterval)
	m.wg.Add(1)
	
	go func() {
		defer m.wg.Done()
		defer m.ticker.Stop()
		
		for {
			select {
			case <-m.ctx.Done():
				return
			case <-m.ticker.C:
				m.updateMetricsAndAdapt()
			}
		}
	}()
	
	log.Info("Load monitor started")
}

// Stop stops the load monitor
func (m *LoadMonitor) Stop() {
	m.cancel()
	m.wg.Wait()
	log.Info("Load monitor stopped")
}

// RecordRequest records the start of a new request
func (m *LoadMonitor) RecordRequest() {
	atomic.AddInt64(&m.requestCount, 1)
	atomic.AddInt64(&m.outstandingRequests, 1)
}

// RecordResponse records the completion of a request with its response time
func (m *LoadMonitor) RecordResponse(responseTime time.Duration) {
	atomic.AddInt64(&m.outstandingRequests, -1)
	atomic.AddInt64(&m.responseTimeSum, int64(responseTime))
	atomic.AddInt64(&m.responseTimeCount, 1)
	
	// Store response time for percentile calculation
	m.responseTimesMu.Lock()
	if len(m.responseTimes) >= m.maxSamples {
		// Remove oldest sample (simple circular buffer)
		copy(m.responseTimes, m.responseTimes[1:])
		m.responseTimes = m.responseTimes[:len(m.responseTimes)-1]
	}
	m.responseTimes = append(m.responseTimes, responseTime)
	m.responseTimesMu.Unlock()
}

// RecordConnectionChange updates the active connection count
func (m *LoadMonitor) RecordConnectionChange(delta int) {
	atomic.AddInt64(&m.activeConnections, int64(delta))
}

// updateMetricsAndAdapt calculates current metrics and triggers adaptation if needed
func (m *LoadMonitor) updateMetricsAndAdapt() {
	// Calculate requests per second
	currentRequests := atomic.SwapInt64(&m.requestCount, 0)
	requestsPerSecond := float64(currentRequests) / m.monitoringInterval.Seconds()
	
	// Calculate average response time
	totalResponseTime := atomic.SwapInt64(&m.responseTimeSum, 0)
	responseCount := atomic.SwapInt64(&m.responseTimeCount, 0)
	
	var avgResponseTime time.Duration
	if responseCount > 0 {
		avgResponseTime = time.Duration(totalResponseTime / responseCount)
	}
	
	// Calculate P95 response time
	p95ResponseTime := m.calculateP95ResponseTime()
	
	// Get current outstanding requests and connections
	outstandingRequests := atomic.LoadInt64(&m.outstandingRequests)
	activeConnections := int(atomic.LoadInt64(&m.activeConnections))
	
	// Update configuration metrics
	m.config.UpdateMetrics(
		requestsPerSecond,
		avgResponseTime,
		p95ResponseTime,
		outstandingRequests,
		activeConnections,
	)
	
	// Trigger adaptation if needed
	adapted := m.config.AdaptConfiguration(m.ctx)
	
	if log.LevelEnabled(logging.LevelDebug) {
		log.Debugf("Metrics update - RPS: %.1f, Avg Response: %v, P95: %v, Outstanding: %d, Connections: %d, Adapted: %v",
			requestsPerSecond, avgResponseTime, p95ResponseTime, outstandingRequests, activeConnections, adapted)
	}
}

// calculateP95ResponseTime calculates the 95th percentile response time
func (m *LoadMonitor) calculateP95ResponseTime() time.Duration {
	m.responseTimesMu.RLock()
	defer m.responseTimesMu.RUnlock()
	
	if len(m.responseTimes) == 0 {
		return 0
	}
	
	// Create a copy for sorting
	times := make([]time.Duration, len(m.responseTimes))
	copy(times, m.responseTimes)
	
	// Simple insertion sort (efficient for small arrays)
	for i := 1; i < len(times); i++ {
		key := times[i]
		j := i - 1
		for j >= 0 && times[j] > key {
			times[j+1] = times[j]
			j--
		}
		times[j+1] = key
	}
	
	// Calculate P95 index (0-based indexing)
	p95Index := int(float64(len(times)) * 0.95)
	if p95Index >= len(times) {
		p95Index = len(times) - 1
	}
	if p95Index < 0 {
		p95Index = 0
	}
	
	return times[p95Index]
}

// GetCurrentMetrics returns the current performance metrics
func (m *LoadMonitor) GetCurrentMetrics() LoadMetrics {
	outstandingRequests := atomic.LoadInt64(&m.outstandingRequests)
	activeConnections := int(atomic.LoadInt64(&m.activeConnections))
	
	snapshot := m.config.GetSnapshot()
	
	return LoadMetrics{
		RequestsPerSecond:     snapshot.RequestsPerSecond,
		AverageResponseTime:   snapshot.AverageResponseTime,
		P95ResponseTime:       snapshot.P95ResponseTime,
		OutstandingRequests:   outstandingRequests,
		ActiveConnections:     activeConnections,
		ConfigSnapshot:        snapshot,
	}
}

// LoadMetrics represents current performance metrics
type LoadMetrics struct {
	RequestsPerSecond     float64
	AverageResponseTime   time.Duration
	P95ResponseTime       time.Duration
	OutstandingRequests   int64
	ActiveConnections     int
	ConfigSnapshot        AdaptiveBitswapConfigSnapshot
}

// RequestTracker helps track individual request lifecycles
type RequestTracker struct {
	monitor   *LoadMonitor
	startTime time.Time
}

// NewRequestTracker creates a new request tracker
func (m *LoadMonitor) NewRequestTracker() *RequestTracker {
	m.RecordRequest()
	return &RequestTracker{
		monitor:   m,
		startTime: time.Now(),
	}
}

// Complete marks the request as completed and records its response time
func (rt *RequestTracker) Complete() {
	responseTime := time.Since(rt.startTime)
	rt.monitor.RecordResponse(responseTime)
}

// CompleteWithError marks the request as completed with an error
func (rt *RequestTracker) CompleteWithError() {
	// Still record the response time even for errors
	rt.Complete()
}