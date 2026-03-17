package guardrails

import (
	"context"
	"fmt"
	"regexp"
	"strings"
	"sync"
)

// GuardCheckpoint identifies where in the inference pipeline a check occurs.
type GuardCheckpoint string

const (
	PreInference  GuardCheckpoint = "pre_inference"
	PostInference GuardCheckpoint = "post_inference"
	PreToolCall   GuardCheckpoint = "pre_tool_call"
	PostToolCall  GuardCheckpoint = "post_tool_call"
)

// GuardInput carries context into a guard check.
type GuardInput struct {
	Text          string         `json:"text"`
	ToolKey       string         `json:"tool_key"`
	ToolArgs      map[string]any `json:"tool_args"`
	ModelResponse string         `json:"model_response"`
	WorkspaceID   string         `json:"workspace_id"`
	TrustSource   string         `json:"trust_source,omitempty"`
	AutonomyTier  string         `json:"autonomy_tier,omitempty"`
	ToolOutput    string         `json:"tool_output,omitempty"`
}

// GuardViolation describes a single rule violation.
type GuardViolation struct {
	Rule        string `json:"rule"`
	Severity    string `json:"severity"` // block | warn | audit
	Description string `json:"description"`
	Evidence    string `json:"evidence"`
}

// GuardResult is the outcome of a checkpoint evaluation.
type GuardResult struct {
	Allowed    bool            `json:"allowed"`
	Checkpoint GuardCheckpoint `json:"checkpoint"`
	Violations []GuardViolation `json:"violations"`
	Sanitized  string          `json:"sanitized"`
}

// GuardRule is a named, pluggable check function.
type GuardRule struct {
	Name  string
	Check func(input *GuardInput) *GuardViolation
}

// InferenceGuard holds registered rules per checkpoint and evaluates inputs.
type InferenceGuard struct {
	mu    sync.RWMutex
	rules map[GuardCheckpoint][]GuardRule
}

// NewInferenceGuard creates an InferenceGuard pre-loaded with the default
// built-in rules.
func NewInferenceGuard() *InferenceGuard {
	g := &InferenceGuard{
		rules: map[GuardCheckpoint][]GuardRule{
			PreInference:  {},
			PostInference: {},
			PreToolCall:   {},
			PostToolCall:  {},
		},
	}
	g.registerDefaults()
	return g
}

// RegisterRule adds a custom rule at the given checkpoint.
func (g *InferenceGuard) RegisterRule(checkpoint GuardCheckpoint, rule GuardRule) {
	g.mu.Lock()
	defer g.mu.Unlock()
	g.rules[checkpoint] = append(g.rules[checkpoint], rule)
}

// CheckInput evaluates all rules for the given checkpoint and returns the
// aggregate result.
func (g *InferenceGuard) CheckInput(_ context.Context, checkpoint GuardCheckpoint, input *GuardInput) (*GuardResult, error) {
	if input == nil {
		return nil, fmt.Errorf("guard input is required")
	}

	g.mu.RLock()
	rules := append([]GuardRule(nil), g.rules[checkpoint]...)
	g.mu.RUnlock()

	result := &GuardResult{
		Allowed:    true,
		Checkpoint: checkpoint,
		Violations: []GuardViolation{},
		Sanitized:  input.Text,
	}

	for _, rule := range rules {
		violation := rule.Check(input)
		if violation == nil {
			continue
		}
		result.Violations = append(result.Violations, *violation)
		switch violation.Severity {
		case "block":
			result.Allowed = false
		case "warn":
			// warn does not block but is recorded
		case "audit":
			// audit-only
		}
	}

	return result, nil
}

// -----------------------------------------------------------------------
// Default built-in rules
// -----------------------------------------------------------------------

var (
	promptInjectionPatterns = []string{
		"ignore previous instructions",
		"ignore all previous",
		"disregard your instructions",
		"forget your instructions",
		"you are now",
		"pretend you are",
		"act as if you have no restrictions",
		"system prompt",
		"reveal your system",
		"what are your instructions",
		"bypass your",
	}

	sqlInjectionPattern    = regexp.MustCompile(`(?i)(\b(union\s+select|drop\s+table|delete\s+from|insert\s+into|update\s+.+\s+set|;--)\b|'\s*(or|and)\s+'?\d)`)
	pathTraversalPattern   = regexp.MustCompile(`(\.\.[\\/]|\.\.%2[fF])`)
	inferenceEmailPattern  = regexp.MustCompile(`(?i)\b[a-z0-9._%+\-]+@[a-z0-9.\-]+\.[a-z]{2,}\b`)
	inferenceSSNPattern    = regexp.MustCompile(`\b\d{3}-\d{2}-\d{4}\b`)
	inferencePhonePattern  = regexp.MustCompile(`\b(?:\+?1[-.\s]?)?\(?\d{3}\)?[-.\s]?\d{3}[-.\s]?\d{4}\b`)

	hallucinationMarkers = []string{
		"i think",
		"i'm not sure",
		"i believe",
		"it might be",
		"i'm guessing",
		"probably",
		"i don't know for certain",
	}

	toxicPatterns = []string{
		"kill yourself",
		"harm yourself",
		"you deserve to die",
		"i hate you",
	}

	errorDisclosurePatterns = []string{
		"stack trace",
		"runtime error",
		"panic:",
		"segmentation fault",
		"connection string",
		"password=",
		"secret_key",
		"api_key=",
	}
)

func (g *InferenceGuard) registerDefaults() {
	// --- PreInference rules ---

	g.rules[PreInference] = append(g.rules[PreInference], GuardRule{
		Name: "prompt_injection_detection",
		Check: func(input *GuardInput) *GuardViolation {
			lower := strings.ToLower(input.Text)
			for _, pattern := range promptInjectionPatterns {
				if strings.Contains(lower, pattern) {
					return &GuardViolation{
						Rule:        "prompt_injection_detection",
						Severity:    "block",
						Description: "Potential prompt injection detected",
						Evidence:    pattern,
					}
				}
			}
			return nil
		},
	})

	g.rules[PreInference] = append(g.rules[PreInference], GuardRule{
		Name: "input_length_limit",
		Check: func(input *GuardInput) *GuardViolation {
			const maxLen = 100_000
			if len(input.Text) > maxLen {
				return &GuardViolation{
					Rule:        "input_length_limit",
					Severity:    "block",
					Description: fmt.Sprintf("Input exceeds maximum length of %d characters", maxLen),
					Evidence:    fmt.Sprintf("length=%d", len(input.Text)),
				}
			}
			return nil
		},
	})

	g.rules[PreInference] = append(g.rules[PreInference], GuardRule{
		Name: "input_pii_detection",
		Check: func(input *GuardInput) *GuardViolation {
			if inferenceSSNPattern.MatchString(input.Text) {
				return &GuardViolation{
					Rule:        "input_pii_detection",
					Severity:    "warn",
					Description: "SSN pattern detected in input",
					Evidence:    "SSN-like pattern",
				}
			}
			return nil
		},
	})

	// --- PostInference rules ---

	g.rules[PostInference] = append(g.rules[PostInference], GuardRule{
		Name: "hallucination_marker_detection",
		Check: func(input *GuardInput) *GuardViolation {
			lower := strings.ToLower(input.ModelResponse)
			for _, marker := range hallucinationMarkers {
				if strings.Contains(lower, marker) {
					// Only flag if the response also makes a definitive claim.
					if containsDefinitiveClaim(lower) {
						return &GuardViolation{
							Rule:        "hallucination_marker_detection",
							Severity:    "warn",
							Description: "Response contains uncertainty markers alongside definitive claims",
							Evidence:    marker,
						}
					}
				}
			}
			return nil
		},
	})

	g.rules[PostInference] = append(g.rules[PostInference], GuardRule{
		Name: "toxic_content_detection",
		Check: func(input *GuardInput) *GuardViolation {
			lower := strings.ToLower(input.ModelResponse)
			for _, pattern := range toxicPatterns {
				if strings.Contains(lower, pattern) {
					return &GuardViolation{
						Rule:        "toxic_content_detection",
						Severity:    "block",
						Description: "Toxic content detected in model response",
						Evidence:    pattern,
					}
				}
			}
			return nil
		},
	})

	// --- PreToolCall rules ---

	g.rules[PreToolCall] = append(g.rules[PreToolCall], GuardRule{
		Name: "sql_injection_in_args",
		Check: func(input *GuardInput) *GuardViolation {
			for key, val := range input.ToolArgs {
				str, ok := val.(string)
				if !ok {
					continue
				}
				if sqlInjectionPattern.MatchString(str) {
					return &GuardViolation{
						Rule:        "sql_injection_in_args",
						Severity:    "block",
						Description: fmt.Sprintf("SQL injection pattern detected in tool arg %q", key),
						Evidence:    str,
					}
				}
			}
			return nil
		},
	})

	g.rules[PreToolCall] = append(g.rules[PreToolCall], GuardRule{
		Name: "path_traversal_in_args",
		Check: func(input *GuardInput) *GuardViolation {
			for key, val := range input.ToolArgs {
				str, ok := val.(string)
				if !ok {
					continue
				}
				if pathTraversalPattern.MatchString(str) {
					return &GuardViolation{
						Rule:        "path_traversal_in_args",
						Severity:    "block",
						Description: fmt.Sprintf("Path traversal detected in tool arg %q", key),
						Evidence:    str,
					}
				}
			}
			return nil
		},
	})

	// --- PostToolCall rules ---

	// IPI taint-tracking rule (highest priority for untrusted tool outputs).
	if defaultIPIRule != nil {
		g.rules[PostToolCall] = append(g.rules[PostToolCall], *defaultIPIRule)
	}

	g.rules[PostToolCall] = append(g.rules[PostToolCall], GuardRule{
		Name: "response_pii_leakage",
		Check: func(input *GuardInput) *GuardViolation {
			text := input.ModelResponse
			if inferenceSSNPattern.MatchString(text) {
				return &GuardViolation{
					Rule:        "response_pii_leakage",
					Severity:    "block",
					Description: "SSN detected in tool response",
					Evidence:    "SSN pattern in response",
				}
			}
			count := len(inferenceEmailPattern.FindAllString(text, -1))
			if count > 3 {
				return &GuardViolation{
					Rule:        "response_pii_leakage",
					Severity:    "warn",
					Description: "Multiple email addresses detected in tool response",
					Evidence:    fmt.Sprintf("%d emails found", count),
				}
			}
			return nil
		},
	})

	g.rules[PostToolCall] = append(g.rules[PostToolCall], GuardRule{
		Name: "error_info_disclosure",
		Check: func(input *GuardInput) *GuardViolation {
			lower := strings.ToLower(input.ModelResponse)
			for _, pattern := range errorDisclosurePatterns {
				if strings.Contains(lower, pattern) {
					return &GuardViolation{
						Rule:        "error_info_disclosure",
						Severity:    "warn",
						Description: "Error message information disclosure detected",
						Evidence:    pattern,
					}
				}
			}
			return nil
		},
	})
}

// containsDefinitiveClaim checks if text contains language suggesting a
// confident factual assertion.
func containsDefinitiveClaim(lower string) bool {
	claims := []string{
		"the answer is",
		"it is exactly",
		"the result is",
		"this is correct",
		"definitely",
		"certainly",
		"without a doubt",
		"100%",
	}
	for _, c := range claims {
		if strings.Contains(lower, c) {
			return true
		}
	}
	return false
}
