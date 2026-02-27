package workflows

import (
	"context"
	"testing"
)

func TestInteractiveTurnV1EndToEnd(t *testing.T) {
	t.Parallel()

	svc := NewService()
	result := svc.InteractiveTurnV1(context.Background(), "hello world")
	if result.FinalState != "TERMINAL" {
		t.Fatalf("unexpected final state: %s", result.FinalState)
	}
	if len(result.Steps) < 4 {
		t.Fatalf("unexpected step count: %d", len(result.Steps))
	}
}

func TestProvisioningCompensationReverseOrder(t *testing.T) {
	t.Parallel()

	svc := NewService()
	result := svc.ProvisioningV9(context.Background(), "DeployServer")
	if result.Status != "failed" {
		t.Fatalf("expected failure status, got %s", result.Status)
	}
	if len(result.CompensatedSteps) == 0 {
		t.Fatal("expected compensation steps")
	}
	if result.CompensatedSteps[0] != "DeployServer" {
		t.Fatalf("expected first compensation to be failed step, got %s", result.CompensatedSteps[0])
	}
}

func TestOnboardingCompletesAllStages(t *testing.T) {
	t.Parallel()

	svc := NewService()
	answers := map[string]string{
		"operator_profile_intake_v1":     "a",
		"behavior_policy_calibration_v1": "a",
		"codebase_map_ingestion_v1":      "a",
		"system_map_ingestion_v1":        "a",
	}
	result := svc.OnboardingV1(context.Background(), answers)
	if result.Status != "completed" {
		t.Fatalf("unexpected onboarding status: %s", result.Status)
	}
	if len(result.CompletedStages) != 4 {
		t.Fatalf("unexpected completed stage count: %d", len(result.CompletedStages))
	}
}

func TestIdempotencyDoubleSubmitReturnsSameResult(t *testing.T) {
	t.Parallel()

	svc := NewService()
	first, err := svc.ExecuteToolWithIdempotency("ws1", "gmail.send", "send invoice")
	if err != nil {
		t.Fatalf("first execute: %v", err)
	}
	second, err := svc.ExecuteToolWithIdempotency("ws1", "gmail.send", "send invoice")
	if err != nil {
		t.Fatalf("second execute: %v", err)
	}
	if first.IdempotencyKey != second.IdempotencyKey {
		t.Fatalf("idempotency mismatch: %s vs %s", first.IdempotencyKey, second.IdempotencyKey)
	}
	if !first.CreatedAt.Equal(second.CreatedAt) {
		t.Fatalf("expected same execution timestamp for idempotent replay")
	}
}
