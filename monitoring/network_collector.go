package monitoring

import (
	"context"
	"sync"
	"time"
)

// NetworkCollector collects network-specific performance metrics
type NetworkCollector struct {
	mu                    sync.RWMutex
	activeConnections     int
	totalBandwidth        int64
	latencySum            time.Duration
	latencyCount          int64
	packetLossRate        float64
	connectionErrors      int64
	bytesSent             int64
	bytesReceived         int64
	peerMetrics           map[string]*PeerMetrics
	
	// Latency tracking
	latencies             []time.Duration
	maxLatencies          int
	
	// Bandwidth tracking
	bandwidthSamples      []int64
	maxBandwidthSamples   int
}

// NewNetworkCollector creates a new network metrics collector
func NewNetworkCollector() *NetworkCollector {
	return &NetworkCollector{
		peerMetrics:         make(map[string]*PeerMetrics),
		maxLatencies:        1000,
		maxBandwidthSamples: 100,
		latencies:           make([]time.Duration, 0, 1000),
		bandwidthSamples:    make([]int64, 0, 100),
	}
}

// CollectMetrics implements MetricCollector interface
func (nc *NetworkCollector) CollectMetrics(ctx context.Context) (map[string]interface{}, error) {
	nc.mu.RLock()
	defer nc.mu.RUnlock()
	
	var avgLatency time.Duration
	if nc.latencyCount > 0 {
		avgLatency = time.Duration(int64(nc.latencySum) / nc.latencyCount)
	}
	
	// Calculate average bandwidth
	var avgBandwidth int64
	if len(nc.bandwidthSamples) > 0 {
		var sum int64
		for _, sample := range nc.bandwidthSamples {
			sum += sample
		}
		avgBandwidth = sum / int64(len(nc.bandwidthSamples))
	}
	
	// Copy peer metrics for safe access
	peerMetricsCopy := make(map[string]PeerMetrics)
	for peerID, metrics := range nc.peerMetrics {
		peerMetricsCopy[peerID] = *metrics
	}
	
	metrics := map[string]interface{}{
		"active_connections":  nc.activeConnections,
		"total_bandwidth":     avgBandwidth,
		"average_latency":     avgLatency,
		"packet_loss_rate":    nc.packetLossRate,
		"connection_errors":   nc.connectionErrors,
		"bytes_sent":          nc.bytesSent,
		"bytes_received":      nc.bytesReceived,
		"peer_connections":    peerMetricsCopy,
		"peer_count":          len(nc.peerMetrics),
	}
	
	// Reset counters for next collection period
	nc.latencySum = 0
	nc.latencyCount = 0
	nc.connectionErrors = 0
	nc.bytesSent = 0
	nc.bytesReceived = 0
	
	return metrics, nil
}

// GetMetricNames returns the list of metric names this collector provides
func (nc *NetworkCollector) GetMetricNames() []string {
	return []string{
		"active_connections",
		"total_bandwidth",
		"average_latency",
		"packet_loss_rate",
		"connection_errors",
		"bytes_sent",
		"bytes_received",
		"peer_connections",
		"peer_count",
	}
}

// UpdateActiveConnections updates the count of active connections
func (nc *NetworkCollector) UpdateActiveConnections(count int) {
	nc.mu.Lock()
	defer nc.mu.Unlock()
	
	nc.activeConnections = count
}

// RecordLatency records a network latency measurement
func (nc *NetworkCollector) RecordLatency(latency time.Duration) {
	nc.mu.Lock()
	defer nc.mu.Unlock()
	
	nc.latencySum += latency
	nc.latencyCount++
	
	// Add to latencies for analysis
	nc.latencies = append(nc.latencies, latency)
	if len(nc.latencies) > nc.maxLatencies {
		nc.latencies = nc.latencies[1:]
	}
}

// RecordBandwidth records a bandwidth measurement
func (nc *NetworkCollector) RecordBandwidth(bandwidth int64) {
	nc.mu.Lock()
	defer nc.mu.Unlock()
	
	nc.totalBandwidth = bandwidth
	
	// Add to bandwidth samples for averaging
	nc.bandwidthSamples = append(nc.bandwidthSamples, bandwidth)
	if len(nc.bandwidthSamples) > nc.maxBandwidthSamples {
		nc.bandwidthSamples = nc.bandwidthSamples[1:]
	}
}

// UpdatePacketLossRate updates the packet loss rate
func (nc *NetworkCollector) UpdatePacketLossRate(rate float64) {
	nc.mu.Lock()
	defer nc.mu.Unlock()
	
	nc.packetLossRate = rate
}

// RecordConnectionError records a connection error
func (nc *NetworkCollector) RecordConnectionError() {
	nc.mu.Lock()
	defer nc.mu.Unlock()
	
	nc.connectionErrors++
}

// RecordBytesSent records bytes sent
func (nc *NetworkCollector) RecordBytesSent(bytes int64) {
	nc.mu.Lock()
	defer nc.mu.Unlock()
	
	nc.bytesSent += bytes
}

// RecordBytesReceived records bytes received
func (nc *NetworkCollector) RecordBytesReceived(bytes int64) {
	nc.mu.Lock()
	defer nc.mu.Unlock()
	
	nc.bytesReceived += bytes
}

// UpdatePeerMetrics updates metrics for a specific peer
func (nc *NetworkCollector) UpdatePeerMetrics(peerID string, metrics PeerMetrics) {
	nc.mu.Lock()
	defer nc.mu.Unlock()
	
	nc.peerMetrics[peerID] = &metrics
}

// RecordPeerLatency records latency for a specific peer
func (nc *NetworkCollector) RecordPeerLatency(peerID string, latency time.Duration) {
	nc.mu.Lock()
	defer nc.mu.Unlock()
	
	if peer, exists := nc.peerMetrics[peerID]; exists {
		peer.Latency = latency
		peer.LastSeen = time.Now()
	} else {
		nc.peerMetrics[peerID] = &PeerMetrics{
			Latency:  latency,
			LastSeen: time.Now(),
		}
	}
}

// RecordPeerBandwidth records bandwidth for a specific peer
func (nc *NetworkCollector) RecordPeerBandwidth(peerID string, bandwidth int64) {
	nc.mu.Lock()
	defer nc.mu.Unlock()
	
	if peer, exists := nc.peerMetrics[peerID]; exists {
		peer.Bandwidth = bandwidth
		peer.LastSeen = time.Now()
	} else {
		nc.peerMetrics[peerID] = &PeerMetrics{
			Bandwidth: bandwidth,
			LastSeen:  time.Now(),
		}
	}
}

// RecordPeerError records an error for a specific peer
func (nc *NetworkCollector) RecordPeerError(peerID string) {
	nc.mu.Lock()
	defer nc.mu.Unlock()
	
	if peer, exists := nc.peerMetrics[peerID]; exists {
		// Simple error rate calculation - increment error count
		peer.ErrorRate += 0.01 // Simplified - in real implementation, track properly
		if peer.ErrorRate > 1.0 {
			peer.ErrorRate = 1.0
		}
		peer.LastSeen = time.Now()
	} else {
		nc.peerMetrics[peerID] = &PeerMetrics{
			ErrorRate: 0.01,
			LastSeen:  time.Now(),
		}
	}
}

// RecordPeerBytes records bytes sent/received for a specific peer
func (nc *NetworkCollector) RecordPeerBytes(peerID string, sent, received int64) {
	nc.mu.Lock()
	defer nc.mu.Unlock()
	
	if peer, exists := nc.peerMetrics[peerID]; exists {
		peer.BytesSent += sent
		peer.BytesReceived += received
		peer.LastSeen = time.Now()
	} else {
		nc.peerMetrics[peerID] = &PeerMetrics{
			BytesSent:     sent,
			BytesReceived: received,
			LastSeen:      time.Now(),
		}
	}
}

// GetLatencyPercentiles returns P95 and P99 latencies
func (nc *NetworkCollector) GetLatencyPercentiles() (p95, p99 time.Duration) {
	nc.mu.RLock()
	defer nc.mu.RUnlock()
	
	return nc.calculatePercentiles(nc.latencies)
}

// RemoveStaleConnections removes peer metrics for connections that haven't been seen recently
func (nc *NetworkCollector) RemoveStaleConnections(maxAge time.Duration) {
	nc.mu.Lock()
	defer nc.mu.Unlock()
	
	now := time.Now()
	for peerID, metrics := range nc.peerMetrics {
		if now.Sub(metrics.LastSeen) > maxAge {
			delete(nc.peerMetrics, peerID)
		}
	}
}

// calculatePercentiles calculates P95 and P99 from a slice of durations
func (nc *NetworkCollector) calculatePercentiles(latencies []time.Duration) (p95, p99 time.Duration) {
	if len(latencies) == 0 {
		return 0, 0
	}
	
	// Create a copy and sort it
	times := make([]time.Duration, len(latencies))
	copy(times, latencies)
	
	// Simple bubble sort for small arrays
	for i := 0; i < len(times); i++ {
		for j := i + 1; j < len(times); j++ {
			if times[i] > times[j] {
				times[i], times[j] = times[j], times[i]
			}
		}
	}
	
	// Calculate percentiles
	p95Index := int(float64(len(times)) * 0.95)
	p99Index := int(float64(len(times)) * 0.99)
	
	if p95Index >= len(times) {
		p95Index = len(times) - 1
	}
	if p99Index >= len(times) {
		p99Index = len(times) - 1
	}
	
	return times[p95Index], times[p99Index]
}