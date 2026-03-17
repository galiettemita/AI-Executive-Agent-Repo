package rag

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"
)

// SelfRAGConfig controls the post-retrieval critique loop.
type SelfRAGConfig struct {
	Enabled            bool
	RelevanceThreshold float64
	MaxRetries         int
}

// DefaultSelfRAGConfig returns production defaults.
func DefaultSelfRAGConfig() SelfRAGConfig {
	return SelfRAGConfig{
		Enabled:            true,
		RelevanceThreshold: 0.55,
		MaxRetries:         2,
	}
}

// SelfRAGLLM is the interface for LLM calls used by the critiquer.
type SelfRAGLLM interface {
	Complete(ctx context.Context, system, user string) (string, error)
}

// critiqueResponse is the JSON shape returned by the critique LLM call.
type critiqueResponse struct {
	Relevance float64 `json:"relevance"`
	Supported bool    `json:"supported"`
	Reason    string  `json:"reason"`
}

// SelfRAGDecision holds the outcome of a critique evaluation round.
type SelfRAGDecision struct {
	PassThreshold  bool
	MaxRelevance   float64
	RewrittenQuery string
	Critiques      []critiqueResponse
	LatencyMs      int64
}

// SelfRAGCritiquer implements the Self-RAG post-retrieval critique loop.
type SelfRAGCritiquer struct {
	llm    SelfRAGLLM
	config SelfRAGConfig
	logger Logger
}

// NewSelfRAGCritiquer creates a new critiquer instance.
func NewSelfRAGCritiquer(llm SelfRAGLLM, config SelfRAGConfig, logger Logger) *SelfRAGCritiquer {
	return &SelfRAGCritiquer{
		llm:    llm,
		config: config,
		logger: logger,
	}
}

const critiqueSystemPrompt = `You are a retrieval quality evaluator. Given a user query and retrieved chunk, score relevance. OUTPUT: JSON only {"relevance":0.0-1.0,"supported":true/false,"reason":"max 15 words"}. 0.0-0.3=unrelated, 0.6-0.8=relevant, 0.8-1.0=directly answers`

const rewriteSystemPrompt = `You are a search query optimizer. The query failed to retrieve relevant documents. Rewrite it to be more specific. OUTPUT: rewritten query only --- no explanation.`

// stripJSONFences removes markdown code fences that LLMs often wrap JSON in.
func stripJSONFences(s string) string {
	s = strings.TrimSpace(s)
	s = strings.TrimPrefix(s, "```json")
	s = strings.TrimPrefix(s, "```")
	s = strings.TrimSuffix(s, "```")
	s = strings.TrimSpace(s)
	return s
}

// Evaluate scores the top-3 retrieval results and optionally rewrites the query.
func (c *SelfRAGCritiquer) Evaluate(ctx context.Context, query string, results []RetrievalResult) (SelfRAGDecision, error) {
	start := time.Now()

	var critiques []critiqueResponse
	maxRelevance := 0.0

	n := min(3, len(results))
	for i := 0; i < n; i++ {
		chunk := results[i]

		snippet := chunk.Snippet
		if len(snippet) > 400 {
			snippet = snippet[:400]
		}

		userMsg := fmt.Sprintf("QUERY: %s\n\nDOCUMENT:\n%s", query, snippet)

		raw, err := c.llm.Complete(ctx, critiqueSystemPrompt, userMsg)
		if err != nil {
			if c.logger != nil {
				c.logger.Warn("self-rag critique LLM error", "error", err)
			}
			continue
		}

		raw = stripJSONFences(raw)

		var cr critiqueResponse
		if err := json.Unmarshal([]byte(raw), &cr); err != nil {
			if c.logger != nil {
				c.logger.Warn("self-rag critique JSON parse error", "error", err)
			}
			continue
		}

		critiques = append(critiques, cr)
		if cr.Relevance > maxRelevance {
			maxRelevance = cr.Relevance
		}
	}

	pass := maxRelevance >= c.config.RelevanceThreshold

	decision := SelfRAGDecision{
		PassThreshold: pass,
		MaxRelevance:  maxRelevance,
		Critiques:     critiques,
		LatencyMs:     time.Since(start).Milliseconds(),
	}

	if !pass && c.config.Enabled {
		rewriteUser := fmt.Sprintf(
			"ORIGINAL QUERY: %s\n\nMAX RELEVANCE SCORE: %.2f\n\nRewrite this query to retrieve more relevant documents.",
			query, maxRelevance,
		)
		rewritten, err := c.llm.Complete(ctx, rewriteSystemPrompt, rewriteUser)
		if err == nil {
			decision.RewrittenQuery = strings.TrimSpace(rewritten)
		}
	}

	return decision, nil
}
