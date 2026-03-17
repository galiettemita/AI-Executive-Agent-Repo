package temporal

import (
	"fmt"
	"time"

	temporalSDK "go.temporal.io/sdk/temporal"
	"go.temporal.io/sdk/workflow"

	"github.com/brevio/brevio/internal/benchmark"
)

// GAIARunnerWorkflow orchestrates the weekly GAIA benchmark run.
func GAIARunnerWorkflow(ctx workflow.Context, in benchmark.GAIARunnerInput) error {
	logger := workflow.GetLogger(ctx)
	logger.Info("GAIARunnerWorkflow started", "triggered_by", in.TriggeredBy)

	baseAO := workflow.ActivityOptions{
		StartToCloseTimeout: 5 * time.Minute,
		RetryPolicy:         &temporalSDK.RetryPolicy{MaximumAttempts: 3, InitialInterval: 5 * time.Second, BackoffCoefficient: 2.0},
	}
	baseCtx := workflow.WithActivityOptions(ctx, baseAO)

	var run benchmark.BenchmarkRun
	if err := workflow.ExecuteActivity(baseCtx, "InitBenchmarkRunActivity", in).Get(ctx, &run); err != nil {
		return fmt.Errorf("GAIARunnerWorkflow: init run: %w", err)
	}

	taskAO := workflow.ActivityOptions{
		StartToCloseTimeout: 30 * time.Minute,
		HeartbeatTimeout:    2 * time.Minute,
		RetryPolicy:         &temporalSDK.RetryPolicy{MaximumAttempts: 2},
	}
	taskCtx := workflow.WithActivityOptions(ctx, taskAO)

	var results []benchmark.TaskResult
	if err := workflow.ExecuteActivity(taskCtx, "RunAllBenchmarkTasksActivity", in, run.ID).Get(ctx, &results); err != nil {
		return fmt.Errorf("GAIARunnerWorkflow: run tasks: %w", err)
	}

	if err := workflow.ExecuteActivity(baseCtx, "FinalizeBenchmarkRunActivity", BenchmarkFinalizeInput{
		RunID: run.ID, Results: results, StartedAt: run.StartedAt, PriorPassRate: run.PriorPassRate,
	}).Get(ctx, nil); err != nil {
		return fmt.Errorf("GAIARunnerWorkflow: finalize: %w", err)
	}

	logger.Info("GAIARunnerWorkflow complete", "run_id", run.ID, "tasks", len(results))
	return nil
}
