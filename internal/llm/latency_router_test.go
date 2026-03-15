package llm

import "testing"

func TestLatencyRouterFastModel(t *testing.T) {
	t.Parallel()

	lr := NewLatencyAwareRouter()
	model := lr.SelectModel(0.8, LatencyBudget{RemainingMs: 1500, TotalBudgetMs: 10000})
	if model != ModelAnthropicHaiku {
		t.Fatalf("expected fast model for <2000ms, got %s", model)
	}
}

func TestLatencyRouterBalancedModel(t *testing.T) {
	t.Parallel()

	lr := NewLatencyAwareRouter()
	model := lr.SelectModel(0.8, LatencyBudget{RemainingMs: 3000, TotalBudgetMs: 10000})
	if model != ModelAnthropicSonnet {
		t.Fatalf("expected balanced model for <4000ms, got %s", model)
	}
}

func TestLatencyRouterBestModel(t *testing.T) {
	t.Parallel()

	lr := NewLatencyAwareRouter()
	model := lr.SelectModel(0.8, LatencyBudget{RemainingMs: 8000, TotalBudgetMs: 10000})
	if model != ModelAnthropicSonnet {
		t.Fatalf("expected best model for ample budget + high complexity, got %s", model)
	}
}

func TestLatencyRouterLowComplexityUsesBalanced(t *testing.T) {
	t.Parallel()

	lr := NewLatencyAwareRouter()
	model := lr.SelectModel(0.2, LatencyBudget{RemainingMs: 8000, TotalBudgetMs: 10000})
	if model != ModelAnthropicSonnet {
		t.Fatalf("expected balanced model for low complexity, got %s", model)
	}
}

func TestLatencyRouterCustomModels(t *testing.T) {
	t.Parallel()

	lr := NewLatencyAwareRouter()
	lr.SetModels("claude-haiku", "claude-sonnet", "claude-opus")

	model := lr.SelectModel(0.9, LatencyBudget{RemainingMs: 500, TotalBudgetMs: 10000})
	if model != "claude-haiku" {
		t.Fatalf("expected claude-haiku, got %s", model)
	}

	model = lr.SelectModel(0.9, LatencyBudget{RemainingMs: 9000, TotalBudgetMs: 10000})
	if model != "claude-opus" {
		t.Fatalf("expected claude-opus, got %s", model)
	}
}

func TestLatencyRouterComputedRemaining(t *testing.T) {
	t.Parallel()

	lr := NewLatencyAwareRouter()
	// RemainingMs = 0 means compute from total - elapsed
	model := lr.SelectModel(0.8, LatencyBudget{RemainingMs: 0, ElapsedMs: 9000, TotalBudgetMs: 10000})
	if model != ModelAnthropicHaiku {
		t.Fatalf("expected fast model when computed remaining < 2000, got %s", model)
	}
}
