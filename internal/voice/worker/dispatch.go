package worker

import (
	"fmt"
	"sync"
	"sync/atomic"
)

// WorkerPool manages a pool of concurrent voice processing workers.
type WorkerPool struct {
	maxWorkers    int
	activeWorkers atomic.Int64
	sem           chan struct{}
}

// WorkerDispatch dispatches voice sessions to workers with concurrency control.
type WorkerDispatch struct {
	mu   sync.Mutex
	pool *WorkerPool
}

// NewWorkerDispatch creates a new WorkerDispatch with the given max concurrency.
func NewWorkerDispatch(maxWorkers int) *WorkerDispatch {
	if maxWorkers <= 0 {
		maxWorkers = 4
	}
	return &WorkerDispatch{
		pool: &WorkerPool{
			maxWorkers: maxWorkers,
			sem:        make(chan struct{}, maxWorkers),
		},
	}
}

// Dispatch processes a voice session using a worker from the pool.
// It blocks if all workers are busy.
func (wd *WorkerDispatch) Dispatch(session VoiceSession) error {
	if session.ID == "" {
		return fmt.Errorf("session ID must not be empty")
	}

	// Acquire semaphore slot.
	wd.pool.sem <- struct{}{}
	wd.pool.activeWorkers.Add(1)

	go func() {
		defer func() {
			wd.pool.activeWorkers.Add(-1)
			<-wd.pool.sem
		}()

		// Process the voice session (extract tasks, generate summary, etc.).
		// This is a placeholder for actual processing logic.
		_ = session
	}()

	return nil
}

// ActiveWorkers returns the number of currently active workers.
func (wd *WorkerDispatch) ActiveWorkers() int {
	return int(wd.pool.activeWorkers.Load())
}

// MaxWorkers returns the maximum number of concurrent workers.
func (wd *WorkerDispatch) MaxWorkers() int {
	return wd.pool.maxWorkers
}
