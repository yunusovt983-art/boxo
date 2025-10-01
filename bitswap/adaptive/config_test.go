package adaptive

import (
	"context"
	"testing"
	"time"

	"github.com/ipfs/boxo/bitswap/internal/defaults"
)

func TestNewAdaptiveBitswapConfig(t *testing.T) {
	config := NewAdaptiveBitswapConfig()
	
	// Test default values
	if config.MaxOutstandingBytesPerPeer != defaults.BitswapMaxOutstandingBytesPerPeer {
		t.Errorf("Expected MaxOutstandingBytesPerPeer to be %d, got %d",
			defaults.BitswapMaxOutstandingBytesPerPeer, config.MaxOutstandingBytesPerPeer)
	}
	
	if config.MinOutstandingBytesPerPeer != 256*1024 {
		t.Errorf("Expected MinOutstandingBytesPerPeer to be %d, got %d",
			256*1024, config.MinOutstandingBytesPerPeer)
	}
	
	if config.CurrentWorkerCount != defaults.BitswapEngineBlockstoreWorkerCount {
		t.Errorf("Expected CurrentWorkerCount to be %d, got %d",
			defaults.BitswapEngineBlockstoreWorkerCount, config.CurrentWorkerCount)
	}
	
	if config.BatchSize != 100 {
		t.Errorf("Expected BatchSize to be 100, got %d", config.BatchSize)
	}
	
	if config.LoadThresholdHigh != 0.8 {
		t.Errorf("Expected LoadThresholdHigh to be 0.8, got %f", config.LoadThresholdHigh)
	}
}

func TestUpdateMetrics(t *testing.T) {
	config := NewAdaptiveBitswapConfig()
	
	// Update metrics
	rps := 500.0
	avgTime := 75 * time.Millisecond
	p95Time := 150 * time.Millisecond
	outstanding := int64(1000)
	connections := 50
	
	config.UpdateMetrics(rps, avgTime, p95Time, outstanding, connections)
	
	// Verify metrics were updated
	if config.RequestsPerSecond != rps {
		t.Errorf("Expected RequestsPerSecond to be %f, got %f", rps, config.RequestsPerSecond)
	}
	
	if config.AverageResponseTime != avgTime {
		t.Errorf("Expected AverageResponseTime to be %v, got %v", avgTime, config.AverageResponseTime)
	}
	
	if config.P95ResponseTime != p95Time {
		t.Errorf("Expected P95ResponseTime to be %v, got %v", p95Time, config.P95ResponseTime)
	}
	
	if config.OutstandingRequests != outstanding {
		t.Errorf("Expected OutstandingRequests to be %d, got %d", outstanding, config.OutstandingRequests)
	}
	
	if config.ActiveConnections != connections {
		t.Errorf("Expected ActiveConnections to be %d, got %d", connections, config.ActiveConnections)
	}
}

func TestCalculateLoadFactor(t *testing.T) {
	config := NewAdaptiveBitswapConfig()
	
	tests := []struct {
		name           string
		avgResponse    time.Duration
		p95Response    time.Duration
		outstanding    int64
		rps            float64
		expectedRange  [2]float64 // min, max expected load factor
	}{
		{
			name:          "Low load",
			avgResponse:   10 * time.Millisecond,
			p95Response:   20 * time.Millisecond,
			outstanding:   100,
			rps:           50,
			expectedRange: [2]float64{0.0, 0.3},
		},
		{
			name:          "Medium load",
			avgResponse:   50 * time.Millisecond,
			p95Response:   100 * time.Millisecond,
			outstanding:   5000,
			rps:           500,
			expectedRange: [2]float64{0.3, 0.7},
		},
		{
			name:          "High load",
			avgResponse:   100 * time.Millisecond,
			p95Response:   200 * time.Millisecond,
			outstanding:   10000,
			rps:           1000,
			expectedRange: [2]float64{0.7, 1.0},
		},
		{
			name:          "Extreme load",
			avgResponse:   200 * time.Millisecond,
			p95Response:   500 * time.Millisecond,
			outstanding:   20000,
			rps:           2000,
			expectedRange: [2]float64{1.0, 2.0},
		},
	}
	
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			config.UpdateMetrics(tt.rps, tt.avgResponse, tt.p95Response, tt.outstanding, 100)
			loadFactor := config.calculateLoadFactor()
			
			if loadFactor < tt.expectedRange[0] || loadFactor > tt.expectedRange[1] {
				t.Errorf("Load factor %f not in expected range [%f, %f]",
					loadFactor, tt.expectedRange[0], tt.expectedRange[1])
			}
		})
	}
}

func TestScaleUp(t *testing.T) {
	config := NewAdaptiveBitswapConfig()
	initialMaxBytes := config.MaxOutstandingBytesPerPeer
	initialWorkerCount := config.CurrentWorkerCount
	initialBatchSize := config.BatchSize
	
	adapted := config.scaleUp()
	
	if !adapted {
		t.Error("Expected scaleUp to return true")
	}
	
	if config.MaxOutstandingBytesPerPeer <= initialMaxBytes {
		t.Errorf("Expected MaxOutstandingBytesPerPeer to increase from %d, got %d",
			initialMaxBytes, config.MaxOutstandingBytesPerPeer)
	}
	
	if config.CurrentWorkerCount <= initialWorkerCount {
		t.Errorf("Expected CurrentWorkerCount to increase from %d, got %d",
			initialWorkerCount, config.CurrentWorkerCount)
	}
	
	if config.BatchSize <= initialBatchSize {
		t.Errorf("Expected BatchSize to increase from %d, got %d",
			initialBatchSize, config.BatchSize)
	}
}

func TestScaleDown(t *testing.T) {
	config := NewAdaptiveBitswapConfig()
	
	// First scale up to have room to scale down
	config.scaleUp()
	config.scaleUp()
	
	maxBytesBeforeScaleDown := config.MaxOutstandingBytesPerPeer
	workerCountBeforeScaleDown := config.CurrentWorkerCount
	batchSizeBeforeScaleDown := config.BatchSize
	
	adapted := config.scaleDown()
	
	if !adapted {
		t.Error("Expected scaleDown to return true")
	}
	
	if config.MaxOutstandingBytesPerPeer >= maxBytesBeforeScaleDown {
		t.Errorf("Expected MaxOutstandingBytesPerPeer to decrease from %d, got %d",
			maxBytesBeforeScaleDown, config.MaxOutstandingBytesPerPeer)
	}
	
	if config.CurrentWorkerCount >= workerCountBeforeScaleDown {
		t.Errorf("Expected CurrentWorkerCount to decrease from %d, got %d",
			workerCountBeforeScaleDown, config.CurrentWorkerCount)
	}
	
	if config.BatchSize >= batchSizeBeforeScaleDown {
		t.Errorf("Expected BatchSize to decrease from %d, got %d",
			batchSizeBeforeScaleDown, config.BatchSize)
	}
}

func TestScaleLimits(t *testing.T) {
	config := NewAdaptiveBitswapConfig()
	
	// Test upper limits
	for i := 0; i < 20; i++ {
		config.scaleUp()
	}
	
	maxLimit := int64(100 * 1024 * 1024) // 100MB
	if config.MaxOutstandingBytesPerPeer > maxLimit {
		t.Errorf("MaxOutstandingBytesPerPeer exceeded upper limit: %d > %d",
			config.MaxOutstandingBytesPerPeer, maxLimit)
	}
	
	if config.CurrentWorkerCount > config.MaxWorkerCount {
		t.Errorf("CurrentWorkerCount exceeded upper limit: %d > %d",
			config.CurrentWorkerCount, config.MaxWorkerCount)
	}
	
	if config.BatchSize > 1000 {
		t.Errorf("BatchSize exceeded upper limit: %d > 1000", config.BatchSize)
	}
	
	// Test lower limits
	for i := 0; i < 20; i++ {
		config.scaleDown()
	}
	
	if config.MaxOutstandingBytesPerPeer < config.MinOutstandingBytesPerPeer {
		t.Errorf("MaxOutstandingBytesPerPeer below lower limit: %d < %d",
			config.MaxOutstandingBytesPerPeer, config.MinOutstandingBytesPerPeer)
	}
	
	if config.CurrentWorkerCount < config.MinWorkerCount {
		t.Errorf("CurrentWorkerCount below lower limit: %d < %d",
			config.CurrentWorkerCount, config.MinWorkerCount)
	}
	
	if config.BatchSize < 50 {
		t.Errorf("BatchSize below lower limit: %d < 50", config.BatchSize)
	}
}

func TestAdaptConfiguration(t *testing.T) {
	config := NewAdaptiveBitswapConfig()
	config.AdaptationInterval = 100 * time.Millisecond // Short interval for testing
	
	ctx := context.Background()
	
	// Test high load adaptation
	config.UpdateMetrics(
		1500.0,                  // High RPS
		120*time.Millisecond,    // High avg response time
		250*time.Millisecond,    // High P95 response time
		15000,                   // High outstanding requests
		200,                     // Many connections
	)
	
	time.Sleep(150 * time.Millisecond) // Wait for adaptation interval
	
	initialMaxBytes := config.MaxOutstandingBytesPerPeer
	adapted := config.AdaptConfiguration(ctx)
	
	if !adapted {
		t.Error("Expected adaptation for high load")
	}
	
	if config.MaxOutstandingBytesPerPeer <= initialMaxBytes {
		t.Error("Expected MaxOutstandingBytesPerPeer to increase under high load")
	}
	
	// Test low load adaptation
	config.UpdateMetrics(
		50.0,                   // Low RPS
		15*time.Millisecond,    // Low avg response time
		30*time.Millisecond,    // Low P95 response time
		100,                    // Low outstanding requests
		10,                     // Few connections
	)
	
	time.Sleep(150 * time.Millisecond) // Wait for adaptation interval
	
	maxBytesBeforeScaleDown := config.MaxOutstandingBytesPerPeer
	adapted = config.AdaptConfiguration(ctx)
	
	if !adapted {
		t.Error("Expected adaptation for low load")
	}
	
	if config.MaxOutstandingBytesPerPeer >= maxBytesBeforeScaleDown {
		t.Error("Expected MaxOutstandingBytesPerPeer to decrease under low load")
	}
}

func TestAdaptationInterval(t *testing.T) {
	config := NewAdaptiveBitswapConfig()
	config.AdaptationInterval = 1 * time.Second
	
	ctx := context.Background()
	
	// Set high load that will definitely trigger adaptation
	config.UpdateMetrics(2000.0, 150*time.Millisecond, 300*time.Millisecond, 20000, 200)
	
	// First adaptation should work
	adapted1 := config.AdaptConfiguration(ctx)
	if !adapted1 {
		t.Error("Expected first adaptation to work")
	}
	
	// Immediate second adaptation should be blocked
	adapted2 := config.AdaptConfiguration(ctx)
	if adapted2 {
		t.Error("Expected second adaptation to be blocked by interval")
	}
	
	// Wait for interval and try again
	time.Sleep(1100 * time.Millisecond)
	adapted3 := config.AdaptConfiguration(ctx)
	if !adapted3 {
		t.Error("Expected third adaptation to work after interval")
	}
}

func TestConfigChangeCallback(t *testing.T) {
	config := NewAdaptiveBitswapConfig()
	
	callbackCalled := false
	var callbackConfig *AdaptiveBitswapConfig
	
	config.SetConfigChangeCallback(func(c *AdaptiveBitswapConfig) {
		callbackCalled = true
		callbackConfig = c
	})
	
	// Trigger adaptation with very high load
	config.UpdateMetrics(2000.0, 150*time.Millisecond, 300*time.Millisecond, 20000, 200)
	adapted := config.AdaptConfiguration(context.Background())
	
	if !adapted {
		t.Error("Expected adaptation to occur for callback test")
	}
	
	if !callbackCalled {
		t.Error("Expected callback to be called")
	}
	
	if callbackConfig != config {
		t.Error("Expected callback to receive the same config instance")
	}
}

func TestGetSnapshot(t *testing.T) {
	config := NewAdaptiveBitswapConfig()
	
	// Update some values
	config.UpdateMetrics(500.0, 50*time.Millisecond, 100*time.Millisecond, 1000, 50)
	
	snapshot := config.GetSnapshot()
	
	// Verify snapshot contains current values
	if snapshot.MaxOutstandingBytesPerPeer != config.MaxOutstandingBytesPerPeer {
		t.Error("Snapshot MaxOutstandingBytesPerPeer mismatch")
	}
	
	if snapshot.RequestsPerSecond != config.RequestsPerSecond {
		t.Error("Snapshot RequestsPerSecond mismatch")
	}
	
	if snapshot.AverageResponseTime != config.AverageResponseTime {
		t.Error("Snapshot AverageResponseTime mismatch")
	}
	
	// Modify original config
	config.MaxOutstandingBytesPerPeer = 999999
	
	// Snapshot should remain unchanged
	if snapshot.MaxOutstandingBytesPerPeer == 999999 {
		t.Error("Snapshot should not be affected by changes to original config")
	}
}

func TestThreadSafety(t *testing.T) {
	config := NewAdaptiveBitswapConfig()
	
	// Run concurrent operations
	done := make(chan bool, 4)
	
	// Concurrent metric updates
	go func() {
		for i := 0; i < 100; i++ {
			config.UpdateMetrics(float64(i), time.Duration(i)*time.Millisecond, time.Duration(i*2)*time.Millisecond, int64(i*10), i)
		}
		done <- true
	}()
	
	// Concurrent adaptations
	go func() {
		for i := 0; i < 100; i++ {
			config.AdaptConfiguration(context.Background())
			time.Sleep(time.Millisecond)
		}
		done <- true
	}()
	
	// Concurrent reads
	go func() {
		for i := 0; i < 100; i++ {
			config.GetMaxOutstandingBytesPerPeer()
			config.GetCurrentWorkerCount()
			config.GetBatchConfig()
			config.GetPriorityThresholds()
		}
		done <- true
	}()
	
	// Concurrent snapshots
	go func() {
		for i := 0; i < 100; i++ {
			config.GetSnapshot()
		}
		done <- true
	}()
	
	// Wait for all goroutines to complete
	for i := 0; i < 4; i++ {
		<-done
	}
	
	// If we reach here without deadlock or race conditions, the test passes
}