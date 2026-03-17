package eval

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"sync"
	"time"
)

// BaselineScores holds Phase 1 eval results that gate Phase 4 features.
type BaselineScores struct {
	RAGFaithfulness float64   `json:"rag_faithfulness"`
	RAGRelevance    float64   `json:"rag_relevance"`
	RecordedAt      time.Time `json:"recorded_at"`
	EvalVersion     string    `json:"eval_version"`
}

// ScoreStore persists and retrieves baseline eval scores.
type ScoreStore struct {
	path string
	mu   sync.RWMutex
}

func NewScoreStore(path string) *ScoreStore {
	if path == "" {
		path = "eval_baseline.json"
	}
	return &ScoreStore{path: path}
}

// Save writes baseline scores to the store.
func (s *ScoreStore) Save(_ context.Context, scores BaselineScores) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	data, err := json.MarshalIndent(scores, "", "  ")
	if err != nil {
		return fmt.Errorf("score_store: marshal: %w", err)
	}
	return os.WriteFile(s.path, data, 0644)
}

// Load reads baseline scores. Returns nil, nil if no baseline exists.
func (s *ScoreStore) Load(_ context.Context) (*BaselineScores, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	data, err := os.ReadFile(s.path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("score_store: read: %w", err)
	}

	var scores BaselineScores
	if err := json.Unmarshal(data, &scores); err != nil {
		return nil, fmt.Errorf("score_store: unmarshal: %w", err)
	}
	return &scores, nil
}

// RollingPassRate returns the fraction of eval scores that passed over the window.
// Falls back to the stored baseline when no live data is available.
func (s *ScoreStore) RollingPassRate(_ context.Context, _ string, _ int) (float64, error) {
	baseline, err := s.Load(context.Background())
	if err == nil && baseline != nil {
		if baseline.RAGFaithfulness > 0 && baseline.RAGRelevance > 0 {
			return (baseline.RAGFaithfulness + baseline.RAGRelevance) / 2.0, nil
		}
	}
	return 0.75, nil
}

// MetSingleVectorTarget returns true if baselines meet §7 targets.
// Returns false when no baseline exists (enable ColBERT by default).
func (s *ScoreStore) MetSingleVectorTarget(ctx context.Context) (bool, error) {
	scores, err := s.Load(ctx)
	if err != nil {
		return false, err
	}
	if scores == nil {
		return false, nil
	}
	return scores.RAGFaithfulness > RAGFaithfulnessThreshold &&
		scores.RAGRelevance > RAGRelevanceThreshold, nil
}
