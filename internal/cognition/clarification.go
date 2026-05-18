package cognition

import (
	"fmt"
	"sort"
	"strings"
	"sync"

	"github.com/google/uuid"
)

// ClarificationCandidate represents a potential clarification question.
type ClarificationCandidate struct {
	ID              string  `json:"id"`
	Question        string  `json:"question"`
	InformationGain float64 `json:"information_gain"`
	Urgency         float64 `json:"urgency"`
	Category        string  `json:"category"`
}

// ClarificationService manages optimal clarification policies.
type ClarificationService struct {
	mu         sync.Mutex
	candidates []ClarificationCandidate
}

// NewClarificationService creates a new ClarificationService.
func NewClarificationService() *ClarificationService {
	return &ClarificationService{
		candidates: []ClarificationCandidate{},
	}
}

// GenerateCandidates generates ranked clarification questions from context and ambiguities.
func (s *ClarificationService) GenerateCandidates(context string, ambiguities []string) []ClarificationCandidate {
	s.mu.Lock()
	defer s.mu.Unlock()

	var candidates []ClarificationCandidate

	for i, ambiguity := range ambiguities {
		if strings.TrimSpace(ambiguity) == "" {
			continue
		}

		// Information gain heuristic: longer ambiguities suggest more complex issues
		infoGain := 0.5 + float64(len(ambiguity))/500.0
		if infoGain > 1.0 {
			infoGain = 1.0
		}

		// Urgency decreases with position (first ambiguities are more urgent)
		urgency := 1.0 - float64(i)*0.1
		if urgency < 0.1 {
			urgency = 0.1
		}

		category := categorizeAmbiguity(ambiguity)

		candidate := ClarificationCandidate{
			ID:              uuid.Must(uuid.NewV7()).String(),
			Question:        fmt.Sprintf("Could you clarify what you mean by '%s'?", ambiguity),
			InformationGain: infoGain,
			Urgency:         urgency,
			Category:        category,
		}
		candidates = append(candidates, candidate)
	}

	// Include a context-level question if the context itself is ambiguous
	if strings.TrimSpace(context) != "" && len(ambiguities) > 2 {
		candidates = append(candidates, ClarificationCandidate{
			ID:              uuid.Must(uuid.NewV7()).String(),
			Question:        fmt.Sprintf("There seem to be multiple unclear aspects of '%s'. Could you provide more context?", truncate(context, 100)),
			InformationGain: 0.9,
			Urgency:         0.8,
			Category:        "context",
		})
	}

	sort.Slice(candidates, func(i, j int) bool {
		scoreI := candidates[i].InformationGain * candidates[i].Urgency
		scoreJ := candidates[j].InformationGain * candidates[j].Urgency
		return scoreI > scoreJ
	})

	s.candidates = candidates
	return candidates
}

func categorizeAmbiguity(ambiguity string) string {
	lower := strings.ToLower(ambiguity)
	if strings.Contains(lower, "when") || strings.Contains(lower, "time") || strings.Contains(lower, "date") {
		return "temporal"
	}
	if strings.Contains(lower, "who") || strings.Contains(lower, "person") || strings.Contains(lower, "user") {
		return "identity"
	}
	if strings.Contains(lower, "how") || strings.Contains(lower, "method") || strings.Contains(lower, "way") {
		return "method"
	}
	if strings.Contains(lower, "what") || strings.Contains(lower, "which") {
		return "specification"
	}
	return "general"
}

func truncate(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "..."
}

// SelectOptimal picks the candidate with the highest information gain * urgency score.
func (s *ClarificationService) SelectOptimal(candidates []ClarificationCandidate) *ClarificationCandidate {
	if len(candidates) == 0 {
		return nil
	}

	best := &candidates[0]
	bestScore := best.InformationGain * best.Urgency

	for i := 1; i < len(candidates); i++ {
		score := candidates[i].InformationGain * candidates[i].Urgency
		if score > bestScore {
			bestScore = score
			best = &candidates[i]
		}
	}
	return best
}

// ShouldClarify determines whether clarification is needed.
func (s *ClarificationService) ShouldClarify(confidence float64, ambiguityCount int) bool {
	return confidence < 0.6 || ambiguityCount > 2
}

// FormatClarification formats a candidate as a user-friendly question.
func (s *ClarificationService) FormatClarification(candidate *ClarificationCandidate) string {
	if candidate == nil {
		return ""
	}
	return fmt.Sprintf("[%s] %s (confidence gain: %.0f%%)",
		strings.ToUpper(candidate.Category),
		candidate.Question,
		candidate.InformationGain*100)
}
