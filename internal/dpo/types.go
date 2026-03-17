package dpo

import (
	"fmt"
	"time"

	"github.com/google/uuid"
)

// MinPairsForDPO is the minimum number of preference pairs required to trigger a DPO round.
const MinPairsForDPO = 50

// DPOBaseModel is the Anthropic model used as the fine-tuning base.
const DPOBaseModel = "claude-haiku-4-5-20251001"

// QualityRollbackThreshold: roll back checkpoint if quality drops by more than this.
const QualityRollbackThreshold = 0.02

// ErrDuplicatePair signals a duplicate insert attempt.
var ErrDuplicatePair = fmt.Errorf("dpo: duplicate preference pair for this workflow run")

// PreferencePair is one (prompt, chosen_response, rejected_response) triple.
type PreferencePair struct {
	ID                 uuid.UUID      `json:"id"`
	WorkspaceID        uuid.UUID      `json:"workspace_id"`
	UserID             uuid.UUID      `json:"user_id"`
	WorkflowRunID      string         `json:"workflow_run_id"`
	PromptHash         string         `json:"prompt_hash"`
	PromptText         string         `json:"prompt_text"`
	ChosenResponse     string         `json:"chosen_response"`
	RejectedResponse   string         `json:"rejected_response"`
	SignalType         string         `json:"signal_type"`
	CorrectionContext  map[string]any `json:"correction_context"`
	QualityScoreBefore *float64       `json:"quality_score_before,omitempty"`
	QualityScoreAfter  *float64       `json:"quality_score_after,omitempty"`
	UsedInRound        *int           `json:"used_in_round,omitempty"`
	CreatedAt          time.Time      `json:"created_at"`
}

// DPORound tracks a complete fine-tuning run.
type DPORound struct {
	ID                   uuid.UUID  `json:"id"`
	WorkspaceID          *uuid.UUID `json:"workspace_id,omitempty"`
	RoundNumber          int        `json:"round_number"`
	PairCount            int        `json:"pair_count"`
	BaseModel            string     `json:"base_model"`
	FineTuneJobID        *string    `json:"fine_tune_job_id,omitempty"`
	CheckpointID         *string    `json:"checkpoint_id,omitempty"`
	Status               string     `json:"status"`
	QualityScoreBaseline *float64   `json:"quality_score_baseline,omitempty"`
	QualityScoreAfter    *float64   `json:"quality_score_after,omitempty"`
	DeployedAt           *time.Time `json:"deployed_at,omitempty"`
	RolledBackAt         *time.Time `json:"rolled_back_at,omitempty"`
	ErrorMessage         *string    `json:"error_message,omitempty"`
	CreatedAt            time.Time  `json:"created_at"`
	UpdatedAt            time.Time  `json:"updated_at"`
}

// FeedbackIngestionInput is the Temporal activity input for capturing a correction event.
type FeedbackIngestionInput struct {
	WorkspaceID       string         `json:"workspace_id"`
	UserID            string         `json:"user_id"`
	WorkflowRunID     string         `json:"workflow_run_id"`
	PromptText        string         `json:"prompt_text"`
	OriginalResponse  string         `json:"original_response"`
	CorrectedResponse string         `json:"corrected_response"`
	SignalType        string         `json:"signal_type"`
	Context           map[string]any `json:"context,omitempty"`
}

// DPORoundInput is the Temporal workflow input.
type DPORoundInput struct {
	WorkspaceID  *string `json:"workspace_id,omitempty"`
	MinPairCount int     `json:"min_pair_count"`
}

// CheckpointDeployInput is the input for deploying a fine-tuned checkpoint.
type CheckpointDeployInput struct {
	WorkspaceID   string  `json:"workspace_id"`
	CheckpointID  string  `json:"checkpoint_id"`
	RoundNumber   int     `json:"round_number"`
	BaselineScore float64 `json:"baseline_score"`
}

// QualityDeltaInput is the input for the A/B quality monitor.
type QualityDeltaInput struct {
	WorkspaceID    string  `json:"workspace_id"`
	RoundNumber    int     `json:"round_number"`
	CheckpointID   string  `json:"checkpoint_id"`
	BaselineScore  float64 `json:"baseline_score"`
	EvalWindowDays int     `json:"eval_window_days"`
}
