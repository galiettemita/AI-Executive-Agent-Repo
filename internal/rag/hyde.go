package rag

import (
	"context"
	"fmt"
	"math"
	"strings"
)

// HyDEExpander implements Hypothetical Document Embedding query expansion.
type HyDEExpander struct {
	llm      HyDELLMClient
	embedder EmbeddingProvider
	logger   Logger
	enabled  bool
}

// HyDELLMClient is the minimal interface for HyDE generation.
type HyDELLMClient interface {
	Complete(ctx context.Context, system, user string) (string, error)
}

// Logger is the minimal logging interface used by RAG components.
type Logger interface {
	Info(msg string, args ...any)
	Warn(msg string, args ...any)
	Error(msg string, args ...any)
}

func NewHyDEExpander(llm HyDELLMClient, embedder EmbeddingProvider, logger Logger) *HyDEExpander {
	return &HyDEExpander{llm: llm, embedder: embedder, logger: logger, enabled: true}
}

const hydeSystemPrompt = `You are a document retrieval assistant.
Given a user's question, write a SHORT (2-4 sentences) hypothetical document excerpt
that would directly answer the question if it existed in the knowledge base.

Write as if you are the source document — factual, specific, and concrete.
Do NOT say "I don't know" or hedge. Fabricate plausible, specific content.
Do NOT include preamble. Output ONLY the hypothetical document text.`

// ExpandQuery generates a HyDE vector and returns it averaged with the original query vector.
// Returns (originalVector, nil) if HyDE generation or embedding fails.
func (h *HyDEExpander) ExpandQuery(ctx context.Context, query string) ([]float32, error) {
	if !h.enabled || !h.IsComplexQuery(query) {
		embeddings, err := h.embedder.Embed(ctx, []string{query})
		if err != nil {
			return nil, err
		}
		return embeddings[0], nil
	}

	queryEmbeddings, err := h.embedder.Embed(ctx, []string{query})
	if err != nil {
		return nil, fmt.Errorf("hyde: query embed: %w", err)
	}
	queryVec := queryEmbeddings[0]

	hypoDoc, err := h.llm.Complete(ctx, hydeSystemPrompt,
		fmt.Sprintf("Query: %s\nOutput:", query))
	if err != nil {
		fallbackReason := "LLM error"
		if strings.Contains(err.Error(), "circuit open") {
			fallbackReason = "HyDE circuit open"
		} else if strings.Contains(err.Error(), "deadline exceeded") || strings.Contains(err.Error(), "llm_timeout") {
			fallbackReason = "HyDE LLM timed out"
		}
		h.logger.Warn("hyde: falling back to direct query embedding",
			"reason", fallbackReason,
			"query_words", len(strings.Fields(query)))
		return queryVec, nil
	}

	hypoDoc = strings.TrimSpace(hypoDoc)
	if len(hypoDoc) < 20 {
		return queryVec, nil
	}

	hypoEmbeddings, err := h.embedder.Embed(ctx, []string{hypoDoc})
	if err != nil {
		h.logger.Warn("hyde: hypothetical embed failed; using original query vector",
			"error", err)
		return queryVec, nil
	}
	hypoVec := hypoEmbeddings[0]

	combined := AvgWeightedVec(hypoVec, queryVec, 0.60, 0.40)
	return combined, nil
}

// IsComplexQuery returns true for queries that benefit from HyDE expansion.
func (h *HyDEExpander) IsComplexQuery(query string) bool {
	words := strings.Fields(query)
	if len(words) < 4 {
		return false
	}
	lower := strings.ToLower(query)
	questionWords := []string{"what", "when", "where", "who", "how", "why", "which", "did", "does"}
	for _, qw := range questionWords {
		if strings.HasPrefix(lower, qw+" ") {
			return true
		}
	}
	return len(words) >= 8
}

// AvgWeightedVec returns weightA*a + weightB*b, normalized to unit length.
func AvgWeightedVec(a, b []float32, weightA, weightB float64) []float32 {
	if len(a) != len(b) || len(a) == 0 {
		return a
	}
	result := make([]float32, len(a))
	var magnitude float64
	for i := range a {
		v := float32(weightA)*a[i] + float32(weightB)*b[i]
		result[i] = v
		magnitude += float64(v) * float64(v)
	}
	if magnitude == 0 {
		return result
	}
	mag := float32(math.Sqrt(magnitude))
	for i := range result {
		result[i] /= mag
	}
	return result
}
