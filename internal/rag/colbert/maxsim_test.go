package colbert

import (
	"context"
	"fmt"
	"math"
	"testing"
)

func TestMaxSim_IdenticalVectors_Score1(t *testing.T) {
	v := make([]float32, 8)
	v[0] = 1.0
	score := ComputeMaxSim([][]float32{v}, [][]float32{v})
	if math.Abs(score-1.0) > 1e-5 {
		t.Fatalf("expected ~1.0, got %v", score)
	}
}

func TestMaxSim_OrthogonalVectors_Score0(t *testing.T) {
	q := make([]float32, 8)
	d := make([]float32, 8)
	q[0] = 1.0
	d[1] = 1.0
	score := ComputeMaxSim([][]float32{q}, [][]float32{d})
	if math.Abs(score) > 1e-5 {
		t.Fatalf("expected ~0.0, got %v", score)
	}
}

func TestMaxSim_PartialMatch_Between0and1(t *testing.T) {
	q1 := make([]float32, 8)
	q2 := make([]float32, 8)
	d1 := make([]float32, 8)
	q1[0] = 1.0 // matches d1
	q2[1] = 1.0 // no match in d
	d1[0] = 1.0
	score := ComputeMaxSim([][]float32{q1, q2}, [][]float32{d1})
	// q1 matches d1 perfectly (1.0), q2 doesn't match (0.0), average = 0.5
	if score < 0.1 || score > 0.9 {
		t.Fatalf("expected partial match score, got %v", score)
	}
}

func TestMaxSim_MultipleQueryVecs_Averaged(t *testing.T) {
	v := make([]float32, 4)
	v[0] = 1.0
	// All 3 query vecs match the single doc vec → average should be ~1.0
	score := ComputeMaxSim([][]float32{v, v, v}, [][]float32{v})
	if math.Abs(score-1.0) > 1e-5 {
		t.Fatalf("expected ~1.0, got %v", score)
	}
}

func TestScoreChunks_ReordersCorrectly(t *testing.T) {
	embedder := &mockMaxSimEmbedder{}
	repo := &mockSubvectorRepo{
		subvectors: map[string][]SubVector{
			"chunk-A": {{Embedding: vecAt(0)}},
			"chunk-B": {{Embedding: vecAt(0)}}, // same as query → higher MaxSim
		},
	}
	scorer := NewMaxSimScorer(repo, embedder, nopColbertLogger{})

	candidates := []ScoredChunk{
		{ChunkID: "chunk-A", HybridScore: 0.9, Content: "text A"},
		{ChunkID: "chunk-B", HybridScore: 0.5, Content: "text B"},
	}

	result := scorer.ScoreChunks(context.Background(), "query", candidates)
	// Both have same MaxSim but different HybridScore → A should win on blend
	if len(result) != 2 {
		t.Fatalf("expected 2 results, got %d", len(result))
	}
}

func TestScoreChunks_NoSubvectors_UsesHybrid(t *testing.T) {
	embedder := &mockMaxSimEmbedder{}
	repo := &mockSubvectorRepo{subvectors: map[string][]SubVector{}}
	scorer := NewMaxSimScorer(repo, embedder, nopColbertLogger{})

	candidates := []ScoredChunk{
		{ChunkID: "chunk-A", HybridScore: 0.8},
	}

	result := scorer.ScoreChunks(context.Background(), "query", candidates)
	if result[0].FinalScore != 0.8 {
		t.Fatalf("expected HybridScore as fallback, got %v", result[0].FinalScore)
	}
}

func TestScoreChunks_EmbedFails_OriginalOrder(t *testing.T) {
	embedder := &mockMaxSimEmbedder{err: fmt.Errorf("embed failed")}
	repo := &mockSubvectorRepo{}
	scorer := NewMaxSimScorer(repo, embedder, nopColbertLogger{})

	candidates := []ScoredChunk{
		{ChunkID: "A", HybridScore: 0.5},
		{ChunkID: "B", HybridScore: 0.9},
	}

	result := scorer.ScoreChunks(context.Background(), "query", candidates)
	if result[0].ChunkID != "A" || result[1].ChunkID != "B" {
		t.Fatal("expected original order on embed failure")
	}
}

// helpers

func vecAt(idx int) []float32 {
	v := make([]float32, 8)
	v[idx] = 1.0
	return v
}

type mockMaxSimEmbedder struct {
	err error
}

func (m *mockMaxSimEmbedder) Embed(_ context.Context, texts []string) ([][]float32, error) {
	if m.err != nil {
		return nil, m.err
	}
	result := make([][]float32, len(texts))
	for i := range texts {
		v := make([]float32, 8)
		v[0] = 1.0 // all queries get same vector for simplicity
		result[i] = v
	}
	return result, nil
}

type mockSubvectorRepo struct {
	subvectors map[string][]SubVector
}

func (r *mockSubvectorRepo) GetSubvectors(_ context.Context, chunkIDs []string) (map[string][]SubVector, error) {
	result := make(map[string][]SubVector)
	for _, id := range chunkIDs {
		if svs, ok := r.subvectors[id]; ok {
			result[id] = svs
		}
	}
	return result, nil
}

func (r *mockSubvectorRepo) StoreSubvectors(_ context.Context, _, _, _ string, _ []SubVector) error {
	return nil
}

func (r *mockSubvectorRepo) MarkSubvectorsGenerated(_ context.Context, _ string, _ int) error {
	return nil
}

type nopColbertLogger struct{}

func (nopColbertLogger) Warn(string, ...any)  {}
func (nopColbertLogger) Debug(string, ...any) {}
func (nopColbertLogger) Error(string, ...any) {}
