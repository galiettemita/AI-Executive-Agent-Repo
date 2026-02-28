package contracts

import (
	"path/filepath"
	"sort"
	"strings"
	"testing"
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
