package brain

import (
	"os"
	"strconv"
)

const defaultCouncilConveneThreshold = 0.8

// MoATrigger determines whether MoA should fire for a given request.
type MoATrigger struct {
	enabled                 bool
	councilConveneThreshold float64
}

// NewMoATrigger creates a MoA trigger from environment configuration.
func NewMoATrigger() *MoATrigger {
	enabled := os.Getenv("FEATURE_MOA_ENABLED") == "true"
	threshold := defaultCouncilConveneThreshold
	if v := os.Getenv("MOA_COUNCIL_THRESHOLD"); v != "" {
		if f, err := strconv.ParseFloat(v, 64); err == nil {
			threshold = f
		}
	}
	return &MoATrigger{enabled: enabled, councilConveneThreshold: threshold}
}

// ShouldInvoke returns true if MoA should be invoked for this request.
// Conditions:
//   - FEATURE_MOA_ENABLED=true (env var)
//   - NOT T0/T1: latencyBudgetMs must be 0 (unconstrained) or >= 4000ms
//   - councilScore > threshold (0.8) OR (isRetry AND reactConfidence < 0.6)
func (t *MoATrigger) ShouldInvoke(
	reactConfidence float64,
	councilScore float64,
	latencyBudgetMs int,
	isRetry bool,
) bool {
	if !t.enabled {
		return false
	}
	// Latency gate: MoA adds ~2s latency, only viable on T2/T3.
	if latencyBudgetMs > 0 && latencyBudgetMs < 4000 {
		return false
	}
	return councilScore > t.councilConveneThreshold ||
		(isRetry && reactConfidence < 0.6)
}
