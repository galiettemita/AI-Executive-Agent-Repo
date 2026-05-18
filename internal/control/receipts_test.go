package control

import (
	"testing"
	"time"
)

func TestReceiptIssuance(t *testing.T) {
	t.Parallel()
	svc := NewReceiptService([]byte("test-hmac-key"))

	receipt, evals, err := svc.EvaluateAndIssue(ReceiptRequest{
		WorkspaceID:   "ws-001",
		WorkflowRunID: "wf-001",
		PlanID:        "plan-001",
		ToolKeys:      []string{"send_email", "calendar_create"},
		RiskLevel:     "LOW",
		PolicyBundle:  "bundle-v1",
	})

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if receipt == nil {
		t.Fatal("receipt should not be nil")
	}
	if receipt.Decision != "allow" {
		t.Fatalf("expected allow, got %s", receipt.Decision)
	}
	if len(evals) != 7 {
		t.Fatalf("expected 7 gate evaluations, got %d", len(evals))
	}
	for _, eval := range evals {
		if eval.Decision != "allow" {
			t.Fatalf("gate %s should allow, got %s", eval.GateName, eval.Decision)
		}
	}
	if receipt.ExpiresAt.Before(receipt.IssuedAt) {
		t.Fatal("receipt should expire after issuance")
	}
}

func TestReceiptCriticalRiskShortTTL(t *testing.T) {
	t.Parallel()
	svc := NewReceiptService([]byte("test-hmac-key"))

	receipt, _, err := svc.EvaluateAndIssue(ReceiptRequest{
		WorkspaceID:   "ws-001",
		WorkflowRunID: "wf-001",
		PlanID:        "plan-001",
		ToolKeys:      []string{"delete_account"},
		RiskLevel:     "CRITICAL",
		PolicyBundle:  "bundle-v1",
	})

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	ttl := receipt.ExpiresAt.Sub(receipt.IssuedAt)
	if ttl > 31*time.Second {
		t.Fatalf("CRITICAL risk TTL should be ~30s, got %v", ttl)
	}
}

func TestReceiptValidation(t *testing.T) {
	t.Parallel()
	svc := NewReceiptService([]byte("test-hmac-key"))

	receipt, _, _ := svc.EvaluateAndIssue(ReceiptRequest{
		WorkspaceID:   "ws-001",
		WorkflowRunID: "wf-001",
		PlanID:        "plan-001",
		ToolKeys:      []string{"send_email"},
		RiskLevel:     "LOW",
		PolicyBundle:  "bundle-v1",
	})

	// Valid receipt
	if err := svc.ValidateReceipt(receipt.ID, "ws-001", "send_email"); err != nil {
		t.Fatalf("valid receipt should pass: %v", err)
	}

	// Wrong workspace
	if err := svc.ValidateReceipt(receipt.ID, "ws-999", "send_email"); err != ErrReceiptMismatch {
		t.Fatalf("expected ErrReceiptMismatch, got %v", err)
	}

	// Wrong tool
	if err := svc.ValidateReceipt(receipt.ID, "ws-001", "delete_user"); err != ErrReceiptMismatch {
		t.Fatalf("expected ErrReceiptMismatch, got %v", err)
	}

	// Empty receipt
	if err := svc.ValidateReceipt("", "ws-001", "send_email"); err != ErrNoReceipt {
		t.Fatalf("expected ErrNoReceipt, got %v", err)
	}

	// Non-existent receipt
	if err := svc.ValidateReceipt("nonexistent", "ws-001", "send_email"); err != ErrNoReceipt {
		t.Fatalf("expected ErrNoReceipt, got %v", err)
	}
}

func TestReceiptConsumption(t *testing.T) {
	t.Parallel()
	svc := NewReceiptService([]byte("test-hmac-key"))

	receipt, _, _ := svc.EvaluateAndIssue(ReceiptRequest{
		WorkspaceID:   "ws-001",
		WorkflowRunID: "wf-001",
		PlanID:        "plan-001",
		ToolKeys:      []string{"send_email"},
		RiskLevel:     "LOW",
		PolicyBundle:  "bundle-v1",
	})

	// Consume
	if err := svc.ConsumeReceipt(receipt.ID); err != nil {
		t.Fatalf("first consume should succeed: %v", err)
	}

	// Double consume
	if err := svc.ConsumeReceipt(receipt.ID); err != ErrReceiptConsumed {
		t.Fatalf("expected ErrReceiptConsumed, got %v", err)
	}

	// Validate consumed receipt
	if err := svc.ValidateReceipt(receipt.ID, "ws-001", "send_email"); err != ErrReceiptConsumed {
		t.Fatalf("expected ErrReceiptConsumed, got %v", err)
	}
}

func TestKillSwitchBlocksReceipt(t *testing.T) {
	t.Parallel()
	svc := NewReceiptService([]byte("test-hmac-key"))

	svc.ActivateKillSwitch("ws-001")

	_, evals, err := svc.EvaluateAndIssue(ReceiptRequest{
		WorkspaceID:   "ws-001",
		WorkflowRunID: "wf-001",
		PlanID:        "plan-001",
		ToolKeys:      []string{"send_email"},
		RiskLevel:     "LOW",
		PolicyBundle:  "bundle-v1",
	})

	if err != ErrKillSwitchActive {
		t.Fatalf("expected ErrKillSwitchActive, got %v", err)
	}
	if len(evals) < 1 || evals[0].GateName != "kill_switch" {
		t.Fatal("kill_switch should be the first gate evaluated")
	}
	if evals[0].Decision != "deny" {
		t.Fatal("kill_switch gate should deny")
	}
}

func TestKillSwitchDeactivation(t *testing.T) {
	t.Parallel()
	svc := NewReceiptService([]byte("test-hmac-key"))

	svc.ActivateKillSwitch("ws-001")
	if !svc.IsKillSwitchActive("ws-001") {
		t.Fatal("kill switch should be active")
	}

	svc.DeactivateKillSwitch("ws-001")
	if svc.IsKillSwitchActive("ws-001") {
		t.Fatal("kill switch should be deactivated")
	}

	// Should now be able to issue receipt
	receipt, _, err := svc.EvaluateAndIssue(ReceiptRequest{
		WorkspaceID:   "ws-001",
		WorkflowRunID: "wf-001",
		PlanID:        "plan-001",
		ToolKeys:      []string{"send_email"},
		RiskLevel:     "LOW",
		PolicyBundle:  "bundle-v1",
	})
	if err != nil {
		t.Fatalf("should succeed after deactivation: %v", err)
	}
	if receipt == nil {
		t.Fatal("receipt should not be nil")
	}
}

func TestExecutionLedger(t *testing.T) {
	t.Parallel()
	ledger := NewExecutionLedger()

	err := ledger.Record(LedgerEntry{
		WorkspaceID:    "ws-001",
		ReceiptID:      "receipt-001",
		ToolKey:        "send_email",
		Phase:          "simulate",
		IdempotencyKey: "idem-001",
		PayloadHash:    "hash-001",
		ResultStatus:   "success",
		DurationMS:     42,
	})
	if err != nil {
		t.Fatalf("first record should succeed: %v", err)
	}

	// Idempotency conflict
	err = ledger.Record(LedgerEntry{
		WorkspaceID:    "ws-001",
		ReceiptID:      "receipt-001",
		ToolKey:        "send_email",
		Phase:          "simulate",
		IdempotencyKey: "idem-001",
		PayloadHash:    "hash-001",
		ResultStatus:   "success",
		DurationMS:     42,
	})
	if err == nil {
		t.Fatal("duplicate record should fail with idempotency conflict")
	}

	// Different phase is allowed
	err = ledger.Record(LedgerEntry{
		WorkspaceID:    "ws-001",
		ReceiptID:      "receipt-001",
		ToolKey:        "send_email",
		Phase:          "commit",
		IdempotencyKey: "idem-001",
		PayloadHash:    "hash-001",
		ResultStatus:   "success",
		DurationMS:     15,
	})
	if err != nil {
		t.Fatalf("different phase should succeed: %v", err)
	}

	if ledger.EntryCount() != 2 {
		t.Fatalf("expected 2 entries, got %d", ledger.EntryCount())
	}
}

func TestNoSideEffectsWithoutReceipt(t *testing.T) {
	t.Parallel()
	svc := NewReceiptService([]byte("test-hmac-key"))

	// Attempting to validate a nonexistent receipt must fail
	err := svc.ValidateReceipt("", "ws-001", "any_tool")
	if err == nil {
		t.Fatal("validation without receipt must fail")
	}
	if err != ErrNoReceipt {
		t.Fatalf("expected ErrNoReceipt, got %v", err)
	}
}

func TestReceiptAuditEntry(t *testing.T) {
	t.Parallel()
	svc := NewReceiptService([]byte("test-hmac-key"))

	receipt, _, _ := svc.EvaluateAndIssue(ReceiptRequest{
		WorkspaceID:   "ws-001",
		WorkflowRunID: "wf-001",
		PlanID:        "plan-001",
		ToolKeys:      []string{"send_email"},
		RiskLevel:     "LOW",
		PolicyBundle:  "bundle-v1",
	})

	entry := FormatReceiptAuditEntry(receipt, "issued")
	if entry["receipt_id"] != receipt.ID {
		t.Fatal("audit entry should contain receipt ID")
	}
	if entry["action"] != "issued" {
		t.Fatal("audit entry should contain action")
	}
}
