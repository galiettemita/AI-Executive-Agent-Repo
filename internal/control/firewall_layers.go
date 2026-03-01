package control

import (
	"regexp"
	"strings"
	"unicode"
)

type FirewallLayerVerdict struct {
	Layer       string
	Action      string
	Reason      string
	Blocked     bool
	Quarantined bool
}

type ContentFirewallResult struct {
	SanitizedContent string
	DataClass        string
	SensitivityLabel string
	ContentTrust     string
	Verdicts         []FirewallLayerVerdict
	ShouldBlock      bool
	ShouldQuarantine bool
}

type ContentPolicy struct {
	BlockedTopics           []string
	ProhibitedActionPhrases []string
	AllowedDataClasses      map[string]struct{}
}

var l1InjectionPatterns = []*regexp.Regexp{
	regexp.MustCompile(`(?i)ignore\s+previous\s+instructions`),
	regexp.MustCompile(`(?i)system\s+prompt\s+override`),
	regexp.MustCompile(`(?i)role-?play\s+as`),
	regexp.MustCompile(`(?i)act\s+as\s+the\s+system`),
}

var emailPattern = regexp.MustCompile(`(?i)\b[a-z0-9._%+\-]+@[a-z0-9.\-]+\.[a-z]{2,}\b`)
var phonePattern = regexp.MustCompile(`\+?[0-9][0-9\-\s\(\)]{7,}[0-9]`)
var financialPattern = regexp.MustCompile(`(?i)\b(invoice|wire|bank|routing number|account number|transaction|credit card)\b`)
var healthPattern = regexp.MustCompile(`(?i)\b(medical|diagnosis|prescription|blood pressure|heart rate|health)\b`)
var secretPattern = regexp.MustCompile(`(?i)\b(api[_ -]?key|secret|password|token|private key)\b`)

func sanitizeL1(input string, maxLen int) (string, bool) {
	if maxLen <= 0 {
		maxLen = 10000
	}
	builder := strings.Builder{}
	blockedForInjection := false
	lastWasSpace := false

	for _, r := range input {
		if r == 0 {
			continue
		}
		if unicode.IsControl(r) && r != '\n' && r != '\t' {
			continue
		}
		if unicode.IsSpace(r) {
			if lastWasSpace {
				continue
			}
			lastWasSpace = true
			builder.WriteRune(' ')
			continue
		}
		lastWasSpace = false
		builder.WriteRune(r)
		if builder.Len() >= maxLen {
			break
		}
	}

	out := strings.TrimSpace(builder.String())
	for _, pattern := range l1InjectionPatterns {
		if pattern.MatchString(out) {
			blockedForInjection = true
			break
		}
	}
	return out, blockedForInjection
}

func classifyL2(content string) (dataClass string, sensitivity string) {
	switch {
	case secretPattern.MatchString(content):
		return "SECRETS", "high"
	case financialPattern.MatchString(content):
		return "FINANCIAL", "high"
	case healthPattern.MatchString(content):
		return "HEALTH", "high"
	case emailPattern.MatchString(content) || phonePattern.MatchString(content):
		return "SENSITIVE", "moderate"
	default:
		return "PRIVATE", "low"
	}
}

func evaluateL3Policy(content string, policy ContentPolicy, dataClass string) (blocked bool, reason string) {
	lowered := strings.ToLower(content)
	for _, topic := range policy.BlockedTopics {
		if topic == "" {
			continue
		}
		if strings.Contains(lowered, strings.ToLower(topic)) {
			return true, "blocked_topic:" + topic
		}
	}
	for _, phrase := range policy.ProhibitedActionPhrases {
		if phrase == "" {
			continue
		}
		if strings.Contains(lowered, strings.ToLower(phrase)) {
			return true, "prohibited_action:" + phrase
		}
	}
	if len(policy.AllowedDataClasses) > 0 {
		if _, ok := policy.AllowedDataClasses[dataClass]; !ok {
			return true, "data_class_not_allowed"
		}
	}
	return false, ""
}

// RunContentFirewall executes L1 -> L2 -> L3 -> L4(quarantine on flag).
func RunContentFirewall(input string, maxLen int, policy ContentPolicy) ContentFirewallResult {
	sanitized, l1InjectionBlocked := sanitizeL1(input, maxLen)
	dataClass, sensitivity := classifyL2(sanitized)
	contentTrust := "trusted"
	if l1InjectionBlocked {
		contentTrust = "untrusted"
	}

	verdicts := []FirewallLayerVerdict{
		{Layer: "L1", Action: "strip_transform", Reason: "sanitized", Blocked: l1InjectionBlocked},
		{Layer: "L2", Action: "classify_tag", Reason: dataClass + ":" + sensitivity},
	}

	l3Blocked, l3Reason := evaluateL3Policy(sanitized, policy, dataClass)
	verdicts = append(verdicts, FirewallLayerVerdict{
		Layer:   "L3",
		Action:  "policy_evaluation",
		Reason:  l3Reason,
		Blocked: l3Blocked || l1InjectionBlocked,
	})

	shouldQuarantine := l3Blocked || l1InjectionBlocked
	if shouldQuarantine {
		verdicts = append(verdicts, FirewallLayerVerdict{
			Layer:       "L4",
			Action:      "quarantine",
			Reason:      "operator_review_required",
			Blocked:     true,
			Quarantined: true,
		})
	}

	return ContentFirewallResult{
		SanitizedContent: sanitized,
		DataClass:        dataClass,
		SensitivityLabel: sensitivity,
		ContentTrust:     contentTrust,
		Verdicts:         verdicts,
		ShouldBlock:      l3Blocked || l1InjectionBlocked,
		ShouldQuarantine: shouldQuarantine,
	}
}
