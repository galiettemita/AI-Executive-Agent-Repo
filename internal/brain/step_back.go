package brain

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/brevio/brevio/internal/llm"
)

const stepBackModel = "claude-haiku-4-5-20251001"
const stepBackMaxTokens = 256
const stepBackConfidenceThreshold = 0.75

// StepBackRequest is input to the Step-Back pre-pass.
type StepBackRequest struct {
	WorkspaceID      string
	RawMessage       string
	IntentConfidence float64
}

// StepBackResult is the output of the pre-pass.
type StepBackResult struct {
	AbstractGoal string  `json:"abstract_goal"`
	Confidence   float64 `json:"confidence"`
	Skipped      bool    `json:"skipped"`
	LatencyMs    int64   `json:"latency_ms"`
}

// StepBackService extracts the user's underlying abstract goal before literal
// intent decomposition.
type StepBackService struct {
	llmClient llm.Client
}

// NewStepBackService creates a StepBackService.
func NewStepBackService(client llm.Client) *StepBackService {
	return &StepBackService{llmClient: client}
}

const stepBackSystemPrompt = `You are an expert at understanding user intent.
Given a user message, output the user's UNDERLYING ABSTRACT GOAL in one concise sentence.
Focus on WHAT they ultimately want to achieve, not HOW they said it.
Respond ONLY with JSON: {"abstract_goal":"<one sentence>","confidence":<0.0-1.0>}
No preamble, no markdown.`

// Infer runs the Step-Back pre-pass. Skipped when confidence >= threshold.
func (s *StepBackService) Infer(ctx context.Context, req StepBackRequest) (*StepBackResult, error) {
	if s.llmClient == nil || req.IntentConfidence >= stepBackConfidenceThreshold {
		return &StepBackResult{Skipped: true}, nil
	}
	if strings.TrimSpace(req.RawMessage) == "" {
		return &StepBackResult{Skipped: true}, nil
	}

	start := time.Now()
	resp, _, err := s.llmClient.Generate(ctx, llm.GenerateRequest{
		Model:       stepBackModel,
		MaxTokens:   stepBackMaxTokens,
		Temperature: 0.1,
		System:      stepBackSystemPrompt,
		Messages:    []llm.ChatMsg{{Role: "user", Content: fmt.Sprintf("User message: %q", req.RawMessage)}},
	})
	latency := time.Since(start).Milliseconds()
	if err != nil {
		return &StepBackResult{Skipped: true, LatencyMs: latency}, nil
	}

	var result struct {
		AbstractGoal string  `json:"abstract_goal"`
		Confidence   float64 `json:"confidence"`
	}
	clean := strings.TrimSpace(resp.Content)
	if err := json.Unmarshal([]byte(clean), &result); err != nil {
		return &StepBackResult{Skipped: true, LatencyMs: latency}, nil
	}
	if strings.TrimSpace(result.AbstractGoal) == "" {
		return &StepBackResult{Skipped: true, LatencyMs: latency}, nil
	}
	return &StepBackResult{
		AbstractGoal: result.AbstractGoal,
		Confidence:   result.Confidence,
		Skipped:      false,
		LatencyMs:    latency,
	}, nil
}
