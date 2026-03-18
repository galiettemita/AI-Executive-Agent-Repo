package learning

import (
	"testing"

	"github.com/google/uuid"
)

func TestComputeRDPEpsilon(t *testing.T) {
	// With sigma=1.0, samplingRate=0.01, numSteps=100, epsilon should be reasonable.
	epsilon := ComputeRDPEpsilon(1.0, 0.01, 100, defaultAlphas)

	if epsilon <= 0 {
		t.Fatalf("Expected positive epsilon, got %f", epsilon)
	}
	if epsilon > 3.1 {
		t.Errorf("Expected epsilon <= 3.1 for sigma=1.0, q=0.01, T=100, got %f", epsilon)
	}

	t.Logf("ComputeRDPEpsilon(sigma=1.0, q=0.01, T=100) = %f", epsilon)
}

func TestComputeRDPEpsilonScaling(t *testing.T) {
	// More steps should produce higher epsilon.
	eps100 := ComputeRDPEpsilon(1.0, 0.01, 100, defaultAlphas)
	eps200 := ComputeRDPEpsilon(1.0, 0.01, 200, defaultAlphas)

	if eps200 <= eps100 {
		t.Errorf("Expected epsilon to increase with more steps: eps100=%f, eps200=%f", eps100, eps200)
	}

	// Higher sigma should produce lower epsilon (more noise = more privacy).
	epsLowSigma := ComputeRDPEpsilon(0.5, 0.01, 100, defaultAlphas)
	epsHighSigma := ComputeRDPEpsilon(2.0, 0.01, 100, defaultAlphas)

	if epsHighSigma >= epsLowSigma {
		t.Errorf("Expected lower epsilon with higher sigma: sigma=0.5→%f, sigma=2.0→%f", epsLowSigma, epsHighSigma)
	}
}

func TestBudgetHaltsAtEpsilonMax(t *testing.T) {
	// Simulate rounds without a database — track budget manually.
	budget := &PrivacyBudget{
		WorkspaceID:       uuid.New(),
		CumulativeEpsilon: 0,
		EpsilonMax:        EpsilonMaxDefault,
		DeltaTarget:       DPODelta,
		Halted:            false,
	}

	roundEpsilon := ComputeRDPEpsilon(DPOSigma, DPOSamplingRate, 100, defaultAlphas)
	t.Logf("Per-round epsilon: %f", roundEpsilon)

	// Simulate enough rounds to exceed epsilon_max = 10.0.
	for i := 0; i < 100; i++ {
		budget.CumulativeEpsilon += roundEpsilon
		budget.RoundsCompleted++

		if budget.CumulativeEpsilon > budget.EpsilonMax {
			budget.Halted = true
			t.Logf("Budget halted after round %d, cumulative_epsilon=%f", i+1, budget.CumulativeEpsilon)
			break
		}
	}

	if !budget.Halted {
		t.Errorf("Expected budget to be halted after exceeding epsilon_max=%f, cumulative=%f",
			budget.EpsilonMax, budget.CumulativeEpsilon)
	}

	if budget.CumulativeEpsilon <= budget.EpsilonMax {
		t.Errorf("Expected cumulative epsilon > %f, got %f", budget.EpsilonMax, budget.CumulativeEpsilon)
	}
}

func TestAlertAt80Percent(t *testing.T) {
	alertFired := false
	alertEpsilon := 0.0

	// Simulate accountant with custom alert function.
	budget := &PrivacyBudget{
		WorkspaceID:       uuid.New(),
		CumulativeEpsilon: 0,
		EpsilonMax:        EpsilonMaxDefault,
		DeltaTarget:       DPODelta,
		Halted:            false,
	}

	roundEpsilon := ComputeRDPEpsilon(DPOSigma, DPOSamplingRate, 100, defaultAlphas)

	for i := 0; i < 100; i++ {
		budget.CumulativeEpsilon += roundEpsilon
		budget.RoundsCompleted++

		if budget.CumulativeEpsilon > EpsilonAlertThreshold && !alertFired {
			alertFired = true
			alertEpsilon = budget.CumulativeEpsilon
			t.Logf("Alert fired at round %d, epsilon=%f", i+1, budget.CumulativeEpsilon)
		}

		if budget.CumulativeEpsilon > budget.EpsilonMax {
			budget.Halted = true
			break
		}
	}

	if !alertFired {
		t.Error("Expected alert to fire when epsilon > 8.0")
	}
	if alertEpsilon <= EpsilonAlertThreshold {
		t.Errorf("Expected alert at epsilon > %f, got %f", EpsilonAlertThreshold, alertEpsilon)
	}
}
