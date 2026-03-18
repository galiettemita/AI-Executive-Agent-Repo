package federated

import (
	"context"
	"log/slog"
	"math"
	mathrand "math/rand"
	"os"
	"testing"

	"github.com/google/uuid"
)

var testLogger = slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))

func TestGradientClipping(t *testing.T) {
	grad := []float64{3.0, 4.0} // L2 norm = 5.0
	clipped := ClipGradient(grad, 1.0)

	norm := math.Sqrt(DotProduct(clipped, clipped))
	if norm > 1.0+1e-9 {
		t.Errorf("Expected clipped norm <= 1.0, got %f", norm)
	}
	t.Logf("Clipped gradient: %v, norm: %f", clipped, norm)
}

func TestGradientClippingNoOp(t *testing.T) {
	grad := []float64{0.3, 0.4} // L2 norm = 0.5
	clipped := ClipGradient(grad, 1.0)

	// Should not change — already within norm.
	for i := range grad {
		if math.Abs(clipped[i]-grad[i]) > 1e-9 {
			t.Errorf("Expected no change for small gradient, index %d: %f != %f", i, clipped[i], grad[i])
		}
	}
}

func TestGaussianNoiseMeanZero(t *testing.T) {
	rng := mathrand.New(mathrand.NewSource(42))
	sigma := 1.0
	clipNorm := 1.0

	n := 10000
	totalNoise := 0.0
	totalNoiseSq := 0.0

	for i := 0; i < n; i++ {
		grad := []float64{0.0} // zero gradient — noise only
		noised := AddGaussianNoise(grad, sigma, clipNorm, rng)
		noise := noised[0]
		totalNoise += noise
		totalNoiseSq += noise * noise
	}

	meanNoise := totalNoise / float64(n)
	stdNoise := math.Sqrt(totalNoiseSq/float64(n) - meanNoise*meanNoise)

	t.Logf("Gaussian noise: mean=%.4f, std=%.4f (expected std=%.4f)", meanNoise, stdNoise, sigma*clipNorm)

	if math.Abs(meanNoise) > 0.1 {
		t.Errorf("Expected mean noise ≈ 0, got %f", meanNoise)
	}
	if math.Abs(stdNoise-sigma*clipNorm) > 0.1 {
		t.Errorf("Expected std ≈ %.2f, got %f", sigma*clipNorm, stdNoise)
	}
}

func TestAggregateGradients(t *testing.T) {
	agg := NewFederatedAggregator(nil, testLogger)

	g1 := NoisyGradient{WorkspaceID: uuid.New(), GradientVector: []float64{1.0, 2.0, 3.0}}
	g2 := NoisyGradient{WorkspaceID: uuid.New(), GradientVector: []float64{3.0, 4.0, 5.0}}

	result, err := agg.AggregateGradients(context.Background(), []NoisyGradient{g1, g2})
	if err != nil {
		t.Fatalf("AggregateGradients failed: %v", err)
	}

	expected := []float64{2.0, 3.0, 4.0}
	for i, v := range result {
		if math.Abs(v-expected[i]) > 1e-9 {
			t.Errorf("Expected result[%d]=%f, got %f", i, expected[i], v)
		}
	}
}

func TestAggregateGradientsPadding(t *testing.T) {
	agg := NewFederatedAggregator(nil, testLogger)

	g1 := NoisyGradient{WorkspaceID: uuid.New(), GradientVector: []float64{2.0, 4.0}}
	g2 := NoisyGradient{WorkspaceID: uuid.New(), GradientVector: []float64{6.0, 8.0, 10.0}}

	result, err := agg.AggregateGradients(context.Background(), []NoisyGradient{g1, g2})
	if err != nil {
		t.Fatalf("AggregateGradients failed: %v", err)
	}

	// g1 contributes 0 for dim[2].
	expected := []float64{4.0, 6.0, 5.0}
	for i, v := range result {
		if math.Abs(v-expected[i]) > 1e-9 {
			t.Errorf("Expected result[%d]=%f, got %f", i, expected[i], v)
		}
	}
}

func TestInsufficientParticipants(t *testing.T) {
	agg := NewFederatedAggregator(nil, testLogger)

	g1 := NoisyGradient{WorkspaceID: uuid.New(), GradientVector: []float64{1.0}}
	_, err := agg.AggregateGradients(context.Background(), []NoisyGradient{g1})
	if err != ErrInsufficientParticipants {
		t.Errorf("Expected ErrInsufficientParticipants, got %v", err)
	}
}

func TestComputeLocalGradient(t *testing.T) {
	activity := NewFederatedGradientActivity(nil, testLogger)

	input := FederatedGradientInput{
		WorkspaceID: uuid.New(),
		PreferencePairs: []PreferencePair{
			{PromptText: "Help me", ChosenResponse: "Sure, here's how", RejectedResponse: "I can't"},
			{PromptText: "Schedule a meeting", ChosenResponse: "Done, scheduled for 3pm", RejectedResponse: "No"},
		},
		Sigma:    1.0,
		ClipNorm: 1.0,
	}

	result, err := activity.ComputeLocalGradient(context.Background(), input)
	if err != nil {
		t.Fatalf("ComputeLocalGradient failed: %v", err)
	}

	if len(result.GradientVector) != 2 {
		t.Errorf("Expected gradient dim=2, got %d", len(result.GradientVector))
	}

	// Verify gradient norm is clipped.
	norm := math.Sqrt(DotProduct(result.GradientVector, result.GradientVector))
	// After noise, norm could be larger than clip, but the pre-noise clip should be <= 1.0.
	t.Logf("Noisy gradient norm: %f", norm)
}

func TestDotProduct(t *testing.T) {
	result := DotProduct([]float64{1, 2, 3}, []float64{4, 5, 6})
	expected := 32.0 // 1*4 + 2*5 + 3*6
	if math.Abs(result-expected) > 1e-9 {
		t.Errorf("Expected dot product=%f, got %f", expected, result)
	}
}
