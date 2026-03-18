package dpo

import (
	"context"
	"fmt"
	"time"

	temporalclient "go.temporal.io/sdk/client"
	"go.temporal.io/sdk/temporal"
	"go.temporal.io/sdk/workflow"
)

const (
	DPOBatchCronWorkflowID = "brevio-dpo-batch-weekly"
	DPOBatchCronSchedule   = "0 1 * * 1" // Mondays 01:00 UTC
)

// DPOBatchWorkflowInput carries batch processing parameters.
type DPOBatchWorkflowInput struct {
	TriggeredBy string `json:"triggered_by"` // "cron" or "admin"
}

// DPOBatchWorkflow processes queued DPO preference pairs weekly.
func DPOBatchWorkflow(ctx workflow.Context, input DPOBatchWorkflowInput) error {
	logger := workflow.GetLogger(ctx)
	logger.Info("DPOBatchWorkflow started", "triggered_by", input.TriggeredBy)

	actCtx := workflow.WithActivityOptions(ctx, workflow.ActivityOptions{
		StartToCloseTimeout: 30 * time.Minute,
		RetryPolicy: &temporal.RetryPolicy{
			MaximumAttempts: 2,
		},
	})

	// Activity: process all queued pairs.
	var processed int
	if err := workflow.ExecuteActivity(actCtx, ProcessQueuedPairsActivity, input).Get(ctx, &processed); err != nil {
		return fmt.Errorf("process queued pairs: %w", err)
	}

	logger.Info("DPOBatchWorkflow complete", "pairs_processed", processed)
	return nil
}

// Activity stub for Temporal registration.
func ProcessQueuedPairsActivity(_ context.Context, _ DPOBatchWorkflowInput) (int, error) {
	return 0, nil
}

// ScheduleDPOBatchCron registers the weekly DPO batch cron.
func ScheduleDPOBatchCron(tc temporalclient.Client, taskQueue string) error {
	opts := temporalclient.StartWorkflowOptions{
		ID:           DPOBatchCronWorkflowID,
		TaskQueue:    taskQueue,
		CronSchedule: DPOBatchCronSchedule,
	}

	_, err := tc.ExecuteWorkflow(context.Background(), opts,
		DPOBatchWorkflow, DPOBatchWorkflowInput{TriggeredBy: "cron"})
	if err != nil {
		return fmt.Errorf("schedule DPO batch cron: %w", err)
	}
	return nil
}
