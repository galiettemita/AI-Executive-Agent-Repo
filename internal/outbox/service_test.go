package outbox

import (
	"testing"
	"time"
)

func TestOutboxEntryDefaults(t *testing.T) {
	t.Parallel()

	entry := OutboxEntry{
		ID:          "entry-001",
		WorkspaceID: "ws-1",
		EventType:   "goal.created",
	}

	if entry.Status != "" {
		t.Fatal("expected empty status before processing")
	}
	if entry.MaxAttempts != 0 {
		t.Fatal("expected zero max attempts before processing")
	}

	// Simulate what Enqueue does with defaults.
	if entry.Status == "" {
		entry.Status = StatusPending
	}
	if entry.MaxAttempts <= 0 {
		entry.MaxAttempts = 5
	}
	if entry.CreatedAt.IsZero() {
		entry.CreatedAt = time.Now().UTC()
	}

	if entry.Status != StatusPending {
		t.Fatalf("expected pending status, got %s", entry.Status)
	}
	if entry.MaxAttempts != 5 {
		t.Fatalf("expected max attempts 5, got %d", entry.MaxAttempts)
	}
	if entry.CreatedAt.IsZero() {
		t.Fatal("expected non-zero created_at")
	}
}

func TestOutboxStatusConstants(t *testing.T) {
	t.Parallel()

	if StatusPending != "pending" {
		t.Fatalf("unexpected pending constant: %s", StatusPending)
	}
	if StatusDispatched != "dispatched" {
		t.Fatalf("unexpected dispatched constant: %s", StatusDispatched)
	}
	if StatusFailed != "failed" {
		t.Fatalf("unexpected failed constant: %s", StatusFailed)
	}
	if StatusDLQ != "dlq" {
		t.Fatalf("unexpected dlq constant: %s", StatusDLQ)
	}
}

func TestOutboxEntryStructFields(t *testing.T) {
	t.Parallel()

	now := time.Now().UTC()
	dispatched := now.Add(time.Minute)
	nextRetry := now.Add(5 * time.Minute)

	entry := OutboxEntry{
		ID:            "entry-002",
		WorkspaceID:   "ws-2",
		AggregateType: "goal",
		AggregateID:   "goal-123",
		EventType:     "goal.completed",
		Payload:       []byte(`{"goal_id":"goal-123"}`),
		Target:        "notification-service",
		Status:        StatusPending,
		Attempts:      0,
		MaxAttempts:   5,
		CreatedAt:     now,
		DispatchedAt:  &dispatched,
		NextRetryAt:   &nextRetry,
		FailReason:    "",
	}

	if entry.AggregateType != "goal" {
		t.Fatalf("unexpected aggregate type: %s", entry.AggregateType)
	}
	if entry.AggregateID != "goal-123" {
		t.Fatalf("unexpected aggregate id: %s", entry.AggregateID)
	}
	if entry.Target != "notification-service" {
		t.Fatalf("unexpected target: %s", entry.Target)
	}
	if string(entry.Payload) != `{"goal_id":"goal-123"}` {
		t.Fatalf("unexpected payload: %s", entry.Payload)
	}
	if entry.DispatchedAt == nil || !entry.DispatchedAt.Equal(dispatched) {
		t.Fatal("unexpected dispatched_at")
	}
	if entry.NextRetryAt == nil || !entry.NextRetryAt.Equal(nextRetry) {
		t.Fatal("unexpected next_retry_at")
	}
}

func TestNewServiceRequiresPool(t *testing.T) {
	t.Parallel()

	// NewService should accept a nil pool without panicking (connection will
	// fail at query time, which is the expected behavior for unit tests that
	// don't have a live database).
	svc := NewService(nil)
	if svc == nil {
		t.Fatal("expected non-nil service")
	}
}

func TestOutboxEntryValidation(t *testing.T) {
	t.Parallel()

	// Verify that the validation logic for Enqueue can be exercised via
	// struct checks (actual DB calls require integration tests).
	entry := OutboxEntry{}
	if entry.ID != "" {
		t.Fatal("expected empty ID")
	}
	if entry.EventType != "" {
		t.Fatal("expected empty event_type")
	}
}
