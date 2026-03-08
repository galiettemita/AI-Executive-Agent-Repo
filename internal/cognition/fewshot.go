package cognition

import (
	"fmt"
	"sort"
	"strings"
	"sync"
)

// Exemplar represents a few-shot example.
type Exemplar struct {
	Input     string    `json:"input"`
	Output    string    `json:"output"`
	Domain    string    `json:"domain"`
	Quality   float64   `json:"quality"`
	Embedding []float32 `json:"embedding"`
}

// FewShotBuilder constructs few-shot prompts using MMR-based selection.
type FewShotBuilder struct {
	mu        sync.Mutex
	exemplars []Exemplar
}

// NewFewShotBuilder creates a new FewShotBuilder.
func NewFewShotBuilder() *FewShotBuilder {
	return &FewShotBuilder{
		exemplars: []Exemplar{},
	}
}

// AddExemplar adds an exemplar to the pool.
func (b *FewShotBuilder) AddExemplar(exemplar Exemplar) error {
	if strings.TrimSpace(exemplar.Input) == "" {
		return fmt.Errorf("input is required")
	}
	if strings.TrimSpace(exemplar.Output) == "" {
		return fmt.Errorf("output is required")
	}

	b.mu.Lock()
	defer b.mu.Unlock()

	b.exemplars = append(b.exemplars, exemplar)
	return nil
}

// SelectExemplars uses MMR-based selection to maximize relevance and diversity.
// score = lambda * relevance - (1-lambda) * max_similarity_to_selected (lambda=0.7)
func (b *FewShotBuilder) SelectExemplars(query string, domain string, maxCount int) []Exemplar {
	b.mu.Lock()
	defer b.mu.Unlock()

	if maxCount <= 0 || len(b.exemplars) == 0 {
		return nil
	}

	// Filter by domain if specified
	var candidates []Exemplar
	for _, e := range b.exemplars {
		if domain != "" && e.Domain != domain {
			continue
		}
		candidates = append(candidates, e)
	}

	if len(candidates) == 0 {
		return nil
	}

	const lambda = 0.7
	queryWords := wordSet(query)
	selected := make([]Exemplar, 0, maxCount)
	used := make(map[int]bool)

	for len(selected) < maxCount && len(selected) < len(candidates) {
		bestIdx := -1
		bestScore := -1e9

		for i, c := range candidates {
			if used[i] {
				continue
			}

			relevance := KeywordSimilarity(query, c.Input)

			// Max similarity to already selected
			maxSim := 0.0
			for _, s := range selected {
				sim := KeywordSimilarity(c.Input, s.Input)
				if sim > maxSim {
					maxSim = sim
				}
			}

			score := lambda*relevance - (1-lambda)*maxSim

			// Quality bonus
			score += c.Quality * 0.1

			if score > bestScore {
				bestScore = score
				bestIdx = i
			}
		}

		if bestIdx < 0 {
			break
		}

		used[bestIdx] = true
		selected = append(selected, candidates[bestIdx])
	}

	_ = queryWords // queryWords used indirectly via KeywordSimilarity
	return selected
}

// BuildPrompt formats selected exemplars into a few-shot prompt.
func (b *FewShotBuilder) BuildPrompt(systemPrompt string, exemplars []Exemplar, query string) string {
	var sb strings.Builder

	if systemPrompt != "" {
		sb.WriteString(systemPrompt)
		sb.WriteString("\n\n")
	}

	for i, e := range exemplars {
		sb.WriteString(fmt.Sprintf("Example %d:\n", i+1))
		sb.WriteString(fmt.Sprintf("Input: %s\n", e.Input))
		sb.WriteString(fmt.Sprintf("Output: %s\n\n", e.Output))
	}

	sb.WriteString(fmt.Sprintf("Input: %s\n", query))
	sb.WriteString("Output:")

	return sb.String()
}

// KeywordSimilarity computes Jaccard similarity between word sets of two strings.
func KeywordSimilarity(a, b string) float64 {
	setA := wordSet(a)
	setB := wordSet(b)
	return jaccardSimilarity(setA, setB)
}

// sortExemplarsByQuality sorts exemplars by quality (for deterministic tests if needed).
func sortExemplarsByQuality(exemplars []Exemplar) {
	sort.Slice(exemplars, func(i, j int) bool {
		return exemplars[i].Quality > exemplars[j].Quality
	})
}
