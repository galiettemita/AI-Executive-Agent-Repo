package federated

import (
	"fmt"
	"time"

	"github.com/google/uuid"
)

// FederatedGradientInput describes a local gradient computation request.
type FederatedGradientInput struct {
	WorkspaceID     uuid.UUID
	PreferencePairs []PreferencePair
	Sigma           float64 // noise multiplier; default 1.0
	ClipNorm        float64 // L2 clip; default 1.0
	EpsilonTarget   float64 // default 3.0
	DeltaTarget     float64 // default 1e-5
}

// PreferencePair is a (prompt, chosen, rejected) triple.
type PreferencePair struct {
	PromptText       string
	ChosenResponse   string
	RejectedResponse string
}

// NoisyGradient is the output of local gradient computation with DP noise.
type NoisyGradient struct {
	WorkspaceID    uuid.UUID
	GradientVector []float64
	NoiseSigma     float64
	ClipNorm       float64
	RoundEpsilon   float64
	ComputedAt     time.Time
}

// Common errors.
var (
	ErrFederatedLearningDisabled  = fmt.Errorf("federated learning is disabled for this workspace (dp_sgd_enabled=false)")
	ErrDelegationWorkspaceExcluded = fmt.Errorf("delegation workspaces are excluded from federated learning")
	ErrConsentRequired            = fmt.Errorf("fine_tuning consent required for federated learning")
	ErrInsufficientParticipants   = fmt.Errorf("at least 2 workspaces required for federated aggregation")
)
