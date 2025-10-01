package adaptive

import (
	"context"
	"sync"
	"time"

	blocks "github.com/ipfs/go-block-format"
	"github.com/ipfs/go-cid"
	logging "github.com/ipfs/go-log/v2"
	"github.com/libp2p/go-libp2p/core/peer"
)

var batchLog = logging.Logger("bitswap/adaptive/batch")

// BatchRequest represents a single request that can be batched
type BatchRequest struct {
	CID        cid.Cid
	PeerID     peer.ID
	Priority   RequestPriority
	Timestamp  time.Time
	ResultChan chan BatchResult
	Context    context.Context
}

// BatchResult represents the result of a batch request
type BatchResult struct {
	Block blocks.Block
	Error error
}

// RequestPriority defines the priority level of a request
type RequestPriority int

const (
	LowPriority RequestPriority = iota
	NormalPriority
	HighPriority
	CriticalPriority
)

// BatchRequestProcessor handles grouping and parallel processing of bitswap requests
type BatchRequestProcessor struct {
	config *AdaptiveBitswapConfig
	
	// Request channels for different priorities
	criticalChan chan *BatchRequest
	highChan     chan *BatchRequest
	normalChan   chan *BatchRequest
	lowChan      chan *BatchRequest
	
	// Worker pools for different priorities
	criticalWorkers *BatchWorkerPool
	highWorkers     *BatchWorkerPool
	normalWorkers   *BatchWorkerPool
	lowWorkers      *BatchWorkerPool
	
	// Batch accumulation
	batchMutex     sync.Mutex
	pendingBatches map[RequestPriority][]*BatchRequest
	batchTimers    map[RequestPriority]*time.Timer
	
	// Request handler function
	requestHandler func(ctx context.Context, requests []*BatchRequest) []BatchResult
	
	// Control channels
	ctx    context.Context
	cancel context.CancelFunc
	wg     sync.WaitGroup
	
	// Metrics
	metrics *BatchProcessorMetrics
}

// BatchProcessorMetrics tracks performance metrics for the batch processor
type BatchProcessorMetrics struct {
	mu sync.RWMutex
	
	TotalRequests       uint64
	BatchedRequests     uint64
	ProcessedBatches    uint64
	AverageBatchSize    float64
	AverageProcessTime  time.Duration
	ThroughputPerSecond float64
	
	// Per-priority metrics
	PriorityMetrics map[RequestPriority]*PriorityMetrics
}

// PriorityMetrics tracks metrics for a specific priority level
type PriorityMetrics struct {
	RequestCount    uint64
	BatchCount      uint64
	AverageLatency  time.Duration
	ThroughputRPS   float64
}

// BatchWorkerPool manages a pool of workers for processing batches
type BatchWorkerPool struct {
	workers    int
	workChan   chan func()
	ctx        context.Context
	cancel     context.CancelFunc
	wg         sync.WaitGroup
}

// NewBatchRequestProcessor creates a new batch request processor
func NewBatchRequestProcessor(ctx context.Context, config *AdaptiveBitswapConfig, requestHandler func(context.Context, []*BatchRequest) []BatchResult) *BatchRequestProcessor {
	processorCtx, cancel := context.WithCancel(ctx)
	
	processor := &BatchRequestProcessor{
		config:         config,
		criticalChan:   make(chan *BatchRequest, 1000),
		highChan:       make(chan *BatchRequest, 1000),
		normalChan:     make(chan *BatchRequest, 1000),
		lowChan:        make(chan *BatchRequest, 1000),
		pendingBatches: make(map[RequestPriority][]*BatchRequest),
		batchTimers:    make(map[RequestPriority]*time.Timer),
		requestHandler: requestHandler,
		ctx:            processorCtx,
		cancel:         cancel,
		metrics:        NewBatchProcessorMetrics(),
	}
	
	// Initialize pending batches for all priorities
	processor.pendingBatches[CriticalPriority] = make([]*BatchRequest, 0)
	processor.pendingBatches[HighPriority] = make([]*BatchRequest, 0)
	processor.pendingBatches[NormalPriority] = make([]*BatchRequest, 0)
	processor.pendingBatches[LowPriority] = make([]*BatchRequest, 0)
	
	// Create worker pools with different sizes based on priority
	processor.criticalWorkers = NewBatchWorkerPool(processorCtx, config.GetCurrentWorkerCount()/4) // 25% for critical
	processor.highWorkers = NewBatchWorkerPool(processorCtx, config.GetCurrentWorkerCount()/4)     // 25% for high
	processor.normalWorkers = NewBatchWorkerPool(processorCtx, config.GetCurrentWorkerCount()/3)   // 33% for normal
	processor.lowWorkers = NewBatchWorkerPool(processorCtx, config.GetCurrentWorkerCount()/6)      // 17% for low
	
	// Start batch processors for each priority
	processor.wg.Add(4)
	go processor.processPriorityQueue(CriticalPriority, processor.criticalChan, processor.criticalWorkers)
	go processor.processPriorityQueue(HighPriority, processor.highChan, processor.highWorkers)
	go processor.processPriorityQueue(NormalPriority, processor.normalChan, processor.normalWorkers)
	go processor.processPriorityQueue(LowPriority, processor.lowChan, processor.lowWorkers)
	
	batchLog.Info("BatchRequestProcessor started with adaptive configuration")
	
	return processor
}

// SubmitRequest submits a request for batch processing
func (p *BatchRequestProcessor) SubmitRequest(ctx context.Context, cid cid.Cid, peerID peer.ID, priority RequestPriority) <-chan BatchResult {
	resultChan := make(chan BatchResult, 1)
	
	request := &BatchRequest{
		CID:        cid,
		PeerID:     peerID,
		Priority:   priority,
		Timestamp:  time.Now(),
		ResultChan: resultChan,
		Context:    ctx,
	}
	
	// Update metrics
	p.metrics.IncrementTotalRequests()
	
	// Route to appropriate priority channel
	select {
	case <-ctx.Done():
		close(resultChan)
		return resultChan
	default:
	}
	
	switch priority {
	case CriticalPriority:
		select {
		case p.criticalChan <- request:
		case <-ctx.Done():
			close(resultChan)
		}
	case HighPriority:
		select {
		case p.highChan <- request:
		case <-ctx.Done():
			close(resultChan)
		}
	case NormalPriority:
		select {
		case p.normalChan <- request:
		case <-ctx.Done():
			close(resultChan)
		}
	case LowPriority:
		select {
		case p.lowChan <- request:
		case <-ctx.Done():
			close(resultChan)
		}
	}
	
	return resultChan
}

// processPriorityQueue processes requests for a specific priority level
func (p *BatchRequestProcessor) processPriorityQueue(priority RequestPriority, requestChan <-chan *BatchRequest, workerPool *BatchWorkerPool) {
	defer p.wg.Done()
	
	batchSize, batchTimeout := p.config.GetBatchConfig()
	
	for {
		select {
		case <-p.ctx.Done():
			return
		case request := <-requestChan:
			p.addToBatch(priority, request, batchSize, batchTimeout, workerPool)
		}
	}
}

// addToBatch adds a request to the appropriate batch and triggers processing if needed
func (p *BatchRequestProcessor) addToBatch(priority RequestPriority, request *BatchRequest, batchSize int, batchTimeout time.Duration, workerPool *BatchWorkerPool) {
	p.batchMutex.Lock()
	defer p.batchMutex.Unlock()
	
	// Add request to pending batch
	p.pendingBatches[priority] = append(p.pendingBatches[priority], request)
	
	// Check if batch is full
	if len(p.pendingBatches[priority]) >= batchSize {
		p.processBatch(priority, workerPool)
		return
	}
	
	// Set timer for batch timeout if this is the first request in the batch
	if len(p.pendingBatches[priority]) == 1 {
		if timer, exists := p.batchTimers[priority]; exists {
			timer.Stop()
		}
		
		p.batchTimers[priority] = time.AfterFunc(batchTimeout, func() {
			p.batchMutex.Lock()
			defer p.batchMutex.Unlock()
			
			if len(p.pendingBatches[priority]) > 0 {
				p.processBatch(priority, workerPool)
			}
		})
	}
}

// processBatch processes a batch of requests using the worker pool
func (p *BatchRequestProcessor) processBatch(priority RequestPriority, workerPool *BatchWorkerPool) {
	if len(p.pendingBatches[priority]) == 0 {
		return
	}
	
	// Extract batch
	batch := make([]*BatchRequest, len(p.pendingBatches[priority]))
	copy(batch, p.pendingBatches[priority])
	p.pendingBatches[priority] = p.pendingBatches[priority][:0] // Clear slice
	
	// Stop timer if it exists
	if timer, exists := p.batchTimers[priority]; exists {
		timer.Stop()
		delete(p.batchTimers, priority)
	}
	
	// Update metrics
	p.metrics.IncrementBatchedRequests(uint64(len(batch)))
	p.metrics.IncrementProcessedBatches()
	
	batchLog.Debugf("Processing batch of %d requests with priority %v", len(batch), priority)
	
	// Submit batch to worker pool
	workerPool.Submit(func() {
		startTime := time.Now()
		
		// Process the batch
		results := p.requestHandler(p.ctx, batch)
		
		// Send results back to requesters
		for i, request := range batch {
			if i < len(results) {
				select {
				case request.ResultChan <- results[i]:
				case <-request.Context.Done():
				case <-p.ctx.Done():
				}
			} else {
				// Handle case where not enough results returned
				select {
				case request.ResultChan <- BatchResult{Error: context.DeadlineExceeded}:
				case <-request.Context.Done():
				case <-p.ctx.Done():
				}
			}
			close(request.ResultChan)
		}
		
		// Update processing time metrics
		processingTime := time.Since(startTime)
		p.metrics.UpdateProcessingTime(processingTime)
		p.metrics.UpdatePriorityMetrics(priority, len(batch), processingTime)
		
		batchLog.Debugf("Completed batch of %d requests in %v", len(batch), processingTime)
	})
}

// UpdateConfiguration updates the processor configuration and adjusts worker pools
func (p *BatchRequestProcessor) UpdateConfiguration(config *AdaptiveBitswapConfig) {
	p.config = config
	
	// Update worker pool sizes based on new configuration
	newWorkerCount := config.GetCurrentWorkerCount()
	
	p.criticalWorkers.Resize(newWorkerCount / 4)
	p.highWorkers.Resize(newWorkerCount / 4)
	p.normalWorkers.Resize(newWorkerCount / 3)
	p.lowWorkers.Resize(newWorkerCount / 6)
	
	batchLog.Infof("Updated BatchRequestProcessor configuration: workers=%d", newWorkerCount)
}

// GetMetrics returns current batch processor metrics
func (p *BatchRequestProcessor) GetMetrics() *BatchProcessorMetrics {
	return p.metrics.GetSnapshot()
}

// Close shuts down the batch processor
func (p *BatchRequestProcessor) Close() error {
	batchLog.Info("Shutting down BatchRequestProcessor")
	
	p.cancel()
	p.wg.Wait()
	
	// Close worker pools
	p.criticalWorkers.Close()
	p.highWorkers.Close()
	p.normalWorkers.Close()
	p.lowWorkers.Close()
	
	// Stop any remaining timers
	p.batchMutex.Lock()
	for _, timer := range p.batchTimers {
		timer.Stop()
	}
	p.batchMutex.Unlock()
	
	batchLog.Info("BatchRequestProcessor shutdown complete")
	return nil
}

// NewBatchProcessorMetrics creates a new metrics instance
func NewBatchProcessorMetrics() *BatchProcessorMetrics {
	return &BatchProcessorMetrics{
		PriorityMetrics: map[RequestPriority]*PriorityMetrics{
			CriticalPriority: &PriorityMetrics{},
			HighPriority:     &PriorityMetrics{},
			NormalPriority:   &PriorityMetrics{},
			LowPriority:      &PriorityMetrics{},
		},
	}
}

// IncrementTotalRequests increments the total request counter
func (m *BatchProcessorMetrics) IncrementTotalRequests() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.TotalRequests++
}

// IncrementBatchedRequests increments the batched request counter
func (m *BatchProcessorMetrics) IncrementBatchedRequests(count uint64) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.BatchedRequests += count
}

// IncrementProcessedBatches increments the processed batch counter
func (m *BatchProcessorMetrics) IncrementProcessedBatches() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.ProcessedBatches++
	
	// Update average batch size
	if m.ProcessedBatches > 0 {
		m.AverageBatchSize = float64(m.BatchedRequests) / float64(m.ProcessedBatches)
	}
}

// UpdateProcessingTime updates the average processing time
func (m *BatchProcessorMetrics) UpdateProcessingTime(duration time.Duration) {
	m.mu.Lock()
	defer m.mu.Unlock()
	
	// Simple moving average
	if m.AverageProcessTime == 0 {
		m.AverageProcessTime = duration
	} else {
		m.AverageProcessTime = (m.AverageProcessTime + duration) / 2
	}
}

// UpdatePriorityMetrics updates metrics for a specific priority level
func (m *BatchProcessorMetrics) UpdatePriorityMetrics(priority RequestPriority, batchSize int, processingTime time.Duration) {
	m.mu.Lock()
	defer m.mu.Unlock()
	
	if metrics, exists := m.PriorityMetrics[priority]; exists {
		metrics.RequestCount += uint64(batchSize)
		metrics.BatchCount++
		
		// Update average latency
		if metrics.AverageLatency == 0 {
			metrics.AverageLatency = processingTime
		} else {
			metrics.AverageLatency = (metrics.AverageLatency + processingTime) / 2
		}
	}
}

// GetSnapshot returns a thread-safe snapshot of the metrics
func (m *BatchProcessorMetrics) GetSnapshot() *BatchProcessorMetrics {
	m.mu.RLock()
	defer m.mu.RUnlock()
	
	snapshot := &BatchProcessorMetrics{
		TotalRequests:       m.TotalRequests,
		BatchedRequests:     m.BatchedRequests,
		ProcessedBatches:    m.ProcessedBatches,
		AverageBatchSize:    m.AverageBatchSize,
		AverageProcessTime:  m.AverageProcessTime,
		ThroughputPerSecond: m.ThroughputPerSecond,
		PriorityMetrics:     make(map[RequestPriority]*PriorityMetrics),
	}
	
	// Deep copy priority metrics
	for priority, metrics := range m.PriorityMetrics {
		snapshot.PriorityMetrics[priority] = &PriorityMetrics{
			RequestCount:   metrics.RequestCount,
			BatchCount:     metrics.BatchCount,
			AverageLatency: metrics.AverageLatency,
			ThroughputRPS:  metrics.ThroughputRPS,
		}
	}
	
	return snapshot
}

// NewBatchWorkerPool creates a new batch worker pool
func NewBatchWorkerPool(ctx context.Context, workers int) *BatchWorkerPool {
	poolCtx, cancel := context.WithCancel(ctx)
	
	pool := &BatchWorkerPool{
		workers:  workers,
		workChan: make(chan func(), workers*2),
		ctx:      poolCtx,
		cancel:   cancel,
	}
	
	// Start workers
	for i := 0; i < workers; i++ {
		pool.wg.Add(1)
		go pool.worker()
	}
	
	return pool
}

// Submit submits a task to the batch worker pool
func (p *BatchWorkerPool) Submit(task func()) {
	select {
	case p.workChan <- task:
		// Task queued successfully
	case <-p.ctx.Done():
		// Pool is shutting down
	default:
		// Channel is full, execute in current goroutine as fallback
		task()
	}
}

// worker runs the worker loop
func (p *BatchWorkerPool) worker() {
	defer p.wg.Done()
	
	for {
		select {
		case <-p.ctx.Done():
			return
		case task := <-p.workChan:
			if task != nil {
				// Execute task with panic recovery
				func() {
					defer func() {
						if r := recover(); r != nil {
							batchLog.Errorf("BatchWorker panic recovered: %v", r)
						}
					}()
					task()
				}()
			}
		}
	}
}

// Resize changes the number of workers (simplified implementation)
func (p *BatchWorkerPool) Resize(newSize int) {
	// For simplicity, we don't implement dynamic resizing in BatchWorkerPool
	// The main WorkerPool handles this functionality
	batchLog.Debugf("BatchWorkerPool resize requested to %d workers (not implemented)", newSize)
}

// Close shuts down the batch worker pool
func (p *BatchWorkerPool) Close() error {
	p.cancel()
	p.wg.Wait()
	close(p.workChan)
	return nil
}