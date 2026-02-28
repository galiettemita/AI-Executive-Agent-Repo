package memory

import (
	"fmt"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
)

const (
	StatusProposed          = "proposed"
	StatusNeedsConfirmation = "needs_confirmation"
	StatusActive            = "active"
	StatusSuperseded        = "superseded"
	StatusDeleted           = "deleted"
)

var validMemoryTypes = map[string]struct{}{
	"semantic":     {},
	"episodic":     {},
	"preference":   {},
	"rule":         {},
	"contact_fact": {},
	"task_fact":    {},
	"daily_log":    {},
	"heartbeat":    {},
}

var validDataClasses = map[string]struct{}{
	"public":       {},
	"internal":     {},
	"confidential": {},
	"restricted":   {},
}

var validSensitivityLabels = map[string]struct{}{
	"none":      {},
	"low":       {},
	"moderate":  {},
	"high":      {},
	"regulated": {},
}

var validContentTrust = map[string]struct{}{
	"trusted":   {},
	"untrusted": {},
	"mixed":     {},
}

type Item struct {
	ID                uuid.UUID
	WorkspaceID       string
	UserID            string
	MemoryType        string
	Status            string
	Body              string
	DataClass         string
	SensitivityLabel  string
	RetentionPolicyID string
	AllowedProcessors []string
	ContentTrust      string
	EmbeddingVersion  int
	ExpiresAt         *time.Time
	CreatedAt         time.Time
	UpdatedAt         time.Time
}

type WriteRequest struct {
	WorkspaceID       string
	UserID            string
	MemoryType        string
	Body              string
	DataClass         string
	SensitivityLabel  string
	RetentionPolicyID string
	AllowedProcessors []string
	ContentTrust      string
	ExpiresAt         *time.Time
}

type WritePolicy struct {
	RequireConfirmationForTypes map[string]struct{}
	BlockedDataClasses          map[string]struct{}
}

type Service struct {
	mu             sync.Mutex
	exclusionRules map[string][]string
	writePolicies  map[string]WritePolicy
	items          map[uuid.UUID]Item
	itemOrder      []uuid.UUID
}

func NewService() *Service {
	return &Service{
		exclusionRules: map[string][]string{},
		writePolicies:  map[string]WritePolicy{},
		items:          map[uuid.UUID]Item{},
		itemOrder:      []uuid.UUID{},
	}
}

func exclusionKey(workspaceID, userID string) string {
	return workspaceID + "::" + userID
}

func (s *Service) SetWritePolicy(workspaceID string, policy WritePolicy) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if policy.RequireConfirmationForTypes == nil {
		policy.RequireConfirmationForTypes = map[string]struct{}{}
	}
	if policy.BlockedDataClasses == nil {
		policy.BlockedDataClasses = map[string]struct{}{}
	}
	s.writePolicies[workspaceID] = policy
}

func (s *Service) AddExclusionRule(workspaceID, userID, rule string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.exclusionRules[exclusionKey(workspaceID, userID)] = append(s.exclusionRules[exclusionKey(workspaceID, userID)], strings.ToLower(rule))
}

func (s *Service) Write(workspaceID, userID, memoryType, body string) (Item, error) {
	return s.WriteWithRequest(WriteRequest{
		WorkspaceID:       workspaceID,
		UserID:            userID,
		MemoryType:        memoryType,
		Body:              body,
		DataClass:         "internal",
		SensitivityLabel:  "moderate",
		RetentionPolicyID: "default",
		AllowedProcessors: []string{"brain", "control", "executor"},
		ContentTrust:      "mixed",
	})
}

func (s *Service) WriteWithRequest(req WriteRequest) (Item, error) {
	if req.WorkspaceID == "" || req.UserID == "" {
		return Item{}, fmt.Errorf("workspace_id and user_id required")
	}
	if strings.TrimSpace(req.Body) == "" {
		return Item{}, fmt.Errorf("body is required")
	}
	if _, ok := validMemoryTypes[req.MemoryType]; !ok {
		return Item{}, fmt.Errorf("invalid memory_type: %s", req.MemoryType)
	}
	if _, ok := validDataClasses[req.DataClass]; !ok {
		return Item{}, fmt.Errorf("invalid data_class: %s", req.DataClass)
	}
	if _, ok := validSensitivityLabels[req.SensitivityLabel]; !ok {
		return Item{}, fmt.Errorf("invalid sensitivity_label: %s", req.SensitivityLabel)
	}
	if _, ok := validContentTrust[req.ContentTrust]; !ok {
		return Item{}, fmt.Errorf("invalid content_trust: %s", req.ContentTrust)
	}
	if len(req.AllowedProcessors) == 0 {
		return Item{}, fmt.Errorf("allowed_processors must be non-empty")
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	for _, rule := range s.exclusionRules[exclusionKey(req.WorkspaceID, req.UserID)] {
		if strings.Contains(strings.ToLower(req.Body), rule) {
			return Item{}, fmt.Errorf("memory write blocked by exclusion rule")
		}
	}

	policy := s.writePolicies[req.WorkspaceID]
	if _, blocked := policy.BlockedDataClasses[req.DataClass]; blocked {
		return Item{}, fmt.Errorf("memory write blocked by data_class policy")
	}

	status := StatusProposed
	if _, requires := policy.RequireConfirmationForTypes[req.MemoryType]; requires {
		status = StatusNeedsConfirmation
	}

	item := Item{
		ID:                uuid.Must(uuid.NewV7()),
		WorkspaceID:       req.WorkspaceID,
		UserID:            req.UserID,
		MemoryType:        req.MemoryType,
		Status:            status,
		Body:              strings.TrimSpace(req.Body),
		DataClass:         req.DataClass,
		SensitivityLabel:  req.SensitivityLabel,
		RetentionPolicyID: req.RetentionPolicyID,
		AllowedProcessors: append([]string(nil), req.AllowedProcessors...),
		ContentTrust:      req.ContentTrust,
		EmbeddingVersion:  1,
		ExpiresAt:         req.ExpiresAt,
		CreatedAt:         time.Now().UTC(),
		UpdatedAt:         time.Now().UTC(),
	}
	s.items[item.ID] = item
	s.itemOrder = append(s.itemOrder, item.ID)
	return item, nil
}

func (s *Service) TransitionStatus(itemID uuid.UUID, nextStatus string) (Item, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	item, ok := s.items[itemID]
	if !ok {
		return Item{}, fmt.Errorf("memory item not found")
	}
	if !isValidStatusTransition(item.Status, nextStatus) {
		return Item{}, fmt.Errorf("invalid status transition: %s -> %s", item.Status, nextStatus)
	}
	item.Status = nextStatus
	item.UpdatedAt = time.Now().UTC()
	s.items[itemID] = item
	return item, nil
}

func isValidStatusTransition(current, next string) bool {
	allowed := map[string]map[string]struct{}{
		StatusProposed: {
			StatusNeedsConfirmation: {},
			StatusActive:            {},
			StatusDeleted:           {},
		},
		StatusNeedsConfirmation: {
			StatusActive:  {},
			StatusDeleted: {},
		},
		StatusActive: {
			StatusSuperseded: {},
			StatusDeleted:    {},
		},
		StatusSuperseded: {
			StatusDeleted: {},
		},
	}
	if _, ok := allowed[current]; !ok {
		return false
	}
	_, ok := allowed[current][next]
	return ok
}

func (s *Service) GetItem(itemID uuid.UUID) (Item, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	item, ok := s.items[itemID]
	return item, ok
}

func (s *Service) Retrieve(workspaceID string) []Item {
	return s.retrieveFiltered(workspaceID, nil)
}

func (s *Service) RetrieveWithTrust(workspaceID string, allowedTrust []string) []Item {
	trustFilter := map[string]struct{}{}
	for _, trust := range allowedTrust {
		trustFilter[trust] = struct{}{}
	}
	return s.retrieveFiltered(workspaceID, trustFilter)
}

func (s *Service) retrieveFiltered(workspaceID string, trustFilter map[string]struct{}) []Item {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := []Item{}
	for _, itemID := range s.itemOrder {
		item := s.items[itemID]
		if item.WorkspaceID != workspaceID {
			continue
		}
		if trustFilter != nil {
			if _, ok := trustFilter[item.ContentTrust]; !ok {
				continue
			}
		}
		out = append(out, item)
	}
	sort.Slice(out, func(i, j int) bool {
		return out[i].CreatedAt.Before(out[j].CreatedAt)
	})
	return out
}

func (s *Service) Consolidate(workspaceID string) []Item {
	s.mu.Lock()
	defer s.mu.Unlock()

	now := time.Now().UTC()
	seen := map[string]uuid.UUID{}
	consolidated := []Item{}

	for _, itemID := range s.itemOrder {
		item := s.items[itemID]
		if item.WorkspaceID != workspaceID {
			continue
		}
		if item.ExpiresAt != nil && now.After(*item.ExpiresAt) {
			item.Status = StatusDeleted
			item.UpdatedAt = now
			s.items[itemID] = item
			continue
		}
		if item.Status == StatusDeleted {
			continue
		}

		normalized := strings.TrimSpace(strings.ToLower(item.MemoryType + "::" + item.Body))
		if canonicalID, exists := seen[normalized]; exists {
			item.Status = StatusSuperseded
			item.UpdatedAt = now
			s.items[itemID] = item

			canonical := s.items[canonicalID]
			canonical.EmbeddingVersion++
			canonical.UpdatedAt = now
			s.items[canonicalID] = canonical
			continue
		}
		seen[normalized] = itemID
		consolidated = append(consolidated, item)
	}

	sort.Slice(consolidated, func(i, j int) bool {
		return consolidated[i].CreatedAt.Before(consolidated[j].CreatedAt)
	})
	return consolidated
}
