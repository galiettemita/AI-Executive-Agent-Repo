package compliance

import (
	"context"
	"errors"
	"fmt"
	"time"
)

type sleepFunc func(ctx context.Context, duration time.Duration) error

type ExecutionLogPIIScrubScheduler struct {
	store     ExecutionLogPIIScrubStore
	batchSize int
	logf      func(format string, args ...any)
	now       func() time.Time
	sleep     sleepFunc
}

func NewExecutionLogPIIScrubScheduler(store ExecutionLogPIIScrubStore, batchSize int, logf func(format string, args ...any)) *ExecutionLogPIIScrubScheduler {
	if batchSize <= 0 {
		batchSize = DefaultExecutionLogPIIScrubBatchSize
	}
	if logf == nil {
		logf = func(string, ...any) {}
	}
	return &ExecutionLogPIIScrubScheduler{
		store:     store,
		batchSize: batchSize,
		logf:      logf,
		now:       func() time.Time { return time.Now().UTC() },
		sleep:     sleepWithContext,
	}
}

func (s *ExecutionLogPIIScrubScheduler) Run(ctx context.Context) error {
	if s == nil || s.store == nil {
		return fmt.Errorf("execution log scrub scheduler requires store")
	}
	for {
		select {
		case <-ctx.Done():
			return nil
		default:
		}

		now := s.now().UTC()
		nextRunAt := NextExecutionLogPIIScrubRun(now)
		wait := nextRunAt.Sub(now)
		if wait > 0 {
			s.logf("execution_log_scrubber waiting_until=%s wait_ms=%d", nextRunAt.Format(time.RFC3339), wait.Milliseconds())
			if err := s.sleep(ctx, wait); err != nil {
				if errors.Is(err, context.Canceled) {
					return nil
				}
				return err
			}
		}

		runNow := s.now().UTC()
		result, err := RunExecutionLogPIIScrub(ctx, s.store, runNow, s.batchSize)
		if err != nil {
			s.logf("execution_log_scrubber error=%q", err.Error())
			continue
		}
		s.logf(
			"execution_log_scrubber completed evaluated=%d scrubbed=%d skipped=%d next_run_at=%s",
			result.Evaluated,
			result.Scrubbed,
			result.Skipped,
			result.NextRunAt.Format(time.RFC3339),
		)
	}
}

func sleepWithContext(ctx context.Context, duration time.Duration) error {
	if duration <= 0 {
		return nil
	}
	timer := time.NewTimer(duration)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}
