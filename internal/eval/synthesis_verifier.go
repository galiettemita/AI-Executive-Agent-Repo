package eval

import (
	"context"
	"fmt"
	"strings"
)

// SynthesisVerifier checks T2/T3 responses for factual self-consistency.
type SynthesisVerifier struct {
	llm           VerifierLLM
	passThreshold float64
	maxSamples    int
}

// VerifierLLM is the minimal LLM interface for self-consistency sampling.
type VerifierLLM interface {
	SampleMultiple(ctx context.Context, workspaceID, systemPrompt, userPrompt, tier string, n int) ([]string, error)
}

// VerificationResult is the outcome of a synthesis verification pass.
type VerificationResult struct {
	Passed           bool     `json:"passed"`
	ConsistencyScore float64  `json:"consistency_score"`
	FlaggedClaims    []string `json:"flagged_claims,omitempty"`
	Reasoning        string   `json:"reasoning,omitempty"`
}

// NewSynthesisVerifier creates a SynthesisVerifier.
func NewSynthesisVerifier(llm VerifierLLM, passThreshold float64) *SynthesisVerifier {
	if passThreshold <= 0 {
		passThreshold = 0.70
	}
	return &SynthesisVerifier{llm: llm, passThreshold: passThreshold, maxSamples: 2}
}

const verifierSystem = `You are a factual consistency evaluator.
You will be given multiple responses to the same query.
Your task is to identify factual claims that appear in some responses but not others,
or that contradict between responses.

Respond ONLY with a JSON object:
{
  "consistency_score": <float 0.0 to 1.0>,
  "flagged_claims": [<string>, ...],
  "reasoning": "<one sentence>"
}`

// Verify runs self-consistency check on a synthesized response.
func (sv *SynthesisVerifier) Verify(
	ctx context.Context,
	workspaceID, systemPrompt, userPrompt, primaryResponse, tier string,
) (*VerificationResult, error) {
	if tier != "t2" && tier != "t3" {
		return &VerificationResult{Passed: true, ConsistencyScore: 1.0}, nil
	}

	samples, err := sv.llm.SampleMultiple(ctx, workspaceID, systemPrompt, userPrompt, tier, sv.maxSamples)
	if err != nil {
		return &VerificationResult{Passed: true, ConsistencyScore: 1.0,
			Reasoning: fmt.Sprintf("verifier sample failed: %v", err)}, nil
	}

	var sb strings.Builder
	sb.WriteString("Response A (primary):\n" + primaryResponse + "\n\n")
	for i, s := range samples {
		sb.WriteString(fmt.Sprintf("Response %c (sample %d):\n%s\n\n", rune('B'+i), i+1, s))
	}

	raw, err := sv.llm.SampleMultiple(ctx, workspaceID, verifierSystem, sb.String(), "t1", 1)
	if err != nil || len(raw) == 0 {
		return &VerificationResult{Passed: true, ConsistencyScore: 1.0}, nil
	}

	result := ParseVerificationResult(raw[0])
	result.Passed = result.ConsistencyScore >= sv.passThreshold
	return result, nil
}

// ParseVerificationResult extracts fields from a JSON-like verifier response.
func ParseVerificationResult(raw string) *VerificationResult {
	result := &VerificationResult{ConsistencyScore: 1.0}

	raw = strings.TrimSpace(raw)
	if idx := strings.Index(raw, "{"); idx >= 0 {
		raw = raw[idx:]
	}
	if idx := strings.LastIndex(raw, "}"); idx >= 0 {
		raw = raw[:idx+1]
	}

	if i := strings.Index(raw, `"consistency_score":`); i >= 0 {
		rest := raw[i+len(`"consistency_score":`):]
		rest = strings.TrimSpace(rest)
		var score float64
		if _, err := fmt.Sscanf(rest, "%f", &score); err == nil {
			result.ConsistencyScore = score
		}
	}

	if i := strings.Index(raw, `"reasoning":`); i >= 0 {
		rest := raw[i+len(`"reasoning":`):]
		rest = strings.TrimSpace(rest)
		if len(rest) > 0 && rest[0] == '"' {
			end := strings.Index(rest[1:], `"`)
			if end >= 0 {
				result.Reasoning = rest[1 : end+1]
			}
		}
	}

	return result
}
