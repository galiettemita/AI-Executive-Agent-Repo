package brain

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/brevio/brevio/internal/llm"
)

const (
	ormModel         = "claude-haiku-4-5-20251001"
	ormPassThreshold = 2.5
)

// OutcomeScore holds the full scored evaluation of a completed agent trajectory.
type OutcomeScore struct {
	OverallQuality  float64  `json:"overall_quality"`  // 1–5
	IntentSatisfied bool     `json:"intent_satisfied"`
	Completeness    float64  `json:"completeness"`     // 0–1
	Accuracy        float64  `json:"accuracy"`         // 0–1
	SideEffects     []string `json:"side_effects"`
	ImprovementHints string  `json:"improvement_hints"`
	LatencyMs       int64    `json:"latency_ms"`
}

// OutcomeRewardModel evaluates the final quality of a completed agent trajectory.
type OutcomeRewardModel struct {
	llmClient llm.Client
	repo      CriticTraceRepository
}

// NewOutcomeRewardModel creates an ORM. Either dependency may be nil (degraded mode).
func NewOutcomeRewardModel(llmClient llm.Client, repo CriticTraceRepository) *OutcomeRewardModel {
	return &OutcomeRewardModel{llmClient: llmClient, repo: repo}
}

// ScoreFinalOutcome evaluates the full trajectory and returns an OutcomeScore.
// Stores result in critic trace repository for replay and trend analysis.
func (o *OutcomeRewardModel) ScoreFinalOutcome(
	ctx context.Context,
	workspaceID string,
	intent string,
	steps []PlanStep,
	results []StepResult,
	finalResp string,
) (*OutcomeScore, error) {
	if o.llmClient == nil {
		return nil, fmt.Errorf("orm: llm client not configured")
	}

	start := time.Now()

	stepsJSON, _ := json.Marshal(steps)
	resultsJSON, _ := json.Marshal(results)

	systemPrompt := `You are an outcome evaluation judge for an AI agent system.
You evaluate whether an agent fully satisfied the user's original intent given the steps it took and the final response.
Respond ONLY with a valid JSON object — no preamble, no markdown fences.`

	userPrompt := fmt.Sprintf(`Evaluate this agent trajectory.

Original intent: %s
Steps executed: %s
Step results: %s
Final response: %s

Return JSON with these exact fields:
{
  "overall_quality": <float 1-5>,
  "intent_satisfied": <bool>,
  "completeness": <float 0-1>,
  "accuracy": <float 0-1>,
  "side_effects": [<string>, ...],
  "improvement_hints": "<string>"
}`, intent, string(stepsJSON), string(resultsJSON), finalResp)

	resp, _, err := o.llmClient.Generate(ctx, llm.GenerateRequest{
		Model:     ormModel,
		MaxTokens: 512,
		System:    systemPrompt,
		Messages:  []llm.ChatMsg{{Role: "user", Content: userPrompt}},
	})
	if err != nil {
		return nil, fmt.Errorf("orm llm call failed: %w", err)
	}

	var score OutcomeScore
	if err := json.Unmarshal([]byte(resp.Content), &score); err != nil {
		return nil, fmt.Errorf("orm: failed to parse score: %w, raw: %s", err, resp.Content)
	}
	if score.OverallQuality < 1 || score.OverallQuality > 5 {
		return nil, fmt.Errorf("orm: invalid overall_quality: %f", score.OverallQuality)
	}
	score.LatencyMs = time.Since(start).Milliseconds()

	// Persist to critic trace repository (non-fatal).
	if o.repo != nil {
		_ = o.repo.StoreORMResult(ctx, workspaceID, intent, &score)
	}
	return &score, nil
}

// Passes returns true if the ORM score meets the pass threshold.
func (o *OutcomeRewardModel) Passes(score *OutcomeScore) bool {
	return score != nil && score.OverallQuality >= ormPassThreshold
}
