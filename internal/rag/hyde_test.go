package rag_test

import (
	"context"
	"fmt"
	"math"
	"testing"

	"github.com/brevio/brevio/internal/rag"
)

type mockHyDELLM struct {
	response string
	err      error
}

func (m *mockHyDELLM) Complete(_ context.Context, _, _ string) (string, error) {
	return m.response, m.err
}

type nopLogger struct{}

func (nopLogger) Info(string, ...any)  {}
func (nopLogger) Warn(string, ...any)  {}
func (nopLogger) Error(string, ...any) {}

func TestHyDE_ComplexQuery_GeneratesHypoDoc(t *testing.T) {
	embedder := rag.NewMockEmbeddingProvider(1536)
	llm := &mockHyDELLM{response: "Alice confirmed the Q3 budget at $4.2M during the board meeting on October 15th."}
	expander := rag.NewHyDEExpander(llm, embedder, nopLogger{})

	vec, err := expander.ExpandQuery(context.Background(), "What did Alice say about the Q3 budget?")
	if err != nil {
		t.Fatal(err)
	}
	if len(vec) != 1536 {
		t.Fatalf("expected 1536 dims, got %d", len(vec))
	}

	// The combined vector should NOT be identical to a direct query embedding
	directEmbs, _ := embedder.Embed(context.Background(), []string{"What did Alice say about the Q3 budget?"})
	directVec := directEmbs[0]
	identical := true
	for i := range vec {
		if vec[i] != directVec[i] {
			identical = false
			break
		}
	}
	if identical {
		t.Error("HyDE-expanded vector should differ from direct query embedding")
	}
}

func TestHyDE_SimpleQuery_SkipsGeneration(t *testing.T) {
	embedder := rag.NewMockEmbeddingProvider(1536)
	llm := &mockHyDELLM{response: "should not be called"}
	expander := rag.NewHyDEExpander(llm, embedder, nopLogger{})

	if expander.IsComplexQuery("ok") {
		t.Error("'ok' should not be complex")
	}

	vec, err := expander.ExpandQuery(context.Background(), "ok")
	if err != nil {
		t.Fatal(err)
	}
	if len(vec) != 1536 {
		t.Fatalf("expected 1536 dims, got %d", len(vec))
	}
}

func TestHyDE_QuestionWord_TriggerHyDE(t *testing.T) {
	embedder := rag.NewMockEmbeddingProvider(1536)
	expander := rag.NewHyDEExpander(nil, embedder, nopLogger{})

	if !expander.IsComplexQuery("What is the current status of project alpha") {
		t.Error("question starting with 'What' should be complex")
	}
}

func TestHyDE_LLMFailure_FallsBackToQueryVec(t *testing.T) {
	embedder := rag.NewMockEmbeddingProvider(1536)
	llm := &mockHyDELLM{err: fmt.Errorf("LLM unavailable")}
	expander := rag.NewHyDEExpander(llm, embedder, nopLogger{})

	vec, err := expander.ExpandQuery(context.Background(), "What did Alice say about the budget in the meeting?")
	if err != nil {
		t.Fatalf("expected no error on LLM failure, got: %v", err)
	}
	if len(vec) != 1536 {
		t.Fatalf("expected 1536 dims, got %d", len(vec))
	}
}

func TestHyDE_WeightedAverage_Normalized(t *testing.T) {
	a := make([]float32, 8)
	b := make([]float32, 8)
	a[0] = 1.0
	b[1] = 1.0

	combined := rag.AvgWeightedVec(a, b, 0.6, 0.4)

	if combined[0] == 0 || combined[1] == 0 {
		t.Error("combined vec should have non-zero values in both dims")
	}

	var mag float64
	for _, v := range combined {
		mag += float64(v) * float64(v)
	}
	norm := math.Sqrt(mag)
	if math.Abs(norm-1.0) > 1e-5 {
		t.Fatalf("combined vec should be unit-normalized, got magnitude %v", norm)
	}
}

func TestHyDE_EmptyHypoDoc_FallsBack(t *testing.T) {
	embedder := rag.NewMockEmbeddingProvider(1536)
	llm := &mockHyDELLM{response: ""}
	expander := rag.NewHyDEExpander(llm, embedder, nopLogger{})

	vec, err := expander.ExpandQuery(context.Background(), "What did Alice say about the Q3 budget in the review?")
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
	if len(vec) != 1536 {
		t.Fatalf("expected 1536 dims, got %d", len(vec))
	}
}
