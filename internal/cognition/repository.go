package cognition

import "context"

// CaseRepository persists cases for case-based reasoning.
type CaseRepository interface {
	Store(ctx context.Context, c *Case) error
	GetByID(ctx context.Context, id string) (*Case, error)
	FindSimilarByEmbedding(ctx context.Context, workspaceID string, embedding []float32, limit int, minScore float64) ([]ScoredCase, error)
	IncrementReuse(ctx context.Context, id string) error
	DeleteByMinReuse(ctx context.Context, minReuses int) (int, error)
}
