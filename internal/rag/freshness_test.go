package rag

import (
	"testing"
	"time"
)

func TestFreshnessScoreNewDocument(t *testing.T) {
	t.Parallel()

	fs := NewFreshnessScorer()
	config := FreshnessConfig{TemporalLambda: 0.7, MaxAgeDays: 30}
	score := fs.ScoreWithFreshness(0.9, 0, config)
	// combined = 0.7*0.9 + 0.3*1.0 = 0.63 + 0.3 = 0.93
	if score < 0.92 || score > 0.94 {
		t.Fatalf("expected ~0.93 for new doc, got %f", score)
	}
}

func TestFreshnessScoreOldDocument(t *testing.T) {
	t.Parallel()

	fs := NewFreshnessScorer()
	config := FreshnessConfig{TemporalLambda: 0.7, MaxAgeDays: 30}
	age := 31 * 24 * time.Hour // older than max
	score := fs.ScoreWithFreshness(0.9, age, config)
	// combined = 0.7*0.9 + 0.3*0.0 = 0.63
	if score < 0.62 || score > 0.64 {
		t.Fatalf("expected ~0.63 for expired doc, got %f", score)
	}
}

func TestFreshnessScoreMidAgeDocument(t *testing.T) {
	t.Parallel()

	fs := NewFreshnessScorer()
	config := FreshnessConfig{TemporalLambda: 0.5, MaxAgeDays: 30}
	age := 15 * 24 * time.Hour // half max age
	score := fs.ScoreWithFreshness(0.8, age, config)
	// temporal_decay = e^(-3*0.5) = e^(-1.5) ≈ 0.2231
	// combined = 0.5*0.8 + 0.5*0.2231 ≈ 0.5116
	if score < 0.49 || score > 0.53 {
		t.Fatalf("expected ~0.51 for mid-age doc, got %f", score)
	}
}

func TestFreshnessScoreDefaultConfig(t *testing.T) {
	t.Parallel()

	config := DefaultFreshnessConfig()
	if config.TemporalLambda != 0.7 {
		t.Fatalf("expected default lambda 0.7, got %f", config.TemporalLambda)
	}
	if config.MaxAgeDays != 30 {
		t.Fatalf("expected default max age 30, got %d", config.MaxAgeDays)
	}
}

func TestFreshnessScoreLambdaBounds(t *testing.T) {
	t.Parallel()

	fs := NewFreshnessScorer()

	// Lambda = 1.0 means only semantic matters
	score := fs.ScoreWithFreshness(0.5, 0, FreshnessConfig{TemporalLambda: 1.0, MaxAgeDays: 30})
	if score < 0.49 || score > 0.51 {
		t.Fatalf("expected ~0.5 for lambda=1.0, got %f", score)
	}

	// Lambda = 0.0 means only temporal matters
	score = fs.ScoreWithFreshness(0.5, 0, FreshnessConfig{TemporalLambda: 0.0, MaxAgeDays: 30})
	if score < 0.99 || score > 1.01 {
		t.Fatalf("expected ~1.0 for lambda=0.0 with new doc, got %f", score)
	}
}
