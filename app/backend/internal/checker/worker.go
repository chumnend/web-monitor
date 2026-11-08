package checker

import (
	"context"
	"sync"
	"time"
)

// Task represents a unit of work executed by a worker.
type Task func()

// WorkerPool manages a bounded pool of goroutines executing submitted tasks.
type WorkerPool struct {
	concurrency int
	taskQueue   chan Task
	wg          sync.WaitGroup
	mu          sync.Mutex
	isClosed    bool
}

// NewWorkerPool creates a new WorkerPool with bounded concurrency and queue buffer.
func NewWorkerPool(concurrency int, queueSize int) *WorkerPool {
	if concurrency <= 0 {
		concurrency = 10
	}
	if queueSize <= 0 {
		queueSize = concurrency * 10
	}

	wp := &WorkerPool{
		concurrency: concurrency,
		taskQueue:   make(chan Task, queueSize),
	}

	wp.start()
	return wp
}

func (wp *WorkerPool) start() {
	for i := 0; i < wp.concurrency; i++ {
		wp.wg.Add(1)
		go func() {
			defer wp.wg.Done()
			for task := range wp.taskQueue {
				task()
			}
		}()
	}
}

// Submit enqueues a task to be processed by a worker. Returns false if pool is closed.
func (wp *WorkerPool) Submit(task Task) bool {
	wp.mu.Lock()
	if wp.isClosed {
		wp.mu.Unlock()
		return false
	}
	wp.mu.Unlock()

	wp.taskQueue <- task
	return true
}

// TrySubmit attempts to submit a task without blocking. Returns false if queue is full or pool closed.
func (wp *WorkerPool) TrySubmit(task Task) bool {
	wp.mu.Lock()
	if wp.isClosed {
		wp.mu.Unlock()
		return false
	}
	wp.mu.Unlock()

	select {
	case wp.taskQueue <- task:
		return true
	default:
		return false
	}
}

// Stop initiates graceful shutdown, waiting for active and queued tasks to complete.
func (wp *WorkerPool) Stop(timeout ...time.Duration) bool {
	wp.mu.Lock()
	if wp.isClosed {
		wp.mu.Unlock()
		return true
	}
	wp.isClosed = true
	close(wp.taskQueue)
	wp.mu.Unlock()

	waitTimeout := 10 * time.Second
	if len(timeout) > 0 && timeout[0] > 0 {
		waitTimeout = timeout[0]
	}

	done := make(chan struct{})
	go func() {
		wp.wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		return true
	case <-time.After(waitTimeout):
		return false
	}
}

// WaitForShutdown waits for workers to drain with context cancellation.
func (wp *WorkerPool) WaitForShutdown(ctx context.Context) error {
	wp.mu.Lock()
	if !wp.isClosed {
		wp.isClosed = true
		close(wp.taskQueue)
	}
	wp.mu.Unlock()

	done := make(chan struct{})
	go func() {
		wp.wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}
