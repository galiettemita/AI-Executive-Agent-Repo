package guardrails

import (
	"fmt"
	"strings"
)

// TrustSource classifies the origin of content being evaluated.
type TrustSource string

const (
	TrustSourceSystem   TrustSource = "system"
	TrustSourceUser     TrustSource = "user"
	TrustSourceInternal TrustSource = "tool_internal"
	TrustSourceWeb      TrustSource = "tool_web"
	TrustSourceEmail    TrustSource = "tool_email"
	TrustSourceCalendar TrustSource = "tool_calendar"
	TrustSourceExternal TrustSource = "tool_external"
)

// IsUntrusted returns true for sources that may carry adversarial content.
func (ts TrustSource) IsUntrusted() bool {
	switch ts {
	case TrustSourceWeb, TrustSourceEmail, TrustSourceCalendar, TrustSourceExternal:
		return true
	}
	return false
}

// InferTrustSource maps a tool key to its TrustSource classification.
func InferTrustSource(toolKey string) TrustSource {
	lower := strings.ToLower(toolKey)
	switch {
	case strings.HasPrefix(lower, "email.") || strings.HasPrefix(lower, "mail.") ||
		strings.HasPrefix(lower, "gmail.") || strings.HasPrefix(lower, "imap.") ||
		strings.HasPrefix(lower, "smtp."):
		return TrustSourceEmail
	case strings.HasPrefix(lower, "calendar.") || strings.HasPrefix(lower, "google_calendar."):
		return TrustSourceCalendar
	case strings.HasPrefix(lower, "web.") || strings.HasPrefix(lower, "browser.") ||
		lower == "web_search" || strings.HasPrefix(lower, "brave") ||
		strings.HasPrefix(lower, "tavily") || strings.HasPrefix(lower, "perplexity"):
		return TrustSourceWeb
	case strings.HasPrefix(lower, "internal.") || strings.HasPrefix(lower, "db."):
		return TrustSourceInternal
	default:
		return TrustSourceExternal
	}
}

// IPIGuardInput extends GuardInput with trust-source metadata.
type IPIGuardInput struct {
	WorkspaceID  string      `json:"workspace_id"`
	TrustSource  TrustSource `json:"trust_source"`
	ToolOutput   string      `json:"tool_output"`
	AutonomyTier string      `json:"autonomy_tier"`
}

// IPIGuardResult extends GuardResult with IPI-specific fields.
type IPIGuardResult struct {
	Allowed          bool   `json:"allowed"`
	Reason           string `json:"reason,omitempty"`
	UntrustedContent bool   `json:"untrusted_content,omitempty"`
	RequiresTier     string `json:"requires_tier,omitempty"`
}

// CheckPostToolCallIPI evaluates tool output for indirect prompt injection.
func (g *InferenceGuard) CheckPostToolCallIPI(input IPIGuardInput) IPIGuardResult {
	if !input.TrustSource.IsUntrusted() {
		return IPIGuardResult{Allowed: true}
	}

	if blocked, reason := ipiRule(input); blocked {
		return IPIGuardResult{
			Allowed:          false,
			Reason:           reason,
			UntrustedContent: true,
			RequiresTier:     "A3",
		}
	}

	return IPIGuardResult{Allowed: true, UntrustedContent: true}
}

// ipiRule detects potential indirect prompt injection in tool output.
func ipiRule(input IPIGuardInput) (blocked bool, reason string) {
	output := strings.ToLower(input.ToolOutput)

	injectionPatterns := []string{
		"ignore previous instructions",
		"ignore all previous",
		"disregard your instructions",
		"disregard all previous",
		"you are now",
		"new system prompt",
		"override your",
		"forget your previous",
		"act as",
		"pretend you are",
		"your new instructions",
		"print your system prompt",
		"reveal your prompt",
		"what are your instructions",
		"repeat after me",
		"say exactly",
		"base64",
		"hex decode",
		"rot13",
		"\\u0000",
		"<!--",
		"<script",
		"javascript:",
		"</s>",
		"[/inst]",
		"### instruction",
		"### system",
		"<<sys>>",
		"assistant:",
		"human:",
		"system override",
		"forward all",
	}

	for _, pattern := range injectionPatterns {
		if strings.Contains(output, pattern) {
			return true, fmt.Sprintf("IPI_DETECTED:source=%s:pattern=%q",
				input.TrustSource, pattern)
		}
	}

	// Structural anomaly: very long single-line without spaces.
	lines := strings.Split(input.ToolOutput, "\n")
	for _, line := range lines {
		if len(line) > 500 && strings.Count(line, " ") < 5 {
			return true, fmt.Sprintf("IPI_SUSPICIOUS_STRUCTURE:source=%s:line_len=%d",
				input.TrustSource, len(line))
		}
	}

	return false, ""
}

func init() {
	// Also register the IPI rule as a PostToolCall GuardRule so it runs
	// in the standard CheckInput pipeline for tool outputs.
	ipiPostToolCallRule := GuardRule{
		Name: "ipi_taint_tracking",
		Check: func(input *GuardInput) *GuardViolation {
			if input.TrustSource == "" || !TrustSource(input.TrustSource).IsUntrusted() {
				return nil
			}
			toolOutput := input.ModelResponse
			if toolOutput == "" {
				toolOutput = input.ToolOutput
			}
			if toolOutput == "" {
				return nil
			}
			ipiInput := IPIGuardInput{
				WorkspaceID: input.WorkspaceID,
				TrustSource: TrustSource(input.TrustSource),
				ToolOutput:  toolOutput,
			}
			if blocked, reason := ipiRule(ipiInput); blocked {
				return &GuardViolation{
					Rule:        "ipi_taint_tracking",
					Severity:    "block",
					Description: reason,
					Evidence:    fmt.Sprintf("source=%s", input.TrustSource),
				}
			}
			return nil
		},
	}
	defaultIPIRule = &ipiPostToolCallRule
}

var defaultIPIRule *GuardRule
