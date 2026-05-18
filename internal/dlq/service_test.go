package dlq

import (
	"testing"
	"time"
)

func TestEnqueueAndDequeue(t *testing.T) {
	t.Parallel()

	svc := NewService()
	now := time.Date(2026, 3, 3, 12, 0, 0, 0, time.UTC)
	svc.now = func() time.Time { return now }

	entry, err := svc.Enqueue(QueueInteractiveTurns, []byte(`{"turn_id":"t1"}`), "timeout")
	if err != nil {
		t.Fatalf("enqueue failed: %v", err)
	}
	if entry.ID == "" {
		t.Fatal("expected entry ID")
	}
	if entry.QueueName != QueueInteractiveTurns {
		t.Fatalf("expected queue=%s got=%s", QueueInteractiveTurns, entry.QueueName)
	}
	if entry.Status != StatusPending {
		t.Fatalf("expected status=pending got=%s", entry.Status)
	}
	if entry.Attempts != 1 {
		t.Fatalf("expected attempts=1 got=%d", entry.Attempts)
	}
	if entry.MaxAttempts != 3 {
		t.Fatalf("expected max_attempts=3 got=%d", entry.MaxAttempts)
	}
	if !entry.FirstFailedAt.Equal(now) {
		t.Fatalf("expected first_failed_at=%v got=%v", now, entry.FirstFailedAt)
	}

	if svc.CountByQueue(QueueInteractiveTurns) != 1 {
		t.Fatalf("expected count=1 got=%d", svc.CountByQueue(QueueInteractiveTurns))
	}

	dequeued, err := svc.Dequeue(QueueInteractiveTurns)
	if err != nil {
		t.Fatalf("dequeue failed: %v", err)
	}
	if dequeued.ID != entry.ID {
		t.Fatalf("expected dequeued id=%s got=%s", entry.ID, dequeued.ID)
	}
	if svc.CountByQueue(QueueInteractiveTurns) != 0 {
		t.Fatalf("expected count=0 after dequeue, got=%d", svc.CountByQueue(QueueInteractiveTurns))
	}
}

func TestEnqueueUnknownQueueFails(t *testing.T) {
	t.Parallel()

	svc := NewService()
	_, err := svc.Enqueue("unknown_queue", []byte("data"), "error")
	if err == nil {
		t.Fatal("expected error for unknown queue")
	}
}

func TestEnqueueEmptyPayloadFails(t *testing.T) {
	t.Parallel()

	svc := NewService()
	_, err := svc.Enqueue(QueueWorkflowTasks, nil, "error")
	if err == nil {
		t.Fatal("expected error for empty payload")
	}
}

func TestRetryExhaustion(t *testing.T) {
	t.Parallel()

	svc := NewService()
	tick := time.Date(2026, 3, 3, 12, 0, 0, 0, time.UTC)
	svc.now = func() time.Time { return tick }

	entry, err := svc.Enqueue(QueueLedgerWrites, []byte(`{"txn":"x"}`), "db timeout")
	if err != nil {
		t.Fatalf("enqueue failed: %v", err)
	}

	// Retry once: attempts=2, status=retrying
	tick = tick.Add(time.Second)
	retried, err := svc.Retry(entry.ID)
	if err != nil {
		t.Fatalf("retry 1 failed: %v", err)
	}
	if retried.Attempts != 2 {
		t.Fatalf("expected attempts=2 got=%d", retried.Attempts)
	}
	if retried.Status != StatusRetrying {
		t.Fatalf("expected status=retrying got=%s", retried.Status)
	}

	// Retry again: attempts=3 (== max), status=exhausted
	tick = tick.Add(time.Second)
	exhausted, err := svc.Retry(entry.ID)
	if err != nil {
		t.Fatalf("retry 2 failed: %v", err)
	}
	if exhausted.Attempts != 3 {
		t.Fatalf("expected attempts=3 got=%d", exhausted.Attempts)
	}
	if exhausted.Status != StatusExhausted {
		t.Fatalf("expected status=exhausted got=%s", exhausted.Status)
	}

	// Further retry should fail.
	_, err = svc.Retry(entry.ID)
	if err == nil {
		t.Fatal("expected error retrying exhausted entry")
	}
}

func TestResolve(t *testing.T) {
	t.Parallel()

	svc := NewService()
	entry, _ := svc.Enqueue(QueueTrajectoryWrites, []byte(`{"t":"1"}`), "fail")

	resolved, err := svc.Resolve(entry.ID)
	if err != nil {
		t.Fatalf("resolve failed: %v", err)
	}
	if resolved.Status != StatusResolved {
		t.Fatalf("expected status=resolved got=%s", resolved.Status)
	}

	// Cannot retry a resolved entry.
	_, err = svc.Retry(entry.ID)
	if err == nil {
		t.Fatal("expected error retrying resolved entry")
	}
}

func TestPeek(t *testing.T) {
	t.Parallel()

	svc := NewService()
	entry, _ := svc.Enqueue(QueueWorkflowTasks, []byte(`{"task":"t1"}`), "err")

	peeked, err := svc.Peek(QueueWorkflowTasks)
	if err != nil {
		t.Fatalf("peek failed: %v", err)
	}
	if peeked.ID != entry.ID {
		t.Fatalf("peek returned wrong entry: got=%s want=%s", peeked.ID, entry.ID)
	}

	// Entry should still be in the queue.
	if svc.CountByQueue(QueueWorkflowTasks) != 1 {
		t.Fatalf("peek should not remove entry")
	}
}

func TestPurgeResolved(t *testing.T) {
	t.Parallel()

	svc := NewService()
	e1, _ := svc.Enqueue(QueueRateLimitLedgerWrites, []byte(`{"a":"1"}`), "err1")
	svc.Enqueue(QueueRateLimitLedgerWrites, []byte(`{"a":"2"}`), "err2")

	svc.Resolve(e1.ID)

	purged := svc.PurgeResolved(QueueRateLimitLedgerWrites)
	if purged != 1 {
		t.Fatalf("expected purged=1 got=%d", purged)
	}
	if svc.CountByQueue(QueueRateLimitLedgerWrites) != 1 {
		t.Fatalf("expected remaining=1 got=%d", svc.CountByQueue(QueueRateLimitLedgerWrites))
	}
}

func TestListByQueue(t *testing.T) {
	t.Parallel()

	svc := NewService()
	svc.Enqueue(QueueInteractiveTurns, []byte(`{"a":"1"}`), "err1")
	svc.Enqueue(QueueInteractiveTurns, []byte(`{"a":"2"}`), "err2")
	svc.Enqueue(QueueWorkflowTasks, []byte(`{"b":"1"}`), "err3")

	list := svc.ListByQueue(QueueInteractiveTurns)
	if len(list) != 2 {
		t.Fatalf("expected 2 entries, got=%d", len(list))
	}
	for _, e := range list {
		if e.QueueName != QueueInteractiveTurns {
			t.Fatalf("wrong queue in listing: %s", e.QueueName)
		}
	}
}
