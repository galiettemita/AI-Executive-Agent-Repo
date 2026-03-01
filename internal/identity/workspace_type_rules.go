package identity

import (
	"slices"
	"strings"
)

func IsTwoManRuleActive(workspaceType string, hasAdminUsers bool) bool {
	return strings.EqualFold(strings.TrimSpace(workspaceType), "professional") && hasAdminUsers
}

func MaxAutonomyForWorkspaceType(workspaceType, delegationGrantMax string) string {
	if strings.EqualFold(strings.TrimSpace(workspaceType), "delegation") {
		cap := strings.ToUpper(strings.TrimSpace(delegationGrantMax))
		switch cap {
		case "A0", "A1", "A2":
			return cap
		default:
			return "A2"
		}
	}
	return "A4"
}

func FilterMemoryKeysForWorkspaceType(workspaceType string, memoryKeys, sharedMemoryKeys []string) []string {
	if !strings.EqualFold(strings.TrimSpace(workspaceType), "delegation") {
		return append([]string{}, memoryKeys...)
	}
	out := make([]string, 0, len(sharedMemoryKeys))
	for _, key := range memoryKeys {
		if slices.Contains(sharedMemoryKeys, key) {
			out = append(out, key)
		}
	}
	return out
}

func FilterToolsForWorkspaceType(workspaceType string, provisionedTools, allowedToolKeys []string) []string {
	if !strings.EqualFold(strings.TrimSpace(workspaceType), "delegation") {
		return append([]string{}, provisionedTools...)
	}
	out := make([]string, 0, len(allowedToolKeys))
	for _, toolKey := range provisionedTools {
		if slices.Contains(allowedToolKeys, toolKey) {
			out = append(out, toolKey)
		}
	}
	return out
}

func DelegateFinancialAccessGranted(allowedDomains, allowedToolKeys []string) bool {
	if slices.Contains(allowedDomains, "financial") {
		return true
	}
	for _, toolKey := range allowedToolKeys {
		lower := strings.ToLower(strings.TrimSpace(toolKey))
		if strings.Contains(lower, "plaid") || strings.Contains(lower, "stripe") || strings.Contains(lower, "quickbooks") || strings.Contains(lower, "mercury") {
			return true
		}
	}
	return false
}
