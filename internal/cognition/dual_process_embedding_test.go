package cognition

import (
	"context"
	"testing"

	"github.com/brevio/brevio/internal/rag"
)

func TestSystem1MatchCtx_EmbeddingPath_HighSimilarity(t *testing.T) {
	embedder := rag.NewMockEmbeddingProvider(1536)
	engine := NewDualProcessEngineWithEmbeddings(embedder)
	ctx := context.Background()

	_, err := engine.LearnHeuristicCtx(ctx, "schedule meeting", "I'll schedule that for you", "scheduling", "test")
	if err != nil {
		t.Fatal(err)
	}

	// MockEmbedder returns deterministic vectors — same text = identical vector = cosine 1.0
	result, found := engine.System1MatchCtx(ctx, "schedule meeting")
	if !found {
		t.Fatal("expected match with identical embedding")
	}
	if result.Confidence < 0.65 {
		t.Fatalf("expected confidence >= 0.65, got %v", result.Confidence)
	}
}

func TestSystem1MatchCtx_FallbackToSubstring_NilProvider(t *testing.T) {
	engine := NewDualProcessEngine() // no embedder
	ctx := context.Background()

	_, _ = engine.LearnHeuristic("schedule meeting", "I'll schedule that", "scheduling", "test")

	result, found := engine.System1MatchCtx(ctx, "schedule meeting please")
	if !found {
		t.Fatal("expected substring fallback match")
	}
	if result.Response != "I'll schedule that" {
		t.Fatalf("wrong response: %q", result.Response)
	}
}

func TestSystem1MatchCtx_NoMatch_BelowThreshold(t *testing.T) {
	engine := NewDualProcessEngine()
	ctx := context.Background()

	// Learn a heuristic but query something completely different
	_, _ = engine.LearnHeuristic("weather forecast", "sunny today", "weather", "test")

	_, found := engine.System1MatchCtx(ctx, "send email to bob")
	if found {
		t.Fatal("expected no match for unrelated query")
	}
}

func TestCosineSimilarityF32_Identity(t *testing.T) {
	v := []float32{1, 0, 0}
	sim := cosineSimilarityF32(v, v)
	if sim < 0.99 {
		t.Fatalf("expected ~1.0, got %v", sim)
	}
}

func TestCosineSimilarityF32_Orthogonal(t *testing.T) {
	a := []float32{1, 0}
	b := []float32{0, 1}
	sim := cosineSimilarityF32(a, b)
	if sim > 0.01 {
		t.Fatalf("expected ~0.0, got %v", sim)
	}
}

func TestCosineSimilarityF32_EmptyVector(t *testing.T) {
	sim := cosineSimilarityF32(nil, []float32{1, 0})
	if sim != 0 {
		t.Fatalf("expected 0 for empty vector, got %v", sim)
	}
}

func TestLearnHeuristicCtx_StoresEmbedding(t *testing.T) {
	embedder := rag.NewMockEmbeddingProvider(8)
	engine := NewDualProcessEngineWithEmbeddings(embedder)

	h, err := engine.LearnHeuristicCtx(context.Background(), "test pattern", "test response", "test", "test")
	if err != nil {
		t.Fatal(err)
	}
	if len(h.PatternEmbedding) != 8 {
		t.Fatalf("expected 8-dim embedding, got %d", len(h.PatternEmbedding))
	}
}

func TestShouldEscalateToSystem2Ctx_LowConfidence(t *testing.T) {
	engine := NewDualProcessEngine()
	result := &System1Result{Confidence: 0.5}
	if !engine.ShouldEscalateToSystem2Ctx(context.Background(), "test", result) {
		t.Fatal("expected escalation for low confidence")
	}
}
