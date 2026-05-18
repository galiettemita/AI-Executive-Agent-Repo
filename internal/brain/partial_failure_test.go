package brain

import (
	"testing"
)

func TestBuildPartialResult(t *testing.T) {
	results := []TaskResult{
		{TaskIndex: 0, TaskName: "task_a", Success: true},
		{TaskIndex: 1, TaskName: "task_b", Success: false, Error: "timeout"},
		{TaskIndex: 2, TaskName: "task_c", Success: true},
	}

	pr := BuildPartialResult(results)
	if pr.TotalTasks != 3 {
		t.Fatalf("expected 3 total, got %d", pr.TotalTasks)
	}
	if pr.Succeeded != 2 {
		t.Fatalf("expected 2 succeeded, got %d", pr.Succeeded)
	}
	if pr.Failed != 1 {
		t.Fatalf("expected 1 failed, got %d", pr.Failed)
	}
	if len(pr.FailedTasks) != 1 {
		t.Fatalf("expected 1 failed task, got %d", len(pr.FailedTasks))
	}
}

func TestEvaluate_AllSucceeded(t *testing.T) {
	policy := NewPartialFailurePolicy()
	pr := BuildPartialResult([]TaskResult{
		{TaskIndex: 0, Success: true},
		{TaskIndex: 1, Success: true},
	})

	decision, err := policy.Evaluate(pr)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if decision != DecisionDeliverPartial {
		t.Fatalf("expected deliver_partial, got %s", decision)
	}
}

func TestEvaluate_AllFailed(t *testing.T) {
	policy := NewPartialFailurePolicy()
	pr := BuildPartialResult([]TaskResult{
		{TaskIndex: 0, Success: false},
		{TaskIndex: 1, Success: false},
	})

	decision, err := policy.Evaluate(pr)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if decision != DecisionFailAll {
		t.Fatalf("expected fail_all, got %s", decision)
	}
}

func TestEvaluate_DeliverPartial(t *testing.T) {
	policy := NewPartialFailurePolicy() // minSuccessRatio=0.5
	pr := BuildPartialResult([]TaskResult{
		{TaskIndex: 0, Success: true},
		{TaskIndex: 1, Success: true},
		{TaskIndex: 2, Success: false},
		{TaskIndex: 3, Success: true},
	})

	decision, _ := policy.Evaluate(pr)
	if decision != DecisionDeliverPartial {
		t.Fatalf("expected deliver_partial for 75%% success, got %s", decision)
	}
}

func TestEvaluate_RetryFailed(t *testing.T) {
	policy := NewPartialFailurePolicyWithConfig(0.8, 2) // high threshold, allow 2 retries
	pr := BuildPartialResult([]TaskResult{
		{TaskIndex: 0, Success: true},
		{TaskIndex: 1, Success: false},
		{TaskIndex: 2, Success: false},
		{TaskIndex: 3, Success: true},
		{TaskIndex: 4, Success: true},
	})

	decision, _ := policy.Evaluate(pr)
	if decision != DecisionRetryFailed {
		t.Fatalf("expected retry_failed, got %s", decision)
	}
}

func TestEvaluate_AskUser(t *testing.T) {
	policy := NewPartialFailurePolicyWithConfig(0.8, 1) // high threshold, only 1 retry allowed
	pr := BuildPartialResult([]TaskResult{
		{TaskIndex: 0, Success: true},
		{TaskIndex: 1, Success: false},
		{TaskIndex: 2, Success: false},
		{TaskIndex: 3, Success: false},
		{TaskIndex: 4, Success: true},
	})

	decision, _ := policy.Evaluate(pr)
	if decision != DecisionAskUser {
		t.Fatalf("expected ask_user, got %s", decision)
	}
}

func TestEvaluate_EmptyTasks(t *testing.T) {
	policy := NewPartialFailurePolicy()
	pr := BuildPartialResult([]TaskResult{})

	_, err := policy.Evaluate(pr)
	if err == nil {
		t.Fatal("expected error for empty tasks")
	}
}
