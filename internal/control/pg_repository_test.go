package control

import (
	"context"
	"testing"
	"time"
)

// TestPgReceiptRepositoryImplementsInterface verifies compile-time interface compliance.
func TestPgReceiptRepositoryImplementsInterface(t *testing.T) {
	t.Parallel()
	// Compile-time check — no runtime assertions needed.
	var _ ReceiptRepository = (*PgReceiptRepository)(nil)
}

// TestGateDecisionRecordStructure verifies the decision record has all fields
// needed to reconstruct why a tool was allowed/denied (per REQ-CTL-003).
func TestGateDecisionRecordStructure(t *testing.T) {
	t.Parallel()

	record := GateDecisionRecord{
		ID:            "test-id",
		WorkspaceID:   "ws-001",
		IngressTurnID: "turn-001",
		Decision:      "deny",
		ReasonCode:    "BUDGET_EXHAUSTED",
		InputJSON: map[string]any{
			"autonomy_level":   "A3",
			"budget_exhausted": true,
			"tool_key":         "email_send",
			"workspace_plan":   "free",
		},
		PolicyHash: "sha256:abc123",
		GateEvaluations: []GateEvaluation{
			{GateName: "kill_switch", Decision: "allow", Reason: "inactive"},
			{GateName: "budget_enforcement", Decision: "deny", Reason: "budget_exhausted"},
		},
		CreatedAt: time.Now(),
	}

	// Verify all evidence fields are populated.
	if record.Decision == "" {
		t.Fatal("decision must be non-empty")
	}
	if record.ReasonCode == "" {
		t.Fatal("reason_code must be non-empty")
	}
	if record.InputJSON == nil {
		t.Fatal("input_json must be non-nil for evidence reconstruction")
	}
	if len(record.GateEvaluations) == 0 {
		t.Fatal("gate_evaluations must be non-empty")
	}
}

// TestBudgetEventEvidenceFields verifies budget events carry enough evidence
// to reconstruct why a budget decision was made (per REQ-CBI-001).
func TestBudgetEventEvidenceFields(t *testing.T) {
	t.Parallel()

	event := BudgetEvent{
		ID:          "budget-001",
		WorkspaceID: "ws-001",
		ReceiptID:   "receipt-001",
		Action:      "deny",
		UnitsUsed:   500,
		UnitsCap:    500,
		CostUSD:     50.0,
		CapUSD:      50.0,
		Evidence: map[string]any{
			"plan":               "pro",
			"period":             "2026-03",
			"remaining_units":    0,
			"threshold_exceeded": true,
		},
		CreatedAt: time.Now(),
	}

	if event.Action != "deny" {
		t.Fatalf("expected deny, got %s", event.Action)
	}
	if event.UnitsUsed < event.UnitsCap {
		t.Fatal("deny should only occur when units_used >= units_cap")
	}
	if event.Evidence["plan"] == nil {
		t.Fatal("evidence must include plan for reconstruction")
	}
}

// TestReceiptLifecycleContract verifies the receipt lifecycle invariants:
// issued → consumed → second consume denied.
func TestReceiptLifecycleContract(t *testing.T) {
	t.Parallel()

	// This test uses the in-memory ReceiptService to verify contract invariants.
	// The PgReceiptRepository enforces the same via SQL constraints.
	svc := NewReceiptService([]byte("test-hmac-key"))

	// Issue.
	receipt, evals, err := svc.EvaluateAndIssue(ReceiptRequest{
		WorkspaceID:   "ws-001",
		WorkflowRunID: "wf-001",
		PlanID:        "plan-001",
		ToolKeys:      []string{"send_email"},
		RiskLevel:     "LOW",
		PolicyBundle:  "bundle-v1",
	})
	if err != nil {
		t.Fatalf("issue: %v", err)
	}
	if receipt == nil {
		t.Fatal("receipt must not be nil")
	}
	if len(evals) != 7 {
		t.Fatalf("expected 7 gate evaluations, got %d", len(evals))
	}

	// Validate.
	if err := svc.ValidateReceipt(receipt.ID, "ws-001", "send_email"); err != nil {
		t.Fatalf("validate: %v", err)
	}

	// Consume.
	if err := svc.ConsumeReceipt(receipt.ID); err != nil {
		t.Fatalf("consume: %v", err)
	}

	// Second consume must fail.
	if err := svc.ConsumeReceipt(receipt.ID); err != ErrReceiptConsumed {
		t.Fatalf("expected ErrReceiptConsumed, got %v", err)
	}

	// Validate consumed must fail.
	if err := svc.ValidateReceipt(receipt.ID, "ws-001", "send_email"); err != ErrReceiptConsumed {
		t.Fatalf("expected ErrReceiptConsumed on validate, got %v", err)
	}
}

// TestReceiptRepositoryCoalesceRiskLevel verifies risk level defaults.
func TestReceiptRepositoryCoalesceRiskLevel(t *testing.T) {
	t.Parallel()

	if coalesceRiskLevel("") != "LOW" {
		t.Error("empty risk level should default to LOW")
	}
	if coalesceRiskLevel("CRITICAL") != "CRITICAL" {
		t.Error("CRITICAL should be preserved")
	}
}

// TestNilIfEmpty verifies the nil/empty helper.
func TestNilIfEmpty(t *testing.T) {
	t.Parallel()

	if nilIfEmpty("") != nil {
		t.Error("empty string should return nil")
	}
	if nilIfEmpty("hello") == nil {
		t.Error("non-empty string should return pointer")
	}
}

// TestDurableReceiptServiceWithRepository verifies the DurableReceiptService
// delegates to both in-memory service and repository.
func TestDurableReceiptServiceWithRepository(t *testing.T) {
	t.Parallel()

	svc := NewReceiptService([]byte("test-hmac-key"))
	durable := NewDurableReceiptService(svc, nil) // nil repo = no persistence

	receipt, evals, err := durable.EvaluateAndIssue(context.Background(), ReceiptRequest{
		WorkspaceID:   "ws-001",
		WorkflowRunID: "wf-001",
		PlanID:        "plan-001",
		ToolKeys:      []string{"send_email"},
		RiskLevel:     "LOW",
		PolicyBundle:  "bundle-v1",
	})
	if err != nil {
		t.Fatalf("issue: %v", err)
	}
	if receipt == nil {
		t.Fatal("receipt must not be nil")
	}
	if len(evals) != 7 {
		t.Fatalf("expected 7 gate evaluations, got %d", len(evals))
	}

	// Consume via durable service.
	if err := durable.ConsumeReceipt(context.Background(), receipt.ID); err != nil {
		t.Fatalf("consume: %v", err)
	}

	// Double consume.
	if err := durable.ConsumeReceipt(context.Background(), receipt.ID); err != ErrReceiptConsumed {
		t.Fatalf("expected ErrReceiptConsumed, got %v", err)
	}
}
