package rag_test

import (
	"context"
	"fmt"
	"testing"

	. "github.com/brevio/brevio/internal/rag"
)

type mockCitationLLM struct {
	response string
	err      error
	called   bool
}

func (m *mockCitationLLM) Complete(_ context.Context, _, _ string) (string, error) {
	m.called = true
	if m.err != nil {
		return "", m.err
	}
	return m.response, nil
}

type citationTestLogger struct{ t *testing.T }

func newCitationTestLogger(t *testing.T) *citationTestLogger { return &citationTestLogger{t: t} }
func (l *citationTestLogger) Info(msg string, args ...any)   { l.t.Logf("[INFO] "+msg, args...) }
func (l *citationTestLogger) Warn(msg string, args ...any)   { l.t.Logf("[WARN] "+msg, args...) }
func (l *citationTestLogger) Error(msg string, args ...any)  { l.t.Logf("[ERROR] "+msg, args...) }

func buildTestChunk(id, collection, sourceURL, content string) RetrievalResult {
	return RetrievalResult{
		ChunkID:    id,
		Collection: collection,
		Source:     sourceURL,
		Snippet:    content,
	}
}

func TestCitationEmptyChunks(t *testing.T) {
	mock := &mockCitationLLM{}
	e := NewCitationExtractor(mock, newCitationTestLogger(t))

	result, err := e.Extract(context.Background(), "any response", nil)
	if err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}
	if result != nil {
		t.Errorf("expected nil result for nil chunks, got %v", result)
	}
	if mock.called {
		t.Error("LLM must not be called for nil chunks")
	}

	mock.called = false
	result, err = e.Extract(context.Background(), "any response", []RetrievalResult{})
	if err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}
	if result != nil {
		t.Errorf("expected nil result for empty chunks, got %v", result)
	}
	if mock.called {
		t.Error("LLM must not be called for empty chunks")
	}
}

func TestCitationHallucinatedIDSkipped(t *testing.T) {
	hallucinatedJSON := `[{"claim_text":"The sky is blue","chunk_id":"fake-nonexistent-999","confidence":0.9}]`
	mock := &mockCitationLLM{response: hallucinatedJSON}
	chunk := buildTestChunk("real-chunk-001", "docs", "https://example.com", "Real content about the sky.")
	e := NewCitationExtractor(mock, newCitationTestLogger(t))

	result, err := e.Extract(context.Background(), "The sky is blue.", []RetrievalResult{chunk})

	if err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}
	if len(result) != 0 {
		t.Errorf("hallucinated chunk ID must be skipped; got %d citations", len(result))
	}
}

func TestCitationValidExtraction(t *testing.T) {
	validJSON := `[{"claim_text":"The sky is blue","chunk_id":"chunk-001","confidence":0.92}]`
	mock := &mockCitationLLM{response: validJSON}
	chunk1 := buildTestChunk("chunk-001", "knowledge-base", "https://docs.example.com/sky", "The sky is blue because of Rayleigh scattering of sunlight through the atmosphere.")
	chunk2 := buildTestChunk("chunk-002", "knowledge-base", "https://docs.example.com/other", "Other content.")
	e := NewCitationExtractor(mock, newCitationTestLogger(t))

	result, err := e.Extract(context.Background(), "The sky is blue.", []RetrievalResult{chunk1, chunk2})

	if err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}
	if len(result) != 1 {
		t.Fatalf("expected 1 citation, got %d", len(result))
	}
	if result[0].ChunkID != "chunk-001" {
		t.Errorf("wrong ChunkID: %q", result[0].ChunkID)
	}
	if result[0].Confidence != 0.92 {
		t.Errorf("wrong Confidence: %f", result[0].Confidence)
	}
	if result[0].ClaimText != "The sky is blue" {
		t.Errorf("wrong ClaimText: %q", result[0].ClaimText)
	}
	if len(result[0].ChunkSnippet) > 200 {
		t.Errorf("ChunkSnippet exceeds 200 chars: %d", len(result[0].ChunkSnippet))
	}
	if result[0].ChunkSnippet == "" {
		t.Error("ChunkSnippet must not be empty")
	}
	if result[0].Collection != "knowledge-base" {
		t.Errorf("wrong Collection: %q", result[0].Collection)
	}
}

func TestCitationLLMErrorFailOpen(t *testing.T) {
	mock := &mockCitationLLM{err: fmt.Errorf("LLM unavailable")}
	chunk := buildTestChunk("chunk-001", "docs", "https://example.com", "Some content.")
	e := NewCitationExtractor(mock, newCitationTestLogger(t))

	result, err := e.Extract(context.Background(), "Some claim.", []RetrievalResult{chunk})

	if err != nil {
		t.Errorf("fail-open: error must not propagate, got %v", err)
	}
	if result != nil {
		t.Errorf("fail-open: result must be nil on LLM error, got %v", result)
	}
}
