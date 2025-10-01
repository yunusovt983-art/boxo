package circuitbreaker

import (
	"context"
	"sync"
	"time"

	"github.com/libp2p/go-libp2p/core/peer"
	logging "github.com/ipfs/go-log/v2"
)

var healthLog = logging.Logger("bitswap/circuitbreaker/health")

// ConnectionHealth represents the health status of a connection to a peer
type ConnectionHealth struct {
	PeerID           peer.ID
	LastSeen         time.Time
	ResponseTime     time.Duration
	SuccessRate      float64
	ErrorCount       int64
	TotalRequests    int64
	ConsecutiveErrors int64
	IsHealthy        bool
	LastError        error
}

// HealthStatus represents overall health categories
type HealthStatus int

const (
	HealthStatusHealthy HealthStatus = iota
	HealthStatusDegraded
	HealthStatusUnhealthy
	HealthStatusUnknown
)

func (hs HealthStatus) String() string {
	switch hs {
	case HealthStatusHealthy:
		return "HEALTHY"
	case HealthStatusDegraded:
		return "DEGRADED"
	case HealthStatusUnhealthy:
		return "UNHEALTHY"
	case HealthStatusUnknown:
		return "UNKNOWN"
	default:
		return "INVALID"
	}
}

// HealthMonitorConfig holds configuration for the health monitor
type HealthMonitorConfig struct {
	// CheckInterval is how often to perform health checks
	CheckInterval time.Duration
	
	// HealthyThreshold defines the success rate threshold for healthy status
	HealthyThreshold float64
	
	// DegradedThreshold defines the success rate threshold for degraded status
	DegradedThreshold float64
	
	// MaxResponseTime is the maximum acceptable response time
	MaxResponseTime time.Duration
	
	// MaxConsecutiveErrors before marking as unhealthy
	MaxConsecutiveErrors int64
	
	// HealthCheckTimeout for individual health checks
	HealthCheckTimeout time.Duration
	
	// CleanupInterval for removing stale health records
	CleanupInterval time.Duration
	
	// MaxAge for health records before cleanup
	MaxAge time.Duration
}

// DefaultHealthMonitorConfig returns a default health monitor configuration
func DefaultHealthMonitorConfig() HealthMonitorConfig {
	return HealthMonitorConfig{
		CheckInterval:        30 * time.Second,
		HealthyThreshold:     0.95, // 95% success rate
		DegradedThreshold:    0.80, // 80% success rate
		MaxResponseTime:      5 * time.Second,
		MaxConsecutiveErrors: 5,
		HealthCheckTimeout:   10 * time.Second,
		CleanupInterval:      5 * time.Minute,
		MaxAge:              10 * time.Minute,
	}
}

// HealthMonitor monitors the health of connections to peers
type HealthMonitor struct {
	config      HealthMonitorConfig
	healthData  map[peer.ID]*ConnectionHealth
	mutex       sync.RWMutex
	ctx         context.Context
	cancel      context.CancelFunc
	wg          sync.WaitGroup
	
	// Callbacks
	onHealthChange func(peer.ID, HealthStatus, HealthStatus)
}

// NewHealthMonitor creates a new health monitor
func NewHealthMonitor(config HealthMonitorConfig) *HealthMonitor {
	ctx, cancel := context.WithCancel(context.Background())
	
	hm := &HealthMonitor{
		config:     config,
		healthData: make(map[peer.ID]*ConnectionHealth),
		ctx:        ctx,
		cancel:     cancel,
	}
	
	// Start background monitoring
	hm.wg.Add(1)
	go hm.monitorLoop()
	
	return hm
}

// RecordRequest records the result of a request to a peer
func (hm *HealthMonitor) RecordRequest(peerID peer.ID, responseTime time.Duration, err error) {
	hm.mutex.Lock()
	defer hm.mutex.Unlock()
	
	health, exists := hm.healthData[peerID]
	var oldStatus HealthStatus
	
	if !exists {
		health = &ConnectionHealth{
			PeerID:    peerID,
			LastSeen:  time.Now(),
			IsHealthy: true,
		}
		hm.healthData[peerID] = health
		oldStatus = HealthStatusUnknown // New peer starts as unknown
	} else {
		oldStatus = hm.getHealthStatus(health)
	}
	
	// Update metrics
	health.LastSeen = time.Now()
	health.TotalRequests++
	health.ResponseTime = responseTime
	
	if err != nil {
		health.ErrorCount++
		health.ConsecutiveErrors++
		health.LastError = err
	} else {
		health.ConsecutiveErrors = 0
		health.LastError = nil
	}
	
	// Calculate success rate
	if health.TotalRequests > 0 {
		successCount := health.TotalRequests - health.ErrorCount
		health.SuccessRate = float64(successCount) / float64(health.TotalRequests)
	}
	
	// Update health status and notify of changes
	hm.updateHealthStatus(health)
	newStatus := hm.getHealthStatus(health)
	
	// Notify of health changes
	if oldStatus != newStatus && hm.onHealthChange != nil {
		hm.onHealthChange(peerID, oldStatus, newStatus)
	}
	
	healthLog.Debugf("Recorded request for peer %s: responseTime=%v, err=%v, successRate=%.2f, status=%s",
		peerID, responseTime, err, health.SuccessRate, newStatus)
}

// GetHealth returns the current health status for a peer
func (hm *HealthMonitor) GetHealth(peerID peer.ID) (*ConnectionHealth, bool) {
	hm.mutex.RLock()
	defer hm.mutex.RUnlock()
	
	health, exists := hm.healthData[peerID]
	if !exists {
		return nil, false
	}
	
	// Return a copy to avoid race conditions
	healthCopy := *health
	return &healthCopy, true
}

// GetHealthStatus returns the health status for a peer
func (hm *HealthMonitor) GetHealthStatus(peerID peer.ID) HealthStatus {
	hm.mutex.RLock()
	defer hm.mutex.RUnlock()
	
	health, exists := hm.healthData[peerID]
	if !exists {
		return HealthStatusUnknown
	}
	
	return hm.getHealthStatus(health)
}

// GetAllHealth returns health data for all monitored peers
func (hm *HealthMonitor) GetAllHealth() map[peer.ID]ConnectionHealth {
	hm.mutex.RLock()
	defer hm.mutex.RUnlock()
	
	result := make(map[peer.ID]ConnectionHealth, len(hm.healthData))
	for peerID, health := range hm.healthData {
		result[peerID] = *health // Copy the health data
	}
	
	return result
}

// IsHealthy returns true if the peer is considered healthy
func (hm *HealthMonitor) IsHealthy(peerID peer.ID) bool {
	status := hm.GetHealthStatus(peerID)
	return status == HealthStatusHealthy
}

// SetHealthChangeCallback sets a callback for health status changes
func (hm *HealthMonitor) SetHealthChangeCallback(callback func(peer.ID, HealthStatus, HealthStatus)) {
	hm.mutex.Lock()
	defer hm.mutex.Unlock()
	hm.onHealthChange = callback
}

// Close stops the health monitor
func (hm *HealthMonitor) Close() error {
	hm.cancel()
	hm.wg.Wait()
	return nil
}

// getHealthStatus determines the health status based on current metrics
func (hm *HealthMonitor) getHealthStatus(health *ConnectionHealth) HealthStatus {
	// Check if we have enough data
	if health.TotalRequests < 3 {
		return HealthStatusUnknown
	}
	
	// Check consecutive errors
	if health.ConsecutiveErrors >= hm.config.MaxConsecutiveErrors {
		return HealthStatusUnhealthy
	}
	
	// Check response time
	if health.ResponseTime > hm.config.MaxResponseTime {
		return HealthStatusDegraded
	}
	
	// Check success rate
	if health.SuccessRate >= hm.config.HealthyThreshold {
		return HealthStatusHealthy
	} else if health.SuccessRate >= hm.config.DegradedThreshold {
		return HealthStatusDegraded
	} else {
		return HealthStatusUnhealthy
	}
}

// updateHealthStatus updates the IsHealthy flag based on current metrics
func (hm *HealthMonitor) updateHealthStatus(health *ConnectionHealth) {
	status := hm.getHealthStatus(health)
	health.IsHealthy = (status == HealthStatusHealthy)
}

// monitorLoop runs the background monitoring tasks
func (hm *HealthMonitor) monitorLoop() {
	defer hm.wg.Done()
	
	cleanupTicker := time.NewTicker(hm.config.CleanupInterval)
	defer cleanupTicker.Stop()
	
	for {
		select {
		case <-hm.ctx.Done():
			return
		case <-cleanupTicker.C:
			hm.cleanupStaleData()
		}
	}
}

// cleanupStaleData removes old health records
func (hm *HealthMonitor) cleanupStaleData() {
	hm.mutex.Lock()
	defer hm.mutex.Unlock()
	
	now := time.Now()
	for peerID, health := range hm.healthData {
		if now.Sub(health.LastSeen) > hm.config.MaxAge {
			delete(hm.healthData, peerID)
			healthLog.Debugf("Cleaned up stale health data for peer %s", peerID)
		}
	}
}

// HealthStats provides aggregate health statistics
type HealthStats struct {
	TotalPeers     int
	HealthyPeers   int
	DegradedPeers  int
	UnhealthyPeers int
	UnknownPeers   int
	AverageSuccessRate float64
	AverageResponseTime time.Duration
}

// GetHealthStats returns aggregate health statistics
func (hm *HealthMonitor) GetHealthStats() HealthStats {
	hm.mutex.RLock()
	defer hm.mutex.RUnlock()
	
	stats := HealthStats{
		TotalPeers: len(hm.healthData),
	}
	
	if stats.TotalPeers == 0 {
		return stats
	}
	
	var totalSuccessRate float64
	var totalResponseTime time.Duration
	
	for _, health := range hm.healthData {
		status := hm.getHealthStatus(health)
		switch status {
		case HealthStatusHealthy:
			stats.HealthyPeers++
		case HealthStatusDegraded:
			stats.DegradedPeers++
		case HealthStatusUnhealthy:
			stats.UnhealthyPeers++
		case HealthStatusUnknown:
			stats.UnknownPeers++
		}
		
		totalSuccessRate += health.SuccessRate
		totalResponseTime += health.ResponseTime
	}
	
	stats.AverageSuccessRate = totalSuccessRate / float64(stats.TotalPeers)
	stats.AverageResponseTime = totalResponseTime / time.Duration(stats.TotalPeers)
	
	return stats
}