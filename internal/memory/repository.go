package memory

import "context"

// ItemRepository persists memory items.
type ItemRepository interface {
	Store(ctx context.Context, item *Item) error
	GetByID(ctx context.Context, id string) (*Item, error)
	ListByWorkspace(ctx context.Context, workspaceID string, limit int) ([]Item, error)
	UpdateStatus(ctx context.Context, id string, status string) error
	FindSimilarByEmbedding(ctx context.Context, workspaceID string, embedding []float32, limit int, minScore float64) ([]ScoredItem, error)
	Delete(ctx context.Context, id string) error
}

// ScoredItem pairs an item with a similarity score.
type ScoredItem struct {
	Item       Item    `json:"item"`
	Similarity float64 `json:"similarity"`
}
