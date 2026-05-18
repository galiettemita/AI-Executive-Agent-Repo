package prefetch

import (
	"sort"
	"strings"
	"sync"
)

// PrefetchCandidate represents a predicted next query with precomputed result.
type PrefetchCandidate struct {
	Pattern           string  `json:"pattern"`
	Probability       float64 `json:"probability"`
	PrecomputedResult string  `json:"precomputed_result"`
}

// PrefetchService predicts likely next queries and caches precomputed results.
type PrefetchService struct {
	mu sync.RWMutex
	// patterns: workspaceID -> (intent sequence key -> next intent with count)
	patterns map[string]map[string]map[string]int
	// cache: workspaceID -> (query -> precomputed result)
	cache map[string]map[string]string
}

// NewPrefetchService creates a new prefetch service.
func NewPrefetchService() *PrefetchService {
	return &PrefetchService{
		patterns: map[string]map[string]map[string]int{},
		cache:    map[string]map[string]string{},
	}
}

// RecordIntentSequence records a sequence of intents for pattern learning.
func (p *PrefetchService) RecordIntentSequence(workspaceID string, intents []string) {
	if len(intents) < 2 {
		return
	}
	p.mu.Lock()
	defer p.mu.Unlock()

	if p.patterns[workspaceID] == nil {
		p.patterns[workspaceID] = map[string]map[string]int{}
	}

	// Build bigram patterns from intent sequences
	for i := 0; i < len(intents)-1; i++ {
		key := intents[i]
		next := intents[i+1]
		if p.patterns[workspaceID][key] == nil {
			p.patterns[workspaceID][key] = map[string]int{}
		}
		p.patterns[workspaceID][key][next]++
	}
}

// PredictNext predicts likely next queries based on recent intents.
func (p *PrefetchService) PredictNext(workspaceID string, recentIntents []string) []PrefetchCandidate {
	if len(recentIntents) == 0 {
		return nil
	}

	p.mu.RLock()
	defer p.mu.RUnlock()

	wsPatterns, ok := p.patterns[workspaceID]
	if !ok {
		return nil
	}

	// Use the last intent as the key
	lastIntent := recentIntents[len(recentIntents)-1]
	nextMap, ok := wsPatterns[lastIntent]
	if !ok {
		return nil
	}

	// Compute total observations for probability
	total := 0
	for _, count := range nextMap {
		total += count
	}
	if total == 0 {
		return nil
	}

	var candidates []PrefetchCandidate
	for pattern, count := range nextMap {
		prob := float64(count) / float64(total)
		candidates = append(candidates, PrefetchCandidate{
			Pattern:     pattern,
			Probability: prob,
		})
	}

	// Sort by probability descending
	sort.Slice(candidates, func(i, j int) bool {
		return candidates[i].Probability > candidates[j].Probability
	})

	// Fill in any precomputed results from cache
	if wsCache, ok := p.cache[workspaceID]; ok {
		for i := range candidates {
			if result, ok := wsCache[candidates[i].Pattern]; ok {
				candidates[i].PrecomputedResult = result
			}
		}
	}

	return candidates
}

// CachePrefetch caches precomputed results for the given candidates.
// Returns the number of candidates cached.
func (p *PrefetchService) CachePrefetch(workspaceID string, candidates []PrefetchCandidate) int {
	p.mu.Lock()
	defer p.mu.Unlock()

	if p.cache[workspaceID] == nil {
		p.cache[workspaceID] = map[string]string{}
	}

	cached := 0
	for _, c := range candidates {
		if c.PrecomputedResult != "" {
			p.cache[workspaceID][c.Pattern] = c.PrecomputedResult
			cached++
		}
	}
	return cached
}

// GetPrefetched retrieves a cached prediction for a query.
func (p *PrefetchService) GetPrefetched(workspaceID, query string) (string, bool) {
	p.mu.RLock()
	defer p.mu.RUnlock()

	wsCache, ok := p.cache[workspaceID]
	if !ok {
		return "", false
	}

	// Exact match first
	if result, ok := wsCache[query]; ok {
		return result, true
	}

	// Prefix match as fallback
	for pattern, result := range wsCache {
		if strings.HasPrefix(query, pattern) || strings.HasPrefix(pattern, query) {
			return result, true
		}
	}

	return "", false
}
