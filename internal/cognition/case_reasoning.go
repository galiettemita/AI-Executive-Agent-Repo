package cognition

import (
	"fmt"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
)

// Case represents a stored case for case-based reasoning.
type Case struct {
	ID           string            `json:"id"`
	WorkspaceID  string            `json:"workspace_id"`
	Problem      string            `json:"problem"`
	Solution     string            `json:"solution"`
	Outcome      string            `json:"outcome"`
	SuccessScore float64           `json:"success_score"`
	Domain       string            `json:"domain"`
	Features     map[string]string `json:"features"`
	CreatedAt    time.Time         `json:"created_at"`
	ReuseCount   int               `json:"reuse_count"`
}

// ScoredCase pairs a case with its similarity score.
type ScoredCase struct {
	Case       Case    `json:"case"`
	Similarity float64 `json:"similarity"`
}

// CaseReasoningEngine implements case-based reasoning.
type CaseReasoningEngine struct {
	mu    sync.Mutex
	cases map[string]*Case
}

// NewCaseReasoningEngine creates a new CaseReasoningEngine.
func NewCaseReasoningEngine() *CaseReasoningEngine {
	return &CaseReasoningEngine{
		cases: make(map[string]*Case),
	}
}

// StoreCase stores a new case.
func (e *CaseReasoningEngine) StoreCase(workspaceID string, problem, solution, outcome string, score float64, features map[string]string) (*Case, error) {
	if strings.TrimSpace(workspaceID) == "" {
		return nil, fmt.Errorf("workspace_id is required")
	}
	if strings.TrimSpace(problem) == "" {
		return nil, fmt.Errorf("problem is required")
	}
	if strings.TrimSpace(solution) == "" {
		return nil, fmt.Errorf("solution is required")
	}

	e.mu.Lock()
	defer e.mu.Unlock()

	id := uuid.Must(uuid.NewV7()).String()
	c := &Case{
		ID:           id,
		WorkspaceID:  workspaceID,
		Problem:      problem,
		Solution:     solution,
		Outcome:      outcome,
		SuccessScore: score,
		Domain:       "",
		Features:     features,
		CreatedAt:    time.Now().UTC(),
		ReuseCount:   0,
	}
	if features == nil {
		c.Features = make(map[string]string)
	}
	e.cases[id] = c
	return c, nil
}

// RetrieveSimilar finds similar past cases by feature overlap and keyword similarity.
func (e *CaseReasoningEngine) RetrieveSimilar(workspaceID, problem string, limit int) []ScoredCase {
	e.mu.Lock()
	defer e.mu.Unlock()

	var scored []ScoredCase
	problemWords := wordSet(problem)

	for _, c := range e.cases {
		if c.WorkspaceID != workspaceID {
			continue
		}

		// Keyword similarity between problems
		caseWords := wordSet(c.Problem)
		similarity := jaccardSimilarity(problemWords, caseWords)

		if similarity > 0 {
			scored = append(scored, ScoredCase{
				Case:       *c,
				Similarity: similarity,
			})
		}
	}

	sort.Slice(scored, func(i, j int) bool {
		return scored[i].Similarity > scored[j].Similarity
	})

	if limit > 0 && len(scored) > limit {
		scored = scored[:limit]
	}
	return scored
}

// AdaptSolution adapts a stored solution to a new problem.
func (e *CaseReasoningEngine) AdaptSolution(caseID string, newProblem string) (string, error) {
	e.mu.Lock()
	defer e.mu.Unlock()

	c, ok := e.cases[caseID]
	if !ok {
		return "", fmt.Errorf("case not found: %s", caseID)
	}

	// Simple adaptation: prefix with context about the new problem
	adapted := fmt.Sprintf("Based on similar case (%s): %s\nAdapted for: %s",
		c.Problem, c.Solution, newProblem)

	return adapted, nil
}

// RecordReuse increments the reuse count for a case.
func (e *CaseReasoningEngine) RecordReuse(caseID string) error {
	e.mu.Lock()
	defer e.mu.Unlock()

	c, ok := e.cases[caseID]
	if !ok {
		return fmt.Errorf("case not found: %s", caseID)
	}
	c.ReuseCount++
	return nil
}

// PruneLowReuse removes cases with fewer than minReuses reuses.
func (e *CaseReasoningEngine) PruneLowReuse(minReuses int) int {
	e.mu.Lock()
	defer e.mu.Unlock()

	pruned := 0
	for id, c := range e.cases {
		if c.ReuseCount < minReuses {
			delete(e.cases, id)
			pruned++
		}
	}
	return pruned
}

// wordSet extracts a set of lowercase words from a string.
func wordSet(s string) map[string]struct{} {
	words := strings.Fields(strings.ToLower(s))
	set := make(map[string]struct{}, len(words))
	for _, w := range words {
		set[w] = struct{}{}
	}
	return set
}

// jaccardSimilarity computes the Jaccard similarity between two word sets.
func jaccardSimilarity(a, b map[string]struct{}) float64 {
	if len(a) == 0 && len(b) == 0 {
		return 0
	}
	intersection := 0
	for w := range a {
		if _, ok := b[w]; ok {
			intersection++
		}
	}
	union := len(a) + len(b) - intersection
	if union == 0 {
		return 0
	}
	return float64(intersection) / float64(union)
}
