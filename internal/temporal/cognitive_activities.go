package temporal

import (
	"context"
	"fmt"
	"strings"
)

// DualProcessRoutingInput is the input to DualProcessRoutingActivity.
type DualProcessRoutingInput struct {
	WorkspaceID    string  `json:"workspace_id"`
	MessageContent string  `json:"message_content"`
	IntentKey      string  `json:"intent_key"`
	Confidence     float64 `json:"confidence"`
}

// DualProcessRoutingResult indicates which reasoning path to use.
type DualProcessRoutingResult struct {
	UseSystem1 bool    `json:"use_system1"` // true = fast path (T0/T1)
	UseSystem2 bool    `json:"use_system2"` // true = full reasoning (T2/T3)
	Confidence float64 `json:"confidence"`
	Reasoning  string  `json:"reasoning"`
}

// ClarificationCheckInput is the input to ClarificationCheckActivity.
type ClarificationCheckInput struct {
	WorkspaceID    string   `json:"workspace_id"`
	MessageContent string   `json:"message_content"`
	IntentKey      string   `json:"intent_key"`
	PlanID         string   `json:"plan_id"`
	ToolKeys       []string `json:"tool_keys"`
	Confidence     float64  `json:"confidence"`
}

// ClarificationCheckResult indicates whether clarification is needed.
type ClarificationCheckResult struct {
	NeedsClarification bool   `json:"needs_clarification"`
	Question           string `json:"question,omitempty"`
}

// ResponseDriftCheckInput is the input to ResponseDriftCheckActivity.
type ResponseDriftCheckInput struct {
	WorkspaceID    string `json:"workspace_id"`
	OriginalIntent string `json:"original_intent"`
	Response       string `json:"response"`
	IntentKey      string `json:"intent_key"`
}

// ResponseDriftCheckResult is the output of ResponseDriftCheckActivity.
type ResponseDriftCheckResult struct {
	DriftDetected bool    `json:"drift_detected"`
	DriftScore    float64 `json:"drift_score"`
	Reason        string  `json:"reason,omitempty"`
}

// DualProcessRoutingActivity routes the request to System 1 (fast) or System 2 (deliberate).
// System 1: high confidence (>0.85), simple intents, low risk
// System 2: lower confidence, complex planning, write operations, high risk
func (a *Activities) DualProcessRoutingActivity(ctx context.Context, input DualProcessRoutingInput) (*DualProcessRoutingResult, error) {
	writeOp := isWriteToolKey(input.IntentKey)
	useSystem2 := input.Confidence < 0.85 || writeOp ||
		strings.Contains(input.IntentKey, "trading") ||
		strings.Contains(input.IntentKey, "transfer")

	return &DualProcessRoutingResult{
		UseSystem1: !useSystem2,
		UseSystem2: useSystem2,
		Confidence: input.Confidence,
		Reasoning:  fmt.Sprintf("confidence=%.2f write_op=%v", input.Confidence, writeOp),
	}, nil
}

// ClarificationCheckActivity determines whether clarification is needed before executing.
func (a *Activities) ClarificationCheckActivity(ctx context.Context, input ClarificationCheckInput) (*ClarificationCheckResult, error) {
	// Low confidence and write operation: always ask for clarification.
	if input.Confidence < 0.6 && len(input.ToolKeys) > 0 {
		for _, tk := range input.ToolKeys {
			if isWriteToolKey(tk) {
				return &ClarificationCheckResult{
					NeedsClarification: true,
					Question:           fmt.Sprintf("Just to confirm — you want me to %s?", intentToHuman(input.IntentKey)),
				}, nil
			}
		}
	}
	return &ClarificationCheckResult{NeedsClarification: false}, nil
}

// ResponseDriftCheckActivity validates that the synthesized response matches the original intent.
func (a *Activities) ResponseDriftCheckActivity(ctx context.Context, input ResponseDriftCheckInput) (*ResponseDriftCheckResult, error) {
	if input.Response == "" {
		return &ResponseDriftCheckResult{DriftDetected: false}, nil
	}

	// Keyword drift check: response should contain intent-relevant terms.
	intentWords := strings.Fields(strings.ReplaceAll(input.OriginalIntent, "_", " "))
	lowerResp := strings.ToLower(input.Response)
	matches := 0
	for _, w := range intentWords {
		if strings.Contains(lowerResp, strings.ToLower(w)) {
			matches++
		}
	}
	driftScore := 0.0
	if len(intentWords) > 0 {
		driftScore = 1.0 - float64(matches)/float64(len(intentWords))
	}

	return &ResponseDriftCheckResult{
		DriftDetected: driftScore > 0.8,
		DriftScore:    driftScore,
	}, nil
}

func intentToHuman(intentKey string) string {
	return strings.ReplaceAll(strings.ReplaceAll(intentKey, "_", " "), ".", " → ")
}

func isSelfModifyingOp(toolKey string) bool {
	lower := strings.ToLower(toolKey)
	return strings.Contains(lower, "agent_config") ||
		strings.Contains(lower, "self_modify") ||
		strings.Contains(lower, "update_policy")
}

func isWriteToolKey(key string) bool {
	lower := strings.ToLower(key)
	for _, verb := range []string{"create", "send", "delete", "update", "move",
		"cancel", "pay", "book", "order", "post", "set", "write"} {
		if strings.Contains(lower, verb) {
			return true
		}
	}
	return false
}
