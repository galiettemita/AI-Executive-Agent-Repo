package eval

import (
	"context"
	"fmt"
	"math/rand"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

const defaultSampleRate = 0.05

// WorkflowRunRecord is a lightweight record of a completed workflow for sampling.
type WorkflowRunRecord struct {
	WorkflowID   string
	WorkspaceID  string
	Intent       string
	Steps        []string
	Results      []string
	QualityScore float64
	CompletedAt  time.Time
}

// EvalJudge re-evaluates a completed workflow and returns a quality score 0.0–1.0.
type EvalJudge interface {
	Score(ctx context.Context, workspaceID, intent string, steps, results []string) (float64, error)
}

// ProductionScoreStore persists per-workflow quality scores for trend analysis.
type ProductionScoreStore interface {
	RecordScore(ctx context.Context, workflowID, workspaceID string, score float64, sampledAt time.Time) error
	GetRolling7DayPassRate(ctx context.Context) (float64, error)
}

// ProductionEvalSampler samples completed workflows and re-scores them.
type ProductionEvalSampler struct {
	pool       *pgxpool.Pool
	judge      EvalJudge
	scoreStore ProductionScoreStore
	sampleRate float64
}

// NewProductionEvalSampler creates a sampler with the given sample rate (0.0–1.0).
func NewProductionEvalSampler(pool *pgxpool.Pool, judge EvalJudge, store ProductionScoreStore, sampleRate float64) *ProductionEvalSampler {
	if sampleRate <= 0 || sampleRate > 1 {
		sampleRate = defaultSampleRate
	}
	return &ProductionEvalSampler{pool: pool, judge: judge, scoreStore: store, sampleRate: sampleRate}
}

// SampleAndScore fetches recently completed workflows, samples them, re-scores each,
// and persists results. Returns the sample count and rolling 7-day pass rate.
func (s *ProductionEvalSampler) SampleAndScore(ctx context.Context) (sampleCount int, passRate float64, err error) {
	runs, err := s.fetchRecentRuns(ctx, 500)
	if err != nil {
		return 0, 0, fmt.Errorf("production_sampler: fetch: %w", err)
	}

	for _, run := range runs {
		if rand.Float64() >= s.sampleRate {
			continue
		}
		score, scoreErr := s.judge.Score(ctx, run.WorkspaceID, run.Intent, run.Steps, run.Results)
		if scoreErr != nil {
			continue
		}
		_ = s.scoreStore.RecordScore(ctx, run.WorkflowID, run.WorkspaceID, score, time.Now().UTC())
		sampleCount++
	}

	passRate, err = s.scoreStore.GetRolling7DayPassRate(ctx)
	return sampleCount, passRate, err
}

func (s *ProductionEvalSampler) fetchRecentRuns(ctx context.Context, limit int) ([]WorkflowRunRecord, error) {
	if s.pool == nil {
		return nil, nil
	}
	rows, err := s.pool.Query(ctx, `
		SELECT workflow_id, workspace_id, intent, quality_score, completed_at
		FROM workflow_runs
		WHERE completed_at >= NOW() - INTERVAL '1 hour'
		  AND status = 'completed'
		ORDER BY completed_at DESC
		LIMIT $1
	`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var runs []WorkflowRunRecord
	for rows.Next() {
		var r WorkflowRunRecord
		if err := rows.Scan(&r.WorkflowID, &r.WorkspaceID, &r.Intent, &r.QualityScore, &r.CompletedAt); err != nil {
			continue
		}
		runs = append(runs, r)
	}
	return runs, rows.Err()
}
