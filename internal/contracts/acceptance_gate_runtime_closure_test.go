package contracts

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"

	"github.com/brevio/brevio/internal/control"
	"github.com/brevio/brevio/internal/crdt"
	"github.com/brevio/brevio/internal/database"
	"github.com/brevio/brevio/internal/gateway"
	"github.com/brevio/brevio/internal/integration"
	"github.com/brevio/brevio/internal/llm"
	"github.com/brevio/brevio/internal/onboarding"
	rageval "github.com/brevio/brevio/internal/rag/eval"
	"github.com/brevio/brevio/internal/security/pii"
	"github.com/brevio/brevio/internal/security/sandbox"
	"github.com/brevio/brevio/internal/structured_generation"
	"github.com/brevio/brevio/internal/workflows"
	"github.com/google/uuid"
)

func TestAcceptanceGateRuntimeCoverageV9(t *testing.T) {
	t.Parallel()

	t.Run("schema_closure", func(t *testing.T) {
		toolCall := loadSchemaDocument(t, "tool_call.v9.json")
		toolCallProps := getObject(t, toolCall, "properties")
		assertHasProperties(t, toolCallProps, "tool_key", "idempotency_key", "arguments", "requested_risk", "workspace_id", "ingress_turn_id")
		assertRequiredIncludes(t, toolCall, "tool_key", "idempotency_key", "arguments", "workspace_id", "ingress_turn_id")

		errorSchema := loadSchemaDocument(t, "error.v9.json")
		errorProps := getObject(t, errorSchema, "properties")
		assertHasProperties(t, errorProps, "error_code", "message", "retryable", "retry_after_ms", "user_message")
		assertRequiredIncludes(t, errorSchema, "error_code", "message", "retryable", "user_message")
	})

	t.Run("determinism", func(t *testing.T) {
		svc := llm.NewService()
		req := llm.Request{
			WorkspaceID: "ws_v9_determinism",
			PromptKey:   "brain.planner.v9",
			Input:       "plan deterministic turn",
			Tier:        "T2",
			ModelID:     "model-a",
			ProviderID:  "provider-a",
		}

		first := svc.Generate(req)
		for i := 0; i < 19; i++ {
			next := svc.Generate(req)
			if next.PlanJSON != first.PlanJSON {
				t.Fatalf("deterministic mismatch on replay run %d", i+2)
			}
		}
		if svc.ReplayHitCount() < 19 {
			t.Fatalf("expected replay hits >= 19, got %d", svc.ReplayHitCount())
		}
	})

	t.Run("webhook_security", func(t *testing.T) {
		const secret = "runtime-secret"
		svc := gateway.NewService(secret)
		svc.BindWorkspace("whatsapp", "+15550001111", uuid.MustParse("018f3f6a-9a0f-7cc6-8f2f-1f0f2d2f2d2f"))

		payload := []byte(`{"channel":"whatsapp","channel_identifier":"+15550001111","user_channel_id":"runtime_user","nonce":"runtime_nonce","message":"hello"}`)

		invalidReq := httptest.NewRequest(http.MethodPost, "/v1/gateway/webhook/whatsapp", bytes.NewReader(payload))
		invalidReq.Header.Set("X-Signature", "invalid")
		invalidResp := httptest.NewRecorder()
		svc.HandleInbound(invalidResp, invalidReq)
		if invalidResp.Code != http.StatusUnauthorized {
			t.Fatalf("expected invalid signature rejection, got %d", invalidResp.Code)
		}

		validReq := httptest.NewRequest(http.MethodPost, "/v1/gateway/webhook/whatsapp", bytes.NewReader(payload))
		validReq.Header.Set("X-Signature", signWebhookPayload(secret, payload))
		validResp := httptest.NewRecorder()
		svc.HandleInbound(validResp, validReq)
		if validResp.Code != http.StatusAccepted {
			t.Fatalf("expected accepted webhook, got %d", validResp.Code)
		}

		replayReq := httptest.NewRequest(http.MethodPost, "/v1/gateway/webhook/whatsapp", bytes.NewReader(payload))
		replayReq.Header.Set("X-Signature", signWebhookPayload(secret, payload))
		replayResp := httptest.NewRecorder()
		svc.HandleInbound(replayResp, replayReq)
		if replayResp.Code != http.StatusConflict {
			t.Fatalf("expected replay rejection, got %d", replayResp.Code)
		}

		auditEntries := svc.AuditEntries()
		if !containsString(auditEntries, "BREVIO.security.webhook.signature_invalid.v1") {
			t.Fatalf("missing canonical invalid-signature event: %v", auditEntries)
		}
		if !containsString(auditEntries, "BREVIO.security.webhook.replay_blocked.v1") {
			t.Fatalf("missing canonical replay-blocked event: %v", auditEntries)
		}
		if !containsString(auditEntries, "BREVIO.ingress.received.v1") {
			t.Fatalf("missing canonical ingress-received event: %v", auditEntries)
		}
	})

	t.Run("acceptance_suites_1_12", func(t *testing.T) {
		svc := integration.NewService("runtime-integration-secret")
		workspaceID := uuid.MustParse("018f3f6a-9a0f-7cc6-8f2f-1f0f2d2f2d2f")
		svc.BindWorkspace("whatsapp", "+15550009999", workspaceID)

		status, err := svc.IngestWebhook(integration.WebhookPayload{
			Channel:           "whatsapp",
			ChannelIdentifier: "+15550009999",
			UserChannelID:     "runtime_acceptance",
			Nonce:             "runtime_acceptance_nonce",
			Message:           "run acceptance gate",
		})
		if err != nil {
			t.Fatalf("ingest webhook: %v", err)
		}
		if status != http.StatusAccepted {
			t.Fatalf("unexpected ingest status: %d", status)
		}

		result, err := svc.ProcessNextQueuedTurn(context.Background(), false)
		if err != nil {
			t.Fatalf("process queued turn: %v", err)
		}
		if result.GateDecision != "allow" || !result.Simulated || !result.Committed || result.WorkflowState != "TERMINAL" {
			t.Fatalf("unexpected pipeline result: %+v", result)
		}

		executorEvents := svc.ExecutorAuditEventTypes()
		for _, event := range []string{
			"BREVIO.hands.tool.simulated.v1",
			"BREVIO.hands.tool.committed.v1",
			"BREVIO.trust.receipt.created.v1",
			"BREVIO.trust.evidence.attached.v1",
		} {
			if !containsString(executorEvents, event) {
				t.Fatalf("missing executor canonical event %s in %v", event, executorEvents)
			}
		}
		gatewayEvents := svc.GatewayAuditEventTypes()
		if !containsString(gatewayEvents, "BREVIO.ingress.received.v1") {
			t.Fatalf("missing gateway canonical ingress event in %v", gatewayEvents)
		}
	})

	t.Run("workspace_isolation", func(t *testing.T) {
		pool := &database.Pool{}
		if _, err := pool.Exec(context.Background(), "SELECT 1"); !errors.Is(err, database.ErrWorkspaceUnset) {
			t.Fatalf("expected ErrWorkspaceUnset from pool exec guard, got %v", err)
		}

		workspaceID := uuid.MustParse("018f3f6a-9a0f-7cc6-8f2f-1f0f2d2f2d2f")
		ctx := database.WithWorkspaceID(context.Background(), workspaceID)
		got, err := database.WorkspaceIDFromContext(ctx)
		if err != nil {
			t.Fatalf("workspace from context: %v", err)
		}
		if got != workspaceID {
			t.Fatalf("workspace mismatch: got=%s want=%s", got, workspaceID)
		}
	})

	t.Run("provisioning_pipeline", func(t *testing.T) {
		svc := workflows.NewService()
		result := svc.ProvisioningV9(context.Background(), "")
		if result.Status != "active" {
			t.Fatalf("unexpected provisioning status: %s", result.Status)
		}
		if len(result.ExecutedSteps) == 0 || result.ExecutedSteps[len(result.ExecutedSteps)-1] != "Active" {
			t.Fatalf("unexpected provisioning execution steps: %v", result.ExecutedSteps)
		}
	})

	t.Run("onboarding_completion", func(t *testing.T) {
		svc := onboarding.NewService()
		workspaceID := onboarding.NewWorkspaceID()
		stageAnswers := acceptanceOnboardingStageAnswers()
		if err := svc.CompleteOnboarding(workspaceID, stageAnswers); err != nil {
			t.Fatalf("complete onboarding: %v", err)
		}
		profile, persona, policy, err := svc.WorkspaceState(workspaceID)
		if err != nil {
			t.Fatalf("workspace state: %v", err)
		}
		if profile.VersionInt < 1 || persona.VersionInt < 1 || policy.VersionInt < 1 {
			t.Fatalf("expected versioned onboarding artifacts, got profile=%d persona=%d policy=%d", profile.VersionInt, persona.VersionInt, policy.VersionInt)
		}
	})

	t.Run("provisioning_recovery", func(t *testing.T) {
		svc := workflows.NewService()
		result := svc.ProvisioningV9(context.Background(), "DeployServer")
		if result.Status != "failed" {
			t.Fatalf("expected failed provisioning for compensation test, got %s", result.Status)
		}
		if len(result.CompensatedSteps) == 0 {
			t.Fatal("expected compensation steps")
		}
		if result.CompensatedSteps[0] != "DeployServer" {
			t.Fatalf("unexpected first compensation step: %s", result.CompensatedSteps[0])
		}
		if len(result.CompensatedSteps) != len(result.ExecutedSteps) {
			t.Fatalf("expected full reverse compensation, executed=%d compensated=%d", len(result.ExecutedSteps), len(result.CompensatedSteps))
		}
	})

	t.Run("deterministic_llm", func(t *testing.T) {
		svc := llm.NewService()
		req := llm.Request{
			WorkspaceID: "ws_v9_llm",
			PromptKey:   "brain.planner.v9",
			Input:       "check deterministic output",
			Tier:        "T3",
			ModelID:     "model-a",
			ProviderID:  "provider-a",
		}
		first := svc.Generate(req)
		if !strings.Contains(first.PlanJSON, `"temperature":0`) {
			t.Fatalf("deterministic plan missing temperature=0: %s", first.PlanJSON)
		}
		if !strings.Contains(first.PlanJSON, `"top_p":1`) {
			t.Fatalf("deterministic plan missing top_p=1: %s", first.PlanJSON)
		}
		second := svc.Generate(req)
		if !second.FromReplay || second.PlanJSON != first.PlanJSON {
			t.Fatalf("expected replay deterministic response, first=%+v second=%+v", first, second)
		}
	})

	t.Run("cve_scanning", func(t *testing.T) {
		root := repositoryRoot(t)
		ciPath := filepath.Join(root, ".github", "workflows", "ci.yaml")
		assertFileContainsTokens(t, ciPath, []string{
			"trivy",
			"trufflehog",
			"govulncheck baseline",
			"bash scripts/security/run_govulncheck.sh",
		})
		assertFileContainsTokens(t, filepath.Join(root, "scripts", "security", "run_security_validation.sh"), []string{
			"run_govulncheck.sh",
			"trivy",
			"trufflehog",
		})
	})
}

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
		stageAnswers := acceptanceOnboardingStageAnswers()
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
		followups := svc.ListAdaptiveQuestions(workspaceID)
		if len(followups) == 0 {
			t.Fatal("expected adaptive discovery followup questions")
		}
		answered, ok, err := svc.AnswerAdaptiveQuestion(workspaceID, followups[0].FollowupID, "Prioritize onboarding automation for reporting.")
		if err != nil {
			t.Fatalf("answer adaptive followup: %v", err)
		}
		if !ok || answered.Status != "answered" {
			t.Fatalf("expected answered adaptive followup state, got ok=%v state=%+v", ok, answered)
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
		payload := callJSON(t, mux, http.MethodGet, "/v1/compliance/evidence?workspace_id=ws_v92_compliance", nil, http.StatusOK)
		evidenceItems, ok := payload["evidence"].([]any)
		if !ok || len(evidenceItems) == 0 {
			t.Fatalf("expected evidence payload, got %v", payload)
		}
		first, ok := evidenceItems[0].(map[string]any)
		if !ok {
			t.Fatalf("invalid evidence item: %v", evidenceItems[0])
		}
		hashValue, _ := first["sha256"].(string)
		if !strings.HasPrefix(hashValue, "sha256:") {
			t.Fatalf("expected prefixed sha256 evidence hash, got %q", hashValue)
		}
		digest := strings.TrimPrefix(hashValue, "sha256:")
		if len(digest) != 64 {
			t.Fatalf("expected 64-hex evidence digest, got %q", hashValue)
		}
		if _, err := hex.DecodeString(digest); err != nil {
			t.Fatalf("expected valid evidence hex digest, got %q: %v", hashValue, err)
		}
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
		reactResult := workflows.ExecuteReActEarlyExit("T3", 10, nil, nil)
		if !reactResult.EarlyExit {
			t.Fatalf("expected ReAct early exit on max steps: %+v", reactResult)
		}
		if reactResult.ExitReason != "MAX_STEPS_REACHED" {
			t.Fatalf("unexpected ReAct exit reason: %+v", reactResult)
		}
		if len(reactResult.PartialResults) == 0 {
			t.Fatalf("expected partial results on early exit: %+v", reactResult)
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

		svc := structured_generation.NewService()
		canonicalJSON, err := svc.CanonicalJSON(structured_generation.ActionProposal{
			Intent: "prepare executive update",
			Actions: []structured_generation.Action{
				{
					Tool:           "tasks.create_task",
					Operation:      "create",
					Params:         map[string]any{"title": "Draft status memo"},
					IdempotencyKey: "idem_abcdefghijklmnop",
				},
				{
					Tool:           "calendar.create_event",
					Operation:      "create",
					Params:         map[string]any{"title": "Ops review"},
					IdempotencyKey: "idem_abcdefghijklmnpp",
				},
			},
			Risk:             structured_generation.Risk{Impact: "low", RollbackPlan: "delete created objects"},
			RequiresApproval: false,
		})
		if err != nil {
			t.Fatalf("structured generation canonicalization failed: %v", err)
		}
		if !strings.Contains(canonicalJSON, "\"tool\":\"calendar.create_event\"") {
			t.Fatalf("expected lexical canonical ordering in output json: %s", canonicalJSON)
		}
	})
}

func acceptanceOnboardingStageAnswers() map[string]map[string]string {
	return map[string]map[string]string{
		"operator_profile_intake_v1": {
			"role":               "operator",
			"goals":              "stability",
			"industry":           "saas",
			"team_size":          "20",
			"timezone":           "UTC",
			"decision_style":     "data-driven",
			"communication_pref": "concise",
			"kpi_primary":        "uptime",
		},
		"behavior_policy_calibration_v1": {
			"tone":                "direct",
			"risk_tolerance":      "medium",
			"autonomy_preference": "A2",
			"approval_threshold":  "critical_only",
			"proactive_mode":      "enabled",
			"notification_window": "09:00-18:00",
			"initiative_level":    "high",
		},
		"codebase_map_ingestion_v1": {
			"repo":             "ai-executive-agent-repo",
			"stack":            "go",
			"planning_horizon": "quarterly",
			"meeting_load":     "medium",
			"focus_mode":       "async",
		},
		"system_map_ingestion_v1": {
			"integrations":     "github",
			"sla":              "p95 under 500ms",
			"escalation_path":  "oncall",
			"privacy_mode":     "strict",
			"audit_strictness": "high",
			"delivery_cadence": "weekly",
			"context_budget":   "balanced",
			"write_actions":    "confirm",
			"language":         "en-US",
		},
	}
}

func signWebhookPayload(secret string, payload []byte) string {
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write(payload)
	return hex.EncodeToString(mac.Sum(nil))
}

func containsString(items []string, needle string) bool {
	for _, item := range items {
		if item == needle {
			return true
		}
	}
	return false
}
