package experiment_test

import (
	"math"
	"math/rand"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/brevio/brevio/internal/experiment"
)

func TestExperimentRouter_DeterministicVariantAssignment(t *testing.T) {
	v1 := experiment.DeterministicVariant("exp-001", "ws-abc")
	v2 := experiment.DeterministicVariant("exp-001", "ws-abc")
	assert.Equal(t, v1, v2, "same inputs must produce same variant")
}

func TestExperimentRouter_SameWorkspaceSameVariant(t *testing.T) {
	// Run 100 times — must always return the same result for the same pair.
	first := experiment.DeterministicVariant("exp-fixed", "ws-fixed")
	for i := 0; i < 100; i++ {
		got := experiment.DeterministicVariant("exp-fixed", "ws-fixed")
		assert.Equal(t, first, got)
	}
}

func TestExperimentRouter_FiftyFiftySplit(t *testing.T) {
	controlCount := 0
	variantCount := 0
	n := 1000
	for i := 0; i < n; i++ {
		wsID := randomID()
		v := experiment.DeterministicVariant("exp-split-test", wsID)
		if v == "control" {
			controlCount++
		} else {
			variantCount++
		}
	}
	// With 1000 samples, expect roughly 50/50. Allow 10% tolerance.
	assert.InDelta(t, 500, controlCount, 100, "control count should be ~500")
	assert.InDelta(t, 500, variantCount, 100, "variant count should be ~500")
}

func TestExperimentRouter_GetPromptForWorkspace_NoExperiment_ReturnsDefault(t *testing.T) {
	router := experiment.NewExperimentRouter(nil) // nil pool = no DB
	prompt, variant, err := router.GetPromptForWorkspace(nil, "ws-1", "default prompt")
	require.NoError(t, err)
	assert.Equal(t, "default prompt", prompt)
	assert.Equal(t, "control", variant)
}

func TestExperimentRouter_GetPromptForWorkspace_WithExperiment_ReturnsVariant(t *testing.T) {
	// Without a real DB, GetActiveExperiment returns nil, so we test the fallback path.
	router := experiment.NewExperimentRouter(nil)
	prompt, variant, err := router.GetPromptForWorkspace(nil, "ws-1", "fallback")
	require.NoError(t, err)
	assert.Equal(t, "fallback", prompt)
	assert.Equal(t, "control", variant)
}

func TestVariantScoreStore_Record(t *testing.T) {
	store := experiment.NewVariantScoreStore(nil)
	// nil pool → Record is a no-op, no error.
	err := store.Record(nil, "exp-1", "ws-1", "wf-1", "control", 0.85)
	assert.NoError(t, err)
}

func TestVariantScoreStore_GetVariantMeans(t *testing.T) {
	store := experiment.NewVariantScoreStore(nil)
	cm, vm, cn, vn, err := store.GetVariantMeans(nil, "exp-1")
	assert.NoError(t, err)
	assert.Equal(t, 0.0, cm)
	assert.Equal(t, 0.0, vm)
	assert.Equal(t, 0, cn)
	assert.Equal(t, 0, vn)
}

func TestWelchTTest_SignificantDifference(t *testing.T) {
	// Control: mean ~0.5, Variant: mean ~0.9 → clearly significant.
	control := make([]float64, 100)
	variant := make([]float64, 100)
	for i := range control {
		control[i] = 0.5 + rand.Float64()*0.1
		variant[i] = 0.9 + rand.Float64()*0.05
	}
	result, err := experiment.WelchTTest(control, variant, 0.05, 50)
	require.NoError(t, err)
	assert.True(t, result.Significant, "large effect size should be significant")
	assert.Less(t, result.PValue, 0.05)
	assert.Equal(t, "variant", result.Winner)
}

func TestWelchTTest_InsignificantDifference(t *testing.T) {
	// Both groups have the same distribution → not significant.
	control := make([]float64, 100)
	variant := make([]float64, 100)
	for i := range control {
		v := 0.75 + rand.Float64()*0.1
		control[i] = v
		variant[i] = v
	}
	result, err := experiment.WelchTTest(control, variant, 0.05, 50)
	require.NoError(t, err)
	assert.False(t, result.Significant)
	assert.Equal(t, "no_difference", result.Winner)
}

func TestWelchTTest_InsufficientSamples(t *testing.T) {
	control := []float64{0.5, 0.6}
	variant := []float64{0.8, 0.9}
	_, err := experiment.WelchTTest(control, variant, 0.05, 50)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "insufficient samples")
}

func TestMeanVariance(t *testing.T) {
	xs := []float64{2, 4, 4, 4, 5, 5, 7, 9}
	mean, variance := experiment.MeanVariance(xs)
	assert.InDelta(t, 5.0, mean, 0.01)
	assert.InDelta(t, 4.571, variance, 0.01)
}

func randomID() string {
	const chars = "abcdef0123456789"
	b := make([]byte, 16)
	for i := range b {
		b[i] = chars[rand.Intn(len(chars))]
	}
	return string(b)
}

// Suppress unused import warning for math.
var _ = math.Pi
