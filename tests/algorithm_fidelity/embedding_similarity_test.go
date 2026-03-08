package algorithm_fidelity

import (
	"context"
	"encoding/json"
	"math"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/brevio/brevio/internal/rag"
)

// deterministicEmbeddingServer returns a mock server that produces deterministic
// embeddings based on input text hash.
func deterministicEmbeddingServer(t *testing.T) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			Input []string `json:"input"`
			Model string   `json:"model"`
		}
		json.NewDecoder(r.Body).Decode(&req)

		data := make([]map[string]any, len(req.Input))
		for i, text := range req.Input {
			// Generate deterministic embedding from text
			embedding := make([]float64, 1536)
			for j := range embedding {
				// Simple deterministic function of text and position
				val := 0.0
				for _, ch := range text {
					val += float64(ch) * 0.0001
				}
				embedding[j] = math.Sin(float64(j)*0.01+val) * 0.5
			}
			data[i] = map[string]any{"embedding": embedding, "index": i}
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{"data": data})
	}))
}

func TestEmbeddingSimilarity_UsesVectorNotJaccard(t *testing.T) {
	server := deterministicEmbeddingServer(t)
	defer server.Close()

	provider := rag.NewOpenAIEmbeddingProvider(server.URL, "test-key")
	svc := rag.NewEmbeddingService(provider)
	ctx := context.Background()

	// Get embeddings for semantically related but lexically different texts
	vecA, err := svc.EmbedDocument(ctx, "the cat sat on the mat")
	if err != nil {
		t.Fatalf("embed A: %v", err)
	}
	vecB, err := svc.EmbedDocument(ctx, "a feline rested on the rug")
	if err != nil {
		t.Fatalf("embed B: %v", err)
	}
	vecC, err := svc.EmbedDocument(ctx, "stock market quarterly earnings report")
	if err != nil {
		t.Fatalf("embed C: %v", err)
	}

	// Verify we get real embedding vectors, not lexical scores
	if len(vecA) != 1536 {
		t.Fatalf("expected 1536 dimensions, got %d", len(vecA))
	}
	if len(vecB) != 1536 {
		t.Fatalf("expected 1536 dimensions, got %d", len(vecB))
	}

	// Cosine similarity must be used (not Jaccard)
	simAB := rag.CosineSimilarity(vecA, vecB)
	simAC := rag.CosineSimilarity(vecA, vecC)

	// Both should produce valid cosine similarity values
	if simAB < -1.0 || simAB > 1.0 {
		t.Errorf("cosine similarity out of range: %f", simAB)
	}
	if simAC < -1.0 || simAC > 1.0 {
		t.Errorf("cosine similarity out of range: %f", simAC)
	}

	t.Logf("sim(cat/feline) = %.4f, sim(cat/stock) = %.4f", simAB, simAC)
}

func TestPgVectorStore_UsesCosineDistance(t *testing.T) {
	store := rag.NewPgVectorStore()
	ctx := context.Background()

	// Store chunks with embeddings
	embA := make([]float32, 1536)
	embB := make([]float32, 1536)
	embC := make([]float32, 1536)

	// Make A and B similar, C different
	for i := range embA {
		embA[i] = float32(math.Sin(float64(i) * 0.01))
		embB[i] = float32(math.Sin(float64(i)*0.01 + 0.1)) // slight shift
		embC[i] = float32(math.Cos(float64(i) * 0.5))      // very different
	}

	store.UpsertChunk(ctx, rag.ChunkWithEmbedding{
		ChunkID: "chunk-a", CollectionID: "col-1", Content: "related content A", Embedding: embA,
	})
	store.UpsertChunk(ctx, rag.ChunkWithEmbedding{
		ChunkID: "chunk-b", CollectionID: "col-1", Content: "related content B", Embedding: embB,
	})
	store.UpsertChunk(ctx, rag.ChunkWithEmbedding{
		ChunkID: "chunk-c", CollectionID: "col-1", Content: "unrelated content", Embedding: embC,
	})

	// Search with embA - should find B as most similar
	results, err := store.SearchSimilar(ctx, embA, 10, 0.0)
	if err != nil {
		t.Fatalf("search: %v", err)
	}
	if len(results) < 2 {
		t.Fatalf("expected at least 2 results, got %d", len(results))
	}

	// First result should be chunk-a itself (exact match)
	if results[0].Chunk.ChunkID != "chunk-a" {
		t.Errorf("expected chunk-a first, got %s", results[0].Chunk.ChunkID)
	}
	// Second should be chunk-b (similar)
	if results[1].Chunk.ChunkID != "chunk-b" {
		t.Errorf("expected chunk-b second, got %s", results[1].Chunk.ChunkID)
	}
	// chunk-b score should be higher than chunk-c
	var scoreB, scoreC float64
	for _, r := range results {
		if r.Chunk.ChunkID == "chunk-b" {
			scoreB = r.Score
		}
		if r.Chunk.ChunkID == "chunk-c" {
			scoreC = r.Score
		}
	}
	if scoreB <= scoreC {
		t.Errorf("expected similar chunk (%.4f) to score higher than dissimilar (%.4f)", scoreB, scoreC)
	}
}

func TestCosineSimilarity_Properties(t *testing.T) {
	// Test mathematical properties of cosine similarity

	t.Run("identical_vectors", func(t *testing.T) {
		v := []float32{1.0, 2.0, 3.0}
		sim := rag.CosineSimilarity(v, v)
		if math.Abs(sim-1.0) > 1e-6 {
			t.Errorf("identical vectors should have similarity 1.0, got %f", sim)
		}
	})

	t.Run("orthogonal_vectors", func(t *testing.T) {
		a := []float32{1.0, 0.0, 0.0}
		b := []float32{0.0, 1.0, 0.0}
		sim := rag.CosineSimilarity(a, b)
		if math.Abs(sim) > 1e-6 {
			t.Errorf("orthogonal vectors should have similarity 0.0, got %f", sim)
		}
	})

	t.Run("opposite_vectors", func(t *testing.T) {
		a := []float32{1.0, 2.0, 3.0}
		b := []float32{-1.0, -2.0, -3.0}
		sim := rag.CosineSimilarity(a, b)
		if math.Abs(sim+1.0) > 1e-6 {
			t.Errorf("opposite vectors should have similarity -1.0, got %f", sim)
		}
	})

	t.Run("empty_vectors", func(t *testing.T) {
		sim := rag.CosineSimilarity([]float32{}, []float32{})
		if sim != 0 {
			t.Errorf("empty vectors should have similarity 0, got %f", sim)
		}
	})
}

func TestHybridSearch_CombinesDenseAndBM25(t *testing.T) {
	store := rag.NewPgVectorStore()
	store.DenseWeight = 0.7
	store.BM25Weight = 0.3
	ctx := context.Background()

	// Create embeddings where vector similarity contradicts lexical
	embQuery := make([]float32, 1536)
	embSemantic := make([]float32, 1536) // semantically similar
	embLexical := make([]float32, 1536)  // lexically similar but semantically different

	for i := range embQuery {
		embQuery[i] = float32(math.Sin(float64(i) * 0.01))
		embSemantic[i] = float32(math.Sin(float64(i)*0.01 + 0.05))
		embLexical[i] = float32(math.Cos(float64(i) * 0.5))
	}

	store.UpsertChunk(ctx, rag.ChunkWithEmbedding{
		ChunkID: "semantic", CollectionID: "col", Content: "a feline rested peacefully", Embedding: embSemantic,
	})
	store.UpsertChunk(ctx, rag.ChunkWithEmbedding{
		ChunkID: "lexical", CollectionID: "col", Content: "cat sat mat words", Embedding: embLexical,
	})

	results, err := store.HybridSearch(ctx, embQuery, "cat sat mat", 10, 0.0)
	if err != nil {
		t.Fatalf("hybrid search: %v", err)
	}
	if len(results) < 2 {
		t.Fatalf("expected at least 2 results, got %d", len(results))
	}

	// With 0.7 dense weight, semantic match should rank higher
	if results[0].Chunk.ChunkID != "semantic" {
		t.Errorf("expected semantic result first (dense weight 0.7), got %s with score %.4f",
			results[0].Chunk.ChunkID, results[0].Score)
	}
}
