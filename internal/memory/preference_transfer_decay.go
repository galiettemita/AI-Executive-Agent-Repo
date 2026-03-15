package memory

import "math"

const (
	// TransferConfidenceMultiplier: transferred prefs start at 70% of source confidence.
	TransferConfidenceMultiplier = 0.70

	// PerLocalSignalDecay: each local observation reduces transfer weight by 12%.
	PerLocalSignalDecay = 0.12

	// MinTransferDecayFactor: transferred prefs never fully vanish.
	MinTransferDecayFactor = 0.10
)

// InitialTransferConfidence computes the starting confidence for a transferred preference.
func InitialTransferConfidence(sourceConfidence float64) float64 {
	return sourceConfidence * TransferConfidenceMultiplier
}

// DecayedTransferConfidence returns effective confidence after local observations.
func DecayedTransferConfidence(transferConfidence float64, localObservations int) float64 {
	if localObservations < 0 {
		localObservations = 0
	}
	decayFactor := math.Max(
		MinTransferDecayFactor,
		1.0-float64(localObservations)*PerLocalSignalDecay,
	)
	return transferConfidence * decayFactor
}

// ShouldUseTransfer returns true when transferred preference should be used.
// Local preference always wins when it has reasonable confidence.
func ShouldUseTransfer(transferConfidence, localConfidence float64) bool {
	if localConfidence > 0.40 {
		return false
	}
	return transferConfidence > 0.30
}
