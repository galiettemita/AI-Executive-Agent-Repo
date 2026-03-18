package brain

// ComputeThinkingBudget returns the token budget for extended thinking.
// Tiers: 4096 / 8192 / 16384 / 32768 / 65536 (ORM retry cap).
// Uses the existing ComplexitySignals struct from dynamic_decomposition.go.
func ComputeThinkingBudget(sig ComplexitySignals, prev *OutcomeScore) int {
	base := 4096
	if sig.DomainCount > 1 || sig.HasDependencies {
		base = 8192
	}
	if sig.DomainCount > 2 || sig.IntentCount > 2 {
		base = 16384
	}
	if sig.EntityCount > 8 || sig.IntentCount > 4 {
		base = 32768
	}
	if prev != nil && prev.OverallQuality < 3.0 {
		base = capBudget(base*2, 65536)
	}
	return base
}

func capBudget(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// ExtractComplexityFromSteps builds ComplexitySignals from plan steps and metadata.
func ExtractComplexityFromSteps(steps []PlanStep, intentCount, entityCount int, hasDependencies, hasTemporal bool) ComplexitySignals {
	domains := make(map[string]struct{})
	for _, step := range steps {
		domain := step.ToolKey
		for i, c := range domain {
			if c == '.' || c == '_' {
				domain = domain[:i]
				break
			}
		}
		if domain != "" {
			domains[domain] = struct{}{}
		}
	}
	if intentCount <= 0 {
		intentCount = 1
	}
	depCount := 0
	for _, step := range steps {
		depCount += len(step.DependsOn)
	}
	return ComplexitySignals{
		DomainCount:            len(domains),
		IntentCount:            intentCount,
		EntityCount:            entityCount,
		HasTemporalConstraints: hasTemporal,
		HasDependencies:        hasDependencies || depCount > 0,
	}
}
