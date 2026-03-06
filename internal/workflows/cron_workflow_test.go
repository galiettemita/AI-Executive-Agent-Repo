package workflows

import (
	"slices"
	"testing"
)

func TestCronWorkflowHappyPath(t *testing.T) {
	t.Parallel()
	svc := NewService()
	result := svc.CronExecutionWorkflowV1(CronWorkflowInput{
		ExecutionID:    "exec-001",
		JobID:          "job-001",
		UserID:         "user-001",
		ActionType:     "skill",
		MaxRetries:     3,
		WebhookEnabled: true,
	})

	if result.WorkflowID != "cron-exec-001" {
		t.Fatalf("unexpected workflow id: %s", result.WorkflowID)
	}
	if result.TerminalState != CronStateCompleted {
		t.Fatalf("expected COMPLETED, got %s", result.TerminalState)
	}
	if !slices.Contains(result.States, CronStateWebhook) {
		t.Fatalf("expected WEBHOOK state: %v", result.States)
	}
}

func TestCronWorkflowExpiredJob(t *testing.T) {
	t.Parallel()
	svc := NewService()
	result := svc.CronExecutionWorkflowV1(CronWorkflowInput{
		ExecutionID: "exec-002",
		JobExpired:  true,
	})

	if result.TerminalState != CronStateSkipped {
		t.Fatalf("expected SKIPPED for expired job, got %s", result.TerminalState)
	}
}

func TestCronWorkflowRetryExhausted(t *testing.T) {
	t.Parallel()
	svc := NewService()
	result := svc.CronExecutionWorkflowV1(CronWorkflowInput{
		ExecutionID:     "exec-003",
		ExecuteError:    true,
		RetryCount:      3,
		MaxRetries:      3,
		NotifyOnFailure: true,
	})

	if result.TerminalState != CronStateFailed {
		t.Fatalf("expected FAILED after retry exhaustion, got %s", result.TerminalState)
	}
	if !slices.Contains(result.States, CronStateNotifying) {
		t.Fatalf("expected notification on failure: %v", result.States)
	}
}

func TestCronWorkflowTimeout(t *testing.T) {
	t.Parallel()
	svc := NewService()
	result := svc.CronExecutionWorkflowV1(CronWorkflowInput{
		ExecutionID:     "exec-004",
		TimeoutExceeded: true,
	})

	if result.TerminalState != CronStateTimedOut {
		t.Fatalf("expected TIMED_OUT, got %s", result.TerminalState)
	}
}

func TestCronWorkflowRetryWithCapacity(t *testing.T) {
	t.Parallel()
	svc := NewService()
	result := svc.CronExecutionWorkflowV1(CronWorkflowInput{
		ExecutionID:  "exec-005",
		ExecuteError: true,
		RetryCount:   1,
		MaxRetries:   3,
	})

	if result.TerminalState != CronStateCompleted {
		t.Fatalf("expected COMPLETED with retry capacity, got %s", result.TerminalState)
	}
	if !slices.Contains(result.Fallbacks, "retry") {
		t.Fatalf("expected retry fallback: %v", result.Fallbacks)
	}
	if result.RetryCount != 2 {
		t.Fatalf("expected retry count 2, got %d", result.RetryCount)
	}
}

func TestCronWorkflowLoadError(t *testing.T) {
	t.Parallel()
	svc := NewService()
	result := svc.CronExecutionWorkflowV1(CronWorkflowInput{
		ExecutionID: "exec-006",
		LoadError:   true,
	})

	if result.TerminalState != CronStateFailed {
		t.Fatalf("expected FAILED on load error, got %s", result.TerminalState)
	}
}

func TestCronWorkflowNoWebhookSkipsState(t *testing.T) {
	t.Parallel()
	svc := NewService()
	result := svc.CronExecutionWorkflowV1(CronWorkflowInput{
		ExecutionID:    "exec-007",
		WebhookEnabled: false,
	})

	if slices.Contains(result.States, CronStateWebhook) {
		t.Fatalf("should not contain WEBHOOK state when disabled: %v", result.States)
	}
}

func TestCronWorkflowWebhookErrorFallback(t *testing.T) {
	t.Parallel()
	svc := NewService()
	result := svc.CronExecutionWorkflowV1(CronWorkflowInput{
		ExecutionID:    "exec-008",
		WebhookEnabled: true,
		WebhookError:   true,
	})

	if result.TerminalState != CronStateCompleted {
		t.Fatalf("expected COMPLETED with webhook fallback, got %s", result.TerminalState)
	}
	if !slices.Contains(result.Fallbacks, "webhook_retry") {
		t.Fatalf("expected webhook_retry fallback: %v", result.Fallbacks)
	}
}

func TestCronWorkflowRetryExhaustedNoNotification(t *testing.T) {
	t.Parallel()
	svc := NewService()
	result := svc.CronExecutionWorkflowV1(CronWorkflowInput{
		ExecutionID:     "exec-009",
		ExecuteError:    true,
		RetryCount:      3,
		MaxRetries:      3,
		NotifyOnFailure: false,
	})

	if result.TerminalState != CronStateFailed {
		t.Fatalf("expected FAILED, got %s", result.TerminalState)
	}
	if slices.Contains(result.States, CronStateNotifying) {
		t.Fatalf("should not contain NOTIFYING when NotifyOnFailure is false: %v", result.States)
	}
}

func TestCronWorkflowIDTrimming(t *testing.T) {
	t.Parallel()
	id := CronWorkflowID("  exec-padded  ")
	if id != "cron-exec-padded" {
		t.Fatalf("expected trimmed ID, got %s", id)
	}
}
