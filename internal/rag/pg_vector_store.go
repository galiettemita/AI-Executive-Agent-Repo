package rag

import (
	"context"
	"fmt"

	"github.com/brevio/brevio/internal/database"
	"github.com/jackc/pgx/v5"
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
func (s *PgVectorProdStore) UpsertChunk(ctx context.Context, chunk ChunkWithEmbedding) error {
	if chunk.ChunkID == "" {
		return fmt.Errorf("chunk_id is required")
	}
	if len(chunk.Embedding) == 0 {
		return fmt.Errorf("embedding is required")
	}

	_, err := s.db.Exec(ctx, `
		INSERT INTO rag_chunks (chunk_id, collection_id, content, embedding, metadata)
		VALUES ($1, $2, $3, $4::vector, $5)
		ON CONFLICT (chunk_id) DO UPDATE SET
			content = EXCLUDED.content,
			embedding = EXCLUDED.embedding,
			metadata = EXCLUDED.metadata`,
		chunk.ChunkID, chunk.CollectionID, chunk.Content, chunk.Embedding, chunk.Metadata)
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

	rows, err := s.db.Query(ctx, `
		SELECT chunk_id, collection_id, content, embedding, metadata,
		       1 - (embedding <=> $1::vector) AS score
		FROM rag_chunks
		WHERE 1 - (embedding <=> $1::vector) >= $2
		ORDER BY embedding <=> $1::vector
		LIMIT $3`,
		queryEmbedding, minScore, limit)
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

	rows, err := s.db.Query(ctx, `
		SELECT chunk_id, collection_id, content, embedding, metadata,
		       ($4 * (1 - (embedding <=> $1::vector))) +
		       ($5 * ts_rank(to_tsvector('english', content), plainto_tsquery('english', $2))) AS score
		FROM rag_chunks
		WHERE ($4 * (1 - (embedding <=> $1::vector))) +
		      ($5 * ts_rank(to_tsvector('english', content), plainto_tsquery('english', $2))) >= $3
		ORDER BY score DESC
		LIMIT $6`,
		queryEmbedding, queryText, minScore, denseW, bm25W, limit)
	if err != nil {
		return nil, fmt.Errorf("hybrid search: %w", err)
	}
	defer rows.Close()

	return scanScoredChunks(rows)
}

// DeleteChunk removes a chunk by ID from PostgreSQL.
func (s *PgVectorProdStore) DeleteChunk(ctx context.Context, chunkID string) (bool, error) {
	tag, err := s.db.Exec(ctx, `DELETE FROM rag_chunks WHERE chunk_id = $1`, chunkID)
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
