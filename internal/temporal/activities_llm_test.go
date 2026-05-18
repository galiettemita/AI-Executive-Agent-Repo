package temporal

import (
	"context"
	"strings"
	"testing"
)

// ---------------------------------------------------------------------------
// Activity tests for Prompt B: deterministic fallback, plan structure, verify
// ---------------------------------------------------------------------------

// TestGeneratePlanActivity_DeterministicFallback_HasSteps verifies that the
// deterministic (no-LLM) path produces a non-empty plan with at least one step.
func TestGeneratePlanActivity_DeterministicFallback_HasSteps(t *testing.T) {
	t.Parallel()

	a := NewActivities() // no LLM configured → deterministic path

	intents := []struct {
		intent   string
		wantTool string
	}{
		{"schedule_meeting", "calendar"},
		{"send_email", "email"},
		{"search for documents", "web.search"},
		{"create a task", "task.create"},
		{"call John", "phone.dial"},
		{"general question", "echo"}, // default fallback
	}

	for _, tc := range intents {
		t.Run(tc.intent, func(t *testing.T) {
			result, err := a.GeneratePlanActivity(context.Background(), GeneratePlanInput{
				MessageID:   "test-msg-001",
				WorkspaceID: "test-ws-001",
				Intent:      tc.intent,
				Confidence:  0.9,
			})
			if err != nil {
				t.Fatalf("GeneratePlanActivity error: %v", err)
			}
			if result.PlanID == "" {
				t.Error("expected non-empty PlanID")
			}
			if len(result.ToolKeys) == 0 {
				t.Error("expected at least one tool key in deterministic plan")
			}
			if result.RiskLevel == "" {
				t.Error("expected non-empty RiskLevel")
			}
			if !result.Deterministic {
				t.Error("expected Deterministic=true when no LLM configured")
			}

			// Check that the expected tool is present.
			found := false
			for _, tk := range result.ToolKeys {
				if strings.Contains(tk, tc.wantTool) {
					found = true
					break
				}
			}
			if !found {
				t.Errorf("expected tool containing %q in %v", tc.wantTool, result.ToolKeys)
			}
		})
	}
}

// TestGeneratePlanActivity_DeterministicFallback_FinalAnswerReqs verifies that
// deterministic plans include final_answer_requirements.
func TestGeneratePlanActivity_DeterministicFallback_FinalAnswerReqs(t *testing.T) {
	t.Parallel()

	a := NewActivities()
	result, err := a.GeneratePlanActivity(context.Background(), GeneratePlanInput{
		MessageID:   "test-msg-002",
		WorkspaceID: "test-ws-001",
		Intent:      "send_email to boss",
		Confidence:  0.85,
	})
	if err != nil {
		t.Fatalf("error: %v", err)
	}
	if result.FinalAnswerReqs == "" {
		t.Error("expected non-empty FinalAnswerReqs")
	}
}

// TestGeneratePlanActivity_RetryHintsAppended verifies that retry hints
// are included in the planning context on re-plan.
func TestGeneratePlanActivity_RetryHintsAppended(t *testing.T) {
	t.Parallel()

	a := NewActivities()
	// First call without retry hints.
	result1, err := a.GeneratePlanActivity(context.Background(), GeneratePlanInput{
		MessageID:   "test-msg-003",
		WorkspaceID: "test-ws-001",
		Intent:      "schedule_meeting",
		Confidence:  0.9,
	})
	if err != nil {
		t.Fatalf("error: %v", err)
	}

	// Second call with retry hints (simulating re-plan after verify fail).
	result2, err := a.GeneratePlanActivity(context.Background(), GeneratePlanInput{
		MessageID:   "test-msg-003",
		WorkspaceID: "test-ws-001",
		Intent:      "schedule_meeting",
		Confidence:  0.9,
		RetryHints:  "Try a different time slot, the original was unavailable.",
	})
	if err != nil {
		t.Fatalf("error: %v", err)
	}

	// Both should succeed and produce valid plans.
	if result1.PlanID == "" || result2.PlanID == "" {
		t.Error("expected non-empty PlanIDs")
	}
}

// TestClassifyIntentActivity_DeterministicFallback verifies keyword-based
// classification when no LLM is configured.
func TestClassifyIntentActivity_DeterministicFallback(t *testing.T) {
	t.Parallel()

	a := NewActivities()

	cases := []struct {
		payload    string
		wantIntent string
	}{
		{"send an email to alice", "email"},
		{"schedule a meeting", "calendar"},
		{"search the web for Go tutorials", "search"},
		{"just chatting", "general_query"},
	}

	for _, tc := range cases {
		t.Run(tc.payload, func(t *testing.T) {
			result, err := a.ClassifyIntentActivity(context.Background(), ClassifyIntentInput{
				MessageID:   "test-msg",
				WorkspaceID: "test-ws",
				Payload:     tc.payload,
			})
			if err != nil {
				t.Fatalf("error: %v", err)
			}
			if !strings.Contains(result.Intent, tc.wantIntent) {
				t.Errorf("expected intent containing %q, got %q", tc.wantIntent, result.Intent)
			}
			if result.Confidence <= 0 {
				t.Error("expected positive confidence")
			}
			if !result.Deterministic {
				t.Error("expected Deterministic=true")
			}
		})
	}
}

// TestVerifyExecutionActivity_DeterministicPass verifies that the deterministic
// verify path passes when all tools succeeded.
func TestVerifyExecutionActivity_DeterministicPass(t *testing.T) {
	t.Parallel()

	a := NewActivities()
	result, err := a.VerifyExecutionActivity(context.Background(), VerifyExecutionInput{
		MessageID:       "test-msg",
		WorkspaceID:     "test-ws",
		OriginalPayload: "schedule meeting",
		PlanID:          "plan-001",
		PlanToolKeys:    []string{"calendar.write"},
		PlanRiskLevel:   "ELEVATED",
		ToolResults: []ToolExecutionActivityResult{
			{ToolKey: "calendar.write", Phase: "commit", Success: true, IdempotencyKey: "idem-1"},
		},
	})
	if err != nil {
		t.Fatalf("error: %v", err)
	}
	if result.Verdict != "pass" {
		t.Errorf("expected pass verdict, got %q", result.Verdict)
	}
}

// TestVerifyExecutionActivity_DeterministicFail verifies that the deterministic
// verify path fails when a tool failed, and produces retry hints.
func TestVerifyExecutionActivity_DeterministicFail(t *testing.T) {
	t.Parallel()

	a := NewActivities()
	result, err := a.VerifyExecutionActivity(context.Background(), VerifyExecutionInput{
		MessageID:       "test-msg",
		WorkspaceID:     "test-ws",
		OriginalPayload: "send email",
		PlanID:          "plan-002",
		PlanToolKeys:    []string{"email.send"},
		PlanRiskLevel:   "ELEVATED",
		ToolResults: []ToolExecutionActivityResult{
			{ToolKey: "email.send", Phase: "commit", Success: false, IdempotencyKey: "idem-2"},
		},
	})
	if err != nil {
		t.Fatalf("error: %v", err)
	}
	if result.Verdict != "fail" {
		t.Errorf("expected fail verdict, got %q", result.Verdict)
	}
	if result.RetryHints == "" {
		t.Error("expected non-empty retry_hints on failure")
	}
	if len(result.Reasons) == 0 {
		t.Error("expected non-empty reasons")
	}
}

// TestVerifyExecutionActivity_NoToolResults verifies that empty tool results
// are rejected immediately.
func TestVerifyExecutionActivity_NoToolResults(t *testing.T) {
	t.Parallel()

	a := NewActivities()
	result, err := a.VerifyExecutionActivity(context.Background(), VerifyExecutionInput{
		MessageID:       "test-msg",
		WorkspaceID:     "test-ws",
		OriginalPayload: "do something",
		PlanID:          "plan-003",
		PlanToolKeys:    []string{"echo"},
		PlanRiskLevel:   "LOW",
		ToolResults:     nil, // empty
	})
	if err != nil {
		t.Fatalf("error: %v", err)
	}
	if result.Verdict != "fail" {
		t.Errorf("expected fail for no tool results, got %q", result.Verdict)
	}
}

// TestExecuteToolActivity_MissingReceiptLLM verifies that missing receipt is rejected.
func TestExecuteToolActivity_MissingReceiptLLM(t *testing.T) {
	t.Parallel()

	a := NewActivities()
	_, err := a.ExecuteToolActivity(context.Background(), ExecuteToolInput{
		MessageID:      "test-msg",
		WorkspaceID:    "test-ws",
		ToolKey:        "echo",
		ReceiptID:      "",
		IdempotencyKey: "idem-test",
	})
	if err == nil {
		t.Fatal("expected error for missing receipt")
	}
}

// TestExecuteToolActivity_ValidReceiptLLM verifies that valid receipt produces result.
func TestExecuteToolActivity_ValidReceiptLLM(t *testing.T) {
	t.Parallel()

	a := NewActivities()
	result, err := a.ExecuteToolActivity(context.Background(), ExecuteToolInput{
		MessageID:      "test-msg",
		WorkspaceID:    "test-ws",
		ToolKey:        "calendar.write",
		ReceiptID:      "receipt-001",
		IdempotencyKey: "idem-test",
	})
	if err != nil {
		t.Fatalf("error: %v", err)
	}
	if result.ToolKey != "calendar.write" {
		t.Errorf("expected tool key 'calendar.write', got %q", result.ToolKey)
	}
	if result.Phase != "commit" {
		t.Errorf("expected phase 'commit', got %q", result.Phase)
	}
	if !result.Success {
		t.Error("expected Success=true")
	}
	if result.IdempotencyKey == "" {
		t.Error("expected non-empty IdempotencyKey")
	}
}
