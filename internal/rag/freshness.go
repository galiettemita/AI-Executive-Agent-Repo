package rag

import (
	"math"
	"sync"
	"time"
)

// FreshnessConfig controls how freshness affects scoring.
type FreshnessConfig struct {
	TemporalLambda float64 // weight for semantic score (1-lambda for temporal)
	MaxAgeDays     int     // documents older than this get zero temporal score
}

// FreshnessScorer scores documents with a combined semantic + temporal freshness score.
type FreshnessScorer struct {
	mu sync.Mutex
}

// NewFreshnessScorer creates a new FreshnessScorer.
func NewFreshnessScorer() *FreshnessScorer {
	return &FreshnessScorer{}
}

// DefaultFreshnessConfig returns a sensible default configuration.
func DefaultFreshnessConfig() FreshnessConfig {
	return FreshnessConfig{
		TemporalLambda: 0.7,
		MaxAgeDays:     30,
	}
}

// ScoreWithFreshness computes a combined score:
//
//	combined = lambda * semanticScore + (1 - lambda) * temporalDecay
//
// where temporalDecay decays exponentially with document age.
func (fs *FreshnessScorer) ScoreWithFreshness(semanticScore float64, documentAge time.Duration, config FreshnessConfig) float64 {
	fs.mu.Lock()
	defer fs.mu.Unlock()

	lambda := config.TemporalLambda
	if lambda < 0 {
		lambda = 0
	}
	if lambda > 1 {
		lambda = 1
	}

	maxAgeDays := config.MaxAgeDays
	if maxAgeDays <= 0 {
		maxAgeDays = 30
	}

	temporalDecay := computeTemporalDecay(documentAge, maxAgeDays)
	combined := lambda*semanticScore + (1-lambda)*temporalDecay

	return roundScore(combined)
}

// computeTemporalDecay returns a decay score from 1.0 (brand new) to 0.0 (expired).
// Uses exponential decay: e^(-3 * ageFraction) where ageFraction = age / maxAge.
func computeTemporalDecay(age time.Duration, maxAgeDays int) float64 {
	if age <= 0 {
		return 1.0
	}
	maxAge := time.Duration(maxAgeDays) * 24 * time.Hour
	if age >= maxAge {
		return 0.0
	}
	ageFraction := float64(age) / float64(maxAge)
	return math.Exp(-3 * ageFraction)
}
