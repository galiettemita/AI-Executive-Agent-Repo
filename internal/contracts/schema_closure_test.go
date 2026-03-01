package contracts

import (
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"testing"
)

func TestJSONSchemaClosure(t *testing.T) {
	t.Parallel()

	required := []string{
		"tool_call.v9.json",
		"error.v9.json",
		"capability_resolver_contract.v1.json",
		"capability_extractor_output.v1.json",
		"capability_resolve_request.v1.json",
		"capability_resolve_response.v1.json",
		"provisioning_policy.v1.json",
		"provision_start_request.v1.json",
		"server_artifact_manifest.v1.json",
		"llm_request.v1.json",
		"provisioning_approval_message.v1.json",
		"provisioning_status_message.v1.json",
		"provisioning_security_justification.v1.json",
		"provisioning_rank_explainer.v1.json",
		"action_proposal.v1.json",
		"goal_item.v1.json",
		"goal_progress_update.v1.json",
		"mission_control_layout.v1.json",
		"trust_score_report.v1.json",
		"promotion_proposal.v1.json",
		"feedback_submission.v1.json",
		"lesson_proposal.v1.json",
		"daily_capture_output.v1.json",
		"capability_recommendation.v1.json",
		"code_context_export_request.v1.json",
		"debt_resolution_task.v1.json",
		"discovery_followup.v1.json",
		"morning_briefing.v1.json",
		"context_budget_config.v1.json",
		"context_allocation_report.v1.json",
		"rag_collection_config.v1.json",
		"rag_search_request.v1.json",
		"rag_search_response.v1.json",
		"session_context.v1.json",
		"temporal_expression.v1.json",
		"scheduling_conflict_report.v1.json",
		"guardrail_event.v1.json",
		"tool_health_report.v1.json",
		"feature_flag_evaluation.v1.json",
		"error_message.v1.json",
		"compliance_evidence_manifest.v1.json",
		"dsr_request.v1.json",
		"admin_kpi_report.v1.json",
		"admin_alert.v1.json",
		"memory_conflict_report.v1.json",
		"model_tier_override_request.v1.json",
		"whatsapp_webhook_payload.v1.json",
		"imessage_webhook_payload.v1.json",
		"activity_ledger_response.v1.json",
		"trust_receipt_evidence_response.v1.json",
		"provision_start_response.v1.json",
		"provision_status_response.v1.json",
		"catalog_search_response.v1.json",
		"provisioning_budget_request.v1.json",
		"provisioning_budget_response.v1.json",
		"forensic_replay_response.v1.json",
		"review_tasks_list_response.v1.json",
		"review_task_decision.v1.json",
		"brain_turn_request.v1.json",
		"brain_turn_response.v1.json",
		"plan_evaluate_request.v1.json",
		"plan_evaluate_response.v1.json",
		"tool_execution_response.v1.json",
		"outbound_send_request.v1.json",
		"canvas_push_request.v1.json",
	}

	root := repositoryRoot(t)
	schemaDir := filepath.Join(root, "schemas")

	entries, err := os.ReadDir(schemaDir)
	if err != nil {
		t.Fatalf("read schemas directory: %v", err)
	}
	requiredSet := map[string]struct{}{}
	for _, file := range required {
		requiredSet[file] = struct{}{}
	}
	extraFiles := make([]string, 0)
	seenRequired := map[string]struct{}{}
	for _, entry := range entries {
		name := entry.Name()
		if _, ok := requiredSet[name]; ok {
			seenRequired[name] = struct{}{}
			continue
		}
		if filepath.Ext(name) == ".json" {
			extraFiles = append(extraFiles, name)
		}
	}
	if len(extraFiles) != 0 {
		sort.Strings(extraFiles)
		t.Fatalf("schemas directory has unexpected json schemas: %v", extraFiles)
	}
	if len(seenRequired) != len(required) {
		t.Fatalf("schema catalog mismatch: got=%d required=%d", len(seenRequired), len(required))
	}

	for _, file := range required {
		path := filepath.Join(schemaDir, file)
		body, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("read schema %s: %v", file, err)
		}

		var doc map[string]any
		if err := json.Unmarshal(body, &doc); err != nil {
			t.Fatalf("parse schema %s: %v", file, err)
		}

		if doc["additionalProperties"] != false {
			t.Fatalf("schema %s must set additionalProperties=false", file)
		}
		properties, ok := doc["properties"].(map[string]any)
		if !ok || len(properties) == 0 {
			t.Fatalf("schema %s must define non-empty properties", file)
		}
	}
}

func repositoryRoot(t *testing.T) string {
	t.Helper()

	_, currentFile, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("unable to resolve current file path")
	}
	return filepath.Clean(filepath.Join(filepath.Dir(currentFile), "..", ".."))
}
