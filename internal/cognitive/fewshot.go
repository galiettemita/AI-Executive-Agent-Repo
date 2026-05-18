package cognitive

import (
	"fmt"
	"math"
	"strings"
	"sync"
)

// Exemplar is a single few-shot example.
type Exemplar struct {
	Input   string
	Output  string
	Task    string
	Quality float64
}

// FewShotBuilder constructs dynamic few-shot prompts using MMR selection.
type FewShotBuilder struct {
	mu        sync.RWMutex
	exemplars []Exemplar
}

// NewFewShotBuilder creates a new FewShotBuilder.
func NewFewShotBuilder() *FewShotBuilder {
	return &FewShotBuilder{
		exemplars: []Exemplar{},
	}
}

// AddExemplar adds an exemplar to the builder.
func (f *FewShotBuilder) AddExemplar(exemplar Exemplar) error {
	if exemplar.Input == "" || exemplar.Output == "" {
		return fmt.Errorf("exemplar must have non-empty input and output")
	}
	if exemplar.Task == "" {
		return fmt.Errorf("exemplar must have a task")
	}

	f.mu.Lock()
	f.exemplars = append(f.exemplars, exemplar)
	f.mu.Unlock()

	return nil
}

// SelectExemplars uses Maximal Marginal Relevance to select diverse, relevant exemplars.
func (f *FewShotBuilder) SelectExemplars(task string, queryInput string, maxExemplars int) []Exemplar {
	f.mu.RLock()
	defer f.mu.RUnlock()

	// Filter by task.
	var candidates []Exemplar
	for _, e := range f.exemplars {
		if e.Task == task {
			candidates = append(candidates, e)
		}
	}

	if len(candidates) == 0 {
		return nil
	}
	if maxExemplars <= 0 {
		maxExemplars = 3
	}
	if maxExemplars > len(candidates) {
		maxExemplars = len(candidates)
	}

	lambda := 0.7 // balance between relevance and diversity
	var selected []Exemplar

	for len(selected) < maxExemplars && len(candidates) > 0 {
		best := f.MaximalMarginalRelevance(candidates, selected, lambda)
		if best == nil {
			break
		}

		selected = append(selected, *best)

		// Remove selected from candidates.
		for i, c := range candidates {
			if c.Input == best.Input && c.Output == best.Output {
				candidates = append(candidates[:i], candidates[i+1:]...)
				break
			}
		}
	}

	return selected
}

// MaximalMarginalRelevance selects the next best exemplar balancing relevance and diversity.
// MMR = lambda * sim(candidate, query) - (1-lambda) * max(sim(candidate, selected))
func (f *FewShotBuilder) MaximalMarginalRelevance(candidates []Exemplar, selected []Exemplar, lambda float64) *Exemplar {
	if len(candidates) == 0 {
		return nil
	}

	bestScore := math.Inf(-1)
	var bestExemplar *Exemplar

	for i := range candidates {
		c := &candidates[i]

		// Relevance: similarity to query (using quality as a proxy combined with content).
		relevance := c.Quality

		// Diversity: max similarity to already selected exemplars.
		maxSimToSelected := 0.0
		for _, s := range selected {
			sim := cosineSimilarityBow(c.Input, s.Input)
			if sim > maxSimToSelected {
				maxSimToSelected = sim
			}
		}

		mmrScore := lambda*relevance - (1-lambda)*maxSimToSelected

		if mmrScore > bestScore {
			bestScore = mmrScore
			bestExemplar = c
		}
	}

	return bestExemplar
}

// BuildPrompt formats selected exemplars into a few-shot prompt string.
func (f *FewShotBuilder) BuildPrompt(task string, queryInput string, maxExemplars int) string {
	exemplars := f.SelectExemplars(task, queryInput, maxExemplars)

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("Task: %s\n\n", task))

	for i, e := range exemplars {
		sb.WriteString(fmt.Sprintf("Example %d:\n", i+1))
		sb.WriteString(fmt.Sprintf("Input: %s\n", e.Input))
		sb.WriteString(fmt.Sprintf("Output: %s\n\n", e.Output))
	}

	sb.WriteString("Now complete:\n")
	sb.WriteString(fmt.Sprintf("Input: %s\n", queryInput))
	sb.WriteString("Output:")

	return sb.String()
}

// cosineSimilarityBow computes bag-of-words cosine similarity between two strings.
func cosineSimilarityBow(a, b string) float64 {
	wordsA := strings.Fields(strings.ToLower(a))
	wordsB := strings.Fields(strings.ToLower(b))

	if len(wordsA) == 0 || len(wordsB) == 0 {
		return 0
	}

	freqA := make(map[string]int)
	freqB := make(map[string]int)
	for _, w := range wordsA {
		freqA[w]++
	}
	for _, w := range wordsB {
		freqB[w]++
	}

	// Compute dot product and magnitudes.
	dotProduct := 0.0
	magA := 0.0
	magB := 0.0

	allWords := make(map[string]bool)
	for w := range freqA {
		allWords[w] = true
	}
	for w := range freqB {
		allWords[w] = true
	}

	for w := range allWords {
		fa := float64(freqA[w])
		fb := float64(freqB[w])
		dotProduct += fa * fb
		magA += fa * fa
		magB += fb * fb
	}

	if magA == 0 || magB == 0 {
		return 0
	}

	return dotProduct / (math.Sqrt(magA) * math.Sqrt(magB))
}
