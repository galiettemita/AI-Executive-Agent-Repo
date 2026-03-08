package admin

import (
	"fmt"
	"sync"
	"time"

	"github.com/google/uuid"
)

// LLMCostRecord represents a single LLM invocation cost entry.
type LLMCostRecord struct {
	ID                  string    `json:"id"`
	WorkspaceID         string    `json:"workspace_id"`
	UserID              string    `json:"user_id"`
	ModelID             string    `json:"model_id"`
	InputTokens         int64     `json:"input_tokens"`
	OutputTokens        int64     `json:"output_tokens"`
	CostUSD             float64   `json:"cost_usd"`
	WorkflowExecutionID string    `json:"workflow_execution_id"`
	CreatedAt           time.Time `json:"created_at"`
}

// ConnectorCostRecord represents a connector invocation cost entry.
type ConnectorCostRecord struct {
	ID          string    `json:"id"`
	WorkspaceID string    `json:"workspace_id"`
	UserID      string    `json:"user_id"`
	ConnectorID string    `json:"connector_id"`
	CallCount   int       `json:"call_count"`
	CostUSD     float64   `json:"cost_usd"`
	CreatedAt   time.Time `json:"created_at"`
}

// TaskCostRollup aggregates costs for a single workflow execution.
type TaskCostRollup struct {
	ID                  string    `json:"id"`
	WorkspaceID         string    `json:"workspace_id"`
	WorkflowExecutionID string    `json:"workflow_execution_id"`
	LLMCostUSD          float64   `json:"llm_cost_usd"`
	ConnectorCostUSD    float64   `json:"connector_cost_usd"`
	TotalCostUSD        float64   `json:"total_cost_usd"`
	CreatedAt           time.Time `json:"created_at"`
}

// UserCostDailyRollup aggregates daily costs per user.
type UserCostDailyRollup struct {
	ID               string    `json:"id"`
	WorkspaceID      string    `json:"workspace_id"`
	UserID           string    `json:"user_id"`
	Date             time.Time `json:"date"`
	LLMCostUSD       float64   `json:"llm_cost_usd"`
	ConnectorCostUSD float64   `json:"connector_cost_usd"`
	TotalCostUSD     float64   `json:"total_cost_usd"`
}

// CostSummaryResult holds workspace-level cost summary data.
type CostSummaryResult struct {
	WorkspaceID      string  `json:"workspace_id"`
	TotalLLMCostUSD  float64 `json:"total_llm_cost_usd"`
	TotalConnCostUSD float64 `json:"total_connector_cost_usd"`
	TotalCostUSD     float64 `json:"total_cost_usd"`
	RecordCount      int     `json:"record_count"`
}

// CostProjection holds projected cost data.
type CostProjection struct {
	WorkspaceID       string  `json:"workspace_id"`
	DailyAvgCostUSD   float64 `json:"daily_avg_cost_usd"`
	ProjectedMonthUSD float64 `json:"projected_month_usd"`
	DaysObserved      int     `json:"days_observed"`
}

// CostAttributionService manages cost attribution records.
type CostAttributionService struct {
	mu               sync.Mutex
	llmRecords       []LLMCostRecord
	connectorRecords []ConnectorCostRecord
	taskRollups      []TaskCostRollup
	dailyRollups     []UserCostDailyRollup
	now              func() time.Time
}

// NewCostAttributionService creates a new CostAttributionService.
func NewCostAttributionService() *CostAttributionService {
	return &CostAttributionService{
		llmRecords:       []LLMCostRecord{},
		connectorRecords: []ConnectorCostRecord{},
		taskRollups:      []TaskCostRollup{},
		dailyRollups:     []UserCostDailyRollup{},
		now:              func() time.Time { return time.Now().UTC() },
	}
}

// RecordLLMCost records a single LLM cost entry.
func (s *CostAttributionService) RecordLLMCost(rec LLMCostRecord) (LLMCostRecord, error) {
	if rec.WorkspaceID == "" {
		return LLMCostRecord{}, fmt.Errorf("workspace_id is required")
	}
	if rec.UserID == "" {
		return LLMCostRecord{}, fmt.Errorf("user_id is required")
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	rec.ID = uuid.Must(uuid.NewV7()).String()
	if rec.CreatedAt.IsZero() {
		rec.CreatedAt = s.now()
	}
	s.llmRecords = append(s.llmRecords, rec)
	return rec, nil
}

// RecordConnectorCost records a single connector cost entry.
func (s *CostAttributionService) RecordConnectorCost(rec ConnectorCostRecord) (ConnectorCostRecord, error) {
	if rec.WorkspaceID == "" {
		return ConnectorCostRecord{}, fmt.Errorf("workspace_id is required")
	}
	if rec.UserID == "" {
		return ConnectorCostRecord{}, fmt.Errorf("user_id is required")
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	rec.ID = uuid.Must(uuid.NewV7()).String()
	if rec.CreatedAt.IsZero() {
		rec.CreatedAt = s.now()
	}
	s.connectorRecords = append(s.connectorRecords, rec)
	return rec, nil
}

// RollupTaskCost aggregates LLM and connector costs for a workflow execution.
func (s *CostAttributionService) RollupTaskCost(workspaceID, workflowExecutionID string) (TaskCostRollup, error) {
	if workspaceID == "" {
		return TaskCostRollup{}, fmt.Errorf("workspace_id is required")
	}
	if workflowExecutionID == "" {
		return TaskCostRollup{}, fmt.Errorf("workflow_execution_id is required")
	}
	s.mu.Lock()
	defer s.mu.Unlock()

	var llmCost, connCost float64
	for _, r := range s.llmRecords {
		if r.WorkspaceID == workspaceID && r.WorkflowExecutionID == workflowExecutionID {
			llmCost += r.CostUSD
		}
	}
	for _, r := range s.connectorRecords {
		if r.WorkspaceID == workspaceID {
			connCost += r.CostUSD
		}
	}

	rollup := TaskCostRollup{
		ID:                  uuid.Must(uuid.NewV7()).String(),
		WorkspaceID:         workspaceID,
		WorkflowExecutionID: workflowExecutionID,
		LLMCostUSD:          llmCost,
		ConnectorCostUSD:    connCost,
		TotalCostUSD:        llmCost + connCost,
		CreatedAt:           s.now(),
	}
	s.taskRollups = append(s.taskRollups, rollup)
	return rollup, nil
}

// RollupDailyUserCost aggregates daily costs per user for a given date.
func (s *CostAttributionService) RollupDailyUserCost(workspaceID, userID string, date time.Time) (UserCostDailyRollup, error) {
	if workspaceID == "" {
		return UserCostDailyRollup{}, fmt.Errorf("workspace_id is required")
	}
	if userID == "" {
		return UserCostDailyRollup{}, fmt.Errorf("user_id is required")
	}
	s.mu.Lock()
	defer s.mu.Unlock()

	dayStart := time.Date(date.Year(), date.Month(), date.Day(), 0, 0, 0, 0, time.UTC)
	dayEnd := dayStart.AddDate(0, 0, 1)

	var llmCost, connCost float64
	for _, r := range s.llmRecords {
		if r.WorkspaceID == workspaceID && r.UserID == userID &&
			!r.CreatedAt.Before(dayStart) && r.CreatedAt.Before(dayEnd) {
			llmCost += r.CostUSD
		}
	}
	for _, r := range s.connectorRecords {
		if r.WorkspaceID == workspaceID && r.UserID == userID &&
			!r.CreatedAt.Before(dayStart) && r.CreatedAt.Before(dayEnd) {
			connCost += r.CostUSD
		}
	}

	rollup := UserCostDailyRollup{
		ID:               uuid.Must(uuid.NewV7()).String(),
		WorkspaceID:      workspaceID,
		UserID:           userID,
		Date:             dayStart,
		LLMCostUSD:       llmCost,
		ConnectorCostUSD: connCost,
		TotalCostUSD:     llmCost + connCost,
	}
	s.dailyRollups = append(s.dailyRollups, rollup)
	return rollup, nil
}

// GetCostSummary returns aggregate cost data for a workspace.
func (s *CostAttributionService) GetCostSummary(workspaceID string) (CostSummaryResult, error) {
	if workspaceID == "" {
		return CostSummaryResult{}, fmt.Errorf("workspace_id is required")
	}
	s.mu.Lock()
	defer s.mu.Unlock()

	var llmTotal, connTotal float64
	var count int
	for _, r := range s.llmRecords {
		if r.WorkspaceID == workspaceID {
			llmTotal += r.CostUSD
			count++
		}
	}
	for _, r := range s.connectorRecords {
		if r.WorkspaceID == workspaceID {
			connTotal += r.CostUSD
			count++
		}
	}
	return CostSummaryResult{
		WorkspaceID:      workspaceID,
		TotalLLMCostUSD:  llmTotal,
		TotalConnCostUSD: connTotal,
		TotalCostUSD:     llmTotal + connTotal,
		RecordCount:      count,
	}, nil
}

// GetUserCosts returns all LLM cost records for a user.
func (s *CostAttributionService) GetUserCosts(workspaceID, userID string) ([]LLMCostRecord, error) {
	if workspaceID == "" {
		return nil, fmt.Errorf("workspace_id is required")
	}
	if userID == "" {
		return nil, fmt.Errorf("user_id is required")
	}
	s.mu.Lock()
	defer s.mu.Unlock()

	var result []LLMCostRecord
	for _, r := range s.llmRecords {
		if r.WorkspaceID == workspaceID && r.UserID == userID {
			result = append(result, r)
		}
	}
	return result, nil
}

// GetUserCostBreakdownV2 returns daily rollups for a user.
func (s *CostAttributionService) GetUserCostBreakdownV2(workspaceID, userID string) ([]UserCostDailyRollup, error) {
	if workspaceID == "" {
		return nil, fmt.Errorf("workspace_id is required")
	}
	if userID == "" {
		return nil, fmt.Errorf("user_id is required")
	}
	s.mu.Lock()
	defer s.mu.Unlock()

	var result []UserCostDailyRollup
	for _, r := range s.dailyRollups {
		if r.WorkspaceID == workspaceID && r.UserID == userID {
			result = append(result, r)
		}
	}
	return result, nil
}

// GetCostProjections returns projected monthly costs for a workspace.
func (s *CostAttributionService) GetCostProjections(workspaceID string) (CostProjection, error) {
	if workspaceID == "" {
		return CostProjection{}, fmt.Errorf("workspace_id is required")
	}
	s.mu.Lock()
	defer s.mu.Unlock()

	dailyCosts := map[string]float64{}
	for _, r := range s.llmRecords {
		if r.WorkspaceID == workspaceID {
			day := r.CreatedAt.Format("2006-01-02")
			dailyCosts[day] += r.CostUSD
		}
	}
	for _, r := range s.connectorRecords {
		if r.WorkspaceID == workspaceID {
			day := r.CreatedAt.Format("2006-01-02")
			dailyCosts[day] += r.CostUSD
		}
	}

	if len(dailyCosts) == 0 {
		return CostProjection{WorkspaceID: workspaceID}, nil
	}

	var totalCost float64
	for _, v := range dailyCosts {
		totalCost += v
	}
	days := len(dailyCosts)
	dailyAvg := totalCost / float64(days)

	return CostProjection{
		WorkspaceID:       workspaceID,
		DailyAvgCostUSD:   dailyAvg,
		ProjectedMonthUSD: dailyAvg * 30,
		DaysObserved:      days,
	}, nil
}
