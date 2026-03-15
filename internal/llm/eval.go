package llm

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
)

// EvalCase is a golden test case for the LLM intelligence pipeline.
type EvalCase struct {
	ID                string   // unique identifier, e.g. "tc_calendar_create_01"
	Input             string   // raw user message to run through the pipeline
	ExpectedIntent    string   // expected classified intent string
	ExpectedToolKeys  []string // expected tool_keys in the plan (order-insensitive subset)
	MinConfidence     float64  // minimum acceptable classification confidence (default 0.70)
	JudgeInstructions string   // optional extra context provided to the LLM judge
}

// EvalResult is the outcome of running one EvalCase through the pipeline.
type EvalResult struct {
	CaseID           string
	ClassifiedIntent string
	PlannedToolKeys  []string
	IntentMatch      bool
	ToolKeyMatch     bool    // true if all ExpectedToolKeys appear in PlannedToolKeys
	ConfidenceOK     bool
	JudgeScore       float64 // 0.0-1.0 scored by LLM judge; -1.0 if judge call failed
	JudgeRationale   string
	Pass             bool
	Err              error
}

// Evaluator runs EvalCases through IntelligenceService and scores results using
// an LLM judge at T0 tier (Haiku — fast and cheap).
type Evaluator struct {
	Intel         *IntelligenceService
	PassThreshold float64 // minimum JudgeScore to consider a case passing (default 0.70)
}

// NewEvaluator creates an Evaluator with the default 0.70 pass threshold.
func NewEvaluator(intel *IntelligenceService) *Evaluator {
	return &Evaluator{Intel: intel, PassThreshold: 0.70}
}

// RunBatch runs all cases sequentially and returns their results.
func (e *Evaluator) RunBatch(ctx context.Context, cases []EvalCase) []EvalResult {
	results := make([]EvalResult, len(cases))
	for i, ec := range cases {
		results[i] = e.runOne(ctx, ec)
	}
	return results
}

func (e *Evaluator) runOne(ctx context.Context, ec EvalCase) EvalResult {
	res := EvalResult{CaseID: ec.ID, JudgeScore: -1.0}

	minConf := ec.MinConfidence
	if minConf <= 0 {
		minConf = 0.70
	}

	// Step 1: classify intent
	classification, _, err := e.Intel.ClassifyIntent(ctx, ec.Input, "eval-workspace")
	if err != nil {
		res.Err = fmt.Errorf("classify: %w", err)
		return res
	}
	res.ClassifiedIntent = classification.Intent
	res.IntentMatch = strings.EqualFold(classification.Intent, ec.ExpectedIntent)
	res.ConfidenceOK = classification.Confidence >= minConf

	// Step 2: generate plan
	plan, _, err := e.Intel.GeneratePlan(
		ctx,
		classification.Intent,
		classification.Confidence,
		ec.Input, "", "",
	)
	if err != nil {
		res.Err = fmt.Errorf("plan: %w", err)
		return res
	}
	res.PlannedToolKeys = plan.Tools
	res.ToolKeyMatch = evalToolKeySubsetMatch(ec.ExpectedToolKeys, plan.Tools)

	// Step 3: score with LLM judge
	res = e.scoreWithJudge(ctx, ec, res)

	res.Pass = res.IntentMatch && res.ToolKeyMatch && res.ConfidenceOK &&
		(res.JudgeScore < 0 || res.JudgeScore >= e.PassThreshold)
	return res
}

// scoreWithJudge calls the T0 classifier to score the pipeline output as an LLM judge.
// Judge failure is non-fatal — res is returned unchanged if the call fails.
func (e *Evaluator) scoreWithJudge(ctx context.Context, ec EvalCase, actual EvalResult) EvalResult {
	if e.Intel == nil || e.Intel.classifier == nil {
		return actual
	}

	judgePrompt := fmt.Sprintf(
		"Rate the quality of this AI assistant response on a scale of 0.0 to 1.0.\n\n"+
			"User input: %q\n"+
			"Expected intent: %q\n"+
			"Classified intent: %q (correct: %v)\n"+
			"Expected tool keys: %v\n"+
			"Planned tool keys: %v (correct: %v)\n"+
			"%s\n\n"+
			"Respond ONLY with JSON: {\"score\": <0.0-1.0>, \"rationale\": \"<one sentence>\"}",
		ec.Input,
		ec.ExpectedIntent, actual.ClassifiedIntent, actual.IntentMatch,
		ec.ExpectedToolKeys, actual.PlannedToolKeys, actual.ToolKeyMatch,
		ec.JudgeInstructions,
	)

	req := GenerateRequest{
		Model:       ResolveTierModel("T0").PrimaryModel,
		MaxTokens:   256,
		Temperature: 0.0,
		Messages:    []ChatMsg{{Role: "user", Content: judgePrompt}},
		JSONSchema: map[string]any{
			"type":     "object",
			"required": []string{"score", "rationale"},
			"properties": map[string]any{
				"score":     map[string]any{"type": "number", "minimum": 0.0, "maximum": 1.0},
				"rationale": map[string]any{"type": "string"},
			},
			"additionalProperties": false,
		},
	}

	resp, _, err := e.Intel.classifier.Generate(ctx, req)
	if err != nil {
		return actual
	}

	var judgeOut struct {
		Score     float64 `json:"score"`
		Rationale string  `json:"rationale"`
	}
	if err := json.Unmarshal([]byte(extractJSON(resp.Content)), &judgeOut); err != nil {
		return actual
	}
	actual.JudgeScore = judgeOut.Score
	actual.JudgeRationale = judgeOut.Rationale
	return actual
}

// evalToolKeySubsetMatch returns true if all expected keys appear in actual.
// Order-insensitive. Empty expected list always returns true.
func evalToolKeySubsetMatch(expected, actual []string) bool {
	if len(expected) == 0 {
		return true
	}
	actualSet := make(map[string]bool, len(actual))
	for _, k := range actual {
		actualSet[k] = true
	}
	for _, k := range expected {
		if !actualSet[k] {
			return false
		}
	}
	return true
}
