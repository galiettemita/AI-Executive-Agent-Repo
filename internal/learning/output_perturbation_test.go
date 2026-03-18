package learning

import (
	"context"
	"log/slog"
	"math"
	"os"
	"testing"
)

func TestPerturbNumerical(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))
	p := NewOutputPerturber(logger)

	// Run 1000 iterations; average perturbation should be approximately 0
	// (mean of Laplace(0, scale) = 0).
	const n = 1000
	total := 0.0
	for i := 0; i < n; i++ {
		perturbed := p.PerturbNumerical(100.0, 1.0, 3.0)
		total += perturbed - 100.0
	}

	meanNoise := total / float64(n)

	// Mean should be approximately 0. Allow ±1.0 for 1000 samples with scale=1/3.
	if math.Abs(meanNoise) > 1.0 {
		t.Errorf("Expected mean noise ≈ 0, got %f (over %d iterations)", meanNoise, n)
	}

	t.Logf("Mean Laplace noise over %d iterations: %f", n, meanNoise)
}

func TestPerturbNumericalNonZeroNoise(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))
	p := NewOutputPerturber(logger)

	// At least some perturbations should differ from the original value.
	changed := 0
	for i := 0; i < 100; i++ {
		perturbed := p.PerturbNumerical(42.0, 1.0, 3.0)
		if perturbed != 42.0 {
			changed++
		}
	}

	if changed == 0 {
		t.Error("Expected at least some values to be perturbed")
	}
}

func TestFinancialDomainPerturbed(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))
	p := NewOutputPerturber(logger)

	original := 1000.0
	perturbed := p.PerturbIfSensitive(context.Background(), "financial", original, 1.0)

	// The value should be different (statistically, with overwhelming probability).
	// Run a few times to be safe.
	allSame := true
	for i := 0; i < 10; i++ {
		v := p.PerturbIfSensitive(context.Background(), "financial", original, 1.0)
		if v != original {
			allSame = false
			break
		}
	}

	if allSame {
		t.Error("Expected financial domain values to be perturbed")
	}

	_ = perturbed // used above
}

func TestHealthDomainPerturbed(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))
	p := NewOutputPerturber(logger)

	allSame := true
	for i := 0; i < 10; i++ {
		v := p.PerturbIfSensitive(context.Background(), "health", 100.0, 1.0)
		if v != 100.0 {
			allSame = false
			break
		}
	}

	if allSame {
		t.Error("Expected health domain values to be perturbed")
	}
}

func TestNonSensitiveDomainUnchanged(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))
	p := NewOutputPerturber(logger)

	original := 42.0
	for _, domain := range []string{"general", "marketing", "engineering", ""} {
		result := p.PerturbIfSensitive(context.Background(), domain, original, 1.0)
		if result != original {
			t.Errorf("Expected non-sensitive domain %q to return original value %f, got %f", domain, original, result)
		}
	}
}

func TestPerturbResponseNumbers(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))
	p := NewOutputPerturber(logger)

	text := "The revenue was 1500.50 and costs were 800.25."

	// Financial domain should perturb numbers.
	perturbed := p.PerturbResponseNumbers(context.Background(), "financial", text)
	if perturbed == text {
		// Very unlikely but possible; try a few times.
		for i := 0; i < 5; i++ {
			perturbed = p.PerturbResponseNumbers(context.Background(), "financial", text)
			if perturbed != text {
				break
			}
		}
	}

	t.Logf("Original:  %s", text)
	t.Logf("Perturbed: %s", perturbed)

	// Non-sensitive domain should not perturb.
	unchanged := p.PerturbResponseNumbers(context.Background(), "general", text)
	if unchanged != text {
		t.Errorf("Expected non-sensitive domain to leave text unchanged.\nGot: %s\nExpected: %s", unchanged, text)
	}
}
