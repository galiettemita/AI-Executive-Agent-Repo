package preference

import (
	"fmt"
	"strings"
)

// ExtractFact infers a structured preference fact from a correction signal.
func ExtractFact(signal PreferenceSignal) PreferenceFact {
	category := detectCategory(signal)
	pref := buildPreferenceStatement(signal, category)

	return PreferenceFact{
		WorkspaceID: signal.WorkspaceID,
		UserID:      signal.UserID,
		Category:    category,
		Preference:  pref,
		Confidence:  0.95, // direct correction = highest confidence
		EvidenceID:  signal.WorkflowRunID,
	}
}

func detectCategory(signal PreferenceSignal) string {
	lower := strings.ToLower(signal.OriginalIntent)
	toolLower := strings.ToLower(signal.ToolKeyUsed)

	switch {
	case strings.Contains(toolLower, "email") || strings.Contains(lower, "email"):
		return "email_style"
	case strings.Contains(toolLower, "calendar") || strings.Contains(lower, "schedule") || strings.Contains(lower, "meeting"):
		return "scheduling"
	case strings.Contains(toolLower, "slack") || strings.Contains(lower, "slack") || strings.Contains(lower, "message"):
		return "messaging_style"
	case strings.Contains(lower, "summarize") || strings.Contains(lower, "summary"):
		return "summary_style"
	case signal.SignalType == "undo":
		return "action_preference"
	default:
		return "general_preference"
	}
}

func buildPreferenceStatement(signal PreferenceSignal, category string) string {
	signalDesc := map[string]string{
		"undo": "undid the action", "edit": "edited the response",
		"retry": "asked for a retry", "skip": "skipped this type of action",
		"explicit_thumbsdown": "explicitly rejected this response",
	}
	action := signalDesc[signal.SignalType]
	if action == "" {
		action = "corrected the response"
	}

	if signal.CorrectedResponse != "" && signal.OriginalResponse != "" {
		origLen := len(strings.Fields(signal.OriginalResponse))
		corrLen := len(strings.Fields(signal.CorrectedResponse))
		if corrLen < origLen/2 {
			return fmt.Sprintf("User prefers shorter, more concise responses in %s context (corrected %d words to %d words)",
				category, origLen, corrLen)
		}
		if corrLen > origLen*2 {
			return fmt.Sprintf("User prefers more detailed responses in %s context (corrected %d words to %d words)",
				category, origLen, corrLen)
		}
	}

	return fmt.Sprintf("User %s for %s intent in %s context. Prefer alternative approach.",
		action, signal.OriginalIntent, category)
}

// FormatForLLM converts preference facts into an LLM-injectable context block.
func FormatForLLM(facts []PreferenceFact) string {
	if len(facts) == 0 {
		return ""
	}
	var sb strings.Builder
	sb.WriteString("USER PREFERENCE CONTEXT (apply these learned preferences):\n")
	for i, f := range facts {
		sb.WriteString(fmt.Sprintf("%d. [%s] %s\n", i+1, f.Category, f.Preference))
	}
	return sb.String()
}
