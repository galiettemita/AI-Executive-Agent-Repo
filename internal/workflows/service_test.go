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

func TestTrustEvalFormulaAndPromotionEligibility(t *testing.T) {
	t.Parallel()

	svc := NewService()
	result := svc.TrustEvalV1(context.Background(), 20, 0, 0, 0)
	if result.TrustScore != 1.0 {
		t.Fatalf("unexpected trust score: %f", result.TrustScore)
	}
	if !result.PromotionEligible {
		t.Fatal("expected promotion eligibility")
	}

	negative := svc.TrustEvalV1(context.Background(), 5, 2, 1, 1)
	if negative.PromotionEligible {
		t.Fatal("did not expect promotion eligibility")
	}
}

func TestV91WorkflowStubs(t *testing.T) {
	t.Parallel()

	svc := NewService()
	if got := svc.DailyCaptureV1(context.Background(), "cron"); got != "completed" {
		t.Fatalf("unexpected daily capture status: %s", got)
	}
	if got := svc.DailyLogCaptureV1(context.Background(), "turn-1"); got != "captured" {
		t.Fatalf("unexpected daily log status: %s", got)
	}
	if got := svc.GoalReviewV1(context.Background(), 1); got != "stalled_detected" {
		t.Fatalf("unexpected goal review status: %s", got)
	}
	if got := svc.LearningConsolidationV1(context.Background(), 5, true); got != "requires_confirmation" {
		t.Fatalf("unexpected learning consolidation status: %s", got)
	}
	if got := svc.CapabilityExplorationV1(context.Background(), 3); got != "recommendations_created" {
		t.Fatalf("unexpected capability exploration status: %s", got)
	}
	if got := svc.CrossRepoAnalysisV1(context.Background(), 2); got != "patterns_detected" {
		t.Fatalf("unexpected cross repo status: %s", got)
	}
	if got := svc.MissionControlRefreshV1(context.Background(), 4); got != "refreshed" {
		t.Fatalf("unexpected mission control status: %s", got)
	}
}

func TestV92WorkflowStubs(t *testing.T) {
	t.Parallel()

	svc := NewService()
	if got := svc.RagIngestV1(context.Background(), 4); got != "ingested" {
		t.Fatalf("unexpected rag ingest status: %s", got)
	}
	if got := svc.RagEvalV1(context.Background(), 0.81, 0.76); got != "passed" {
		t.Fatalf("unexpected rag eval status: %s", got)
	}
	if got := svc.ToolHealthEvalV1(context.Background(), 5); got != "quarantined" {
		t.Fatalf("unexpected tool health status: %s", got)
	}
	if got := svc.MemoryConflictResolveV1(context.Background(), true); got != "resolved" {
		t.Fatalf("unexpected memory conflict status: %s", got)
	}
	if got := svc.ComplianceEvidenceV1(context.Background(), "soc2"); got != "collected" {
		t.Fatalf("unexpected compliance evidence status: %s", got)
	}
	if got := svc.AdminKPIReportV1(context.Background(), 5); got != "generated" {
		t.Fatalf("unexpected admin report status: %s", got)
	}
	if got := svc.AdminAlertEvalV1(context.Background(), true); got != "fired" {
		t.Fatalf("unexpected admin alert status: %s", got)
	}
	if removed := svc.CacheMaintenanceV1(context.Background(), 3); removed != 3 {
		t.Fatalf("unexpected cache maintenance result: %d", removed)
	}
}
