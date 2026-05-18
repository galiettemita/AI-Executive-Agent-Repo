package cognitive

import (
	"fmt"
	"strings"
	"sync"
	"time"
)

// UserKnowledgeModel represents the system's model of a user's knowledge.
type UserKnowledgeModel struct {
	WorkspaceID              string
	UserID                   string
	KnownTopics              map[string]float64 // topic -> familiarity 0-1
	PreferredExplanationDepth string             // brief, standard, detailed
	TechnicalLevel           string             // beginner, intermediate, expert
	LastUpdated              time.Time
}

// TheoryOfMindService tracks and infers user knowledge to adapt responses.
type TheoryOfMindService struct {
	mu     sync.RWMutex
	models map[string]*UserKnowledgeModel // key: workspaceID:userID
}

// NewTheoryOfMindService creates a new TheoryOfMindService.
func NewTheoryOfMindService() *TheoryOfMindService {
	return &TheoryOfMindService{
		models: make(map[string]*UserKnowledgeModel),
	}
}

func modelKey(workspaceID, userID string) string {
	return workspaceID + ":" + userID
}

func (t *TheoryOfMindService) getOrCreate(workspaceID, userID string) *UserKnowledgeModel {
	key := modelKey(workspaceID, userID)
	m, ok := t.models[key]
	if !ok {
		m = &UserKnowledgeModel{
			WorkspaceID:              workspaceID,
			UserID:                   userID,
			KnownTopics:              make(map[string]float64),
			PreferredExplanationDepth: "standard",
			TechnicalLevel:           "intermediate",
			LastUpdated:              time.Now(),
		}
		t.models[key] = m
	}
	return m
}

// UpdateModel adjusts a user's topic familiarity based on demonstrated behavior.
func (t *TheoryOfMindService) UpdateModel(workspaceID, userID, topic string, demonstrated bool) {
	t.mu.Lock()
	defer t.mu.Unlock()

	m := t.getOrCreate(workspaceID, userID)
	current := m.KnownTopics[topic]

	if demonstrated {
		// Increase familiarity.
		current += (1.0 - current) * 0.2
	} else {
		// Decrease familiarity.
		current *= 0.8
	}

	m.KnownTopics[topic] = current
	m.LastUpdated = time.Now()

	// Infer technical level from average familiarity.
	t.inferTechnicalLevel(m)
}

func (t *TheoryOfMindService) inferTechnicalLevel(m *UserKnowledgeModel) {
	if len(m.KnownTopics) == 0 {
		return
	}
	total := 0.0
	for _, v := range m.KnownTopics {
		total += v
	}
	avg := total / float64(len(m.KnownTopics))

	switch {
	case avg > 0.7:
		m.TechnicalLevel = "expert"
		m.PreferredExplanationDepth = "brief"
	case avg > 0.4:
		m.TechnicalLevel = "intermediate"
		m.PreferredExplanationDepth = "standard"
	default:
		m.TechnicalLevel = "beginner"
		m.PreferredExplanationDepth = "detailed"
	}
}

// InferKnowledge returns the estimated familiarity of a user with a topic.
func (t *TheoryOfMindService) InferKnowledge(workspaceID, userID, topic string) float64 {
	t.mu.RLock()
	defer t.mu.RUnlock()

	key := modelKey(workspaceID, userID)
	m, ok := t.models[key]
	if !ok {
		return 0.0
	}

	if fam, exists := m.KnownTopics[topic]; exists {
		return fam
	}

	// Infer from related topics (prefix match).
	topicLower := strings.ToLower(topic)
	var related []float64
	for k, v := range m.KnownTopics {
		kLower := strings.ToLower(k)
		if strings.HasPrefix(topicLower, kLower) || strings.HasPrefix(kLower, topicLower) {
			related = append(related, v)
		}
	}
	if len(related) > 0 {
		sum := 0.0
		for _, v := range related {
			sum += v
		}
		return sum / float64(len(related)) * 0.5 // discounted inference
	}

	return 0.0
}

// ShouldExplain returns true if the user's familiarity with a concept is below 0.5.
func (t *TheoryOfMindService) ShouldExplain(workspaceID, userID, concept string) bool {
	return t.InferKnowledge(workspaceID, userID, concept) < 0.5
}

// AdaptExplanation adjusts the detail level of content based on the user's model.
func (t *TheoryOfMindService) AdaptExplanation(workspaceID, userID, content string) string {
	t.mu.RLock()
	key := modelKey(workspaceID, userID)
	m, ok := t.models[key]
	t.mu.RUnlock()

	if !ok {
		return content
	}

	switch m.PreferredExplanationDepth {
	case "brief":
		// Truncate to first sentence or 200 chars.
		if idx := strings.Index(content, ". "); idx > 0 && idx < 200 {
			return content[:idx+1]
		}
		if len(content) > 200 {
			return content[:200] + "..."
		}
		return content
	case "detailed":
		return fmt.Sprintf("%s\n\n[Additional context: This is explained in detail because your profile indicates you may benefit from more thorough explanations on this topic.]", content)
	default:
		return content
	}
}

// DetectKnowledgeGap returns topics where the user's familiarity is below 0.5.
func (t *TheoryOfMindService) DetectKnowledgeGap(workspaceID, userID string, topics []string) []string {
	var gaps []string
	for _, topic := range topics {
		if t.InferKnowledge(workspaceID, userID, topic) < 0.5 {
			gaps = append(gaps, topic)
		}
	}
	return gaps
}
