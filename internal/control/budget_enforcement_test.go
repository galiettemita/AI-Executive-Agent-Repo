package control

import (
	"context"
	"errors"
	"testing"
)

func TestBudgetEnforcer_NoCap_Allows(t *testing.T) {
	t.Parallel()
	enforcer := NewBudgetEnforcer(nil)

	result := enforcer.Check(context.Background(), "ws-001", 100, 10.0)
	if !result.Allowed {
		t.Fatal("no cap should allow")
	}
	if result.ReasonCode != "NO_BUDGET_CAP" {
		t.Errorf("got reason=%q want=%q", result.ReasonCode, "NO_BUDGET_CAP")
	}
}

func TestBudgetEnforcer_ConsumeWithinBudget(t *testing.T) {
	t.Parallel()
	enforcer := NewBudgetEnforcer(nil)
	enforcer.SetBudget("ws-001", "pro", 1000, 50.0)

	err := enforcer.Consume(context.Background(), "ws-001", "receipt-001", 100, 5.0)
	if err != nil {
		t.Fatalf("expected success: %v", err)
	}

	if enforcer.IsExhausted("ws-001") {
		t.Fatal("should not be exhausted after partial use")
	}
}

func TestBudgetEnforcer_ConsumeExhaustsDenies(t *testing.T) {
	t.Parallel()
	enforcer := NewBudgetEnforcer(nil)
	enforcer.SetBudget("ws-001", "free", 100, 5.0)

	// Fully exhaust the budget.
	err := enforcer.Consume(context.Background(), "ws-001", "receipt-001", 100, 5.0)
	if err != nil {
		t.Fatalf("first consume: %v", err)
	}

	if !enforcer.IsExhausted("ws-001") {
		t.Fatal("should be exhausted after full consumption")
	}

	// Next consume must be denied.
	err = enforcer.Consume(context.Background(), "ws-001", "receipt-002", 1, 0.1)
	if err == nil {
		t.Fatal("expected budget exceeded error")
	}
	if !errors.Is(err, ErrBudgetExceeded) {
		t.Fatalf("expected ErrBudgetExceeded, got %v", err)
	}
}

func TestBudgetEnforcer_USDExhaustion(t *testing.T) {
	t.Parallel()
	enforcer := NewBudgetEnforcer(nil)
	enforcer.SetBudget("ws-001", "pro", 10000, 50.0) // High unit cap, low USD cap

	err := enforcer.Consume(context.Background(), "ws-001", "receipt-001", 10, 45.0)
	if err != nil {
		t.Fatalf("first consume: %v", err)
	}

	err = enforcer.Consume(context.Background(), "ws-001", "receipt-002", 10, 10.0)
	if err == nil {
		t.Fatal("expected USD budget exceeded error")
	}
	if !errors.Is(err, ErrBudgetExceeded) {
		t.Fatalf("expected ErrBudgetExceeded, got %v", err)
	}
}

func TestBudgetEnforcer_CheckWarningAt80Percent(t *testing.T) {
	t.Parallel()
	enforcer := NewBudgetEnforcer(nil)
	enforcer.SetBudget("ws-001", "pro", 100, 50.0)

	// Consume 78 units — check for 1 would be 79/100 = 79%, no warning.
	enforcer.Consume(context.Background(), "ws-001", "r1", 78, 0)
	result := enforcer.Check(context.Background(), "ws-001", 1, 0)
	if result.Warning {
		t.Fatal("79/100 should not trigger warning")
	}

	// Consume 1 more to 79 — check for 1 would be 80/100 = 80%, should warn.
	enforcer.Consume(context.Background(), "ws-001", "r2", 1, 0)
	result = enforcer.Check(context.Background(), "ws-001", 1, 0)
	if !result.Warning {
		t.Fatal("80/100 should trigger warning")
	}
	if !result.Allowed {
		t.Fatal("warning should still allow")
	}
}

func TestBudgetEnforcer_CheckDeniesExhausted(t *testing.T) {
	t.Parallel()
	enforcer := NewBudgetEnforcer(nil)
	enforcer.SetBudget("ws-001", "free", 10, 5.0)

	// Exhaust budget.
	enforcer.Consume(context.Background(), "ws-001", "r1", 10, 5.0)

	result := enforcer.Check(context.Background(), "ws-001", 1, 0.5)
	if result.Allowed {
		t.Fatal("exhausted budget check should deny")
	}
	if !result.Exhausted {
		t.Fatal("should report exhausted")
	}
	// Both units and USD are exhausted; the last check (USD) sets the reason.
	if result.ReasonCode != "BUDGET_UNITS_EXHAUSTED" && result.ReasonCode != "BUDGET_USD_EXHAUSTED" {
		t.Errorf("got unexpected reason=%q", result.ReasonCode)
	}
}

func TestBudgetEnforcer_NoCap_ConsumeAllowed(t *testing.T) {
	t.Parallel()
	enforcer := NewBudgetEnforcer(nil)

	// No budget set for ws-002 — should allow unlimited.
	err := enforcer.Consume(context.Background(), "ws-002", "r1", 9999, 9999.0)
	if err != nil {
		t.Fatalf("no cap should allow: %v", err)
	}
	if enforcer.IsExhausted("ws-002") {
		t.Fatal("no cap workspace should never be exhausted")
	}
}

func TestBudgetEnforcer_IntegrationWithGateDecision(t *testing.T) {
	t.Parallel()

	// This test demonstrates the full flow: budget check → gate decision → receipt
	// → budget consume → denial on exhaustion.
	enforcer := NewBudgetEnforcer(nil)
	enforcer.SetBudget("ws-001", "free", 5, 5.0)
	receiptSvc := NewReceiptService([]byte("test-key"))

	for i := 0; i < 5; i++ {
		// Check budget.
		check := enforcer.Check(context.Background(), "ws-001", 1, 1.0)
		if !check.Allowed {
			t.Fatalf("iteration %d: budget check should allow", i)
		}

		// Issue receipt.
		receipt, _, err := receiptSvc.EvaluateAndIssue(ReceiptRequest{
			WorkspaceID:   "ws-001",
			WorkflowRunID: "wf-001",
			PlanID:        "plan-001",
			ToolKeys:      []string{"email_send"},
			RiskLevel:     "LOW",
			PolicyBundle:  "bundle-v1",
		})
		if err != nil {
			t.Fatalf("iteration %d: receipt issue: %v", i, err)
		}

		// Consume budget.
		err = enforcer.Consume(context.Background(), "ws-001", receipt.ID, 1, 1.0)
		if err != nil {
			t.Fatalf("iteration %d: budget consume: %v", i, err)
		}
	}

	// 6th attempt should be denied by budget check.
	check := enforcer.Check(context.Background(), "ws-001", 1, 1.0)
	if check.Allowed {
		t.Fatal("6th check should be denied — budget exhausted")
	}
	if !check.Exhausted {
		t.Fatal("should report exhausted")
	}

	// 6th consume should also fail.
	err := enforcer.Consume(context.Background(), "ws-001", "receipt-6", 1, 1.0)
	if !errors.Is(err, ErrBudgetExceeded) {
		t.Fatalf("expected ErrBudgetExceeded, got %v", err)
	}
}
