package learning

import (
	"context"
	"crypto/rand"
	"encoding/binary"
	"fmt"
	"log/slog"
	"math"
	mathrand "math/rand"
	"regexp"
	"strconv"
	"strings"
)

// OutputPerturber adds calibrated Laplace noise to numerical values in AI
// responses when the workspace operates in a sensitive domain.
type OutputPerturber struct {
	rng    *mathrand.Rand
	logger *slog.Logger
}

// NewOutputPerturber creates a perturber seeded from crypto/rand.
func NewOutputPerturber(logger *slog.Logger) *OutputPerturber {
	var seed int64
	if err := binary.Read(rand.Reader, binary.LittleEndian, &seed); err != nil {
		seed = 42 // fallback for environments without /dev/urandom
	}
	return &OutputPerturber{
		rng:    mathrand.New(mathrand.NewSource(seed)),
		logger: logger,
	}
}

// PerturbNumerical adds Laplace noise to a value.
//
//	scale = sensitivity / epsilon
//	noise ~ Laplace(0, scale)
func (p *OutputPerturber) PerturbNumerical(value float64, sensitivity float64, epsilon float64) float64 {
	if epsilon <= 0 {
		return value
	}
	scale := sensitivity / epsilon
	noise := p.laplaceNoise(scale)
	return value + noise
}

// PerturbIfSensitive adds noise only for sensitive domains (financial, health).
func (p *OutputPerturber) PerturbIfSensitive(_ context.Context, domain string, value float64, sensitivity float64) float64 {
	if isSensitiveDomain(domain) {
		return p.PerturbNumerical(value, sensitivity, 3.0)
	}
	return value
}

var numberRegex = regexp.MustCompile(`\d+\.?\d*`)

// PerturbResponseNumbers finds all numeric values in responseText and perturbs
// them with Laplace noise if the domain is sensitive. Perturbed values are
// rounded to the same number of decimal places as the original.
func (p *OutputPerturber) PerturbResponseNumbers(_ context.Context, domain string, responseText string) string {
	if !isSensitiveDomain(domain) {
		return responseText
	}

	return numberRegex.ReplaceAllStringFunc(responseText, func(match string) string {
		val, err := strconv.ParseFloat(match, 64)
		if err != nil {
			return match
		}

		perturbed := p.PerturbNumerical(val, 1.0, 3.0)

		// Preserve decimal places.
		decPlaces := countDecimalPlaces(match)
		format := fmt.Sprintf("%%.%df", decPlaces)
		return fmt.Sprintf(format, perturbed)
	})
}

func isSensitiveDomain(domain string) bool {
	d := strings.ToLower(domain)
	return d == "financial" || d == "health"
}

func countDecimalPlaces(s string) int {
	idx := strings.Index(s, ".")
	if idx < 0 {
		return 0
	}
	return len(s) - idx - 1
}

// laplaceNoise generates a sample from the Laplace(0, scale) distribution.
func (p *OutputPerturber) laplaceNoise(scale float64) float64 {
	u := p.rng.Float64() - 0.5
	sign := 1.0
	if u < 0 {
		sign = -1.0
	}
	return -scale * sign * math.Log(1-2*math.Abs(u))
}
