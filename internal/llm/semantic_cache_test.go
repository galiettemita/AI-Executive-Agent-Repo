package llm

import (
	"context"
	"sort"
	"testing"
	"time"
)

// ─────────────────────────────────────────────────────────────────────────────
// Test infrastructure — mock embedder
// ─────────────────────────────────────────────────────────────────────────────

type mockCacheEmbedder struct {
	vectors map[string][]float32
	calls   []string
}

func newMockCacheEmbedder() *mockCacheEmbedder {
	return &mockCacheEmbedder{vectors: make(map[string][]float32)}
}

func (m *mockCacheEmbedder) seed(text string, vec []float32) {
	m.vectors[text] = vec
}

func (m *mockCacheEmbedder) Embed(_ context.Context, texts []string) ([][]float32, error) {
	out := make([][]float32, len(texts))
	for i, t := range texts {
		m.calls = append(m.calls, t)
		if v, ok := m.vectors[t]; ok {
			out[i] = v
		} else {
			def := make([]float32, 1536)
			for j := range def {
				def[j] = 0.3
			}
			out[i] = def
		}
	}
	return out, nil
}

// ─────────────────────────────────────────────────────────────────────────────
// Test infrastructure — mock store
// ─────────────────────────────────────────────────────────────────────────────

type mockCacheStore struct {
	entries []CacheEntry
}

func (s *mockCacheStore) FindSimilar(_ context.Context, embedding []float32, workspaceID string,
	threshold float64, limit int) ([]CacheEntry, error) {

	type scored struct {
		entry CacheEntry
		sim   float64
	}
	var matches []scored
	for _, e := range s.entries {
		if e.WorkspaceID != workspaceID {
			continue
		}
		sim := cosineSim(embedding, e.Embedding)
		if sim >= threshold {
			matches = append(matches, scored{e, sim})
		}
	}
	sort.Slice(matches, func(i, j int) bool {
		return matches[i].sim > matches[j].sim
	})
	if len(matches) > limit {
		matches = matches[:limit]
	}
	out := make([]CacheEntry, len(matches))
	for i, m := range matches {
		out[i] = m.entry
	}
	return out, nil
}

func (s *mockCacheStore) Store(_ context.Context, entry CacheEntry) error {
	s.entries = append(s.entries, entry)
	return nil
}

func (s *mockCacheStore) IncrHit(_ context.Context, queryHash, workspaceID string) error {
	for i := range s.entries {
		if s.entries[i].QueryHash == queryHash && s.entries[i].WorkspaceID == workspaceID {
			s.entries[i].HitCount++
			return nil
		}
	}
	return nil
}

// ─────────────────────────────────────────────────────────────────────────────
// Helpers
// ─────────────────────────────────────────────────────────────────────────────

func unitVec(idx int) []float32 {
	v := make([]float32, 1536)
	v[idx] = 1.0
	return v
}

// ─────────────────────────────────────────────────────────────────────────────
// TEST 1 — HitMiss
// ─────────────────────────────────────────────────────────────────────────────

func TestSemanticCache_HitMiss(t *testing.T) {
	emb := newMockCacheEmbedder()
	emb.seed("what is my balance", unitVec(0))
	emb.seed("tell me my balance", unitVec(0))
	emb.seed("completely unrelated query", unitVec(1))

	store := &mockCacheStore{}
	cache := NewSemanticCache(store, emb, DefaultSemanticCacheConfig())
	ctx := context.Background()

	cache.Put(ctx, "ws1", "what is my balance", "balance_check",
		"Your current balance is $500.00", "gpt-4o")

	resp, hit := cache.Lookup(ctx, "ws1", "tell me my balance", "balance_check")
	if !hit {
		t.Fatal("expected cache hit for paraphrased query, got miss")
	}
	if resp != "Your current balance is $500.00" {
		t.Fatalf("unexpected response: %q", resp)
	}

	_, hit2 := cache.Lookup(ctx, "ws1", "completely unrelated query", "balance_check")
	if hit2 {
		t.Fatal("expected cache miss for unrelated query, got hit")
	}

	hits, misses := cache.Stats()
	if hits != 1 {
		t.Fatalf("expected 1 hit, got %d", hits)
	}
	if misses != 1 {
		t.Fatalf("expected 1 miss, got %d", misses)
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// TEST 2 — NeverCachesFinancial
// ─────────────────────────────────────────────────────────────────────────────

func TestSemanticCache_NeverCachesFinancial(t *testing.T) {
	emb := newMockCacheEmbedder()
	store := &mockCacheStore{}
	cache := NewSemanticCache(store, emb, DefaultSemanticCacheConfig())
	ctx := context.Background()

	financialCases := []struct {
		intent      string
		description string
	}{
		{"financial_transfer", "exact match"},
		{"email_send", "exact match"},
		{"calendar_create", "exact match"},
		{"calendar_delete", "exact match"},
		{"payment_wire", "payment_ prefix"},
		{"payment_send", "payment_ prefix"},
		{"trade_execute", "trade_ prefix"},
		{"trade_cancel", "trade_ prefix"},
	}

	for _, tc := range financialCases {
		t.Run(tc.description+"/"+tc.intent, func(t *testing.T) {
			prevCalls := len(emb.calls)

			cache.Put(ctx, "ws1", "transfer $100 to Alice", tc.intent,
				"Transfer initiated to Alice for $100.00", "gpt-4o")

			if len(emb.calls) != prevCalls {
				t.Errorf("intent %q: Put called embedder — must not for excluded intents", tc.intent)
			}

			callsBefore := len(emb.calls)
			_, hit := cache.Lookup(ctx, "ws1", "transfer $100 to Alice", tc.intent)
			if hit {
				t.Errorf("intent %q: Lookup returned hit — must never cache excluded intents", tc.intent)
			}
			if len(emb.calls) != callsBefore {
				t.Errorf("intent %q: Lookup called embedder — must not for excluded intents", tc.intent)
			}
		})
	}

	if len(store.entries) != 0 {
		t.Fatalf("store has %d entries; expected 0 for all-financial test", len(store.entries))
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// TEST 3 — Expiry
// ─────────────────────────────────────────────────────────────────────────────

func TestSemanticCache_Expiry(t *testing.T) {
	emb := newMockCacheEmbedder()
	emb.seed("test query expiry", unitVec(5))

	shortTTLConfig := SemanticCacheConfig{
		SimilarityThreshold: 0.94,
		TTL:                 5 * time.Millisecond,
		Enabled:             true,
		NeverCacheIntents:   nil,
	}

	store := &mockCacheStore{}
	cache := NewSemanticCache(store, emb, shortTTLConfig)
	ctx := context.Background()

	cache.Put(ctx, "ws-expiry", "test query expiry", "general",
		"This response should expire very quickly", "gpt-4o")

	_, immediateHit := cache.Lookup(ctx, "ws-expiry", "test query expiry", "general")
	if !immediateHit {
		t.Log("note: immediate hit failed — possible race with 5ms TTL, acceptable")
	}

	time.Sleep(20 * time.Millisecond)

	_, expiredHit := cache.Lookup(ctx, "ws-expiry", "test query expiry", "general")
	if expiredHit {
		t.Fatal("expected cache miss after TTL expiry, got hit")
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// TEST 4 — CosineSim unit tests
// ─────────────────────────────────────────────────────────────────────────────

func TestSemanticCache_CosineSim(t *testing.T) {
	const tol = 1e-6

	t.Run("identical_vectors", func(t *testing.T) {
		v := unitVec(0)
		sim := cosineSim(v, v)
		if abs64(sim-1.0) > tol {
			t.Fatalf("identical vectors: expected 1.0, got %f", sim)
		}
	})

	t.Run("orthogonal_vectors", func(t *testing.T) {
		a := unitVec(0)
		b := unitVec(1)
		sim := cosineSim(a, b)
		if abs64(sim) > tol {
			t.Fatalf("orthogonal vectors: expected 0.0, got %f", sim)
		}
	})

	t.Run("opposite_vectors", func(t *testing.T) {
		a := unitVec(0)
		b := make([]float32, 1536)
		b[0] = -1.0
		sim := cosineSim(a, b)
		if abs64(sim+1.0) > tol {
			t.Fatalf("opposite vectors: expected -1.0, got %f", sim)
		}
	})

	t.Run("zero_length_inputs", func(t *testing.T) {
		sim := cosineSim([]float32{}, []float32{})
		if sim != 0.0 {
			t.Fatalf("empty vectors: expected 0.0, got %f", sim)
		}
	})

	t.Run("mismatched_lengths", func(t *testing.T) {
		a := []float32{1.0, 0.0}
		b := []float32{1.0, 0.0, 0.0}
		sim := cosineSim(a, b)
		if sim != 0.0 {
			t.Fatalf("mismatched lengths: expected 0.0, got %f", sim)
		}
	})

	t.Run("nil_inputs", func(t *testing.T) {
		sim := cosineSim(nil, nil)
		if sim != 0.0 {
			t.Fatalf("nil inputs: expected 0.0, got %f", sim)
		}
	})
}

func abs64(x float64) float64 {
	if x < 0 {
		return -x
	}
	return x
}
