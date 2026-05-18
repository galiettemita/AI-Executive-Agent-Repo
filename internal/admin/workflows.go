package admin

import (
	"sync"
	"time"
)

// WorkflowResult holds the result of an admin workflow execution.
type WorkflowResult struct {
	WorkflowName string    `json:"workflow_name"`
	Status       string    `json:"status"` // completed, failed
	StartedAt    time.Time `json:"started_at"`
	CompletedAt  time.Time `json:"completed_at"`
	Details      any       `json:"details,omitempty"`
	Error        string    `json:"error,omitempty"`
}

// CostRollupWorkflow simulates a cost rollup workflow execution.
func CostRollupWorkflow(costSvc *CostAttributionService, workspaceID, workflowExecutionID string) WorkflowResult {
	start := time.Now().UTC()
	rollup, err := costSvc.RollupTaskCost(workspaceID, workflowExecutionID)
	if err != nil {
		return WorkflowResult{
			WorkflowName: "CostRollupWorkflow",
			Status:       "failed",
			StartedAt:    start,
			CompletedAt:  time.Now().UTC(),
			Error:        err.Error(),
		}
	}
	return WorkflowResult{
		WorkflowName: "CostRollupWorkflow",
		Status:       "completed",
		StartedAt:    start,
		CompletedAt:  time.Now().UTC(),
		Details:      rollup,
	}
}

// DailyUserCostRollupWorkflow simulates a daily user cost rollup.
func DailyUserCostRollupWorkflow(costSvc *CostAttributionService, workspaceID, userID string, date time.Time) WorkflowResult {
	start := time.Now().UTC()
	rollup, err := costSvc.RollupDailyUserCost(workspaceID, userID, date)
	if err != nil {
		return WorkflowResult{
			WorkflowName: "DailyUserCostRollupWorkflow",
			Status:       "failed",
			StartedAt:    start,
			CompletedAt:  time.Now().UTC(),
			Error:        err.Error(),
		}
	}
	return WorkflowResult{
		WorkflowName: "DailyUserCostRollupWorkflow",
		Status:       "completed",
		StartedAt:    start,
		CompletedAt:  time.Now().UTC(),
		Details:      rollup,
	}
}

// MarginSnapshotWorkflow simulates a margin snapshot computation.
func MarginSnapshotWorkflow(revSvc *RevenueOpsService, workspaceID string) WorkflowResult {
	start := time.Now().UTC()
	report := revSvc.GetMarginReport(workspaceID)
	return WorkflowResult{
		WorkflowName: "MarginSnapshotWorkflow",
		Status:       "completed",
		StartedAt:    start,
		CompletedAt:  time.Now().UTC(),
		Details:      report,
	}
}

// MRRSnapshotWorkflow simulates an MRR snapshot computation.
func MRRSnapshotWorkflow(revSvc *RevenueOpsService, workspaceID string) WorkflowResult {
	start := time.Now().UTC()
	snap, err := revSvc.ComputeMRRSnapshot(workspaceID)
	if err != nil {
		return WorkflowResult{
			WorkflowName: "MRRSnapshotWorkflow",
			Status:       "failed",
			StartedAt:    start,
			CompletedAt:  time.Now().UTC(),
			Error:        err.Error(),
		}
	}
	return WorkflowResult{
		WorkflowName: "MRRSnapshotWorkflow",
		Status:       "completed",
		StartedAt:    start,
		CompletedAt:  time.Now().UTC(),
		Details:      snap,
	}
}

// CohortComputeWorkflow simulates a cohort retention computation.
func CohortComputeWorkflow(revSvc *RevenueOpsService, workspaceID, userID string, signupWeek time.Time, retentionWeek int, isActive bool) WorkflowResult {
	start := time.Now().UTC()
	cohort := revSvc.ComputeCohortRetention(workspaceID, userID, signupWeek, retentionWeek, isActive)
	return WorkflowResult{
		WorkflowName: "CohortComputeWorkflow",
		Status:       "completed",
		StartedAt:    start,
		CompletedAt:  time.Now().UTC(),
		Details:      cohort,
	}
}

// BehavioralRiskWorkflow simulates a behavioral risk computation.
func BehavioralRiskWorkflow(riskSvc *BehavioralRiskService, workspaceID, userID string) WorkflowResult {
	start := time.Now().UTC()
	score, err := riskSvc.ComputeRisk(workspaceID, userID)
	if err != nil {
		return WorkflowResult{
			WorkflowName: "BehavioralRiskWorkflow",
			Status:       "failed",
			StartedAt:    start,
			CompletedAt:  time.Now().UTC(),
			Error:        err.Error(),
		}
	}
	return WorkflowResult{
		WorkflowName: "BehavioralRiskWorkflow",
		Status:       "completed",
		StartedAt:    start,
		CompletedAt:  time.Now().UTC(),
		Details:      score,
	}
}

// OAuthExpiryCheckWorkflow simulates an OAuth expiry check.
func OAuthExpiryCheckWorkflow(oauthSvc *OAuthMonitorService, within time.Duration) WorkflowResult {
	start := time.Now().UTC()
	expiring := oauthSvc.GetExpiringTokens(within)
	return WorkflowResult{
		WorkflowName: "OAuthExpiryCheckWorkflow",
		Status:       "completed",
		StartedAt:    start,
		CompletedAt:  time.Now().UTC(),
		Details:      expiring,
	}
}

// AdminWorkflowService wraps all admin workflows with a unified interface.
type AdminWorkflowService struct {
	mu       sync.Mutex
	costSvc  *CostAttributionService
	revSvc   *RevenueOpsService
	riskSvc  *BehavioralRiskService
	oauthSvc *OAuthMonitorService
	history  []WorkflowResult
}

// NewAdminWorkflowService creates a new AdminWorkflowService.
func NewAdminWorkflowService(
	costSvc *CostAttributionService,
	revSvc *RevenueOpsService,
	riskSvc *BehavioralRiskService,
	oauthSvc *OAuthMonitorService,
) *AdminWorkflowService {
	return &AdminWorkflowService{
		costSvc:  costSvc,
		revSvc:   revSvc,
		riskSvc:  riskSvc,
		oauthSvc: oauthSvc,
		history:  []WorkflowResult{},
	}
}

func (s *AdminWorkflowService) record(r WorkflowResult) WorkflowResult {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.history = append(s.history, r)
	return r
}

// RunCostRollup runs the cost rollup workflow.
func (s *AdminWorkflowService) RunCostRollup(workspaceID, workflowExecutionID string) WorkflowResult {
	return s.record(CostRollupWorkflow(s.costSvc, workspaceID, workflowExecutionID))
}

// RunDailyRollup runs the daily user cost rollup workflow.
func (s *AdminWorkflowService) RunDailyRollup(workspaceID, userID string, date time.Time) WorkflowResult {
	return s.record(DailyUserCostRollupWorkflow(s.costSvc, workspaceID, userID, date))
}

// RunMarginSnapshot runs the margin snapshot workflow.
func (s *AdminWorkflowService) RunMarginSnapshot(workspaceID string) WorkflowResult {
	return s.record(MarginSnapshotWorkflow(s.revSvc, workspaceID))
}

// RunMRRSnapshot runs the MRR snapshot workflow.
func (s *AdminWorkflowService) RunMRRSnapshot(workspaceID string) WorkflowResult {
	return s.record(MRRSnapshotWorkflow(s.revSvc, workspaceID))
}

// RunCohortCompute runs the cohort compute workflow.
func (s *AdminWorkflowService) RunCohortCompute(workspaceID, userID string, signupWeek time.Time, retentionWeek int, isActive bool) WorkflowResult {
	return s.record(CohortComputeWorkflow(s.revSvc, workspaceID, userID, signupWeek, retentionWeek, isActive))
}

// RunBehavioralRisk runs the behavioral risk workflow.
func (s *AdminWorkflowService) RunBehavioralRisk(workspaceID, userID string) WorkflowResult {
	return s.record(BehavioralRiskWorkflow(s.riskSvc, workspaceID, userID))
}

// RunOAuthExpiryCheck runs the OAuth expiry check workflow.
func (s *AdminWorkflowService) RunOAuthExpiryCheck(within time.Duration) WorkflowResult {
	return s.record(OAuthExpiryCheckWorkflow(s.oauthSvc, within))
}

// GetHistory returns the workflow execution history.
func (s *AdminWorkflowService) GetHistory() []WorkflowResult {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := make([]WorkflowResult, len(s.history))
	copy(out, s.history)
	return out
}

// GetHistoryByName returns workflow results filtered by name.
func (s *AdminWorkflowService) GetHistoryByName(name string) []WorkflowResult {
	s.mu.Lock()
	defer s.mu.Unlock()
	var result []WorkflowResult
	for _, r := range s.history {
		if r.WorkflowName == name {
			result = append(result, r)
		}
	}
	return result
}

// FailedWorkflows returns only failed workflow results.
func (s *AdminWorkflowService) FailedWorkflows() []WorkflowResult {
	s.mu.Lock()
	defer s.mu.Unlock()
	var result []WorkflowResult
	for _, r := range s.history {
		if r.Status == "failed" {
			result = append(result, r)
		}
	}
	return result
}

