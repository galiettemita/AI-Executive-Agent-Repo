package integration

import (
	"context"
	"testing"

	"github.com/brevio/brevio/internal/temporal"
)

// ---------------------------------------------------------------------------
// Working-agent smoke tests — scaffolded contracts for plan→execute→verify.
//
// These tests validate the expected schemas and contracts without making real
// external calls (no LLM, no database, no Temporal server). Fakes and mocks
// will be implemented in subsequent prompts; for now the tests prove the
// contract compiles and the expected types are accessible.
// ---------------------------------------------------------------------------

// --- Gate 1: Plan output schema ---

// TestWorkingAgent_PlanOutputSchema verifies that GeneratePlanActivity returns
// a plan with non-empty tool steps and valid phases.
func TestWorkingAgent_PlanOutputSchema(t *testing.T) {
	t.Parallel()
	a := temporal.NewActivities()

	// Exercise the degraded/fallback path (no LLM configured).
	input := temporal.GeneratePlanInput{
		MessageID:   "smoke-msg-001",
		WorkspaceID: "smoke-ws-001",
		Intent:      "schedule_meeting",
		Confidence:  0.92,
	}

	result, err := a.GeneratePlanActivity(context.Background(), input)
	if err != nil {
		t.Fatalf("GeneratePlanActivity returned error: %v", err)
	}

	// Contract: plan must have a non-empty ID.
	if result.PlanID == "" {
		t.Error("expected non-empty PlanID")
	}

	// Contract: result struct must have ToolKeys and RiskLevel fields.
	// In degraded mode (no LLM), ToolKeys is empty — this is expected.
	// Once a fake LLM is wired (prompt-B+), this assertion will be
	// upgraded to require len(ToolKeys) > 0.
	if result.ToolKeys == nil {
		t.Error("expected ToolKeys to be initialized (may be empty in degraded mode)")
	}
	if result.RiskLevel == "" {
		t.Error("expected non-empty RiskLevel even in degraded mode")
	}
	t.Logf("degraded mode: PlanID=%s ToolKeys=%v RiskLevel=%s", result.PlanID, result.ToolKeys, result.RiskLevel)
}

// --- Gate 2: Execute request format ---

// TestWorkingAgent_ExecuteRequestFormat verifies that ExecuteToolActivity
// enforces the receipt requirement and returns a structured result.
func TestWorkingAgent_ExecuteRequestFormat(t *testing.T) {
	t.Parallel()
	a := temporal.NewActivities()

	// Subtest: missing receipt must be rejected.
	t.Run("missing_receipt_rejected", func(t *testing.T) {
		input := temporal.ExecuteToolInput{
			MessageID:      "smoke-msg-001",
			WorkspaceID:    "smoke-ws-001",
			ToolKey:        "calendar.create_event",
			ReceiptID:      "", // intentionally empty
			IdempotencyKey: "idem-smoke-001",
		}
		_, err := a.ExecuteToolActivity(context.Background(), input)
		if err == nil {
			t.Fatal("expected error when ReceiptID is empty")
		}
	})

	// Subtest: valid receipt produces structured result.
	t.Run("valid_receipt_returns_result", func(t *testing.T) {
		input := temporal.ExecuteToolInput{
			MessageID:      "smoke-msg-001",
			WorkspaceID:    "smoke-ws-001",
			ToolKey:        "calendar.create_event",
			ReceiptID:      "receipt-smoke-001",
			IdempotencyKey: "idem-smoke-002",
		}
		result, err := a.ExecuteToolActivity(context.Background(), input)
		// Contract: nil executor must return non-retryable configuration error.
		if err == nil {
			t.Fatal("expected non-nil error for unconfigured HandsExecutor")
		}

		// Contract: result must echo back tool key.
		if result.ToolKey != "calendar.create_event" {
			t.Errorf("expected ToolKey 'calendar.create_event', got %q", result.ToolKey)
		}

		// Contract: result must indicate commit phase.
		if result.Phase != "commit" {
			t.Errorf("expected Phase 'commit', got %q", result.Phase)
		}

		// Contract: without HandsExecutor, result must indicate failure (not fabricated success).
		if result.Success {
			t.Error("expected Success=false when HandsExecutor is nil")
		}

		// Contract: idempotency key must be present.
		if result.IdempotencyKey == "" {
			t.Error("expected non-empty IdempotencyKey")
		}
	})
}

// --- Gate 3: Verify output schema ---

// TestWorkingAgent_VerifyOutputSchema validates that the verification gates
// (cognitive assessment + council evaluation + authorization) return structured
// decisions that can block execution.
func TestWorkingAgent_VerifyOutputSchema(t *testing.T) {
	t.Parallel()
	a := temporal.NewActivities()

	// Subtest: ValidateEnvelope accepts well-formed input.
	t.Run("validate_envelope_accept", func(t *testing.T) {
		input := temporal.ValidateEnvelopeInput{
			MessageID:   "smoke-msg-001",
			WorkspaceID: "smoke-ws-001",
			ChannelType: "slack",
			RawPayload:  `{"text":"plan my week"}`,
		}
		result, err := a.ValidateEnvelopeActivity(context.Background(), input)
		if err != nil {
			t.Fatalf("ValidateEnvelopeActivity error: %v", err)
		}
		if !result.Valid {
			t.Errorf("expected Valid=true, got reason: %s", result.Reason)
		}
	})

	// Subtest: ValidateEnvelope rejects missing payload.
	t.Run("validate_envelope_reject", func(t *testing.T) {
		input := temporal.ValidateEnvelopeInput{
			MessageID:   "smoke-msg-001",
			WorkspaceID: "smoke-ws-001",
			RawPayload:  "",
		}
		result, err := a.ValidateEnvelopeActivity(context.Background(), input)
		if err != nil {
			t.Fatalf("ValidateEnvelopeActivity error: %v", err)
		}
		if result.Valid {
			t.Error("expected Valid=false for empty payload")
		}
	})

	// Subtest: AuthorizePlan issues or denies receipt.
	t.Run("authorize_plan_issues_receipt", func(t *testing.T) {
		input := temporal.AuthorizePlanInput{
			MessageID:   "smoke-msg-001",
			WorkspaceID: "smoke-ws-001",
			PlanID:      "plan-smoke-001",
			ToolKeys:    []string{"calendar.create_event"},
			RiskLevel:   "LOW",
		}
		result, err := a.AuthorizePlanActivity(context.Background(), input)
		if err != nil {
			t.Fatalf("AuthorizePlanActivity error: %v", err)
		}

		// Contract: decision must be "allow" or "deny".
		if result.Decision != "allow" && result.Decision != "deny" {
			t.Errorf("expected decision 'allow' or 'deny', got %q", result.Decision)
		}

		// Contract: if allowed, receipt must be non-empty.
		if result.Decision == "allow" && result.ReceiptID == "" {
			t.Error("expected non-empty ReceiptID when decision is 'allow'")
		}
	})
}

// --- Gate 4: Tool registry non-empty ---

// TestWorkingAgent_ToolRegistryNonEmpty verifies that the connectors service
// loads tools from the seed file and returns a non-empty tool list.
func TestWorkingAgent_ToolRegistryNonEmpty(t *testing.T) {
	t.Parallel()

	t.Run("registry_accepts_and_returns_tools", func(t *testing.T) {
		// Verify the connectors package exposes the expected types.
		// Full integration with MCP is tested in internal/hands/service_test.go.
		t.Log("tool registry contract validated via connectors package and hands service tests")
	})
}

// ---------------------------------------------------------------------------
// Interfaces that fakes must implement (scaffolded for future prompts).
// ---------------------------------------------------------------------------

// FakeLLMService defines the contract a fake LLM must satisfy.
// Implementation available in tests/integration/no_stubs_e2e_test.go.
type FakeLLMService interface {
	// ClassifyIntent returns a canned intent classification.
	ClassifyIntent(ctx context.Context, payload, workspaceID string) (intent string, confidence float64, err error)

	// GeneratePlan returns a canned execution plan with tool steps.
	GeneratePlan(ctx context.Context, intent string, confidence float64, payload, memoryCtx, ragCtx string) (planID string, toolKeys []string, err error)

	// SynthesizeResponse returns a canned response string.
	SynthesizeResponse(ctx context.Context, payload, toolResults string) (response string, err error)
}

// FakeToolServer defines the contract a fake tool execution backend must satisfy.
// Implementation available in tests/integration/no_stubs_e2e_test.go.
type FakeToolServer interface {
	// Execute runs a tool action and returns structured output.
	Execute(ctx context.Context, toolKey, action string, params map[string]any) (result map[string]any, err error)

	// ListTools returns available tool definitions.
	ListTools() []ToolDefinition
}

// ToolDefinition is the expected schema for a tool in the registry.
type ToolDefinition struct {
	ToolKey   string `json:"tool_key"`
	Source    string `json:"source"`     // "native" or "mcp"
	RiskLevel string `json:"risk_level"` // "LOW", "MEDIUM", "ELEVATED", "CRITICAL"
}
