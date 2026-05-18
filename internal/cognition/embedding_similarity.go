package cognition

import (
	"context"
	"fmt"
	"math"
)

// EmbeddingProvider generates embedding vectors for text.
type EmbeddingProvider interface {
	Embed(ctx context.Context, texts []string) ([][]float32, error)
	Dimensions() int
}

// EmbeddingCaseReasoningEngine implements case-based reasoning using dense
// embeddings for semantic similarity instead of lexical Jaccard heuristics.
// This satisfies the S3 algorithm fidelity constraint.
type EmbeddingCaseReasoningEngine struct {
	repo     CaseRepository
	embedder EmbeddingProvider
}

// NewEmbeddingCaseReasoningEngine creates a production case reasoning engine
// that uses embeddings for similarity search.
func NewEmbeddingCaseReasoningEngine(repo CaseRepository, embedder EmbeddingProvider) *EmbeddingCaseReasoningEngine {
	return &EmbeddingCaseReasoningEngine{
		repo:     repo,
		embedder: embedder,
	}
}

// StoreCase stores a new case with its embedding vector.
func (e *EmbeddingCaseReasoningEngine) StoreCase(ctx context.Context, workspaceID string, problem, solution, outcome string, score float64, features map[string]string) (*Case, error) {
	if workspaceID == "" {
		return nil, fmt.Errorf("workspace_id is required")
	}
	if problem == "" {
		return nil, fmt.Errorf("problem is required")
	}
	if solution == "" {
		return nil, fmt.Errorf("solution is required")
	}

	// Generate embedding for the problem text
	vecs, err := e.embedder.Embed(ctx, []string{problem})
	if err != nil {
		return nil, fmt.Errorf("generate embedding: %w", err)
	}
	if len(vecs) == 0 || len(vecs[0]) == 0 {
		return nil, fmt.Errorf("empty embedding returned")
	}

	c := &Case{
		WorkspaceID:  workspaceID,
		Problem:      problem,
		Solution:     solution,
		Outcome:      outcome,
		SuccessScore: score,
		Features:     features,
		Embedding:    vecs[0],
	}
	if c.Features == nil {
		c.Features = make(map[string]string)
	}

	if err := e.repo.Store(ctx, c); err != nil {
		return nil, fmt.Errorf("store case: %w", err)
	}
	return c, nil
}

// RetrieveSimilar finds similar past cases using embedding-based semantic
// similarity (cosine distance via pgvector), NOT lexical Jaccard.
func (e *EmbeddingCaseReasoningEngine) RetrieveSimilar(ctx context.Context, workspaceID, problem string, limit int) ([]ScoredCase, error) {
	// Generate embedding for the query problem
	vecs, err := e.embedder.Embed(ctx, []string{problem})
	if err != nil {
		return nil, fmt.Errorf("generate query embedding: %w", err)
	}
	if len(vecs) == 0 || len(vecs[0]) == 0 {
		return nil, fmt.Errorf("empty query embedding returned")
	}

	// Search by embedding similarity (cosine distance in pgvector)
	return e.repo.FindSimilarByEmbedding(ctx, workspaceID, vecs[0], limit, 0.3)
}

// AdaptSolution adapts a stored solution to a new problem.
func (e *EmbeddingCaseReasoningEngine) AdaptSolution(ctx context.Context, caseID string, newProblem string) (string, error) {
	c, err := e.repo.GetByID(ctx, caseID)
	if err != nil {
		return "", err
	}

	adapted := fmt.Sprintf("Based on similar case (%s): %s\nAdapted for: %s",
		c.Problem, c.Solution, newProblem)
	return adapted, nil
}

// RecordReuse increments the reuse count for a case.
func (e *EmbeddingCaseReasoningEngine) RecordReuse(ctx context.Context, caseID string) error {
	return e.repo.IncrementReuse(ctx, caseID)
}

// PruneLowReuse removes cases with fewer than minReuses reuses.
func (e *EmbeddingCaseReasoningEngine) PruneLowReuse(ctx context.Context, minReuses int) (int, error) {
	return e.repo.DeleteByMinReuse(ctx, minReuses)
}

// cosineSimilarityF32 computes cosine similarity between two float32 vectors.
func cosineSimilarityF32(a, b []float32) float64 {
	if len(a) == 0 || len(b) == 0 {
		return 0
	}
	dims := len(a)
	if len(b) < dims {
		dims = len(b)
	}
	var dot, normA, normB float64
	for i := 0; i < dims; i++ {
		dot += float64(a[i]) * float64(b[i])
		normA += float64(a[i]) * float64(a[i])
		normB += float64(b[i]) * float64(b[i])
	}
	if normA == 0 || normB == 0 {
		return 0
	}
	return dot / (math.Sqrt(normA) * math.Sqrt(normB))
}
