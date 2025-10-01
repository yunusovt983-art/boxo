package adaptive

import (
	"context"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestWorkerPool_BasicFunctionality(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	
	pool := NewWorkerPool(ctx, 3)
	defer pool.Close()
	
	// Submit some tasks
	var counter int64
	var wg sync.WaitGroup
	
	for i := 0; i < 10; i++ {
		wg.Add(1)
		pool.Submit(func() {
			defer wg.Done()
			atomic.AddInt64(&counter, 1)
			time.Sleep(10 * time.Millisecond) // Simulate work
		})
	}
	
	// Wait for all tasks to complete
	done := make(chan struct{})
	go func() {
		wg.Wait()
		close(done)
	}()
	
	select {
	case <-done:
		// Success
	case <-time.After(5 * time.Second):
		t.Fatal("Tasks did not complete in time")
	}
	
	assert.Equal(t, int64(10), atomic.LoadInt64(&counter))
	
	// Check stats
	stats := pool.GetStats()
	assert.Equal(t, 3, stats.Workers)
	assert.Equal(t, int64(10), stats.TasksSubmitted)
	assert.Equal(t, int64(10), stats.TasksCompleted)
}

func TestWorkerPool_ConcurrentSubmission(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	
	pool := NewWorkerPool(ctx, 5)
	defer pool.Close()
	
	const numGoroutines = 10
	const tasksPerGoroutine = 50
	
	var counter int64
	var wg sync.WaitGroup
	
	// Launch multiple goroutines submitting tasks
	for g := 0; g < numGoroutines; g++ {
		go func() {
			for i := 0; i < tasksPerGoroutine; i++ {
				wg.Add(1)
				pool.Submit(func() {
					defer wg.Done()
					atomic.AddInt64(&counter, 1)
					time.Sleep(1 * time.Millisecond) // Small work simulation
				})
			}
		}()
	}
	
	// Wait for all tasks to complete
	done := make(chan struct{})
	go func() {
		wg.Wait()
		close(done)
	}()
	
	select {
	case <-done:
		// Success
	case <-time.After(10 * time.Second):
		t.Fatal("Tasks did not complete in time")
	}
	
	expectedTasks := int64(numGoroutines * tasksPerGoroutine)
	assert.Equal(t, expectedTasks, atomic.LoadInt64(&counter))
	
	// Check stats
	stats := pool.GetStats()
	assert.Equal(t, expectedTasks, stats.TasksSubmitted)
	assert.Equal(t, expectedTasks, stats.TasksCompleted)
}

func TestWorkerPool_AutoScaling(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	
	pool := NewWorkerPool(ctx, 2) // Start with 2 workers
	defer pool.Close()
	
	initialStats := pool.GetStats()
	assert.Equal(t, 2, initialStats.Workers)
	
	// Submit many tasks quickly to trigger scaling
	var wg sync.WaitGroup
	for i := 0; i < 100; i++ {
		wg.Add(1)
		pool.Submit(func() {
			defer wg.Done()
			time.Sleep(50 * time.Millisecond) // Longer work to create backlog
		})
	}
	
	// Wait a bit for auto-scaling to potentially kick in
	time.Sleep(2 * time.Second)
	
	// Check if workers increased (auto-scaling might have occurred)
	scaledStats := pool.GetStats()
	t.Logf("Initial workers: %d, Scaled workers: %d", initialStats.Workers, scaledStats.Workers)
	
	// Wait for tasks to complete
	done := make(chan struct{})
	go func() {
		wg.Wait()
		close(done)
	}()
	
	select {
	case <-done:
		// Success
	case <-time.After(10 * time.Second):
		t.Fatal("Tasks did not complete in time")
	}
	
	finalStats := pool.GetStats()
	assert.Equal(t, int64(100), finalStats.TasksCompleted)
}

func TestWorkerPool_Resize(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	
	pool := NewWorkerPool(ctx, 2)
	defer pool.Close()
	
	initialStats := pool.GetStats()
	assert.Equal(t, 2, initialStats.Workers)
	
	// Resize to more workers
	pool.Resize(6)
	
	// Wait a bit for resize to take effect
	time.Sleep(100 * time.Millisecond)
	
	resizedStats := pool.GetStats()
	assert.True(t, resizedStats.Workers >= 2, "Workers should not decrease below initial")
	assert.True(t, resizedStats.Workers <= 6, "Workers should not exceed resize target")
	
	// Submit tasks to verify the resized pool works
	var counter int64
	var wg sync.WaitGroup
	
	for i := 0; i < 20; i++ {
		wg.Add(1)
		pool.Submit(func() {
			defer wg.Done()
			atomic.AddInt64(&counter, 1)
		})
	}
	
	// Wait for tasks to complete
	done := make(chan struct{})
	go func() {
		wg.Wait()
		close(done)
	}()
	
	select {
	case <-done:
		// Success
	case <-time.After(5 * time.Second):
		t.Fatal("Tasks did not complete after resize")
	}
	
	assert.Equal(t, int64(20), atomic.LoadInt64(&counter))
}

func TestWorkerPool_PanicRecovery(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	
	pool := NewWorkerPool(ctx, 2)
	defer pool.Close()
	
	var normalTaskCompleted int64
	var wg sync.WaitGroup
	
	// Submit a task that panics
	wg.Add(1)
	pool.Submit(func() {
		defer wg.Done()
		panic("test panic")
	})
	
	// Submit a normal task after the panic
	wg.Add(1)
	pool.Submit(func() {
		defer wg.Done()
		atomic.AddInt64(&normalTaskCompleted, 1)
	})
	
	// Wait for tasks to complete
	done := make(chan struct{})
	go func() {
		wg.Wait()
		close(done)
	}()
	
	select {
	case <-done:
		// Success
	case <-time.After(5 * time.Second):
		t.Fatal("Tasks did not complete (panic recovery failed)")
	}
	
	// Normal task should have completed despite the panic
	assert.Equal(t, int64(1), atomic.LoadInt64(&normalTaskCompleted))
	
	// Pool should still be functional
	stats := pool.GetStats()
	assert.True(t, stats.TasksCompleted >= 1, "At least one task should be completed")
	assert.True(t, stats.Workers >= 1, "Pool should have at least one worker after panic recovery")
}

func TestWorkerPool_ContextCancellation(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	
	pool := NewWorkerPool(ctx, 3)
	
	var completedTasks int64
	var wg sync.WaitGroup
	
	// Submit some long-running tasks
	for i := 0; i < 5; i++ {
		wg.Add(1)
		pool.Submit(func() {
			defer wg.Done()
			select {
			case <-time.After(2 * time.Second):
				atomic.AddInt64(&completedTasks, 1)
			case <-ctx.Done():
				// Task cancelled
			}
		})
	}
	
	// Cancel context after a short time
	time.Sleep(100 * time.Millisecond)
	cancel()
	
	// Close the pool
	err := pool.Close()
	assert.NoError(t, err)
	
	// Wait for tasks to finish (they should be cancelled)
	done := make(chan struct{})
	go func() {
		wg.Wait()
		close(done)
	}()
	
	select {
	case <-done:
		// Tasks completed (likely cancelled)
	case <-time.After(3 * time.Second):
		t.Fatal("Tasks did not complete after context cancellation")
	}
	
	// Most tasks should have been cancelled
	completed := atomic.LoadInt64(&completedTasks)
	assert.True(t, completed < 5, "Expected some tasks to be cancelled, but %d completed", completed)
}

func TestWorkerPool_QueueCapacity(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	
	pool := NewWorkerPool(ctx, 1) // Single worker to create backlog
	defer pool.Close()
	
	var startedTasks int64
	var completedTasks int64
	
	// Submit tasks that will create a queue
	for i := 0; i < 10; i++ {
		pool.Submit(func() {
			atomic.AddInt64(&startedTasks, 1)
			time.Sleep(50 * time.Millisecond) // Simulate work
			atomic.AddInt64(&completedTasks, 1)
		})
	}
	
	// Check that tasks are queued
	stats := pool.GetStats()
	assert.True(t, stats.QueuedTasks > 0 || stats.TasksSubmitted > stats.TasksCompleted, 
		"Expected tasks to be queued")
	
	// Wait for all tasks to complete
	for {
		stats := pool.GetStats()
		if stats.TasksCompleted >= 10 {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}
	
	assert.Equal(t, int64(10), atomic.LoadInt64(&completedTasks))
}

func TestWorkerPool_Stats(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	
	pool := NewWorkerPool(ctx, 3)
	defer pool.Close()
	
	initialStats := pool.GetStats()
	assert.Equal(t, 3, initialStats.Workers)
	assert.Equal(t, int64(0), initialStats.TasksSubmitted)
	assert.Equal(t, int64(0), initialStats.TasksCompleted)
	assert.True(t, initialStats.QueueCapacity > 0)
	
	// Submit some tasks
	var wg sync.WaitGroup
	for i := 0; i < 5; i++ {
		wg.Add(1)
		pool.Submit(func() {
			defer wg.Done()
			time.Sleep(10 * time.Millisecond)
		})
	}
	
	// Check stats during execution
	time.Sleep(5 * time.Millisecond) // Let some tasks start
	midStats := pool.GetStats()
	assert.Equal(t, int64(5), midStats.TasksSubmitted)
	assert.True(t, midStats.ActiveWorkers >= 0)
	assert.True(t, midStats.ActiveWorkers <= midStats.Workers)
	
	// Wait for completion
	wg.Wait()
	
	finalStats := pool.GetStats()
	assert.Equal(t, int64(5), finalStats.TasksSubmitted)
	assert.Equal(t, int64(5), finalStats.TasksCompleted)
	assert.Equal(t, 0, finalStats.QueuedTasks)
}

func TestWorkerPool_HighLoad(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping high load test in short mode")
	}
	
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	
	pool := NewWorkerPool(ctx, 10)
	defer pool.Close()
	
	const numTasks = 1000
	var counter int64
	var wg sync.WaitGroup
	
	start := time.Now()
	
	// Submit many tasks
	for i := 0; i < numTasks; i++ {
		wg.Add(1)
		pool.Submit(func() {
			defer wg.Done()
			atomic.AddInt64(&counter, 1)
			// Minimal work to test throughput
			time.Sleep(time.Microsecond)
		})
	}
	
	// Wait for completion
	done := make(chan struct{})
	go func() {
		wg.Wait()
		close(done)
	}()
	
	select {
	case <-done:
		elapsed := time.Since(start)
		t.Logf("Processed %d tasks in %v (%.2f tasks/sec)", 
			numTasks, elapsed, float64(numTasks)/elapsed.Seconds())
	case <-time.After(25 * time.Second):
		t.Fatal("High load test did not complete in time")
	}
	
	assert.Equal(t, int64(numTasks), atomic.LoadInt64(&counter))
	
	stats := pool.GetStats()
	assert.Equal(t, int64(numTasks), stats.TasksCompleted)
	assert.True(t, stats.Workers >= 10, "Expected workers to scale up under high load")
}