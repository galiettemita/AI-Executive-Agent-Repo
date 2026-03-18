package caching

import (
	"context"
	"crypto/sha256"
	"fmt"
)

// hashIntent returns a hex SHA-256 of the intent string for use as a cache key.
func hashIntent(intent string) string {
	h := sha256.Sum256([]byte(intent))
	return fmt.Sprintf("llm_bridge:%x", h)
}

// SemanticLookup abstracts the semantic cache so the bridge can be tested
// without a real pgvector-backed SemanticCache.
type SemanticLookup interface {
	Lookup(ctx context.Context, workspaceID, query, intent string) (string, bool)
	Put(ctx context.Context, workspaceID, query, intent, response, model string)
}

// LLMCacheBridge unifies the 3-layer policy cache (L1/L2/L3) with the LLM semantic
// pgvector cache. Lookup order: L1→L2→L3 (exact hash) → semantic (cosine ≥ 0.94).
// On semantic hit, the result is written through to L1/L2/L3 via PutEntry so that
// subsequent identical requests are served from fast in-memory L1.
type LLMCacheBridge struct {
	semantic SemanticLookup
	layers   *Service
}

// NewLLMCacheBridge creates a new bridge. Returns an error if either dependency is nil.
func NewLLMCacheBridge(semantic SemanticLookup, layers *Service) (*LLMCacheBridge, error) {
	if semantic == nil {
		return nil, fmt.Errorf("caching.NewLLMCacheBridge: semantic must not be nil")
	}
	if layers == nil {
		return nil, fmt.Errorf("caching.NewLLMCacheBridge: layers must not be nil")
	}
	return &LLMCacheBridge{semantic: semantic, layers: layers}, nil
}

// Lookup performs a layered cache lookup for a given workspaceID and query.
// Returns the cached response string and true on hit, empty string and false on miss.
func (b *LLMCacheBridge) Lookup(ctx context.Context, workspaceID, query, intent string) (string, bool) {
	key := hashIntent(query)

	// L1 → L2 → L3 exact hash lookup via the 3-layer cache service.
	if cached, ok := b.layers.GetEntry(workspaceID, key); ok {
		return cached, true
	}

	// Semantic cache: pgvector cosine similarity ≥ 0.94.
	if response, ok := b.semantic.Lookup(ctx, workspaceID, query, intent); ok {
		// Write-through: populate L1/L2/L3 so the next identical request
		// is served from fast layers without hitting pgvector again.
		b.layers.PutEntry(workspaceID, key, response)
		return response, true
	}

	return "", false
}

// Put stores an LLM response in all cache layers (L1/L2/L3 and semantic).
func (b *LLMCacheBridge) Put(ctx context.Context, workspaceID, query, intent, response, model string) {
	key := hashIntent(query)
	b.layers.PutEntry(workspaceID, key, response)
	b.semantic.Put(ctx, workspaceID, query, intent, response, model)
}
