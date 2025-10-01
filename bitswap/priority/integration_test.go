package priority

import (
	"context"
	"testing"
	"time"

	"github.com/ipfs/boxo/bitswap/adaptive"
	"github.com/ipfs/go-cid"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAdaptivePriorityManager(t *testing.T) {
	adaptiveConfig := adaptive.NewAdaptiveBitswapConfig()
	apm := NewAdaptivePriorityManager(adaptiveConfig, 100*time.Millisecond)
	
	apm.Start()
	defer apm.Stop()
	
	// Test that manager is properly initialized
	assert.NotNil(t, apm.PriorityRequestManager)
	assert.NotNil(t, apm.adaptiveConfig)
	assert.NotNil(t, apm.metricsCollector)
	
	// Test basic request processing
	testCID, _ := cid.Parse("QmTest")
	testPeerID, _ := peer.Decode("12D3KooWTest")
	
	reqCtx := RequestContext{
		CID:         testCID,
		PeerID:      testPeerID,
		RequestTime: time.Now(),
		Deadline:    time.Now().Add(100 * time.Millisecond),
		BlockSize:   100,
	}
	
	err := apm.EnqueueRequest(reqCtx)
	assert.NoError(t, err)
	
	// Wait for processing and adaptation
	time.Sleep(200 * time.Millisecond)
	
	// Check that metrics are being collected
	metrics := apm.GetAdaptiveMetrics()
	assert.NotNil(t, metrics.PriorityStats)
	assert.NotNil(t, metrics.ConfigSnapshot)
	assert.NotNil(t, metrics.MetricsSnapshot)
}

func TestMetricsCollection(t *testing.T) {
	collector := &PriorityMetricsCollector{
		priorityDistribution: make(map[Priority]float64),
		priorityWaitTimes:    make(map[Priority]time.Duration),
		metricsWindow:        time.Minute,
		lastCounterReset:     time.Now(),
	}
	
	// Simulate processing requests
	testCID, _ := cid.Parse("QmTest")
	testPeerID, _ := peer.Decode("12D3KooWTest")
	
	for i := 0; i < 10; i++ {
		request := &PriorityRequest{
			Context: RequestContext{
				CID:         testCID,
				PeerID:      testPeerID,
				RequestTime: time.Now().Add(-time.Duration(i) * time.Millisecond),
			},
			Priority:  Priority(i % 4),
			QueueTime: time.Now(),
		}
		
		result := &RequestResult{
			Request:     request,
			Success:     true,
			ProcessTime: time.Duration(10+i) * time.Millisecond,
			DataSize:    100,
		}
		
		collector.RecordRequest(request, result)
	}
	
	metrics := collector.GetCurrentMetrics()
	
	// Check that metrics are being recorded
	assert.True(t, metrics.AverageResponseTime > 0)
	assert.True(t, metrics.P95ResponseTime > 0)
	assert.True(t, len(metrics.PriorityDistribution) > 0)
	assert.True(t, len(metrics.PriorityWaitTimes) > 0)
}

func TestAdaptiveConfigurationUpdates(t *testing.T) {
	adaptiveConfig := adaptive.NewAdaptiveBitswapConfig()
	apm := NewAdaptivePriorityManager(adaptiveConfig, 50*time.Millisecond)
	
	apm.Start()
	defer apm.Stop()
	
	// Get initial configuration
	initialWorkers := adaptiveConfig.GetCurrentWorkerCount()
	initialBytes := adaptiveConfig.GetMaxOutstandingBytesPerPeer()
	
	// Simulate high load to trigger adaptation
	adaptiveConfig.UpdateMetrics(
		2000.0,                    // High RPS
		150*time.Millisecond,      // High response time
		300*time.Millisecond,      // High P95
		5000,                      // Many outstanding requests
		100,                       // Many connections
	)
	
	// Manually trigger adaptation to ensure it happens
	adapted := adaptiveConfig.AdaptConfiguration(context.Background())
	
	// Wait for adaptation cycle to process
	time.Sleep(100 * time.Millisecond)
	
	// Check if configuration was adapted
	newWorkers := adaptiveConfig.GetCurrentWorkerCount()
	newBytes := adaptiveConfig.GetMaxOutstandingBytesPerPeer()
	
	if adapted {
		// If adaptation occurred, should have scaled up due to high load
		assert.True(t, newWorkers >= initialWorkers, "Workers should not decrease under high load")
		assert.True(t, newBytes >= initialBytes, "Bytes limit should not decrease under high load")
	} else {
		// If no adaptation occurred, values should remain the same
		assert.Equal(t, initialWorkers, newWorkers, "Workers should remain unchanged if no adaptation")
		assert.Equal(t, initialBytes, newBytes, "Bytes should remain unchanged if no adaptation")
	}
	
	// Test that the adaptive manager can handle configuration updates
	assert.NotNil(t, apm.adaptiveConfig)
	assert.NotNil(t, apm.metricsCollector)
}

func TestPriorityDistributionTracking(t *testing.T) {
	collector := &PriorityMetricsCollector{
		priorityDistribution: make(map[Priority]float64),
		priorityWaitTimes:    make(map[Priority]time.Duration),
		metricsWindow:        time.Minute,
		lastCounterReset:     time.Now(),
	}
	
	testCID, _ := cid.Parse("QmTest")
	testPeerID, _ := peer.Decode("12D3KooWTest")
	
	// Record requests with different priorities
	priorities := []Priority{CriticalPriority, HighPriority, NormalPriority, LowPriority}
	counts := []int{1, 2, 4, 3} // Different counts for each priority
	
	for i, priority := range priorities {
		for j := 0; j < counts[i]; j++ {
			request := &PriorityRequest{
				Context: RequestContext{
					CID:         testCID,
					PeerID:      testPeerID,
					RequestTime: time.Now(),
				},
				Priority:  priority,
				QueueTime: time.Now(),
			}
			
			result := &RequestResult{
				Request:     request,
				Success:     true,
				ProcessTime: 10 * time.Millisecond,
				DataSize:    100,
			}
			
			collector.RecordRequest(request, result)
		}
	}
	
	metrics := collector.GetCurrentMetrics()
	
	// Check priority distribution
	totalRequests := 10.0 // Sum of counts
	expectedDistribution := map[Priority]float64{
		CriticalPriority: 1.0 / totalRequests,
		HighPriority:     2.0 / totalRequests,
		NormalPriority:   4.0 / totalRequests,
		LowPriority:      3.0 / totalRequests,
	}
	
	for priority, expected := range expectedDistribution {
		actual := metrics.PriorityDistribution[priority]
		assert.InDelta(t, expected, actual, 0.01, "Priority %s distribution mismatch", priority)
	}
}

func TestResourceMetricsUpdate(t *testing.T) {
	collector := &PriorityMetricsCollector{
		priorityDistribution: make(map[Priority]float64),
		priorityWaitTimes:    make(map[Priority]time.Duration),
		metricsWindow:        time.Minute,
		lastCounterReset:     time.Now(),
	}
	
	// Update resource metrics
	collector.UpdateResourceMetrics(1000, 50, 0.75)
	
	metrics := collector.GetCurrentMetrics()
	
	assert.Equal(t, int64(1000), metrics.OutstandingRequests)
	assert.Equal(t, 50, metrics.ActiveConnections)
	assert.Equal(t, 0.75, metrics.ResourceUtilization)
}

func TestAdaptiveMetricsSnapshot(t *testing.T) {
	adaptiveConfig := adaptive.NewAdaptiveBitswapConfig()
	apm := NewAdaptivePriorityManager(adaptiveConfig, 100*time.Millisecond)
	
	apm.Start()
	defer apm.Stop()
	
	// Process some requests to generate metrics
	testCID, _ := cid.Parse("QmTest")
	testPeerID, _ := peer.Decode("12D3KooWTest")
	
	for i := 0; i < 5; i++ {
		reqCtx := RequestContext{
			CID:         testCID,
			PeerID:      testPeerID,
			RequestTime: time.Now(),
			Deadline:    time.Now().Add(100 * time.Millisecond),
			BlockSize:   100,
		}
		err := apm.EnqueueRequest(reqCtx)
		require.NoError(t, err)
	}
	
	// Wait for processing
	time.Sleep(150 * time.Millisecond)
	
	// Get adaptive metrics
	adaptiveMetrics := apm.GetAdaptiveMetrics()
	
	// Verify all components are present
	assert.NotNil(t, adaptiveMetrics.PriorityStats)
	assert.NotNil(t, adaptiveMetrics.ConfigSnapshot)
	assert.NotNil(t, adaptiveMetrics.MetricsSnapshot)
	assert.True(t, adaptiveMetrics.AdaptationInterval > 0)
	
	// Verify metrics contain expected data
	assert.True(t, adaptiveMetrics.PriorityStats.TotalRequests > 0)
	assert.True(t, adaptiveMetrics.ConfigSnapshot.CurrentWorkerCount > 0)
	assert.True(t, adaptiveMetrics.ConfigSnapshot.MaxOutstandingBytesPerPeer > 0)
}

func TestCreateAdaptivePriorityManagerFromConfig(t *testing.T) {
	adaptiveConfig := adaptive.NewAdaptiveBitswapConfig()
	
	// Modify some config values
	adaptiveConfig.UpdateMetrics(100, 50*time.Millisecond, 100*time.Millisecond, 500, 10)
	
	apm := CreateAdaptivePriorityManagerFromConfig(adaptiveConfig)
	
	assert.NotNil(t, apm)
	assert.Equal(t, adaptiveConfig, apm.adaptiveConfig)
	assert.Equal(t, 30*time.Second, apm.updateInterval)
	
	// Test that it can be started and stopped
	apm.Start()
	time.Sleep(50 * time.Millisecond)
	apm.Stop()
}

func BenchmarkAdaptivePriorityManager(b *testing.B) {
	adaptiveConfig := adaptive.NewAdaptiveBitswapConfig()
	apm := NewAdaptivePriorityManager(adaptiveConfig, time.Second)
	
	apm.Start()
	defer apm.Stop()
	
	testCID, _ := cid.Parse("QmTest")
	testPeerID, _ := peer.Decode("12D3KooWTest")
	
	b.ResetTimer()
	
	for i := 0; i < b.N; i++ {
		reqCtx := RequestContext{
			CID:         testCID,
			PeerID:      testPeerID,
			RequestTime: time.Now(),
			Deadline:    time.Now().Add(100 * time.Millisecond),
			BlockSize:   100,
		}
		apm.EnqueueRequest(reqCtx)
	}
}

func BenchmarkMetricsCollection(b *testing.B) {
	collector := &PriorityMetricsCollector{
		priorityDistribution: make(map[Priority]float64),
		priorityWaitTimes:    make(map[Priority]time.Duration),
		metricsWindow:        time.Minute,
		lastCounterReset:     time.Now(),
	}
	
	testCID, _ := cid.Parse("QmTest")
	testPeerID, _ := peer.Decode("12D3KooWTest")
	
	request := &PriorityRequest{
		Context: RequestContext{
			CID:         testCID,
			PeerID:      testPeerID,
			RequestTime: time.Now(),
		},
		Priority:  NormalPriority,
		QueueTime: time.Now(),
	}
	
	result := &RequestResult{
		Request:     request,
		Success:     true,
		ProcessTime: 10 * time.Millisecond,
		DataSize:    100,
	}
	
	b.ResetTimer()
	
	for i := 0; i < b.N; i++ {
		collector.RecordRequest(request, result)
	}
}