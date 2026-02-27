package memory

import (
	"fmt"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
)

type Item struct {
	ID               uuid.UUID
	WorkspaceID      string
	MemoryType       string
	Status           string
	Body             string
	DataClass        string
	SensitivityLabel string
	ContentTrust     string
	CreatedAt        time.Time
	UpdatedAt        time.Time
}

type Service struct {
	mu             sync.Mutex
	exclusionRules map[string][]string
	items          []Item
}

func NewService() *Service {
	return &Service{exclusionRules: map[string][]string{}, items: []Item{}}
}

func exclusionKey(workspaceID, userID string) string {
	return workspaceID + "::" + userID
}

func (s *Service) AddExclusionRule(workspaceID, userID, rule string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.exclusionRules[exclusionKey(workspaceID, userID)] = append(s.exclusionRules[exclusionKey(workspaceID, userID)], strings.ToLower(rule))
}

func (s *Service) Write(workspaceID, userID, memoryType, body string) (Item, error) {
	if workspaceID == "" || userID == "" {
		return Item{}, fmt.Errorf("workspace_id and user_id required")
	}
	if body == "" {
		return Item{}, fmt.Errorf("body is required")
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	for _, rule := range s.exclusionRules[exclusionKey(workspaceID, userID)] {
		if strings.Contains(strings.ToLower(body), rule) {
			return Item{}, fmt.Errorf("memory write blocked by exclusion rule")
		}
	}

	item := Item{
		ID:               uuid.Must(uuid.NewV7()),
		WorkspaceID:      workspaceID,
		MemoryType:       memoryType,
		Status:           "proposed",
		Body:             body,
		DataClass:        "internal",
		SensitivityLabel: "moderate",
		ContentTrust:     "mixed",
		CreatedAt:        time.Now().UTC(),
		UpdatedAt:        time.Now().UTC(),
	}
	s.items = append(s.items, item)
	return item, nil
}

func (s *Service) Retrieve(workspaceID string) []Item {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := []Item{}
	for _, item := range s.items {
		if item.WorkspaceID == workspaceID {
			out = append(out, item)
		}
	}
	sort.Slice(out, func(i, j int) bool {
		return out[i].CreatedAt.Before(out[j].CreatedAt)
	})
	return out
}

func (s *Service) Consolidate(workspaceID string) []Item {
	s.mu.Lock()
	defer s.mu.Unlock()
	seen := map[string]struct{}{}
	consolidated := []Item{}
	for _, item := range s.items {
		if item.WorkspaceID != workspaceID {
			continue
		}
		normalized := strings.TrimSpace(strings.ToLower(item.Body))
		if _, exists := seen[normalized]; exists {
			continue
		}
		seen[normalized] = struct{}{}
		consolidated = append(consolidated, item)
	}
	return consolidated
}
