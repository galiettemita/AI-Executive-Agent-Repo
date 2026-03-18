package federated

import (
	"context"
	"fmt"
	"time"

	temporalclient "go.temporal.io/sdk/client"
	"go.temporal.io/sdk/temporal"
	"go.temporal.io/sdk/workflow"
)

const (
	FederatedCronWorkflowID = "brevio-federated-weekly"
	FederatedCronSchedule   = "0 2 * * 1" // Mondays 02:00 UTC
)

// FederatedFineTuningWorkflow orchestrates a federated round across eligible workspaces.
func FederatedFineTuningWorkflow(ctx workflow.Context) error {
	logger := workflow.GetLogger(ctx)
	logger.Info("FederatedFineTuningWorkflow started")

	actCtx := workflow.WithActivityOptions(ctx, workflow.ActivityOptions{
		StartToCloseTimeout: 30 * time.Minute,
		RetryPolicy:         &temporal.RetryPolicy{MaximumAttempts: 2},
	})

	var participantCount int
	if err := workflow.ExecuteActivity(actCtx, RunFederatedRoundActivity).Get(ctx, &participantCount); err != nil {
		return fmt.Errorf("federated round: %w", err)
	}

	logger.Info("FederatedFineTuningWorkflow complete", "participants", participantCount)
	return nil
}

// Activity stub for Temporal registration.
func RunFederatedRoundActivity(_ context.Context) (int, error) { return 0, nil }

// ScheduleFederatedCron registers the weekly federated cron.
func ScheduleFederatedCron(tc temporalclient.Client, taskQueue string) error {
	opts := temporalclient.StartWorkflowOptions{
		ID:           FederatedCronWorkflowID,
		TaskQueue:    taskQueue,
		CronSchedule: FederatedCronSchedule,
	}
	_, err := tc.ExecuteWorkflow(context.Background(), opts, FederatedFineTuningWorkflow)
	if err != nil {
		return fmt.Errorf("schedule federated cron: %w", err)
	}
	return nil
}
