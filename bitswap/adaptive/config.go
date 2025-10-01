// Package adaptive provides adaptive configuration management for Bitswap
// to optimize performance under high load conditions.
package adaptive

import (
	"context"
	"sync"
	"time"

	"github.com/ipfs/boxo/bitswap/internal/defaults"
	logging "github.com/ipfs/go-log/v2"
)

var log = logging.Logger("bitswap/adaptive")

// AdaptiveBitswapConfig manages dynamic configuration parameters for Bitswap
// to optimize performance based on current load conditions.
type AdaptiveBitswapConfig struct {
	mu sync.RWMutex

	// Dynamic byte limits per peer
	MaxOutstandingBytesPerPeer int64 // Current adaptive limit (1MB-100MB)
	MinOutstandingBytesPerPeer int64 // Minimum limit (256KB)
	BaseOutstandingBytesPerPeer int64 // Base limit (1MB default)

	// Worker pool configuration
	MinWorkerCount int // Minimum workers (128)
	MaxWorkerCount int // Maximum workers (2048)
	CurrentWorkerCount int // Current active workers

	// Priority thresholds for request classification
	HighPriorityThreshold     time.Duration // 50ms
	CriticalPriorityThreshold time.Duration // 10ms

	// Batch processing configuration
	BatchSize    int           // 100-1000 requests
	BatchTimeout time.Duration // 10ms

	// Load monitoring
	RequestsPerSecond     float64   // Current RPS
	AverageResponseTime   time.Duration // Current avg response time
	P95ResponseTime       time.Duration // 95th percentile response time
	OutstandingRequests   int64     // Current outstanding requests
	ActiveConnections     int       // Current active connections
	LastAdaptationTime    time.Time // Last time config was adapted

	// Adaptation parameters
	LoadThresholdHigh     float64 // 0.8 - trigger scale up
	LoadThresholdLow      float64 // 0.3 - trigger scale down
	AdaptationInterval    time.Duration // 30s - minimum time between adaptations
	ScaleUpFactor         float64 // 1.5 - multiplier for scaling up
	ScaleDownFactor       float64 // 0.8 - multiplier for scaling down

	// Callbacks for configuration changes
	onConfigChange func(*AdaptiveBitswapConfig)
}

// NewAdaptiveBitswapConfig creates a new adaptive configuration with default values
func NewAdaptiveBitswapConfig() *AdaptiveBitswapConfig {
	return &AdaptiveBitswapConfig{
		// Byte limits
		MaxOutstandingBytesPerPeer:  defaults.BitswapMaxOutstandingBytesPerPeer,
		MinOutstandingBytesPerPeer:  256 * 1024, // 256KB
		BaseOutstandingBytesPerPeer: defaults.BitswapMaxOutstandingBytesPerPeer,

		// Worker configuration
		MinWorkerCount:     defaults.BitswapEngineBlockstoreWorkerCount,
		MaxWorkerCount:     2048,
		CurrentWorkerCount: defaults.BitswapEngineBlockstoreWorkerCount,

		// Priority thresholds
		HighPriorityThreshold:     50 * time.Millisecond,
		CriticalPriorityThreshold: 10 * time.Millisecond,

		// Batch configuration
		BatchSize:    100,
		BatchTimeout: 10 * time.Millisecond,

		// Adaptation parameters
		LoadThresholdHigh:  0.8,
		LoadThresholdLow:   0.3,
		AdaptationInterval: 30 * time.Second,
		ScaleUpFactor:      1.5,
		ScaleDownFactor:    0.8,

		LastAdaptationTime: time.Time{}, // Zero time to allow immediate first adaptation
	}
}

// UpdateMetrics updates the current performance metrics used for adaptation decisions
func (c *AdaptiveBitswapConfig) UpdateMetrics(
	requestsPerSecond float64,
	avgResponseTime time.Duration,
	p95ResponseTime time.Duration,
	outstandingRequests int64,
	activeConnections int,
) {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.RequestsPerSecond = requestsPerSecond
	c.AverageResponseTime = avgResponseTime
	c.P95ResponseTime = p95ResponseTime
	c.OutstandingRequests = outstandingRequests
	c.ActiveConnections = activeConnections
}

// AdaptConfiguration automatically adjusts configuration based on current load
func (c *AdaptiveBitswapConfig) AdaptConfiguration(ctx context.Context) bool {
	c.mu.Lock()
	defer c.mu.Unlock()

	// Check if enough time has passed since last adaptation
	if time.Since(c.LastAdaptationTime) < c.AdaptationInterval {
		return false
	}

	adapted := false
	
	// Calculate current load factor based on multiple metrics
	loadFactor := c.calculateLoadFactor()
	
	log.Debugf("Current load factor: %.2f, RPS: %.1f, Avg Response: %v, P95: %v, Outstanding: %d",
		loadFactor, c.RequestsPerSecond, c.AverageResponseTime, c.P95ResponseTime, c.OutstandingRequests)

	// Adapt based on load
	if loadFactor > c.LoadThresholdHigh {
		adapted = c.scaleUp()
		if adapted {
			log.Infof("Scaled up configuration due to high load (%.2f)", loadFactor)
		}
	} else if loadFactor < c.LoadThresholdLow {
		adapted = c.scaleDown()
		if adapted {
			log.Infof("Scaled down configuration due to low load (%.2f)", loadFactor)
		}
	}

	if adapted {
		c.LastAdaptationTime = time.Now()
		if c.onConfigChange != nil {
			c.onConfigChange(c)
		}
	}

	return adapted
}

// calculateLoadFactor computes a normalized load factor (0.0 to 1.0+) based on current metrics
func (c *AdaptiveBitswapConfig) calculateLoadFactor() float64 {
	// Response time factor (0.0 to 1.0, where 1.0 = 100ms response time)
	responseTimeFactor := float64(c.AverageResponseTime) / float64(100*time.Millisecond)
	if responseTimeFactor > 1.0 {
		responseTimeFactor = 1.0
	}

	// P95 response time factor (0.0 to 1.0, where 1.0 = 200ms P95 response time)
	p95Factor := float64(c.P95ResponseTime) / float64(200*time.Millisecond)
	if p95Factor > 1.0 {
		p95Factor = 1.0
	}

	// Outstanding requests factor (0.0 to 1.0, where 1.0 = 10000 outstanding requests)
	outstandingFactor := float64(c.OutstandingRequests) / 10000.0
	if outstandingFactor > 1.0 {
		outstandingFactor = 1.0
	}

	// RPS factor (0.0 to 1.0, where 1.0 = 1000 RPS)
	rpsFactor := c.RequestsPerSecond / 1000.0
	if rpsFactor > 1.0 {
		rpsFactor = 1.0
	}

	// Weighted average of factors
	loadFactor := (responseTimeFactor*0.3 + p95Factor*0.3 + outstandingFactor*0.2 + rpsFactor*0.2)
	
	return loadFactor
}

// scaleUp increases configuration limits to handle higher load
func (c *AdaptiveBitswapConfig) scaleUp() bool {
	adapted := false

	// Increase outstanding bytes per peer
	newMaxBytes := int64(float64(c.MaxOutstandingBytesPerPeer) * c.ScaleUpFactor)
	maxLimit := int64(100 * 1024 * 1024) // 100MB max
	if newMaxBytes <= maxLimit && newMaxBytes > c.MaxOutstandingBytesPerPeer {
		c.MaxOutstandingBytesPerPeer = newMaxBytes
		adapted = true
	}

	// Increase worker count
	newWorkerCount := int(float64(c.CurrentWorkerCount) * c.ScaleUpFactor)
	if newWorkerCount <= c.MaxWorkerCount && newWorkerCount > c.CurrentWorkerCount {
		c.CurrentWorkerCount = newWorkerCount
		adapted = true
	}

	// Increase batch size for better throughput
	newBatchSize := int(float64(c.BatchSize) * c.ScaleUpFactor)
	maxBatchSize := 1000
	if newBatchSize <= maxBatchSize && newBatchSize > c.BatchSize {
		c.BatchSize = newBatchSize
		adapted = true
	}

	return adapted
}

// scaleDown decreases configuration limits to conserve resources
func (c *AdaptiveBitswapConfig) scaleDown() bool {
	adapted := false

	// Decrease outstanding bytes per peer
	newMaxBytes := int64(float64(c.MaxOutstandingBytesPerPeer) * c.ScaleDownFactor)
	if newMaxBytes >= c.MinOutstandingBytesPerPeer && newMaxBytes < c.MaxOutstandingBytesPerPeer {
		c.MaxOutstandingBytesPerPeer = newMaxBytes
		adapted = true
	}

	// Decrease worker count
	newWorkerCount := int(float64(c.CurrentWorkerCount) * c.ScaleDownFactor)
	if newWorkerCount >= c.MinWorkerCount && newWorkerCount < c.CurrentWorkerCount {
		c.CurrentWorkerCount = newWorkerCount
		adapted = true
	}

	// Decrease batch size
	newBatchSize := int(float64(c.BatchSize) * c.ScaleDownFactor)
	minBatchSize := 50
	if newBatchSize >= minBatchSize && newBatchSize < c.BatchSize {
		c.BatchSize = newBatchSize
		adapted = true
	}

	return adapted
}

// GetMaxOutstandingBytesPerPeer returns the current adaptive limit thread-safely
func (c *AdaptiveBitswapConfig) GetMaxOutstandingBytesPerPeer() int64 {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.MaxOutstandingBytesPerPeer
}

// GetCurrentWorkerCount returns the current worker count thread-safely
func (c *AdaptiveBitswapConfig) GetCurrentWorkerCount() int {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.CurrentWorkerCount
}

// GetBatchConfig returns the current batch configuration thread-safely
func (c *AdaptiveBitswapConfig) GetBatchConfig() (int, time.Duration) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.BatchSize, c.BatchTimeout
}

// GetPriorityThresholds returns the current priority thresholds thread-safely
func (c *AdaptiveBitswapConfig) GetPriorityThresholds() (time.Duration, time.Duration) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.HighPriorityThreshold, c.CriticalPriorityThreshold
}

// SetConfigChangeCallback sets a callback function to be called when configuration changes
func (c *AdaptiveBitswapConfig) SetConfigChangeCallback(callback func(*AdaptiveBitswapConfig)) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.onConfigChange = callback
}

// GetSnapshot returns a read-only snapshot of the current configuration
func (c *AdaptiveBitswapConfig) GetSnapshot() AdaptiveBitswapConfigSnapshot {
	c.mu.RLock()
	defer c.mu.RUnlock()

	return AdaptiveBitswapConfigSnapshot{
		MaxOutstandingBytesPerPeer:  c.MaxOutstandingBytesPerPeer,
		MinOutstandingBytesPerPeer:  c.MinOutstandingBytesPerPeer,
		CurrentWorkerCount:          c.CurrentWorkerCount,
		BatchSize:                   c.BatchSize,
		BatchTimeout:                c.BatchTimeout,
		HighPriorityThreshold:       c.HighPriorityThreshold,
		CriticalPriorityThreshold:   c.CriticalPriorityThreshold,
		RequestsPerSecond:           c.RequestsPerSecond,
		AverageResponseTime:         c.AverageResponseTime,
		P95ResponseTime:             c.P95ResponseTime,
		OutstandingRequests:         c.OutstandingRequests,
		ActiveConnections:           c.ActiveConnections,
		LastAdaptationTime:          c.LastAdaptationTime,
	}
}

// AdaptiveBitswapConfigSnapshot provides a read-only view of the configuration
type AdaptiveBitswapConfigSnapshot struct {
	MaxOutstandingBytesPerPeer  int64
	MinOutstandingBytesPerPeer  int64
	CurrentWorkerCount          int
	BatchSize                   int
	BatchTimeout                time.Duration
	HighPriorityThreshold       time.Duration
	CriticalPriorityThreshold   time.Duration
	RequestsPerSecond           float64
	AverageResponseTime         time.Duration
	P95ResponseTime             time.Duration
	OutstandingRequests         int64
	ActiveConnections           int
	LastAdaptationTime          time.Time
}