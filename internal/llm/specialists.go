package llm

import (
	"regexp"
	"sort"
	"strings"
)

type SpecialistAgent struct {
	SpecialistID    string
	WorkspaceID     string
	Name            string
	SystemPromptKey string
	AllowedToolKeys []string
	AllowedDomains  []string
	Tier            string
	TriggerRegexes  []string
	TriggerKeywords []string
	IsActive        bool
}

func normalizeSpecialistName(name string) string {
	normalized := strings.ToLower(strings.TrimSpace(name))
	normalized = strings.TrimPrefix(normalized, "@")
	normalized = strings.TrimPrefix(normalized, "ask my ")
	return normalized
}

func activeSpecialists(specialists []SpecialistAgent) []SpecialistAgent {
	out := make([]SpecialistAgent, 0, len(specialists))
	for _, specialist := range specialists {
		if specialist.IsActive {
			out = append(out, specialist)
		}
	}
	sort.Slice(out, func(i, j int) bool {
		return strings.ToLower(out[i].Name) < strings.ToLower(out[j].Name)
	})
	return out
}

// RouteToSpecialist applies explicit invocation, trigger-pattern matching, then planner suggestion.
func RouteToSpecialist(query, explicitInvocation, plannerSuggestion string, specialists []SpecialistAgent) (SpecialistAgent, bool, string) {
	active := activeSpecialists(specialists)
	if len(active) == 0 {
		return SpecialistAgent{}, false, "default_brain"
	}

	explicit := normalizeSpecialistName(explicitInvocation)
	if explicit != "" {
		for _, specialist := range active {
			if normalizeSpecialistName(specialist.Name) == explicit {
				return specialist, true, "explicit_invocation"
			}
		}
	}

	loweredQuery := strings.ToLower(query)
	for _, specialist := range active {
		for _, pattern := range specialist.TriggerRegexes {
			re, err := regexp.Compile(pattern)
			if err != nil {
				continue
			}
			if re.MatchString(loweredQuery) {
				return specialist, true, "pattern_match"
			}
		}
		for _, keyword := range specialist.TriggerKeywords {
			if strings.Contains(loweredQuery, strings.ToLower(strings.TrimSpace(keyword))) {
				return specialist, true, "pattern_match"
			}
		}
	}

	if suggestion := normalizeSpecialistName(plannerSuggestion); suggestion != "" {
		for _, specialist := range active {
			if normalizeSpecialistName(specialist.Name) == suggestion {
				return specialist, true, "planner_suggestion"
			}
		}
	}

	return SpecialistAgent{}, false, "default_brain"
}

func FilterToolsForSpecialist(availableToolKeys, allowedToolKeys []string) []string {
	allowed := map[string]struct{}{}
	for _, toolKey := range allowedToolKeys {
		allowed[strings.TrimSpace(toolKey)] = struct{}{}
	}
	filtered := make([]string, 0, len(availableToolKeys))
	for _, toolKey := range availableToolKeys {
		if _, ok := allowed[toolKey]; ok {
			filtered = append(filtered, toolKey)
		}
	}
	sort.Strings(filtered)
	return filtered
}
