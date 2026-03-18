package redteam

import (
	"context"
	"fmt"
	"time"

	temporalclient "go.temporal.io/sdk/client"
	"go.temporal.io/sdk/temporal"
	"go.temporal.io/sdk/workflow"
)

const (
	// TaskQueueSecurity is the preferred task queue for red-team activities.
	TaskQueueSecurity = "security-worker"

	// RedTeamCronWorkflowID is the stable workflow ID for the weekly cron.
	RedTeamCronWorkflowID = "brevio-redteam-weekly-cron"

	// RedTeamCronSchedule runs every Sunday at 02:00 UTC.
	RedTeamCronSchedule = "0 2 * * 0"
)

// RedTeamWorkflow is the Temporal workflow that orchestrates the full
// red-team pipeline: GCG attacks, AutoDAN, HarmBench, persistence,
// and auto-hardening.
func RedTeamWorkflow(ctx workflow.Context) error {
	logger := workflow.GetLogger(ctx)
	logger.Info("RedTeamWorkflow started")

	// Activity 1: Run GCG attacks — 30min timeout, 2 retries.
	gcgCtx := workflow.WithActivityOptions(ctx, workflow.ActivityOptions{
		StartToCloseTimeout: 30 * time.Minute,
		RetryPolicy: &temporal.RetryPolicy{
			MaximumAttempts: 3,
		},
	})

	var gcgResults []AttackResult
	if err := workflow.ExecuteActivity(gcgCtx, RunGCGAttacksActivity).Get(ctx, &gcgResults); err != nil {
		logger.Error("GCG attacks failed", "error", err)
		return fmt.Errorf("GCG attacks: %w", err)
	}
	logger.Info("GCG attacks complete", "count", len(gcgResults))

	// Activity 2: Run AutoDAN — 45min timeout, 1 retry.
	autoDanCtx := workflow.WithActivityOptions(ctx, workflow.ActivityOptions{
		StartToCloseTimeout: 45 * time.Minute,
		RetryPolicy: &temporal.RetryPolicy{
			MaximumAttempts: 2,
		},
	})

	var autoDanResults []AttackResult
	if err := workflow.ExecuteActivity(autoDanCtx, RunAutoDanActivity).Get(ctx, &autoDanResults); err != nil {
		logger.Error("AutoDAN failed", "error", err)
		return fmt.Errorf("AutoDAN: %w", err)
	}
	logger.Info("AutoDAN complete", "count", len(autoDanResults))

	// Activity 3: Run HarmBench — 60min timeout, 2 retries.
	hbCtx := workflow.WithActivityOptions(ctx, workflow.ActivityOptions{
		StartToCloseTimeout: 60 * time.Minute,
		RetryPolicy: &temporal.RetryPolicy{
			MaximumAttempts: 3,
		},
	})

	var hbReport HarmBenchReport
	if err := workflow.ExecuteActivity(hbCtx, RunHarmBenchActivity).Get(ctx, &hbReport); err != nil {
		logger.Error("HarmBench failed", "error", err)
		return fmt.Errorf("HarmBench: %w", err)
	}
	logger.Info("HarmBench complete", "pass_rate", hbReport.OverallPassRate)

	// Assemble report.
	report := RedTeamReport{
		GCGResults:     gcgResults,
		AutoDANResults: autoDanResults,
		HarmBench:      &hbReport,
		RunAt:          workflow.Now(ctx),
	}

	// Activity 4: Persist report — 5min timeout.
	persistCtx := workflow.WithActivityOptions(ctx, workflow.ActivityOptions{
		StartToCloseTimeout: 5 * time.Minute,
		RetryPolicy: &temporal.RetryPolicy{
			MaximumAttempts: 3,
		},
	})

	if err := workflow.ExecuteActivity(persistCtx, PersistReportActivity, report).Get(ctx, nil); err != nil {
		logger.Error("PersistReport failed", "error", err)
		return fmt.Errorf("persist report: %w", err)
	}

	// Activity 5: Check auto-hardening — 5min timeout.
	hardenCtx := workflow.WithActivityOptions(ctx, workflow.ActivityOptions{
		StartToCloseTimeout: 5 * time.Minute,
		RetryPolicy: &temporal.RetryPolicy{
			MaximumAttempts: 2,
		},
	})

	if err := workflow.ExecuteActivity(hardenCtx, CheckAutoHardeningActivity, report).Get(ctx, nil); err != nil {
		logger.Error("AutoHardening failed", "error", err)
		return fmt.Errorf("auto-hardening: %w", err)
	}

	logger.Info("RedTeamWorkflow complete")
	return nil
}

// Activity functions — these are registered as Temporal activities and delegate
// to the RedTeamRunner instance. The runner is injected via the Activities struct.

// Activities holds the RedTeamRunner for activity method binding.
type Activities struct {
	Runner *RedTeamRunner
}

// RunGCGAttacksActivity executes GCG suffix attacks.
func (a *Activities) RunGCGAttacksActivity(ctx context.Context) ([]AttackResult, error) {
	return a.Runner.RunGCGAttacks(ctx)
}

// RunAutoDanActivity executes AutoDAN jailbreak generation.
func (a *Activities) RunAutoDanActivity(ctx context.Context) ([]AttackResult, error) {
	return a.Runner.RunAutoDAN(ctx)
}

// RunHarmBenchActivity executes the HarmBench evaluation.
func (a *Activities) RunHarmBenchActivity(ctx context.Context) (*HarmBenchReport, error) {
	return a.Runner.RunHarmBench(ctx)
}

// PersistReportActivity stores the full red-team report in the database.
func (a *Activities) PersistReportActivity(ctx context.Context, report RedTeamReport) error {
	return a.Runner.PersistReport(ctx, &report)
}

// CheckAutoHardeningActivity runs auto-hardening logic for any detected bypasses.
func (a *Activities) CheckAutoHardeningActivity(ctx context.Context, report RedTeamReport) error {
	return CheckAutoHardening(ctx, &report, a.Runner.db, a.Runner.logger)
}

// Standalone activity functions for workflow.ExecuteActivity references.
func RunGCGAttacksActivity(_ context.Context) ([]AttackResult, error)       { return nil, nil }
func RunAutoDanActivity(_ context.Context) ([]AttackResult, error)          { return nil, nil }
func RunHarmBenchActivity(_ context.Context) (*HarmBenchReport, error)      { return nil, nil }
func PersistReportActivity(_ context.Context, _ RedTeamReport) error        { return nil }
func CheckAutoHardeningActivity(_ context.Context, _ RedTeamReport) error   { return nil }

// ScheduleRedTeamCron registers the weekly red-team cron schedule with Temporal.
// It uses the security-worker task queue if available, otherwise falls back
// to the provided fallback queue.
func ScheduleRedTeamCron(tc temporalclient.Client, fallbackTaskQueue string) error {
	taskQueue := TaskQueueSecurity

	// Use fallback task queue (e.g., brain-worker) if the security-worker
	// queue is not explicitly configured.
	if fallbackTaskQueue != "" {
		taskQueue = fallbackTaskQueue
	}

	opts := temporalclient.StartWorkflowOptions{
		ID:           RedTeamCronWorkflowID,
		TaskQueue:    taskQueue,
		CronSchedule: RedTeamCronSchedule,
	}

	_, err := tc.ExecuteWorkflow(context.Background(), opts, RedTeamWorkflow)
	if err != nil {
		return fmt.Errorf("schedule red-team cron: %w", err)
	}
	return nil
}
