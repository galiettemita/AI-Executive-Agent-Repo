package policy_test

import (
	"context"
	"testing"

	"github.com/brevio/brevio/internal/policy"
)

func TestNewEvaluator_LoadsWithoutError(t *testing.T) {
	t.Parallel()
	_, err := policy.NewEvaluator()
	if err != nil {
		t.Fatalf("NewEvaluator: %v", err)
	}
}

func TestEvaluatePlan_AllowOnValidLowRisk(t *testing.T) {
	t.Parallel()
	ev, err := policy.NewEvaluator()
	if err != nil {
		t.Fatalf("NewEvaluator: %v", err)
	}
	d := ev.EvaluatePlan(context.Background(), policy.PlanAuthzInput{
		WorkspaceID: "ws-test",
		PlanID:      "plan-001",
		ToolKeys:    []string{"google_calendar.read_events"},
		RiskLevel:   "low",
		Autonomy:    "A4", // autonomy.rego requires A4
		BudgetCents: 1000,
		UsedCents:   0,
		UserTier:    "pro",
	})
	if !d.Allowed {
		t.Errorf("expected allow on low-risk read at A4, got deny: %s (policy: %s)", d.Reason, d.Policy)
	}
}

func TestEvaluatePlan_DenyOnCriticalRiskLowAutonomy(t *testing.T) {
	t.Parallel()
	ev, _ := policy.NewEvaluator()
	d := ev.EvaluatePlan(context.Background(), policy.PlanAuthzInput{
		WorkspaceID: "ws-test",
		RiskLevel:   "critical",
		Autonomy:    "A0", // not A4 → autonomy deny
		ToolKeys:    []string{"ibkr_trading.place_order"},
		BudgetCents: 5000,
		UsedCents:   0,
		UserTier:    "enterprise",
	})
	if d.Allowed {
		t.Error("expected deny for critical risk at A0 autonomy")
	}
	if d.Policy != "autonomy" {
		t.Errorf("expected autonomy policy, got %q", d.Policy)
	}
}

func TestEvaluatePlan_DenyOnBudgetExhausted(t *testing.T) {
	t.Parallel()
	ev, _ := policy.NewEvaluator()
	d := ev.EvaluatePlan(context.Background(), policy.PlanAuthzInput{
		WorkspaceID: "ws-test",
		RiskLevel:   "low",
		Autonomy:    "A4", // pass autonomy
		ToolKeys:    []string{"google_calendar.read_events"},
		BudgetCents: 100,
		UsedCents:   100, // exhausted
		UserTier:    "pro",
	})
	if d.Allowed {
		t.Error("expected deny when budget exhausted")
	}
	if d.Policy != "budget_enforcement" {
		t.Errorf("expected budget_enforcement policy, got %q", d.Policy)
	}
}

func TestEvaluatePlan_ConcurrentSafe(t *testing.T) {
	t.Parallel()
	ev, _ := policy.NewEvaluator()
	done := make(chan struct{}, 50)
	for i := 0; i < 50; i++ {
		go func() {
			ev.EvaluatePlan(context.Background(), policy.PlanAuthzInput{
				WorkspaceID: "ws-concurrent",
				RiskLevel:   "low",
				Autonomy:    "A4",
				ToolKeys:    []string{"google_calendar.read_events"},
				BudgetCents: 1000,
				UserTier:    "pro",
			})
			done <- struct{}{}
		}()
	}
	for i := 0; i < 50; i++ {
		<-done
	}
}
