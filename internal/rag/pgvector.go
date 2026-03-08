package rag

import (
	"context"
	"fmt"
	"math"
	"sort"
	"strings"
	"sync"
)

// ChunkWithEmbedding represents a chunk stored with its embedding vector.
type ChunkWithEmbedding struct {
	ChunkID      string         `json:"chunk_id"`
	CollectionID string         `json:"collection_id"`
	Content      string         `json:"content"`
	Embedding    []float32      `json:"embedding"`
	Metadata     map[string]any `json:"metadata,omitempty"`
}

// ScoredChunk pairs a chunk with its similarity score.
type ScoredChunk struct {
	Chunk ChunkWithEmbedding `json:"chunk"`
	Score float64            `json:"score"`
}

// PgVectorStore provides vector-similarity search. In production this would
// use pgx and pgvector; here we keep an in-memory store so the rest of the
// codebase can compile and test without a running Postgres instance. The SQL
// queries are preserved in comments for documentation.
type PgVectorStore struct {
	mu     sync.RWMutex
	chunks map[string]ChunkWithEmbedding // keyed by ChunkID

	// HybridSearch tuning weights.
	DenseWeight float64
	BM25Weight  float64
}

// NewPgVectorStore creates a new in-memory vector store.
func NewPgVectorStore() *PgVectorStore {
	return &PgVectorStore{
		chunks:      make(map[string]ChunkWithEmbedding),
		DenseWeight: 0.7,
		BM25Weight:  0.3,
	}
}

// UpsertChunk inserts or updates a chunk with its embedding.
//
// Production SQL (pgvector):
//
//	INSERT INTO rag_chunks (chunk_id, collection_id, content, embedding, metadata)
//	VALUES ($1, $2, $3, $4, $5)
//	ON CONFLICT (chunk_id) DO UPDATE SET content = $3, embedding = $4, metadata = $5
func (s *PgVectorStore) UpsertChunk(_ context.Context, chunk ChunkWithEmbedding) error {
	if strings.TrimSpace(chunk.ChunkID) == "" {
		return fmt.Errorf("chunk_id is required")
	}
	if len(chunk.Embedding) == 0 {
		return fmt.Errorf("embedding is required")
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	s.chunks[chunk.ChunkID] = chunk
	return nil
}

// SearchSimilar returns the top-N chunks closest to queryEmbedding using
// cosine similarity, filtered by minScore.
//
// Production SQL (pgvector):
//
//	SELECT chunk_id, collection_id, content, embedding, metadata,
//	       1 - (embedding <=> $1) AS score
//	FROM rag_chunks
//	WHERE 1 - (embedding <=> $1) >= $2
//	ORDER BY embedding <=> $1
//	LIMIT $3
func (s *PgVectorStore) SearchSimilar(_ context.Context, queryEmbedding []float32, limit int, minScore float64) ([]ScoredChunk, error) {
	if len(queryEmbedding) == 0 {
		return nil, fmt.Errorf("query embedding is required")
	}
	if limit <= 0 {
		limit = 10
	}

	s.mu.RLock()
	defer s.mu.RUnlock()

	scored := make([]ScoredChunk, 0, len(s.chunks))
	for _, c := range s.chunks {
		sim := CosineSimilarity(queryEmbedding, c.Embedding)
		if sim < minScore {
			continue
		}
		scored = append(scored, ScoredChunk{Chunk: c, Score: sim})
	}

	sort.Slice(scored, func(i, j int) bool {
		return scored[i].Score > scored[j].Score
	})
	if len(scored) > limit {
		scored = scored[:limit]
	}
	return scored, nil
}

// HybridSearch combines dense vector similarity with BM25 token-overlap
// scoring. The weights are configurable via DenseWeight and BM25Weight.
func (s *PgVectorStore) HybridSearch(_ context.Context, queryEmbedding []float32, queryText string, limit int, minScore float64) ([]ScoredChunk, error) {
	if len(queryEmbedding) == 0 {
		return nil, fmt.Errorf("query embedding is required")
	}
	if limit <= 0 {
		limit = 10
	}

	queryTokens := tokenize(queryText)

	s.mu.RLock()
	defer s.mu.RUnlock()

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

	scored := make([]ScoredChunk, 0, len(s.chunks))
	for _, c := range s.chunks {
		dense := CosineSimilarity(queryEmbedding, c.Embedding)
		chunkTokens := tokenize(c.Content)
		bm25 := bm25Score(queryTokens, chunkTokens)
		combined := denseW*dense + bm25W*bm25
		if combined < minScore {
			continue
		}
		scored = append(scored, ScoredChunk{Chunk: c, Score: combined})
	}

	sort.Slice(scored, func(i, j int) bool {
		return scored[i].Score > scored[j].Score
	})
	if len(scored) > limit {
		scored = scored[:limit]
	}
	return scored, nil
}

// DeleteChunk removes a chunk by ID.
func (s *PgVectorStore) DeleteChunk(_ context.Context, chunkID string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.chunks[chunkID]; !ok {
		return false
	}
	delete(s.chunks, chunkID)
	return true
}

// bm25Score computes a simplified BM25-style score based on term overlap.
func bm25Score(queryTokens, docTokens []string) float64 {
	if len(queryTokens) == 0 || len(docTokens) == 0 {
		return 0
	}
	docSet := make(map[string]int, len(docTokens))
	for _, t := range docTokens {
		docSet[t]++
	}

	k1 := 1.2
	b := 0.75
	avgDL := float64(len(docTokens))
	dl := float64(len(docTokens))

	score := 0.0
	for _, qt := range queryTokens {
		tf := float64(docSet[qt])
		if tf == 0 {
			continue
		}
		idf := math.Log(1 + 1.0) // simplified: assume single doc
		num := tf * (k1 + 1)
		denom := tf + k1*(1-b+b*(dl/avgDL))
		score += idf * (num / denom)
	}
	// Normalize to [0,1] range.
	maxScore := float64(len(queryTokens)) * math.Log(2) * (k1 + 1) / (1 + k1)
	if maxScore > 0 {
		score /= maxScore
	}
	return score
}
