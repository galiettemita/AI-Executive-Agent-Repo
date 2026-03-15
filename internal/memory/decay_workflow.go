package memory

import (
	"context"
	"fmt"
	"time"

	"go.temporal.io/sdk/activity"
	"go.temporal.io/sdk/temporal"
	"go.temporal.io/sdk/workflow"
)

// DecaySweepWorkflowInput is the input for the scheduled decay sweep workflow.
type DecaySweepWorkflowInput struct {
	WorkspaceID string `json:"workspace_id"`
}

// DecaySweepResult holds sweep outcome metrics.
type DecaySweepResult struct {
	ItemsProcessed     int   `json:"items_processed"`
	ItemsUpdated       int   `json:"items_updated"`
	ItemsMarkedDeleted int   `json:"items_marked_deleted"`
	DurationMs         int64 `json:"duration_ms"`
}

// DecaySweepWorkflow is a Temporal workflow that runs periodic memory decay sweeps.
// Schedule: every 6 hours (CRON_SCHEDULE: "0 */6 * * *").
func DecaySweepWorkflow(ctx workflow.Context, input DecaySweepWorkflowInput) error {
	logger := workflow.GetLogger(ctx)
	logger.Info("DecaySweepWorkflow started", "workspace_id", input.WorkspaceID)

	ao := workflow.ActivityOptions{
		StartToCloseTimeout: 30 * time.Minute,
		RetryPolicy: &temporal.RetryPolicy{
			MaximumAttempts:    3,
			InitialInterval:    30 * time.Second,
			BackoffCoefficient: 2.0,
			MaximumInterval:    5 * time.Minute,
		},
	}
	ctx = workflow.WithActivityOptions(ctx, ao)

	var result DecaySweepResult
	if err := workflow.ExecuteActivity(ctx, "DecaySweepActivity", input).Get(ctx, &result); err != nil {
		logger.Error("DecaySweepActivity failed", "error", err)
		return fmt.Errorf("decay sweep activity: %w", err)
	}

	logger.Info("DecaySweepWorkflow complete",
		"items_processed", result.ItemsProcessed,
		"items_updated", result.ItemsUpdated,
		"items_marked_deleted", result.ItemsMarkedDeleted,
		"duration_ms", result.DurationMs,
	)
	return nil
}

// DecaySweepActivities holds the decay service for Temporal activity registration.
type DecaySweepActivities struct {
	DecaySvc *MemoryDecayService
}

// DecaySweepActivity is the Temporal activity that performs the actual decay computation.
func (a *DecaySweepActivities) DecaySweepActivity(ctx context.Context, input DecaySweepWorkflowInput) (DecaySweepResult, error) {
	start := time.Now()
	var result DecaySweepResult

	if a.DecaySvc == nil {
		return result, temporal.NewNonRetryableApplicationError(
			"DecaySweepActivity: decayService not available",
			"SERVICE_NOT_FOUND", nil,
		)
	}

	config := DecayConfig{
		HalfLifeDays:  30.0,
		MinRetention:  0.01,
		DecayFunction: "exponential",
		MinWeight:     0.01,
	}

	if input.WorkspaceID != "" {
		activity.RecordHeartbeat(ctx, fmt.Sprintf("sweeping workspace %s", input.WorkspaceID))
		count, err := a.DecaySvc.ApplyDecay(input.WorkspaceID, config)
		if err != nil {
			return result, fmt.Errorf("decay sweep: %w", err)
		}
		result.ItemsProcessed = count
		result.ItemsUpdated = count
	}

	result.DurationMs = time.Since(start).Milliseconds()
	return result, nil
}
