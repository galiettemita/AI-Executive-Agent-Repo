package temporal

import (
	"context"
	"fmt"

	"go.temporal.io/sdk/activity"
	"go.temporal.io/sdk/temporal"

	"github.com/brevio/brevio/internal/dpo"
	"github.com/google/uuid"
)

// FeedbackIngestionActivity captures a user correction as a DPO preference pair.
func (a *Activities) FeedbackIngestionActivity(ctx context.Context, in dpo.FeedbackIngestionInput) (dpo.PreferencePair, error) {
	logger := activity.GetLogger(ctx)
	if a.dpoService == nil {
		logger.Warn("FeedbackIngestionActivity: dpoService not configured")
		return dpo.PreferencePair{}, nil
	}
	pair, err := a.dpoService.IngestFeedback(ctx, in)
	if err != nil {
		if err == dpo.ErrDuplicatePair {
			return dpo.PreferencePair{}, temporal.NewNonRetryableApplicationError(
				"duplicate preference pair", "DUPLICATE_PAIR", err)
		}
		return dpo.PreferencePair{}, fmt.Errorf("FeedbackIngestionActivity: %w", err)
	}
	logger.Info("DPO: preference pair stored", "pair_id", pair.ID, "signal", in.SignalType)
	return pair, nil
}

// DPODatasetReadyActivity checks whether enough pairs exist to trigger a DPO round.
func (a *Activities) DPODatasetReadyActivity(ctx context.Context, workspaceID string) (DPOReadinessResult, error) {
	if a.dpoService == nil {
		return DPOReadinessResult{}, nil
	}
	var wsIDPtr *uuid.UUID
	if workspaceID != "" {
		wsID, err := uuid.Parse(workspaceID)
		if err != nil {
			return DPOReadinessResult{}, temporal.NewNonRetryableApplicationError("invalid workspace_id", "INVALID_INPUT", err)
		}
		wsIDPtr = &wsID
	}
	ready, count, err := a.dpoService.PairsReadyForRound(ctx, wsIDPtr)
	if err != nil {
		return DPOReadinessResult{}, fmt.Errorf("DPODatasetReadyActivity: %w", err)
	}
	return DPOReadinessResult{Ready: ready, PairCount: count}, nil
}

// DPOReadinessResult is the output of DPODatasetReadyActivity.
type DPOReadinessResult struct {
	Ready     bool `json:"ready"`
	PairCount int  `json:"pair_count"`
}

// StartDPORoundActivity fetches unused pairs, submits fine-tune job, creates round record.
func (a *Activities) StartDPORoundActivity(ctx context.Context, in dpo.DPORoundInput) (dpo.DPORound, error) {
	if a.dpoService == nil {
		return dpo.DPORound{}, temporal.NewNonRetryableApplicationError("dpoService not configured", "NOT_CONFIGURED", nil)
	}

	baselineScore := 0.75
	if a.scoreStore != nil {
		if score, err := a.scoreStore.RollingPassRate(ctx, "", 7); err == nil {
			baselineScore = score
		}
	}

	var wsIDPtr *uuid.UUID
	if in.WorkspaceID != nil {
		wsID, err := uuid.Parse(*in.WorkspaceID)
		if err != nil {
			return dpo.DPORound{}, temporal.NewNonRetryableApplicationError("invalid workspace_id", "INVALID_INPUT", err)
		}
		wsIDPtr = &wsID
	}

	round, err := a.dpoService.StartDPORound(ctx, wsIDPtr, baselineScore)
	if err != nil {
		return dpo.DPORound{}, fmt.Errorf("StartDPORoundActivity: %w", err)
	}
	return round, nil
}

// PollDPOJobActivity polls the fine-tune API until job completes.
func (a *Activities) PollDPOJobActivity(ctx context.Context, round dpo.DPORound) (string, error) {
	if a.dpoService == nil {
		return "", temporal.NewNonRetryableApplicationError("dpoService not configured", "NOT_CONFIGURED", nil)
	}
	checkpointID, err := a.dpoService.PollUntilComplete(ctx, round)
	if err != nil {
		errMsg := err.Error()
		_ = a.dpoService.RecordRoundFailed(ctx, round.ID, errMsg)
		return "", fmt.Errorf("PollDPOJobActivity: %w", err)
	}
	_ = a.dpoService.RecordRoundComplete(ctx, round.ID, checkpointID)
	return checkpointID, nil
}

// CheckpointDeployActivity enables the fine-tuned checkpoint via feature flag.
func (a *Activities) CheckpointDeployActivity(ctx context.Context, in dpo.CheckpointDeployInput) error {
	if a.featureFlagService == nil {
		return nil
	}
	flagKey := fmt.Sprintf("dpo_checkpoint_%s", in.WorkspaceID)
	return a.featureFlagService.EnableForWorkspace(ctx, flagKey, in.WorkspaceID, map[string]any{
		"checkpoint_id":  in.CheckpointID,
		"round_number":   in.RoundNumber,
		"baseline_score": in.BaselineScore,
		"rollout_pct":    50,
	})
}

// QualityDeltaMonitorActivity measures quality score delta after the eval window.
func (a *Activities) QualityDeltaMonitorActivity(ctx context.Context, in dpo.QualityDeltaInput) (DPOQualityDeltaResult, error) {
	actualScore := in.BaselineScore
	if a.scoreStore != nil {
		if score, err := a.scoreStore.RollingPassRate(ctx, in.WorkspaceID, in.EvalWindowDays); err == nil {
			actualScore = score
		}
	}

	delta := actualScore - in.BaselineScore

	if delta < -dpo.QualityRollbackThreshold {
		if a.featureFlagService != nil {
			flagKey := fmt.Sprintf("dpo_checkpoint_%s", in.WorkspaceID)
			_ = a.featureFlagService.DisableForWorkspace(ctx, flagKey, in.WorkspaceID)
		}
		return DPOQualityDeltaResult{
			Delta: delta, RolledBack: true,
			Reason: fmt.Sprintf("quality degraded by %.3f (threshold %.3f)", -delta, dpo.QualityRollbackThreshold),
		}, nil
	}

	return DPOQualityDeltaResult{
		Delta:  delta,
		Reason: fmt.Sprintf("quality improved/unchanged by %.3f — checkpoint retained", delta),
	}, nil
}

// DPOQualityDeltaResult is the output of QualityDeltaMonitorActivity.
type DPOQualityDeltaResult struct {
	Delta      float64 `json:"delta"`
	RolledBack bool    `json:"rolled_back"`
	Reason     string  `json:"reason"`
}
