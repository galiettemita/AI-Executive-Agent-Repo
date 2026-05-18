package contract

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/brevio/brevio/internal/rag"
)

func TestOpenAIEmbeddingsContract_RequestFormat(t *testing.T) {
	var receivedReq struct {
		Input []string `json:"input"`
		Model string   `json:"model"`
	}
	var receivedHeaders http.Header

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		receivedHeaders = r.Header.Clone()

		if r.Method != http.MethodPost {
			t.Errorf("expected POST, got %s", r.Method)
		}
		if r.URL.Path != "/v1/embeddings" {
			t.Errorf("expected /v1/embeddings, got %s", r.URL.Path)
		}

		if err := json.NewDecoder(r.Body).Decode(&receivedReq); err != nil {
			t.Fatalf("decode request body: %v", err)
		}

		// Return deterministic embeddings
		dims := 1536
		embedding := make([]float64, dims)
		for i := range embedding {
			embedding[i] = 0.01 * float64(i%100)
		}

		resp := map[string]any{
			"data": []map[string]any{
				{"embedding": embedding, "index": 0},
			},
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	provider := rag.NewOpenAIEmbeddingProvider(server.URL, "test-api-key")
	_, err := provider.Embed(context.Background(), []string{"hello world"})
	if err != nil {
		t.Fatalf("Embed failed: %v", err)
	}

	// Validate request format
	if receivedReq.Model != "text-embedding-3-small" {
		t.Errorf("expected model text-embedding-3-small, got %s", receivedReq.Model)
	}
	if len(receivedReq.Input) != 1 || receivedReq.Input[0] != "hello world" {
		t.Errorf("unexpected input: %v", receivedReq.Input)
	}

	// Validate auth header
	auth := receivedHeaders.Get("Authorization")
	if auth != "Bearer test-api-key" {
		t.Errorf("expected Bearer auth, got %s", auth)
	}

	// Validate content type
	ct := receivedHeaders.Get("Content-Type")
	if ct != "application/json" {
		t.Errorf("expected application/json content type, got %s", ct)
	}
}

func TestOpenAIEmbeddingsContract_BatchInput(t *testing.T) {
	var receivedReq struct {
		Input []string `json:"input"`
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewDecoder(r.Body).Decode(&receivedReq)

		dims := 1536
		data := make([]map[string]any, len(receivedReq.Input))
		for i := range data {
			embedding := make([]float64, dims)
			data[i] = map[string]any{"embedding": embedding, "index": i}
		}
		resp := map[string]any{"data": data}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	provider := rag.NewOpenAIEmbeddingProvider(server.URL, "key")
	texts := []string{"text one", "text two", "text three"}
	result, err := provider.Embed(context.Background(), texts)
	if err != nil {
		t.Fatalf("Embed failed: %v", err)
	}
	if len(result) != 3 {
		t.Fatalf("expected 3 embeddings, got %d", len(result))
	}
	if len(receivedReq.Input) != 3 {
		t.Errorf("expected 3 inputs sent, got %d", len(receivedReq.Input))
	}
}

func TestOpenAIEmbeddingsContract_ErrorHandling(t *testing.T) {
	tests := []struct {
		name       string
		statusCode int
		body       string
	}{
		{"rate_limited", 429, `{"error":{"message":"rate limited","type":"rate_limit"}}`},
		{"unauthorized", 401, `{"error":{"message":"invalid api key","type":"auth_error"}}`},
		{"server_error", 500, `{"error":{"message":"internal error","type":"server_error"}}`},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(tt.statusCode)
				w.Write([]byte(tt.body))
			}))
			defer server.Close()

			provider := rag.NewOpenAIEmbeddingProvider(server.URL, "key")
			_, err := provider.Embed(context.Background(), []string{"test"})
			if err == nil {
				t.Fatalf("expected error for status %d", tt.statusCode)
			}
		})
	}
}

func TestOpenAIEmbeddingsContract_Timeout(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Simulate slow response - but we use context cancellation
		<-r.Context().Done()
	}))
	defer server.Close()

	provider := rag.NewOpenAIEmbeddingProvider(server.URL, "key")
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // cancel immediately

	_, err := provider.Embed(ctx, []string{"test"})
	if err == nil {
		t.Fatal("expected error for cancelled context")
	}
}

func TestOpenAIEmbeddingsContract_EmptyInput(t *testing.T) {
	provider := rag.NewOpenAIEmbeddingProvider("http://unused", "key")
	result, err := provider.Embed(context.Background(), []string{})
	if err != nil {
		t.Fatalf("unexpected error for empty input: %v", err)
	}
	if result != nil {
		t.Errorf("expected nil result for empty input, got %v", result)
	}
}
