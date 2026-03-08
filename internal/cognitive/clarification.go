package cognitive

import (
	"sort"
	"strings"
	"sync"
)

// ClarificationCandidate represents a potential clarifying question.
type ClarificationCandidate struct {
	Question        string
	InformationGain float64
	Urgency         float64
	Category        string
}

// ClarificationService generates and ranks clarifying questions for ambiguous inputs.
type ClarificationService struct {
	mu        sync.RWMutex
	templates map[string][]string // category -> question templates
}

// NewClarificationService creates a new ClarificationService with default templates.
func NewClarificationService() *ClarificationService {
	cs := &ClarificationService{
		templates: map[string][]string{
			"intent": {
				"Could you clarify what you mean by '%s'?",
				"Are you asking about '%s' in a specific context?",
			},
			"scope": {
				"Should this apply to all items, or just a subset?",
				"What is the scope of this request?",
			},
			"constraint": {
				"Are there any constraints or requirements I should be aware of?",
				"Is there a deadline or priority level for this?",
			},
			"preference": {
				"Do you have a preferred approach for this?",
				"Would you like a brief or detailed response?",
			},
		},
	}
	return cs
}

// GenerateClarifications produces clarification candidates for ambiguous input.
func (cs *ClarificationService) GenerateClarifications(workspaceID string, ambiguousInput string, context map[string]any) []ClarificationCandidate {
	cs.mu.RLock()
	defer cs.mu.RUnlock()

	var candidates []ClarificationCandidate

	words := strings.Fields(ambiguousInput)
	wordCount := len(words)

	// Short inputs are more ambiguous.
	ambiguityFromLength := 0.0
	if wordCount < 3 {
		ambiguityFromLength = 0.8
	} else if wordCount < 7 {
		ambiguityFromLength = 0.5
	} else {
		ambiguityFromLength = 0.2
	}

	// Check for question marks (user might already be asking).
	hasQuestion := strings.Contains(ambiguousInput, "?")

	// Generate intent clarifications.
	if ambiguityFromLength > 0.4 {
		for _, tmpl := range cs.templates["intent"] {
			q := tmpl
			if strings.Contains(tmpl, "%s") {
				q = strings.Replace(tmpl, "%s", ambiguousInput, 1)
			}
			candidates = append(candidates, ClarificationCandidate{
				Question:        q,
				InformationGain: ambiguityFromLength,
				Urgency:         0.7,
				Category:        "intent",
			})
		}
	}

	// Scope clarifications if context is sparse.
	contextSize := len(context)
	if contextSize < 2 {
		for _, tmpl := range cs.templates["scope"] {
			candidates = append(candidates, ClarificationCandidate{
				Question:        tmpl,
				InformationGain: 0.6,
				Urgency:         0.5,
				Category:        "scope",
			})
		}
	}

	// Constraint clarifications.
	if _, hasDeadline := context["deadline"]; !hasDeadline {
		for _, tmpl := range cs.templates["constraint"] {
			candidates = append(candidates, ClarificationCandidate{
				Question:        tmpl,
				InformationGain: 0.4,
				Urgency:         0.6,
				Category:        "constraint",
			})
		}
	}

	// Preference clarifications if not a question.
	if !hasQuestion {
		for _, tmpl := range cs.templates["preference"] {
			candidates = append(candidates, ClarificationCandidate{
				Question:        tmpl,
				InformationGain: 0.3,
				Urgency:         0.3,
				Category:        "preference",
			})
		}
	}

	return candidates
}

// RankClarifications sorts candidates by information gain * urgency (descending).
func (cs *ClarificationService) RankClarifications(candidates []ClarificationCandidate) []ClarificationCandidate {
	sorted := make([]ClarificationCandidate, len(candidates))
	copy(sorted, candidates)

	sort.Slice(sorted, func(i, j int) bool {
		scoreI := sorted[i].InformationGain * sorted[i].Urgency
		scoreJ := sorted[j].InformationGain * sorted[j].Urgency
		return scoreI > scoreJ
	})

	return sorted
}

// ShouldAskClarification returns true if the ambiguity exceeds the threshold.
func (cs *ClarificationService) ShouldAskClarification(ambiguityScore float64, threshold float64) bool {
	return ambiguityScore > threshold
}
