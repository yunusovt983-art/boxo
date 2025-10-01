package adaptive

import (
	"context"
	"sync"
	"sync/atomic"
	"time"

	logging "github.com/ipfs/go-log/v2"
)

var workerLog = logging.Logger("bitswap/adaptive/worker")

// WorkerPool manages a pool of workers for parallel processing
type WorkerPool struct {
	workers    int32 // Current number of workers (atomic)
	maxWorkers int32 // Maximum number of workers allowed
	workChan   chan func()
	ctx        context.Context
	cancel     context.CancelFunc
	wg         sync.WaitGroup
	
	// Metrics
	tasksSubmitted  int64 // Total tasks submitted (atomic)
	tasksCompleted  int64 // Total tasks completed (atomic)
	activeWorkers   int32 // Currently active workers (atomic)
	
	// Dynamic scaling
	lastScaleTime   time.Time
	scaleInterval   time.Duration
	scaleMutex      sync.Mutex
}

// NewWorkerPool creates a new worker pool with the specified number of workers
func NewWorkerPool(ctx context.Context, workers int) *WorkerPool {
	poolCtx, cancel := context.WithCancel(ctx)
	
	pool := &WorkerPool{
		workers:       0, // Will be incremented by startWorker
		maxWorkers:    int32(workers * 4), // Allow scaling up to 4x initial size
		workChan:      make(chan func(), workers*2), // Buffer for 2x workers
		ctx:           poolCtx,
		cancel:        cancel,
		scaleInterval: 10 * time.Second, // Minimum time between scaling operations
		lastScaleTime: time.Now().Add(-11 * time.Second), // Allow immediate first scaling
	}
	
	// Start initial workers
	for i := 0; i < workers; i++ {
		atomic.AddInt32(&pool.workers, 1)
		pool.startWorker()
	}
	
	// Start monitoring goroutine for dynamic scaling (disabled by default for predictable testing)
	// pool.wg.Add(1)
	// go pool.monitor()
	
	workerLog.Infof("WorkerPool started with %d workers", workers)
	
	return pool
}

// EnableAutoScaling enables automatic scaling for the worker pool
func (p *WorkerPool) EnableAutoScaling() {
	p.scaleMutex.Lock()
	defer p.scaleMutex.Unlock()
	
	// Start monitoring if not already started
	select {
	case <-p.ctx.Done():
		return
	default:
		p.wg.Add(1)
		go p.monitor()
	}
}

// Submit submits a task to the worker pool
func (p *WorkerPool) Submit(task func()) {
	select {
	case p.workChan <- task:
		// Task queued successfully
		atomic.AddInt64(&p.tasksSubmitted, 1)
	case <-p.ctx.Done():
		// Pool is shutting down
		return
	default:
		// Channel is full, try to scale up if possible
		p.tryScaleUp()
		
		// Try to submit again with timeout
		select {
		case p.workChan <- task:
			atomic.AddInt64(&p.tasksSubmitted, 1)
		case <-time.After(100 * time.Millisecond):
			workerLog.Warnf("Task submission timeout, dropping task")
		case <-p.ctx.Done():
			return
		}
	}
}

// startWorker starts a new worker goroutine
func (p *WorkerPool) startWorker() {
	p.wg.Add(1)
	go func() {
		defer func() {
			p.wg.Done()
			atomic.AddInt32(&p.workers, -1)
		}()
		
		for {
			select {
			case <-p.ctx.Done():
				return
			case task := <-p.workChan:
				if task != nil {
					atomic.AddInt32(&p.activeWorkers, 1)
					
					// Execute task with panic recovery
					func() {
						defer func() {
							if r := recover(); r != nil {
								workerLog.Errorf("Worker panic recovered: %v", r)
							}
							atomic.AddInt32(&p.activeWorkers, -1)
							atomic.AddInt64(&p.tasksCompleted, 1)
						}()
						
						task()
					}()
				}
			}
		}
	}()
}

// tryScaleUp attempts to scale up the worker pool if conditions are met
func (p *WorkerPool) tryScaleUp() {
	p.scaleMutex.Lock()
	defer p.scaleMutex.Unlock()
	
	// Check if enough time has passed since last scaling
	if time.Since(p.lastScaleTime) < p.scaleInterval {
		return
	}
	
	currentWorkers := atomic.LoadInt32(&p.workers)
	maxWorkers := atomic.LoadInt32(&p.maxWorkers)
	
	// Check if we can scale up
	if currentWorkers >= maxWorkers {
		return
	}
	
	// Check if scaling is needed (queue is getting full and we have high active workers)
	queueLength := len(p.workChan)
	queueCapacity := cap(p.workChan)
	activeWorkers := atomic.LoadInt32(&p.activeWorkers)
	
	// Only scale up if queue is very full AND most workers are active
	if float64(queueLength)/float64(queueCapacity) > 0.9 && float64(activeWorkers)/float64(currentWorkers) > 0.8 {
		// Scale up by 1 worker at a time for more controlled scaling
		newWorkers := 1
		
		// Don't exceed max workers
		if currentWorkers+int32(newWorkers) > maxWorkers {
			newWorkers = int(maxWorkers - currentWorkers)
		}
		
		if newWorkers > 0 {
			for i := 0; i < newWorkers; i++ {
				p.startWorker()
			}
			
			p.lastScaleTime = time.Now()
			workerLog.Infof("Scaled up worker pool by %d workers (total: %d)", newWorkers, atomic.LoadInt32(&p.workers))
		}
	}
}

// tryScaleDown attempts to scale down the worker pool if conditions are met
func (p *WorkerPool) tryScaleDown() {
	p.scaleMutex.Lock()
	defer p.scaleMutex.Unlock()
	
	// Check if enough time has passed since last scaling
	if time.Since(p.lastScaleTime) < p.scaleInterval {
		return
	}
	
	currentWorkers := atomic.LoadInt32(&p.workers)
	activeWorkers := atomic.LoadInt32(&p.activeWorkers)
	
	// Don't scale down if we have minimum workers or if most workers are active
	minWorkers := int32(1)
	if currentWorkers <= minWorkers || float64(activeWorkers)/float64(currentWorkers) > 0.7 {
		return
	}
	
	// Check if queue is mostly empty
	queueLength := len(p.workChan)
	if queueLength < int(currentWorkers)/4 {
		// We can scale down - workers will naturally exit when context is cancelled
		// For now, we just log this condition. Actual scaling down would require
		// more sophisticated worker lifecycle management.
		workerLog.Debugf("Worker pool could scale down: %d workers, %d active, %d queued", 
			currentWorkers, activeWorkers, queueLength)
	}
}

// monitor runs periodic monitoring and scaling decisions
func (p *WorkerPool) monitor() {
	defer p.wg.Done()
	
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()
	
	for {
		select {
		case <-p.ctx.Done():
			return
		case <-ticker.C:
			p.tryScaleDown()
			
			// Log metrics periodically
			if workerLog.LevelEnabled(logging.LevelDebug) {
				submitted := atomic.LoadInt64(&p.tasksSubmitted)
				completed := atomic.LoadInt64(&p.tasksCompleted)
				workers := atomic.LoadInt32(&p.workers)
				active := atomic.LoadInt32(&p.activeWorkers)
				queued := len(p.workChan)
				
				workerLog.Debugf("WorkerPool stats: workers=%d, active=%d, queued=%d, submitted=%d, completed=%d", 
					workers, active, queued, submitted, completed)
			}
		}
	}
}

// Resize changes the target number of workers in the pool
func (p *WorkerPool) Resize(newSize int) {
	p.scaleMutex.Lock()
	defer p.scaleMutex.Unlock()
	
	currentWorkers := int(atomic.LoadInt32(&p.workers))
	
	if newSize > currentWorkers {
		// Scale up
		for i := 0; i < newSize-currentWorkers; i++ {
			p.startWorker()
		}
		workerLog.Infof("Resized worker pool from %d to %d workers", currentWorkers, newSize)
	} else if newSize < currentWorkers {
		// For scaling down, we update the max workers and let natural attrition handle it
		// This is safer than forcefully terminating workers
		atomic.StoreInt32(&p.maxWorkers, int32(newSize*4))
		workerLog.Infof("Set worker pool target size to %d (current: %d)", newSize, currentWorkers)
	}
	
	p.lastScaleTime = time.Now()
}

// GetStats returns current worker pool statistics
func (p *WorkerPool) GetStats() WorkerPoolStats {
	return WorkerPoolStats{
		Workers:        int(atomic.LoadInt32(&p.workers)),
		ActiveWorkers:  int(atomic.LoadInt32(&p.activeWorkers)),
		QueuedTasks:    len(p.workChan),
		QueueCapacity:  cap(p.workChan),
		TasksSubmitted: atomic.LoadInt64(&p.tasksSubmitted),
		TasksCompleted: atomic.LoadInt64(&p.tasksCompleted),
		MaxWorkers:     int(atomic.LoadInt32(&p.maxWorkers)),
	}
}

// WorkerPoolStats contains statistics about the worker pool
type WorkerPoolStats struct {
	Workers        int   // Current number of workers
	ActiveWorkers  int   // Currently active workers
	QueuedTasks    int   // Number of tasks in queue
	QueueCapacity  int   // Queue capacity
	TasksSubmitted int64 // Total tasks submitted
	TasksCompleted int64 // Total tasks completed
	MaxWorkers     int   // Maximum allowed workers
}

// Close shuts down the worker pool
func (p *WorkerPool) Close() error {
	workerLog.Info("Shutting down WorkerPool")
	
	p.cancel()
	p.wg.Wait()
	
	// Drain remaining tasks
	close(p.workChan)
	for task := range p.workChan {
		if task != nil {
			// Execute remaining tasks to avoid dropping them
			func() {
				defer func() {
					if r := recover(); r != nil {
						workerLog.Errorf("Task panic during shutdown: %v", r)
					}
				}()
				task()
			}()
		}
	}
	
	stats := p.GetStats()
	workerLog.Infof("WorkerPool shutdown complete. Final stats: submitted=%d, completed=%d", 
		stats.TasksSubmitted, stats.TasksCompleted)
	
	return nil
}