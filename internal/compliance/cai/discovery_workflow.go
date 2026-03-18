package cai

import (
	"context"
	"fmt"
	"time"

	temporalclient "go.temporal.io/sdk/client"
	"go.temporal.io/sdk/temporal"
	"go.temporal.io/sdk/workflow"
)

const (
	CAIDiscoveryCronWorkflowID = "brevio-cai-discovery-weekly"
	CAIDiscoveryCronSchedule   = "0 6 * * 1" // Mondays 06:00 UTC
)

// ConstitutionalPrincipleDiscoveryWorkflow runs weekly principle discovery.
func ConstitutionalPrincipleDiscoveryWorkflow(ctx workflow.Context) error {
	logger := workflow.GetLogger(ctx)
	logger.Info("ConstitutionalPrincipleDiscoveryWorkflow started")

	actCtx := workflow.WithActivityOptions(ctx, workflow.ActivityOptions{
		StartToCloseTimeout: 30 * time.Minute,
		RetryPolicy:         &temporal.RetryPolicy{MaximumAttempts: 2},
	})

	var proposalCount int
	if err := workflow.ExecuteActivity(actCtx, RunDiscoveryActivity).Get(ctx, &proposalCount); err != nil {
		return fmt.Errorf("run discovery: %w", err)
	}

	if proposalCount > 0 {
		notifyCtx := workflow.WithActivityOptions(ctx, workflow.ActivityOptions{
			StartToCloseTimeout: 5 * time.Minute,
		})
		_ = workflow.ExecuteActivity(notifyCtx, NotifyAdminActivity, proposalCount).Get(ctx, nil)
	}

	logger.Info("ConstitutionalPrincipleDiscoveryWorkflow complete", "proposals", proposalCount)
	return nil
}

// Activity stubs for Temporal registration.
func RunDiscoveryActivity(_ context.Context) (int, error) { return 0, nil }
func NotifyAdminActivity(_ context.Context, _ int) error   { return nil }

// ScheduleCAIDiscoveryCron registers the weekly cron.
func ScheduleCAIDiscoveryCron(tc temporalclient.Client, taskQueue string) error {
	opts := temporalclient.StartWorkflowOptions{
		ID:           CAIDiscoveryCronWorkflowID,
		TaskQueue:    taskQueue,
		CronSchedule: CAIDiscoveryCronSchedule,
	}
	_, err := tc.ExecuteWorkflow(context.Background(), opts, ConstitutionalPrincipleDiscoveryWorkflow)
	if err != nil {
		return fmt.Errorf("schedule CAI discovery cron: %w", err)
	}
	return nil
}
