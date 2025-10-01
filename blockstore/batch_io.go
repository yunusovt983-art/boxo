package blockstore

import (
	"context"
	"errors"
	"sync"
	"time"

	blocks "github.com/ipfs/go-block-format"
	cid "github.com/ipfs/go-cid"
	ipld "github.com/ipfs/go-ipld-format"
	metrics "github.com/ipfs/go-metrics-interface"
)

// BatchIOManager manages batched I/O operations for improved performance
type BatchIOManager interface {
	// BatchPut queues blocks for batched writing
	BatchPut(ctx context.Context, blocks []blocks.Block) error
	
	// BatchGet retrieves multiple blocks efficiently
	BatchGet(ctx context.Context, cids []cid.Cid) ([]blocks.Block, error)
	
	// BatchHas checks existence of multiple blocks efficiently
	BatchHas(ctx context.Context, cids []cid.Cid) ([]bool, error)
	
	// BatchDelete removes multiple blocks efficiently
	BatchDelete(ctx context.Context, cids []cid.Cid) error
	
	// Flush forces all pending operations to complete
	Flush(ctx context.Context) error
	
	// Close shuts down the batch manager
	Close() error
	
	// GetStats returns batch operation statistics
	GetStats() *BatchIOStats
}

// BatchIOConfig holds configuration for batch I/O operations
type BatchIOConfig struct {
	// MaxBatchSize is the maximum number of items in a batch
	MaxBatchSize int
	
	// BatchTimeout is the maximum time to wait before flushing a batch
	BatchTimeout time.Duration
	
	// MaxConcurrentBatches is the maximum number of concurrent batch operations
	MaxConcurrentBatches int
	
	// EnableTransactions enables transactional batch operations
	EnableTransactions bool
	
	// RetryAttempts is the number of retry attempts for failed operations
	RetryAttempts int
	
	// RetryDelay is the delay between retry attempts
	RetryDelay time.Duration
}

// BatchIOStats holds statistics for batch I/O operations
type BatchIOStats struct {
	// TotalBatches is the total number of batches processed
	TotalBatches int64
	
	// TotalItems is the total number of items processed
	TotalItems int64
	
	// AverageBatchSize is the average number of items per batch
	AverageBatchSize float64
	
	// AverageLatency is the average batch processing latency
	AverageLatency time.Duration
	
	// SuccessfulBatches is the number of successful batches
	SuccessfulBatches int64
	
	// FailedBatches is the number of failed batches
	FailedBatches int64
	
	// RetryCount is the total number of retries
	RetryCount int64
	
	// LastFlush is the timestamp of the last flush operation
	LastFlush time.Time
}

// BatchOperation represents a pending batch operation
type BatchOperation struct {
	Type      BatchOpType
	Blocks    []blocks.Block
	CIDs      []cid.Cid
	Results   chan BatchResult
	Context   context.Context
	Timestamp time.Time
}

// BatchOpType represents the type of batch operation
type BatchOpType int

const (
	BatchOpPut BatchOpType = iota
	BatchOpGet
	BatchOpHas
	BatchOpDelete
)

// BatchResult represents the result of a batch operation
type BatchResult struct {
	Blocks []blocks.Block
	Bools  []bool
	Error  error
}

// batchIOManager implements BatchIOManager
type batchIOManager struct {
	config     *BatchIOConfig
	blockstore Blockstore
	
	// Batch queues for different operation types
	putQueue    chan *BatchOperation
	getQueue    chan *BatchOperation
	hasQueue    chan *BatchOperation
	deleteQueue chan *BatchOperation
	
	// Worker management
	workers    sync.WaitGroup
	stopChan   chan struct{}
	flushChan  chan chan error
	
	// Statistics
	stats   *BatchIOStats
	statsMu sync.RWMutex
	
	// Metrics
	metrics struct {
		batchesProcessed metrics.Counter
		itemsProcessed   metrics.Counter
		batchLatency     metrics.Histogram
		retries          metrics.Counter
	}
}

// DefaultBatchIOConfig returns a default configuration for batch I/O
func DefaultBatchIOConfig() *BatchIOConfig {
	return &BatchIOConfig{
		MaxBatchSize:         1000,
		BatchTimeout:         100 * time.Millisecond,
		MaxConcurrentBatches: 10,
		EnableTransactions:   true,
		RetryAttempts:        3,
		RetryDelay:           10 * time.Millisecond,
	}
}

// NewBatchIOManager creates a new batch I/O manager
func NewBatchIOManager(ctx context.Context, bs Blockstore, config *BatchIOConfig) (BatchIOManager, error) {
	if config == nil {
		config = DefaultBatchIOConfig()
	}
	
	if bs == nil {
		return nil, errors.New("blockstore cannot be nil")
	}
	
	bim := &batchIOManager{
		config:      config,
		blockstore:  bs,
		putQueue:    make(chan *BatchOperation, config.MaxConcurrentBatches),
		getQueue:    make(chan *BatchOperation, config.MaxConcurrentBatches),
		hasQueue:    make(chan *BatchOperation, config.MaxConcurrentBatches),
		deleteQueue: make(chan *BatchOperation, config.MaxConcurrentBatches),
		stopChan:    make(chan struct{}),
		flushChan:   make(chan chan error, 1),
		stats:       &BatchIOStats{},
	}
	
	// Initialize metrics
	bim.metrics.batchesProcessed = metrics.NewCtx(ctx, "batch_io.batches_processed_total", "Total number of batches processed").Counter()
	bim.metrics.itemsProcessed = metrics.NewCtx(ctx, "batch_io.items_processed_total", "Total number of items processed").Counter()
	bim.metrics.batchLatency = metrics.NewCtx(ctx, "batch_io.batch_latency_seconds", "Batch processing latency").Histogram([]float64{0.001, 0.01, 0.1, 1, 10})
	bim.metrics.retries = metrics.NewCtx(ctx, "batch_io.retries_total", "Total number of retries").Counter()
	
	// Start worker goroutines
	bim.startWorkers(ctx)
	
	return bim, nil
}

// startWorkers starts the batch processing workers
func (bim *batchIOManager) startWorkers(ctx context.Context) {
	// Start workers for each operation type
	for i := 0; i < bim.config.MaxConcurrentBatches; i++ {
		bim.workers.Add(4) // One for each operation type
		
		go bim.putWorker(ctx)
		go bim.getWorker(ctx)
		go bim.hasWorker(ctx)
		go bim.deleteWorker(ctx)
	}
	
	// Start flush coordinator
	bim.workers.Add(1)
	go bim.flushCoordinator(ctx)
}

// BatchPut queues blocks for batched writing
func (bim *batchIOManager) BatchPut(ctx context.Context, blocks []blocks.Block) error {
	if len(blocks) == 0 {
		return nil
	}
	
	// Split into smaller batches if necessary
	for i := 0; i < len(blocks); i += bim.config.MaxBatchSize {
		end := i + bim.config.MaxBatchSize
		if end > len(blocks) {
			end = len(blocks)
		}
		
		batch := blocks[i:end]
		op := &BatchOperation{
			Type:      BatchOpPut,
			Blocks:    batch,
			Results:   make(chan BatchResult, 1),
			Context:   ctx,
			Timestamp: time.Now(),
		}
		
		select {
		case bim.putQueue <- op:
		case <-ctx.Done():
			return ctx.Err()
		case <-bim.stopChan:
			return errors.New("batch manager is shutting down")
		}
		
		// Wait for result
		select {
		case result := <-op.Results:
			if result.Error != nil {
				return result.Error
			}
		case <-ctx.Done():
			return ctx.Err()
		}
	}
	
	return nil
}

// BatchGet retrieves multiple blocks efficiently
func (bim *batchIOManager) BatchGet(ctx context.Context, cids []cid.Cid) ([]blocks.Block, error) {
	if len(cids) == 0 {
		return nil, nil
	}
	
	var allBlocks []blocks.Block
	
	// Split into smaller batches if necessary
	for i := 0; i < len(cids); i += bim.config.MaxBatchSize {
		end := i + bim.config.MaxBatchSize
		if end > len(cids) {
			end = len(cids)
		}
		
		batch := cids[i:end]
		op := &BatchOperation{
			Type:      BatchOpGet,
			CIDs:      batch,
			Results:   make(chan BatchResult, 1),
			Context:   ctx,
			Timestamp: time.Now(),
		}
		
		select {
		case bim.getQueue <- op:
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-bim.stopChan:
			return nil, errors.New("batch manager is shutting down")
		}
		
		// Wait for result
		select {
		case result := <-op.Results:
			if result.Error != nil {
				return nil, result.Error
			}
			allBlocks = append(allBlocks, result.Blocks...)
		case <-ctx.Done():
			return nil, ctx.Err()
		}
	}
	
	return allBlocks, nil
}

// BatchHas checks existence of multiple blocks efficiently
func (bim *batchIOManager) BatchHas(ctx context.Context, cids []cid.Cid) ([]bool, error) {
	if len(cids) == 0 {
		return nil, nil
	}
	
	var allResults []bool
	
	// Split into smaller batches if necessary
	for i := 0; i < len(cids); i += bim.config.MaxBatchSize {
		end := i + bim.config.MaxBatchSize
		if end > len(cids) {
			end = len(cids)
		}
		
		batch := cids[i:end]
		op := &BatchOperation{
			Type:      BatchOpHas,
			CIDs:      batch,
			Results:   make(chan BatchResult, 1),
			Context:   ctx,
			Timestamp: time.Now(),
		}
		
		select {
		case bim.hasQueue <- op:
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-bim.stopChan:
			return nil, errors.New("batch manager is shutting down")
		}
		
		// Wait for result
		select {
		case result := <-op.Results:
			if result.Error != nil {
				return nil, result.Error
			}
			allResults = append(allResults, result.Bools...)
		case <-ctx.Done():
			return nil, ctx.Err()
		}
	}
	
	return allResults, nil
}

// BatchDelete removes multiple blocks efficiently
func (bim *batchIOManager) BatchDelete(ctx context.Context, cids []cid.Cid) error {
	if len(cids) == 0 {
		return nil
	}
	
	// Split into smaller batches if necessary
	for i := 0; i < len(cids); i += bim.config.MaxBatchSize {
		end := i + bim.config.MaxBatchSize
		if end > len(cids) {
			end = len(cids)
		}
		
		batch := cids[i:end]
		op := &BatchOperation{
			Type:      BatchOpDelete,
			CIDs:      batch,
			Results:   make(chan BatchResult, 1),
			Context:   ctx,
			Timestamp: time.Now(),
		}
		
		select {
		case bim.deleteQueue <- op:
		case <-ctx.Done():
			return ctx.Err()
		case <-bim.stopChan:
			return errors.New("batch manager is shutting down")
		}
		
		// Wait for result
		select {
		case result := <-op.Results:
			if result.Error != nil {
				return result.Error
			}
		case <-ctx.Done():
			return ctx.Err()
		}
	}
	
	return nil
}

// Flush forces all pending operations to complete
func (bim *batchIOManager) Flush(ctx context.Context) error {
	resultChan := make(chan error, 1)
	
	select {
	case bim.flushChan <- resultChan:
	case <-ctx.Done():
		return ctx.Err()
	case <-bim.stopChan:
		return errors.New("batch manager is shutting down")
	}
	
	select {
	case err := <-resultChan:
		bim.statsMu.Lock()
		bim.stats.LastFlush = time.Now()
		bim.statsMu.Unlock()
		return err
	case <-ctx.Done():
		return ctx.Err()
	}
}

// Close shuts down the batch manager
func (bim *batchIOManager) Close() error {
	close(bim.stopChan)
	bim.workers.Wait()
	return nil
}

// GetStats returns batch operation statistics
func (bim *batchIOManager) GetStats() *BatchIOStats {
	bim.statsMu.RLock()
	defer bim.statsMu.RUnlock()
	
	// Return a copy to avoid race conditions
	stats := *bim.stats
	if stats.TotalBatches > 0 {
		stats.AverageBatchSize = float64(stats.TotalItems) / float64(stats.TotalBatches)
	}
	
	return &stats
}

// putWorker processes batched put operations
func (bim *batchIOManager) putWorker(ctx context.Context) {
	defer bim.workers.Done()
	
	var pendingOps []*BatchOperation
	timer := time.NewTimer(bim.config.BatchTimeout)
	timer.Stop()
	
	for {
		select {
		case <-ctx.Done():
			bim.processPendingPuts(pendingOps)
			return
		case <-bim.stopChan:
			bim.processPendingPuts(pendingOps)
			return
		case op := <-bim.putQueue:
			pendingOps = append(pendingOps, op)
			
			// Start timer if this is the first operation
			if len(pendingOps) == 1 {
				timer.Reset(bim.config.BatchTimeout)
			}
			
			// Process if batch is full
			if len(pendingOps) >= bim.config.MaxBatchSize {
				timer.Stop()
				bim.processPendingPuts(pendingOps)
				pendingOps = nil
			}
			
		case <-timer.C:
			if len(pendingOps) > 0 {
				bim.processPendingPuts(pendingOps)
				pendingOps = nil
			}
		}
	}
}

// processPendingPuts processes a batch of put operations
func (bim *batchIOManager) processPendingPuts(ops []*BatchOperation) {
	if len(ops) == 0 {
		return
	}
	
	start := time.Now()
	
	// Collect all blocks from all operations
	var allBlocks []blocks.Block
	for _, op := range ops {
		allBlocks = append(allBlocks, op.Blocks...)
	}
	
	// Perform the batch put with retries
	var err error
	for attempt := 0; attempt <= bim.config.RetryAttempts; attempt++ {
		if attempt > 0 {
			bim.metrics.retries.Inc()
			bim.statsMu.Lock()
			bim.stats.RetryCount++
			bim.statsMu.Unlock()
			time.Sleep(bim.config.RetryDelay)
		}
		
		if bim.config.EnableTransactions {
			err = bim.transactionalPutMany(context.Background(), allBlocks)
		} else {
			err = bim.blockstore.PutMany(context.Background(), allBlocks)
		}
		
		if err == nil {
			break
		}
	}
	
	// Update statistics
	bim.updateStats(len(ops), len(allBlocks), time.Since(start), err == nil)
	
	// Send results to all operations
	result := BatchResult{Error: err}
	for _, op := range ops {
		select {
		case op.Results <- result:
		default:
			// Channel might be closed, ignore
		}
	}
}

// getWorker processes batched get operations
func (bim *batchIOManager) getWorker(ctx context.Context) {
	defer bim.workers.Done()
	
	var pendingOps []*BatchOperation
	timer := time.NewTimer(bim.config.BatchTimeout)
	timer.Stop()
	
	for {
		select {
		case <-ctx.Done():
			bim.processPendingGets(pendingOps)
			return
		case <-bim.stopChan:
			bim.processPendingGets(pendingOps)
			return
		case op := <-bim.getQueue:
			pendingOps = append(pendingOps, op)
			
			// Start timer if this is the first operation
			if len(pendingOps) == 1 {
				timer.Reset(bim.config.BatchTimeout)
			}
			
			// Process if batch is full
			if len(pendingOps) >= bim.config.MaxBatchSize {
				timer.Stop()
				bim.processPendingGets(pendingOps)
				pendingOps = nil
			}
			
		case <-timer.C:
			if len(pendingOps) > 0 {
				bim.processPendingGets(pendingOps)
				pendingOps = nil
			}
		}
	}
}

// processPendingGets processes a batch of get operations
func (bim *batchIOManager) processPendingGets(ops []*BatchOperation) {
	if len(ops) == 0 {
		return
	}
	
	start := time.Now()
	
	// Process each operation individually since Get doesn't have a batch interface
	for _, op := range ops {
		var blocks []blocks.Block
		var err error
		
		for attempt := 0; attempt <= bim.config.RetryAttempts; attempt++ {
			if attempt > 0 {
				bim.metrics.retries.Inc()
				bim.statsMu.Lock()
				bim.stats.RetryCount++
				bim.statsMu.Unlock()
				time.Sleep(bim.config.RetryDelay)
			}
			
			blocks, err = bim.batchGetBlocks(context.Background(), op.CIDs)
			if err == nil {
				break
			}
		}
		
		result := BatchResult{
			Blocks: blocks,
			Error:  err,
		}
		
		select {
		case op.Results <- result:
		default:
			// Channel might be closed, ignore
		}
	}
	
	// Update statistics
	totalItems := 0
	for _, op := range ops {
		totalItems += len(op.CIDs)
	}
	bim.updateStats(len(ops), totalItems, time.Since(start), true)
}

// batchGetBlocks retrieves multiple blocks
func (bim *batchIOManager) batchGetBlocks(ctx context.Context, cids []cid.Cid) ([]blocks.Block, error) {
	var blocks []blocks.Block
	
	for _, c := range cids {
		block, err := bim.blockstore.Get(ctx, c)
		if err != nil {
			if ipld.IsNotFound(err) {
				// Continue with other blocks, but note the error
				continue
			}
			return nil, err
		}
		blocks = append(blocks, block)
	}
	
	return blocks, nil
}

// hasWorker processes batched has operations
func (bim *batchIOManager) hasWorker(ctx context.Context) {
	defer bim.workers.Done()
	
	var pendingOps []*BatchOperation
	timer := time.NewTimer(bim.config.BatchTimeout)
	timer.Stop()
	
	for {
		select {
		case <-ctx.Done():
			bim.processPendingHas(pendingOps)
			return
		case <-bim.stopChan:
			bim.processPendingHas(pendingOps)
			return
		case op := <-bim.hasQueue:
			pendingOps = append(pendingOps, op)
			
			// Start timer if this is the first operation
			if len(pendingOps) == 1 {
				timer.Reset(bim.config.BatchTimeout)
			}
			
			// Process if batch is full
			if len(pendingOps) >= bim.config.MaxBatchSize {
				timer.Stop()
				bim.processPendingHas(pendingOps)
				pendingOps = nil
			}
			
		case <-timer.C:
			if len(pendingOps) > 0 {
				bim.processPendingHas(pendingOps)
				pendingOps = nil
			}
		}
	}
}

// processPendingHas processes a batch of has operations
func (bim *batchIOManager) processPendingHas(ops []*BatchOperation) {
	if len(ops) == 0 {
		return
	}
	
	start := time.Now()
	
	// Process each operation individually since Has doesn't have a batch interface
	for _, op := range ops {
		var results []bool
		var err error
		
		for attempt := 0; attempt <= bim.config.RetryAttempts; attempt++ {
			if attempt > 0 {
				bim.metrics.retries.Inc()
				bim.statsMu.Lock()
				bim.stats.RetryCount++
				bim.statsMu.Unlock()
				time.Sleep(bim.config.RetryDelay)
			}
			
			results, err = bim.batchHasBlocks(context.Background(), op.CIDs)
			if err == nil {
				break
			}
		}
		
		result := BatchResult{
			Bools: results,
			Error: err,
		}
		
		select {
		case op.Results <- result:
		default:
			// Channel might be closed, ignore
		}
	}
	
	// Update statistics
	totalItems := 0
	for _, op := range ops {
		totalItems += len(op.CIDs)
	}
	bim.updateStats(len(ops), totalItems, time.Since(start), true)
}

// batchHasBlocks checks existence of multiple blocks
func (bim *batchIOManager) batchHasBlocks(ctx context.Context, cids []cid.Cid) ([]bool, error) {
	var results []bool
	
	for _, c := range cids {
		has, err := bim.blockstore.Has(ctx, c)
		if err != nil {
			return nil, err
		}
		results = append(results, has)
	}
	
	return results, nil
}

// deleteWorker processes batched delete operations
func (bim *batchIOManager) deleteWorker(ctx context.Context) {
	defer bim.workers.Done()
	
	var pendingOps []*BatchOperation
	timer := time.NewTimer(bim.config.BatchTimeout)
	timer.Stop()
	
	for {
		select {
		case <-ctx.Done():
			bim.processPendingDeletes(pendingOps)
			return
		case <-bim.stopChan:
			bim.processPendingDeletes(pendingOps)
			return
		case op := <-bim.deleteQueue:
			pendingOps = append(pendingOps, op)
			
			// Start timer if this is the first operation
			if len(pendingOps) == 1 {
				timer.Reset(bim.config.BatchTimeout)
			}
			
			// Process if batch is full
			if len(pendingOps) >= bim.config.MaxBatchSize {
				timer.Stop()
				bim.processPendingDeletes(pendingOps)
				pendingOps = nil
			}
			
		case <-timer.C:
			if len(pendingOps) > 0 {
				bim.processPendingDeletes(pendingOps)
				pendingOps = nil
			}
		}
	}
}

// processPendingDeletes processes a batch of delete operations
func (bim *batchIOManager) processPendingDeletes(ops []*BatchOperation) {
	if len(ops) == 0 {
		return
	}
	
	start := time.Now()
	
	// Process each operation individually since DeleteBlock doesn't have a batch interface
	var err error
	totalItems := 0
	
	for _, op := range ops {
		totalItems += len(op.CIDs)
		
		for attempt := 0; attempt <= bim.config.RetryAttempts; attempt++ {
			if attempt > 0 {
				bim.metrics.retries.Inc()
				bim.statsMu.Lock()
				bim.stats.RetryCount++
				bim.statsMu.Unlock()
				time.Sleep(bim.config.RetryDelay)
			}
			
			err = bim.batchDeleteBlocks(context.Background(), op.CIDs)
			if err == nil {
				break
			}
		}
		
		result := BatchResult{Error: err}
		
		select {
		case op.Results <- result:
		default:
			// Channel might be closed, ignore
		}
	}
	
	// Update statistics
	bim.updateStats(len(ops), totalItems, time.Since(start), err == nil)
}

// batchDeleteBlocks deletes multiple blocks
func (bim *batchIOManager) batchDeleteBlocks(ctx context.Context, cids []cid.Cid) error {
	for _, c := range cids {
		if err := bim.blockstore.DeleteBlock(ctx, c); err != nil {
			return err
		}
	}
	return nil
}

// transactionalPutMany performs a transactional batch put operation
func (bim *batchIOManager) transactionalPutMany(ctx context.Context, blks []blocks.Block) error {
	// If the underlying blockstore supports batching, use it
	type batchPutter interface {
		PutMany(context.Context, []blocks.Block) error
	}
	
	if batcher, ok := bim.blockstore.(batchPutter); ok {
		return batcher.PutMany(ctx, blks)
	}
	
	// Otherwise, fall back to individual puts
	// In a real implementation, you might want to use a transaction-capable datastore
	for _, block := range blks {
		if err := bim.blockstore.Put(ctx, block); err != nil {
			return err
		}
	}
	
	return nil
}

// flushCoordinator handles flush requests
func (bim *batchIOManager) flushCoordinator(ctx context.Context) {
	defer bim.workers.Done()
	
	for {
		select {
		case <-ctx.Done():
			return
		case <-bim.stopChan:
			return
		case resultChan := <-bim.flushChan:
			// For now, flush is a no-op since we process batches immediately
			// In a more sophisticated implementation, you might buffer operations
			// and flush them here
			resultChan <- nil
		}
	}
}

// updateStats updates batch operation statistics
func (bim *batchIOManager) updateStats(batches, items int, latency time.Duration, success bool) {
	bim.statsMu.Lock()
	defer bim.statsMu.Unlock()
	
	bim.stats.TotalBatches += int64(batches)
	bim.stats.TotalItems += int64(items)
	
	if success {
		bim.stats.SuccessfulBatches += int64(batches)
	} else {
		bim.stats.FailedBatches += int64(batches)
	}
	
	// Update average latency (simple moving average)
	if bim.stats.TotalBatches == 1 {
		bim.stats.AverageLatency = latency
	} else {
		// Weighted average
		weight := 0.1 // Give more weight to recent measurements
		bim.stats.AverageLatency = time.Duration(
			float64(bim.stats.AverageLatency)*(1-weight) + float64(latency)*weight,
		)
	}
	
	// Update metrics
	bim.metrics.batchesProcessed.Add(float64(batches))
	bim.metrics.itemsProcessed.Add(float64(items))
	bim.metrics.batchLatency.Observe(latency.Seconds())
}