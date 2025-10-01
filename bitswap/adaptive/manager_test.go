package adaptive

import (
	"context"
	"testing"
	"time"

	bsmsg "github.com/ipfs/boxo/bitswap/message"
	"github.com/ipfs/boxo/bitswap/network"
	"github.com/libp2p/go-libp2p/core/host"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/libp2p/go-libp2p/p2p/protocol/ping"
)

// Mock implementations for testing
type mockNetwork struct{}

func (m *mockNetwork) SendMessage(ctx context.Context, p peer.ID, msg bsmsg.BitSwapMessage) error {
	return nil
}
func (m *mockNetwork) Start(receivers ...network.Receiver) {}
func (m *mockNetwork) Stop()                               {}
func (m *mockNetwork) Connect(ctx context.Context, ai peer.AddrInfo) error {
	return nil
}
func (m *mockNetwork) DisconnectFrom(ctx context.Context, p peer.ID) error { return nil }
func (m *mockNetwork) IsConnectedToPeer(ctx context.Context, p peer.ID) bool {
	return true
}
func (m *mockNetwork) NewMessageSender(ctx context.Context, p peer.ID, opts *network.MessageSenderOpts) (network.MessageSender, error) {
	return nil, nil
}
func (m *mockNetwork) Host() host.Host { return nil }
func (m *mockNetwork) Stats() network.Stats {
	return network.Stats{}
}
func (m *mockNetwork) Self() peer.ID { return peer.ID("mock") }
func (m *mockNetwork) Ping(ctx context.Context, p peer.ID) ping.Result {
	return ping.Result{}
}
func (m *mockNetwork) Latency(p peer.ID) time.Duration { return 0 }
func (m *mockNetwork) TagPeer(p peer.ID, tag string, val int) {}
func (m *mockNetwork) UntagPeer(p peer.ID, tag string)        {}
func (m *mockNetwork) Protect(p peer.ID, tag string)          {}
func (m *mockNetwork) Unprotect(p peer.ID, tag string) bool   { return false }

func TestNewAdaptiveConnectionManager(t *testing.T) {
	config := NewAdaptiveBitswapConfig()
	network := &mockNetwork{}
	
	manager := NewAdaptiveConnectionManager(config, network)
	
	if manager.config != config {
		t.Error("Expected manager to reference the provided config")
	}
	
	if manager.connections == nil {
		t.Error("Expected connections map to be initialized")
	}
	
	if manager.monitor == nil {
		t.Error("Expected monitor to be initialized")
	}
}

func TestPeerConnectedDisconnected(t *testing.T) {
	config := NewAdaptiveBitswapConfig()
	manager := NewAdaptiveConnectionManager(config, &mockNetwork{})
	
	peerID := peer.ID("test-peer")
	
	// Test peer connection
	manager.PeerConnected(peerID)
	
	conn, exists := manager.GetConnectionInfo(peerID)
	if !exists {
		t.Error("Expected peer to be in connections after PeerConnected")
	}
	
	if conn.PeerID != peerID {
		t.Errorf("Expected peer ID to be %s, got %s", peerID, conn.PeerID)
	}
	
	if conn.ConnectedAt.IsZero() {
		t.Error("Expected ConnectedAt to be set")
	}
	
	if conn.LastActivity.IsZero() {
		t.Error("Expected LastActivity to be set")
	}
	
	// Test peer disconnection
	manager.PeerDisconnected(peerID)
	
	_, exists = manager.GetConnectionInfo(peerID)
	if exists {
		t.Error("Expected peer to be removed from connections after PeerDisconnected")
	}
}

func TestRecordRequest(t *testing.T) {
	config := NewAdaptiveBitswapConfig()
	manager := NewAdaptiveConnectionManager(config, &mockNetwork{})
	
	peerID := peer.ID("test-peer")
	manager.PeerConnected(peerID)
	
	bytes := int64(1024)
	tracker := manager.RecordRequest(peerID, bytes)
	
	if tracker == nil {
		t.Error("Expected RecordRequest to return a tracker")
	}
	
	conn, exists := manager.GetConnectionInfo(peerID)
	if !exists {
		t.Fatal("Expected peer to exist")
	}
	
	if conn.RequestCount != 1 {
		t.Errorf("Expected RequestCount to be 1, got %d", conn.RequestCount)
	}
	
	if conn.OutstandingBytes != bytes {
		t.Errorf("Expected OutstandingBytes to be %d, got %d", bytes, conn.OutstandingBytes)
	}
}

func TestRecordResponse(t *testing.T) {
	config := NewAdaptiveBitswapConfig()
	manager := NewAdaptiveConnectionManager(config, &mockNetwork{})
	
	peerID := peer.ID("test-peer")
	manager.PeerConnected(peerID)
	
	// Record a request first
	bytes := int64(1024)
	manager.RecordRequest(peerID, bytes)
	
	// Record successful response
	responseTime := 50 * time.Millisecond
	manager.RecordResponse(peerID, bytes, responseTime, true)
	
	conn, exists := manager.GetConnectionInfo(peerID)
	if !exists {
		t.Fatal("Expected peer to exist")
	}
	
	if conn.ResponseCount != 1 {
		t.Errorf("Expected ResponseCount to be 1, got %d", conn.ResponseCount)
	}
	
	if conn.OutstandingBytes != 0 {
		t.Errorf("Expected OutstandingBytes to be 0 after response, got %d", conn.OutstandingBytes)
	}
	
	if conn.AverageResponseTime != responseTime {
		t.Errorf("Expected AverageResponseTime to be %v, got %v", responseTime, conn.AverageResponseTime)
	}
	
	if conn.ErrorCount != 0 {
		t.Errorf("Expected ErrorCount to be 0, got %d", conn.ErrorCount)
	}
}

func TestRecordResponseWithError(t *testing.T) {
	config := NewAdaptiveBitswapConfig()
	manager := NewAdaptiveConnectionManager(config, &mockNetwork{})
	
	peerID := peer.ID("test-peer")
	manager.PeerConnected(peerID)
	
	// Record a request first
	bytes := int64(1024)
	manager.RecordRequest(peerID, bytes)
	
	// Record error response
	responseTime := 100 * time.Millisecond
	manager.RecordResponse(peerID, bytes, responseTime, false)
	
	conn, exists := manager.GetConnectionInfo(peerID)
	if !exists {
		t.Fatal("Expected peer to exist")
	}
	
	if conn.ResponseCount != 1 {
		t.Errorf("Expected ResponseCount to be 1, got %d", conn.ResponseCount)
	}
	
	if conn.ErrorCount != 1 {
		t.Errorf("Expected ErrorCount to be 1, got %d", conn.ErrorCount)
	}
	
	// Average response time should not be updated for errors
	if conn.AverageResponseTime != 0 {
		t.Errorf("Expected AverageResponseTime to be 0 for error response, got %v", conn.AverageResponseTime)
	}
}

func TestAverageResponseTimeCalculation(t *testing.T) {
	config := NewAdaptiveBitswapConfig()
	manager := NewAdaptiveConnectionManager(config, &mockNetwork{})
	
	peerID := peer.ID("test-peer")
	manager.PeerConnected(peerID)
	
	// Record multiple successful responses
	responseTimes := []time.Duration{
		50 * time.Millisecond,
		100 * time.Millisecond,
		75 * time.Millisecond,
	}
	
	for i, rt := range responseTimes {
		bytes := int64(1024)
		manager.RecordRequest(peerID, bytes)
		manager.RecordResponse(peerID, bytes, rt, true)
		
		conn, _ := manager.GetConnectionInfo(peerID)
		
		if i == 0 {
			// First response should set the average directly
			if conn.AverageResponseTime != rt {
				t.Errorf("Expected first AverageResponseTime to be %v, got %v", rt, conn.AverageResponseTime)
			}
		} else {
			// Subsequent responses should use exponential moving average
			// We don't test the exact value due to floating point precision,
			// but ensure it's reasonable
			if conn.AverageResponseTime <= 0 || conn.AverageResponseTime > 200*time.Millisecond {
				t.Errorf("AverageResponseTime seems unreasonable: %v", conn.AverageResponseTime)
			}
		}
	}
}

func TestGetMaxOutstandingBytesForPeer(t *testing.T) {
	config := NewAdaptiveBitswapConfig()
	manager := NewAdaptiveConnectionManager(config, &mockNetwork{})
	
	peerID := peer.ID("test-peer")
	
	// Test with unknown peer
	baseLimit := config.GetMaxOutstandingBytesPerPeer()
	limit := manager.GetMaxOutstandingBytesForPeer(peerID)
	if limit != baseLimit {
		t.Errorf("Expected limit for unknown peer to be base limit %d, got %d", baseLimit, limit)
	}
	
	// Test with connected peer (no performance data)
	manager.PeerConnected(peerID)
	limit = manager.GetMaxOutstandingBytesForPeer(peerID)
	if limit != baseLimit {
		t.Errorf("Expected limit for new peer to be base limit %d, got %d", baseLimit, limit)
	}
}

func TestGetMaxOutstandingBytesForPeerWithHighErrorRate(t *testing.T) {
	config := NewAdaptiveBitswapConfig()
	manager := NewAdaptiveConnectionManager(config, &mockNetwork{})
	
	peerID := peer.ID("test-peer")
	manager.PeerConnected(peerID)
	
	// Simulate high error rate (50% errors)
	for i := 0; i < 20; i++ {
		bytes := int64(1024)
		manager.RecordRequest(peerID, bytes)
		success := i%2 == 0 // 50% success rate
		manager.RecordResponse(peerID, bytes, 50*time.Millisecond, success)
	}
	
	baseLimit := config.GetMaxOutstandingBytesPerPeer()
	adjustedLimit := manager.GetMaxOutstandingBytesForPeer(peerID)
	
	// Limit should be reduced due to high error rate
	if adjustedLimit >= baseLimit {
		t.Errorf("Expected adjusted limit to be less than base limit %d, got %d", baseLimit, adjustedLimit)
	}
	
	// Should not go below minimum
	minLimit := config.MinOutstandingBytesPerPeer
	if adjustedLimit < minLimit {
		t.Errorf("Expected adjusted limit to be at least %d, got %d", minLimit, adjustedLimit)
	}
}

func TestGetMaxOutstandingBytesForPeerWithHighResponseTime(t *testing.T) {
	config := NewAdaptiveBitswapConfig()
	manager := NewAdaptiveConnectionManager(config, &mockNetwork{})
	
	peerID := peer.ID("test-peer")
	manager.PeerConnected(peerID)
	
	// Simulate high response times
	highResponseTime := 200 * time.Millisecond // Higher than HighPriorityThreshold (50ms)
	for i := 0; i < 10; i++ {
		bytes := int64(1024)
		manager.RecordRequest(peerID, bytes)
		manager.RecordResponse(peerID, bytes, highResponseTime, true)
	}
	
	baseLimit := config.GetMaxOutstandingBytesPerPeer()
	adjustedLimit := manager.GetMaxOutstandingBytesForPeer(peerID)
	
	// Limit should be reduced due to high response time
	if adjustedLimit >= baseLimit {
		t.Errorf("Expected adjusted limit to be less than base limit %d, got %d", baseLimit, adjustedLimit)
	}
}

func TestGetAllConnections(t *testing.T) {
	config := NewAdaptiveBitswapConfig()
	manager := NewAdaptiveConnectionManager(config, &mockNetwork{})
	
	// Connect multiple peers
	peerIDs := []peer.ID{
		peer.ID("peer1"),
		peer.ID("peer2"),
		peer.ID("peer3"),
	}
	
	for _, peerID := range peerIDs {
		manager.PeerConnected(peerID)
	}
	
	connections := manager.GetAllConnections()
	
	if len(connections) != len(peerIDs) {
		t.Errorf("Expected %d connections, got %d", len(peerIDs), len(connections))
	}
	
	for _, peerID := range peerIDs {
		if _, exists := connections[peerID]; !exists {
			t.Errorf("Expected connection for peer %s to exist", peerID)
		}
	}
}

func TestGetLoadMetrics(t *testing.T) {
	config := NewAdaptiveBitswapConfig()
	manager := NewAdaptiveConnectionManager(config, &mockNetwork{})
	
	// Update some metrics
	config.UpdateMetrics(100.0, 50*time.Millisecond, 100*time.Millisecond, 1000, 50)
	
	metrics := manager.GetLoadMetrics()
	
	if metrics.RequestsPerSecond != 100.0 {
		t.Errorf("Expected RequestsPerSecond to be 100.0, got %f", metrics.RequestsPerSecond)
	}
	
	if metrics.AverageResponseTime != 50*time.Millisecond {
		t.Errorf("Expected AverageResponseTime to be 50ms, got %v", metrics.AverageResponseTime)
	}
}

func TestConnectionInfoCopy(t *testing.T) {
	config := NewAdaptiveBitswapConfig()
	manager := NewAdaptiveConnectionManager(config, &mockNetwork{})
	
	peerID := peer.ID("test-peer")
	manager.PeerConnected(peerID)
	
	// Get connection info
	conn1, exists := manager.GetConnectionInfo(peerID)
	if !exists {
		t.Fatal("Expected peer to exist")
	}
	
	// Modify the returned connection info
	originalRequestCount := conn1.RequestCount
	conn1.RequestCount = 999
	
	// Get connection info again
	conn2, exists := manager.GetConnectionInfo(peerID)
	if !exists {
		t.Fatal("Expected peer to exist")
	}
	
	// Should not be affected by modification of the first copy
	if conn2.RequestCount != originalRequestCount {
		t.Errorf("Expected RequestCount to be %d, got %d (connection info should be copied)", originalRequestCount, conn2.RequestCount)
	}
}

func TestStartStop(t *testing.T) {
	config := NewAdaptiveBitswapConfig()
	manager := NewAdaptiveConnectionManager(config, &mockNetwork{})
	
	// Start manager
	err := manager.Start()
	if err != nil {
		t.Fatalf("Expected Start to succeed, got error: %v", err)
	}
	
	// Let it run briefly
	time.Sleep(50 * time.Millisecond)
	
	// Stop manager
	err = manager.Stop()
	if err != nil {
		t.Fatalf("Expected Stop to succeed, got error: %v", err)
	}
	
	// If we reach here without deadlock, the test passes
}