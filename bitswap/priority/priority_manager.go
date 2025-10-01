// Package priority provides request prioritization and resource management
// for Bitswap to handle high-load scenarios efficiently.
package priority

import (
	"container/heap"
	"context"
	"sync"
	"time"

	"github.com/ipfs/go-cid"
	logging "github.com/ipfs/go-log/v2"
	"github.com/libp2p/go-libp2p/core/peer"
)

var log = logging.Logger("bitswap/priority")

// Priority levels for request classification
type Priority int

const (
	LowPriority Priority = iota
	NormalPriority
	HighPriority
	CriticalPriority
)

func (p Priority) String() string {
	switch p {
	case LowPriority:
		return "low"
	case NormalPriority:
		return "normal"
	case HighPriority:
		return "high"
	case CriticalPriority:
		return "critical"
	default:
		return "unknown"
	}
}

// RequestContext contains metadata about a request for priority classification
type RequestContext struct {
	CID           cid.Cid
	PeerID        peer.ID
	RequestTime   time.Time
	Deadline      time.Time
	SessionID     string
	BlockSize     int64
	RetryCount    int
	IsSession     bool
	UserPriority  Priority // User-specified priority
}

// PriorityRequest represents a prioritized request in the queue
type PriorityRequest struct {
	Context   RequestContext
	Priority  Priority
	QueueTime time.Time
	Index     int // Index in the heap
}

// PriorityQueue implements a priority queue for requests
type PriorityQueue []*PriorityRequest

func (pq PriorityQueue) Len() int { return len(pq) }

func (pq PriorityQueue) Less(i, j int) bool {
	// Higher priority values come first
	if pq[i].Priority != pq[j].Priority {
		return pq[i].Priority > pq[j].Priority
	}
	// For same priority, earlier requests come first
	return pq[i].QueueTime.Before(pq[j].QueueTime)
}

func (pq PriorityQueue) Swap(i, j int) {
	pq[i], pq[j] = pq[j], pq[i]
	pq[i].Index = i
	pq[j].Index = j
}

func (pq *PriorityQueue) Push(x interface{}) {
	n := len(*pq)
	item := x.(*PriorityRequest)
	item.Index = n
	*pq = append(*pq, item)
}

func (pq *PriorityQueue) Pop() interface{} {
	old := *pq
	n := len(old)
	item := old[n-1]
	old[n-1] = nil  // avoid memory leak
	item.Index = -1 // for safety
	*pq = old[0 : n-1]
	return item
}

// ResourcePool manages available resources for request processing
type ResourcePool struct {
	mu                    sync.RWMutex
	maxConcurrentRequests int
	currentRequests       int
	maxBandwidthPerSec    int64
	currentBandwidthUsage int64
	lastBandwidthReset    time.Time
}

// NewResourcePool creates a new resource pool with specified limits
func NewResourcePool(maxConcurrent int, maxBandwidthPerSec int64) *ResourcePool {
	return &ResourcePool{
		maxConcurrentRequests: maxConcurrent,
		maxBandwidthPerSec:    maxBandwidthPerSec,
		lastBandwidthReset:    time.Now(),
	}
}

// CanAllocate checks if resources can be allocated for a request
func (rp *ResourcePool) CanAllocate(estimatedSize int64) bool {
	rp.mu.RLock()
	defer rp.mu.RUnlock()

	// Check concurrent request limit
	if rp.currentRequests >= rp.maxConcurrentRequests {
		return false
	}

	// Reset bandwidth counter if a second has passed
	now := time.Now()
	if now.Sub(rp.lastBandwidthReset) >= time.Second {
		rp.currentBandwidthUsage = 0
		rp.lastBandwidthReset = now
	}

	// Check bandwidth limit
	if rp.currentBandwidthUsage+estimatedSize > rp.maxBandwidthPerSec {
		return false
	}

	return true
}

// Allocate reserves resources for a request
func (rp *ResourcePool) Allocate(estimatedSize int64) bool {
	rp.mu.Lock()
	defer rp.mu.Unlock()

	// Reset bandwidth counter if needed
	now := time.Now()
	if now.Sub(rp.lastBandwidthReset) >= time.Second {
		rp.currentBandwidthUsage = 0
		rp.lastBandwidthReset = now
	}

	// Check limits again under write lock
	if rp.currentRequests >= rp.maxConcurrentRequests ||
		rp.currentBandwidthUsage+estimatedSize > rp.maxBandwidthPerSec {
		return false
	}

	rp.currentRequests++
	rp.currentBandwidthUsage += estimatedSize
	return true
}

// Release frees resources after request completion
func (rp *ResourcePool) Release(actualSize int64) {
	rp.mu.Lock()
	defer rp.mu.Unlock()

	if rp.currentRequests > 0 {
		rp.currentRequests--
	}
	// Note: We don't subtract from bandwidth usage as it's reset every second
}

// GetStats returns current resource usage statistics
func (rp *ResourcePool) GetStats() (int, int, int64, int64) {
	rp.mu.RLock()
	defer rp.mu.RUnlock()
	return rp.currentRequests, rp.maxConcurrentRequests, rp.currentBandwidthUsage, rp.maxBandwidthPerSec
}

// UpdateLimits dynamically adjusts resource limits
func (rp *ResourcePool) UpdateLimits(maxConcurrent int, maxBandwidthPerSec int64) {
	rp.mu.Lock()
	defer rp.mu.Unlock()
	rp.maxConcurrentRequests = maxConcurrent
	rp.maxBandwidthPerSec = maxBandwidthPerSec
}

// PriorityRequestManager manages prioritized request queues and resource allocation
type PriorityRequestManager struct {
	mu sync.RWMutex

	// Priority queues for different priority levels
	queues map[Priority]*PriorityQueue

	// Resource management
	resourcePool *ResourcePool

	// Configuration
	highPriorityThreshold     time.Duration
	criticalPriorityThreshold time.Duration
	maxQueueSize              int
	processingTimeout         time.Duration

	// Statistics
	stats *ManagerStats

	// Channels for request processing
	requestChan  chan *PriorityRequest
	responseChan chan *RequestResult
	stopChan     chan struct{}
	wg           sync.WaitGroup

	// Callbacks
	onRequestProcessed func(*PriorityRequest, *RequestResult)
}

// ManagerStats tracks statistics for the priority manager
type ManagerStats struct {
	mu                    sync.RWMutex
	TotalRequests         int64
	ProcessedRequests     int64
	DroppedRequests       int64
	AverageWaitTime       time.Duration
	RequestsByPriority    map[Priority]int64
	ResourceUtilization   float64
	LastStatsUpdate       time.Time
}

// RequestResult contains the result of processing a prioritized request
type RequestResult struct {
	Request     *PriorityRequest
	Success     bool
	Error       error
	ProcessTime time.Duration
	DataSize    int64
}

// NewPriorityRequestManager creates a new priority request manager
func NewPriorityRequestManager(
	maxConcurrentRequests int,
	maxBandwidthPerSec int64,
	highPriorityThreshold time.Duration,
	criticalPriorityThreshold time.Duration,
) *PriorityRequestManager {
	prm := &PriorityRequestManager{
		queues: make(map[Priority]*PriorityQueue),
		resourcePool: NewResourcePool(maxConcurrentRequests, maxBandwidthPerSec),
		highPriorityThreshold:     highPriorityThreshold,
		criticalPriorityThreshold: criticalPriorityThreshold,
		maxQueueSize:              10000, // Default max queue size
		processingTimeout:         30 * time.Second,
		stats: &ManagerStats{
			RequestsByPriority: make(map[Priority]int64),
			LastStatsUpdate:    time.Now(),
		},
		requestChan:  make(chan *PriorityRequest, 1000),
		responseChan: make(chan *RequestResult, 1000),
		stopChan:     make(chan struct{}),
	}

	// Initialize priority queues
	for _, priority := range []Priority{LowPriority, NormalPriority, HighPriority, CriticalPriority} {
		pq := &PriorityQueue{}
		heap.Init(pq)
		prm.queues[priority] = pq
	}

	return prm
}

// Start begins processing requests in the background
func (prm *PriorityRequestManager) Start(ctx context.Context) {
	prm.wg.Add(1)
	go prm.processRequests(ctx)
}

// Stop gracefully shuts down the request manager
func (prm *PriorityRequestManager) Stop() {
	close(prm.stopChan)
	prm.wg.Wait()
}

// ClassifyRequest determines the priority of a request based on its context
func (prm *PriorityRequestManager) ClassifyRequest(ctx RequestContext) Priority {
	// Start with user-specified priority if provided
	if ctx.UserPriority != LowPriority {
		return ctx.UserPriority
	}

	now := time.Now()
	
	// Calculate time until deadline
	timeToDeadline := ctx.Deadline.Sub(now)
	
	// Critical priority for very urgent requests
	if timeToDeadline <= prm.criticalPriorityThreshold {
		return CriticalPriority
	}
	
	// High priority for urgent requests
	if timeToDeadline <= prm.highPriorityThreshold {
		return HighPriority
	}
	
	// Consider retry count - higher retries get higher priority
	if ctx.RetryCount >= 3 {
		return HighPriority
	}
	
	// Session requests get slightly higher priority
	if ctx.IsSession {
		return NormalPriority
	}
	
	// Default to low priority
	return LowPriority
}

// EnqueueRequest adds a request to the appropriate priority queue
func (prm *PriorityRequestManager) EnqueueRequest(ctx RequestContext) error {
	priority := prm.ClassifyRequest(ctx)
	
	request := &PriorityRequest{
		Context:   ctx,
		Priority:  priority,
		QueueTime: time.Now(),
	}

	prm.mu.Lock()
	defer prm.mu.Unlock()

	// Check queue size limits
	queue := prm.queues[priority]
	if queue.Len() >= prm.maxQueueSize {
		prm.updateStats(func(s *ManagerStats) {
			s.DroppedRequests++
		})
		return ErrQueueFull
	}

	// Add to priority queue
	heap.Push(queue, request)
	
	prm.updateStats(func(s *ManagerStats) {
		s.TotalRequests++
		s.RequestsByPriority[priority]++
	})

	log.Debugf("Enqueued request for CID %s with priority %s (queue size: %d)", 
		ctx.CID, priority, queue.Len())

	// Try to process immediately if resources are available
	select {
	case prm.requestChan <- request:
		// Successfully sent for processing
	default:
		// Channel full, will be processed later
	}

	return nil
}

// processRequests is the main processing loop that handles prioritized requests
func (prm *PriorityRequestManager) processRequests(ctx context.Context) {
	defer prm.wg.Done()

	ticker := time.NewTicker(100 * time.Millisecond) // Check for work every 100ms
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-prm.stopChan:
			return
		case <-ticker.C:
			prm.processNextRequest()
		case request := <-prm.requestChan:
			prm.handleRequest(request)
		case result := <-prm.responseChan:
			prm.handleResult(result)
		}
	}
}

// processNextRequest finds and processes the highest priority request
func (prm *PriorityRequestManager) processNextRequest() {
	prm.mu.Lock()
	defer prm.mu.Unlock()

	// Find highest priority non-empty queue
	for _, priority := range []Priority{CriticalPriority, HighPriority, NormalPriority, LowPriority} {
		queue := prm.queues[priority]
		if queue.Len() > 0 {
			// Check if we can allocate resources
			request := (*queue)[0] // Peek at top request
			if prm.resourcePool.CanAllocate(request.Context.BlockSize) {
				// Remove from queue and process
				item := heap.Pop(queue).(*PriorityRequest)
				go prm.processRequest(item)
				return
			}
		}
	}
}

// handleRequest processes a request that was sent via the request channel
func (prm *PriorityRequestManager) handleRequest(request *PriorityRequest) {
	if prm.resourcePool.Allocate(request.Context.BlockSize) {
		go prm.processRequest(request)
	} else {
		// Put back in queue if resources not available
		prm.mu.Lock()
		queue := prm.queues[request.Priority]
		heap.Push(queue, request)
		prm.mu.Unlock()
	}
}

// processRequest handles the actual processing of a request
func (prm *PriorityRequestManager) processRequest(request *PriorityRequest) {
	startTime := time.Now()
	
	// Simulate request processing (in real implementation, this would call bitswap)
	// For now, we'll just track the request and simulate completion
	
	result := &RequestResult{
		Request:     request,
		Success:     true,
		ProcessTime: time.Since(startTime),
		DataSize:    request.Context.BlockSize,
	}

	// Send result back
	select {
	case prm.responseChan <- result:
	default:
		// Channel full, handle synchronously
		prm.handleResult(result)
	}
}

// handleResult processes the result of a completed request
func (prm *PriorityRequestManager) handleResult(result *RequestResult) {
	// Release resources
	prm.resourcePool.Release(result.DataSize)
	
	// Update statistics
	waitTime := result.Request.QueueTime.Sub(result.Request.Context.RequestTime)
	prm.updateStats(func(s *ManagerStats) {
		s.ProcessedRequests++
		// Update average wait time (simple moving average)
		if s.ProcessedRequests == 1 {
			s.AverageWaitTime = waitTime
		} else {
			s.AverageWaitTime = time.Duration(
				(int64(s.AverageWaitTime)*9 + int64(waitTime)) / 10,
			)
		}
	})

	log.Debugf("Processed request for CID %s (priority: %s, wait: %v, process: %v)",
		result.Request.Context.CID, result.Request.Priority, waitTime, result.ProcessTime)

	// Call callback if set
	if prm.onRequestProcessed != nil {
		prm.onRequestProcessed(result.Request, result)
	}
}

// updateStats safely updates statistics
func (prm *PriorityRequestManager) updateStats(updateFunc func(*ManagerStats)) {
	prm.stats.mu.Lock()
	defer prm.stats.mu.Unlock()
	updateFunc(prm.stats)
	prm.stats.LastStatsUpdate = time.Now()
}

// GetStats returns current manager statistics
func (prm *PriorityRequestManager) GetStats() ManagerStats {
	prm.stats.mu.RLock()
	defer prm.stats.mu.RUnlock()
	
	// Calculate resource utilization
	current, max, _, _ := prm.resourcePool.GetStats()
	utilization := float64(current) / float64(max)
	
	stats := *prm.stats
	stats.ResourceUtilization = utilization
	return stats
}

// UpdateConfiguration dynamically updates manager configuration
func (prm *PriorityRequestManager) UpdateConfiguration(
	maxConcurrentRequests int,
	maxBandwidthPerSec int64,
	highPriorityThreshold time.Duration,
	criticalPriorityThreshold time.Duration,
) {
	prm.mu.Lock()
	defer prm.mu.Unlock()
	
	prm.resourcePool.UpdateLimits(maxConcurrentRequests, maxBandwidthPerSec)
	prm.highPriorityThreshold = highPriorityThreshold
	prm.criticalPriorityThreshold = criticalPriorityThreshold
	
	log.Infof("Updated priority manager configuration: maxConcurrent=%d, maxBandwidth=%d, highThreshold=%v, criticalThreshold=%v",
		maxConcurrentRequests, maxBandwidthPerSec, highPriorityThreshold, criticalPriorityThreshold)
}

// SetRequestProcessedCallback sets a callback for when requests are processed
func (prm *PriorityRequestManager) SetRequestProcessedCallback(callback func(*PriorityRequest, *RequestResult)) {
	prm.mu.Lock()
	defer prm.mu.Unlock()
	prm.onRequestProcessed = callback
}

// GetQueueSizes returns the current size of each priority queue
func (prm *PriorityRequestManager) GetQueueSizes() map[Priority]int {
	prm.mu.RLock()
	defer prm.mu.RUnlock()
	
	sizes := make(map[Priority]int)
	for priority, queue := range prm.queues {
		sizes[priority] = queue.Len()
	}
	return sizes
}