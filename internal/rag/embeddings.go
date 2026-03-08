package rag

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"net/http"
	"sync"
)

// EmbeddingProvider abstracts a text-embedding backend.
type EmbeddingProvider interface {
	Embed(ctx context.Context, texts []string) ([][]float32, error)
	Dimensions() int
}

// -----------------------------------------------------------------------
// OpenAI provider
// -----------------------------------------------------------------------

// OpenAIEmbeddingProvider calls the OpenAI-compatible embeddings API
// (text-embedding-3-small, 1536 dimensions by default).
type OpenAIEmbeddingProvider struct {
	BaseURL    string
	APIKey     string
	Model      string
	HTTPClient *http.Client
}

type openAIEmbeddingRequest struct {
	Input []string `json:"input"`
	Model string   `json:"model"`
}

type openAIEmbeddingResponse struct {
	Data []struct {
		Embedding []float32 `json:"embedding"`
		Index     int       `json:"index"`
	} `json:"data"`
}

// NewOpenAIEmbeddingProvider creates a provider targeting the OpenAI
// embeddings endpoint (or any compatible API).
func NewOpenAIEmbeddingProvider(baseURL, apiKey string) *OpenAIEmbeddingProvider {
	if baseURL == "" {
		baseURL = "https://api.openai.com"
	}
	return &OpenAIEmbeddingProvider{
		BaseURL:    baseURL,
		APIKey:     apiKey,
		Model:      "text-embedding-3-small",
		HTTPClient: http.DefaultClient,
	}
}

// Dimensions returns the embedding vector size.
func (p *OpenAIEmbeddingProvider) Dimensions() int { return 1536 }

// Embed sends texts to the embeddings API and returns vectors.
func (p *OpenAIEmbeddingProvider) Embed(ctx context.Context, texts []string) ([][]float32, error) {
	if len(texts) == 0 {
		return nil, nil
	}

	reqBody := openAIEmbeddingRequest{
		Input: texts,
		Model: p.Model,
	}
	body, err := json.Marshal(reqBody)
	if err != nil {
		return nil, fmt.Errorf("marshal embedding request: %w", err)
	}

	url := fmt.Sprintf("%s/v1/embeddings", p.BaseURL)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("build embedding request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	if p.APIKey != "" {
		req.Header.Set("Authorization", "Bearer "+p.APIKey)
	}

	resp, err := p.HTTPClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("embedding API call: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("embedding API returned %d: %s", resp.StatusCode, string(respBody))
	}

	var result openAIEmbeddingResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("decode embedding response: %w", err)
	}

	embeddings := make([][]float32, len(texts))
	for _, item := range result.Data {
		if item.Index < len(embeddings) {
			embeddings[item.Index] = item.Embedding
		}
	}
	return embeddings, nil
}

// -----------------------------------------------------------------------
// EmbeddingService — caching wrapper
// -----------------------------------------------------------------------

// EmbeddingService wraps an EmbeddingProvider and adds a simple in-memory
// cache for repeated texts.
type EmbeddingService struct {
	provider EmbeddingProvider
	mu       sync.RWMutex
	cache    map[string][]float32
}

// NewEmbeddingService creates an EmbeddingService around the given provider.
func NewEmbeddingService(provider EmbeddingProvider) *EmbeddingService {
	return &EmbeddingService{
		provider: provider,
		cache:    make(map[string][]float32),
	}
}

// EmbedDocument embeds a document string, using the cache when available.
func (s *EmbeddingService) EmbedDocument(ctx context.Context, text string) ([]float32, error) {
	s.mu.RLock()
	if cached, ok := s.cache[text]; ok {
		s.mu.RUnlock()
		return cached, nil
	}
	s.mu.RUnlock()

	vecs, err := s.provider.Embed(ctx, []string{text})
	if err != nil {
		return nil, err
	}
	if len(vecs) == 0 || len(vecs[0]) == 0 {
		return nil, fmt.Errorf("empty embedding returned")
	}

	s.mu.Lock()
	s.cache[text] = vecs[0]
	s.mu.Unlock()

	return vecs[0], nil
}

// EmbedQuery embeds a query string (no caching — queries tend to be unique).
func (s *EmbeddingService) EmbedQuery(ctx context.Context, query string) ([]float32, error) {
	vecs, err := s.provider.Embed(ctx, []string{query})
	if err != nil {
		return nil, err
	}
	if len(vecs) == 0 || len(vecs[0]) == 0 {
		return nil, fmt.Errorf("empty embedding returned")
	}
	return vecs[0], nil
}

// BatchEmbed calls the provider in batches of batchSize.
func (s *EmbeddingService) BatchEmbed(ctx context.Context, texts []string, batchSize int) ([][]float32, error) {
	if batchSize <= 0 {
		batchSize = 64
	}
	all := make([][]float32, len(texts))

	for start := 0; start < len(texts); start += batchSize {
		end := start + batchSize
		if end > len(texts) {
			end = len(texts)
		}
		batch := texts[start:end]

		vecs, err := s.provider.Embed(ctx, batch)
		if err != nil {
			return nil, fmt.Errorf("batch embed [%d:%d]: %w", start, end, err)
		}
		for i, vec := range vecs {
			all[start+i] = vec
		}
	}
	return all, nil
}

// CosineSimilarity computes the cosine similarity between two float32 vectors.
func CosineSimilarity(a, b []float32) float64 {
	if len(a) == 0 || len(b) == 0 {
		return 0
	}
	dims := len(a)
	if len(b) < dims {
		dims = len(b)
	}
	var dot, normA, normB float64
	for i := 0; i < dims; i++ {
		dot += float64(a[i]) * float64(b[i])
		normA += float64(a[i]) * float64(a[i])
		normB += float64(b[i]) * float64(b[i])
	}
	if normA == 0 || normB == 0 {
		return 0
	}
	return dot / (math.Sqrt(normA) * math.Sqrt(normB))
}
