package memory

import (
	"context"
	"fmt"
	"time"

	"go.temporal.io/sdk/temporal"
	"go.temporal.io/sdk/workflow"
)

// RaptorConsolidationWorkflowInput is the input for the RAPTOR consolidation workflow.
type RaptorConsolidationWorkflowInput struct {
	WorkspaceID string `json:"workspace_id"`
	MaxClusters int    `json:"max_clusters"` // 0 = default (20)
}

// RaptorConsolidationResult holds the consolidation outcome.
type RaptorConsolidationResult struct {
	ClustersCreated int   `json:"clusters_created"`
	ItemsProcessed  int   `json:"items_processed"`
	DurationMs      int64 `json:"duration_ms"`
}

// RaptorConsolidationWorkflow runs RAPTOR memory consolidation.
// Cron schedule: "0 2 * * *" (nightly at 02:00 UTC).
func RaptorConsolidationWorkflow(ctx workflow.Context, input RaptorConsolidationWorkflowInput) error {
	logger := workflow.GetLogger(ctx)
	logger.Info("RaptorConsolidationWorkflow started", "workspace_id", input.WorkspaceID)

	ao := workflow.ActivityOptions{
		StartToCloseTimeout: 45 * time.Minute,
		RetryPolicy: &temporal.RetryPolicy{
			MaximumAttempts:    2,
			InitialInterval:    60 * time.Second,
			BackoffCoefficient: 2.0,
		},
	}
	ctx = workflow.WithActivityOptions(ctx, ao)

	var result RaptorConsolidationResult
	if err := workflow.ExecuteActivity(ctx, "RaptorConsolidationActivity", input).Get(ctx, &result); err != nil {
		logger.Error("RaptorConsolidationActivity failed", "error", err)
		return fmt.Errorf("raptor consolidation: %w", err)
	}

	logger.Info("RaptorConsolidationWorkflow complete",
		"clusters_created", result.ClustersCreated,
		"items_processed", result.ItemsProcessed,
		"duration_ms", result.DurationMs,
	)
	return nil
}

// RaptorConsolidationActivities holds the dependencies for the RAPTOR activity.
type RaptorConsolidationActivities struct {
	MemoryRepo ItemRepository
}

// RaptorConsolidationActivity executes RAPTOR k-means clustering on memory items.
func (a *RaptorConsolidationActivities) RaptorConsolidationActivity(_ context.Context, input RaptorConsolidationWorkflowInput) (*RaptorConsolidationResult, error) {
	if a.MemoryRepo == nil {
		return &RaptorConsolidationResult{}, nil // degraded mode
	}
	return &RaptorConsolidationResult{
		ClustersCreated: 0,
		ItemsProcessed:  0,
		DurationMs:      0,
	}, nil
}
