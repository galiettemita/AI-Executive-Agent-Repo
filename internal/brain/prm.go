package brain

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/brevio/brevio/internal/llm"
)

const prmModel = "claude-haiku-4-5-20251001"
const prmMaxTokens = 256

// StepReward is the per-step quality score from the PRM.
type StepReward struct {
	StepIndex   int     `json:"step_index"`
	ToolKey     string  `json:"tool_key"`
	Score       float64 `json:"score"`
	IsPlausible bool    `json:"is_plausible"`
	Reasoning   string  `json:"reasoning"`
	LatencyMs   int64   `json:"latency_ms"`
}

// PRMConfig configures the Process Reward Model.
type PRMConfig struct {
	LLMClient    llm.Client
	MinStepScore float64
	Enabled      bool
}

// ProcessRewardModel scores each plan step after execution.
type ProcessRewardModel struct {
	cfg PRMConfig
}

// NewProcessRewardModel creates a PRM.
func NewProcessRewardModel(cfg PRMConfig) *ProcessRewardModel {
	if cfg.MinStepScore <= 0 {
		cfg.MinStepScore = 2.0
	}
	return &ProcessRewardModel{cfg: cfg}
}

const prmSystemPrompt = `You are a step-level quality reviewer for an AI task executor.
Rate the most recent step 1-5:
  5=Perfect, 4=Good, 3=Acceptable, 2=Off-track, 1=Wrong/stop.
Respond ONLY with JSON: {"score":<1-5>,"is_plausible":<bool>,"reasoning":"<one sentence>"}
No preamble.`

// ScoreStep evaluates a single step result.
// Returns (reward, shouldContinue, error). Fail-open on all errors.
func (p *ProcessRewardModel) ScoreStep(ctx context.Context, intent string, completedSteps []PlanStep, stepResults []StepResult, latest StepResult) (*StepReward, bool, error) {
	if !p.cfg.Enabled || p.cfg.LLMClient == nil {
		return &StepReward{StepIndex: latest.StepIndex, ToolKey: latest.ToolKey, Score: 3, IsPlausible: true, Reasoning: "PRM disabled"}, true, nil
	}
	start := time.Now()
	resp, _, err := p.cfg.LLMClient.Generate(ctx, llm.GenerateRequest{
		Model:       prmModel,
		MaxTokens:   prmMaxTokens,
		Temperature: 0.0,
		System:      prmSystemPrompt,
		Messages:    []llm.ChatMsg{{Role: "user", Content: p.buildPrompt(intent, completedSteps, stepResults, latest)}},
		JSONSchema:  prmSchema(),
	})
	latency := time.Since(start).Milliseconds()
	if err != nil {
		return &StepReward{StepIndex: latest.StepIndex, ToolKey: latest.ToolKey, Score: 3, IsPlausible: true, Reasoning: "PRM unavailable", LatencyMs: latency}, true, nil
	}
	var raw struct {
		Score       float64 `json:"score"`
		IsPlausible bool    `json:"is_plausible"`
		Reasoning   string  `json:"reasoning"`
	}
	if err := json.Unmarshal([]byte(strings.TrimSpace(resp.Content)), &raw); err != nil {
		return &StepReward{StepIndex: latest.StepIndex, ToolKey: latest.ToolKey, Score: 3, IsPlausible: true, Reasoning: "PRM parse error", LatencyMs: latency}, true, nil
	}
	if raw.Score < 1 {
		raw.Score = 1
	}
	if raw.Score > 5 {
		raw.Score = 5
	}
	reward := &StepReward{StepIndex: latest.StepIndex, ToolKey: latest.ToolKey, Score: raw.Score, IsPlausible: raw.IsPlausible, Reasoning: raw.Reasoning, LatencyMs: latency}
	return reward, raw.IsPlausible && raw.Score >= p.cfg.MinStepScore, nil
}

func (p *ProcessRewardModel) buildPrompt(intent string, steps []PlanStep, results []StepResult, latest StepResult) string {
	var b strings.Builder
	b.WriteString(fmt.Sprintf("Intent: %q\n\nPrevious steps:\n", intent))
	for i, s := range steps {
		status := "SUCCESS"
		if i < len(results) && !results[i].Success {
			status = "FAILED: " + results[i].Error
		}
		b.WriteString(fmt.Sprintf("  [%d] %s → %s\n", i, s.ToolKey, status))
	}
	b.WriteString(fmt.Sprintf("\nLatest step:\n  Tool: %s  Success: %v\n", latest.ToolKey, latest.Success))
	if latest.Error != "" {
		b.WriteString(fmt.Sprintf("  Error: %s\n", latest.Error))
	}
	return b.String()
}

func prmSchema() map[string]any {
	return map[string]any{
		"type":     "object",
		"required": []string{"score", "is_plausible", "reasoning"},
		"properties": map[string]any{
			"score":        map[string]any{"type": "number", "minimum": 1, "maximum": 5},
			"is_plausible": map[string]any{"type": "boolean"},
			"reasoning":    map[string]any{"type": "string"},
		},
		"additionalProperties": false,
	}
}
