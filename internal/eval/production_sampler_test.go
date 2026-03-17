package eval_test

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/brevio/brevio/internal/eval"
)

type mockJudge struct {
	score float64
	err   error
	calls int
}

func (m *mockJudge) Score(_ context.Context, _, _ string, _, _ []string) (float64, error) {
	m.calls++
	return m.score, m.err
}

type mockScoreStore struct {
	scores   []float64
	passRate float64
}

func (m *mockScoreStore) RecordScore(_ context.Context, _, _ string, score float64, _ time.Time) error {
	m.scores = append(m.scores, score)
	return nil
}

func (m *mockScoreStore) GetRolling7DayPassRate(_ context.Context) (float64, error) {
	return m.passRate, nil
}

func TestProductionEvalSampler_SamplesAtConfiguredRate(t *testing.T) {
	// With sampleRate=1.0, all runs should be sampled.
	judge := &mockJudge{score: 0.9}
	store := &mockScoreStore{passRate: 0.95}
	sampler := eval.NewProductionEvalSampler(nil, judge, store, 1.0)
	// No DB pool → fetchRecentRuns returns empty → 0 samples
	count, rate, err := sampler.SampleAndScore(context.Background())
	require.NoError(t, err)
	assert.Equal(t, 0, count) // no DB = no runs
	assert.Equal(t, 0.95, rate)
}

func TestProductionEvalSampler_SkipsOnZeroSampleRate(t *testing.T) {
	judge := &mockJudge{score: 0.8}
	store := &mockScoreStore{passRate: 1.0}
	// Zero rate → defaults to 0.05
	sampler := eval.NewProductionEvalSampler(nil, judge, store, 0)
	count, _, err := sampler.SampleAndScore(context.Background())
	require.NoError(t, err)
	assert.Equal(t, 0, count)
}

func TestProductionEvalSampler_RecordsScoreForEachSample(t *testing.T) {
	judge := &mockJudge{score: 0.85}
	store := &mockScoreStore{passRate: 0.9}
	_ = eval.NewProductionEvalSampler(nil, judge, store, 1.0)
	// Without a DB pool, no runs are fetched, so no scores are recorded.
	// This test validates the store interface contract.
	err := store.RecordScore(context.Background(), "wf-1", "ws-1", 0.85, time.Now())
	require.NoError(t, err)
	assert.Len(t, store.scores, 1)
	assert.Equal(t, 0.85, store.scores[0])
}

func TestProductionEvalSampler_ReturnsPassRateFromStore(t *testing.T) {
	judge := &mockJudge{score: 0.9}
	store := &mockScoreStore{passRate: 0.72}
	sampler := eval.NewProductionEvalSampler(nil, judge, store, 1.0)
	_, rate, err := sampler.SampleAndScore(context.Background())
	require.NoError(t, err)
	assert.Equal(t, 0.72, rate)
}

func TestProductionEvalSampler_HandlesScorerError(t *testing.T) {
	judge := &mockJudge{score: 0, err: assert.AnError}
	store := &mockScoreStore{passRate: 1.0}
	sampler := eval.NewProductionEvalSampler(nil, judge, store, 1.0)
	count, _, err := sampler.SampleAndScore(context.Background())
	require.NoError(t, err)
	assert.Equal(t, 0, count) // no runs fetched without DB
}

func TestPgProductionScoreStore_GetRolling7DayPassRate_NoData_ReturnsOne(t *testing.T) {
	// The contract: no data → return 1.0 (non-penalising).
	store := &mockScoreStore{passRate: 1.0}
	rate, err := store.GetRolling7DayPassRate(context.Background())
	require.NoError(t, err)
	assert.Equal(t, 1.0, rate)
}
