package experiments

import (
	"fmt"
	"hash/fnv"
	"sync"
	"time"

	"github.com/brevio/brevio/internal/determinism"
)

// Variant represents a variant in an A/B experiment.
type Variant struct {
	ID          string
	Name        string
	Weight      float64
	Conversions int
	Impressions int
}

// Experiment represents an A/B experiment.
type Experiment struct {
	ID          string
	WorkspaceID string
	Name        string
	Status      string // draft, running, stopped
	Variants    []Variant
	StartedAt   *time.Time
	EndedAt     *time.Time
}

// ExperimentService manages A/B experiments.
type ExperimentService struct {
	mu          sync.Mutex
	experiments map[string]*Experiment
	assignments map[string]string // experimentID:userID -> variantID
}

// NewExperimentService creates a new ExperimentService.
func NewExperimentService() *ExperimentService {
	return &ExperimentService{
		experiments: make(map[string]*Experiment),
		assignments: make(map[string]string),
	}
}

// CreateExperiment creates a new A/B experiment with variants.
func (s *ExperimentService) CreateExperiment(workspaceID, name string, variants []Variant) (*Experiment, error) {
	if name == "" {
		return nil, fmt.Errorf("experiment name must not be empty")
	}
	if len(variants) < 2 {
		return nil, fmt.Errorf("experiment must have at least 2 variants")
	}

	expID, err := determinism.NewUUIDv7()
	if err != nil {
		return nil, fmt.Errorf("generate experiment id: %w", err)
	}

	// Assign IDs to variants and normalize weights.
	totalWeight := 0.0
	for i := range variants {
		vid, err := determinism.NewUUIDv7()
		if err != nil {
			return nil, fmt.Errorf("generate variant id: %w", err)
		}
		variants[i].ID = vid.String()
		totalWeight += variants[i].Weight
	}

	// Normalize weights to sum to 1.
	if totalWeight > 0 {
		for i := range variants {
			variants[i].Weight /= totalWeight
		}
	}

	now := time.Now()
	exp := &Experiment{
		ID:          expID.String(),
		WorkspaceID: workspaceID,
		Name:        name,
		Status:      "running",
		Variants:    variants,
		StartedAt:   &now,
	}

	s.mu.Lock()
	s.experiments[exp.ID] = exp
	s.mu.Unlock()

	return exp, nil
}

// AssignVariant deterministically assigns a user to a variant.
func (s *ExperimentService) AssignVariant(experimentID, userID string) (*Variant, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	exp, ok := s.experiments[experimentID]
	if !ok {
		return nil, fmt.Errorf("experiment %s not found", experimentID)
	}
	if exp.Status != "running" {
		return nil, fmt.Errorf("experiment %s is not running (status: %s)", experimentID, exp.Status)
	}

	assignKey := experimentID + ":" + userID

	// Return existing assignment if present.
	if variantID, exists := s.assignments[assignKey]; exists {
		for i := range exp.Variants {
			if exp.Variants[i].ID == variantID {
				return &exp.Variants[i], nil
			}
		}
	}

	// Deterministic assignment using hash.
	h := fnv.New32a()
	h.Write([]byte(assignKey))
	hashVal := h.Sum32()

	// Select variant based on cumulative weights.
	normalized := float64(hashVal) / float64(^uint32(0))
	cumulative := 0.0
	for i := range exp.Variants {
		cumulative += exp.Variants[i].Weight
		if normalized <= cumulative {
			exp.Variants[i].Impressions++
			s.assignments[assignKey] = exp.Variants[i].ID
			return &exp.Variants[i], nil
		}
	}

	// Fallback to last variant.
	last := &exp.Variants[len(exp.Variants)-1]
	last.Impressions++
	s.assignments[assignKey] = last.ID
	return last, nil
}

// RecordConversion records a conversion for a user in an experiment.
func (s *ExperimentService) RecordConversion(experimentID, userID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	exp, ok := s.experiments[experimentID]
	if !ok {
		return fmt.Errorf("experiment %s not found", experimentID)
	}

	assignKey := experimentID + ":" + userID
	variantID, exists := s.assignments[assignKey]
	if !exists {
		return fmt.Errorf("user %s not assigned to experiment %s", userID, experimentID)
	}

	for i := range exp.Variants {
		if exp.Variants[i].ID == variantID {
			exp.Variants[i].Conversions++
			return nil
		}
	}

	return fmt.Errorf("variant %s not found", variantID)
}

// GetResults returns the experiment with current results.
func (s *ExperimentService) GetResults(experimentID string) (*Experiment, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	exp, ok := s.experiments[experimentID]
	if !ok {
		return nil, fmt.Errorf("experiment %s not found", experimentID)
	}
	return exp, nil
}

// StopExperiment stops a running experiment.
func (s *ExperimentService) StopExperiment(experimentID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	exp, ok := s.experiments[experimentID]
	if !ok {
		return fmt.Errorf("experiment %s not found", experimentID)
	}
	if exp.Status != "running" {
		return fmt.Errorf("experiment %s is already %s", experimentID, exp.Status)
	}

	now := time.Now()
	exp.Status = "stopped"
	exp.EndedAt = &now
	return nil
}
