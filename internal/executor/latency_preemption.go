package executor

import (
	"fmt"
	"sync"
)

// PreemptionDecision describes whether to proceed with the next step.
type PreemptionDecision struct {
	ShouldProceed      bool
	Reason             string
	RemainingBudgetMs  float64
}

// LatencyPreemptor decides whether to proceed based on latency budget.
type LatencyPreemptor struct {
	mu sync.Mutex
}

// NewLatencyPreemptor creates a new LatencyPreemptor.
func NewLatencyPreemptor() *LatencyPreemptor {
	return &LatencyPreemptor{}
}

// ShouldProceed determines if there is enough latency budget remaining
// to execute the next step.
func (lp *LatencyPreemptor) ShouldProceed(budgetMs, elapsedMs, estimatedNextStepMs float64) PreemptionDecision {
	lp.mu.Lock()
	defer lp.mu.Unlock()

	remaining := budgetMs - elapsedMs
	if remaining <= 0 {
		return PreemptionDecision{
			ShouldProceed:     false,
			Reason:            "budget exhausted",
			RemainingBudgetMs: 0,
		}
	}

	if estimatedNextStepMs > remaining {
		return PreemptionDecision{
			ShouldProceed:     false,
			Reason:            fmt.Sprintf("estimated step %.0fms exceeds remaining budget %.0fms", estimatedNextStepMs, remaining),
			RemainingBudgetMs: remaining,
		}
	}

	// Allow a 10% safety margin.
	safetyMargin := budgetMs * 0.10
	if estimatedNextStepMs > remaining-safetyMargin {
		return PreemptionDecision{
			ShouldProceed:     true,
			Reason:            "proceeding with tight budget margin",
			RemainingBudgetMs: remaining,
		}
	}

	return PreemptionDecision{
		ShouldProceed:     true,
		Reason:            "sufficient budget remaining",
		RemainingBudgetMs: remaining,
	}
}
