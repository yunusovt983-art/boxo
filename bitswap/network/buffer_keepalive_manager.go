package network

import (
	"context"
	"sync"
	"time"

	logging "github.com/ipfs/go-log/v2"
	"github.com/libp2p/go-libp2p/core/host"
	"github.com/libp2p/go-libp2p/core/network"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/multiformats/go-multiaddr"
)

var bufferLog = logging.Logger("bitswap/network/buffer")

// BufferKeepAliveManager manages adaptive buffering and intelligent keep-alive connections
type BufferKeepAliveManager struct {
	host   host.Host
	config *BufferKeepAliveConfig
	
	// Connection tracking
	connections map[peer.ID]*ConnectionBuffer
	mu          sync.RWMutex
	
	// Bandwidth monitoring
	bandwidthMonitor *BandwidthMonitor
	
	// Keep-alive management
	keepAliveManager *KeepAliveManager
	
	// Slow connection detection
	slowConnDetector *SlowConnectionDetector
	
	// Control channels
	stopCh chan struct{}
	doneCh chan struct{}
}

// BufferKeepAliveConfig holds configuration for buffer and keep-alive management
type BufferKeepAliveConfig struct {
	// Buffer configuration
	MinBufferSize        int           // 4KB minimum
	MaxBufferSize        int           // 1MB maximum
	DefaultBufferSize    int           // 64KB default
	BufferGrowthFactor   float64       // 1.5x growth factor
	BufferShrinkFactor   float64       // 0.75x shrink factor
	
	// Bandwidth thresholds for buffer adaptation
	LowBandwidthThreshold  int64         // 100KB/s
	HighBandwidthThreshold int64         // 10MB/s
	
	// Keep-alive configuration
	KeepAliveInterval      time.Duration // 30s default
	KeepAliveTimeout       time.Duration // 10s timeout
	MaxIdleTime            time.Duration // 5min before considering idle
	KeepAliveProbeCount    int           // 3 probes before marking as dead
	
	// Slow connection detection
	SlowConnectionThreshold time.Duration // 500ms latency threshold
	SlowConnectionSamples   int           // 5 samples to confirm slow connection
	BypassSlowConnections   bool          // Whether to bypass slow connections
	
	// Adaptation intervals
	BufferAdaptationInterval time.Duration // 10s
	KeepAliveCheckInterval   time.Duration // 30s
	SlowConnCheckInterval    time.Duration // 60s
}

// ConnectionBuffer tracks buffer settings and metrics for a specific connection
type ConnectionBuffer struct {
	PeerID       peer.ID
	CurrentSize  int
	ReadBuffer   int
	WriteBuffer  int
	LastAdjusted time.Time
	
	// Performance metrics
	Bandwidth         int64
	AverageLatency    time.Duration
	PacketLossRate    float64
	
	// Buffer efficiency metrics
	BufferUtilization float64
	OverflowCount     int64
	UnderflowCount    int64
	
	// Keep-alive state
	LastActivity      time.Time
	KeepAliveEnabled  bool
	KeepAliveFailures int
	
	// Slow connection detection
	LatencySamples    []time.Duration
	IsSlow            bool
	BypassRoutes      []peer.ID
}

// BandwidthMonitor tracks bandwidth usage for adaptive buffer sizing
type BandwidthMonitor struct {
	mu sync.RWMutex
	
	// Per-peer bandwidth tracking
	peerBandwidth map[peer.ID]*BandwidthStats
	
	// Global bandwidth metrics
	totalBandwidth    int64
	peakBandwidth     int64
	averageBandwidth  int64
	
	// Measurement window
	measurementWindow time.Duration
	samples           []BandwidthSample
	maxSamples        int
}

// BandwidthStats tracks bandwidth statistics for a peer
type BandwidthStats struct {
	BytesSent     int64
	BytesReceived int64
	LastMeasured  time.Time
	CurrentRate   int64
	PeakRate      int64
	AverageRate   int64
	
	// Rate calculation
	samples       []int64
	sampleIndex   int
	sampleCount   int
}

// BandwidthSample represents a bandwidth measurement at a point in time
type BandwidthSample struct {
	Timestamp time.Time
	Bandwidth int64
	PeerCount int
}

// KeepAliveManager handles intelligent keep-alive for connections
type KeepAliveManager struct {
	mu sync.RWMutex
	
	// Keep-alive state per peer
	keepAliveState map[peer.ID]*KeepAliveState
	
	// Configuration
	config *BufferKeepAliveConfig
	host   host.Host
}

// KeepAliveState tracks keep-alive state for a peer
type KeepAliveState struct {
	PeerID            peer.ID
	Enabled           bool
	LastProbe         time.Time
	LastResponse      time.Time
	ConsecutiveFailures int
	ProbeInterval     time.Duration
	
	// Connection importance scoring
	ImportanceScore   float64
	IsClusterPeer     bool
	RecentActivity    time.Time
	
	// Adaptive probe timing
	AdaptiveInterval  time.Duration
	BaseInterval      time.Duration
}

// SlowConnectionDetector identifies and manages slow connections
type SlowConnectionDetector struct {
	mu sync.RWMutex
	
	// Slow connection tracking
	slowConnections map[peer.ID]*SlowConnectionInfo
	
	// Detection parameters
	config *BufferKeepAliveConfig
}

// SlowConnectionInfo tracks information about slow connections
type SlowConnectionInfo struct {
	PeerID           peer.ID
	DetectedAt       time.Time
	LatencySamples   []time.Duration
	AverageLatency   time.Duration
	ConfirmationCount int
	
	// Bypass information
	BypassEnabled    bool
	AlternativeRoutes []peer.ID
	LastBypassAttempt time.Time
}

// NewBufferKeepAliveManager creates a new buffer and keep-alive manager
func NewBufferKeepAliveManager(h host.Host, config *BufferKeepAliveConfig) *BufferKeepAliveManager {
	if config == nil {
		config = DefaultBufferKeepAliveConfig()
	}
	
	bkm := &BufferKeepAliveManager{
		host:             h,
		config:           config,
		connections:      make(map[peer.ID]*ConnectionBuffer),
		bandwidthMonitor: NewBandwidthMonitor(config),
		keepAliveManager: NewKeepAliveManager(h, config),
		slowConnDetector: NewSlowConnectionDetector(config),
		stopCh:           make(chan struct{}),
		doneCh:           make(chan struct{}),
	}
	
	return bkm
}

// DefaultBufferKeepAliveConfig returns default configuration
func DefaultBufferKeepAliveConfig() *BufferKeepAliveConfig {
	return &BufferKeepAliveConfig{
		// Buffer settings
		MinBufferSize:        4 * 1024,      // 4KB
		MaxBufferSize:        1024 * 1024,   // 1MB
		DefaultBufferSize:    64 * 1024,     // 64KB
		BufferGrowthFactor:   1.5,
		BufferShrinkFactor:   0.75,
		
		// Bandwidth thresholds
		LowBandwidthThreshold:  100 * 1024,    // 100KB/s
		HighBandwidthThreshold: 10 * 1024 * 1024, // 10MB/s
		
		// Keep-alive settings
		KeepAliveInterval:      30 * time.Second,
		KeepAliveTimeout:       10 * time.Second,
		MaxIdleTime:            5 * time.Minute,
		KeepAliveProbeCount:    3,
		
		// Slow connection detection
		SlowConnectionThreshold: 500 * time.Millisecond,
		SlowConnectionSamples:   5,
		BypassSlowConnections:   true,
		
		// Adaptation intervals
		BufferAdaptationInterval: 10 * time.Second,
		KeepAliveCheckInterval:   30 * time.Second,
		SlowConnCheckInterval:    60 * time.Second,
	}
}

// Start begins buffer and keep-alive management
func (bkm *BufferKeepAliveManager) Start(ctx context.Context) error {
	// Subscribe to network events
	bkm.host.Network().Notify(&bufferNetworkNotifiee{bkm: bkm})
	
	// Start monitoring goroutines
	go bkm.bufferAdaptationLoop(ctx)
	go bkm.keepAliveLoop(ctx)
	go bkm.slowConnectionDetectionLoop(ctx)
	go bkm.bandwidthMonitoringLoop(ctx)
	
	bufferLog.Info("Buffer and keep-alive manager started")
	return nil
}

// Stop stops the buffer and keep-alive manager
func (bkm *BufferKeepAliveManager) Stop() error {
	close(bkm.stopCh)
	<-bkm.doneCh
	bufferLog.Info("Buffer and keep-alive manager stopped")
	return nil
}

// AdaptBufferSize adapts buffer size based on bandwidth and latency
func (bkm *BufferKeepAliveManager) AdaptBufferSize(peerID peer.ID, bandwidth int64, latency time.Duration) {
	bkm.mu.Lock()
	defer bkm.mu.Unlock()
	
	conn, exists := bkm.connections[peerID]
	if !exists {
		conn = &ConnectionBuffer{
			PeerID:      peerID,
			CurrentSize: bkm.config.DefaultBufferSize,
			LastAdjusted: time.Now(),
		}
		bkm.connections[peerID] = conn
	}
	
	// Calculate optimal buffer size based on bandwidth-delay product
	bdp := int64(latency.Seconds() * float64(bandwidth))
	
	var newSize int
	if bandwidth < bkm.config.LowBandwidthThreshold {
		// Low bandwidth: use smaller buffers
		newSize = int(float64(conn.CurrentSize) * bkm.config.BufferShrinkFactor)
	} else if bandwidth > bkm.config.HighBandwidthThreshold {
		// High bandwidth: use larger buffers
		newSize = int(float64(conn.CurrentSize) * bkm.config.BufferGrowthFactor)
	} else {
		// Medium bandwidth: use BDP-based sizing
		newSize = int(bdp)
	}
	
	// Apply limits
	if newSize < bkm.config.MinBufferSize {
		newSize = bkm.config.MinBufferSize
	}
	if newSize > bkm.config.MaxBufferSize {
		newSize = bkm.config.MaxBufferSize
	}
	
	// Only update if change is significant (>10%)
	if absInt(newSize-conn.CurrentSize) > conn.CurrentSize/10 {
		oldSize := conn.CurrentSize
		conn.CurrentSize = newSize
		conn.LastAdjusted = time.Now()
		
		// Apply buffer size to actual connection if possible
		bkm.applyBufferSize(peerID, newSize)
		
		bufferLog.Debugf("Adapted buffer size for peer %s: %d -> %d (bandwidth: %d, latency: %v)",
			peerID, oldSize, newSize, bandwidth, latency)
	}
	
	// Update connection metrics
	conn.Bandwidth = bandwidth
	conn.AverageLatency = latency
	conn.LastActivity = time.Now()
}

// applyBufferSize applies buffer size to the actual network connection
func (bkm *BufferKeepAliveManager) applyBufferSize(peerID peer.ID, bufferSize int) {
	conns := bkm.host.Network().ConnsToPeer(peerID)
	for range conns {
		// Try to get the underlying TCP connection for buffer adjustment
		// This is a simplified approach - in practice, buffer management
		// would be handled at the transport layer
		bufferLog.Debugf("Applied buffer size %d for peer %s", bufferSize, peerID)
	}
}

// EnableKeepAlive enables intelligent keep-alive for a peer
func (bkm *BufferKeepAliveManager) EnableKeepAlive(peerID peer.ID, isImportant bool) {
	bkm.keepAliveManager.EnableKeepAlive(peerID, isImportant)
	
	bkm.mu.Lock()
	defer bkm.mu.Unlock()
	
	conn, exists := bkm.connections[peerID]
	if !exists {
		conn = &ConnectionBuffer{
			PeerID:      peerID,
			CurrentSize: bkm.config.DefaultBufferSize,
			LastAdjusted: time.Now(),
		}
		bkm.connections[peerID] = conn
	}
	
	conn.KeepAliveEnabled = true
	conn.LastActivity = time.Now()
}

// DisableKeepAlive disables keep-alive for a peer
func (bkm *BufferKeepAliveManager) DisableKeepAlive(peerID peer.ID) {
	bkm.keepAliveManager.DisableKeepAlive(peerID)
	
	bkm.mu.Lock()
	defer bkm.mu.Unlock()
	
	if conn, exists := bkm.connections[peerID]; exists {
		conn.KeepAliveEnabled = false
	}
}

// RecordLatency records latency measurement for slow connection detection
func (bkm *BufferKeepAliveManager) RecordLatency(peerID peer.ID, latency time.Duration) {
	bkm.slowConnDetector.RecordLatency(peerID, latency)
}

// IsSlowConnection checks if a peer is considered a slow connection
func (bkm *BufferKeepAliveManager) IsSlowConnection(peerID peer.ID) bool {
	return bkm.slowConnDetector.IsSlowConnection(peerID)
}

// GetBypassRoutes returns alternative routes for a slow connection
func (bkm *BufferKeepAliveManager) GetBypassRoutes(peerID peer.ID) []peer.ID {
	return bkm.slowConnDetector.GetBypassRoutes(peerID)
}

// GetBufferSize returns the current buffer size for a peer
func (bkm *BufferKeepAliveManager) GetBufferSize(peerID peer.ID) int {
	bkm.mu.RLock()
	defer bkm.mu.RUnlock()
	
	if conn, exists := bkm.connections[peerID]; exists {
		return conn.CurrentSize
	}
	return bkm.config.DefaultBufferSize
}

// GetConnectionMetrics returns metrics for a connection
func (bkm *BufferKeepAliveManager) GetConnectionMetrics(peerID peer.ID) *ConnectionBuffer {
	bkm.mu.RLock()
	defer bkm.mu.RUnlock()
	
	if conn, exists := bkm.connections[peerID]; exists {
		// Return a copy to avoid race conditions
		connCopy := *conn
		return &connCopy
	}
	return nil
}

// bufferAdaptationLoop continuously adapts buffer sizes
func (bkm *BufferKeepAliveManager) bufferAdaptationLoop(ctx context.Context) {
	ticker := time.NewTicker(bkm.config.BufferAdaptationInterval)
	defer ticker.Stop()
	
	for {
		select {
		case <-ctx.Done():
			return
		case <-bkm.stopCh:
			return
		case <-ticker.C:
			bkm.performBufferAdaptation()
		}
	}
}

// performBufferAdaptation adapts buffer sizes for all connections
func (bkm *BufferKeepAliveManager) performBufferAdaptation() {
	bkm.mu.RLock()
	peersToAdapt := make([]peer.ID, 0, len(bkm.connections))
	for peerID := range bkm.connections {
		peersToAdapt = append(peersToAdapt, peerID)
	}
	bkm.mu.RUnlock()
	
	for _, peerID := range peersToAdapt {
		// Get current bandwidth and latency
		bandwidth := bkm.bandwidthMonitor.GetPeerBandwidth(peerID)
		
		// Measure current latency (simplified - in real implementation would use ping)
		latency := bkm.estimateLatency(peerID)
		
		// Adapt buffer size
		bkm.AdaptBufferSize(peerID, bandwidth, latency)
	}
}

// estimateLatency estimates latency for a peer (simplified implementation)
func (bkm *BufferKeepAliveManager) estimateLatency(peerID peer.ID) time.Duration {
	bkm.mu.RLock()
	defer bkm.mu.RUnlock()
	
	if conn, exists := bkm.connections[peerID]; exists {
		return conn.AverageLatency
	}
	return 50 * time.Millisecond // Default estimate
}

// keepAliveLoop manages keep-alive probes
func (bkm *BufferKeepAliveManager) keepAliveLoop(ctx context.Context) {
	ticker := time.NewTicker(bkm.config.KeepAliveCheckInterval)
	defer ticker.Stop()
	
	for {
		select {
		case <-ctx.Done():
			return
		case <-bkm.stopCh:
			return
		case <-ticker.C:
			bkm.keepAliveManager.PerformKeepAliveChecks(ctx)
		}
	}
}

// slowConnectionDetectionLoop detects and manages slow connections
func (bkm *BufferKeepAliveManager) slowConnectionDetectionLoop(ctx context.Context) {
	ticker := time.NewTicker(bkm.config.SlowConnCheckInterval)
	defer ticker.Stop()
	
	for {
		select {
		case <-ctx.Done():
			return
		case <-bkm.stopCh:
			return
		case <-ticker.C:
			bkm.slowConnDetector.DetectSlowConnections()
		}
	}
}

// bandwidthMonitoringLoop monitors bandwidth usage
func (bkm *BufferKeepAliveManager) bandwidthMonitoringLoop(ctx context.Context) {
	ticker := time.NewTicker(5 * time.Second) // Monitor every 5 seconds
	defer ticker.Stop()
	
	for {
		select {
		case <-ctx.Done():
			return
		case <-bkm.stopCh:
			close(bkm.doneCh)
			return
		case <-ticker.C:
			bkm.bandwidthMonitor.UpdateBandwidthMeasurements()
		}
	}
}

// Helper function - using absInt to avoid conflict
func absInt(x int) int {
	if x < 0 {
		return -x
	}
	return x
}

// bufferNetworkNotifiee handles network events for buffer management
type bufferNetworkNotifiee struct {
	bkm *BufferKeepAliveManager
}

// Connected handles new connections
func (bnn *bufferNetworkNotifiee) Connected(n network.Network, conn network.Conn) {
	peerID := conn.RemotePeer()
	
	// Initialize connection buffer
	bnn.bkm.mu.Lock()
	if _, exists := bnn.bkm.connections[peerID]; !exists {
		bnn.bkm.connections[peerID] = &ConnectionBuffer{
			PeerID:       peerID,
			CurrentSize:  bnn.bkm.config.DefaultBufferSize,
			LastAdjusted: time.Now(),
			LastActivity: time.Now(),
		}
	}
	bnn.bkm.mu.Unlock()
	
	// Apply default buffer size
	bnn.bkm.applyBufferSize(peerID, bnn.bkm.config.DefaultBufferSize)
	
	bufferLog.Debugf("Initialized buffer management for peer %s", peerID)
}

// Disconnected handles connection closures
func (bnn *bufferNetworkNotifiee) Disconnected(n network.Network, conn network.Conn) {
	peerID := conn.RemotePeer()
	
	// Clean up connection tracking
	bnn.bkm.mu.Lock()
	delete(bnn.bkm.connections, peerID)
	bnn.bkm.mu.Unlock()
	
	// Clean up keep-alive state
	bnn.bkm.keepAliveManager.DisableKeepAlive(peerID)
	
	// Clean up slow connection tracking
	bnn.bkm.slowConnDetector.RemoveConnection(peerID)
	
	bufferLog.Debugf("Cleaned up buffer management for peer %s", peerID)
}

// Listen handles listening events
func (bnn *bufferNetworkNotifiee) Listen(network.Network, multiaddr.Multiaddr) {}

// ListenClose handles listen close events
func (bnn *bufferNetworkNotifiee) ListenClose(network.Network, multiaddr.Multiaddr) {}

// NewBandwidthMonitor creates a new bandwidth monitor
func NewBandwidthMonitor(config *BufferKeepAliveConfig) *BandwidthMonitor {
	return &BandwidthMonitor{
		peerBandwidth:     make(map[peer.ID]*BandwidthStats),
		measurementWindow: 60 * time.Second,
		samples:           make([]BandwidthSample, 0, 60),
		maxSamples:        60,
	}
}

// RecordTransfer records a data transfer for bandwidth calculation
func (bm *BandwidthMonitor) RecordTransfer(peerID peer.ID, bytesSent, bytesReceived int64) {
	bm.mu.Lock()
	defer bm.mu.Unlock()
	
	stats, exists := bm.peerBandwidth[peerID]
	if !exists {
		stats = &BandwidthStats{
			samples:     make([]int64, 10),
			sampleIndex: 0,
			sampleCount: 0,
		}
		bm.peerBandwidth[peerID] = stats
	}
	
	now := time.Now()
	
	// Update transfer counters
	stats.BytesSent += bytesSent
	stats.BytesReceived += bytesReceived
	
	// Calculate rate if we have a previous measurement
	if !stats.LastMeasured.IsZero() {
		duration := now.Sub(stats.LastMeasured)
		if duration > 0 {
			totalBytes := bytesSent + bytesReceived
			rate := int64(float64(totalBytes) / duration.Seconds())
			
			// Update current rate
			stats.CurrentRate = rate
			
			// Update peak rate
			if rate > stats.PeakRate {
				stats.PeakRate = rate
			}
			
			// Add to samples for average calculation
			stats.samples[stats.sampleIndex] = rate
			stats.sampleIndex = (stats.sampleIndex + 1) % len(stats.samples)
			if stats.sampleCount < len(stats.samples) {
				stats.sampleCount++
			}
			
			// Calculate average rate
			var sum int64
			for i := 0; i < stats.sampleCount; i++ {
				sum += stats.samples[i]
			}
			stats.AverageRate = sum / int64(stats.sampleCount)
		}
	}
	
	stats.LastMeasured = now
}

// GetPeerBandwidth returns the current bandwidth for a peer
func (bm *BandwidthMonitor) GetPeerBandwidth(peerID peer.ID) int64 {
	bm.mu.RLock()
	defer bm.mu.RUnlock()
	
	if stats, exists := bm.peerBandwidth[peerID]; exists {
		return stats.CurrentRate
	}
	return 0
}

// GetPeerAverageBandwidth returns the average bandwidth for a peer
func (bm *BandwidthMonitor) GetPeerAverageBandwidth(peerID peer.ID) int64 {
	bm.mu.RLock()
	defer bm.mu.RUnlock()
	
	if stats, exists := bm.peerBandwidth[peerID]; exists {
		return stats.AverageRate
	}
	return 0
}

// UpdateBandwidthMeasurements updates global bandwidth measurements
func (bm *BandwidthMonitor) UpdateBandwidthMeasurements() {
	bm.mu.Lock()
	defer bm.mu.Unlock()
	
	now := time.Now()
	var totalBandwidth int64
	peerCount := 0
	
	// Calculate total bandwidth across all peers
	for _, stats := range bm.peerBandwidth {
		totalBandwidth += stats.CurrentRate
		peerCount++
	}
	
	// Update global metrics
	bm.totalBandwidth = totalBandwidth
	if totalBandwidth > bm.peakBandwidth {
		bm.peakBandwidth = totalBandwidth
	}
	
	// Add sample
	sample := BandwidthSample{
		Timestamp: now,
		Bandwidth: totalBandwidth,
		PeerCount: peerCount,
	}
	
	bm.samples = append(bm.samples, sample)
	if len(bm.samples) > bm.maxSamples {
		bm.samples = bm.samples[1:]
	}
	
	// Calculate average bandwidth
	if len(bm.samples) > 0 {
		var sum int64
		for _, sample := range bm.samples {
			sum += sample.Bandwidth
		}
		bm.averageBandwidth = sum / int64(len(bm.samples))
	}
}

// GetGlobalBandwidthStats returns global bandwidth statistics
func (bm *BandwidthMonitor) GetGlobalBandwidthStats() (total, peak, average int64) {
	bm.mu.RLock()
	defer bm.mu.RUnlock()
	
	return bm.totalBandwidth, bm.peakBandwidth, bm.averageBandwidth
}

// NewKeepAliveManager creates a new keep-alive manager
func NewKeepAliveManager(h host.Host, config *BufferKeepAliveConfig) *KeepAliveManager {
	return &KeepAliveManager{
		keepAliveState: make(map[peer.ID]*KeepAliveState),
		config:         config,
		host:           h,
	}
}

// EnableKeepAlive enables keep-alive for a peer
func (kam *KeepAliveManager) EnableKeepAlive(peerID peer.ID, isImportant bool) {
	kam.mu.Lock()
	defer kam.mu.Unlock()
	
	state, exists := kam.keepAliveState[peerID]
	if !exists {
		state = &KeepAliveState{
			PeerID:           peerID,
			BaseInterval:     kam.config.KeepAliveInterval,
			AdaptiveInterval: kam.config.KeepAliveInterval,
		}
		kam.keepAliveState[peerID] = state
	}
	
	state.Enabled = true
	state.IsClusterPeer = isImportant
	state.RecentActivity = time.Now()
	
	// Important peers get higher importance score and more frequent probes
	if isImportant {
		state.ImportanceScore = 1.0
		state.AdaptiveInterval = kam.config.KeepAliveInterval / 2
	} else {
		state.ImportanceScore = 0.5
		state.AdaptiveInterval = kam.config.KeepAliveInterval
	}
	
	bufferLog.Debugf("Enabled keep-alive for peer %s (important: %v)", peerID, isImportant)
}

// DisableKeepAlive disables keep-alive for a peer
func (kam *KeepAliveManager) DisableKeepAlive(peerID peer.ID) {
	kam.mu.Lock()
	defer kam.mu.Unlock()
	
	if state, exists := kam.keepAliveState[peerID]; exists {
		state.Enabled = false
		bufferLog.Debugf("Disabled keep-alive for peer %s", peerID)
	}
}

// RecordActivity records activity for a peer to adjust keep-alive timing
func (kam *KeepAliveManager) RecordActivity(peerID peer.ID) {
	kam.mu.Lock()
	defer kam.mu.Unlock()
	
	if state, exists := kam.keepAliveState[peerID]; exists {
		state.RecentActivity = time.Now()
		
		// Reset failure count on activity
		state.ConsecutiveFailures = 0
		
		// Adjust adaptive interval based on activity
		// More active connections need less frequent probes
		timeSinceLastProbe := time.Since(state.LastProbe)
		if timeSinceLastProbe < state.AdaptiveInterval/2 {
			// Very active, reduce probe frequency
			state.AdaptiveInterval = minDur(state.AdaptiveInterval*2, state.BaseInterval*4)
		}
	}
}

// PerformKeepAliveChecks performs keep-alive checks for all enabled peers
func (kam *KeepAliveManager) PerformKeepAliveChecks(ctx context.Context) {
	kam.mu.RLock()
	peersToCheck := make([]*KeepAliveState, 0)
	
	now := time.Now()
	for _, state := range kam.keepAliveState {
		if state.Enabled && now.Sub(state.LastProbe) >= state.AdaptiveInterval {
			peersToCheck = append(peersToCheck, state)
		}
	}
	kam.mu.RUnlock()
	
	// Perform probes for selected peers
	for _, state := range peersToCheck {
		select {
		case <-ctx.Done():
			return
		default:
			kam.performKeepAliveProbe(ctx, state.PeerID)
		}
	}
}

// performKeepAliveProbe performs a keep-alive probe for a specific peer
func (kam *KeepAliveManager) performKeepAliveProbe(ctx context.Context, peerID peer.ID) {
	kam.mu.Lock()
	state, exists := kam.keepAliveState[peerID]
	if !exists || !state.Enabled {
		kam.mu.Unlock()
		return
	}
	
	state.LastProbe = time.Now()
	kam.mu.Unlock()
	
	// Check if peer is still connected
	if kam.host.Network().Connectedness(peerID) != network.Connected {
		kam.handleKeepAliveFailure(peerID)
		return
	}
	
	// Perform a simple connectivity check
	// In a real implementation, this could be a ping or a small data exchange
	conns := kam.host.Network().ConnsToPeer(peerID)
	if len(conns) == 0 {
		kam.handleKeepAliveFailure(peerID)
		return
	}
	
	// Check connection health by examining connection stats
	conn := conns[0]
	stat := conn.Stat()
	
	// If connection is very old but no recent activity, it might be stale
	if time.Since(stat.Opened) > kam.config.MaxIdleTime {
		// Check for recent activity
		kam.mu.RLock()
		recentActivity := state.RecentActivity
		kam.mu.RUnlock()
		
		if time.Since(recentActivity) > kam.config.MaxIdleTime {
			bufferLog.Debugf("Keep-alive detected stale connection to peer %s", peerID)
			kam.handleKeepAliveFailure(peerID)
			return
		}
	}
	
	// Probe successful
	kam.handleKeepAliveSuccess(peerID)
}

// handleKeepAliveSuccess handles successful keep-alive probe
func (kam *KeepAliveManager) handleKeepAliveSuccess(peerID peer.ID) {
	kam.mu.Lock()
	defer kam.mu.Unlock()
	
	if state, exists := kam.keepAliveState[peerID]; exists {
		state.LastResponse = time.Now()
		state.ConsecutiveFailures = 0
		
		// Adjust adaptive interval based on success
		// Successful probes can reduce frequency slightly
		if state.AdaptiveInterval < state.BaseInterval*2 {
			state.AdaptiveInterval = time.Duration(float64(state.AdaptiveInterval) * 1.1)
		}
		
		bufferLog.Debugf("Keep-alive success for peer %s", peerID)
	}
}

// handleKeepAliveFailure handles failed keep-alive probe
func (kam *KeepAliveManager) handleKeepAliveFailure(peerID peer.ID) {
	kam.mu.Lock()
	defer kam.mu.Unlock()
	
	if state, exists := kam.keepAliveState[peerID]; exists {
		state.ConsecutiveFailures++
		
		// Increase probe frequency on failures
		state.AdaptiveInterval = maxDur(state.AdaptiveInterval/2, state.BaseInterval/4)
		
		bufferLog.Warnf("Keep-alive failure for peer %s (failures: %d)", peerID, state.ConsecutiveFailures)
		
		// If too many failures, consider connection dead
		if state.ConsecutiveFailures >= kam.config.KeepAliveProbeCount {
			bufferLog.Warnf("Peer %s marked as unresponsive after %d keep-alive failures", 
				peerID, state.ConsecutiveFailures)
			
			// Disconnect the peer
			if err := kam.host.Network().ClosePeer(peerID); err != nil {
				bufferLog.Errorf("Failed to close connection to unresponsive peer %s: %v", peerID, err)
			}
			
			// Disable keep-alive for this peer
			state.Enabled = false
		}
	}
}

// GetKeepAliveStats returns keep-alive statistics for a peer
func (kam *KeepAliveManager) GetKeepAliveStats(peerID peer.ID) *KeepAliveState {
	kam.mu.RLock()
	defer kam.mu.RUnlock()
	
	if state, exists := kam.keepAliveState[peerID]; exists {
		// Return a copy to avoid race conditions
		stateCopy := *state
		return &stateCopy
	}
	return nil
}

// NewSlowConnectionDetector creates a new slow connection detector
func NewSlowConnectionDetector(config *BufferKeepAliveConfig) *SlowConnectionDetector {
	return &SlowConnectionDetector{
		slowConnections: make(map[peer.ID]*SlowConnectionInfo),
		config:          config,
	}
}

// RecordLatency records a latency measurement for slow connection detection
func (scd *SlowConnectionDetector) RecordLatency(peerID peer.ID, latency time.Duration) {
	scd.mu.Lock()
	defer scd.mu.Unlock()
	
	info, exists := scd.slowConnections[peerID]
	if !exists {
		info = &SlowConnectionInfo{
			PeerID:         peerID,
			LatencySamples: make([]time.Duration, 0, scd.config.SlowConnectionSamples),
		}
		scd.slowConnections[peerID] = info
	}
	
	// Add latency sample
	info.LatencySamples = append(info.LatencySamples, latency)
	if len(info.LatencySamples) > scd.config.SlowConnectionSamples {
		info.LatencySamples = info.LatencySamples[1:]
	}
	
	// Calculate average latency
	if len(info.LatencySamples) > 0 {
		var sum time.Duration
		for _, sample := range info.LatencySamples {
			sum += sample
		}
		info.AverageLatency = sum / time.Duration(len(info.LatencySamples))
	}
}

// DetectSlowConnections analyzes connections to detect slow ones
func (scd *SlowConnectionDetector) DetectSlowConnections() {
	scd.mu.Lock()
	defer scd.mu.Unlock()
	
	for peerID, info := range scd.slowConnections {
		// Need enough samples to make a determination
		if len(info.LatencySamples) < scd.config.SlowConnectionSamples {
			continue
		}
		
		// Check if average latency exceeds threshold
		if info.AverageLatency > scd.config.SlowConnectionThreshold {
			if !info.BypassEnabled {
				info.ConfirmationCount++
				
				// Confirm slow connection after multiple detections
				if info.ConfirmationCount >= 3 {
					info.DetectedAt = time.Now()
					info.BypassEnabled = scd.config.BypassSlowConnections
					
					bufferLog.Warnf("Detected slow connection to peer %s (avg latency: %v, threshold: %v)",
						peerID, info.AverageLatency, scd.config.SlowConnectionThreshold)
				}
			}
		} else {
			// Connection improved, reset confirmation count
			if info.ConfirmationCount > 0 {
				info.ConfirmationCount--
			}
			
			// If connection is no longer slow, disable bypass
			if info.BypassEnabled && info.ConfirmationCount == 0 {
				info.BypassEnabled = false
				bufferLog.Infof("Connection to peer %s is no longer slow, disabled bypass", peerID)
			}
		}
	}
}

// IsSlowConnection checks if a peer is considered slow
func (scd *SlowConnectionDetector) IsSlowConnection(peerID peer.ID) bool {
	scd.mu.RLock()
	defer scd.mu.RUnlock()
	
	if info, exists := scd.slowConnections[peerID]; exists {
		return info.BypassEnabled
	}
	return false
}

// GetBypassRoutes returns alternative routes for a slow connection
func (scd *SlowConnectionDetector) GetBypassRoutes(peerID peer.ID) []peer.ID {
	scd.mu.RLock()
	defer scd.mu.RUnlock()
	
	if info, exists := scd.slowConnections[peerID]; exists {
		return info.AlternativeRoutes
	}
	return nil
}

// SetBypassRoutes sets alternative routes for a slow connection
func (scd *SlowConnectionDetector) SetBypassRoutes(peerID peer.ID, routes []peer.ID) {
	scd.mu.Lock()
	defer scd.mu.Unlock()
	
	if info, exists := scd.slowConnections[peerID]; exists {
		info.AlternativeRoutes = routes
		bufferLog.Debugf("Set %d bypass routes for slow peer %s", len(routes), peerID)
	}
}

// RemoveConnection removes tracking for a disconnected peer
func (scd *SlowConnectionDetector) RemoveConnection(peerID peer.ID) {
	scd.mu.Lock()
	defer scd.mu.Unlock()
	
	delete(scd.slowConnections, peerID)
}

// GetSlowConnectionInfo returns information about a slow connection
func (scd *SlowConnectionDetector) GetSlowConnectionInfo(peerID peer.ID) *SlowConnectionInfo {
	scd.mu.RLock()
	defer scd.mu.RUnlock()
	
	if info, exists := scd.slowConnections[peerID]; exists {
		// Return a copy to avoid race conditions
		infoCopy := *info
		infoCopy.LatencySamples = make([]time.Duration, len(info.LatencySamples))
		copy(infoCopy.LatencySamples, info.LatencySamples)
		infoCopy.AlternativeRoutes = make([]peer.ID, len(info.AlternativeRoutes))
		copy(infoCopy.AlternativeRoutes, info.AlternativeRoutes)
		return &infoCopy
	}
	return nil
}

// Helper functions for duration operations - using different names to avoid conflicts
func minDur(a, b time.Duration) time.Duration {
	if a < b {
		return a
	}
	return b
}

func maxDur(a, b time.Duration) time.Duration {
	if a > b {
		return a
	}
	return b
}