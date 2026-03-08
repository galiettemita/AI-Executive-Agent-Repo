package temporal

import (
	"testing"

	"github.com/stretchr/testify/mock"
	"go.temporal.io/sdk/testsuite"
)

// TestMessageProcessingWorkflowReplay verifies that MessageProcessingWorkflow
// is replay-safe: it executes deterministically with mocked activities.
// This ensures workflow code is deterministic per D2 and D7.
func TestMessageProcessingWorkflowReplay(t *testing.T) {
	suite := &testsuite.WorkflowTestSuite{}
	env := suite.NewTestWorkflowEnvironment()

	env.RegisterWorkflow(MessageProcessingWorkflow)

	// Mock all activities with deterministic results using mock.Anything for context
	env.OnActivity(ValidateEnvelopeActivity, mock.Anything, mock.Anything).Return(
		&ValidateEnvelopeResult{Valid: true, NormalizedPayload: `{"text":"hello"}`}, nil,
	)
	env.OnActivity(ClassifyIntentActivity, mock.Anything, mock.Anything).Return(
		&ClassifyIntentResult{Intent: "general_query", Confidence: 0.95}, nil,
	)
	env.OnActivity(GeneratePlanActivity, mock.Anything, mock.Anything).Return(
		&GeneratePlanResult{PlanID: "plan-001", ToolKeys: []string{"search"}, RiskLevel: "LOW"}, nil,
	)
	env.OnActivity(AuthorizePlanActivity, mock.Anything, mock.Anything).Return(
		&AuthorizePlanResult{Decision: "allow", ReceiptID: "receipt-001"}, nil,
	)
	env.OnActivity(ExecuteToolActivity, mock.Anything, mock.Anything).Return(
		&ToolExecutionActivityResult{
			ToolKey:        "search",
			Phase:          "commit",
			Success:        true,
			IdempotencyKey: "idem-001:search",
			PayloadHash:    "abc123",
		}, nil,
	)
	env.OnActivity(SynthesizeResponseActivity, mock.Anything, mock.Anything).Return(
		&SynthesizeResponseResult{ResponsePayload: "Here are the results."}, nil,
	)

	env.ExecuteWorkflow(MessageProcessingWorkflow, MessageProcessingWorkflowInput{
		MessageID:      "msg-001",
		WorkspaceID:    "ws-001",
		ChannelType:    "slack",
		RawPayload:     `{"text":"hello"}`,
		IdempotencyKey: "idem-001",
	})

	if !env.IsWorkflowCompleted() {
		t.Fatal("workflow did not complete")
	}
	if err := env.GetWorkflowError(); err != nil {
		t.Fatalf("workflow failed: %v", err)
	}

	var result MessageProcessingWorkflowResult
	if err := env.GetWorkflowResult(&result); err != nil {
		t.Fatalf("failed to get result: %v", err)
	}

	if result.TerminalState != "COMPLETED" {
		t.Errorf("expected terminal state 'COMPLETED', got %q", result.TerminalState)
	}
	if result.ResponsePayload == "" {
		t.Error("expected non-empty response payload")
	}
}

// TestMessageProcessingWorkflow_AuthDenied verifies that the workflow handles
// authorization denial correctly (deny-by-default per D3).
func TestMessageProcessingWorkflow_AuthDenied(t *testing.T) {
	suite := &testsuite.WorkflowTestSuite{}
	env := suite.NewTestWorkflowEnvironment()

	env.RegisterWorkflow(MessageProcessingWorkflow)

	env.OnActivity(ValidateEnvelopeActivity, mock.Anything, mock.Anything).Return(
		&ValidateEnvelopeResult{Valid: true, NormalizedPayload: `{"text":"delete everything"}`}, nil,
	)
	env.OnActivity(ClassifyIntentActivity, mock.Anything, mock.Anything).Return(
		&ClassifyIntentResult{Intent: "destructive_action", Confidence: 0.99}, nil,
	)
	env.OnActivity(GeneratePlanActivity, mock.Anything, mock.Anything).Return(
		&GeneratePlanResult{PlanID: "plan-002", ToolKeys: []string{"delete_all"}, RiskLevel: "CRITICAL"}, nil,
	)
	env.OnActivity(AuthorizePlanActivity, mock.Anything, mock.Anything).Return(
		&AuthorizePlanResult{Decision: "deny", Reason: "POLICY_DENY: destructive action blocked"}, nil,
	)

	env.ExecuteWorkflow(MessageProcessingWorkflow, MessageProcessingWorkflowInput{
		MessageID:      "msg-002",
		WorkspaceID:    "ws-001",
		ChannelType:    "slack",
		RawPayload:     `{"text":"delete everything"}`,
		IdempotencyKey: "idem-002",
	})

	if !env.IsWorkflowCompleted() {
		t.Fatal("workflow did not complete")
	}

	var result MessageProcessingWorkflowResult
	if err := env.GetWorkflowResult(&result); err != nil {
		t.Fatalf("failed to get result: %v", err)
	}

	if result.TerminalState != "FAILED" {
		t.Errorf("expected terminal state 'FAILED' for denied auth, got %q", result.TerminalState)
	}
}

// TestExecuteToolActivity_MissingReceipt verifies ExecuteToolActivity
// refuses execution without a receipt (D3 enforcement).
func TestExecuteToolActivity_MissingReceipt(t *testing.T) {
	a := NewActivities()
	_, err := a.ExecuteToolActivity(nil, ExecuteToolInput{
		MessageID:   "msg-003",
		WorkspaceID: "ws-001",
		ToolKey:     "search",
		ReceiptID:   "", // missing receipt
	})
	if err == nil {
		t.Fatal("expected error for missing receipt, got nil")
	}
	expected := "AUTHORIZATION_REQUIRED: no receipt provided"
	if err.Error() != expected {
		t.Errorf("expected error %q, got %q", expected, err.Error())
	}
}

// TestDeterministicJitterConsistency verifies FNV jitter produces identical
// results across calls with the same seed (D7 replay safety).
func TestDeterministicJitterConsistency(t *testing.T) {
	cfg := DefaultJitterConfig()

	result1 := ComputeDeterministicBackoff(cfg, "wf-123", "ExecuteToolActivity", 3)
	result2 := ComputeDeterministicBackoff(cfg, "wf-123", "ExecuteToolActivity", 3)

	if result1 != result2 {
		t.Errorf("jitter is non-deterministic: %v != %v", result1, result2)
	}

	result3 := ComputeDeterministicBackoff(cfg, "wf-456", "ExecuteToolActivity", 3)
	if result1 == result3 {
		t.Log("info: different seeds produced same jitter (unlikely but possible)")
	}
}
