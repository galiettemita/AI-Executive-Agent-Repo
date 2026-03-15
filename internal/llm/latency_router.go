package llm

import "sync"

// LatencyBudget represents the remaining time budget for a request.
type LatencyBudget struct {
	RemainingMs   float64
	ElapsedMs     float64
	TotalBudgetMs float64
}

// LatencyAwareRouter selects models based on latency constraints.
type LatencyAwareRouter struct {
	mu           sync.Mutex
	fastModel    string
	balancedModel string
	bestModel    string
}

// NewLatencyAwareRouter creates a new LatencyAwareRouter with default model tiers.
func NewLatencyAwareRouter() *LatencyAwareRouter {
	return &LatencyAwareRouter{
		fastModel:     ModelAnthropicHaiku,
		balancedModel: ModelAnthropicSonnet,
		bestModel:     ModelAnthropicSonnet, // Opus reserved for orchestrator tier (Prompt 8)
	}
}

// SetModels allows overriding the model names for each tier.
func (lr *LatencyAwareRouter) SetModels(fast, balanced, best string) {
	lr.mu.Lock()
	defer lr.mu.Unlock()
	if fast != "" {
		lr.fastModel = fast
	}
	if balanced != "" {
		lr.balancedModel = balanced
	}
	if best != "" {
		lr.bestModel = best
	}
}

// SelectModel picks the appropriate model based on task complexity and latency budget.
// If remaining budget < 2000ms, use fastest model.
// If remaining budget < 4000ms, use balanced model.
// Otherwise, use the best model.
func (lr *LatencyAwareRouter) SelectModel(taskComplexity float64, budget LatencyBudget) string {
	lr.mu.Lock()
	defer lr.mu.Unlock()

	remaining := budget.RemainingMs
	if remaining <= 0 {
		remaining = budget.TotalBudgetMs - budget.ElapsedMs
	}

	if remaining < 2000 {
		return lr.fastModel
	}
	if remaining < 4000 {
		return lr.balancedModel
	}

	// With ample budget, consider task complexity.
	if taskComplexity < 0.3 {
		return lr.balancedModel
	}
	return lr.bestModel
}
