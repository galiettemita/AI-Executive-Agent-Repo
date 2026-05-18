package cognition

import (
	"fmt"
	"strings"
	"sync"
	"time"
)

// UserKnowledgeModel represents a model of a user's knowledge and preferences.
type UserKnowledgeModel struct {
	WorkspaceID        string             `json:"workspace_id"`
	UserID             string             `json:"user_id"`
	KnownTopics        map[string]float64 `json:"known_topics"`
	BeliefGaps         []string           `json:"belief_gaps"`
	ExpertiseDomains   []string           `json:"expertise_domains"`
	PreferredComplexity string            `json:"preferred_complexity"` // simple, moderate, expert
	LastUpdatedAt      time.Time          `json:"last_updated_at"`
}

// TheoryOfMindService manages user knowledge models.
type TheoryOfMindService struct {
	mu     sync.Mutex
	models map[string]*UserKnowledgeModel // key: workspaceID::userID
}

// NewTheoryOfMindService creates a new TheoryOfMindService.
func NewTheoryOfMindService() *TheoryOfMindService {
	return &TheoryOfMindService{
		models: make(map[string]*UserKnowledgeModel),
	}
}

func tomKey(workspaceID, userID string) string {
	return workspaceID + "::" + userID
}

// UpdateModel updates a user's knowledge model for a specific topic.
func (s *TheoryOfMindService) UpdateModel(workspaceID, userID string, topic string, proficiency float64) error {
	if strings.TrimSpace(workspaceID) == "" || strings.TrimSpace(userID) == "" {
		return fmt.Errorf("workspace_id and user_id are required")
	}
	if strings.TrimSpace(topic) == "" {
		return fmt.Errorf("topic is required")
	}
	if proficiency < 0 || proficiency > 1 {
		return fmt.Errorf("proficiency must be between 0 and 1")
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	key := tomKey(workspaceID, userID)
	model, ok := s.models[key]
	if !ok {
		model = &UserKnowledgeModel{
			WorkspaceID:        workspaceID,
			UserID:             userID,
			KnownTopics:        make(map[string]float64),
			BeliefGaps:         []string{},
			ExpertiseDomains:   []string{},
			PreferredComplexity: "moderate",
			LastUpdatedAt:      time.Now().UTC(),
		}
		s.models[key] = model
	}

	model.KnownTopics[topic] = proficiency
	model.LastUpdatedAt = time.Now().UTC()

	// Update expertise domains if proficiency is high
	if proficiency >= 0.8 {
		found := false
		for _, d := range model.ExpertiseDomains {
			if d == topic {
				found = true
				break
			}
		}
		if !found {
			model.ExpertiseDomains = append(model.ExpertiseDomains, topic)
		}
	}

	// Update preferred complexity based on average proficiency
	model.PreferredComplexity = computePreferredComplexity(model.KnownTopics)

	return nil
}

func computePreferredComplexity(topics map[string]float64) string {
	if len(topics) == 0 {
		return "moderate"
	}
	var total float64
	for _, p := range topics {
		total += p
	}
	avg := total / float64(len(topics))
	if avg >= 0.7 {
		return "expert"
	}
	if avg >= 0.4 {
		return "moderate"
	}
	return "simple"
}

// GetModel returns the knowledge model for a user.
func (s *TheoryOfMindService) GetModel(workspaceID, userID string) (*UserKnowledgeModel, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	key := tomKey(workspaceID, userID)
	model, ok := s.models[key]
	if !ok {
		return nil, fmt.Errorf("no model found for user %s in workspace %s", userID, workspaceID)
	}
	return model, nil
}

// InferKnowledgeGap returns the estimated gap (0=knows, 1=doesn't know) for a topic.
func (s *TheoryOfMindService) InferKnowledgeGap(model *UserKnowledgeModel, topic string) float64 {
	if model == nil {
		return 1.0
	}

	// Direct topic match
	if proficiency, ok := model.KnownTopics[topic]; ok {
		return 1.0 - proficiency
	}

	// Check if any known topic partially overlaps
	topicLower := strings.ToLower(topic)
	bestMatch := 0.0
	for knownTopic, proficiency := range model.KnownTopics {
		if strings.Contains(topicLower, strings.ToLower(knownTopic)) ||
			strings.Contains(strings.ToLower(knownTopic), topicLower) {
			if proficiency > bestMatch {
				bestMatch = proficiency
			}
		}
	}

	if bestMatch > 0 {
		return 1.0 - bestMatch*0.5 // partial transfer
	}

	return 1.0
}

// AdaptExplanation simplifies or enriches an explanation based on the user's level.
func (s *TheoryOfMindService) AdaptExplanation(model *UserKnowledgeModel, explanation string) string {
	if model == nil {
		return explanation
	}

	switch model.PreferredComplexity {
	case "simple":
		// Simplify: truncate long explanations and add context markers
		if len(explanation) > 200 {
			explanation = explanation[:200] + "..."
		}
		return "In simple terms: " + explanation
	case "expert":
		return "Technical detail: " + explanation
	default:
		return explanation
	}
}

// PredictUnderstanding predicts how well a user will understand a concept (0-1).
func (s *TheoryOfMindService) PredictUnderstanding(model *UserKnowledgeModel, concept string) float64 {
	if model == nil {
		return 0.5
	}

	// Direct knowledge
	if proficiency, ok := model.KnownTopics[concept]; ok {
		return proficiency
	}

	// Related domain knowledge
	conceptLower := strings.ToLower(concept)
	var relatedScores []float64
	for topic, prof := range model.KnownTopics {
		words := strings.Fields(strings.ToLower(topic))
		for _, w := range words {
			if strings.Contains(conceptLower, w) {
				relatedScores = append(relatedScores, prof)
				break
			}
		}
	}

	if len(relatedScores) > 0 {
		var total float64
		for _, s := range relatedScores {
			total += s
		}
		return total / float64(len(relatedScores)) * 0.7 // discount for indirect knowledge
	}

	// Baseline from preferred complexity
	switch model.PreferredComplexity {
	case "expert":
		return 0.4
	case "moderate":
		return 0.3
	default:
		return 0.2
	}
}
