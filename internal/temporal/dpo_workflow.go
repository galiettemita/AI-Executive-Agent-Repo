package temporal

import (
	"fmt"
	"time"

	temporalSDK "go.temporal.io/sdk/temporal"
	"go.temporal.io/sdk/workflow"

	"github.com/brevio/brevio/internal/dpo"
)

// DPORoundWorkflow runs the full DPO fine-tuning pipeline.
// Scheduled nightly at 03:00 UTC via "dpo-nightly-cron".
func DPORoundWorkflow(ctx workflow.Context, in dpo.DPORoundInput) error {
	logger := workflow.GetLogger(ctx)
	if in.MinPairCount == 0 {
		in.MinPairCount = dpo.MinPairsForDPO
	}

	baseCtx := workflow.WithActivityOptions(ctx, workflow.ActivityOptions{
		StartToCloseTimeout: 2 * time.Minute,
		RetryPolicy:         &temporalSDK.RetryPolicy{MaximumAttempts: 3, InitialInterval: 5 * time.Second, BackoffCoefficient: 2.0},
	})

	wsID := ""
	if in.WorkspaceID != nil {
		wsID = *in.WorkspaceID
	}

	// Step 1: Dataset readiness check
	var readiness DPOReadinessResult
	if err := workflow.ExecuteActivity(baseCtx, "DPODatasetReadyActivity", wsID).Get(ctx, &readiness); err != nil {
		return fmt.Errorf("DPORoundWorkflow: dataset check: %w", err)
	}
	if !readiness.Ready {
		logger.Info("DPORoundWorkflow: not enough pairs, skipping", "count", readiness.PairCount)
		return nil
	}

	// Step 2: Start DPO round
	var round dpo.DPORound
	if err := workflow.ExecuteActivity(baseCtx, "StartDPORoundActivity", in).Get(ctx, &round); err != nil {
		return fmt.Errorf("DPORoundWorkflow: start round: %w", err)
	}

	// Step 3: Poll until fine-tune completes (8h timeout)
	pollCtx := workflow.WithActivityOptions(ctx, workflow.ActivityOptions{
		StartToCloseTimeout: 8 * time.Hour,
		HeartbeatTimeout:    5 * time.Minute,
		RetryPolicy:         &temporalSDK.RetryPolicy{MaximumAttempts: 1},
	})
	var checkpointID string
	if err := workflow.ExecuteActivity(pollCtx, "PollDPOJobActivity", round).Get(ctx, &checkpointID); err != nil {
		return fmt.Errorf("DPORoundWorkflow: poll job: %w", err)
	}

	// Step 4: Deploy checkpoint
	baseline := 0.75
	if round.QualityScoreBaseline != nil {
		baseline = *round.QualityScoreBaseline
	}
	deployIn := dpo.CheckpointDeployInput{
		WorkspaceID: wsID, CheckpointID: checkpointID,
		RoundNumber: round.RoundNumber, BaselineScore: baseline,
	}
	if err := workflow.ExecuteActivity(baseCtx, "CheckpointDeployActivity", deployIn).Get(ctx, nil); err != nil {
		return fmt.Errorf("DPORoundWorkflow: deploy checkpoint: %w", err)
	}

	// Step 5: Wait 7 days then measure quality delta
	if err := workflow.Sleep(ctx, 7*24*time.Hour); err != nil {
		return fmt.Errorf("DPORoundWorkflow: sleep: %w", err)
	}

	monitorIn := dpo.QualityDeltaInput{
		WorkspaceID: wsID, RoundNumber: round.RoundNumber,
		CheckpointID: checkpointID, BaselineScore: baseline, EvalWindowDays: 7,
	}
	var monitorResult DPOQualityDeltaResult
	if err := workflow.ExecuteActivity(baseCtx, "QualityDeltaMonitorActivity", monitorIn).Get(ctx, &monitorResult); err != nil {
		return fmt.Errorf("DPORoundWorkflow: quality monitor: %w", err)
	}

	logger.Info("DPORoundWorkflow complete",
		"delta", monitorResult.Delta, "rolled_back", monitorResult.RolledBack, "reason", monitorResult.Reason)
	return nil
}
