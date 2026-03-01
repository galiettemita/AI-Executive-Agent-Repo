package memory

import (
	"regexp"
	"strings"
)

type ExclusionRule struct {
	Pattern string
	Scope   string // exact or semantic
}

func EvaluateMemoryExclusionRules(content string, embeddingSimilarity float64, rules []ExclusionRule) bool {
	normalizedContent := strings.ToLower(strings.TrimSpace(content))
	for _, rule := range rules {
		pattern := strings.TrimSpace(rule.Pattern)
		if pattern == "" {
			continue
		}
		scope := strings.ToLower(strings.TrimSpace(rule.Scope))
		switch scope {
		case "semantic":
			if embeddingSimilarity > 0.85 {
				return true
			}
		case "exact":
			if strings.Contains(normalizedContent, strings.ToLower(pattern)) {
				return true
			}
		default:
			re, err := regexp.Compile(pattern)
			if err != nil {
				continue
			}
			if re.MatchString(normalizedContent) {
				return true
			}
		}
	}
	return false
}
