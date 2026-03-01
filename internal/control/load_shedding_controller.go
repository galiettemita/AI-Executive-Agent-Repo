package control

import (
	"strings"
	"time"
)

type LoadSheddingMetrics struct {
	CPUPercent                 float64
	ErrorRatePercent           float64
	DBPoolUtilizationPercent   float64
	LLMProviderDegraded        bool
	MultipleProviderFailures   bool
	InfrastructureEmergency    bool
	ManualOperatorEmergencyD5  bool
	OperatorConfirmedRecovery  bool
	CurrentConditionPersisted  time.Duration
	ResolvedConditionPersisted time.Duration
}

var loadSheddingOrder = map[string]int{
	"D0": 0,
	"D1": 1,
	"D2": 2,
	"D3": 3,
	"D4": 4,
	"D5": 5,
}

func normalizeLoadSheddingTier(tier string) string {
	normalized := strings.ToUpper(strings.TrimSpace(tier))
	if _, ok := loadSheddingOrder[normalized]; ok {
		return normalized
	}
	return "D0"
}

// TargetLoadSheddingTier computes the desired operating tier from live system metrics.
// D5 is always manual-only (operator emergency action).
func TargetLoadSheddingTier(metrics LoadSheddingMetrics) string {
	if metrics.CPUPercent > 95 || metrics.ErrorRatePercent > 10 || metrics.DBPoolUtilizationPercent > 90 {
		return "D4"
	}
	if metrics.CPUPercent > 90 || metrics.ErrorRatePercent > 5 || metrics.MultipleProviderFailures {
		return "D3"
	}
	if metrics.CPUPercent > 85 || metrics.ErrorRatePercent > 3 || metrics.LLMProviderDegraded {
		return "D2"
	}
	if metrics.CPUPercent > 80 || metrics.ErrorRatePercent > 2 {
		return "D1"
	}
	return "D0"
}

// NextLoadSheddingTier applies addendum auto-escalation/auto-recovery rules.
func NextLoadSheddingTier(currentTier string, metrics LoadSheddingMetrics) string {
	current := normalizeLoadSheddingTier(currentTier)
	target := TargetLoadSheddingTier(metrics)

	if metrics.ManualOperatorEmergencyD5 {
		return "D5"
	}

	// D5 and D4 never auto-recover without operator confirmation.
	if current == "D5" {
		if metrics.OperatorConfirmedRecovery {
			return "D4"
		}
		return "D5"
	}
	if current == "D4" {
		if metrics.OperatorConfirmedRecovery && metrics.ResolvedConditionPersisted >= 5*time.Minute {
			return "D3"
		}
		// Continue normal escalation behavior if conditions worsen.
		if target == "D4" {
			return "D4"
		}
		return "D4"
	}

	// Immediate first-step escalation from nominal.
	if current == "D0" && loadSheddingOrder[target] >= loadSheddingOrder["D1"] {
		return "D1"
	}

	// Timed escalations.
	switch current {
	case "D1":
		if loadSheddingOrder[target] >= loadSheddingOrder["D2"] && metrics.CurrentConditionPersisted >= 5*time.Minute {
			return "D2"
		}
	case "D2":
		if loadSheddingOrder[target] >= loadSheddingOrder["D3"] && metrics.CurrentConditionPersisted >= 5*time.Minute {
			return "D3"
		}
	case "D3":
		if loadSheddingOrder[target] >= loadSheddingOrder["D4"] && metrics.CurrentConditionPersisted >= 10*time.Minute {
			return "D4"
		}
	}

	// Timed auto-recovery for D1-D3 only.
	if loadSheddingOrder[target] < loadSheddingOrder[current] && metrics.ResolvedConditionPersisted >= 5*time.Minute {
		switch current {
		case "D3":
			return "D2"
		case "D2":
			return "D1"
		case "D1":
			return "D0"
		}
	}

	return current
}

func LoadSheddingTierChanged(previousTier, nextTier string) bool {
	return normalizeLoadSheddingTier(previousTier) != normalizeLoadSheddingTier(nextTier)
}
