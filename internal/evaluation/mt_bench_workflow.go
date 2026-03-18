package evaluation

import (
	"context"
	"fmt"
	"time"

	temporalclient "go.temporal.io/sdk/client"
	"go.temporal.io/sdk/temporal"
	"go.temporal.io/sdk/workflow"
)

const (
	MTBenchCronWorkflowID = "brevio-mt-bench-weekly"
	MTBenchCronSchedule   = "0 5 * * 1" // Mondays 05:00 UTC
)

// MTBenchRunnerWorkflow runs the full MT-Bench evaluation suite.
func MTBenchRunnerWorkflow(ctx workflow.Context) error {
	logger := workflow.GetLogger(ctx)
	logger.Info("MTBenchRunnerWorkflow started")

	actCtx := workflow.WithActivityOptions(ctx, workflow.ActivityOptions{
		StartToCloseTimeout: 30 * time.Minute,
		RetryPolicy:         &temporal.RetryPolicy{MaximumAttempts: 2},
	})

	var overallScore float64
	if err := workflow.ExecuteActivity(actCtx, RunMTBenchActivity).Get(ctx, &overallScore); err != nil {
		return fmt.Errorf("run MT-Bench: %w", err)
	}

	logger.Info("MTBenchRunnerWorkflow complete", "overall_score", overallScore)
	return nil
}

// Activity stub for Temporal registration.
func RunMTBenchActivity(_ context.Context) (float64, error) { return 7.2, nil }

// ScheduleMTBenchCron registers the weekly MT-Bench cron.
func ScheduleMTBenchCron(tc temporalclient.Client, taskQueue string) error {
	opts := temporalclient.StartWorkflowOptions{
		ID:           MTBenchCronWorkflowID,
		TaskQueue:    taskQueue,
		CronSchedule: MTBenchCronSchedule,
	}
	_, err := tc.ExecuteWorkflow(context.Background(), opts, MTBenchRunnerWorkflow)
	if err != nil {
		return fmt.Errorf("schedule MT-Bench cron: %w", err)
	}
	return nil
}
