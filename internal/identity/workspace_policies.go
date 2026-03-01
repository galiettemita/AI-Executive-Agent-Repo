package identity

import (
	"slices"
	"strings"
)

func NormalizeDomainAutonomy(input map[string]string) map[string]string {
	requiredDomains := []string{"calendar", "email", "messaging", "tasks", "documents", "crm", "travel", "financial", "health", "environment", "web"}
	out := map[string]string{}
	for _, domain := range requiredDomains {
		value := strings.ToUpper(strings.TrimSpace(input[domain]))
		switch value {
		case "A0", "A1", "A2", "A3", "A4":
			out[domain] = value
		default:
			out[domain] = "A0"
		}
	}
	return out
}

func UpdateAllowedConnectorKeys(current []string, event, connectorKey string) []string {
	event = strings.ToLower(strings.TrimSpace(event))
	connectorKey = strings.TrimSpace(connectorKey)
	if connectorKey == "" {
		return current
	}
	out := append([]string{}, current...)
	switch event {
	case "provisioned":
		if !slices.Contains(out, connectorKey) {
			out = append(out, connectorKey)
		}
	case "deprovisioned", "admin_block":
		filtered := make([]string, 0, len(out))
		for _, key := range out {
			if key != connectorKey {
				filtered = append(filtered, key)
			}
		}
		out = filtered
	}
	return out
}

func EffectiveWorkspaceAutonomyCap(workspaceType, delegationGrantMax string) string {
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
