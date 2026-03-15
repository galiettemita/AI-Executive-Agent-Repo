package brain

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/brevio/brevio/internal/llm"
)

const judgeModel = "claude-haiku-4-5-20251001"
const judgeMaxTokens = 512
const judgePassThreshold = 0.75

// SemanticCriticRequest is input to the LLM judge.
type SemanticCriticRequest struct {
	WorkspaceID    string
	OriginalIntent string
	StepBackGoal   string
	Steps          []PlanStep
	Results        []StepResult
	Duration       time.Duration
}

// SemanticCriticScore is the judge output.
type SemanticCriticScore struct {
	IntentSatisfied bool     `json:"intent_satisfied"`
	QualityScore    float64  `json:"quality_score"`
	Completeness    float64  `json:"completeness"`
	Accuracy        float64  `json:"accuracy"`
	Issues          []string `json:"issues,omitempty"`
	ShouldRetry     bool     `json:"should_retry"`
	RetryGuidance   string   `json:"retry_guidance,omitempty"`
	Passed          bool     `json:"passed"`
}

// SemanticCriticService evaluates execution quality using an LLM judge.
type SemanticCriticService struct {
	llmClient     llm.Client
	passThreshold float64
	fallback      *HeuristicCriticService
}

// NewSemanticCriticService creates an LLM-as-judge critic.
func NewSemanticCriticService(client llm.Client, passThreshold float64) *SemanticCriticService {
	if passThreshold <= 0 {
		passThreshold = judgePassThreshold
	}
	return &SemanticCriticService{
		llmClient:     client,
		passThreshold: passThreshold,
		fallback:      NewHeuristicCriticService(),
	}
}

const judgeSystemPrompt = `You are an impartial quality evaluator for an AI executive assistant.
Given a user's intent and the plan execution results, evaluate whether the execution
ACTUALLY satisfied what the user wanted.
Respond ONLY with JSON. No preamble.`

// Evaluate runs LLM-as-judge evaluation. Falls back to heuristic on failure.
func (s *SemanticCriticService) Evaluate(ctx context.Context, req SemanticCriticRequest) (*SemanticCriticScore, error) {
	if s.llmClient == nil {
		return s.heuristicFallback(req)
	}
	resp, _, err := s.llmClient.Generate(ctx, llm.GenerateRequest{
		Model:       judgeModel,
		MaxTokens:   judgeMaxTokens,
		Temperature: 0.0,
		System:      judgeSystemPrompt,
		Messages:    []llm.ChatMsg{{Role: "user", Content: s.buildPrompt(req)}},
		JSONSchema:  judgeSchema(),
	})
	if err != nil {
		return s.heuristicFallback(req)
	}
	var score SemanticCriticScore
	if err := json.Unmarshal([]byte(strings.TrimSpace(resp.Content)), &score); err != nil {
		return s.heuristicFallback(req)
	}
	score.Passed = score.QualityScore >= s.passThreshold
	return &score, nil
}

func (s *SemanticCriticService) buildPrompt(req SemanticCriticRequest) string {
	var b strings.Builder
	b.WriteString(fmt.Sprintf("User intent: %q\n", req.OriginalIntent))
	if req.StepBackGoal != "" {
		b.WriteString(fmt.Sprintf("Underlying goal: %q\n", req.StepBackGoal))
	}
	b.WriteString(fmt.Sprintf("Duration: %dms\n\nExecution:\n", req.Duration.Milliseconds()))
	for i, step := range req.Steps {
		status, output := "no_result", ""
		if i < len(req.Results) {
			r := req.Results[i]
			if r.Success {
				status = "SUCCESS"
				if len(r.Output) > 0 {
					enc, _ := json.Marshal(r.Output)
					if len(enc) > 200 {
						enc = append(enc[:200], []byte("...")...)
					}
					output = string(enc)
				}
			} else {
				status = "FAILED: " + r.Error
			}
		}
		b.WriteString(fmt.Sprintf("  [%d] %s (%s) → %s %s\n", i, step.ToolKey, step.Phase, status, output))
	}
	return b.String()
}

func (s *SemanticCriticService) heuristicFallback(req SemanticCriticRequest) (*SemanticCriticScore, error) {
	trace := ExecutionTrace{
		WorkspaceID: req.WorkspaceID,
		Intent:      req.OriginalIntent,
		PlanSteps:   len(req.Steps),
		Duration:    req.Duration,
	}
	for _, r := range req.Results {
		if r.Success {
			trace.Succeeded++
		} else {
			trace.Failed++
		}
		trace.ToolsUsed = append(trace.ToolsUsed, r.ToolKey)
	}
	out, err := s.fallback.Critique(trace)
	if err != nil {
		return &SemanticCriticScore{Passed: false, Issues: []string{"fallback failed"}}, nil
	}
	return &SemanticCriticScore{
		IntentSatisfied: out.Passed,
		QualityScore:    out.OverallScore,
		Completeness:    out.DimensionScores["completeness"],
		Accuracy:        out.DimensionScores["reliability"],
		Issues:          out.FailureModes,
		ShouldRetry:     !out.Passed,
		RetryGuidance:   out.ImprovementDirective,
		Passed:          out.Passed,
	}, nil
}

func judgeSchema() map[string]any {
	return map[string]any{
		"type":     "object",
		"required": []string{"intent_satisfied", "quality_score", "completeness", "accuracy", "should_retry"},
		"properties": map[string]any{
			"intent_satisfied": map[string]any{"type": "boolean"},
			"quality_score":    map[string]any{"type": "number", "minimum": 0, "maximum": 1},
			"completeness":     map[string]any{"type": "number", "minimum": 0, "maximum": 1},
			"accuracy":         map[string]any{"type": "number", "minimum": 0, "maximum": 1},
			"issues":           map[string]any{"type": "array", "items": map[string]any{"type": "string"}},
			"should_retry":     map[string]any{"type": "boolean"},
			"retry_guidance":   map[string]any{"type": "string"},
		},
		"additionalProperties": false,
	}
}
