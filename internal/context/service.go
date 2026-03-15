package contextlayer

import (
	"context"
	"fmt"
	"log"
	"sort"
	"strings"
	"sync"
	"time"
)

// WorkingMemoryProvider is the context-assembly-facing interface.
type WorkingMemoryProvider interface {
	BuildContextSnippet(ctx context.Context, workspaceID, taskID string) (string, error)
}

type AttentionBudget struct {
	Tier                string
	MaxInputTokens      int
	MaxTotalTokens      int
	MaxLLMCallsPerTurn  int
	MaxContextFitTokens int
}

type ContextSlot struct {
	Slot              int
	Name              string
	MaxTokens         int
	Priority          int
	NeverTruncate     bool
	AllocatedTokens   int
	TruncatedTokens   int
	OriginalTokens    int
	TruncationApplied bool
}

type AssemblyResult struct {
	Tier                string
	BudgetTokens        int
	TotalOriginalTokens int
	TotalFinalTokens    int
	Slots               []ContextSlot
}

type MemoryCandidate struct {
	ID               string
	CosineSimilarity float64
	CreatedAt        time.Time
}

type ConversationTurn struct {
	IngressTurnID string
	CreatedAt     time.Time
}

type ToolResultItem struct {
	Sequence        int
	ToolExecutionID string
}

type EvidenceItem struct {
	Confidence   float64
	SourceTurnID string
}

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
	workingMemory WorkingMemoryProvider // may be nil (gracefully skipped)
	mu            sync.RWMutex
	budgets       map[string]Budget
	allocations   map[string][]Allocation
	reports       map[string]AllocationReport
}

func NewService() *Service {
	return &Service{
		budgets:     map[string]Budget{},
		allocations: map[string][]Allocation{},
		reports:     map[string]AllocationReport{},
	}
}

// SetWorkingMemory injects the working memory provider after construction.
func (s *Service) SetWorkingMemory(wm WorkingMemoryProvider) {
	s.workingMemory = wm
}

// MinContextConfidence is the minimum certainty for a memory item to appear in context.
const MinContextConfidence = 0.3

// FilterByConfidence removes memory items below the confidence threshold.
// Items with Confidence=0 are treated as 1.0 for backwards compatibility.
func FilterByConfidence(items []MemoryCandidate) []MemoryCandidate {
	filtered := make([]MemoryCandidate, 0, len(items))
	for _, item := range items {
		conf := item.CosineSimilarity // proxy: candidates with score >= threshold pass
		if conf == 0 {
			conf = 1.0
		}
		if conf >= MinContextConfidence {
			filtered = append(filtered, item)
		}
	}
	return filtered
}

// ContradictionExclusionThreshold: contradicted items below this score are excluded.
const ContradictionExclusionThreshold = 0.50

// FilterContradicted applies scoring penalties and exclusion for contradicted memory items.
// Items with IsContradicted=true and score below threshold are excluded entirely.
// Items above threshold are included with a 40% penalty.
type ContradictionFilterableItem struct {
	IsContradicted bool
	Score          float64
	RelevanceScore float64
	Body           string
}

// RankByRelevanceAndConfidence sorts memory candidates by effective score.
func RankByRelevanceAndConfidence(items []MemoryCandidate) []MemoryCandidate {
	out := append([]MemoryCandidate(nil), items...)
	sort.Slice(out, func(i, j int) bool {
		si := out[i].CosineSimilarity
		sj := out[j].CosineSimilarity
		if si != sj {
			return si > sj
		}
		return out[i].CreatedAt.After(out[j].CreatedAt)
	})
	return out
}

// AssembleWorkingMemorySlot populates the working_memory context slot for a task.
// Returns empty string when no meaningful state is present (slot is omitted).
func (s *Service) AssembleWorkingMemorySlot(ctx context.Context, workspaceID, taskID string) string {
	if taskID == "" || s.workingMemory == nil {
		return ""
	}
	snippet, err := s.workingMemory.BuildContextSnippet(ctx, workspaceID, taskID)
	if err != nil {
		log.Printf("[context] working_memory snippet failed: %v", err)
		return ""
	}
	return snippet
}

func AttentionBudgetForTier(tier string) AttentionBudget {
	switch strings.ToUpper(strings.TrimSpace(tier)) {
	case "T0":
		return AttentionBudget{Tier: "T0", MaxInputTokens: 4000, MaxTotalTokens: 5000, MaxLLMCallsPerTurn: 1, MaxContextFitTokens: 4000}
	case "T1":
		return AttentionBudget{Tier: "T1", MaxInputTokens: 16000, MaxTotalTokens: 20000, MaxLLMCallsPerTurn: 3, MaxContextFitTokens: 16000}
	case "T2":
		return AttentionBudget{Tier: "T2", MaxInputTokens: 32000, MaxTotalTokens: 50000, MaxLLMCallsPerTurn: 8, MaxContextFitTokens: 32000}
	case "T3":
		return AttentionBudget{Tier: "T3", MaxInputTokens: 64000, MaxTotalTokens: 100000, MaxLLMCallsPerTurn: 15, MaxContextFitTokens: 64000}
	default:
		return AttentionBudget{Tier: "T1", MaxInputTokens: 16000, MaxTotalTokens: 20000, MaxLLMCallsPerTurn: 3, MaxContextFitTokens: 16000}
	}
}

func DefaultContextSlots() []ContextSlot {
	return []ContextSlot{
		{Slot: 1, Name: "system_prompt", MaxTokens: 2000, NeverTruncate: true, Priority: 999},
		{Slot: 2, Name: "workspace_context", MaxTokens: 1000, Priority: 7},
		{Slot: 3, Name: "tool_registry", MaxTokens: 2000, Priority: 6},
		{Slot: 4, Name: "memory_items", MaxTokens: 3000, Priority: 4},
		{Slot: 5, Name: "conversation_history", MaxTokens: 4000, Priority: 3},
		{Slot: 6, Name: "current_turn", MaxTokens: 2000, NeverTruncate: true, Priority: 999},
		{Slot: 7, Name: "prior_tool_results", MaxTokens: 2000, Priority: 5},
		{Slot: 8, Name: "evidence_citations", MaxTokens: 1000, Priority: 2},
		{Slot: 9, Name: "working_memory", MaxTokens: 1500, Priority: 8},
		{Slot: 10, Name: "proactive_memories", MaxTokens: 600, Priority: 6},
		{Slot: 11, Name: "knowledge_graph", MaxTokens: 800, Priority: 7},
		{Slot: 12, Name: "transferred_preferences", MaxTokens: 400, Priority: 5},
	}
}

// AssembleDeterministicContext applies the Section R slot model and truncation
// priorities until token usage fits the tier budget.
func AssembleDeterministicContext(tier string, requestedBySlot map[string]int) AssemblyResult {
	slots := DefaultContextSlots()
	budget := AttentionBudgetForTier(tier)
	result := AssemblyResult{
		Tier:         budget.Tier,
		BudgetTokens: budget.MaxContextFitTokens,
		Slots:        make([]ContextSlot, 0, len(slots)),
	}

	for _, slot := range slots {
		req := requestedBySlot[slot.Name]
		if req < 0 {
			req = 0
		}
		slot.OriginalTokens = req
		slot.AllocatedTokens = minInt(req, slot.MaxTokens)
		result.TotalOriginalTokens += slot.OriginalTokens
		result.TotalFinalTokens += slot.AllocatedTokens
		result.Slots = append(result.Slots, slot)
	}

	if result.TotalFinalTokens <= result.BudgetTokens {
		return result
	}

	indices := make([]int, 0, len(result.Slots))
	for idx, slot := range result.Slots {
		if slot.NeverTruncate {
			continue
		}
		indices = append(indices, idx)
	}
	sort.Slice(indices, func(i, j int) bool {
		left := result.Slots[indices[i]]
		right := result.Slots[indices[j]]
		if left.Priority == right.Priority {
			return left.Slot < right.Slot
		}
		return left.Priority < right.Priority
	})

	overflow := result.TotalFinalTokens - result.BudgetTokens
	for _, idx := range indices {
		if overflow <= 0 {
			break
		}
		slot := result.Slots[idx]
		if slot.AllocatedTokens <= 0 {
			continue
		}
		trim := minInt(slot.AllocatedTokens, overflow)
		slot.AllocatedTokens -= trim
		slot.TruncatedTokens += trim
		slot.TruncationApplied = trim > 0
		result.Slots[idx] = slot
		result.TotalFinalTokens -= trim
		overflow -= trim
	}
	return result
}

func SortMemoryCandidates(items []MemoryCandidate) []MemoryCandidate {
	out := append([]MemoryCandidate(nil), items...)
	sort.Slice(out, func(i, j int) bool {
		if out[i].CosineSimilarity == out[j].CosineSimilarity {
			return out[i].CreatedAt.After(out[j].CreatedAt)
		}
		return out[i].CosineSimilarity > out[j].CosineSimilarity
	})
	return out
}

func SortConversationTurns(items []ConversationTurn) []ConversationTurn {
	out := append([]ConversationTurn(nil), items...)
	sort.Slice(out, func(i, j int) bool {
		if out[i].CreatedAt.Equal(out[j].CreatedAt) {
			return out[i].IngressTurnID > out[j].IngressTurnID
		}
		return out[i].CreatedAt.After(out[j].CreatedAt)
	})
	return out
}

func SortToolResultItems(items []ToolResultItem) []ToolResultItem {
	out := append([]ToolResultItem(nil), items...)
	sort.Slice(out, func(i, j int) bool {
		if out[i].Sequence == out[j].Sequence {
			return out[i].ToolExecutionID < out[j].ToolExecutionID
		}
		return out[i].Sequence < out[j].Sequence
	})
	return out
}

func SortEvidenceItems(items []EvidenceItem) []EvidenceItem {
	out := append([]EvidenceItem(nil), items...)
	sort.Slice(out, func(i, j int) bool {
		if out[i].Confidence == out[j].Confidence {
			return out[i].SourceTurnID > out[j].SourceTurnID
		}
		return out[i].Confidence > out[j].Confidence
	})
	return out
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
