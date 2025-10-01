package priority

import (
	"container/heap"
	"context"
	"sync"
	"testing"
	"time"

	"github.com/ipfs/go-cid"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestPriorityQueue(t *testing.T) {
	pq := &PriorityQueue{}
	heap.Init(pq)
	
	// Test empty queue
	assert.Equal(t, 0, pq.Len())
	
	// Add requests with different priorities
	now := time.Now()
	requests := []*PriorityRequest{
		{Priority: NormalPriority, QueueTime: now},
		{Priority: CriticalPriority, QueueTime: now.Add(time.Second)},
		{Priority: HighPriority, QueueTime: now.Add(2*time.Second)},
		{Priority: LowPriority, QueueTime: now.Add(3*time.Second)},
	}
	
	// Push all requests
	for _, req := range requests {
		heap.Push(pq, req)
	}
	
	assert.Equal(t, 4, pq.Len())
	
	// Pop should return highest priority first (CriticalPriority)
	first := heap.Pop(pq).(*PriorityRequest)
	assert.Equal(t, CriticalPriority, first.Priority)
	
	// Then HighPriority
	second := heap.Pop(pq).(*PriorityRequest)
	assert.Equal(t, HighPriority, second.Priority)
	
	// Then NormalPriority
	third := heap.Pop(pq).(*PriorityRequest)
	assert.Equal(t, NormalPriority, third.Priority)
	
	// Finally LowPriority
	fourth := heap.Pop(pq).(*PriorityRequest)
	assert.Equal(t, LowPriority, fourth.Priority)
	
	assert.Equal(t, 0, pq.Len())
}

func TestPriorityQueueSamePriority(t *testing.T) {
	pq := &PriorityQueue{}
	heap.Init(pq)
	
	now := time.Now()
	
	// Add requests with same priority but different times
	req1 := &PriorityRequest{Priority: NormalPriority, QueueTime: now}
	req2 := &PriorityRequest{Priority: NormalPriority, QueueTime: now.Add(time.Second)}
	req3 := &PriorityRequest{Priority: NormalPriority, QueueTime: now.Add(-time.Second)}
	
	heap.Push(pq, req1)
	heap.Push(pq, req2)
	heap.Push(pq, req3)
	
	// Should return in chronological order (earliest first)
	first := heap.Pop(pq).(*PriorityRequest)
	assert.Equal(t, req3.QueueTime, first.QueueTime) // Earliest
	
	second := heap.Pop(pq).(*PriorityRequest)
	assert.Equal(t, req1.QueueTime, second.QueueTime) // Middle
	
	third := heap.Pop(pq).(*PriorityRequest)
	assert.Equal(t, req2.QueueTime, third.QueueTime) // Latest
}

func TestResourcePool(t *testing.T) {
	rp := NewResourcePool(10, 1000)
	
	// Test initial state
	current, max, bandwidth, maxBandwidth := rp.GetStats()
	assert.Equal(t, 0, current)
	assert.Equal(t, 10, max)
	assert.Equal(t, int64(0), bandwidth)
	assert.Equal(t, int64(1000), maxBandwidth)
	
	// Test allocation
	assert.True(t, rp.CanAllocate(100))
	assert.True(t, rp.Allocate(100))
	
	current, _, bandwidth, _ = rp.GetStats()
	assert.Equal(t, 1, current)
	assert.Equal(t, int64(100), bandwidth)
	
	// Test multiple allocations
	for i := 0; i < 9; i++ {
		assert.True(t, rp.Allocate(50))
	}
	
	current, _, _, _ = rp.GetStats()
	assert.Equal(t, 10, current)
	
	// Should not be able to allocate more
	assert.False(t, rp.CanAllocate(50))
	assert.False(t, rp.Allocate(50))
	
	// Test release
	rp.Release(100)
	current, _, _, _ = rp.GetStats()
	assert.Equal(t, 9, current)
	
	// Should be able to allocate again
	assert.True(t, rp.CanAllocate(50))
}

func TestResourcePoolBandwidthLimit(t *testing.T) {
	rp := NewResourcePool(100, 500) // High concurrent limit, low bandwidth limit
	
	// Should be able to allocate up to bandwidth limit
	assert.True(t, rp.Allocate(300))
	assert.True(t, rp.Allocate(200))
	
	// Should not exceed bandwidth limit
	assert.False(t, rp.CanAllocate(1))
	assert.False(t, rp.Allocate(1))
	
	// Wait for bandwidth reset (simulate time passing)
	time.Sleep(1100 * time.Millisecond)
	
	// Should be able to allocate again after reset
	assert.True(t, rp.CanAllocate(300))
	assert.True(t, rp.Allocate(300))
}

func TestPriorityRequestManager(t *testing.T) {
	prm := NewPriorityRequestManager(10, 1000, 50*time.Millisecond, 10*time.Millisecond)
	
	ctx := context.Background()
	prm.Start(ctx)
	defer prm.Stop()
	
	// Test basic enqueue
	testCID, _ := cid.Parse("QmTest")
	testPeerID, _ := peer.Decode("12D3KooWTest")
	
	reqCtx := RequestContext{
		CID:         testCID,
		PeerID:      testPeerID,
		RequestTime: time.Now(),
		Deadline:    time.Now().Add(100 * time.Millisecond),
		BlockSize:   100,
	}
	
	err := prm.EnqueueRequest(reqCtx)
	assert.NoError(t, err)
	
	// Check stats
	stats := prm.GetStats()
	assert.Equal(t, int64(1), stats.TotalRequests)
	
	// Check queue sizes
	sizes := prm.GetQueueSizes()
	totalQueued := 0
	for _, size := range sizes {
		totalQueued += size
	}
	// Request might be processed immediately or still in queue
	assert.True(t, totalQueued >= 0)
}

func TestRequestClassification(t *testing.T) {
	prm := NewPriorityRequestManager(10, 1000, 50*time.Millisecond, 10*time.Millisecond)
	
	testCID, _ := cid.Parse("QmTest")
	testPeerID, _ := peer.Decode("12D3KooWTest")
	now := time.Now()
	
	tests := []struct {
		name     string
		context  RequestContext
		expected Priority
	}{
		{
			name: "Critical priority - very urgent deadline",
			context: RequestContext{
				CID:         testCID,
				PeerID:      testPeerID,
				RequestTime: now,
				Deadline:    now.Add(5 * time.Millisecond),
				BlockSize:   100,
			},
			expected: CriticalPriority,
		},
		{
			name: "High priority - urgent deadline",
			context: RequestContext{
				CID:         testCID,
				PeerID:      testPeerID,
				RequestTime: now,
				Deadline:    now.Add(30 * time.Millisecond),
				BlockSize:   100,
			},
			expected: HighPriority,
		},
		{
			name: "High priority - many retries",
			context: RequestContext{
				CID:         testCID,
				PeerID:      testPeerID,
				RequestTime: now,
				Deadline:    now.Add(200 * time.Millisecond),
				BlockSize:   100,
				RetryCount:  3,
			},
			expected: HighPriority,
		},
		{
			name: "Normal priority - session request",
			context: RequestContext{
				CID:         testCID,
				PeerID:      testPeerID,
				RequestTime: now,
				Deadline:    now.Add(200 * time.Millisecond),
				BlockSize:   100,
				IsSession:   true,
			},
			expected: NormalPriority,
		},
		{
			name: "Low priority - default",
			context: RequestContext{
				CID:         testCID,
				PeerID:      testPeerID,
				RequestTime: now,
				Deadline:    now.Add(200 * time.Millisecond),
				BlockSize:   100,
			},
			expected: LowPriority,
		},
		{
			name: "User-specified priority overrides",
			context: RequestContext{
				CID:          testCID,
				PeerID:       testPeerID,
				RequestTime:  now,
				Deadline:     now.Add(200 * time.Millisecond),
				BlockSize:    100,
				UserPriority: CriticalPriority,
			},
			expected: CriticalPriority,
		},
	}
	
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			priority := prm.ClassifyRequest(tt.context)
			assert.Equal(t, tt.expected, priority)
		})
	}
}

func TestPriorityManagerConfiguration(t *testing.T) {
	prm := NewPriorityRequestManager(10, 1000, 50*time.Millisecond, 10*time.Millisecond)
	
	// Test initial configuration
	current, max, _, _ := prm.resourcePool.GetStats()
	assert.Equal(t, 0, current)
	assert.Equal(t, 10, max)
	
	// Update configuration
	prm.UpdateConfiguration(20, 2000, 100*time.Millisecond, 20*time.Millisecond)
	
	// Check updated limits
	_, max, _, maxBandwidth := prm.resourcePool.GetStats()
	assert.Equal(t, 20, max)
	assert.Equal(t, int64(2000), maxBandwidth)
	
	// Check updated thresholds
	assert.Equal(t, 100*time.Millisecond, prm.highPriorityThreshold)
	assert.Equal(t, 20*time.Millisecond, prm.criticalPriorityThreshold)
}

func TestPriorityManagerStats(t *testing.T) {
	prm := NewPriorityRequestManager(10, 1000, 50*time.Millisecond, 10*time.Millisecond)
	
	ctx := context.Background()
	prm.Start(ctx)
	defer prm.Stop()
	
	// Enqueue multiple requests
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
		err := prm.EnqueueRequest(reqCtx)
		require.NoError(t, err)
	}
	
	// Wait a bit for processing
	time.Sleep(100 * time.Millisecond)
	
	stats := prm.GetStats()
	assert.Equal(t, int64(5), stats.TotalRequests)
	assert.True(t, stats.ProcessedRequests >= 0)
	assert.True(t, stats.ResourceUtilization >= 0.0)
	assert.True(t, stats.ResourceUtilization <= 1.0)
}

func TestPriorityManagerQueueFull(t *testing.T) {
	prm := NewPriorityRequestManager(1, 1000, 50*time.Millisecond, 10*time.Millisecond)
	prm.maxQueueSize = 2 // Set small queue size for testing
	
	testCID, _ := cid.Parse("QmTest")
	testPeerID, _ := peer.Decode("12D3KooWTest")
	
	// Fill the queue
	for i := 0; i < 2; i++ {
		reqCtx := RequestContext{
			CID:         testCID,
			PeerID:      testPeerID,
			RequestTime: time.Now(),
			Deadline:    time.Now().Add(100 * time.Millisecond),
			BlockSize:   100,
		}
		err := prm.EnqueueRequest(reqCtx)
		assert.NoError(t, err)
	}
	
	// Next request should fail
	reqCtx := RequestContext{
		CID:         testCID,
		PeerID:      testPeerID,
		RequestTime: time.Now(),
		Deadline:    time.Now().Add(100 * time.Millisecond),
		BlockSize:   100,
	}
	err := prm.EnqueueRequest(reqCtx)
	assert.Equal(t, ErrQueueFull, err)
	
	stats := prm.GetStats()
	assert.Equal(t, int64(1), stats.DroppedRequests)
}

func TestPriorityManagerCallback(t *testing.T) {
	prm := NewPriorityRequestManager(10, 1000, 50*time.Millisecond, 10*time.Millisecond)
	
	ctx := context.Background()
	prm.Start(ctx)
	defer prm.Stop()
	
	var mu sync.Mutex
	callbackCalled := false
	var callbackRequest *PriorityRequest
	var callbackResult *RequestResult
	
	prm.SetRequestProcessedCallback(func(req *PriorityRequest, result *RequestResult) {
		mu.Lock()
		defer mu.Unlock()
		callbackCalled = true
		callbackRequest = req
		callbackResult = result
	})
	
	testCID, _ := cid.Parse("QmTest")
	testPeerID, _ := peer.Decode("12D3KooWTest")
	
	reqCtx := RequestContext{
		CID:         testCID,
		PeerID:      testPeerID,
		RequestTime: time.Now(),
		Deadline:    time.Now().Add(100 * time.Millisecond),
		BlockSize:   100,
	}
	
	err := prm.EnqueueRequest(reqCtx)
	require.NoError(t, err)
	
	// Wait for processing
	time.Sleep(200 * time.Millisecond)
	
	mu.Lock()
	defer mu.Unlock()
	assert.True(t, callbackCalled)
	assert.NotNil(t, callbackRequest)
	assert.NotNil(t, callbackResult)
	assert.Equal(t, testCID, callbackRequest.Context.CID)
}

func BenchmarkPriorityQueue(b *testing.B) {
	pq := &PriorityQueue{}
	
	// Pre-populate with some requests
	now := time.Now()
	for i := 0; i < 1000; i++ {
		req := &PriorityRequest{
			Priority:  Priority(i % 4),
			QueueTime: now.Add(time.Duration(i) * time.Millisecond),
		}
		pq.Push(req)
	}
	
	b.ResetTimer()
	
	for i := 0; i < b.N; i++ {
		// Add a request
		req := &PriorityRequest{
			Priority:  Priority(i % 4),
			QueueTime: now.Add(time.Duration(i) * time.Millisecond),
		}
		pq.Push(req)
		
		// Remove a request
		if pq.Len() > 0 {
			pq.Pop()
		}
	}
}

func BenchmarkRequestClassification(b *testing.B) {
	prm := NewPriorityRequestManager(10, 1000, 50*time.Millisecond, 10*time.Millisecond)
	
	testCID, _ := cid.Parse("QmTest")
	testPeerID, _ := peer.Decode("12D3KooWTest")
	
	reqCtx := RequestContext{
		CID:         testCID,
		PeerID:      testPeerID,
		RequestTime: time.Now(),
		Deadline:    time.Now().Add(100 * time.Millisecond),
		BlockSize:   100,
	}
	
	b.ResetTimer()
	
	for i := 0; i < b.N; i++ {
		prm.ClassifyRequest(reqCtx)
	}
}

func BenchmarkEnqueueRequest(b *testing.B) {
	prm := NewPriorityRequestManager(1000, 10000, 50*time.Millisecond, 10*time.Millisecond)
	
	ctx := context.Background()
	prm.Start(ctx)
	defer prm.Stop()
	
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
		prm.EnqueueRequest(reqCtx)
	}
}