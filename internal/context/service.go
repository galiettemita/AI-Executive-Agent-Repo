package contextlayer

import (
	"sort"
	"sync"
)

type Budget struct {
	WorkspaceID  string `json:"workspace_id"`
	BudgetTokens int    `json:"budget_tokens"`
	Status       string `json:"status"`
}

type Allocation struct {
	ItemType        string `json:"item_type"`
	AllocatedTokens int    `json:"allocated_tokens"`
}

type Service struct {
	mu          sync.RWMutex
	budgets     map[string]Budget
	allocations map[string][]Allocation
}

func NewService() *Service {
	return &Service{
		budgets:     map[string]Budget{},
		allocations: map[string][]Allocation{},
	}
}

func (s *Service) SetBudget(workspaceID string, budgetTokens int, status string) Budget {
	s.mu.Lock()
	defer s.mu.Unlock()
	budget := Budget{
		WorkspaceID:  workspaceID,
		BudgetTokens: budgetTokens,
		Status:       status,
	}
	s.budgets[workspaceID] = budget
	return budget
}

func (s *Service) GetBudget(workspaceID string) (Budget, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	budget, ok := s.budgets[workspaceID]
	return budget, ok
}

func (s *Service) SetAllocations(workspaceID string, allocations map[string]int) []Allocation {
	s.mu.Lock()
	defer s.mu.Unlock()

	itemTypes := make([]string, 0, len(allocations))
	for itemType := range allocations {
		itemTypes = append(itemTypes, itemType)
	}
	sort.Strings(itemTypes)

	out := make([]Allocation, 0, len(itemTypes))
	for _, itemType := range itemTypes {
		out = append(out, Allocation{ItemType: itemType, AllocatedTokens: allocations[itemType]})
	}
	s.allocations[workspaceID] = out
	return out
}

func (s *Service) GetAllocations(workspaceID string) []Allocation {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]Allocation, len(s.allocations[workspaceID]))
	copy(out, s.allocations[workspaceID])
	return out
}
