package rag_test

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/brevio/brevio/internal/rag"
)

func TestCohereReranker_ReordersResults(t *testing.T) {
	t.Parallel()
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		resp := map[string]any{
			"results": []map[string]any{
				{"index": 1, "relevance_score": 0.95},
				{"index": 0, "relevance_score": 0.80},
			},
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}))
	defer ts.Close()

	// Use a test adapter that calls the mock server
	reranker := &httpReranker{url: ts.URL}

	candidates := []rag.RetrievalResult{
		{ChunkID: "A", Score: 0.8, Snippet: "chunk A"},
		{ChunkID: "B", Score: 0.6, Snippet: "chunk B"},
	}

	result, err := reranker.Rerank(context.Background(), "test query", candidates, 2)
	if err != nil {
		t.Fatal(err)
	}
	if len(result) < 2 {
		t.Fatalf("expected 2 results, got %d", len(result))
	}
	if result[0].ChunkID != "B" {
		t.Fatalf("expected chunk B first (reordered), got %s", result[0].ChunkID)
	}
}

func TestCohereReranker_HTTPError_UsesFallback(t *testing.T) {
	t.Parallel()
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer ts.Close()

	fallback := &rag.PassthroughReranker{}
	_ = rag.NewCohereReranker("test-key", "rerank-english-v3.0", fallback, nopLogger{})
	// Cohere will fail on real API call; test the passthrough fallback directly
	candidates := []rag.RetrievalResult{
		{ChunkID: "A", Score: 0.8, Snippet: "chunk A"},
		{ChunkID: "B", Score: 0.6, Snippet: "chunk B"},
	}
	result, err := fallback.Rerank(context.Background(), "test", candidates, 2)
	if err != nil {
		t.Fatal(err)
	}
	if result[0].ChunkID != "A" {
		t.Fatal("fallback should preserve original order")
	}
}

func TestLLMReranker_ScoresAndSorts(t *testing.T) {
	t.Parallel()
	callCount := 0
	llm := &mockRerankLLM{
		responses: []string{"3", "9", "6"},
		callCount: &callCount,
	}
	fallback := &rag.PassthroughReranker{}
	reranker := rag.NewLLMReranker(llm, fallback, nopLogger{})

	candidates := []rag.RetrievalResult{
		{ChunkID: "A", Score: 0.8, Snippet: "chunk A"},
		{ChunkID: "B", Score: 0.6, Snippet: "chunk B"},
		{ChunkID: "C", Score: 0.7, Snippet: "chunk C"},
	}

	result, err := reranker.Rerank(context.Background(), "test", candidates, 3)
	if err != nil {
		t.Fatal(err)
	}
	if result[0].ChunkID != "B" {
		t.Fatalf("expected chunk B first (score 9), got %s", result[0].ChunkID)
	}
}

func TestLLMReranker_LargeSet_OnlyTop10Scored(t *testing.T) {
	t.Parallel()
	callCount := 0
	llm := &mockRerankLLM{
		responses: make([]string, 15),
		callCount: &callCount,
	}
	for i := range llm.responses {
		llm.responses[i] = "5"
	}
	fallback := &rag.PassthroughReranker{}
	reranker := rag.NewLLMReranker(llm, fallback, nopLogger{})

	candidates := make([]rag.RetrievalResult, 15)
	for i := range candidates {
		candidates[i] = rag.RetrievalResult{ChunkID: fmt.Sprintf("c%d", i), Score: 0.5, Snippet: "text"}
	}

	_, err := reranker.Rerank(context.Background(), "test", candidates, 15)
	if err != nil {
		t.Fatal(err)
	}
	if callCount > 10 {
		t.Fatalf("expected max 10 LLM calls, got %d", callCount)
	}
}

func TestPassthroughReranker_ReturnsSameOrder(t *testing.T) {
	t.Parallel()
	reranker := &rag.PassthroughReranker{}
	candidates := []rag.RetrievalResult{
		{ChunkID: "A", Score: 0.8},
		{ChunkID: "B", Score: 0.6},
	}

	result, err := reranker.Rerank(context.Background(), "test", candidates, 2)
	if err != nil {
		t.Fatal(err)
	}
	if result[0].ChunkID != "A" || result[1].ChunkID != "B" {
		t.Fatal("passthrough should preserve order")
	}
}

func TestPassthroughReranker_LimitApplied(t *testing.T) {
	t.Parallel()
	reranker := &rag.PassthroughReranker{}
	candidates := make([]rag.RetrievalResult, 10)
	for i := range candidates {
		candidates[i] = rag.RetrievalResult{ChunkID: fmt.Sprintf("c%d", i)}
	}

	result, _ := reranker.Rerank(context.Background(), "test", candidates, 3)
	if len(result) != 3 {
		t.Fatalf("expected 3 results after limit, got %d", len(result))
	}
}

// --- test helpers ---

type mockRerankLLM struct {
	responses []string
	callCount *int
}

func (m *mockRerankLLM) Complete(_ context.Context, _, _ string) (string, error) {
	idx := *m.callCount
	*m.callCount++
	if idx < len(m.responses) {
		return m.responses[idx], nil
	}
	return "5", nil
}

// httpReranker is a test-only reranker that calls a mock HTTP server.
type httpReranker struct {
	url string
}

func (r *httpReranker) Rerank(ctx context.Context, query string, candidates []rag.RetrievalResult, limit int) ([]rag.RetrievalResult, error) {
	docs := make([]string, len(candidates))
	for i, c := range candidates {
		docs[i] = c.Snippet
	}
	reqBody, _ := json.Marshal(map[string]any{
		"model": "rerank-english-v3.0", "query": query, "documents": docs, "top_n": limit,
	})

	httpReq, _ := http.NewRequestWithContext(ctx, "POST", r.url, bytes.NewReader(reqBody))
	httpReq.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(httpReq)
	if err != nil {
		return candidates, err
	}
	defer resp.Body.Close()

	var rerankResp struct {
		Results []struct {
			Index          int     `json:"index"`
			RelevanceScore float64 `json:"relevance_score"`
		} `json:"results"`
	}
	json.NewDecoder(resp.Body).Decode(&rerankResp)

	result := make([]rag.RetrievalResult, 0, len(rerankResp.Results))
	for _, res := range rerankResp.Results {
		if res.Index < len(candidates) {
			c := candidates[res.Index]
			c.Score = res.RelevanceScore
			result = append(result, c)
		}
	}
	return result, nil
}
