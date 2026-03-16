package worker

import (
	"context"
	"fmt"
	"log/slog"
	"sync"
	"sync/atomic"
	"time"
)

// ProcessorFunc is called for each dispatched VoiceSession.
// Errors are logged; the session is not retried automatically.
type ProcessorFunc func(ctx context.Context, session VoiceSession) error

// WorkerPool holds concurrency primitives.
type WorkerPool struct {
	maxWorkers    int
	activeWorkers atomic.Int64
	sem           chan struct{}
}

// WorkerDispatch submits voice sessions to a bounded pool of goroutines.
type WorkerDispatch struct {
	mu        sync.Mutex
	pool      *WorkerPool
	processor ProcessorFunc
	logger    *slog.Logger
}

// NewWorkerDispatch creates a WorkerDispatch.
// maxWorkers <= 0 defaults to 4.
// processor must not be nil.
func NewWorkerDispatch(maxWorkers int, processor ProcessorFunc, logger *slog.Logger) (*WorkerDispatch, error) {
	if processor == nil {
		return nil, fmt.Errorf("worker dispatch: processor must not be nil")
	}
	if maxWorkers <= 0 {
		maxWorkers = 4
	}
	if logger == nil {
		logger = slog.Default()
	}
	return &WorkerDispatch{
		pool: &WorkerPool{
			maxWorkers: maxWorkers,
			sem:        make(chan struct{}, maxWorkers),
		},
		processor: processor,
		logger:    logger,
	}, nil
}

// Dispatch submits a session for asynchronous processing.
// Blocks if all worker slots are busy until a slot is free or ctx is cancelled.
func (wd *WorkerDispatch) Dispatch(ctx context.Context, session VoiceSession) error {
	if session.ID == "" {
		return fmt.Errorf("worker dispatch: session ID must not be empty")
	}

	select {
	case wd.pool.sem <- struct{}{}:
	case <-ctx.Done():
		return fmt.Errorf("worker dispatch: cancelled while waiting for slot: %w", ctx.Err())
	}

	wd.pool.activeWorkers.Add(1)
	go func() {
		defer func() {
			wd.pool.activeWorkers.Add(-1)
			<-wd.pool.sem
		}()
		defer func() {
			if r := recover(); r != nil {
				wd.logger.Error("voice worker panic",
					"session_id", session.ID,
					"panic", fmt.Sprintf("%v", r),
				)
			}
		}()
		// Detached context — worker runs to completion even if caller cancels.
		workerCtx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
		defer cancel()
		if err := wd.processor(workerCtx, session); err != nil {
			wd.logger.Error("voice worker error",
				"session_id", session.ID,
				"error", err,
			)
		}
	}()
	return nil
}

// Shutdown waits for all active workers to finish or ctx to expire.
func (wd *WorkerDispatch) Shutdown(ctx context.Context) error {
	ticker := time.NewTicker(50 * time.Millisecond)
	defer ticker.Stop()
	for {
		if wd.pool.activeWorkers.Load() == 0 {
			return nil
		}
		select {
		case <-ctx.Done():
			return fmt.Errorf("worker dispatch: shutdown timed out with %d active workers",
				wd.pool.activeWorkers.Load())
		case <-ticker.C:
		}
	}
}

// ActiveWorkers returns the number of currently running workers.
func (wd *WorkerDispatch) ActiveWorkers() int {
	return int(wd.pool.activeWorkers.Load())
}

// MaxWorkers returns the configured pool ceiling.
func (wd *WorkerDispatch) MaxWorkers() int {
	return wd.pool.maxWorkers
}
