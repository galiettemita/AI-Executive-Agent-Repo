package workflows

import (
	"context"
	"regexp"
	"slices"
	"testing"
	"time"
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
	match, err := regexp.MatchString(`^idem_[0-9a-v]{16,}$`, first.IdempotencyKey)
	if err != nil {
		t.Fatalf("compile idempotency regex: %v", err)
	}
	if !match {
		t.Fatalf("idempotency key format mismatch: %s", first.IdempotencyKey)
	}
	if !first.CreatedAt.Equal(second.CreatedAt) {
		t.Fatalf("expected same execution timestamp for idempotent replay")
	}
}

func TestReasoningConstraintsForTier(t *testing.T) {
	t.Parallel()

	cases := []struct {
		tier         string
		resolvedTier string
		loopLimit    int
		candidates   int
	}{
		{tier: "T1", resolvedTier: "T1", loopLimit: 2, candidates: 1},
		{tier: "T2", resolvedTier: "T2", loopLimit: 5, candidates: 2},
		{tier: "T3", resolvedTier: "T3", loopLimit: 10, candidates: 3},
		{tier: "unknown", resolvedTier: "T1", loopLimit: 2, candidates: 1},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.tier, func(t *testing.T) {
			got := ReasoningConstraintsForTier(tc.tier)
			if got.PlannerRetryLimit != 1 || got.CriticRetryLimit != 1 {
				t.Fatalf("retry limit mismatch: %+v", got)
			}
			if got.ResolvedTier != tc.resolvedTier {
				t.Fatalf("resolved tier mismatch: got=%s want=%s", got.ResolvedTier, tc.resolvedTier)
			}
			if got.ExecutorLoopLimit != tc.loopLimit {
				t.Fatalf("loop limit mismatch: got=%d want=%d", got.ExecutorLoopLimit, tc.loopLimit)
			}
			if got.MaxPlanCandidates != tc.candidates {
				t.Fatalf("candidate limit mismatch: got=%d want=%d", got.MaxPlanCandidates, tc.candidates)
			}
		})
	}
}

func TestDeterministicOrderingHelpers(t *testing.T) {
	t.Parallel()

	toolOrder := DeterministicToolSchemaOrder([]string{"zeta.run", "alpha.call", "beta.exec"})
	if !slices.Equal(toolOrder, []string{"alpha.call", "beta.exec", "zeta.run"}) {
		t.Fatalf("unexpected tool order: %v", toolOrder)
	}

	contextOrder := DeterministicContextItemOrder([]string{"memory:z", "history:a", "tool:b"})
	if !slices.Equal(contextOrder, []string{"history:a", "memory:z", "tool:b"}) {
		t.Fatalf("unexpected context order: %v", contextOrder)
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
	if got := svc.DailyCaptureV1(context.Background(), "cron"); got != "skipped" {
		t.Fatalf("expected idempotent daily capture skip, got: %s", got)
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
	crossRepoInstance, ok := svc.WorkflowInstance("cross_repo_analysis_v1")
	if !ok {
		t.Fatal("expected cross_repo_analysis_v1 workflow mirror instance")
	}
	if crossRepoInstance.Status != "completed" {
		t.Fatalf("unexpected cross_repo_analysis_v1 workflow status: %s", crossRepoInstance.Status)
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

func TestV91WorkflowTriggerSpecs(t *testing.T) {
	t.Parallel()

	specs := V91WorkflowTriggerSpecs()
	expected := map[string]string{
		"daily_capture_v1":           "end_of_last_session_each_day_or_cron_if_no_session_by_configured_time",
		"daily_log_capture_v1":       "after_each_interactive_turn_v1_completion",
		"goal_review_v1":             "mission_control_refresh_or_cron_default_weekly",
		"trust_eval_v1":              "daily_03_00_utc_or_after_operator_override_event",
		"learning_consolidation_v1":  "weekly_or_pending_feedback_gt_20",
		"capability_exploration_v1":  "monthly_or_capability_gap_events_gte_3_within_7d",
		"cross_repo_analysis_v1":     "after_codebase_map_ingestion_v1_or_operator_request",
		"mission_control_refresh_v1": "cron_per_mission_control_config_refresh_cadence_minutes",
	}

	if len(specs) != len(expected) {
		t.Fatalf("unexpected trigger-spec count: got=%d want=%d", len(specs), len(expected))
	}

	for workflowID, trigger := range expected {
		spec, ok := specs[workflowID]
		if !ok {
			t.Fatalf("missing trigger spec for workflow %s", workflowID)
		}
		if spec.WorkflowID != workflowID {
			t.Fatalf("workflow id mismatch for %s: got=%s", workflowID, spec.WorkflowID)
		}
		if spec.Trigger != trigger {
			t.Fatalf("trigger mismatch for %s: got=%s want=%s", workflowID, spec.Trigger, trigger)
		}
	}
}

func TestWorkflowStateMirrorInteractiveTurn(t *testing.T) {
	t.Parallel()

	svc := NewService()
	_ = svc.InteractiveTurnV1(context.Background(), "plan and execute")

	instance, ok := svc.WorkflowInstance("interactive_turn_v1")
	if !ok {
		t.Fatal("expected workflow instance mirror for interactive_turn_v1")
	}
	if instance.Status != "completed" {
		t.Fatalf("unexpected workflow status: %s", instance.Status)
	}

	steps := svc.WorkflowSteps("interactive_turn_v1")
	if len(steps) == 0 {
		t.Fatal("expected workflow step mirror entries")
	}
	if steps[0].StepKey != "planner" {
		t.Fatalf("unexpected first mirrored step: %s", steps[0].StepKey)
	}
}

func TestWorkflowStateMirrorProvisioningFailure(t *testing.T) {
	t.Parallel()

	svc := NewService()
	result := svc.ProvisioningV9(context.Background(), "DeployServer")
	if result.Status != "failed" {
		t.Fatalf("expected failed status, got %s", result.Status)
	}
	instance, ok := svc.WorkflowInstance("provisioning_v9")
	if !ok {
		t.Fatal("expected provisioning workflow mirror")
	}
	if instance.Status != "failed" {
		t.Fatalf("unexpected provisioning mirror status: %s", instance.Status)
	}

	steps := svc.WorkflowSteps("provisioning_v9")
	if len(steps) == 0 {
		t.Fatal("expected mirrored provisioning steps")
	}
	foundCompensation := false
	for _, step := range steps {
		if len(step.StepKey) > len("compensate_") && step.StepKey[:len("compensate_")] == "compensate_" {
			foundCompensation = true
			break
		}
	}
	if !foundCompensation {
		t.Fatalf("expected compensation mirror steps, got %+v", steps)
	}
}

func TestExecuteTwoPhaseToolIdempotency(t *testing.T) {
	t.Parallel()

	svc := NewService()
	now := time.Date(2026, 2, 28, 0, 0, 0, 0, time.UTC)
	first, err := svc.ExecuteTwoPhaseTool("ws1", "gmail.send", "send invoice", now)
	if err != nil {
		t.Fatalf("first two-phase execution: %v", err)
	}
	if first.Replayed {
		t.Fatal("first execution should not be replay")
	}

	second, err := svc.ExecuteTwoPhaseTool("ws1", "gmail.send", "send invoice", now.Add(10*time.Minute))
	if err != nil {
		t.Fatalf("second two-phase execution: %v", err)
	}
	if !second.Replayed {
		t.Fatal("expected second execution to replay within idempotency ttl")
	}
	if second.Simulate.IdempotencyKey != first.Simulate.IdempotencyKey {
		t.Fatalf("simulate idempotency key mismatch: %s vs %s", second.Simulate.IdempotencyKey, first.Simulate.IdempotencyKey)
	}
	if second.Commit.IdempotencyKey != first.Commit.IdempotencyKey {
		t.Fatalf("commit idempotency key mismatch: %s vs %s", second.Commit.IdempotencyKey, first.Commit.IdempotencyKey)
	}
}

func TestExecuteTwoPhaseToolIdempotencyTTLExpiry(t *testing.T) {
	t.Parallel()

	svc := NewService()
	svc.idempotencyTTL = time.Second
	base := time.Date(2026, 2, 28, 0, 0, 0, 0, time.UTC)

	first, err := svc.ExecuteTwoPhaseTool("ws2", "calendar.create_event", "book meeting", base)
	if err != nil {
		t.Fatalf("first two-phase execution: %v", err)
	}
	second, err := svc.ExecuteTwoPhaseTool("ws2", "calendar.create_event", "book meeting", base.Add(2*time.Second))
	if err != nil {
		t.Fatalf("second two-phase execution after ttl expiry: %v", err)
	}
	if second.Replayed {
		t.Fatal("expected execution after ttl expiry to be a fresh run")
	}
	if !second.Commit.CreatedAt.After(first.Commit.CreatedAt) {
		t.Fatalf("expected refreshed commit timestamp after ttl expiry: first=%s second=%s", first.Commit.CreatedAt, second.Commit.CreatedAt)
	}
}
