package temporal

import (
	"time"

	"go.temporal.io/sdk/temporal"
	"go.temporal.io/sdk/workflow"
)

// MemoryContextMaintenanceWorkflow orchestrates the v10.2 P8 memory/context pipeline:
// 1. Apply memory decay
// 2. Enforce context budget
// 3. Evaluate latency budget
// 4. Warm fast-path cache
func MemoryContextMaintenanceWorkflow(ctx workflow.Context, input struct {
	WorkspaceID   string  `json:"workspace_id"`
	SessionID     string  `json:"session_id"`
	IngressTurnID string  `json:"ingress_turn_id"`
	HalfLifeDays  float64 `json:"half_life_days"`
	BudgetMs      float64 `json:"budget_ms"`
	ElapsedMs     float64 `json:"elapsed_ms"`
}) error {
	ao := workflow.ActivityOptions{
		StartToCloseTimeout: 30 * time.Second,
		RetryPolicy: &temporal.RetryPolicy{
			MaximumAttempts: 2,
		},
	}
	ctx = workflow.WithActivityOptions(ctx, ao)

	workflowRunID := workflow.GetInfo(ctx).WorkflowExecution.RunID

	// Step 1: Memory decay.
	if err := workflow.ExecuteActivity(ctx, (*Activities).ApplyMemoryDecayActivity, ApplyMemoryDecayInput{
		WorkspaceID:   input.WorkspaceID,
		DecayFunction: "exponential",
		HalfLifeDays:  input.HalfLifeDays,
		MinWeight:     0.05,
		Purge:         true,
	}).Get(ctx, nil); err != nil {
		_ = err // Non-fatal.
	}

	// Step 2: Context budget enforcement.
	if err := workflow.ExecuteActivity(ctx, (*Activities).EnforceContextBudgetActivity, EnforceContextBudgetInput{
		WorkspaceID:            input.WorkspaceID,
		IngressTurnID:          input.IngressTurnID,
		PromptRequestedTokens:  4000,
		RAGRequestedTokens:     2000,
		HistoryRequestedTokens: 8000,
	}).Get(ctx, nil); err != nil {
		_ = err // Non-fatal.
	}

	// Step 3: Latency budget check.
	var latencyResult EvaluateLatencyBudgetResult
	if err := workflow.ExecuteActivity(ctx, (*Activities).EvaluateLatencyBudgetActivity, EvaluateLatencyBudgetInput{
		WorkspaceID:     input.WorkspaceID,
		WorkflowRunID:   workflowRunID,
		BudgetMs:        input.BudgetMs,
		ElapsedMs:       input.ElapsedMs,
		EstimatedNextMs: 500,
	}).Get(ctx, &latencyResult); err != nil {
		_ = err
	}

	// Step 4: Warm fast-path cache (only if latency allows).
	if latencyResult.ShouldProceed {
		_ = workflow.ExecuteActivity(ctx, (*Activities).WarmFastPathCacheActivity, WarmFastPathCacheInput{
			WorkspaceID: input.WorkspaceID,
			MaxRoutes:   100,
		}).Get(ctx, nil)
	}

	return nil
}
