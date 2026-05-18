package executor

import "testing"

func TestLatencyPreemptorSufficientBudget(t *testing.T) {
	t.Parallel()

	lp := NewLatencyPreemptor()
	decision := lp.ShouldProceed(10000, 2000, 1000)
	if !decision.ShouldProceed {
		t.Fatal("expected proceed with sufficient budget")
	}
	if decision.RemainingBudgetMs != 8000 {
		t.Fatalf("expected 8000ms remaining, got %f", decision.RemainingBudgetMs)
	}
}

func TestLatencyPreemptorBudgetExhausted(t *testing.T) {
	t.Parallel()

	lp := NewLatencyPreemptor()
	decision := lp.ShouldProceed(5000, 5000, 1000)
	if decision.ShouldProceed {
		t.Fatal("expected preemption when budget exhausted")
	}
	if decision.Reason != "budget exhausted" {
		t.Fatalf("expected 'budget exhausted', got %s", decision.Reason)
	}
}

func TestLatencyPreemptorEstimatedExceedsBudget(t *testing.T) {
	t.Parallel()

	lp := NewLatencyPreemptor()
	decision := lp.ShouldProceed(5000, 4000, 2000)
	if decision.ShouldProceed {
		t.Fatal("expected preemption when estimated exceeds remaining")
	}
}

func TestLatencyPreemptorTightMargin(t *testing.T) {
	t.Parallel()

	lp := NewLatencyPreemptor()
	// Budget=10000, elapsed=8500, remaining=1500, estimated=1100, margin=1000
	// 1100 > 1500-1000=500, so tight margin
	decision := lp.ShouldProceed(10000, 8500, 1100)
	if !decision.ShouldProceed {
		t.Fatal("expected proceed with tight margin")
	}
	if decision.Reason != "proceeding with tight budget margin" {
		t.Fatalf("expected tight margin reason, got %s", decision.Reason)
	}
}

func TestLatencyPreemptorZeroBudget(t *testing.T) {
	t.Parallel()

	lp := NewLatencyPreemptor()
	decision := lp.ShouldProceed(0, 0, 100)
	if decision.ShouldProceed {
		t.Fatal("expected preemption with zero budget")
	}
}
