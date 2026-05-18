package cognition

import (
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
)

// System1Heuristic represents a learned fast-path response pattern.
type System1Heuristic struct {
	ID           string  `json:"id"`
	Pattern      string  `json:"pattern"`
	Response     string  `json:"response"`
	SuccessCount int     `json:"success_count"`
	FailCount    int     `json:"fail_count"`
	AvgLatencyMs float64 `json:"avg_latency_ms"`
	Domain       string  `json:"domain"`
	LearnedFrom  string  `json:"learned_from"`
}

// System1Result is the outcome of a System 1 fast-path match.
type System1Result struct {
	HeuristicID string  `json:"heuristic_id"`
	Response    string  `json:"response"`
	Confidence  float64 `json:"confidence"`
	LatencyMs   float64 `json:"latency_ms"`
}

// DualProcessEngine implements System 1 (fast heuristic) and System 2 (slow deliberate) reasoning.
type DualProcessEngine struct {
	mu         sync.Mutex
	heuristics map[string]*System1Heuristic
}

// NewDualProcessEngine creates a new DualProcessEngine.
func NewDualProcessEngine() *DualProcessEngine {
	return &DualProcessEngine{
		heuristics: make(map[string]*System1Heuristic),
	}
}

// LearnHeuristic creates a heuristic from a successful execution.
func (e *DualProcessEngine) LearnHeuristic(pattern, response, domain, learnedFrom string) (*System1Heuristic, error) {
	if strings.TrimSpace(pattern) == "" {
		return nil, fmt.Errorf("pattern is required")
	}
	if strings.TrimSpace(response) == "" {
		return nil, fmt.Errorf("response is required")
	}

	e.mu.Lock()
	defer e.mu.Unlock()

	id := uuid.Must(uuid.NewV7()).String()
	h := &System1Heuristic{
		ID:           id,
		Pattern:      pattern,
		Response:     response,
		SuccessCount: 1,
		FailCount:    0,
		AvgLatencyMs: 0,
		Domain:       domain,
		LearnedFrom:  learnedFrom,
	}
	e.heuristics[id] = h
	return h, nil
}

// System1Match performs a fast pattern match against stored heuristics.
func (e *DualProcessEngine) System1Match(input string) (*System1Result, bool) {
	start := time.Now()

	e.mu.Lock()
	defer e.mu.Unlock()

	inputLower := strings.ToLower(input)
	var bestMatch *System1Heuristic
	var bestScore float64

	for _, h := range e.heuristics {
		patternLower := strings.ToLower(h.Pattern)
		if strings.Contains(inputLower, patternLower) || strings.Contains(patternLower, inputLower) {
			total := h.SuccessCount + h.FailCount
			if total == 0 {
				continue
			}
			score := float64(h.SuccessCount) / float64(total)
			if score > bestScore {
				bestScore = score
				bestMatch = h
			}
		}
	}

	if bestMatch == nil {
		return nil, false
	}

	elapsed := float64(time.Since(start).Microseconds()) / 1000.0
	return &System1Result{
		HeuristicID: bestMatch.ID,
		Response:    bestMatch.Response,
		Confidence:  bestScore,
		LatencyMs:   elapsed,
	}, true
}

// ShouldEscalateToSystem2 determines if the input should be handled by slower deliberate reasoning.
func (e *DualProcessEngine) ShouldEscalateToSystem2(input string, system1Result *System1Result) bool {
	if system1Result == nil {
		return true
	}
	if system1Result.Confidence < 0.7 {
		return true
	}
	if e.IsComplex(input) {
		return true
	}
	return false
}

// RecordOutcome updates success/fail counts for a heuristic.
func (e *DualProcessEngine) RecordOutcome(heuristicID string, success bool) {
	e.mu.Lock()
	defer e.mu.Unlock()

	h, ok := e.heuristics[heuristicID]
	if !ok {
		return
	}
	if success {
		h.SuccessCount++
	} else {
		h.FailCount++
	}
}

// PruneIneffective removes heuristics with a success rate below minSuccessRate.
func (e *DualProcessEngine) PruneIneffective(minSuccessRate float64) int {
	e.mu.Lock()
	defer e.mu.Unlock()

	pruned := 0
	for id, h := range e.heuristics {
		total := h.SuccessCount + h.FailCount
		if total == 0 {
			continue
		}
		rate := float64(h.SuccessCount) / float64(total)
		if rate < minSuccessRate {
			delete(e.heuristics, id)
			pruned++
		}
	}
	return pruned
}

// IsComplex determines if an input requires deliberate reasoning.
func (e *DualProcessEngine) IsComplex(input string) bool {
	if len(input) > 200 {
		return true
	}
	lower := strings.ToLower(input)
	questionCount := strings.Count(lower, "?")
	if questionCount > 1 {
		return true
	}
	conditionals := []string{"if ", "else ", "unless ", "otherwise ", "however ", "but "}
	for _, c := range conditionals {
		if strings.Contains(lower, c) {
			return true
		}
	}
	negations := []string{"not ", "don't ", "doesn't ", "never ", "no ", "cannot ", "can't "}
	for _, n := range negations {
		if strings.Contains(lower, n) {
			return true
		}
	}
	return false
}
