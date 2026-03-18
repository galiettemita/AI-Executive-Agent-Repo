package federated

import (
	"context"
	"crypto/rand"
	"encoding/binary"
	"fmt"
	"log/slog"
	"math"
	mathrand "math/rand"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

// FederatedGradientActivity computes local noisy gradients for a workspace.
type FederatedGradientActivity struct {
	db     *pgxpool.Pool
	logger *slog.Logger
	rng    *mathrand.Rand
}

// NewFederatedGradientActivity creates a gradient activity with crypto-safe seed.
func NewFederatedGradientActivity(db *pgxpool.Pool, logger *slog.Logger) *FederatedGradientActivity {
	var seed int64
	if err := binary.Read(rand.Reader, binary.LittleEndian, &seed); err != nil {
		seed = time.Now().UnixNano()
	}
	return &FederatedGradientActivity{
		db:     db,
		logger: logger,
		rng:    mathrand.New(mathrand.NewSource(seed)),
	}
}

// ComputeLocalGradient computes a DP-SGD noisy gradient for a workspace.
func (a *FederatedGradientActivity) ComputeLocalGradient(ctx context.Context, input FederatedGradientInput) (*NoisyGradient, error) {
	if len(input.PreferencePairs) == 0 {
		return nil, fmt.Errorf("no preference pairs provided")
	}

	sigma := input.Sigma
	if sigma <= 0 {
		sigma = 1.0
	}
	clipNorm := input.ClipNorm
	if clipNorm <= 0 {
		clipNorm = 1.0
	}

	// Step 1: Compute gradient proxy for each preference pair.
	grad := make([]float64, len(input.PreferencePairs))
	for i, pair := range input.PreferencePairs {
		chosenScore := scoreText(pair.ChosenResponse)
		rejectedScore := scoreText(pair.RejectedResponse)
		grad[i] = chosenScore - rejectedScore
	}

	// Step 2: Clip L2 norm.
	grad = ClipGradient(grad, clipNorm)

	// Step 3: Add Gaussian noise (DP-SGD).
	noisedGrad := AddGaussianNoise(grad, sigma, clipNorm, a.rng)

	result := &NoisyGradient{
		WorkspaceID:    input.WorkspaceID,
		GradientVector: noisedGrad,
		NoiseSigma:     sigma,
		ClipNorm:       clipNorm,
		ComputedAt:     time.Now(),
	}

	a.logger.Info("local_gradient_computed",
		"workspace_id", input.WorkspaceID,
		"dimensions", len(noisedGrad),
		"sigma", sigma,
		"clip_norm", clipNorm,
	)

	return result, nil
}

// scoreText computes a simple proxy score for text (length-normalized hash).
func scoreText(text string) float64 {
	if len(text) == 0 {
		return 0
	}
	score := 0.0
	for _, r := range text {
		score += float64(r) / 1000.0
	}
	return score / float64(len(text))
}

// ClipGradient clips the L2 norm of a gradient vector.
func ClipGradient(grad []float64, maxNorm float64) []float64 {
	l2Norm := math.Sqrt(DotProduct(grad, grad))
	if l2Norm > maxNorm {
		scale := maxNorm / l2Norm
		for i := range grad {
			grad[i] *= scale
		}
	}
	return grad
}

// AddGaussianNoise adds calibrated Gaussian noise for DP-SGD.
func AddGaussianNoise(grad []float64, sigma float64, clipNorm float64, rng *mathrand.Rand) []float64 {
	noised := make([]float64, len(grad))
	for i, g := range grad {
		noise := rng.NormFloat64() * sigma * clipNorm
		noised[i] = g + noise
	}
	return noised
}

// DotProduct computes the dot product of two vectors.
func DotProduct(a, b []float64) float64 {
	sum := 0.0
	n := len(a)
	if len(b) < n {
		n = len(b)
	}
	for i := 0; i < n; i++ {
		sum += a[i] * b[i]
	}
	return sum
}
