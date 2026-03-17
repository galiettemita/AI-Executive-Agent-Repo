package eval

import (
	"context"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

// PgProductionScoreStore persists sampled quality scores to Postgres.
type PgProductionScoreStore struct {
	pool *pgxpool.Pool
}

func NewPgProductionScoreStore(pool *pgxpool.Pool) *PgProductionScoreStore {
	return &PgProductionScoreStore{pool: pool}
}

func (s *PgProductionScoreStore) RecordScore(ctx context.Context, workflowID, workspaceID string, score float64, sampledAt time.Time) error {
	_, err := s.pool.Exec(ctx, `
		INSERT INTO production_eval_scores (workflow_id, workspace_id, quality_score, sampled_at)
		VALUES ($1, $2, $3, $4)
		ON CONFLICT (workflow_id) DO UPDATE SET quality_score = EXCLUDED.quality_score
	`, workflowID, workspaceID, score, sampledAt)
	return err
}

// GetRolling7DayPassRate returns the fraction of sampled workflows with quality_score >= 0.75.
// Returns 1.0 when no data exists.
func (s *PgProductionScoreStore) GetRolling7DayPassRate(ctx context.Context) (float64, error) {
	var total, passed int
	err := s.pool.QueryRow(ctx, `
		SELECT
			COUNT(*) AS total,
			COUNT(*) FILTER (WHERE quality_score >= 0.75) AS passed
		FROM production_eval_scores
		WHERE sampled_at >= NOW() - INTERVAL '7 days'
	`).Scan(&total, &passed)
	if err != nil {
		return 1.0, err
	}
	if total == 0 {
		return 1.0, nil
	}
	return float64(passed) / float64(total), nil
}
