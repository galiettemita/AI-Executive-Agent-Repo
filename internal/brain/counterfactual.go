package brain

import (
	"fmt"
	"sync"
)

// CounterfactualAnalysis captures the comparison between an original plan and
// its alternatives.
type CounterfactualAnalysis struct {
	OriginalPlanScore    float64   `json:"original_plan_score"`
	AlternativeScores    []float64 `json:"alternative_scores"`
	BestAlternativeIndex int       `json:"best_alternative_index"`
	ImprovementPotential float64   `json:"improvement_potential"` // best alt score - original score
}

// CounterfactualService scores alternative plans against the original.
type CounterfactualService struct {
	mu sync.Mutex
}

// NewCounterfactualService creates a new counterfactual scoring service.
func NewCounterfactualService() *CounterfactualService {
	return &CounterfactualService{}
}

// scorePlan computes a heuristic score for a plan based on step count, risk,
// and estimated tokens.
func scorePlan(plan Plan) float64 {
	if len(plan.Steps) == 0 {
		return 0
	}

	// Base score from step efficiency (fewer steps = better, up to a point).
	stepScore := 1.0
	if len(plan.Steps) > 5 {
		stepScore = 5.0 / float64(len(plan.Steps))
	}

	// Risk penalty.
	riskMultiplier := 1.0
	switch plan.RiskLevel {
	case "critical":
		riskMultiplier = 0.6
	case "elevated":
		riskMultiplier = 0.8
	case "low":
		riskMultiplier = 1.0
	}

	// Token efficiency (lower token usage is better).
	tokenScore := 1.0
	if plan.EstimatedTokens > 2000 {
		tokenScore = 2000.0 / float64(plan.EstimatedTokens)
	}

	// Phase coverage bonus: plans with all three phases score higher.
	phases := map[string]bool{}
	for _, step := range plan.Steps {
		phases[step.Phase] = true
	}
	phaseBonus := float64(len(phases)) * 0.1

	score := (stepScore*0.4 + tokenScore*0.3 + phaseBonus) * riskMultiplier
	if score > 1.0 {
		score = 1.0
	}
	return score
}

// ScoreAlternatives evaluates the original plan against a list of alternative
// plans and returns a counterfactual analysis.
func (s *CounterfactualService) ScoreAlternatives(originalPlan Plan, alternatives []Plan) (*CounterfactualAnalysis, error) {
	if len(alternatives) == 0 {
		return nil, fmt.Errorf("at least one alternative plan is required")
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	originalScore := scorePlan(originalPlan)

	altScores := make([]float64, len(alternatives))
	bestIdx := 0
	bestScore := -1.0

	for i, alt := range alternatives {
		altScores[i] = scorePlan(alt)
		if altScores[i] > bestScore {
			bestScore = altScores[i]
			bestIdx = i
		}
	}

	improvement := bestScore - originalScore
	if improvement < 0 {
		improvement = 0
	}

	return &CounterfactualAnalysis{
		OriginalPlanScore:    originalScore,
		AlternativeScores:    altScores,
		BestAlternativeIndex: bestIdx,
		ImprovementPotential: improvement,
	}, nil
}

// ShouldHaveChosen returns true if the best alternative plan scores meaningfully
// better than the original (improvement > 0.1).
func (s *CounterfactualService) ShouldHaveChosen(analysis *CounterfactualAnalysis) bool {
	if analysis == nil {
		return false
	}
	return analysis.ImprovementPotential > 0.1
}
