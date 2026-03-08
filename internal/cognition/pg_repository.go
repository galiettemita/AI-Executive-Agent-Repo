package cognition

import (
	"context"
	"fmt"

	"github.com/brevio/brevio/internal/database"
	"github.com/jackc/pgx/v5"
)

// PgCaseRepository implements CaseRepository using PostgreSQL with pgvector.
type PgCaseRepository struct {
	db database.Querier
}

// NewPgCaseRepository creates a new PgCaseRepository.
func NewPgCaseRepository(db database.Querier) *PgCaseRepository {
	return &PgCaseRepository{db: db}
}

func (r *PgCaseRepository) Store(ctx context.Context, c *Case) error {
	_, err := r.db.Exec(ctx, `
		INSERT INTO cbr_cases (id, workspace_id, problem, solution, outcome, success_score, domain, features, embedding, created_at, reuse_count)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11)
		ON CONFLICT (id) DO UPDATE SET
			solution = EXCLUDED.solution,
			outcome = EXCLUDED.outcome,
			success_score = EXCLUDED.success_score,
			features = EXCLUDED.features,
			embedding = EXCLUDED.embedding`,
		c.ID, c.WorkspaceID, c.Problem, c.Solution, c.Outcome,
		c.SuccessScore, c.Domain, c.Features, c.Embedding, c.CreatedAt, c.ReuseCount)
	if err != nil {
		return fmt.Errorf("store case: %w", err)
	}
	return nil
}

func (r *PgCaseRepository) GetByID(ctx context.Context, id string) (*Case, error) {
	row := r.db.QueryRow(ctx, `
		SELECT id, workspace_id, problem, solution, outcome, success_score, domain, features, created_at, reuse_count
		FROM cbr_cases WHERE id = $1`, id)

	c := &Case{}
	err := row.Scan(&c.ID, &c.WorkspaceID, &c.Problem, &c.Solution, &c.Outcome,
		&c.SuccessScore, &c.Domain, &c.Features, &c.CreatedAt, &c.ReuseCount)
	if err == pgx.ErrNoRows {
		return nil, fmt.Errorf("case not found: %s", id)
	}
	if err != nil {
		return nil, fmt.Errorf("get case: %w", err)
	}
	return c, nil
}

func (r *PgCaseRepository) FindSimilarByEmbedding(ctx context.Context, workspaceID string, embedding []float32, limit int, minScore float64) ([]ScoredCase, error) {
	if limit <= 0 {
		limit = 10
	}
	rows, err := r.db.Query(ctx, `
		SELECT id, workspace_id, problem, solution, outcome, success_score, domain, features, created_at, reuse_count,
			   1 - (embedding <=> $1::vector) AS similarity
		FROM cbr_cases
		WHERE workspace_id = $2 AND 1 - (embedding <=> $1::vector) >= $3
		ORDER BY embedding <=> $1::vector
		LIMIT $4`, embedding, workspaceID, minScore, limit)
	if err != nil {
		return nil, fmt.Errorf("find similar cases: %w", err)
	}
	defer rows.Close()

	var results []ScoredCase
	for rows.Next() {
		var sc ScoredCase
		err := rows.Scan(&sc.Case.ID, &sc.Case.WorkspaceID, &sc.Case.Problem, &sc.Case.Solution,
			&sc.Case.Outcome, &sc.Case.SuccessScore, &sc.Case.Domain, &sc.Case.Features,
			&sc.Case.CreatedAt, &sc.Case.ReuseCount, &sc.Similarity)
		if err != nil {
			return nil, fmt.Errorf("scan similar case: %w", err)
		}
		results = append(results, sc)
	}
	return results, rows.Err()
}

func (r *PgCaseRepository) IncrementReuse(ctx context.Context, id string) error {
	tag, err := r.db.Exec(ctx, `UPDATE cbr_cases SET reuse_count = reuse_count + 1 WHERE id = $1`, id)
	if err != nil {
		return fmt.Errorf("increment reuse: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return fmt.Errorf("case not found: %s", id)
	}
	return nil
}

func (r *PgCaseRepository) DeleteByMinReuse(ctx context.Context, minReuses int) (int, error) {
	tag, err := r.db.Exec(ctx, `DELETE FROM cbr_cases WHERE reuse_count < $1`, minReuses)
	if err != nil {
		return 0, fmt.Errorf("delete low reuse cases: %w", err)
	}
	return int(tag.RowsAffected()), nil
}
