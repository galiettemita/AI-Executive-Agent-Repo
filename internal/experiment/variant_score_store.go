package experiment

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"
)

// VariantScoreStore persists quality scores per experiment variant.
type VariantScoreStore struct {
	pool *pgxpool.Pool
}

func NewVariantScoreStore(pool *pgxpool.Pool) *VariantScoreStore {
	return &VariantScoreStore{pool: pool}
}

// Record stores one quality score for a workflow in an experiment.
func (s *VariantScoreStore) Record(ctx context.Context, experimentID, workspaceID, workflowID, variant string, score float64) error {
	if s.pool == nil {
		return nil
	}
	_, err := s.pool.Exec(ctx, `
		INSERT INTO experiment_scores (experiment_id, workspace_id, workflow_id, variant, quality_score)
		VALUES ($1,$2,$3,$4,$5)
		ON CONFLICT (experiment_id, workflow_id) DO NOTHING
	`, experimentID, workspaceID, workflowID, variant, score)
	return err
}

// GetVariantScores returns all individual scores for a variant.
func (s *VariantScoreStore) GetVariantScores(ctx context.Context, experimentID, variant string) ([]float64, error) {
	if s.pool == nil {
		return nil, nil
	}
	rows, err := s.pool.Query(ctx, `
		SELECT quality_score FROM experiment_scores
		WHERE experiment_id = $1 AND variant = $2
	`, experimentID, variant)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var scores []float64
	for rows.Next() {
		var sc float64
		if err := rows.Scan(&sc); err != nil {
			continue
		}
		scores = append(scores, sc)
	}
	return scores, nil
}

// GetVariantMeans returns the mean quality score for each variant.
func (s *VariantScoreStore) GetVariantMeans(ctx context.Context, experimentID string) (controlMean, variantMean float64, controlN, variantN int, err error) {
	if s.pool == nil {
		return 0, 0, 0, 0, nil
	}
	rows, err := s.pool.Query(ctx, `
		SELECT variant, AVG(quality_score), COUNT(*)
		FROM experiment_scores WHERE experiment_id = $1 GROUP BY variant
	`, experimentID)
	if err != nil {
		return 0, 0, 0, 0, fmt.Errorf("get_variant_means: %w", err)
	}
	defer rows.Close()

	for rows.Next() {
		var variant string
		var mean float64
		var n int
		if err := rows.Scan(&variant, &mean, &n); err != nil {
			continue
		}
		if variant == "control" {
			controlMean, controlN = mean, n
		} else {
			variantMean, variantN = mean, n
		}
	}
	return
}
