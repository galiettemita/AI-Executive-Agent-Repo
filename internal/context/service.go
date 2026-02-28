package contextlayer

import (
	"fmt"
	"sort"
	"strings"
	"sync"
	"time"
)

type Budget struct {
	WorkspaceID            string `json:"workspace_id"`
	Tier                   string `json:"tier"`
	MaxContextTokens       int    `json:"max_context_tokens"`
	ReservedResponseTokens int    `json:"reserved_response_tokens"`
	MaxRAGTokens           int    `json:"max_rag_tokens"`
	BudgetTokens           int    `json:"budget_tokens"`
	Status                 string `json:"status"`
}

type Allocation struct {
	ItemType        string `json:"item_type"`
	AllocatedTokens int    `json:"allocated_tokens"`
}

type AllocationReport struct {
	IngressTurnID          string `json:"ingress_turn_id"`
	TotalBudgetTokens      int    `json:"total_budget_tokens"`
	AllocatedPromptTokens  int    `json:"allocated_prompt_tokens"`
	AllocatedRAGTokens     int    `json:"allocated_rag_tokens"`
	AllocatedHistoryTokens int    `json:"allocated_history_tokens"`
	Overflowed             bool   `json:"overflowed"`
}

type Service struct {
	mu          sync.RWMutex
	budgets     map[string]Budget
	allocations map[string][]Allocation
	reports     map[string]AllocationReport
}

func NewService() *Service {
	return &Service{
		budgets:     map[string]Budget{},
		allocations: map[string][]Allocation{},
		reports:     map[string]AllocationReport{},
	}
}

func (s *Service) SetBudget(workspaceID string, budgetTokens int, status string) Budget {
	return s.UpsertBudgetConfig(workspaceID, "T2", budgetTokens, 256, 512, status)
}

func (s *Service) UpsertBudgetConfig(workspaceID, tier string, maxContextTokens, reservedResponseTokens, maxRAGTokens int, status string) Budget {
	s.mu.Lock()
	defer s.mu.Unlock()

	workspaceID = normalizeWorkspaceID(workspaceID)
	if maxContextTokens <= 0 {
		maxContextTokens = 2048
	}
	if reservedResponseTokens <= 0 {
		reservedResponseTokens = 256
	}
	if maxRAGTokens < 0 {
		maxRAGTokens = 0
	}
	if strings.TrimSpace(tier) == "" {
		tier = "T2"
	}
	if strings.TrimSpace(status) == "" {
		status = "active"
	}

	budget := Budget{
		WorkspaceID:            workspaceID,
		Tier:                   tier,
		MaxContextTokens:       maxContextTokens,
		ReservedResponseTokens: reservedResponseTokens,
		MaxRAGTokens:           maxRAGTokens,
		BudgetTokens:           maxContextTokens,
		Status:                 status,
	}
	s.budgets[workspaceID] = budget
	return budget
}

func (s *Service) GetBudget(workspaceID string) (Budget, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	workspaceID = normalizeWorkspaceID(workspaceID)
	budget, ok := s.budgets[workspaceID]
	return budget, ok
}

func (s *Service) SetAllocations(workspaceID string, allocations map[string]int) []Allocation {
	s.mu.Lock()
	defer s.mu.Unlock()

	workspaceID = normalizeWorkspaceID(workspaceID)
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

	report := s.reports[workspaceID]
	if strings.TrimSpace(report.IngressTurnID) == "" {
		report.IngressTurnID = fmt.Sprintf("context_%d", time.Now().UTC().UnixNano())
	}
	budget := s.budgets[workspaceID]
	report.TotalBudgetTokens = budget.MaxContextTokens
	report.AllocatedHistoryTokens = extractAllocationToken(allocations, "history")
	report.AllocatedRAGTokens = extractAllocationToken(allocations, "rag", "retrieval")
	report.AllocatedPromptTokens = extractAllocationToken(allocations, "prompt", "tool")
	report.Overflowed = report.AllocatedPromptTokens+report.AllocatedRAGTokens+report.AllocatedHistoryTokens > effectiveRequestBudget(budget)
	s.reports[workspaceID] = report

	return out
}

func (s *Service) GetAllocations(workspaceID string) []Allocation {
	s.mu.RLock()
	defer s.mu.RUnlock()
	workspaceID = normalizeWorkspaceID(workspaceID)
	out := make([]Allocation, len(s.allocations[workspaceID]))
	copy(out, s.allocations[workspaceID])
	return out
}

func (s *Service) AllocateContext(workspaceID, ingressTurnID string, promptRequestedTokens, ragRequestedTokens, historyRequestedTokens int) (AllocationReport, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	workspaceID = normalizeWorkspaceID(workspaceID)
	budget, ok := s.budgets[workspaceID]
	if !ok {
		return AllocationReport{}, fmt.Errorf("CONTEXT_BUDGET_EXCEEDED")
	}
	if strings.TrimSpace(ingressTurnID) == "" {
		ingressTurnID = fmt.Sprintf("turn_%d", time.Now().UTC().UnixNano())
	}
	available := effectiveRequestBudget(budget)
	if available <= 0 {
		return AllocationReport{}, fmt.Errorf("CONTEXT_BUDGET_EXCEEDED")
	}

	report := AllocationReport{
		IngressTurnID:     ingressTurnID,
		TotalBudgetTokens: budget.MaxContextTokens,
	}

	report.AllocatedPromptTokens = minInt(maxInt(promptRequestedTokens, 0), available)
	remaining := available - report.AllocatedPromptTokens

	maxRAGAllowed := minInt(budget.MaxRAGTokens, remaining)
	report.AllocatedRAGTokens = minInt(maxInt(ragRequestedTokens, 0), maxRAGAllowed)
	remaining -= report.AllocatedRAGTokens

	report.AllocatedHistoryTokens = minInt(maxInt(historyRequestedTokens, 0), remaining)
	requestedTotal := maxInt(promptRequestedTokens, 0) + maxInt(ragRequestedTokens, 0) + maxInt(historyRequestedTokens, 0)
	report.Overflowed = requestedTotal > available
	if report.Overflowed {
		s.reports[workspaceID] = report
		s.allocations[workspaceID] = []Allocation{
			{ItemType: "history", AllocatedTokens: report.AllocatedHistoryTokens},
			{ItemType: "rag", AllocatedTokens: report.AllocatedRAGTokens},
			{ItemType: "prompt", AllocatedTokens: report.AllocatedPromptTokens},
		}
		return report, fmt.Errorf("CONTEXT_BUDGET_EXCEEDED")
	}

	s.reports[workspaceID] = report
	s.allocations[workspaceID] = []Allocation{
		{ItemType: "history", AllocatedTokens: report.AllocatedHistoryTokens},
		{ItemType: "rag", AllocatedTokens: report.AllocatedRAGTokens},
		{ItemType: "prompt", AllocatedTokens: report.AllocatedPromptTokens},
	}
	return report, nil
}

func (s *Service) GetAllocationReport(workspaceID string) (AllocationReport, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	workspaceID = normalizeWorkspaceID(workspaceID)
	report, ok := s.reports[workspaceID]
	return report, ok
}

func normalizeWorkspaceID(workspaceID string) string {
	if strings.TrimSpace(workspaceID) == "" {
		return "default"
	}
	return workspaceID
}

func effectiveRequestBudget(budget Budget) int {
	total := budget.MaxContextTokens - budget.ReservedResponseTokens
	if total < 0 {
		return 0
	}
	return total
}

func extractAllocationToken(allocations map[string]int, primary string, fallbacks ...string) int {
	if value, ok := allocations[primary]; ok {
		return value
	}
	for _, fallback := range fallbacks {
		if value, ok := allocations[fallback]; ok {
			return value
		}
	}
	return 0
}

func minInt(left, right int) int {
	if left < right {
		return left
	}
	return right
}

func maxInt(left, right int) int {
	if left > right {
		return left
	}
	return right
}
