package memory

import "strings"

func ConsolidationCadenceHours() int {
	return 6
}

func DuplicateMergeThreshold(activeItems int) float64 {
	if activeItems > 10000 {
		return 0.85
	}
	return 0.92
}

func ShouldExpireByStaleness(daysSinceAccess int) bool {
	return daysSinceAccess >= 90
}

func ShouldSupersedeByConfidence(confidence float64, activeItems int) bool {
	threshold := 0.3
	if activeItems > 10000 {
		threshold = 0.5
	}
	return confidence < threshold
}

func ShouldAutoApproveMemoryWrite(memoryType string, confidence float64, isUpdateWithHigherConfidence bool) bool {
	switch strings.ToLower(strings.TrimSpace(memoryType)) {
	case "daily_log", "heartbeat":
		return true
	case "task_fact":
		return confidence >= 0.9
	default:
		return isUpdateWithHigherConfidence
	}
}

func ShouldRequireMemoryConfirmation(memoryType string, confidence float64, dataClass string, containsPII bool) bool {
	normalizedType := strings.ToLower(strings.TrimSpace(memoryType))
	normalizedClass := strings.ToUpper(strings.TrimSpace(dataClass))

	if normalizedType == "preference" || normalizedType == "rule" {
		return true
	}
	if normalizedType == "contact_fact" && containsPII {
		return true
	}
	if confidence < 0.7 {
		return true
	}
	switch normalizedClass {
	case "SENSITIVE", "FINANCIAL", "HEALTH":
		return true
	default:
		return false
	}
}
