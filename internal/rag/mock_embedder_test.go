package rag_test

import (
	"context"
	"math"
	"testing"

	"github.com/brevio/brevio/internal/rag"
)

func TestMockEmbedder_Deterministic(t *testing.T) {
	m := rag.NewMockEmbeddingProvider(1536)
	ctx := context.Background()

	r1, err := m.Embed(ctx, []string{"hello world"})
	if err != nil {
		t.Fatalf("Embed: %v", err)
	}
	r2, _ := m.Embed(ctx, []string{"hello world"})

	for i := range r1[0] {
		if r1[0][i] != r2[0][i] {
			t.Fatalf("dim %d: got %v vs %v (not deterministic)", i, r1[0][i], r2[0][i])
		}
	}
}

func TestMockEmbedder_UnitNorm(t *testing.T) {
	m := rag.NewMockEmbeddingProvider(1536)
	ctx := context.Background()

	results, _ := m.Embed(ctx, []string{"test text"})
	vec := results[0]

	var sumSq float64
	for _, x := range vec {
		sumSq += float64(x) * float64(x)
	}
	norm := math.Sqrt(sumSq)
	if math.Abs(norm-1.0) > 1e-5 {
		t.Fatalf("L2 norm = %v, want 1.0 (±1e-5)", norm)
	}
}

func TestMockEmbedder_DifferentTexts_DifferentVectors(t *testing.T) {
	m := rag.NewMockEmbeddingProvider(1536)
	ctx := context.Background()

	results, _ := m.Embed(ctx, []string{"hello", "world"})
	v1, v2 := results[0], results[1]

	identical := true
	for i := range v1 {
		if v1[i] != v2[i] {
			identical = false
			break
		}
	}
	if identical {
		t.Fatal("different inputs produced identical vectors")
	}
}

func TestMockEmbedder_Dimensions(t *testing.T) {
	m := rag.NewMockEmbeddingProvider(1536)
	if m.Dimensions() != 1536 {
		t.Fatalf("Dimensions() = %d, want 1536", m.Dimensions())
	}
}

func TestMockEmbedder_BatchLength(t *testing.T) {
	m := rag.NewMockEmbeddingProvider(1536)
	ctx := context.Background()

	texts := []string{"a", "b", "c", "d"}
	results, err := m.Embed(ctx, texts)
	if err != nil {
		t.Fatal(err)
	}
	if len(results) != len(texts) {
		t.Fatalf("len(results) = %d, want %d", len(results), len(texts))
	}
	for i, v := range results {
		if len(v) != 1536 {
			t.Fatalf("result[%d]: len = %d, want 1536", i, len(v))
		}
	}
}
