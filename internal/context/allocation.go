package contextlayer

import (
	"sort"
	"strings"
	"sync"
)

// AllocationStrategy defines how a source should be allocated tokens.
type AllocationStrategy struct {
	SourceName string
	Strategy   string // "full", "recency", "relevance", "diversity_mmr"
	MaxTokens  int
	Config     map[string]string
}

// ContextAllocator allocates context from multiple sources using strategies.
type ContextAllocator struct {
	mu sync.Mutex
}

// NewContextAllocator creates a new ContextAllocator.
func NewContextAllocator() *ContextAllocator {
	return &ContextAllocator{}
}

// Allocate applies allocation strategies to sources and returns selected items per source.
func (ca *ContextAllocator) Allocate(sources map[string][]string, strategies []AllocationStrategy) map[string][]string {
	ca.mu.Lock()
	defer ca.mu.Unlock()

	result := map[string][]string{}
	strategyMap := map[string]AllocationStrategy{}
	for _, s := range strategies {
		strategyMap[s.SourceName] = s
	}

	for sourceName, items := range sources {
		strategy, ok := strategyMap[sourceName]
		if !ok {
			strategy = AllocationStrategy{
				SourceName: sourceName,
				Strategy:   "full",
				MaxTokens:  4096,
			}
		}

		selected := applyStrategy(items, strategy)
		result[sourceName] = selected
	}

	return result
}

func applyStrategy(items []string, strategy AllocationStrategy) []string {
	if len(items) == 0 {
		return nil
	}

	maxItems := strategy.MaxTokens / 100 // approximate: 100 tokens per item
	if maxItems <= 0 {
		maxItems = len(items)
	}

	switch strings.ToLower(strategy.Strategy) {
	case "full":
		return applyFull(items, maxItems)
	case "recency":
		return applyRecency(items, maxItems)
	case "relevance":
		return applyRelevance(items, maxItems, strategy.Config)
	case "diversity_mmr":
		return applyDiversityMMR(items, maxItems)
	default:
		return applyFull(items, maxItems)
	}
}

// applyFull returns all items up to the limit.
func applyFull(items []string, maxItems int) []string {
	if len(items) <= maxItems {
		out := make([]string, len(items))
		copy(out, items)
		return out
	}
	out := make([]string, maxItems)
	copy(out, items[:maxItems])
	return out
}

// applyRecency returns the newest items (last in slice order).
func applyRecency(items []string, maxItems int) []string {
	if len(items) <= maxItems {
		out := make([]string, len(items))
		copy(out, items)
		return out
	}
	start := len(items) - maxItems
	out := make([]string, maxItems)
	copy(out, items[start:])
	return out
}

// applyRelevance returns items sorted by keyword relevance to a query.
func applyRelevance(items []string, maxItems int, config map[string]string) []string {
	query := ""
	if config != nil {
		query = config["query"]
	}
	if query == "" {
		return applyFull(items, maxItems)
	}

	queryWords := strings.Fields(strings.ToLower(query))
	querySet := map[string]struct{}{}
	for _, w := range queryWords {
		querySet[w] = struct{}{}
	}

	type scored struct {
		item  string
		score int
	}

	scoredItems := make([]scored, 0, len(items))
	for _, item := range items {
		words := strings.Fields(strings.ToLower(item))
		matches := 0
		for _, w := range words {
			if _, ok := querySet[w]; ok {
				matches++
			}
		}
		scoredItems = append(scoredItems, scored{item: item, score: matches})
	}

	sort.Slice(scoredItems, func(i, j int) bool {
		return scoredItems[i].score > scoredItems[j].score
	})

	if len(scoredItems) > maxItems {
		scoredItems = scoredItems[:maxItems]
	}

	out := make([]string, 0, len(scoredItems))
	for _, si := range scoredItems {
		out = append(out, si.item)
	}
	return out
}

// applyDiversityMMR applies Maximal Marginal Relevance to select diverse items.
func applyDiversityMMR(items []string, maxItems int) []string {
	if len(items) <= maxItems {
		out := make([]string, len(items))
		copy(out, items)
		return out
	}

	selected := []string{}
	remaining := make([]string, len(items))
	copy(remaining, items)

	// Always include the first item
	selected = append(selected, remaining[0])
	remaining = remaining[1:]

	for len(selected) < maxItems && len(remaining) > 0 {
		bestIdx := 0
		bestScore := -1.0

		for i, candidate := range remaining {
			// Compute diversity: how different is this from already selected items
			minSimilarity := 1.0
			candidateWords := strings.Fields(strings.ToLower(candidate))
			for _, sel := range selected {
				selWords := strings.Fields(strings.ToLower(sel))
				sim := wordOverlap(candidateWords, selWords)
				if sim < minSimilarity {
					minSimilarity = sim
				}
			}
			// MMR score = diversity (1 - max_similarity)
			diversityScore := 1.0 - minSimilarity
			if diversityScore > bestScore {
				bestScore = diversityScore
				bestIdx = i
			}
		}

		selected = append(selected, remaining[bestIdx])
		remaining = append(remaining[:bestIdx], remaining[bestIdx+1:]...)
	}

	return selected
}

// wordOverlap computes the fraction of words in a that also appear in b.
func wordOverlap(a, b []string) float64 {
	if len(a) == 0 || len(b) == 0 {
		return 0
	}
	bSet := map[string]struct{}{}
	for _, w := range b {
		bSet[w] = struct{}{}
	}
	matches := 0
	for _, w := range a {
		if _, ok := bSet[w]; ok {
			matches++
		}
	}
	return float64(matches) / float64(len(a))
}
