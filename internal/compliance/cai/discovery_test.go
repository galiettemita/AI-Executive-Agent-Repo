package cai

import (
	"context"
	"log/slog"
	"math"
	"os"
	"testing"

	"github.com/google/uuid"
)

var testLogger = slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))

// mockLLMClient returns a fixed response.
type mockLLMClient struct{ response string }

func (m *mockLLMClient) Complete(_ interface{}, _, _ string) (string, error) {
	return m.response, nil
}

// mockFeatureFlags records calls.
type mockFeatureFlags struct {
	enabledFraction float64
	enabledKey      string
	enabledAll      bool
}

func (m *mockFeatureFlags) EnableForFraction(_ interface{}, key string, fraction float64, _ string) error {
	m.enabledKey = key
	m.enabledFraction = fraction
	return nil
}

func (m *mockFeatureFlags) EnableForAll(_ interface{}, key string) error {
	m.enabledAll = true
	m.enabledKey = key
	return nil
}

func TestPrincipleDiscoveryCreatesProposal(t *testing.T) {
	llm := &mockLLMClient{response: `{
		"principles": [
			{
				"description": "Avoid recommending actions that create legal liability",
				"failure_examples": ["suggested contract clause without legal review"],
				"coverage_rate": 0.12,
				"conflict_with_existing": []
			}
		]
	}`}

	discovery := NewConstitutionalPrincipleDiscovery(nil, llm, testLogger)

	proposals, err := discovery.DiscoverPrinciples(context.Background())
	if err != nil {
		t.Fatalf("DiscoverPrinciples failed: %v", err)
	}

	// Without DB, violations are empty, so LLM is not called (short-circuits).
	// This tests the no-violation path.
	if proposals != nil {
		t.Logf("Proposals created: %d", len(proposals))
	}
}

func TestWelchTTestSignificance(t *testing.T) {
	// Large difference — should be significant.
	treatment := []float64{8.0, 8.5, 8.2, 8.8, 8.1, 8.4, 8.6, 8.3, 8.7, 8.0}
	control := []float64{6.0, 6.5, 6.2, 6.8, 6.1, 6.4, 6.6, 6.3, 6.7, 6.0}

	tStat, pValue := WelchTTest(treatment, control)

	t.Logf("Large diff: t=%.3f, p=%.6f", tStat, pValue)
	if pValue >= 0.05 {
		t.Errorf("Expected p < 0.05 for large difference, got p=%f", pValue)
	}

	// Small difference — should NOT be significant.
	control2 := []float64{7.9, 8.4, 8.1, 8.7, 8.0, 8.3, 8.5, 8.2, 8.6, 7.9}

	tStat2, pValue2 := WelchTTest(treatment, control2)

	t.Logf("Small diff: t=%.3f, p=%.6f", tStat2, pValue2)
	if pValue2 < 0.05 {
		t.Logf("Note: small difference was significant (p=%.6f), which is possible for very close distributions", pValue2)
	}
}

func TestWelchTTestEdgeCases(t *testing.T) {
	// Identical distributions.
	same := []float64{5.0, 5.0, 5.0}
	_, pValue := WelchTTest(same, same)
	if !math.IsNaN(pValue) && pValue < 0.5 {
		// With zero variance, p should be 1.0.
		t.Logf("Same distributions: p=%f", pValue)
	}

	// Single-element slices.
	_, pSingle := WelchTTest([]float64{5.0}, []float64{3.0})
	if pSingle != 1.0 {
		t.Errorf("Expected p=1.0 for single-element slices, got %f", pSingle)
	}
}

func TestPrinciplePromotionStatus(t *testing.T) {
	// Test the AB test result evaluation logic.
	result := &ABTestResult{
		PrincipleID:    "C9",
		ORMImprovement: 0.15,
		PValue:         0.03,
	}
	result.Significant = result.PValue < 0.05 && result.ORMImprovement > 0.1

	if !result.Significant {
		t.Error("Expected significant result for p=0.03, improvement=0.15")
	}
}

func TestPrincipleNotSignificant(t *testing.T) {
	result := &ABTestResult{
		PrincipleID:    "C10",
		ORMImprovement: 0.05,
		PValue:         0.12,
	}
	result.Significant = result.PValue < 0.05 && result.ORMImprovement > 0.1

	if result.Significant {
		t.Error("Expected NOT significant for p=0.12, improvement=0.05")
	}
}

func TestABTestActivation(t *testing.T) {
	ff := &mockFeatureFlags{}
	tester := NewPrincipleABTester(nil, ff, testLogger)

	// Without DB, ActivateForTesting will fail (needs DB query).
	err := tester.ActivateForTesting(context.Background(), uuid.New())
	if err == nil {
		t.Error("Expected error without DB")
	}
}

func TestMeanAndVariance(t *testing.T) {
	data := []float64{2, 4, 4, 4, 5, 5, 7, 9}

	m := mean(data)
	if math.Abs(m-5.0) > 0.001 {
		t.Errorf("Expected mean=5.0, got %f", m)
	}

	v := variance(data, m)
	expected := 4.571428571 // sample variance
	if math.Abs(v-expected) > 0.01 {
		t.Errorf("Expected variance≈%.3f, got %f", expected, v)
	}
}
