package executor

import (
	"sort"
	"strings"
	"time"
)

func HomeAssistantSupportedActions() []string {
	return []string{
		"homeassistant.turn_on",
		"homeassistant.turn_off",
		"light.turn_on",
		"climate.set_temperature",
		"scene.turn_on",
		"script.turn_on",
	}
}

func HomeAssistantEntityCacheRefreshInterval() time.Duration {
	return 60 * time.Second
}

func HomeAssistantRateLimitPerMinute() int {
	return 30
}

func CanRunEnvironmentProactiveAction(proactiveEnabled bool, domainAutonomy string) bool {
	if !proactiveEnabled {
		return false
	}
	switch strings.ToUpper(strings.TrimSpace(domainAutonomy)) {
	case "A2", "A3", "A4":
		return true
	default:
		return false
	}
}

func NormalizeEnvironmentSignalType(signalType string) string {
	allowed := map[string]struct{}{
		"temperature": {},
		"humidity":    {},
		"motion":      {},
		"door":        {},
		"weather":     {},
		"arrival":     {},
		"departure":   {},
	}
	normalized := strings.ToLower(strings.TrimSpace(signalType))
	if _, ok := allowed[normalized]; ok {
		return normalized
	}
	return "weather"
}

func FilterAllowedHomeAssistantActions(requested []string) []string {
	allowedSet := map[string]struct{}{}
	for _, action := range HomeAssistantSupportedActions() {
		allowedSet[action] = struct{}{}
	}
	filtered := make([]string, 0, len(requested))
	for _, action := range requested {
		if _, ok := allowedSet[action]; ok {
			filtered = append(filtered, action)
		}
	}
	sort.Strings(filtered)
	return filtered
}
