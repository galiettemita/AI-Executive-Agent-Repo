package temporal

import (
	"context"
	"fmt"

	"go.temporal.io/sdk/activity"
	temporalSDK "go.temporal.io/sdk/temporal"

	"github.com/brevio/brevio/internal/subagent"
)

var tierRank = map[string]int{"A0": 0, "A1": 1, "A2": 2, "A3": 3, "A4": 4}

// CheckSubAgentAutonomyActivity verifies the workspace has reached autonomy tier A3+.
func (a *Activities) CheckSubAgentAutonomyActivity(ctx context.Context, in subagent.CheckAutonomyInput) (subagent.CheckAutonomyResult, error) {
	logger := activity.GetLogger(ctx)

	if in.WorkspaceID == "" {
		return subagent.CheckAutonomyResult{}, temporalSDK.NewNonRetryableApplicationError("workspace_id required", "INVALID_INPUT", nil)
	}

	required := in.RequiredTier
	if required == "" {
		required = "A3"
	}

	currentTier := "A1"
	if a.trustSvc != nil {
		for _, score := range a.trustSvc.ListScores() {
			if score.WorkspaceID == in.WorkspaceID {
				currentTier = score.CurrentAutonomy
				break
			}
		}
	}

	requiredRank, ok := tierRank[required]
	if !ok {
		requiredRank = 3
	}
	permitted := tierRank[currentTier] >= requiredRank

	reason := fmt.Sprintf("workspace %s at tier %s (required %s, permitted=%v)",
		in.WorkspaceID, currentTier, required, permitted)
	logger.Info("CheckSubAgentAutonomyActivity",
		"workspace_id", in.WorkspaceID, "current_tier", currentTier,
		"required_tier", required, "permitted", permitted)

	return subagent.CheckAutonomyResult{CurrentTier: currentTier, Permitted: permitted, Reason: reason}, nil
}

// DecomposeSubTasksActivity applies the Decompose function to a tool list.
func (a *Activities) DecomposeSubTasksActivity(ctx context.Context, in subagent.DecomposeInput) (subagent.DecompositionResult, error) {
	if in.Intent == "" {
		return subagent.DecompositionResult{}, temporalSDK.NewNonRetryableApplicationError("intent required", "INVALID_INPUT", nil)
	}
	result := subagent.Decompose(in.Intent, in.ToolKeys)
	activity.GetLogger(ctx).Info("DecomposeSubTasksActivity",
		"can_parallelize", result.CanParallelize, "sub_tasks", len(result.SubTasks))
	return result, nil
}
