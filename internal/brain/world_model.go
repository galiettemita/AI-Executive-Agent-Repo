package brain

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
)

// WorldFact represents a fact about the world learned through interaction.
type WorldFact struct {
	ID          uuid.UUID
	WorkspaceID string
	Subject     string
	Predicate   string
	Value       string
	Source      string
	Confidence  float64
	LearnedAt   time.Time
	ExpiresAt   time.Time
	CreatedAt   time.Time
}

// WorldModelService maintains a cache of world knowledge facts.
// When a WorldModelRepository is provided, facts are persisted to PostgreSQL.
// When repo is nil, facts are stored in-memory only (test/degraded mode).
type WorldModelService struct {
	repo WorldModelRepository

	// In-memory fallback for test/degraded mode (repo == nil).
	mu    sync.Mutex
	facts map[string][]WorldFact
}

// NewWorldModelService creates a new WorldModelService.
// If a repository is provided, facts are persisted to PostgreSQL.
// Otherwise, falls back to in-memory storage.
func NewWorldModelService(repo ...WorldModelRepository) *WorldModelService {
	var r WorldModelRepository
	if len(repo) > 0 {
		r = repo[0]
	}
	return &WorldModelService{
		repo:  r,
		facts: map[string][]WorldFact{},
	}
}

// AddFact stores a new world fact.
func (wm *WorldModelService) AddFact(workspaceID, subject, predicate, value, source string) (WorldFact, error) {
	if strings.TrimSpace(workspaceID) == "" {
		return WorldFact{}, fmt.Errorf("workspace_id is required")
	}
	if strings.TrimSpace(subject) == "" {
		return WorldFact{}, fmt.Errorf("subject is required")
	}

	subject = strings.TrimSpace(subject)
	predicate = strings.TrimSpace(predicate)
	value = strings.TrimSpace(value)
	expiresAt := time.Now().UTC().Add(24 * time.Hour)

	if wm.repo != nil {
		wsID, err := uuid.Parse(workspaceID)
		if err != nil {
			wsID = uuid.NewSHA1(uuid.NameSpaceURL, []byte(workspaceID))
		}
		return wm.repo.AddFact(context.Background(), wsID, subject, predicate, value, source, 1.0, expiresAt)
	}

	// In-memory fallback.
	fact := WorldFact{
		ID:          uuid.Must(uuid.NewV7()),
		WorkspaceID: workspaceID,
		Subject:     subject,
		Predicate:   predicate,
		Value:       value,
		Source:      source,
		Confidence:  1.0,
		LearnedAt:   time.Now().UTC(),
		ExpiresAt:   expiresAt,
		CreatedAt:   time.Now().UTC(),
	}
	wm.mu.Lock()
	defer wm.mu.Unlock()
	wm.facts[workspaceID] = append(wm.facts[workspaceID], fact)
	return fact, nil
}

// UpdateFromFailure learns from tool execution failures and records facts.
func (wm *WorldModelService) UpdateFromFailure(workspaceID, toolName, errorMsg string) (WorldFact, error) {
	if strings.TrimSpace(workspaceID) == "" {
		return WorldFact{}, fmt.Errorf("workspace_id is required")
	}
	if strings.TrimSpace(toolName) == "" {
		return WorldFact{}, fmt.Errorf("tool_name is required")
	}

	predicate := "last_failure"
	value := strings.TrimSpace(errorMsg)
	if value == "" {
		value = "unknown error"
	}

	return wm.AddFact(workspaceID, toolName, predicate, value, "failure_observation")
}

// GetFacts returns all facts for a workspace and subject.
func (wm *WorldModelService) GetFacts(workspaceID, subject string) []WorldFact {
	if wm.repo != nil {
		wsID, err := uuid.Parse(workspaceID)
		if err != nil {
			wsID = uuid.NewSHA1(uuid.NameSpaceURL, []byte(workspaceID))
		}
		facts, err := wm.repo.GetFacts(context.Background(), wsID, subject)
		if err != nil {
			return nil
		}
		return facts
	}

	// In-memory fallback.
	wm.mu.Lock()
	defer wm.mu.Unlock()
	now := time.Now().UTC()
	var result []WorldFact
	for _, fact := range wm.facts[workspaceID] {
		if !now.Before(fact.ExpiresAt) {
			continue
		}
		if strings.EqualFold(fact.Subject, subject) {
			result = append(result, fact)
		}
	}
	return result
}

// StoreFact stores a fact learned from a tool execution output.
func (wm *WorldModelService) StoreFact(workspaceID, toolKey, content string) error {
	if strings.TrimSpace(workspaceID) == "" || strings.TrimSpace(content) == "" {
		return nil
	}
	_, err := wm.AddFact(workspaceID, toolKey, "tool_output", content, toolKey)
	return err
}

// QueryFacts retrieves all facts for a workspace, sorted by LearnedAt DESC.
// Returns empty slice (never nil) if no facts exist.
func (wm *WorldModelService) QueryFacts(workspaceID string) []WorldFact {
	if wm.repo != nil {
		wsID, err := uuid.Parse(workspaceID)
		if err != nil {
			wsID = uuid.NewSHA1(uuid.NameSpaceURL, []byte(workspaceID))
		}
		facts, err := wm.repo.GetAllFacts(context.Background(), wsID)
		if err != nil {
			return []WorldFact{}
		}
		return facts
	}

	// In-memory fallback.
	wm.mu.Lock()
	defer wm.mu.Unlock()
	facts, ok := wm.facts[workspaceID]
	if !ok || len(facts) == 0 {
		return []WorldFact{}
	}
	sorted := make([]WorldFact, len(facts))
	copy(sorted, facts)
	sort.Slice(sorted, func(i, j int) bool {
		return sorted[i].LearnedAt.After(sorted[j].LearnedAt)
	})
	return sorted
}

// SerializeForContext converts facts to a compact string for injection into LLM prompts.
// Format: "[source] value" — one fact per line.
// Truncated to maxFacts most-recent entries (default 20).
func (wm *WorldModelService) SerializeForContext(facts []WorldFact, maxFacts int) string {
	if maxFacts <= 0 {
		maxFacts = 20
	}
	if len(facts) > maxFacts {
		facts = facts[:maxFacts]
	}
	var sb strings.Builder
	for _, f := range facts {
		sb.WriteString(fmt.Sprintf("[%s] %s\n", f.Source, f.Value))
	}
	return strings.TrimSpace(sb.String())
}

// CheckFact checks if a specific fact exists for a subject and predicate.
func (wm *WorldModelService) CheckFact(workspaceID, subject, predicate string) (WorldFact, bool) {
	if wm.repo != nil {
		facts := wm.GetFacts(workspaceID, subject)
		for _, fact := range facts {
			if strings.EqualFold(fact.Predicate, predicate) {
				return fact, true
			}
		}
		return WorldFact{}, false
	}

	// In-memory fallback.
	wm.mu.Lock()
	defer wm.mu.Unlock()
	now := time.Now().UTC()
	for _, fact := range wm.facts[workspaceID] {
		if !now.Before(fact.ExpiresAt) {
			continue
		}
		if strings.EqualFold(fact.Subject, subject) && strings.EqualFold(fact.Predicate, predicate) {
			return fact, true
		}
	}
	return WorldFact{}, false
}
