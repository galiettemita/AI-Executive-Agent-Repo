package brain

import (
	"context"
	"fmt"
	"strings"

	"github.com/brevio/brevio/internal/llm"
)

// selfConsistencyTemps are the temperatures cycled through across samples.
var selfConsistencyTemps = []float64{0.1, 0.4, 0.6, 0.3, 0.5}

// SelfConsistencyPlanner samples K plans and returns the majority-voted plan.
type SelfConsistencyPlanner struct {
	client llm.Client
	k      int
}

// NewSelfConsistencyPlanner creates a planner with k samples (minimum 3).
func NewSelfConsistencyPlanner(client llm.Client, k int) *SelfConsistencyPlanner {
	if k < 3 {
		k = 3
	}
	return &SelfConsistencyPlanner{client: client, k: k}
}

// SelectPlan samples k plans with varying temperatures and returns the majority plan.
func (p *SelfConsistencyPlanner) SelectPlan(ctx context.Context, input LLMPlannerInput) (*Plan, error) {
	type candidate struct {
		plan      *Plan
		signature string
	}

	candidates := make([]candidate, 0, p.k)
	for i := 0; i < p.k; i++ {
		sampleInput := input
		sampleInput.Temperature = selfConsistencyTemps[i%len(selfConsistencyTemps)]
		sampleInput.UseThinking = input.UseThinking && (i == 0)

		plan, _, err := callLLMPlanner(ctx, p.client, sampleInput)
		if err != nil {
			continue
		}
		candidates = append(candidates, candidate{plan: plan, signature: PlanSignature(plan)})
	}

	if len(candidates) == 0 {
		return nil, fmt.Errorf("self_consistency: all %d samples failed", p.k)
	}
	if len(candidates) == 1 {
		return candidates[0].plan, nil
	}

	votes := make(map[string]int)
	sigToPlan := make(map[string]*Plan)
	for _, c := range candidates {
		votes[c.signature]++
		sigToPlan[c.signature] = c.plan
	}
	bestSig, bestCount := "", 0
	for sig, count := range votes {
		if count > bestCount {
			bestCount = count
			bestSig = sig
		}
	}
	return sigToPlan[bestSig], nil
}

// PlanSignature produces a canonical fingerprint of a plan's tool+phase sequence.
func PlanSignature(plan *Plan) string {
	if plan == nil {
		return ""
	}
	parts := make([]string, len(plan.Steps))
	for i, s := range plan.Steps {
		parts[i] = s.Phase + ":" + s.ToolKey
	}
	return strings.Join(parts, "|")
}
