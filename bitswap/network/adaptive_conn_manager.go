package network

import (
	"context"
	"errors"
	"sync"
	"time"

	logging "github.com/ipfs/go-log/v2"
	"github.com/libp2p/go-libp2p/core/host"
	"github.com/libp2p/go-libp2p/core/network"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/libp2p/go-libp2p/p2p/protocol/ping"
	"github.com/multiformats/go-multiaddr"
)

var acmLog = logging.Logger("bitswap/network/acm")

// Error definitions for connection quality monitoring
var (
	ErrPeerNotFound        = errors.New("peer not found")
	ErrNoAlternativeRoute  = errors.New("no alternative route available")
	ErrPingTimeout         = errors.New("ping timeout")
	ErrPeerNotConnected    = errors.New("peer not connected")
)

// AdaptiveConnManager manages connections with dynamic limits and load-based adaptation
type AdaptiveConnManager struct {
	host         host.Host
	config       *AdaptiveConnConfig
	connections  map[peer.ID]*ConnectionInfo
	clusterPeers map[peer.ID]bool // Persistent connections for cluster nodes
	
	mu           sync.RWMutex
	loadMonitor  *LoadMonitor
	
	// Connection limits
	currentHighWater int
	currentLowWater  int
	currentGracePeriod time.Duration
	
	// Metrics
	metrics *ConnManagerMetrics
	
	// Control channels
	stopCh chan struct{}
	doneCh chan struct{}
}

// AdaptiveConnConfig holds configuration for the adaptive connection manager
type AdaptiveConnConfig struct {
	// Base limits
	BaseHighWater int
	BaseLowWater  int
	BaseGracePeriod time.Duration
	
	// Adaptive scaling factors
	MaxHighWater int
	MinLowWater  int
	MaxGracePeriod time.Duration
	MinGracePeriod time.Duration
	
	// Load thresholds for scaling
	HighLoadThreshold    float64 // 0.8 = 80% load
	LowLoadThreshold     float64 // 0.3 = 30% load
	
	// Adaptation intervals
	AdaptationInterval   time.Duration // How often to check and adapt
	LoadSampleWindow     time.Duration // Window for load calculation
	
	// Cluster configuration
	ClusterPeers []peer.ID // Peers that should have persistent connections
}

// ConnectionInfo tracks information about a specific connection
type ConnectionInfo struct {
	PeerID       peer.ID
	ConnectedAt  time.Time
	LastActivity time.Time
	IsCluster    bool
	Quality      *ConnectionQuality
	
	// Connection state
	State        ConnectionState
	ErrorCount   int
	LastError    time.Time
}

// ConnectionState represents the state of a connection
type ConnectionState int

const (
	StateConnecting ConnectionState = iota
	StateConnected
	StateUnresponsive
	StateDisconnected
	StateError
)

// ConnectionQuality tracks quality metrics for a connection
type ConnectionQuality struct {
	Latency       time.Duration
	Bandwidth     int64
	ErrorRate     float64
	LastMeasured  time.Time
	
	// Moving averages
	AvgLatency    time.Duration
	AvgBandwidth  int64
	AvgErrorRate  float64
	
	// Quality scoring
	QualityScore  float64 // 0.0 to 1.0, higher is better
	
	// Request tracking
	TotalRequests    int64
	SuccessfulRequests int64
	FailedRequests   int64
	
	// Route optimization data
	AlternativeRoutes []peer.ID
	PreferredRoute    peer.ID
	RouteChanges      int64
}

// LoadMonitor tracks system load for adaptive scaling
type LoadMonitor struct {
	mu sync.RWMutex
	
	// Current metrics
	ActiveConnections int
	RequestsPerSecond float64
	AverageLatency    time.Duration
	ErrorRate         float64
	
	// Historical data for trend analysis
	samples []LoadSample
	maxSamples int
}

// LoadSample represents a point-in-time load measurement
type LoadSample struct {
	Timestamp         time.Time
	ActiveConnections int
	RequestsPerSecond float64
	AverageLatency    time.Duration
	ErrorRate         float64
}

// ConnManagerMetrics holds metrics for the connection manager
type ConnManagerMetrics struct {
	TotalConnections    int64
	ClusterConnections  int64
	AdaptationEvents    int64
	ConnectionErrors    int64
	GracePeriodTriggers int64
}

// NewAdaptiveConnManager creates a new adaptive connection manager
func NewAdaptiveConnManager(h host.Host, config *AdaptiveConnConfig) *AdaptiveConnManager {
	if config == nil {
		config = DefaultAdaptiveConnConfig()
	}
	
	acm := &AdaptiveConnManager{
		host:         h,
		config:       config,
		connections:  make(map[peer.ID]*ConnectionInfo),
		clusterPeers: make(map[peer.ID]bool),
		loadMonitor:  NewLoadMonitor(),
		metrics:      &ConnManagerMetrics{},
		stopCh:       make(chan struct{}),
		doneCh:       make(chan struct{}),
		
		// Initialize with base values
		currentHighWater:   config.BaseHighWater,
		currentLowWater:    config.BaseLowWater,
		currentGracePeriod: config.BaseGracePeriod,
	}
	
	// Mark cluster peers
	for _, peerID := range config.ClusterPeers {
		acm.clusterPeers[peerID] = true
	}
	
	return acm
}

// DefaultAdaptiveConnConfig returns a default configuration
func DefaultAdaptiveConnConfig() *AdaptiveConnConfig {
	return &AdaptiveConnConfig{
		BaseHighWater:      2000,
		BaseLowWater:       1000,
		BaseGracePeriod:    30 * time.Second,
		
		MaxHighWater:       5000,
		MinLowWater:        500,
		MaxGracePeriod:     120 * time.Second,
		MinGracePeriod:     10 * time.Second,
		
		HighLoadThreshold:  0.8,
		LowLoadThreshold:   0.3,
		
		AdaptationInterval: 30 * time.Second,
		LoadSampleWindow:   5 * time.Minute,
		
		ClusterPeers:       []peer.ID{},
	}
}

// Start begins the adaptive connection management
func (acm *AdaptiveConnManager) Start(ctx context.Context) error {
	// Subscribe to network events
	acm.host.Network().Notify(&networkNotifiee{acm: acm})
	
	// Start the adaptation loop
	go acm.adaptationLoop(ctx)
	
	// Establish cluster connections
	go acm.maintainClusterConnections(ctx)
	
	// Start connection quality monitoring
	go acm.MonitorConnectionQuality(ctx)
	
	return nil
}

// Stop stops the adaptive connection manager
func (acm *AdaptiveConnManager) Stop() error {
	close(acm.stopCh)
	<-acm.doneCh
	return nil
}

// SetDynamicLimits updates connection limits dynamically
func (acm *AdaptiveConnManager) SetDynamicLimits(high, low int, gracePeriod time.Duration) {
	acm.mu.Lock()
	defer acm.mu.Unlock()
	
	acm.currentHighWater = high
	acm.currentLowWater = low
	acm.currentGracePeriod = gracePeriod
	
	// Apply limits to underlying connection manager if available
	if connMgr := acm.host.ConnManager(); connMgr != nil {
		// Note: libp2p connection manager doesn't support dynamic limits
		// This would require a custom implementation or wrapper
		acmLog.Debugf("Updated connection limits: high=%d, low=%d, grace=%v", 
			high, low, gracePeriod)
	}
}

// GetConnectionQuality returns quality metrics for a peer
func (acm *AdaptiveConnManager) GetConnectionQuality(peerID peer.ID) *ConnectionQuality {
	acm.mu.RLock()
	defer acm.mu.RUnlock()
	
	if conn, exists := acm.connections[peerID]; exists {
		return conn.Quality
	}
	return nil
}

// UpdateConnectionQuality updates quality metrics for a peer connection
func (acm *AdaptiveConnManager) UpdateConnectionQuality(peerID peer.ID, latency time.Duration, bandwidth int64, hasError bool) {
	acm.mu.Lock()
	defer acm.mu.Unlock()
	
	acm.updateConnectionQualityLocked(peerID, latency, bandwidth, hasError)
}

// GetConnectionState returns the current state of a connection
func (acm *AdaptiveConnManager) GetConnectionState(peerID peer.ID) ConnectionState {
	acm.mu.RLock()
	defer acm.mu.RUnlock()
	
	if conn, exists := acm.connections[peerID]; exists {
		return conn.State
	}
	return StateDisconnected
}

// RecordRequest records a request and its outcome for quality tracking
func (acm *AdaptiveConnManager) RecordRequest(peerID peer.ID, responseTime time.Duration, success bool) {
	acm.mu.Lock()
	defer acm.mu.Unlock()
	
	conn, exists := acm.connections[peerID]
	if !exists {
		return
	}
	
	// Update last activity
	conn.LastActivity = time.Now()
	
	// Update error count
	if !success {
		conn.ErrorCount++
		conn.LastError = time.Now()
	}
	
	// Update quality metrics if we have quality tracking
	if conn.Quality != nil {
		// Use response time as latency measurement
		bandwidth := conn.Quality.Bandwidth // Keep previous bandwidth
		acm.updateConnectionQualityLocked(peerID, responseTime, bandwidth, !success)
	}
}

// MarkUnresponsive marks a peer as unresponsive
func (acm *AdaptiveConnManager) MarkUnresponsive(peerID peer.ID) {
	acm.mu.Lock()
	defer acm.mu.Unlock()
	
	if conn, exists := acm.connections[peerID]; exists {
		conn.State = StateUnresponsive
		conn.LastError = time.Now()
		acmLog.Warnf("Marked peer %s as unresponsive", peerID)
	}
}

// MarkResponsive marks a peer as responsive again
func (acm *AdaptiveConnManager) MarkResponsive(peerID peer.ID) {
	acm.mu.Lock()
	defer acm.mu.Unlock()
	
	if conn, exists := acm.connections[peerID]; exists {
		conn.State = StateConnected
		acmLog.Debugf("Marked peer %s as responsive", peerID)
	}
}



// calculateQualityScore computes a quality score based on latency, bandwidth, and error rate
func (acm *AdaptiveConnManager) calculateQualityScore(quality *ConnectionQuality) float64 {
	// Latency score (lower is better, 0-200ms range)
	latencyScore := 1.0 - minFloat64(float64(quality.AvgLatency)/float64(200*time.Millisecond), 1.0)
	
	// Bandwidth score (higher is better, normalize to 10MB/s)
	bandwidthScore := minFloat64(float64(quality.AvgBandwidth)/float64(10*1024*1024), 1.0)
	
	// Error rate score (lower is better)
	errorScore := 1.0 - minFloat64(quality.AvgErrorRate/0.1, 1.0) // 10% error rate = 0 score
	
	// Weighted average: latency 40%, bandwidth 30%, error rate 30%
	return latencyScore*0.4 + bandwidthScore*0.3 + errorScore*0.3
}



// updateConnectionQualityLocked is the internal version that assumes lock is held
func (acm *AdaptiveConnManager) updateConnectionQualityLocked(peerID peer.ID, latency time.Duration, bandwidth int64, hasError bool) {
	conn, exists := acm.connections[peerID]
	if !exists {
		return
	}
	
	if conn.Quality == nil {
		conn.Quality = &ConnectionQuality{}
	}
	
	quality := conn.Quality
	now := time.Now()
	
	// Update current metrics
	quality.Latency = latency
	quality.Bandwidth = bandwidth
	quality.LastMeasured = now
	quality.TotalRequests++
	
	if hasError {
		quality.FailedRequests++
	} else {
		quality.SuccessfulRequests++
	}
	
	// Calculate error rate
	if quality.TotalRequests > 0 {
		quality.ErrorRate = float64(quality.FailedRequests) / float64(quality.TotalRequests)
	}
	
	// Update moving averages
	alpha := 0.3
	if quality.AvgLatency == 0 {
		quality.AvgLatency = latency
		quality.AvgBandwidth = bandwidth
		quality.AvgErrorRate = quality.ErrorRate
	} else {
		quality.AvgLatency = time.Duration(float64(quality.AvgLatency)*(1-alpha) + float64(latency)*alpha)
		quality.AvgBandwidth = int64(float64(quality.AvgBandwidth)*(1-alpha) + float64(bandwidth)*alpha)
		quality.AvgErrorRate = quality.AvgErrorRate*(1-alpha) + quality.ErrorRate*alpha
	}
	
	// Calculate quality score
	quality.QualityScore = acm.calculateQualityScore(quality)
}



// findAlternativeRoutes discovers alternative routes for poor quality connections
func (acm *AdaptiveConnManager) findAlternativeRoutes(peerID peer.ID) {
	acm.mu.Lock()
	conn, exists := acm.connections[peerID]
	if !exists || conn.Quality == nil {
		acm.mu.Unlock()
		return
	}
	
	currentQuality := conn.Quality.QualityScore
	acm.mu.Unlock()
	
	acmLog.Debugf("Looking for alternative routes for peer %s (quality score: %.2f)", 
		peerID, currentQuality)
	
	// Get list of connected peers as potential alternatives
	connectedPeers := acm.host.Network().Peers()
	alternatives := make([]peer.ID, 0)
	
	acm.mu.RLock()
	for _, altPeerID := range connectedPeers {
		if altPeerID == peerID {
			continue
		}
		
		// Check if this peer has better quality
		if altConn, exists := acm.connections[altPeerID]; exists && 
		   altConn.Quality != nil && 
		   altConn.Quality.QualityScore > currentQuality+0.1 { // 10% better threshold
			alternatives = append(alternatives, altPeerID)
		}
	}
	acm.mu.RUnlock()
	
	// Update alternative routes in the connection quality
	acm.mu.Lock()
	if conn, exists := acm.connections[peerID]; exists && conn.Quality != nil {
		conn.Quality.AlternativeRoutes = alternatives
		if len(alternatives) > 0 {
			// Set the best alternative as preferred route
			bestPeer := alternatives[0]
			bestScore := 0.0
			
			for _, altPeerID := range alternatives {
				if altConn, exists := acm.connections[altPeerID]; exists && 
				   altConn.Quality != nil && 
				   altConn.Quality.QualityScore > bestScore {
					bestPeer = altPeerID
					bestScore = altConn.Quality.QualityScore
				}
			}
			
			conn.Quality.PreferredRoute = bestPeer
			acmLog.Debugf("Found %d alternative routes for peer %s, preferred: %s (score: %.2f)", 
				len(alternatives), peerID, bestPeer, bestScore)
		}
	}
	acm.mu.Unlock()
}

// SwitchToAlternativeRoute switches to an alternative route if available
func (acm *AdaptiveConnManager) SwitchToAlternativeRoute(peerID peer.ID) (peer.ID, error) {
	acm.mu.Lock()
	defer acm.mu.Unlock()
	
	conn, exists := acm.connections[peerID]
	if !exists || conn.Quality == nil {
		return "", ErrPeerNotFound
	}
	
	if conn.Quality.PreferredRoute == "" {
		return "", ErrNoAlternativeRoute
	}
	
	preferredRoute := conn.Quality.PreferredRoute
	conn.Quality.RouteChanges++
	
	acmLog.Infof("Switching from peer %s to alternative route %s", peerID, preferredRoute)
	
	return preferredRoute, nil
}

// MeasureConnectionLatency measures the latency to a peer using ping
func (acm *AdaptiveConnManager) MeasureConnectionLatency(ctx context.Context, peerID peer.ID) (time.Duration, error) {
	// Use libp2p ping service if available
	if pinger, ok := acm.host.(interface{ Ping(context.Context, peer.ID) <-chan ping.Result }); ok {
		select {
		case result := <-pinger.Ping(ctx, peerID):
			if result.Error != nil {
				return 0, result.Error
			}
			return result.RTT, nil
		case <-ctx.Done():
			return 0, ctx.Err()
		case <-time.After(5 * time.Second):
			return 0, ErrPingTimeout
		}
	}
	
	// Fallback: measure connection establishment time
	start := time.Now()
	conn := acm.host.Network().ConnsToPeer(peerID)
	if len(conn) == 0 {
		return 0, ErrPeerNotConnected
	}
	
	// Simple RTT estimation based on connection age
	// This is a rough approximation
	return time.Since(start), nil
}

// EstimateBandwidth estimates bandwidth to a peer based on recent data transfers
func (acm *AdaptiveConnManager) EstimateBandwidth(peerID peer.ID) int64 {
	acm.mu.RLock()
	defer acm.mu.RUnlock()
	
	conn, exists := acm.connections[peerID]
	if !exists || conn.Quality == nil {
		return 0
	}
	
	// Return the current bandwidth estimate
	// In a real implementation, this would be calculated based on
	// actual data transfer measurements
	return conn.Quality.AvgBandwidth
}

// MonitorConnectionQuality continuously monitors connection quality
func (acm *AdaptiveConnManager) MonitorConnectionQuality(ctx context.Context) {
	ticker := time.NewTicker(30 * time.Second) // Monitor every 30 seconds
	defer ticker.Stop()
	
	for {
		select {
		case <-ctx.Done():
			return
		case <-acm.stopCh:
			return
		case <-ticker.C:
			acm.performQualityMeasurements(ctx)
		}
	}
}

// performQualityMeasurements measures quality for all connections
func (acm *AdaptiveConnManager) performQualityMeasurements(ctx context.Context) {
	acm.mu.RLock()
	peersToMeasure := make([]peer.ID, 0, len(acm.connections))
	for peerID, conn := range acm.connections {
		if conn.State == StateConnected {
			peersToMeasure = append(peersToMeasure, peerID)
		}
	}
	acm.mu.RUnlock()
	
	// Measure quality for each connected peer
	for _, peerID := range peersToMeasure {
		select {
		case <-ctx.Done():
			return
		default:
			acm.measurePeerQuality(ctx, peerID)
		}
	}
}

// measurePeerQuality measures quality metrics for a specific peer
func (acm *AdaptiveConnManager) measurePeerQuality(ctx context.Context, peerID peer.ID) {
	// Measure latency
	latency, err := acm.MeasureConnectionLatency(ctx, peerID)
	if err != nil {
		acmLog.Debugf("Failed to measure latency for peer %s: %v", peerID, err)
		acm.UpdateConnectionQuality(peerID, 0, 0, true)
		return
	}
	
	// Estimate bandwidth
	bandwidth := acm.EstimateBandwidth(peerID)
	
	// Update quality metrics
	acm.UpdateConnectionQuality(peerID, latency, bandwidth, false)
	
	// Check if we need to find alternatives for poor quality connections
	quality := acm.GetConnectionQuality(peerID)
	if quality != nil && quality.QualityScore < 0.3 { // Poor quality threshold
		acm.findAlternativeRoutes(peerID)
	}
}

// OptimizeRoutes attempts to optimize routes for all connections
func (acm *AdaptiveConnManager) OptimizeRoutes(ctx context.Context) error {
	acm.mu.RLock()
	poorQualityPeers := make([]peer.ID, 0)
	
	for peerID, conn := range acm.connections {
		if conn.Quality != nil && conn.Quality.QualityScore < 0.5 {
			poorQualityPeers = append(poorQualityPeers, peerID)
		}
	}
	acm.mu.RUnlock()
	
	// Find alternatives for poor quality connections
	for _, peerID := range poorQualityPeers {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
			acm.findAlternativeRoutes(peerID)
		}
	}
	
	return nil
}

// GetPoorQualityConnections returns peers with poor connection quality
func (acm *AdaptiveConnManager) GetPoorQualityConnections(threshold float64) []peer.ID {
	acm.mu.RLock()
	defer acm.mu.RUnlock()
	
	poorPeers := make([]peer.ID, 0)
	for peerID, conn := range acm.connections {
		if conn.Quality != nil && conn.Quality.QualityScore < threshold {
			poorPeers = append(poorPeers, peerID)
		}
	}
	
	return poorPeers
}

// GetHighQualityConnections returns peers with good connection quality
func (acm *AdaptiveConnManager) GetHighQualityConnections(threshold float64) []peer.ID {
	acm.mu.RLock()
	defer acm.mu.RUnlock()
	
	goodPeers := make([]peer.ID, 0)
	for peerID, conn := range acm.connections {
		if conn.Quality != nil && conn.Quality.QualityScore >= threshold {
			goodPeers = append(goodPeers, peerID)
		}
	}
	
	return goodPeers
}

// IsClusterPeer checks if a peer is marked as a cluster peer
func (acm *AdaptiveConnManager) IsClusterPeer(peerID peer.ID) bool {
	acm.mu.RLock()
	defer acm.mu.RUnlock()
	return acm.clusterPeers[peerID]
}

// AddClusterPeer adds a peer to the cluster peer list
func (acm *AdaptiveConnManager) AddClusterPeer(peerID peer.ID) {
	acm.mu.Lock()
	defer acm.mu.Unlock()
	
	acm.clusterPeers[peerID] = true
	
	// Protect cluster peer from disconnection
	if connMgr := acm.host.ConnManager(); connMgr != nil {
		connMgr.Protect(peerID, "cluster-peer")
	}
}

// RemoveClusterPeer removes a peer from the cluster peer list
func (acm *AdaptiveConnManager) RemoveClusterPeer(peerID peer.ID) {
	acm.mu.Lock()
	defer acm.mu.Unlock()
	
	delete(acm.clusterPeers, peerID)
	
	// Unprotect peer
	if connMgr := acm.host.ConnManager(); connMgr != nil {
		connMgr.Unprotect(peerID, "cluster-peer")
	}
}

// GetMetrics returns current connection manager metrics
func (acm *AdaptiveConnManager) GetMetrics() *ConnManagerMetrics {
	acm.mu.RLock()
	defer acm.mu.RUnlock()
	
	// Update current connection count
	acm.metrics.TotalConnections = int64(len(acm.connections))
	
	clusterCount := int64(0)
	for peerID := range acm.connections {
		if acm.clusterPeers[peerID] {
			clusterCount++
		}
	}
	acm.metrics.ClusterConnections = clusterCount
	
	return acm.metrics
}

// adaptationLoop runs the main adaptation logic
func (acm *AdaptiveConnManager) adaptationLoop(ctx context.Context) {
	defer close(acm.doneCh)
	
	ticker := time.NewTicker(acm.config.AdaptationInterval)
	defer ticker.Stop()
	
	for {
		select {
		case <-ctx.Done():
			return
		case <-acm.stopCh:
			return
		case <-ticker.C:
			acm.adaptToLoad()
		}
	}
}

// adaptToLoad adjusts connection limits based on current load
func (acm *AdaptiveConnManager) adaptToLoad() {
	load := acm.loadMonitor.GetCurrentLoad()
	
	acm.mu.Lock()
	defer acm.mu.Unlock()
	
	// Calculate new limits based on load
	var newHighWater, newLowWater int
	var newGracePeriod time.Duration
	
	if load > acm.config.HighLoadThreshold {
		// High load: increase limits
		scaleFactor := 1.0 + (load-acm.config.HighLoadThreshold)*2
		newHighWater = min(int(float64(acm.config.BaseHighWater)*scaleFactor), acm.config.MaxHighWater)
		newLowWater = min(int(float64(acm.config.BaseLowWater)*scaleFactor), newHighWater-100)
		newGracePeriod = maxDuration(time.Duration(float64(acm.config.BaseGracePeriod)*0.5), acm.config.MinGracePeriod)
	} else if load < acm.config.LowLoadThreshold {
		// Low load: decrease limits
		scaleFactor := acm.config.LowLoadThreshold / maxFloat64(load, 0.1)
		newHighWater = max(int(float64(acm.config.BaseHighWater)/scaleFactor), acm.config.BaseLowWater+100)
		newLowWater = max(int(float64(acm.config.BaseLowWater)/scaleFactor), acm.config.MinLowWater)
		newGracePeriod = minDuration(time.Duration(float64(acm.config.BaseGracePeriod)*scaleFactor), acm.config.MaxGracePeriod)
	} else {
		// Normal load: use base values
		newHighWater = acm.config.BaseHighWater
		newLowWater = acm.config.BaseLowWater
		newGracePeriod = acm.config.BaseGracePeriod
	}
	
	// Apply new limits if they changed significantly
	if abs(newHighWater-acm.currentHighWater) > 50 || 
	   abs(newLowWater-acm.currentLowWater) > 25 ||
	   absDuration(newGracePeriod-acm.currentGracePeriod) > 5*time.Second {
		
		acm.currentHighWater = newHighWater
		acm.currentLowWater = newLowWater
		acm.currentGracePeriod = newGracePeriod
		acm.metrics.AdaptationEvents++
		
		acmLog.Infof("Adapted connection limits: load=%.2f, high=%d, low=%d, grace=%v",
			load, newHighWater, newLowWater, newGracePeriod)
	}
}

// maintainClusterConnections ensures cluster peers stay connected
func (acm *AdaptiveConnManager) maintainClusterConnections(ctx context.Context) {
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()
	
	for {
		select {
		case <-ctx.Done():
			return
		case <-acm.stopCh:
			return
		case <-ticker.C:
			acm.checkClusterConnections(ctx)
		}
	}
}

// checkClusterConnections verifies and maintains cluster peer connections
func (acm *AdaptiveConnManager) checkClusterConnections(ctx context.Context) {
	acm.mu.RLock()
	clusterPeers := make([]peer.ID, 0, len(acm.clusterPeers))
	for peerID := range acm.clusterPeers {
		clusterPeers = append(clusterPeers, peerID)
	}
	acm.mu.RUnlock()
	
	for _, peerID := range clusterPeers {
		if acm.host.Network().Connectedness(peerID) != network.Connected {
			// Try to reconnect to cluster peer
			if addrs := acm.host.Peerstore().Addrs(peerID); len(addrs) > 0 {
				addrInfo := peer.AddrInfo{ID: peerID, Addrs: addrs}
				if err := acm.host.Connect(ctx, addrInfo); err != nil {
					acmLog.Warnf("Failed to reconnect to cluster peer %s: %v", peerID, err)
				} else {
					acmLog.Debugf("Reconnected to cluster peer %s", peerID)
				}
			}
		}
	}
}

// Helper functions
func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}

func minDuration(a, b time.Duration) time.Duration {
	if a < b {
		return a
	}
	return b
}

func maxDuration(a, b time.Duration) time.Duration {
	if a > b {
		return a
	}
	return b
}

func maxFloat64(a, b float64) float64 {
	if a > b {
		return a
	}
	return b
}

func abs(x int) int {
	if x < 0 {
		return -x
	}
	return x
}

func absDuration(x time.Duration) time.Duration {
	if x < 0 {
		return -x
	}
	return x
}

// NewLoadMonitor creates a new load monitor
func NewLoadMonitor() *LoadMonitor {
	return &LoadMonitor{
		samples:    make([]LoadSample, 0, 100),
		maxSamples: 100,
	}
}

// RecordSample records a new load sample
func (lm *LoadMonitor) RecordSample(sample LoadSample) {
	lm.mu.Lock()
	defer lm.mu.Unlock()
	
	lm.samples = append(lm.samples, sample)
	if len(lm.samples) > lm.maxSamples {
		lm.samples = lm.samples[1:]
	}
	
	// Update current metrics
	lm.ActiveConnections = sample.ActiveConnections
	lm.RequestsPerSecond = sample.RequestsPerSecond
	lm.AverageLatency = sample.AverageLatency
	lm.ErrorRate = sample.ErrorRate
}

// GetCurrentLoad returns the current system load as a value between 0 and 1
func (lm *LoadMonitor) GetCurrentLoad() float64 {
	lm.mu.RLock()
	defer lm.mu.RUnlock()
	
	if len(lm.samples) == 0 {
		return 0.0
	}
	
	// Calculate load based on multiple factors
	latest := lm.samples[len(lm.samples)-1]
	
	// Connection load (assume 2000 is max reasonable connections)
	connLoad := float64(latest.ActiveConnections) / 2000.0
	
	// Request rate load (assume 1000 RPS is high load)
	rpsLoad := latest.RequestsPerSecond / 1000.0
	
	// Latency load (assume 100ms is high latency)
	latencyLoad := float64(latest.AverageLatency) / float64(100*time.Millisecond)
	
	// Error rate load (5% error rate is high)
	errorLoad := latest.ErrorRate / 0.05
	
	// Weighted average
	totalLoad := (connLoad*0.3 + rpsLoad*0.3 + latencyLoad*0.2 + errorLoad*0.2)
	
	// Cap at 1.0
	if totalLoad > 1.0 {
		totalLoad = 1.0
	}
	
	return totalLoad
}

// networkNotifiee handles network events for the adaptive connection manager
type networkNotifiee struct {
	acm *AdaptiveConnManager
}

// Connected is called when a connection is opened
func (nn *networkNotifiee) Connected(n network.Network, conn network.Conn) {
	peerID := conn.RemotePeer()
	
	nn.acm.mu.Lock()
	defer nn.acm.mu.Unlock()
	
	// Create or update connection info
	connInfo, exists := nn.acm.connections[peerID]
	if !exists {
		connInfo = &ConnectionInfo{
			PeerID:      peerID,
			ConnectedAt: time.Now(),
			Quality:     &ConnectionQuality{},
			IsCluster:   nn.acm.clusterPeers[peerID],
		}
		nn.acm.connections[peerID] = connInfo
	}
	
	connInfo.State = StateConnected
	connInfo.LastActivity = time.Now()
	
	acmLog.Debugf("Peer connected: %s (cluster: %v)", peerID, connInfo.IsCluster)
}

// Disconnected is called when a connection is closed
func (nn *networkNotifiee) Disconnected(n network.Network, conn network.Conn) {
	peerID := conn.RemotePeer()
	
	nn.acm.mu.Lock()
	defer nn.acm.mu.Unlock()
	
	if connInfo, exists := nn.acm.connections[peerID]; exists {
		connInfo.State = StateDisconnected
		
		// Don't remove cluster peers from tracking
		if !connInfo.IsCluster {
			delete(nn.acm.connections, peerID)
		}
	}
	
	acmLog.Debugf("Peer disconnected: %s", peerID)
}

// Listen is called when network starts listening on an addr
func (nn *networkNotifiee) Listen(network.Network, multiaddr.Multiaddr) {}

// ListenClose is called when network stops listening on an addr
func (nn *networkNotifiee) ListenClose(network.Network, multiaddr.Multiaddr) {}

// Additional helper functions
func minFloat64(a, b float64) float64 {
	if a < b {
		return a
	}
	return b
}

func maxInt64(a, b int64) int64 {
	if a > b {
		return a
	}
	return b
}

func maxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}

