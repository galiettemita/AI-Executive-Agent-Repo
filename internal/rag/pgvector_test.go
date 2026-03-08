package rag

import (
	"context"
	"testing"
)

func TestPgVectorStoreUpsertAndSearch(t *testing.T) {
	t.Parallel()

	store := NewPgVectorStore()
	ctx := context.Background()

	err := store.UpsertChunk(ctx, ChunkWithEmbedding{
		ChunkID:      "c1",
		CollectionID: "col1",
		Content:      "Go programming language",
		Embedding:    []float32{1.0, 0.0, 0.0, 0.0},
		Metadata:     map[string]any{"source": "docs"},
	})
	if err != nil {
		t.Fatalf("upsert error: %v", err)
	}

	err = store.UpsertChunk(ctx, ChunkWithEmbedding{
		ChunkID:      "c2",
		CollectionID: "col1",
		Content:      "Python programming language",
		Embedding:    []float32{0.9, 0.1, 0.0, 0.0},
	})
	if err != nil {
		t.Fatalf("upsert error: %v", err)
	}

	err = store.UpsertChunk(ctx, ChunkWithEmbedding{
		ChunkID:      "c3",
		CollectionID: "col1",
		Content:      "Cooking recipes for pasta",
		Embedding:    []float32{0.0, 0.0, 1.0, 0.0},
	})
	if err != nil {
		t.Fatalf("upsert error: %v", err)
	}

	// Search for something similar to c1.
	results, err := store.SearchSimilar(ctx, []float32{1.0, 0.0, 0.0, 0.0}, 2, 0.5)
	if err != nil {
		t.Fatalf("search error: %v", err)
	}
	if len(results) < 1 {
		t.Fatalf("expected at least 1 result, got %d", len(results))
	}
	if results[0].Chunk.ChunkID != "c1" {
		t.Fatalf("expected c1 as top result, got %s", results[0].Chunk.ChunkID)
	}
	if results[0].Score < 0.99 {
		t.Fatalf("expected high similarity for exact match, got %f", results[0].Score)
	}
}

func TestPgVectorStoreUpsertValidation(t *testing.T) {
	t.Parallel()

	store := NewPgVectorStore()
	ctx := context.Background()

	err := store.UpsertChunk(ctx, ChunkWithEmbedding{ChunkID: "", Embedding: []float32{1}})
	if err == nil {
		t.Fatalf("expected error for empty chunk_id")
	}

	err = store.UpsertChunk(ctx, ChunkWithEmbedding{ChunkID: "c1", Embedding: nil})
	if err == nil {
		t.Fatalf("expected error for nil embedding")
	}
}

func TestPgVectorStoreSearchMinScore(t *testing.T) {
	t.Parallel()

	store := NewPgVectorStore()
	ctx := context.Background()

	_ = store.UpsertChunk(ctx, ChunkWithEmbedding{
		ChunkID:   "c1",
		Content:   "relevant",
		Embedding: []float32{1, 0, 0},
	})
	_ = store.UpsertChunk(ctx, ChunkWithEmbedding{
		ChunkID:   "c2",
		Content:   "irrelevant",
		Embedding: []float32{0, 0, 1},
	})

	results, err := store.SearchSimilar(ctx, []float32{1, 0, 0}, 10, 0.9)
	if err != nil {
		t.Fatalf("search error: %v", err)
	}
	// Only c1 should pass the 0.9 threshold.
	if len(results) != 1 {
		t.Fatalf("expected 1 result above minScore, got %d", len(results))
	}
}

func TestPgVectorStoreHybridSearch(t *testing.T) {
	t.Parallel()

	store := NewPgVectorStore()
	ctx := context.Background()

	_ = store.UpsertChunk(ctx, ChunkWithEmbedding{
		ChunkID:   "c1",
		Content:   "machine learning algorithms neural networks",
		Embedding: []float32{0.9, 0.1, 0.0},
	})
	_ = store.UpsertChunk(ctx, ChunkWithEmbedding{
		ChunkID:   "c2",
		Content:   "cooking recipes pasta tomato sauce",
		Embedding: []float32{0.0, 0.1, 0.9},
	})

	results, err := store.HybridSearch(ctx, []float32{0.8, 0.2, 0.0}, "machine learning", 5, 0.0)
	if err != nil {
		t.Fatalf("hybrid search error: %v", err)
	}
	if len(results) < 1 {
		t.Fatalf("expected at least 1 result")
	}
	if results[0].Chunk.ChunkID != "c1" {
		t.Fatalf("expected c1 as top hybrid result, got %s", results[0].Chunk.ChunkID)
	}
}

func TestPgVectorStoreDeleteChunk(t *testing.T) {
	t.Parallel()

	store := NewPgVectorStore()
	ctx := context.Background()

	_ = store.UpsertChunk(ctx, ChunkWithEmbedding{
		ChunkID:   "c1",
		Content:   "test",
		Embedding: []float32{1, 0},
	})

	if !store.DeleteChunk(ctx, "c1") {
		t.Fatalf("expected successful delete")
	}
	if store.DeleteChunk(ctx, "c1") {
		t.Fatalf("expected false for already-deleted chunk")
	}

	results, _ := store.SearchSimilar(ctx, []float32{1, 0}, 10, 0.0)
	if len(results) != 0 {
		t.Fatalf("expected 0 results after delete, got %d", len(results))
	}
}

func TestPgVectorStoreSearchEmptyEmbedding(t *testing.T) {
	t.Parallel()

	store := NewPgVectorStore()
	_, err := store.SearchSimilar(context.Background(), nil, 10, 0)
	if err == nil {
		t.Fatalf("expected error for nil query embedding")
	}
}
