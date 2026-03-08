package cognition

import (
	"fmt"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
)

// BeliefDistribution represents a Bayesian belief about a hypothesis.
type BeliefDistribution struct {
	ID               string    `json:"id"`
	WorkspaceID      string    `json:"workspace_id"`
	Hypothesis       string    `json:"hypothesis"`
	Prior            float64   `json:"prior"`
	Likelihood       float64   `json:"likelihood"`
	Posterior        float64   `json:"posterior"`
	Evidence         []string  `json:"evidence"`
	ObservationCount int       `json:"observation_count"`
	LastUpdatedAt    time.Time `json:"last_updated_at"`
}

// BayesianEngine manages Bayesian belief distributions.
type BayesianEngine struct {
	mu      sync.Mutex
	beliefs map[string]*BeliefDistribution // keyed by ID
}

// NewBayesianEngine creates a new BayesianEngine.
func NewBayesianEngine() *BayesianEngine {
	return &BayesianEngine{
		beliefs: make(map[string]*BeliefDistribution),
	}
}

// CreateBelief creates a new belief distribution for a hypothesis.
func (e *BayesianEngine) CreateBelief(workspaceID, hypothesis string, prior float64) (*BeliefDistribution, error) {
	if strings.TrimSpace(workspaceID) == "" {
		return nil, fmt.Errorf("workspace_id is required")
	}
	if strings.TrimSpace(hypothesis) == "" {
		return nil, fmt.Errorf("hypothesis is required")
	}
	if prior < 0 || prior > 1 {
		return nil, fmt.Errorf("prior must be between 0 and 1")
	}

	e.mu.Lock()
	defer e.mu.Unlock()

	id := uuid.Must(uuid.NewV7()).String()
	b := &BeliefDistribution{
		ID:               id,
		WorkspaceID:      workspaceID,
		Hypothesis:       hypothesis,
		Prior:            prior,
		Likelihood:       0.5,
		Posterior:         prior,
		Evidence:         []string{},
		ObservationCount: 0,
		LastUpdatedAt:    time.Now().UTC(),
	}

	e.beliefs[id] = b
	return b, nil
}

// UpdateBelief performs a Bayesian update: posterior = (prior * likelihood) / normalizer.
func (e *BayesianEngine) UpdateBelief(beliefID string, evidence string, likelihood float64) error {
	if strings.TrimSpace(evidence) == "" {
		return fmt.Errorf("evidence is required")
	}
	if likelihood < 0 || likelihood > 1 {
		return fmt.Errorf("likelihood must be between 0 and 1")
	}

	e.mu.Lock()
	defer e.mu.Unlock()

	b, ok := e.beliefs[beliefID]
	if !ok {
		return fmt.Errorf("belief not found: %s", beliefID)
	}

	// Bayesian update: P(H|E) = P(E|H) * P(H) / P(E)
	// P(E) = P(E|H)*P(H) + P(E|~H)*P(~H)
	pEGivenH := likelihood
	pH := b.Posterior // use current posterior as new prior
	pEGivenNotH := 1.0 - likelihood
	pNotH := 1.0 - pH

	normalizer := pEGivenH*pH + pEGivenNotH*pNotH
	if normalizer == 0 {
		normalizer = 1e-10
	}

	b.Posterior = (pEGivenH * pH) / normalizer
	b.Likelihood = likelihood
	b.Prior = pH
	b.Evidence = append(b.Evidence, evidence)
	b.ObservationCount++
	b.LastUpdatedAt = time.Now().UTC()

	return nil
}

// GetBelief returns a belief distribution by workspace and hypothesis.
func (e *BayesianEngine) GetBelief(workspaceID, hypothesis string) (*BeliefDistribution, error) {
	e.mu.Lock()
	defer e.mu.Unlock()

	for _, b := range e.beliefs {
		if b.WorkspaceID == workspaceID && b.Hypothesis == hypothesis {
			return b, nil
		}
	}
	return nil, fmt.Errorf("belief not found for hypothesis: %s", hypothesis)
}

// GetStrongestBeliefs returns beliefs sorted by posterior, limited to the given count.
func (e *BayesianEngine) GetStrongestBeliefs(workspaceID string, limit int) []BeliefDistribution {
	e.mu.Lock()
	defer e.mu.Unlock()

	var results []BeliefDistribution
	for _, b := range e.beliefs {
		if b.WorkspaceID == workspaceID {
			results = append(results, *b)
		}
	}

	sort.Slice(results, func(i, j int) bool {
		return results[i].Posterior > results[j].Posterior
	})

	if limit > 0 && len(results) > limit {
		results = results[:limit]
	}
	return results
}

// DecayBeliefs reduces confidence of beliefs with few observations.
func (e *BayesianEngine) DecayBeliefs(workspaceID string, decayRate float64) int {
	e.mu.Lock()
	defer e.mu.Unlock()

	decayed := 0
	for _, b := range e.beliefs {
		if b.WorkspaceID != workspaceID {
			continue
		}
		if b.ObservationCount < 3 {
			b.Posterior *= (1.0 - decayRate)
			if b.Posterior < 0.01 {
				b.Posterior = 0.01
			}
			b.LastUpdatedAt = time.Now().UTC()
			decayed++
		}
	}
	return decayed
}
