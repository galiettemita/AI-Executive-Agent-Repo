package rag

import (
	"context"
	"fmt"

	"github.com/brevio/brevio/internal/database"
	"github.com/jackc/pgx/v5"
	pgvector "github.com/pgvector/pgvector-go"
)

// PgVectorProdStore implements vector-similarity search using PostgreSQL with
// the pgvector extension. This is the production implementation that replaces
// the in-memory PgVectorStore used in tests.
type PgVectorProdStore struct {
	db database.Querier

	// HybridSearch tuning weights.
	DenseWeight float64
	BM25Weight  float64
}

// NewPgVectorProdStore creates a production pgvector store backed by PostgreSQL.
func NewPgVectorProdStore(db database.Querier) *PgVectorProdStore {
	return &PgVectorProdStore{
		db:          db,
		DenseWeight: 0.7,
		BM25Weight:  0.3,
	}
}

// UpsertChunk inserts or updates a chunk with its embedding in PostgreSQL.
// Column mapping: id=ChunkID, rag_collection_id=CollectionID, chunk_text=Content.
func (s *PgVectorProdStore) UpsertChunk(ctx context.Context, chunk ChunkWithEmbedding) error {
	if chunk.ChunkID == "" {
		return fmt.Errorf("chunk_id is required")
	}
	if len(chunk.Embedding) == 0 {
		return fmt.Errorf("embedding is required")
	}

	vec := pgvector.NewVector(chunk.Embedding)
	_, err := s.db.Exec(ctx, `
		INSERT INTO rag_chunks (id, workspace_id, rag_collection_id, chunk_text, embedding, metadata)
		VALUES ($1, $2, $3, $4, $5, $6)
		ON CONFLICT (id) DO UPDATE SET
			chunk_text = EXCLUDED.chunk_text,
			embedding = EXCLUDED.embedding,
			metadata = EXCLUDED.metadata`,
		chunk.ChunkID, chunk.WorkspaceID, chunk.CollectionID, chunk.Content, vec, chunk.Metadata)
	if err != nil {
		return fmt.Errorf("upsert chunk: %w", err)
	}
	return nil
}

// SearchSimilar returns the top-N chunks closest to queryEmbedding using
// cosine similarity via pgvector's <=> operator, filtered by minScore.
func (s *PgVectorProdStore) SearchSimilar(ctx context.Context, queryEmbedding []float32, limit int, minScore float64) ([]ScoredChunk, error) {
	if len(queryEmbedding) == 0 {
		return nil, fmt.Errorf("query embedding is required")
	}
	if limit <= 0 {
		limit = 10
	}

	qvec := pgvector.NewVector(queryEmbedding)
	rows, err := s.db.Query(ctx, `
		SELECT id, workspace_id, rag_collection_id, chunk_text, embedding, metadata,
		       1 - (embedding <=> $1) AS score
		FROM rag_chunks
		WHERE 1 - (embedding <=> $1) >= $2
		ORDER BY embedding <=> $1
		LIMIT $3`,
		qvec, minScore, limit)
	if err != nil {
		return nil, fmt.Errorf("search similar: %w", err)
	}
	defer rows.Close()

	return scanScoredChunks(rows)
}

// HybridSearch combines dense vector similarity with BM25 token-overlap scoring
// using PostgreSQL's ts_rank for the lexical component.
func (s *PgVectorProdStore) HybridSearch(ctx context.Context, queryEmbedding []float32, queryText string, limit int, minScore float64) ([]ScoredChunk, error) {
	if len(queryEmbedding) == 0 {
		return nil, fmt.Errorf("query embedding is required")
	}
	if limit <= 0 {
		limit = 10
	}

	denseW := s.DenseWeight
	bm25W := s.BM25Weight
	total := denseW + bm25W
	if total == 0 {
		denseW = 0.7
		bm25W = 0.3
		total = 1.0
	}
	denseW /= total
	bm25W /= total

	qvec := pgvector.NewVector(queryEmbedding)
	rows, err := s.db.Query(ctx, `
		SELECT id, workspace_id, rag_collection_id, chunk_text, embedding, metadata,
		       ($4 * (1 - (embedding <=> $1))) +
		       ($5 * ts_rank(to_tsvector('english', chunk_text), plainto_tsquery('english', $2))) AS score
		FROM rag_chunks
		WHERE ($4 * (1 - (embedding <=> $1))) +
		      ($5 * ts_rank(to_tsvector('english', chunk_text), plainto_tsquery('english', $2))) >= $3
		ORDER BY score DESC
		LIMIT $6`,
		qvec, queryText, minScore, denseW, bm25W, limit)
	if err != nil {
		return nil, fmt.Errorf("hybrid search: %w", err)
	}
	defer rows.Close()

	return scanScoredChunks(rows)
}

// DeleteChunk removes a chunk by ID from PostgreSQL.
func (s *PgVectorProdStore) DeleteChunk(ctx context.Context, chunkID string) (bool, error) {
	tag, err := s.db.Exec(ctx, `DELETE FROM rag_chunks WHERE id = $1`, chunkID)
	if err != nil {
		return false, fmt.Errorf("delete chunk: %w", err)
	}
	return tag.RowsAffected() > 0, nil
}

func scanScoredChunks(rows pgx.Rows) ([]ScoredChunk, error) {
	var results []ScoredChunk
	for rows.Next() {
		var sc ScoredChunk
		err := rows.Scan(
			&sc.Chunk.ChunkID,
			&sc.Chunk.WorkspaceID,
			&sc.Chunk.CollectionID,
			&sc.Chunk.Content,
			&sc.Chunk.Embedding,
			&sc.Chunk.Metadata,
			&sc.Score,
		)
		if err != nil {
			return nil, fmt.Errorf("scan scored chunk: %w", err)
		}
		results = append(results, sc)
	}
	return results, rows.Err()
}
