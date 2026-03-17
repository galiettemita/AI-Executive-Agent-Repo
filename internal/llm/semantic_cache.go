package llm

import (
	"context"
	"crypto/sha256"
	"fmt"
	"math"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

// SemanticCacheConfig controls cache behavior. Use DefaultSemanticCacheConfig()
// for production defaults.
type SemanticCacheConfig struct {
	// SimilarityThreshold is the minimum cosine similarity [0,1] required for a
	// cache hit. Plan specifies 0.94.
	SimilarityThreshold float64

	// TTL is how long a cache entry remains valid after creation. Plan specifies 4h.
	TTL time.Duration

	// Enabled allows the cache to be disabled entirely (e.g. in tests that bypass it).
	Enabled bool

	// NeverCacheIntents lists intent strings that must never be cached.
	// Entries ending with "_" are treated as prefix patterns.
	NeverCacheIntents []string
}

// DefaultSemanticCacheConfig returns production-ready cache configuration.
// SimilarityThreshold=0.94, TTL=4h, financial/write intents excluded.
func DefaultSemanticCacheConfig() SemanticCacheConfig {
	return SemanticCacheConfig{
		SimilarityThreshold: 0.94,
		TTL:                 4 * time.Hour,
		Enabled:             true,
		NeverCacheIntents: []string{
			"financial_transfer",
			"email_send",
			"calendar_create",
			"calendar_delete",
			"payment_", // prefix: matches payment_wire, payment_send, etc.
			"trade_",   // prefix: matches trade_execute, trade_cancel, etc.
		},
	}
}

// CacheEntry is a single cached LLM response.
type CacheEntry struct {
	ID          string
	WorkspaceID string
	QueryHash   string    // sha256(lower(trim(QueryText)))
	QueryText   string
	Embedding   []float32 // vector(1536) from OpenAI text-embedding-3-small
	Response    string
	Model       string
	Intent      string
	HitCount    int
	CreatedAt   time.Time
	ExpiresAt   time.Time
}

// CacheEmbedder converts text into embedding vectors.
// Satisfied by OpenAI embedding providers and test mocks.
type CacheEmbedder interface {
	Embed(ctx context.Context, texts []string) ([][]float32, error)
}

// CacheStore is the persistence layer for the semantic cache.
// A nil store causes SemanticCache to fall back to an in-memory slice.
type CacheStore interface {
	// FindSimilar returns entries whose embedding is within threshold cosine
	// similarity of the query embedding, scoped to workspaceID, up to limit results.
	FindSimilar(ctx context.Context, embedding []float32, workspaceID string,
		threshold float64, limit int) ([]CacheEntry, error)

	// Store persists a new cache entry. Implementations must honour the
	// llm_semantic_cache_unique constraint (workspace_id, query_hash) by upserting
	// or ignoring duplicates.
	Store(ctx context.Context, entry CacheEntry) error

	// IncrHit atomically increments hit_count for the entry identified by
	// (queryHash, workspaceID). Best-effort — callers ignore errors.
	IncrHit(ctx context.Context, queryHash, workspaceID string) error
}

// SemanticCache is a semantic deduplication cache for LLM responses.
// It embeds queries and uses cosine similarity to find cached responses
// for paraphrased queries. Financial and write intents are never cached.
//
// When store is nil the cache operates in in-memory fallback mode
// (linear scan, capped at 500 entries). This is suitable for development
// and tests; production must supply a pgvector-backed CacheStore.
type SemanticCache struct {
	mu       sync.RWMutex
	store    CacheStore    // nil → in-memory fallback
	embedder CacheEmbedder
	config   SemanticCacheConfig
	inmem    []CacheEntry // in-memory fallback; capped at 500 entries

	hits   int64 // atomic
	misses int64 // atomic
}

// NewSemanticCache constructs a SemanticCache. Pass store=nil to use the
// in-memory fallback (development/test). The embedder must not be nil.
func NewSemanticCache(store CacheStore, embedder CacheEmbedder, config SemanticCacheConfig) *SemanticCache {
	return &SemanticCache{
		store:    store,
		embedder: embedder,
		config:   config,
		inmem:    make([]CacheEntry, 0, 64),
	}
}

// neverCache returns true if intent must never be stored or retrieved from cache.
func (sc *SemanticCache) neverCache(intent string) bool {
	for _, blocked := range sc.config.NeverCacheIntents {
		if strings.HasSuffix(blocked, "_") {
			if strings.HasPrefix(intent, blocked) {
				return true
			}
		} else {
			if intent == blocked {
				return true
			}
		}
	}
	return false
}

// hashQuery returns the hex-encoded SHA-256 of the normalised query string.
func hashQuery(q string) string {
	normalised := strings.ToLower(strings.TrimSpace(q))
	sum := sha256.Sum256([]byte(normalised))
	return fmt.Sprintf("%x", sum)
}

// cosineSim returns the cosine similarity between two float32 vectors.
func cosineSim(a, b []float32) float64 {
	if len(a) == 0 || len(b) == 0 || len(a) != len(b) {
		return 0.0
	}
	var dot, na, nb float64
	for i := range a {
		fa := float64(a[i])
		fb := float64(b[i])
		dot += fa * fb
		na += fa * fa
		nb += fb * fb
	}
	if na == 0.0 || nb == 0.0 {
		return 0.0
	}
	return dot / (math.Sqrt(na) * math.Sqrt(nb))
}

// Lookup attempts to find a cached response for the given query within the
// specified workspace. Returns the cached response and true on a hit.
func (sc *SemanticCache) Lookup(ctx context.Context, workspaceID, query, intent string) (string, bool) {
	if !sc.config.Enabled || sc.neverCache(intent) {
		return "", false
	}

	embeddings, err := sc.embedder.Embed(ctx, []string{query})
	if err != nil || len(embeddings) == 0 {
		atomic.AddInt64(&sc.misses, 1)
		return "", false
	}
	queryEmbed := embeddings[0]

	// pgvector path
	if sc.store != nil {
		entries, err := sc.store.FindSimilar(ctx, queryEmbed, workspaceID,
			sc.config.SimilarityThreshold, 3)
		if err == nil {
			for _, entry := range entries {
				if entry.ExpiresAt.After(time.Now()) {
					_ = sc.store.IncrHit(ctx, entry.QueryHash, workspaceID)
					atomic.AddInt64(&sc.hits, 1)
					return entry.Response, true
				}
			}
		}
		atomic.AddInt64(&sc.misses, 1)
		return "", false
	}

	// In-memory fallback path
	sc.mu.RLock()
	defer sc.mu.RUnlock()

	now := time.Now()
	for _, entry := range sc.inmem {
		if entry.WorkspaceID != workspaceID {
			continue
		}
		if entry.ExpiresAt.Before(now) {
			continue
		}
		if cosineSim(queryEmbed, entry.Embedding) >= sc.config.SimilarityThreshold {
			atomic.AddInt64(&sc.hits, 1)
			return entry.Response, true
		}
	}

	atomic.AddInt64(&sc.misses, 1)
	return "", false
}

// Put stores an LLM response in the cache. It is best-effort: all errors
// are silently discarded.
func (sc *SemanticCache) Put(ctx context.Context, workspaceID, query, intent, response, model string) {
	if !sc.config.Enabled {
		return
	}
	if sc.neverCache(intent) {
		return
	}
	if len(strings.TrimSpace(response)) < 20 {
		return
	}

	embeddings, err := sc.embedder.Embed(ctx, []string{query})
	if err != nil || len(embeddings) == 0 {
		return
	}

	entry := CacheEntry{
		WorkspaceID: workspaceID,
		QueryHash:   hashQuery(query),
		QueryText:   query,
		Embedding:   embeddings[0],
		Response:    response,
		Model:       model,
		Intent:      intent,
		CreatedAt:   time.Now(),
		ExpiresAt:   time.Now().Add(sc.config.TTL),
	}

	// pgvector path
	if sc.store != nil {
		_ = sc.store.Store(ctx, entry)
		return
	}

	// In-memory fallback path
	sc.mu.Lock()
	defer sc.mu.Unlock()

	if len(sc.inmem) >= 500 {
		sc.inmem = sc.inmem[1:]
	}
	sc.inmem = append(sc.inmem, entry)
}

// Stats returns the cumulative hit and miss counts since construction.
func (sc *SemanticCache) Stats() (hits, misses int64) {
	return atomic.LoadInt64(&sc.hits), atomic.LoadInt64(&sc.misses)
}
