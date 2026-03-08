package cognitive

import (
	"fmt"
	"sort"
	"sync"
	"time"

)

// Belief represents a Bayesian belief about a topic.
type Belief struct {
	ID               string
	WorkspaceID      string
	Topic            string
	Prior            float64
	Posterior        float64
	ObservationCount int
	LastObserved     time.Time
	Confidence       float64
}

// BeliefService manages Bayesian belief distributions.
type BeliefService struct {
	mu      sync.RWMutex
	beliefs map[string]*Belief // keyed by ID
}

// NewBeliefService creates a new BeliefService.
func NewBeliefService() *BeliefService {
	return &BeliefService{
		beliefs: make(map[string]*Belief),
	}
}

// RegisterBelief creates a new belief with the given prior probability.
func (b *BeliefService) RegisterBelief(workspaceID, topic string, prior float64) (*Belief, error) {
	if prior < 0 || prior > 1 {
		return nil, fmt.Errorf("prior must be between 0 and 1, got %f", prior)
	}
	if topic == "" {
		return nil, fmt.Errorf("topic must not be empty")
	}

	belief := &Belief{
		ID:               newID(),
		WorkspaceID:      workspaceID,
		Topic:            topic,
		Prior:            prior,
		Posterior:        prior,
		ObservationCount: 0,
		LastObserved:     time.Now(),
		Confidence:       0.5,
	}

	b.mu.Lock()
	b.beliefs[belief.ID] = belief
	b.mu.Unlock()

	return belief, nil
}

// UpdateBelief performs a Bayesian update on a belief.
// P(H|E) = P(E|H)*P(H) / P(E)
// observation=true means evidence supports the belief, false means against.
// strength controls how strongly the evidence affects the update (0-1).
func (b *BeliefService) UpdateBelief(beliefID string, observation bool, strength float64) (*Belief, error) {
	b.mu.Lock()
	defer b.mu.Unlock()

	belief, ok := b.beliefs[beliefID]
	if !ok {
		return nil, fmt.Errorf("belief %s not found", beliefID)
	}

	if strength < 0 || strength > 1 {
		return nil, fmt.Errorf("strength must be between 0 and 1")
	}

	pH := belief.Posterior // P(H)

	var pEgivenH float64  // P(E|H)
	var pEgivenNH float64 // P(E|not H)

	if observation {
		pEgivenH = 0.5 + strength*0.5  // ranges from 0.5 to 1.0
		pEgivenNH = 0.5 - strength*0.4 // ranges from 0.5 to 0.1
	} else {
		pEgivenH = 0.5 - strength*0.4  // ranges from 0.5 to 0.1
		pEgivenNH = 0.5 + strength*0.5 // ranges from 0.5 to 1.0
	}

	// P(E) = P(E|H)*P(H) + P(E|not H)*P(not H)
	pE := pEgivenH*pH + pEgivenNH*(1-pH)

	if pE == 0 {
		return belief, nil
	}

	// Bayes' theorem
	posterior := (pEgivenH * pH) / pE

	// Clamp to [0.001, 0.999] to avoid certainty collapse.
	if posterior < 0.001 {
		posterior = 0.001
	}
	if posterior > 0.999 {
		posterior = 0.999
	}

	belief.Posterior = posterior
	belief.ObservationCount++
	belief.LastObserved = time.Now()

	// Confidence grows with observations.
	belief.Confidence = 1.0 - (1.0 / (1.0 + float64(belief.ObservationCount)*0.3))

	return belief, nil
}

// GetBelief retrieves a belief by workspace and topic.
func (b *BeliefService) GetBelief(workspaceID, topic string) (*Belief, error) {
	b.mu.RLock()
	defer b.mu.RUnlock()

	for _, belief := range b.beliefs {
		if belief.WorkspaceID == workspaceID && belief.Topic == topic {
			return belief, nil
		}
	}
	return nil, fmt.Errorf("belief not found for workspace %s, topic %s", workspaceID, topic)
}

// DecayBeliefs reduces the confidence of beliefs that haven't been observed recently.
// Returns the count of decayed beliefs.
func (b *BeliefService) DecayBeliefs(workspaceID string, decayRate float64) int {
	b.mu.Lock()
	defer b.mu.Unlock()

	decayed := 0
	for _, belief := range b.beliefs {
		if belief.WorkspaceID != workspaceID {
			continue
		}
		belief.Confidence *= (1.0 - decayRate)
		// Pull posterior toward 0.5 (maximum uncertainty).
		belief.Posterior = belief.Posterior + (0.5-belief.Posterior)*decayRate
		decayed++
	}
	return decayed
}

// MostConfident returns the top N most confident beliefs for a workspace.
func (b *BeliefService) MostConfident(workspaceID string, limit int) []Belief {
	return b.sortedBeliefs(workspaceID, limit, true)
}

// LeastConfident returns the top N least confident beliefs for a workspace.
func (b *BeliefService) LeastConfident(workspaceID string, limit int) []Belief {
	return b.sortedBeliefs(workspaceID, limit, false)
}

func (b *BeliefService) sortedBeliefs(workspaceID string, limit int, descending bool) []Belief {
	b.mu.RLock()
	defer b.mu.RUnlock()

	var filtered []Belief
	for _, belief := range b.beliefs {
		if belief.WorkspaceID == workspaceID {
			filtered = append(filtered, *belief)
		}
	}

	sort.Slice(filtered, func(i, j int) bool {
		if descending {
			return filtered[i].Confidence > filtered[j].Confidence
		}
		return filtered[i].Confidence < filtered[j].Confidence
	})

	if limit > 0 && limit < len(filtered) {
		filtered = filtered[:limit]
	}
	return filtered
}
