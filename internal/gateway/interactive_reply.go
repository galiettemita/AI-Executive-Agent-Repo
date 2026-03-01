package gateway

import (
	"strconv"
	"strings"
)

// ResolveInteractiveIntentWithPendingOptions maps parsed replies against a pending approval context.
func ResolveInteractiveIntentWithPendingOptions(parsed string, pendingOptions []string) (intent string, resolvedOption string, matched bool) {
	normalized := strings.TrimSpace(parsed)
	if normalized == "" {
		return "", "", false
	}

	switch normalized {
	case string(IntentApprove), string(IntentDeny), string(IntentUndo):
		return normalized, "", true
	}

	if strings.HasPrefix(normalized, "OPTION:") {
		option := strings.TrimSpace(strings.TrimPrefix(normalized, "OPTION:"))
		for _, pending := range pendingOptions {
			if pending == option {
				return string(IntentOption), option, true
			}
		}
		return string(IntentOption), option, false
	}

	if strings.HasPrefix(normalized, "OPTION_INDEX:") {
		rawIndex := strings.TrimSpace(strings.TrimPrefix(normalized, "OPTION_INDEX:"))
		index, err := strconv.Atoi(rawIndex)
		if err != nil || index <= 0 || index > len(pendingOptions) {
			return string(IntentOption), "", false
		}
		return string(IntentOption), pendingOptions[index-1], true
	}

	if strings.HasPrefix(normalized, string(IntentEdit)+":") {
		return string(IntentEdit), strings.TrimSpace(strings.TrimPrefix(normalized, string(IntentEdit)+":")), true
	}

	return "", "", false
}
