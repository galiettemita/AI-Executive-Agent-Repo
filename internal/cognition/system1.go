package cognition

import (
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
)

// Heuristic represents a learned fast-path reasoning pattern
// with confidence-based tracking, used by System1Service.
type Heuristic struct {
	ID           string
	Pattern      string
	Response     string
	SuccessCount int
	FailureCount int
	Confidence   float64
	LearnedFrom  string
	CreatedAt    time.Time
}

// System1FastResult captures the outcome of a System1Service fast decision attempt.
type System1FastResult struct {
	UsedHeuristic bool
	HeuristicID   string
	Response      string
	Confidence    float64
	LatencyMs     int
}

// System1Service implements confidence-based System 1 (fast, heuristic) reasoning
// with word-level matching and automatic confidence decay.
type System1Service struct {
	mu         sync.RWMutex
	heuristics map[string]*Heuristic
}

// NewSystem1Service creates a new System1Service.
func NewSystem1Service() *System1Service {
	return &System1Service{
		heuristics: make(map[string]*Heuristic),
	}
}

// LearnHeuristic creates a new heuristic from a successful execution pattern.
func (s *System1Service) LearnHeuristic(pattern, response, learnedFrom string) (*Heuristic, error) {
	if pattern == "" {
		return nil, fmt.Errorf("pattern must not be empty")
	}
	if response == "" {
		return nil, fmt.Errorf("response must not be empty")
	}

	h := &Heuristic{
		ID:           uuid.Must(uuid.NewV7()).String(),
		Pattern:      pattern,
		Response:     response,
		SuccessCount: 1,
		FailureCount: 0,
		Confidence:   0.5,
		LearnedFrom:  learnedFrom,
		CreatedAt:    time.Now(),
	}

	s.mu.Lock()
	s.heuristics[h.ID] = h
	s.mu.Unlock()

	return h, nil
}

// MatchHeuristic tries to match the input against learned heuristic patterns.
// It returns the best-matching heuristic and true if a match is found.
func (s *System1Service) MatchHeuristic(input string) (*Heuristic, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var best *Heuristic
	bestScore := 0.0

	inputLower := strings.ToLower(input)

	for _, h := range s.heuristics {
		patternLower := strings.ToLower(h.Pattern)

		// Exact match gets highest score.
		if inputLower == patternLower {
			return h, true
		}

		// Substring containment with confidence weighting.
		if strings.Contains(inputLower, patternLower) || strings.Contains(patternLower, inputLower) {
			overlap := float64(len(patternLower))
			maxLen := float64(len(inputLower))
			if maxLen == 0 {
				continue
			}
			score := (overlap / maxLen) * h.Confidence
			if score > bestScore {
				bestScore = score
				best = h
			}
		}

		// Word-level overlap.
		inputWords := strings.Fields(inputLower)
		patternWords := strings.Fields(patternLower)
		if len(patternWords) == 0 {
			continue
		}
		matchCount := 0
		for _, pw := range patternWords {
			for _, iw := range inputWords {
				if iw == pw {
					matchCount++
					break
				}
			}
		}
		wordScore := (float64(matchCount) / float64(len(patternWords))) * h.Confidence
		if wordScore > bestScore {
			bestScore = wordScore
			best = h
		}
	}

	if best != nil && bestScore > 0.1 {
		return best, true
	}
	return nil, false
}

// UpdateHeuristic adjusts the confidence of a heuristic based on success/failure.
func (s *System1Service) UpdateHeuristic(heuristicID string, success bool) {
	s.mu.Lock()
	defer s.mu.Unlock()

	h, ok := s.heuristics[heuristicID]
	if !ok {
		return
	}

	if success {
		h.SuccessCount++
		// Increase confidence, bounded at 1.0.
		h.Confidence += (1.0 - h.Confidence) * 0.1
	} else {
		h.FailureCount++
		// Decrease confidence, bounded at 0.0.
		h.Confidence *= 0.85
	}
}

// PruneHeuristics removes heuristics with confidence below the threshold.
// Returns the number of heuristics removed.
func (s *System1Service) PruneHeuristics(minConfidence float64) int {
	s.mu.Lock()
	defer s.mu.Unlock()

	pruned := 0
	for id, h := range s.heuristics {
		if h.Confidence < minConfidence {
			delete(s.heuristics, id)
			pruned++
		}
	}
	return pruned
}

// System1Decision attempts a fast-path decision using heuristics.
// If a heuristic with confidence > 0.85 is matched, it is used directly.
// Otherwise the result indicates deferral to System 2.
func (s *System1Service) System1Decision(input string) (*System1FastResult, error) {
	if input == "" {
		return nil, fmt.Errorf("input must not be empty")
	}

	start := time.Now()

	h, found := s.MatchHeuristic(input)
	latency := int(time.Since(start).Milliseconds())

	if found && h.Confidence > 0.85 {
		return &System1FastResult{
			UsedHeuristic: true,
			HeuristicID:   h.ID,
			Response:      h.Response,
			Confidence:    h.Confidence,
			LatencyMs:     latency,
		}, nil
	}

	// Defer to System 2.
	return &System1FastResult{
		UsedHeuristic: false,
		LatencyMs:     latency,
	}, nil
}
