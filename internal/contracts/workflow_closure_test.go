package contracts

import (
	"context"
	"path/filepath"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/brevio/brevio/internal/workflows"
)

func TestWorkflowIdentifierClosure(t *testing.T) {
	t.Parallel()

	root := repositoryRoot(t)
	rows := readCSVRows(t, filepath.Join(root, "spec", "traceability", "workflow_state_map.csv"))
	if len(rows) < 2 {
		t.Fatal("workflow_state_map.csv must contain header + rows")
	}

	expectedWorkflowIDs := []string{
		"interactive_turn_v1",
		"provisioning_v9",
		"onboarding_v1",
		"drift_watchdog_v1",
		"outbox_hold_worker",
		"memory_consolidation",
		"daily_capture_v1",
		"daily_log_capture_v1",
		"goal_review_v1",
		"trust_eval_v1",
		"learning_consolidation_v1",
		"capability_exploration_v1",
		"cross_repo_analysis_v1",
		"mission_control_refresh_v1",
		"rag_ingest_v1",
		"rag_eval_v1",
		"tool_health_eval_v1",
		"memory_conflict_resolve_v1",
		"compliance_evidence_v1",
		"admin_kpi_report_v1",
		"admin_alert_eval_v1",
		"cache_maintenance_v1",
	}

	gotSet := map[string]struct{}{}
	for i, row := range rows {
		if i == 0 {
			continue
		}
		if len(row) < 2 {
			t.Fatalf("invalid workflow_state_map row %d", i+1)
		}
		workflowID := strings.TrimSpace(row[0])
		if workflowID == "" {
			t.Fatalf("empty workflow_id at row %d", i+1)
		}
		if _, exists := gotSet[workflowID]; exists {
			t.Fatalf("duplicate workflow_id in map: %s", workflowID)
		}
		gotSet[workflowID] = struct{}{}
	}

	expectedSet := map[string]struct{}{}
	for _, id := range expectedWorkflowIDs {
		expectedSet[id] = struct{}{}
	}

	missing := diffKeys(expectedSet, gotSet)
	extra := diffKeys(gotSet, expectedSet)
	if len(missing) != 0 || len(extra) != 0 {
		t.Fatalf("workflow identifier closure mismatch: missing=%v extra=%v", missing, extra)
	}
}

func TestWorkflowRuntimeExerciseClosure(t *testing.T) {
	t.Parallel()

	svc := workflows.NewService()
	ctx := context.Background()

	if result := svc.InteractiveTurnV1(ctx, "runtime"); result.FinalState != "TERMINAL" {
		t.Fatalf("interactive_turn_v1 unexpected final state: %s", result.FinalState)
	}
	if result := svc.ProvisioningV9(ctx, ""); result.Status != "active" {
		t.Fatalf("provisioning_v9 unexpected status: %s", result.Status)
	}
	if result := svc.OnboardingV1(ctx, map[string]string{
		"operator_profile_intake_v1":     "done",
		"behavior_policy_calibration_v1": "done",
		"codebase_map_ingestion_v1":      "done",
		"system_map_ingestion_v1":        "done",
	}); result.Status != "completed" {
		t.Fatalf("onboarding_v1 unexpected status: %s", result.Status)
	}
	if got := svc.DriftWatchdogV1(ctx, true); got != "quarantined" {
		t.Fatalf("drift_watchdog_v1 unexpected status: %s", got)
	}
	if got := svc.OutboxHoldWorker(ctx, time.Now().UTC().Add(-time.Minute)); got != "sent" {
		t.Fatalf("outbox_hold_worker unexpected status: %s", got)
	}
	if got := svc.MemoryConsolidation(ctx, []string{"a", "a", "b"}); len(got) != 2 {
		t.Fatalf("memory_consolidation unexpected merge output: %v", got)
	}
	if got := svc.DailyCaptureV1(ctx, "cron"); got != "completed" {
		t.Fatalf("daily_capture_v1 unexpected status: %s", got)
	}
	if got := svc.DailyLogCaptureV1(ctx, "turn_1"); got != "captured" {
		t.Fatalf("daily_log_capture_v1 unexpected status: %s", got)
	}
	if got := svc.GoalReviewV1(ctx, 1); got != "stalled_detected" {
		t.Fatalf("goal_review_v1 unexpected status: %s", got)
	}
	if got := svc.TrustEvalV1(ctx, 20, 0, 0, 0); !got.PromotionEligible {
		t.Fatalf("trust_eval_v1 expected promotion eligibility, got %+v", got)
	}
	if got := svc.LearningConsolidationV1(ctx, 5, true); got != "requires_confirmation" {
		t.Fatalf("learning_consolidation_v1 unexpected status: %s", got)
	}
	if got := svc.CapabilityExplorationV1(ctx, 3); got != "recommendations_created" {
		t.Fatalf("capability_exploration_v1 unexpected status: %s", got)
	}
	if got := svc.CrossRepoAnalysisV1(ctx, 2); got != "patterns_detected" {
		t.Fatalf("cross_repo_analysis_v1 unexpected status: %s", got)
	}
	if got := svc.MissionControlRefreshV1(ctx, 3); got != "refreshed" {
		t.Fatalf("mission_control_refresh_v1 unexpected status: %s", got)
	}
	if got := svc.RagIngestV1(ctx, 1); got != "ingested" {
		t.Fatalf("rag_ingest_v1 unexpected status: %s", got)
	}
	if got := svc.RagEvalV1(ctx, 0.81, 0.76); got != "passed" {
		t.Fatalf("rag_eval_v1 unexpected status: %s", got)
	}
	if got := svc.ToolHealthEvalV1(ctx, 5); got != "quarantined" {
		t.Fatalf("tool_health_eval_v1 unexpected status: %s", got)
	}
	if got := svc.MemoryConflictResolveV1(ctx, true); got != "resolved" {
		t.Fatalf("memory_conflict_resolve_v1 unexpected status: %s", got)
	}
	if got := svc.ComplianceEvidenceV1(ctx, "soc2"); got != "collected" {
		t.Fatalf("compliance_evidence_v1 unexpected status: %s", got)
	}
	if got := svc.AdminKPIReportV1(ctx, 2); got != "generated" {
		t.Fatalf("admin_kpi_report_v1 unexpected status: %s", got)
	}
	if got := svc.AdminAlertEvalV1(ctx, true); got != "fired" {
		t.Fatalf("admin_alert_eval_v1 unexpected status: %s", got)
	}
	if got := svc.CacheMaintenanceV1(ctx, 2); got != 2 {
		t.Fatalf("cache_maintenance_v1 unexpected result: %d", got)
	}
}

func diffKeys(left, right map[string]struct{}) []string {
	out := make([]string, 0)
	for key := range left {
		if _, ok := right[key]; !ok {
			out = append(out, key)
		}
	}
	sort.Strings(out)
	return out
}
