package cognitive

import (
	"fmt"
	"sort"
	"strings"
	"sync"
	"time"

)

// Case represents a stored problem-solution pair in the case library.
type Case struct {
	ID          string
	WorkspaceID string
	Problem     string
	Solution    string
	Outcome     string // success, partial, failure
	Similarity  float64
	UseCount    int
	CreatedAt   time.Time
}

// AdaptedSolution is the result of retrieving and adapting a past case.
type AdaptedSolution struct {
	OriginalCase    *Case
	AdaptedSolution string
	Confidence      float64
}

// CaseLibrary implements case-based reasoning by storing and retrieving past cases.
type CaseLibrary struct {
	mu    sync.RWMutex
	cases map[string]*Case
}

// NewCaseLibrary creates a new CaseLibrary.
func NewCaseLibrary() *CaseLibrary {
	return &CaseLibrary{
		cases: make(map[string]*Case),
	}
}

// StoreCase adds a new case to the library.
func (cl *CaseLibrary) StoreCase(workspaceID, problem, solution, outcome string) (*Case, error) {
	if problem == "" {
		return nil, fmt.Errorf("problem must not be empty")
	}
	if solution == "" {
		return nil, fmt.Errorf("solution must not be empty")
	}

	c := &Case{
		ID:          newID(),
		WorkspaceID: workspaceID,
		Problem:     problem,
		Solution:    solution,
		Outcome:     outcome,
		Similarity:  0,
		UseCount:    0,
		CreatedAt:   time.Now(),
	}

	cl.mu.Lock()
	cl.cases[c.ID] = c
	cl.mu.Unlock()

	return c, nil
}

// FindSimilar finds the most similar cases to a given problem using keyword overlap.
func (cl *CaseLibrary) FindSimilar(workspaceID, problem string, limit int) []Case {
	cl.mu.RLock()
	defer cl.mu.RUnlock()

	queryWords := tokenize(problem)
	if len(queryWords) == 0 {
		return nil
	}

	type scored struct {
		c    Case
		sim  float64
	}

	var candidates []scored
	for _, c := range cl.cases {
		if c.WorkspaceID != workspaceID {
			continue
		}
		caseWords := tokenize(c.Problem)
		sim := jaccardSimilarity(queryWords, caseWords)
		if sim > 0 {
			cc := *c
			cc.Similarity = sim
			candidates = append(candidates, scored{c: cc, sim: sim})
		}
	}

	sort.Slice(candidates, func(i, j int) bool {
		return candidates[i].sim > candidates[j].sim
	})

	if limit > 0 && limit < len(candidates) {
		candidates = candidates[:limit]
	}

	result := make([]Case, len(candidates))
	for i, sc := range candidates {
		result[i] = sc.c
	}
	return result
}

// RetrieveAndAdapt finds the best matching case and adapts its solution.
func (cl *CaseLibrary) RetrieveAndAdapt(workspaceID, problem string) (*AdaptedSolution, error) {
	similar := cl.FindSimilar(workspaceID, problem, 1)
	if len(similar) == 0 {
		return nil, fmt.Errorf("no similar cases found")
	}

	best := similar[0]

	// Increment use count.
	cl.mu.Lock()
	if orig, ok := cl.cases[best.ID]; ok {
		orig.UseCount++
	}
	cl.mu.Unlock()

	// Adapt solution: prepend context about the new problem.
	adapted := fmt.Sprintf("Based on similar case (similarity=%.2f): %s", best.Similarity, best.Solution)

	// Confidence is similarity * outcome weight.
	outcomeWeight := 0.5
	switch best.Outcome {
	case "success":
		outcomeWeight = 1.0
	case "partial":
		outcomeWeight = 0.6
	case "failure":
		outcomeWeight = 0.2
	}

	return &AdaptedSolution{
		OriginalCase:    &best,
		AdaptedSolution: adapted,
		Confidence:      best.Similarity * outcomeWeight,
	}, nil
}

// PruneLowReuseCases removes cases with use count below the threshold.
// Returns the number of cases removed.
func (cl *CaseLibrary) PruneLowReuseCases(workspaceID string, minUseCount int) int {
	cl.mu.Lock()
	defer cl.mu.Unlock()

	pruned := 0
	for id, c := range cl.cases {
		if c.WorkspaceID == workspaceID && c.UseCount < minUseCount {
			delete(cl.cases, id)
			pruned++
		}
	}
	return pruned
}

// tokenize splits text into lowercase words.
func tokenize(text string) map[string]bool {
	words := make(map[string]bool)
	for _, w := range strings.Fields(strings.ToLower(text)) {
		// Strip common punctuation.
		w = strings.Trim(w, ".,;:!?\"'()[]{}#")
		if len(w) > 1 {
			words[w] = true
		}
	}
	return words
}

// jaccardSimilarity computes the Jaccard similarity between two word sets.
func jaccardSimilarity(a, b map[string]bool) float64 {
	if len(a) == 0 && len(b) == 0 {
		return 0
	}

	intersection := 0
	for w := range a {
		if b[w] {
			intersection++
		}
	}

	union := len(a)
	for w := range b {
		if !a[w] {
			union++
		}
	}

	if union == 0 {
		return 0
	}
	return float64(intersection) / float64(union)
}
