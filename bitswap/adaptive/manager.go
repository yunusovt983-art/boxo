package adaptive

import (
	"context"
	"sync"
	"time"

	"github.com/ipfs/boxo/bitswap/network"
	"github.com/libp2p/go-libp2p/core/peer"
)

// AdaptiveConnectionManager manages bitswap connections with adaptive configuration
type AdaptiveConnectionManager struct {
	config  *AdaptiveBitswapConfig
	monitor *LoadMonitor
	network network.BitSwapNetwork
	
	// Connection tracking
	connectionsMu sync.RWMutex
	connections   map[peer.ID]*ConnectionInfo
	
	// Lifecycle management
	ctx    context.Context
	cancel context.CancelFunc
	wg     sync.WaitGroup
}

// ConnectionInfo tracks information about individual peer connections
type ConnectionInfo struct {
	PeerID              peer.ID
	ConnectedAt         time.Time
	LastActivity        time.Time
	OutstandingBytes    int64
	RequestCount        int64
	ResponseCount       int64
	AverageResponseTime time.Duration
	ErrorCount          int64
}

// NewAdaptiveConnectionManager creates a new adaptive connection manager
func NewAdaptiveConnectionManager(
	config *AdaptiveBitswapConfig,
	network network.BitSwapNetwork,
) *AdaptiveConnectionManager {
	ctx, cancel := context.WithCancel(context.Background())
	
	monitor := NewLoadMonitor(config)
	
	manager := &AdaptiveConnectionManager{
		config:      config,
		monitor:     monitor,
		network:     network,
		connections: make(map[peer.ID]*ConnectionInfo),
		ctx:         ctx,
		cancel:      cancel,
	}
	
	// Set up configuration change callback
	config.SetConfigChangeCallback(manager.onConfigurationChange)
	
	return manager
}

// Start begins the adaptive connection management
func (m *AdaptiveConnectionManager) Start() error {
	m.monitor.Start()
	
	// Start periodic connection optimization
	m.wg.Add(1)
	go func() {
		defer m.wg.Done()
		m.optimizeConnectionsLoop()
	}()
	
	log.Info("Adaptive connection manager started")
	return nil
}

// Stop stops the adaptive connection manager
func (m *AdaptiveConnectionManager) Stop() error {
	m.cancel()
	m.monitor.Stop()
	m.wg.Wait()
	
	log.Info("Adaptive connection manager stopped")
	return nil
}

// PeerConnected handles a new peer connection
func (m *AdaptiveConnectionManager) PeerConnected(peerID peer.ID) {
	m.connectionsMu.Lock()
	defer m.connectionsMu.Unlock()
	
	now := time.Now()
	m.connections[peerID] = &ConnectionInfo{
		PeerID:       peerID,
		ConnectedAt:  now,
		LastActivity: now,
	}
	
	m.monitor.RecordConnectionChange(1)
	
	log.Debugf("Peer connected: %s", peerID)
}

// PeerDisconnected handles a peer disconnection
func (m *AdaptiveConnectionManager) PeerDisconnected(peerID peer.ID) {
	m.connectionsMu.Lock()
	defer m.connectionsMu.Unlock()
	
	delete(m.connections, peerID)
	m.monitor.RecordConnectionChange(-1)
	
	log.Debugf("Peer disconnected: %s", peerID)
}

// RecordRequest records a new request to a peer
func (m *AdaptiveConnectionManager) RecordRequest(peerID peer.ID, bytes int64) *RequestTracker {
	m.connectionsMu.Lock()
	if conn, exists := m.connections[peerID]; exists {
		conn.RequestCount++
		conn.OutstandingBytes += bytes
		conn.LastActivity = time.Now()
	}
	m.connectionsMu.Unlock()
	
	return m.monitor.NewRequestTracker()
}

// RecordResponse records a response from a peer
func (m *AdaptiveConnectionManager) RecordResponse(peerID peer.ID, bytes int64, responseTime time.Duration, success bool) {
	m.connectionsMu.Lock()
	if conn, exists := m.connections[peerID]; exists {
		conn.ResponseCount++
		conn.OutstandingBytes -= bytes
		conn.LastActivity = time.Now()
		
		if success {
			// Update average response time using exponential moving average
			if conn.AverageResponseTime == 0 {
				conn.AverageResponseTime = responseTime
			} else {
				// EMA with alpha = 0.1
				conn.AverageResponseTime = time.Duration(
					0.9*float64(conn.AverageResponseTime) + 0.1*float64(responseTime),
				)
			}
		} else {
			conn.ErrorCount++
		}
	}
	m.connectionsMu.Unlock()
}

// GetMaxOutstandingBytesForPeer returns the current adaptive limit for a specific peer
func (m *AdaptiveConnectionManager) GetMaxOutstandingBytesForPeer(peerID peer.ID) int64 {
	baseLimit := m.config.GetMaxOutstandingBytesPerPeer()
	
	m.connectionsMu.RLock()
	conn, exists := m.connections[peerID]
	m.connectionsMu.RUnlock()
	
	if !exists {
		return baseLimit
	}
	
	// Adjust limit based on peer performance
	adjustmentFactor := 1.0
	
	// Reduce limit for peers with high error rates
	if conn.ResponseCount > 10 {
		errorRate := float64(conn.ErrorCount) / float64(conn.ResponseCount)
		if errorRate > 0.1 { // More than 10% error rate
			adjustmentFactor *= (1.0 - errorRate)
		}
	}
	
	// Reduce limit for peers with high response times
	if conn.AverageResponseTime > 0 {
		highPriorityThreshold, _ := m.config.GetPriorityThresholds()
		if conn.AverageResponseTime > highPriorityThreshold {
			// Reduce limit proportionally to response time
			timeoutFactor := float64(highPriorityThreshold) / float64(conn.AverageResponseTime)
			if timeoutFactor < 0.5 {
				timeoutFactor = 0.5 // Don't reduce below 50%
			}
			adjustmentFactor *= timeoutFactor
		}
	}
	
	adjustedLimit := int64(float64(baseLimit) * adjustmentFactor)
	
	// Ensure minimum limit
	minLimit := m.config.MinOutstandingBytesPerPeer
	if adjustedLimit < minLimit {
		adjustedLimit = minLimit
	}
	
	return adjustedLimit
}

// GetConnectionInfo returns information about a specific connection
func (m *AdaptiveConnectionManager) GetConnectionInfo(peerID peer.ID) (*ConnectionInfo, bool) {
	m.connectionsMu.RLock()
	defer m.connectionsMu.RUnlock()
	
	conn, exists := m.connections[peerID]
	if !exists {
		return nil, false
	}
	
	// Return a copy to avoid race conditions
	connCopy := *conn
	return &connCopy, true
}

// GetAllConnections returns information about all current connections
func (m *AdaptiveConnectionManager) GetAllConnections() map[peer.ID]*ConnectionInfo {
	m.connectionsMu.RLock()
	defer m.connectionsMu.RUnlock()
	
	result := make(map[peer.ID]*ConnectionInfo, len(m.connections))
	for peerID, conn := range m.connections {
		connCopy := *conn
		result[peerID] = &connCopy
	}
	
	return result
}

// GetLoadMetrics returns current load metrics
func (m *AdaptiveConnectionManager) GetLoadMetrics() LoadMetrics {
	return m.monitor.GetCurrentMetrics()
}

// onConfigurationChange is called when the adaptive configuration changes
func (m *AdaptiveConnectionManager) onConfigurationChange(config *AdaptiveBitswapConfig) {
	snapshot := config.GetSnapshot()
	log.Infof("Configuration adapted - MaxBytes: %d, Workers: %d, BatchSize: %d",
		snapshot.MaxOutstandingBytesPerPeer,
		snapshot.CurrentWorkerCount,
		snapshot.BatchSize)
}

// optimizeConnectionsLoop periodically optimizes connections based on performance
func (m *AdaptiveConnectionManager) optimizeConnectionsLoop() {
	ticker := time.NewTicker(60 * time.Second) // Optimize every minute
	defer ticker.Stop()
	
	for {
		select {
		case <-m.ctx.Done():
			return
		case <-ticker.C:
			m.optimizeConnections()
		}
	}
}

// optimizeConnections performs periodic connection optimization
func (m *AdaptiveConnectionManager) optimizeConnections() {
	m.connectionsMu.RLock()
	connections := make([]*ConnectionInfo, 0, len(m.connections))
	for _, conn := range m.connections {
		connCopy := *conn
		connections = append(connections, &connCopy)
	}
	m.connectionsMu.RUnlock()
	
	now := time.Now()
	staleThreshold := 5 * time.Minute
	
	for _, conn := range connections {
		// Check for stale connections
		if now.Sub(conn.LastActivity) > staleThreshold {
			log.Debugf("Detected stale connection to peer %s, last activity: %v ago",
				conn.PeerID, now.Sub(conn.LastActivity))
			// Note: We don't automatically disconnect here as that should be handled
			// by the underlying network layer. We just log for monitoring.
		}
		
		// Log performance statistics for poorly performing peers
		if conn.ResponseCount > 10 {
			errorRate := float64(conn.ErrorCount) / float64(conn.ResponseCount)
			if errorRate > 0.2 { // More than 20% error rate
				log.Warnf("Peer %s has high error rate: %.2f%% (%d errors out of %d responses)",
					conn.PeerID, errorRate*100, conn.ErrorCount, conn.ResponseCount)
			}
		}
		
		highPriorityThreshold, _ := m.config.GetPriorityThresholds()
		if conn.AverageResponseTime > 2*highPriorityThreshold {
			log.Warnf("Peer %s has high average response time: %v (threshold: %v)",
				conn.PeerID, conn.AverageResponseTime, highPriorityThreshold)
		}
	}
}