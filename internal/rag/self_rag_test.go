package rag

import (
	"context"
	"encoding/json"
	"fmt"
	"testing"
)

type mockSelfRAGLLM struct {
	responses []string
	callIdx   int
	err       error
}

func (m *mockSelfRAGLLM) Complete(_ context.Context, _, _ string) (string, error) {
	if m.err != nil {
		return "", m.err
	}
	if m.callIdx >= len(m.responses) {
		return "", nil
	}
	resp := m.responses[m.callIdx]
	m.callIdx++
	return resp, nil
}

func makeTestResults(n int) []RetrievalResult {
	results := make([]RetrievalResult, n)
	for i := range results {
		results[i].Snippet = fmt.Sprintf("test document content for chunk %d", i)
	}
	return results
}

func marshalCritique(relevance float64, supported bool, reason string) string {
	cr := critiqueResponse{
		Relevance: relevance,
		Supported: supported,
		Reason:    reason,
	}
	b, _ := json.Marshal(cr)
	return string(b)
}

func TestSelfRAG_PassThreshold(t *testing.T) {
	mock := &mockSelfRAGLLM{
		responses: []string{
			marshalCritique(0.85, true, "directly answers the question"),
			marshalCritique(0.90, true, "exact match to query"),
			marshalCritique(0.75, true, "strongly relevant content"),
		},
	}

	cfg := DefaultSelfRAGConfig()
	critiquer := NewSelfRAGCritiquer(mock, cfg, nil)

	results := makeTestResults(3)
	decision, err := critiquer.Evaluate(context.Background(), "what is vector search", results)

	if err != nil {
		t.Fatalf("Evaluate() returned unexpected error: %v", err)
	}
	if !decision.PassThreshold {
		t.Errorf("expected PassThreshold=true, got false (MaxRelevance=%.3f)", decision.MaxRelevance)
	}
	if decision.MaxRelevance < 0.55 {
		t.Errorf("expected MaxRelevance >= 0.55, got %.3f", decision.MaxRelevance)
	}
	if decision.RewrittenQuery != "" {
		t.Errorf("expected RewrittenQuery=\"\" on pass, got %q", decision.RewrittenQuery)
	}
	if len(decision.Critiques) != 3 {
		t.Errorf("expected 3 critiques, got %d", len(decision.Critiques))
	}
	if mock.callIdx != 3 {
		t.Errorf("expected exactly 3 LLM calls on pass, got %d", mock.callIdx)
	}
}

func TestSelfRAG_FailAndRewrite(t *testing.T) {
	expectedRewrite := "more specific rewritten query about vector similarity search"

	mock := &mockSelfRAGLLM{
		responses: []string{
			marshalCritique(0.10, false, "completely unrelated content"),
			marshalCritique(0.20, false, "tangentially related only"),
			marshalCritique(0.15, false, "does not address the query"),
			expectedRewrite,
		},
	}

	cfg := DefaultSelfRAGConfig()
	critiquer := NewSelfRAGCritiquer(mock, cfg, nil)

	results := makeTestResults(3)
	decision, err := critiquer.Evaluate(context.Background(), "vague query", results)

	if err != nil {
		t.Fatalf("Evaluate() returned unexpected error: %v", err)
	}
	if decision.PassThreshold {
		t.Errorf("expected PassThreshold=false for low scores, got true")
	}
	if decision.MaxRelevance >= 0.55 {
		t.Errorf("expected MaxRelevance < 0.55, got %.3f", decision.MaxRelevance)
	}
	if decision.RewrittenQuery != expectedRewrite {
		t.Errorf("expected RewrittenQuery=%q, got %q", expectedRewrite, decision.RewrittenQuery)
	}
	if len(decision.Critiques) != 3 {
		t.Errorf("expected 3 critiques, got %d", len(decision.Critiques))
	}
	if mock.callIdx != 4 {
		t.Errorf("expected exactly 4 LLM calls (3 critique + 1 rewrite), got %d", mock.callIdx)
	}
}

func TestSelfRAG_Disabled(t *testing.T) {
	mock := &mockSelfRAGLLM{
		responses: []string{
			marshalCritique(0.05, false, "irrelevant"),
			marshalCritique(0.10, false, "irrelevant"),
			marshalCritique(0.08, false, "irrelevant"),
		},
	}

	cfg := DefaultSelfRAGConfig()
	cfg.Enabled = false
	critiquer := NewSelfRAGCritiquer(mock, cfg, nil)

	results := makeTestResults(3)
	decision, err := critiquer.Evaluate(context.Background(), "any query", results)

	if err != nil {
		t.Fatalf("Evaluate() returned unexpected error: %v", err)
	}
	if decision.PassThreshold {
		t.Errorf("expected PassThreshold=false for low scores when disabled")
	}
	if decision.RewrittenQuery != "" {
		t.Errorf("expected RewrittenQuery=\"\" when Enabled=false, got %q", decision.RewrittenQuery)
	}
	if mock.callIdx != 3 {
		t.Errorf("expected exactly 3 LLM calls (no rewrite when disabled), got %d", mock.callIdx)
	}
}
