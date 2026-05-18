package rag

import (
	"context"
	"math"
	"testing"
)

// mockProvider implements EmbeddingProvider for unit testing.
type mockProvider struct {
	dims int
}

func (m *mockProvider) Dimensions() int { return m.dims }

func (m *mockProvider) Embed(_ context.Context, texts []string) ([][]float32, error) {
	result := make([][]float32, len(texts))
	for i, text := range texts {
		vec := make([]float32, m.dims)
		// Deterministic fake embedding based on text length.
		for j := 0; j < m.dims; j++ {
			vec[j] = float32(len(text)+j+1) / float32(m.dims+len(text))
		}
		result[i] = vec
	}
	return result, nil
}

func TestEmbeddingServiceEmbedDocument(t *testing.T) {
	t.Parallel()

	provider := &mockProvider{dims: 8}
	svc := NewEmbeddingService(provider)

	ctx := context.Background()
	vec, err := svc.EmbedDocument(ctx, "hello world")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(vec) != 8 {
		t.Fatalf("expected 8 dimensions, got %d", len(vec))
	}

	// Second call should hit cache.
	vec2, err := svc.EmbedDocument(ctx, "hello world")
	if err != nil {
		t.Fatalf("unexpected error on cache hit: %v", err)
	}
	for i := range vec {
		if vec[i] != vec2[i] {
			t.Fatalf("cache mismatch at index %d", i)
		}
	}
}

func TestEmbeddingServiceEmbedQuery(t *testing.T) {
	t.Parallel()

	provider := &mockProvider{dims: 4}
	svc := NewEmbeddingService(provider)

	vec, err := svc.EmbedQuery(context.Background(), "search query")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(vec) != 4 {
		t.Fatalf("expected 4 dimensions, got %d", len(vec))
	}
}

func TestBatchEmbed(t *testing.T) {
	t.Parallel()

	provider := &mockProvider{dims: 4}
	svc := NewEmbeddingService(provider)

	texts := []string{"a", "b", "c", "d", "e"}
	vecs, err := svc.BatchEmbed(context.Background(), texts, 2)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(vecs) != 5 {
		t.Fatalf("expected 5 vectors, got %d", len(vecs))
	}
	for i, v := range vecs {
		if len(v) != 4 {
			t.Fatalf("vector %d has %d dims, expected 4", i, len(v))
		}
	}
}

func TestCosineSimilarity(t *testing.T) {
	t.Parallel()

	a := []float32{1, 0, 0}
	b := []float32{1, 0, 0}
	sim := CosineSimilarity(a, b)
	if math.Abs(sim-1.0) > 1e-6 {
		t.Fatalf("identical vectors should have similarity 1.0, got %f", sim)
	}

	c := []float32{0, 1, 0}
	sim2 := CosineSimilarity(a, c)
	if math.Abs(sim2) > 1e-6 {
		t.Fatalf("orthogonal vectors should have similarity 0, got %f", sim2)
	}

	empty := CosineSimilarity(nil, b)
	if empty != 0 {
		t.Fatalf("nil vector should return 0, got %f", empty)
	}
}

func TestCosineSimilarityPartialOverlap(t *testing.T) {
	t.Parallel()

	a := []float32{1, 1, 0}
	b := []float32{1, 0, 1}
	sim := CosineSimilarity(a, b)
	if sim < 0.3 || sim > 0.7 {
		t.Fatalf("expected partial similarity, got %f", sim)
	}
}
