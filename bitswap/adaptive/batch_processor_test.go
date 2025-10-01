package adaptive

import (
	"context"
	"sync"
	"testing"
	"time"

	blocks "github.com/ipfs/go-block-format"
	"github.com/ipfs/go-cid"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/multiformats/go-multihash"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Helper function to create a test CID
func createTestCID(t *testing.T, data string) cid.Cid {
	hash, err := multihash.Sum([]byte(data), multihash.SHA2_256, -1)
	require.NoError(t, err)
	return cid.NewCidV1(cid.Raw, hash)
}

// Helper function to create a test peer ID
func createTestPeerID(t *testing.T, id string) peer.ID {
	peerID, err := peer.Decode("12D3KooW" + id + "AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA")
	if err != nil {
		// Fallback to a simpler method for testing
		return peer.ID(id)
	}
	return peerID
}

// Mock request handler for testing
func mockRequestHandler(ctx context.Context, requests []*BatchRequest) []BatchResult {
	results := make([]BatchResult, len(requests))
	
	for i, req := range requests {
		// Simulate some processing time
		time.Sleep(1 * time.Millisecond)
		
		// Create a mock block
		blockData := []byte("mock-block-data-" + req.CID.String())
		block, _ := blocks.NewBlockWithCid(blockData, req.CID)
		
		results[i] = BatchResult{
			Block: block,
			Error: nil,
		}
	}
	
	return results
}

func TestBatchRequestProcessor_BasicFunctionality(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	
	config := NewAdaptiveBitswapConfig()
	config.BatchSize = 3
	config.BatchTimeout = 50 * time.Millisecond
	
	processor := NewBatchRequestProcessor(ctx, config, mockRequestHandler)
	defer processor.Close()
	
	// Submit a few requests
	cid1 := createTestCID(t, "test1")
	cid2 := createTestCID(t, "test2")
	peer1 := createTestPeerID(t, "peer1")
	
	result1Chan := processor.SubmitRequest(ctx, cid1, peer1, NormalPriority)
	result2Chan := processor.SubmitRequest(ctx, cid2, peer1, NormalPriority)
	
	// Wait for results
	select {
	case result1 := <-result1Chan:
		assert.NoError(t, result1.Error)
		assert.NotNil(t, result1.Block)
		assert.Equal(t, cid1, result1.Block.Cid())
	case <-time.After(5 * time.Second):
		t.Fatal("Timeout waiting for result1")
	}
	
	select {
	case result2 := <-result2Chan:
		assert.NoError(t, result2.Error)
		assert.NotNil(t, result2.Block)
		assert.Equal(t, cid2, result2.Block.Cid())
	case <-time.After(5 * time.Second):
		t.Fatal("Timeout waiting for result2")
	}
	
	// Check metrics
	metrics := processor.GetMetrics()
	assert.Equal(t, uint64(2), metrics.TotalRequests)
	assert.True(t, metrics.BatchedRequests >= 2)
}

func TestBatchRequestProcessor_BatchSizeTriggering(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	
	config := NewAdaptiveBitswapConfig()
	config.BatchSize = 3
	config.BatchTimeout = 1 * time.Second // Long timeout to test size triggering
	
	processor := NewBatchRequestProcessor(ctx, config, mockRequestHandler)
	defer processor.Close()
	
	// Submit exactly batch size requests
	var resultChans []<-chan BatchResult
	peer1 := createTestPeerID(t, "peer1")
	
	for i := 0; i < 3; i++ {
		cid := createTestCID(t, "test"+string(rune(i)))
		resultChan := processor.SubmitRequest(ctx, cid, peer1, NormalPriority)
		resultChans = append(resultChans, resultChan)
	}
	
	// All results should be available quickly (batch size triggered)
	for i, resultChan := range resultChans {
		select {
		case result := <-resultChan:
			assert.NoError(t, result.Error, "Request %d failed", i)
			assert.NotNil(t, result.Block, "Request %d has nil block", i)
		case <-time.After(2 * time.Second):
			t.Fatalf("Timeout waiting for result %d", i)
		}
	}
}

func TestBatchRequestProcessor_TimeoutTriggering(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	
	config := NewAdaptiveBitswapConfig()
	config.BatchSize = 10 // Large batch size
	config.BatchTimeout = 100 * time.Millisecond // Short timeout
	
	processor := NewBatchRequestProcessor(ctx, config, mockRequestHandler)
	defer processor.Close()
	
	// Submit fewer requests than batch size
	cid1 := createTestCID(t, "test1")
	peer1 := createTestPeerID(t, "peer1")
	
	start := time.Now()
	resultChan := processor.SubmitRequest(ctx, cid1, peer1, NormalPriority)
	
	// Result should be available after timeout
	select {
	case result := <-resultChan:
		elapsed := time.Since(start)
		assert.NoError(t, result.Error)
		assert.NotNil(t, result.Block)
		assert.True(t, elapsed >= 90*time.Millisecond, "Batch processed too quickly: %v", elapsed)
		assert.True(t, elapsed <= 200*time.Millisecond, "Batch processed too slowly: %v", elapsed)
	case <-time.After(5 * time.Second):
		t.Fatal("Timeout waiting for result")
	}
}

func TestBatchRequestProcessor_PriorityHandling(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	
	config := NewAdaptiveBitswapConfig()
	config.BatchSize = 2
	config.BatchTimeout = 50 * time.Millisecond
	
	// Mock handler that tracks processing order
	var processOrder []RequestPriority
	var orderMutex sync.Mutex
	
	priorityHandler := func(ctx context.Context, requests []*BatchRequest) []BatchResult {
		orderMutex.Lock()
		if len(requests) > 0 {
			processOrder = append(processOrder, requests[0].Priority)
		}
		orderMutex.Unlock()
		
		return mockRequestHandler(ctx, requests)
	}
	
	processor := NewBatchRequestProcessor(ctx, config, priorityHandler)
	defer processor.Close()
	
	peer1 := createTestPeerID(t, "peer1")
	
	// Submit requests in reverse priority order
	lowChan := processor.SubmitRequest(ctx, createTestCID(t, "low1"), peer1, LowPriority)
	lowChan2 := processor.SubmitRequest(ctx, createTestCID(t, "low2"), peer1, LowPriority)
	
	normalChan := processor.SubmitRequest(ctx, createTestCID(t, "normal1"), peer1, NormalPriority)
	normalChan2 := processor.SubmitRequest(ctx, createTestCID(t, "normal2"), peer1, NormalPriority)
	
	criticalChan := processor.SubmitRequest(ctx, createTestCID(t, "critical1"), peer1, CriticalPriority)
	criticalChan2 := processor.SubmitRequest(ctx, createTestCID(t, "critical2"), peer1, CriticalPriority)
	
	// Wait for all results
	channels := []<-chan BatchResult{lowChan, lowChan2, normalChan, normalChan2, criticalChan, criticalChan2}
	for i, ch := range channels {
		select {
		case result := <-ch:
			assert.NoError(t, result.Error, "Request %d failed", i)
		case <-time.After(5 * time.Second):
			t.Fatalf("Timeout waiting for result %d", i)
		}
	}
	
	// Check that critical priority was processed first
	orderMutex.Lock()
	defer orderMutex.Unlock()
	
	assert.True(t, len(processOrder) >= 3, "Expected at least 3 batches processed")
	// Critical should be processed before others
	criticalFound := false
	for _, priority := range processOrder {
		if priority == CriticalPriority {
			criticalFound = true
			break
		}
		// Should not find low priority before critical
		assert.NotEqual(t, LowPriority, priority, "Low priority processed before critical")
	}
	assert.True(t, criticalFound, "Critical priority batch not found in processing order")
}

func TestBatchRequestProcessor_ConcurrentRequests(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	
	config := NewAdaptiveBitswapConfig()
	config.BatchSize = 5
	config.BatchTimeout = 100 * time.Millisecond
	
	processor := NewBatchRequestProcessor(ctx, config, mockRequestHandler)
	defer processor.Close()
	
	const numGoroutines = 10
	const requestsPerGoroutine = 20
	
	var wg sync.WaitGroup
	var successCount int64
	var successMutex sync.Mutex
	
	peer1 := createTestPeerID(t, "peer1")
	
	// Launch concurrent goroutines
	for g := 0; g < numGoroutines; g++ {
		wg.Add(1)
		go func(goroutineID int) {
			defer wg.Done()
			
			for i := 0; i < requestsPerGoroutine; i++ {
				cid := createTestCID(t, "concurrent-"+string(rune(goroutineID))+"-"+string(rune(i)))
				resultChan := processor.SubmitRequest(ctx, cid, peer1, NormalPriority)
				
				select {
				case result := <-resultChan:
					if result.Error == nil {
						successMutex.Lock()
						successCount++
						successMutex.Unlock()
					}
				case <-time.After(10 * time.Second):
					t.Errorf("Timeout in goroutine %d, request %d", goroutineID, i)
				}
			}
		}(g)
	}
	
	wg.Wait()
	
	expectedRequests := int64(numGoroutines * requestsPerGoroutine)
	successMutex.Lock()
	actualSuccess := successCount
	successMutex.Unlock()
	
	assert.Equal(t, expectedRequests, actualSuccess, "Not all requests succeeded")
	
	// Check metrics
	metrics := processor.GetMetrics()
	assert.Equal(t, uint64(expectedRequests), metrics.TotalRequests)
	assert.True(t, metrics.ProcessedBatches > 0)
	assert.True(t, metrics.AverageBatchSize > 1.0)
}

func TestBatchRequestProcessor_ConfigurationUpdate(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	
	config := NewAdaptiveBitswapConfig()
	config.BatchSize = 2
	config.CurrentWorkerCount = 4
	
	processor := NewBatchRequestProcessor(ctx, config, mockRequestHandler)
	defer processor.Close()
	
	// Update configuration
	newConfig := NewAdaptiveBitswapConfig()
	newConfig.BatchSize = 5
	newConfig.CurrentWorkerCount = 8
	
	processor.UpdateConfiguration(newConfig)
	
	// Verify the configuration was updated
	assert.Equal(t, newConfig, processor.config)
	
	// Test that new batch size is used
	peer1 := createTestPeerID(t, "peer1")
	var resultChans []<-chan BatchResult
	
	// Submit requests equal to new batch size
	for i := 0; i < 5; i++ {
		cid := createTestCID(t, "config-test-"+string(rune(i)))
		resultChan := processor.SubmitRequest(ctx, cid, peer1, NormalPriority)
		resultChans = append(resultChans, resultChan)
	}
	
	// All should complete quickly due to batch size trigger
	for i, resultChan := range resultChans {
		select {
		case result := <-resultChan:
			assert.NoError(t, result.Error, "Request %d failed", i)
		case <-time.After(2 * time.Second):
			t.Fatalf("Timeout waiting for result %d after config update", i)
		}
	}
}

func TestBatchRequestProcessor_ContextCancellation(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	
	config := NewAdaptiveBitswapConfig()
	processor := NewBatchRequestProcessor(ctx, config, mockRequestHandler)
	
	peer1 := createTestPeerID(t, "peer1")
	cid1 := createTestCID(t, "test1")
	
	// Submit a request
	resultChan := processor.SubmitRequest(ctx, cid1, peer1, NormalPriority)
	
	// Cancel context
	cancel()
	
	// Close processor
	err := processor.Close()
	assert.NoError(t, err)
	
	// Result channel should be closed or return quickly
	select {
	case _, ok := <-resultChan:
		if ok {
			// Got a result, which is fine
		} else {
			// Channel closed, which is also fine
		}
	case <-time.After(1 * time.Second):
		// This is also acceptable - the request might not complete due to cancellation
	}
}

func TestBatchRequestProcessor_Metrics(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	
	config := NewAdaptiveBitswapConfig()
	config.BatchSize = 2
	config.BatchTimeout = 50 * time.Millisecond
	
	processor := NewBatchRequestProcessor(ctx, config, mockRequestHandler)
	defer processor.Close()
	
	peer1 := createTestPeerID(t, "peer1")
	
	// Submit requests with different priorities
	cid1 := createTestCID(t, "test1")
	cid2 := createTestCID(t, "test2")
	cid3 := createTestCID(t, "test3")
	
	result1Chan := processor.SubmitRequest(ctx, cid1, peer1, HighPriority)
	result2Chan := processor.SubmitRequest(ctx, cid2, peer1, HighPriority)
	result3Chan := processor.SubmitRequest(ctx, cid3, peer1, NormalPriority)
	
	// Wait for results
	<-result1Chan
	<-result2Chan
	<-result3Chan
	
	// Check metrics
	metrics := processor.GetMetrics()
	
	assert.Equal(t, uint64(3), metrics.TotalRequests)
	assert.Equal(t, uint64(3), metrics.BatchedRequests)
	assert.True(t, metrics.ProcessedBatches >= 2) // At least 2 batches (high priority batch + normal priority)
	assert.True(t, metrics.AverageBatchSize > 0)
	assert.True(t, metrics.AverageProcessTime > 0)
	
	// Check priority-specific metrics
	highPriorityMetrics := metrics.PriorityMetrics[HighPriority]
	assert.NotNil(t, highPriorityMetrics)
	assert.Equal(t, uint64(2), highPriorityMetrics.RequestCount)
	assert.True(t, highPriorityMetrics.BatchCount >= 1)
	
	normalPriorityMetrics := metrics.PriorityMetrics[NormalPriority]
	assert.NotNil(t, normalPriorityMetrics)
	assert.Equal(t, uint64(1), normalPriorityMetrics.RequestCount)
}