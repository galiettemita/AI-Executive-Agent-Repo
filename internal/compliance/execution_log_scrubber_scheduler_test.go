package compliance

import (
	"context"
	"fmt"
	"strings"
	"testing"
	"time"
)

func TestExecutionLogPIIScrubSchedulerRun(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, time.March, 3, 2, 15, 0, 0, time.UTC)
	store := &fakeExecutionLogPIIScrubStore{
		records: []ExecutionLogRecord{
			{
				ID:            "log-1",
				CreatedAt:     now.Add(-40 * 24 * time.Hour),
				InputPayload:  `{"email":"ops@brevio.app"}`,
				OutputPayload: `{"status":"ok"}`,
			},
		},
	}

	scheduler := NewExecutionLogPIIScrubScheduler(store, 100, nil)
	scheduler.now = func() time.Time { return now }

	var waited time.Duration
	ctx, cancel := context.WithCancel(context.Background())
	scheduler.sleep = func(_ context.Context, d time.Duration) error {
		waited = d
		return nil
	}
	var logs []string
	scheduler.logf = func(format string, args ...any) {
		logs = append(logs, fmt.Sprintf(format, args...))
		if strings.Contains(format, "completed") {
			cancel()
		}
	}

	if err := scheduler.Run(ctx); err != nil {
		t.Fatalf("run scheduler: %v", err)
	}

	if waited != 45*time.Minute {
		t.Fatalf("unexpected wait duration: got=%s want=%s", waited, 45*time.Minute)
	}
	if store.receivedLimit != 100 {
		t.Fatalf("unexpected scheduler batch limit: %d", store.receivedLimit)
	}
	if len(store.scrubbedIDs) != 1 || store.scrubbedIDs[0] != "log-1" {
		t.Fatalf("unexpected scrubbed ids: %v", store.scrubbedIDs)
	}
	if len(logs) == 0 {
		t.Fatalf("expected scheduler logs")
	}
}

func TestExecutionLogPIIScrubSchedulerRequiresStore(t *testing.T) {
	t.Parallel()

	scheduler := NewExecutionLogPIIScrubScheduler(nil, 0, nil)
	if err := scheduler.Run(context.Background()); err == nil {
		t.Fatalf("expected missing-store error")
	}
}
