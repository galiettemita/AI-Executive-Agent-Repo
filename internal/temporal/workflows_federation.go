package temporal

import (
	"time"

	"go.temporal.io/sdk/temporal"
	"go.temporal.io/sdk/workflow"
)

// FederationSyncInput is the input for federation synchronization.
type FederationSyncInput struct {
	SourceWorkspaceID string `json:"source_workspace_id"`
	TargetWorkspaceID string `json:"target_workspace_id"`
	SyncType          string `json:"sync_type"` // full, incremental
}

// FederationSyncResult is the output of federation synchronization.
type FederationSyncResult struct {
	ItemsSynced    int    `json:"items_synced"`
	ConflictsFound int    `json:"conflicts_found"`
	Status         string `json:"status"`
}

// FederationSyncWorkflow synchronizes data between federated workspaces.
func FederationSyncWorkflow(ctx workflow.Context, input FederationSyncInput) (*FederationSyncResult, error) {
	logger := workflow.GetLogger(ctx)
	logger.Info("FederationSyncWorkflow started",
		"source", input.SourceWorkspaceID,
		"target", input.TargetWorkspaceID)

	ao := workflow.ActivityOptions{
		StartToCloseTimeout: 300 * time.Second,
		RetryPolicy: &temporal.RetryPolicy{
			InitialInterval:    2 * time.Second,
			BackoffCoefficient: 2.0,
			MaximumInterval:    120 * time.Second,
			MaximumAttempts:    3,
		},
	}
	ctx = workflow.WithActivityOptions(ctx, ao)

	var a *Activities
	var result FederationSyncResult
	err := workflow.ExecuteActivity(ctx, a.ExecuteFederationSyncActivity, input).Get(ctx, &result)
	if err != nil {
		return &FederationSyncResult{Status: "FAILED"}, nil
	}
	return &result, nil
}
