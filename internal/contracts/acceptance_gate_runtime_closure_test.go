package contracts

import (
	"net/http"
	"testing"

	"github.com/brevio/brevio/internal/control"
	"github.com/brevio/brevio/internal/crdt"
	"github.com/brevio/brevio/internal/onboarding"
	rageval "github.com/brevio/brevio/internal/rag/eval"
	"github.com/brevio/brevio/internal/security/pii"
	"github.com/brevio/brevio/internal/security/sandbox"
	"github.com/brevio/brevio/internal/workflows"
)

func TestAcceptanceGateRuntimeCoverageV91(t *testing.T) {
	t.Parallel()

	t.Run("goal_lifecycle", func(t *testing.T) {
		mux := control.NewMux(control.NewService("dev-secret"))
		goal := callJSON(t, mux, http.MethodPost, "/v1/goals", map[string]any{
			"workspace_id": "ws_v91_goal",
			"title":        "Gate goal",
			"status":       "active",
			"priority":     "high",
		}, http.StatusCreated)
		goalID := mustString(t, goal, "id")
		callJSON(t, mux, http.MethodPost, "/v1/goals/"+goalID+"/milestones", map[string]any{
			"title":  "Gate milestone",
			"status": "pending",
		}, http.StatusCreated)
		callJSON(t, mux, http.MethodGet, "/v1/goals/"+goalID+"/progress", nil, http.StatusOK)
	})

	t.Run("trust_scoring", func(t *testing.T) {
		mux := control.NewMux(control.NewService("dev-secret"))
		payload := callJSON(t, mux, http.MethodGet, "/v1/autonomy/trust-scores", nil, http.StatusOK)
		if _, ok := payload["trust_scores"]; !ok {
			t.Fatalf("trust score payload missing trust_scores: %v", payload)
		}
	})

	t.Run("trust_autonomy_upgrade", func(t *testing.T) {
		mux := control.NewMux(control.NewService("dev-secret"))
		promotions := callJSON(t, mux, http.MethodGet, "/v1/autonomy/promotions", nil, http.StatusOK)
		list, ok := promotions["promotions"].([]any)
		if !ok || len(list) == 0 {
			t.Fatalf("promotion list missing: %v", promotions)
		}
		first, ok := list[0].(map[string]any)
		if !ok {
			t.Fatalf("invalid promotion payload: %v", list[0])
		}
		promotionID, _ := first["id"].(string)
		if promotionID == "" {
			t.Fatalf("promotion id missing: %v", first)
		}
		callJSON(t, mux, http.MethodPost, "/v1/autonomy/promotions/"+promotionID+"/decide", map[string]any{
			"decision": "approve",
		}, http.StatusOK)
	})

	t.Run("learning_pipeline", func(t *testing.T) {
		mux := control.NewMux(control.NewService("dev-secret"))
		callJSON(t, mux, http.MethodPost, "/v1/learning/feedback?workspace_id=ws_v91_learning", map[string]any{
			"feedback_type": "positive",
			"content":       "learning gate",
		}, http.StatusCreated)
		callJSON(t, mux, http.MethodGet, "/v1/learning/lessons?workspace_id=ws_v91_learning", nil, http.StatusOK)
	})

	t.Run("daily_capture", func(t *testing.T) {
		mux := control.NewMux(control.NewService("dev-secret"))
		callJSON(t, mux, http.MethodGet, "/v1/captures/daily?workspace_id=ws_v91_daily", nil, http.StatusOK)
		callJSON(t, mux, http.MethodGet, "/v1/captures/daily/2026-02-27?workspace_id=ws_v91_daily", nil, http.StatusOK)
	})

	t.Run("mission_control", func(t *testing.T) {
		mux := control.NewMux(control.NewService("dev-secret"))
		callJSON(t, mux, http.MethodPut, "/v1/mission-control/config?workspace_id=ws_v91_mission", map[string]any{
			"refresh_cadence_minutes": 15,
		}, http.StatusOK)
		callJSON(t, mux, http.MethodGet, "/v1/mission-control/snapshot?workspace_id=ws_v91_mission", nil, http.StatusOK)
	})

	t.Run("self_modification_gate", func(t *testing.T) {
		mux := control.NewMux(control.NewService("dev-secret"))
		callJSON(t, mux, http.MethodPut, "/v1/self-modification/policy?workspace_id=ws_v91_selfmod", map[string]any{
			"enabled":          true,
			"require_approval": true,
			"max_allowed_risk": "elevated",
		}, http.StatusOK)
	})

	t.Run("cross_repo_intelligence", func(t *testing.T) {
		mux := control.NewMux(control.NewService("dev-secret"))
		callJSON(t, mux, http.MethodGet, "/v1/codebase/dependencies?workspace_id=ws_v91_codebase", nil, http.StatusOK)
		callJSON(t, mux, http.MethodGet, "/v1/codebase/patterns?workspace_id=ws_v91_codebase", nil, http.StatusOK)
	})

	t.Run("capability_exploration", func(t *testing.T) {
		mux := control.NewMux(control.NewService("dev-secret"))
		callJSON(t, mux, http.MethodGet, "/v1/capabilities/recommendations?workspace_id=ws_v91_exploration", nil, http.StatusOK)
	})

	t.Run("code_context_export", func(t *testing.T) {
		mux := control.NewMux(control.NewService("dev-secret"))
		export := callJSON(t, mux, http.MethodPost, "/v1/codebase/context-export?workspace_id=ws_v91_export", map[string]any{
			"format": "markdown",
		}, http.StatusCreated)
		exportID := mustString(t, export, "id")
		callJSON(t, mux, http.MethodGet, "/v1/codebase/context-export/"+exportID+"?workspace_id=ws_v91_export", nil, http.StatusOK)
	})

	t.Run("adaptive_discovery", func(t *testing.T) {
		svc := onboarding.NewService()
		workspaceID := onboarding.NewWorkspaceID()
		stageAnswers := map[string]map[string]string{
			"operator_profile_intake_v1": {
				"role":  "operator",
				"goals": "stability",
			},
			"behavior_policy_calibration_v1": {
				"tone": "direct",
				"risk": "medium",
			},
			"codebase_map_ingestion_v1": {
				"repo":  "ai-executive-agent-repo",
				"stack": "go",
			},
			"system_map_ingestion_v1": {
				"integrations": "github",
				"sla":          "p95 under 500ms",
			},
		}
		if err := svc.CompleteOnboarding(workspaceID, stageAnswers); err != nil {
			t.Fatalf("complete onboarding: %v", err)
		}
		profile, persona, policy, err := svc.WorkspaceState(workspaceID)
		if err != nil {
			t.Fatalf("workspace state: %v", err)
		}
		if profile.VersionInt < 1 || persona.VersionInt < 1 || policy.VersionInt < 1 {
			t.Fatalf("expected versioned discovery artifacts, got profile=%d persona=%d policy=%d", profile.VersionInt, persona.VersionInt, policy.VersionInt)
		}
	})
}

func TestAcceptanceGateRuntimeCoverageV92(t *testing.T) {
	t.Parallel()

	t.Run("context_budget_enforcement", func(t *testing.T) {
		mux := control.NewMux(control.NewService("dev-secret"))
		callJSON(t, mux, http.MethodPut, "/v1/context/budget", map[string]any{
			"workspace_id":  "ws_v92_context",
			"budget_tokens": 2048,
			"status":        "active",
			"allocations":   map[string]int{"history": 1024},
		}, http.StatusOK)
		callJSON(t, mux, http.MethodGet, "/v1/context/allocations?workspace_id=ws_v92_context", nil, http.StatusOK)
	})

	t.Run("rag_pipeline_functional", func(t *testing.T) {
		mux := control.NewMux(control.NewService("dev-secret"))
		collection := callJSON(t, mux, http.MethodPost, "/v1/rag/collections", map[string]any{
			"workspace_id": "ws_v92_rag",
			"name":         "gate-rag",
		}, http.StatusCreated)
		collectionID := mustString(t, collection, "id")
		callJSON(t, mux, http.MethodPost, "/v1/rag/collections/"+collectionID+"/ingest", map[string]any{
			"documents": []string{"gate content"},
		}, http.StatusAccepted)
	})

	t.Run("rag_eval_gate", func(t *testing.T) {
		svc := rageval.NewService()
		passing := svc.Evaluate("collection_gate", 0.91, 0.80)
		if !passing.Pass {
			t.Fatalf("expected passing rag eval score: %+v", passing)
		}
		failing := svc.Evaluate("collection_gate_fail", 0.60, 0.50)
		if failing.Pass {
			t.Fatalf("expected failing rag eval score: %+v", failing)
		}
	})

	t.Run("session_management", func(t *testing.T) {
		mux := control.NewMux(control.NewService("dev-secret"))
		callJSON(t, mux, http.MethodGet, "/v1/sessions/session_gate?workspace_id=ws_v92_sessions&user_id=user_gate", nil, http.StatusOK)
		callJSON(t, mux, http.MethodGet, "/v1/sessions/session_gate/entities?workspace_id=ws_v92_sessions", nil, http.StatusOK)
	})

	t.Run("temporal_reasoning", func(t *testing.T) {
		mux := control.NewMux(control.NewService("dev-secret"))
		callJSON(t, mux, http.MethodPost, "/v1/temporal/resolve", map[string]any{
			"workspace_id":   "ws_v92_temporal",
			"expression":     "tomorrow 10am",
			"reference_date": "2026-02-27",
		}, http.StatusOK)
		callJSON(t, mux, http.MethodPost, "/v1/temporal/conflicts", map[string]any{
			"workspace_id":   "ws_v92_temporal",
			"proposed_start": "2026-02-28T10:10:00Z",
			"proposed_end":   "2026-02-28T10:20:00Z",
		}, http.StatusOK)
	})

	t.Run("guardrails_runtime", func(t *testing.T) {
		mux := control.NewMux(control.NewService("dev-secret"))
		callJSON(t, mux, http.MethodPut, "/v1/guardrails/config", map[string]any{
			"workspace_id":               "ws_v92_guardrails",
			"enable_pii_redaction":       true,
			"enable_jailbreak_detection": true,
			"block_threshold":            85,
		}, http.StatusOK)
		callJSON(t, mux, http.MethodGet, "/v1/guardrails/events", nil, http.StatusOK)
	})

	t.Run("tool_health_scoring", func(t *testing.T) {
		mux := control.NewMux(control.NewService("dev-secret"))
		callJSON(t, mux, http.MethodGet, "/v1/tools/health/calendar.create_event?workspace_id=ws_v92_health", nil, http.StatusOK)
	})

	t.Run("feature_flag_system", func(t *testing.T) {
		mux := control.NewMux(control.NewService("dev-secret"))
		callJSON(t, mux, http.MethodPost, "/v1/flags", map[string]any{
			"key":       "gate_flag",
			"flag_type": "boolean",
			"enabled":   true,
		}, http.StatusAccepted)
		callJSON(t, mux, http.MethodPost, "/v1/flags/gate_flag/evaluate", map[string]any{
			"attributes": map[string]string{"workspace": "ws_v92_flags"},
		}, http.StatusOK)
	})

	t.Run("crdt_conflict_resolution", func(t *testing.T) {
		svc := crdt.NewService()
		if _, conflict := svc.Apply("memory_gate", "actor_a", 1, "alpha"); conflict {
			t.Fatal("unexpected conflict on initial apply")
		}
		if _, conflict := svc.Apply("memory_gate", "actor_a", 1, "beta"); !conflict {
			t.Fatal("expected conflict on stale apply")
		}
		conflicts := svc.ListConflicts()
		if len(conflicts) != 1 {
			t.Fatalf("expected one conflict, got %d", len(conflicts))
		}
		if _, ok := svc.ResolveConflict(conflicts[0].ID, "resolved"); !ok {
			t.Fatal("expected conflict resolution success")
		}
	})

	t.Run("streaming_sla", func(t *testing.T) {
		mux := control.NewMux(control.NewService("dev-secret"))
		payload := callJSON(t, mux, http.MethodPut, "/v1/streaming/config", map[string]any{
			"workspace_id":           "ws_v92_streaming",
			"first_byte_sla_ms":      450,
			"chunk_size_bytes":       2048,
			"ack_enabled":            true,
			"typing_indicator":       true,
			"progressive_disclosure": true,
		}, http.StatusOK)
		firstByte, _ := payload["first_byte_sla_ms"].(float64)
		if firstByte > 500 {
			t.Fatalf("streaming first-byte sla exceeds threshold: %v", firstByte)
		}
	})

	t.Run("error_communication", func(t *testing.T) {
		mux := control.NewMux(control.NewService("dev-secret"))
		callJSON(t, mux, http.MethodGet, "/v1/errors/taxonomy", nil, http.StatusOK)
		callJSON(t, mux, http.MethodPost, "/v1/errors/templates", map[string]any{
			"workspace_id": "ws_v92_errors",
			"persona":      "default",
			"code_pattern": "FEATURE_DISABLED",
			"template":     "The feature is currently disabled for this workspace.",
			"status":       "active",
		}, http.StatusCreated)
	})

	t.Run("event_schema_registry", func(t *testing.T) {
		mux := control.NewMux(control.NewService("dev-secret"))
		callJSON(t, mux, http.MethodPost, "/v1/event-schemas/BREVIO.gate.event.v1/versions", map[string]any{
			"schema": "{\"type\":\"object\"}",
			"status": "active",
		}, http.StatusCreated)
		callJSON(t, mux, http.MethodPost, "/v1/event-schemas/BREVIO.gate.event.v1/validate", map[string]any{
			"event": map[string]any{"type": "BREVIO.gate.event.v1", "version": 1},
		}, http.StatusOK)
	})

	t.Run("compliance_automation", func(t *testing.T) {
		mux := control.NewMux(control.NewService("dev-secret"))
		callJSON(t, mux, http.MethodPost, "/v1/compliance/frameworks", map[string]any{
			"workspace_id": "ws_v92_compliance",
			"key":          "soc2",
		}, http.StatusCreated)
		callJSON(t, mux, http.MethodGet, "/v1/compliance/evidence?workspace_id=ws_v92_compliance", nil, http.StatusOK)
	})

	t.Run("caching_layers", func(t *testing.T) {
		mux := control.NewMux(control.NewService("dev-secret"))
		callJSON(t, mux, http.MethodPost, "/v1/cache/policies", map[string]any{
			"workspace_id": "ws_v92_cache",
			"cache_key":    "compiled_context",
			"ttl_seconds":  600,
			"max_bytes":    1048576,
			"enabled":      true,
		}, http.StatusCreated)
		callJSON(t, mux, http.MethodGet, "/v1/cache/stats?workspace_id=ws_v92_cache", nil, http.StatusOK)
	})

	t.Run("model_tier_enforcement", func(t *testing.T) {
		mux := control.NewMux(control.NewService("dev-secret"))
		callJSON(t, mux, http.MethodPost, "/v1/model-tiers/policies?workspace_id=ws_v92_tier", map[string]any{
			"workspace_id": "ws_v92_tier",
			"tier":         "T3",
			"enabled":      true,
		}, http.StatusCreated)
		payload := callJSON(t, mux, http.MethodGet, "/v1/model-tiers/overrides?workspace_id=ws_v92_tier", nil, http.StatusOK)
		overrides, ok := payload["overrides"].([]any)
		if !ok || len(overrides) == 0 {
			t.Fatalf("expected model tier overrides payload, got %v", payload)
		}
	})

	t.Run("react_early_exit", func(t *testing.T) {
		constraints := workflows.ReasoningConstraintsForTier("T3")
		if constraints.ExecutorLoopLimit != 10 {
			t.Fatalf("unexpected T3 executor limit: %d", constraints.ExecutorLoopLimit)
		}
		if constraints.MaxPlanCandidates != 3 {
			t.Fatalf("unexpected T3 plan candidate limit: %d", constraints.MaxPlanCandidates)
		}
	})

	t.Run("security_hardening", func(t *testing.T) {
		piiSvc := pii.NewService()
		record, err := piiSvc.EncryptField("email", "user@example.com")
		if err != nil {
			t.Fatalf("pii encrypt: %v", err)
		}
		plaintext, err := piiSvc.DecryptField(record)
		if err != nil {
			t.Fatalf("pii decrypt: %v", err)
		}
		if plaintext != "user@example.com" {
			t.Fatalf("unexpected decrypted payload: %s", plaintext)
		}

		sandboxSvc := sandbox.NewService()
		allowed, reason := sandboxSvc.IsAllowedURL("http://169.254.169.254/latest/meta-data")
		if allowed || reason != "IMDS_BLOCKED" {
			t.Fatalf("expected IMDS block, got allowed=%t reason=%s", allowed, reason)
		}
	})

	t.Run("admin_backend", func(t *testing.T) {
		mux := control.NewMux(control.NewService("dev-secret"))
		callJSON(t, mux, http.MethodGet, "/v1/admin/operations/dashboard", nil, http.StatusOK)
		callJSON(t, mux, http.MethodGet, "/v1/admin/kpi/report", nil, http.StatusOK)
	})

	t.Run("structured_generation", func(t *testing.T) {
		actionProposal := loadSchemaDocument(t, "action_proposal.v1.json")
		props := getObject(t, actionProposal, "properties")
		assertHasProperties(t, props, "intent", "actions", "risk", "requires_approval")
		assertRequiredIncludes(t, actionProposal, "intent", "actions", "risk", "requires_approval")
	})
}
