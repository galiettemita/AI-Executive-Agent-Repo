package rag

import (
	"context"
	"fmt"
	"time"

	"github.com/brevio/brevio/internal/database"
)

// ChunkSpecRow represents a persisted chunking specification.
type ChunkSpecRow struct {
	ID             string    `json:"id"`
	WorkspaceID    string    `json:"workspace_id"`
	CollectionID   string    `json:"collection_id"`
	ChunkStrategy  string    `json:"chunk_strategy"`
	ChunkSize      int       `json:"chunk_size"`
	ChunkOverlap   int       `json:"chunk_overlap"`
	EmbeddingModel string    `json:"embedding_model"`
	Dimensions     int       `json:"dimensions"`
	CreatedAt      time.Time `json:"created_at"`
	UpdatedAt      time.Time `json:"updated_at"`
}

// ChunkSpecRepository persists embedding chunk specifications.
type ChunkSpecRepository interface {
	UpsertChunkSpec(ctx context.Context, row ChunkSpecRow) error
	GetChunkSpec(ctx context.Context, workspaceID, collectionID string) (*ChunkSpecRow, error)
	ListChunkSpecs(ctx context.Context, workspaceID string) ([]ChunkSpecRow, error)
}

// DeterministicEmbeddingProvider is a test stub that returns deterministic
// embeddings for contract tests. In production, use OpenAIEmbeddingProvider.
type DeterministicEmbeddingProvider struct {
	dims int
}

// NewDeterministicEmbeddingProvider creates a deterministic test stub.
func NewDeterministicEmbeddingProvider(dims int) *DeterministicEmbeddingProvider {
	if dims <= 0 {
		dims = 1536
	}
	return &DeterministicEmbeddingProvider{dims: dims}
}

// Embed returns deterministic embeddings based on text hash.
func (d *DeterministicEmbeddingProvider) Embed(_ context.Context, texts []string) ([][]float32, error) {
	result := make([][]float32, len(texts))
	for i, text := range texts {
		vec := make([]float32, d.dims)
		for j := 0; j < d.dims && j < len(text); j++ {
			vec[j] = float32(text[j]) / 255.0
		}
		// Normalize for cosine similarity.
		var norm float32
		for _, v := range vec {
			norm += v * v
		}
		if norm > 0 {
			invNorm := 1.0 / float32(norm)
			for j := range vec {
				vec[j] *= invNorm
			}
		}
		result[i] = vec
	}
	return result, nil
}

// Dimensions returns the embedding vector size.
func (d *DeterministicEmbeddingProvider) Dimensions() int { return d.dims }

// PgChunkSpecRepository implements ChunkSpecRepository backed by pgx.
type PgChunkSpecRepository struct {
	q database.Querier
}

// NewPgChunkSpecRepository creates a new PgChunkSpecRepository.
func NewPgChunkSpecRepository(q database.Querier) *PgChunkSpecRepository {
	return &PgChunkSpecRepository{q: q}
}

// UpsertChunkSpec creates or updates a chunk specification.
func (r *PgChunkSpecRepository) UpsertChunkSpec(ctx context.Context, row ChunkSpecRow) error {
	_, err := r.q.Exec(ctx,
		`INSERT INTO embedding_chunk_specs (workspace_id, collection_id, chunk_strategy, chunk_size, chunk_overlap, embedding_model, dimensions)
		 VALUES ($1::uuid, $2::uuid, $3, $4, $5, $6, $7)
		 ON CONFLICT (workspace_id, collection_id) DO UPDATE SET
		   chunk_strategy = EXCLUDED.chunk_strategy,
		   chunk_size = EXCLUDED.chunk_size,
		   chunk_overlap = EXCLUDED.chunk_overlap,
		   embedding_model = EXCLUDED.embedding_model,
		   dimensions = EXCLUDED.dimensions,
		   updated_at = now()`,
		row.WorkspaceID, row.CollectionID, row.ChunkStrategy, row.ChunkSize, row.ChunkOverlap, row.EmbeddingModel, row.Dimensions,
	)
	if err != nil {
		return fmt.Errorf("upsert chunk spec: %w", err)
	}
	return nil
}

// GetChunkSpec retrieves a chunk specification.
func (r *PgChunkSpecRepository) GetChunkSpec(ctx context.Context, workspaceID, collectionID string) (*ChunkSpecRow, error) {
	var row ChunkSpecRow
	err := r.q.QueryRow(ctx,
		`SELECT id, workspace_id, COALESCE(collection_id::text, ''), chunk_strategy, chunk_size, chunk_overlap, embedding_model, dimensions, created_at, updated_at
		 FROM embedding_chunk_specs
		 WHERE workspace_id = $1::uuid AND collection_id = $2::uuid`,
		workspaceID, collectionID,
	).Scan(&row.ID, &row.WorkspaceID, &row.CollectionID, &row.ChunkStrategy, &row.ChunkSize, &row.ChunkOverlap, &row.EmbeddingModel, &row.Dimensions, &row.CreatedAt, &row.UpdatedAt)
	if err != nil {
		return nil, fmt.Errorf("get chunk spec: %w", err)
	}
	return &row, nil
}

// ListChunkSpecs returns all chunk specifications for a workspace.
func (r *PgChunkSpecRepository) ListChunkSpecs(ctx context.Context, workspaceID string) ([]ChunkSpecRow, error) {
	rows, err := r.q.Query(ctx,
		`SELECT id, workspace_id, COALESCE(collection_id::text, ''), chunk_strategy, chunk_size, chunk_overlap, embedding_model, dimensions, created_at, updated_at
		 FROM embedding_chunk_specs
		 WHERE workspace_id = $1::uuid
		 ORDER BY created_at ASC`,
		workspaceID,
	)
	if err != nil {
		return nil, fmt.Errorf("list chunk specs: %w", err)
	}
	defer rows.Close()

	var result []ChunkSpecRow
	for rows.Next() {
		var s ChunkSpecRow
		if err := rows.Scan(&s.ID, &s.WorkspaceID, &s.CollectionID, &s.ChunkStrategy, &s.ChunkSize, &s.ChunkOverlap, &s.EmbeddingModel, &s.Dimensions, &s.CreatedAt, &s.UpdatedAt); err != nil {
			return nil, fmt.Errorf("scan chunk spec: %w", err)
		}
		result = append(result, s)
	}
	return result, rows.Err()
}
