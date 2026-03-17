package dpo

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/google/uuid"
)

// dpoBaseModel returns the OpenAI base model for DPO fine-tuning.
// Reads DPO_BASE_MODEL from the environment at runtime.
// Default: gpt-4o-mini-2024-07-18
func dpoBaseModel() string {
	if m := os.Getenv("DPO_BASE_MODEL"); m != "" {
		return m
	}
	return "gpt-4o-mini-2024-07-18"
}

// Service orchestrates the DPO feedback ingestion and round management.
type Service struct {
	repo *Repository
	ftc  *FineTuneClient
}

func NewService(repo *Repository, ftc *FineTuneClient) *Service {
	return &Service{repo: repo, ftc: ftc}
}

// IngestFeedback validates and stores a preference pair.
func (s *Service) IngestFeedback(ctx context.Context, in FeedbackIngestionInput) (PreferencePair, error) {
	validNegative := map[string]bool{
		"undo": true, "edit": true, "retry": true,
		"skip": true, "explicit_thumbsdown": true,
	}
	if !validNegative[in.SignalType] {
		return PreferencePair{}, fmt.Errorf("dpo.Service.IngestFeedback: %q is not a correction signal", in.SignalType)
	}
	if in.OriginalResponse == in.CorrectedResponse {
		return PreferencePair{}, fmt.Errorf("dpo.Service.IngestFeedback: chosen and rejected are identical")
	}
	wsID, err := uuid.Parse(in.WorkspaceID)
	if err != nil {
		return PreferencePair{}, fmt.Errorf("dpo.Service.IngestFeedback: invalid workspace_id: %w", err)
	}
	uID, err := uuid.Parse(in.UserID)
	if err != nil {
		return PreferencePair{}, fmt.Errorf("dpo.Service.IngestFeedback: invalid user_id: %w", err)
	}
	if s.repo == nil {
		return PreferencePair{
			ID: uuid.New(), WorkspaceID: wsID, UserID: uID,
			WorkflowRunID: in.WorkflowRunID, PromptHash: HashPrompt(in.PromptText),
			PromptText: in.PromptText, ChosenResponse: in.CorrectedResponse,
			RejectedResponse: in.OriginalResponse, SignalType: in.SignalType,
			CreatedAt: time.Now().UTC(),
		}, nil
	}
	return s.repo.InsertPreferencePair(ctx, PreferencePair{
		WorkspaceID:       wsID,
		UserID:            uID,
		WorkflowRunID:     in.WorkflowRunID,
		PromptHash:        HashPrompt(in.PromptText),
		PromptText:        in.PromptText,
		ChosenResponse:    in.CorrectedResponse,
		RejectedResponse:  in.OriginalResponse,
		SignalType:        in.SignalType,
		CorrectionContext: in.Context,
	})
}

// PairsReadyForRound returns true when unused pair count >= MinPairsForDPO.
func (s *Service) PairsReadyForRound(ctx context.Context, workspaceID *uuid.UUID) (bool, int, error) {
	if s.repo == nil {
		return false, 0, nil
	}
	count, err := s.repo.CountUnusedPairs(ctx, workspaceID)
	if err != nil {
		return false, 0, err
	}
	return count >= MinPairsForDPO, count, nil
}

// StartDPORound fetches unused pairs, submits fine-tune job, creates round record.
func (s *Service) StartDPORound(ctx context.Context, workspaceID *uuid.UUID, baselineScore float64) (DPORound, error) {
	if s.repo == nil || s.ftc == nil {
		return DPORound{}, fmt.Errorf("dpo.Service.StartDPORound: repo or fine-tune client not configured")
	}
	roundNum, err := s.repo.NextRoundNumber(ctx, workspaceID)
	if err != nil {
		return DPORound{}, err
	}
	pairs, err := s.repo.FetchUnusedPairs(ctx, workspaceID, 500)
	if err != nil {
		return DPORound{}, err
	}
	if len(pairs) < MinPairsForDPO {
		return DPORound{}, fmt.Errorf("dpo.Service.StartDPORound: only %d pairs (need %d)", len(pairs), MinPairsForDPO)
	}

	jobID, err := s.ftc.SubmitDPOJob(ctx, dpoBaseModel(), pairs)
	if err != nil {
		return DPORound{}, fmt.Errorf("dpo.Service.StartDPORound: submit job: %w", err)
	}

	baseline := baselineScore
	round, err := s.repo.InsertDPORound(ctx, DPORound{
		WorkspaceID:          workspaceID,
		RoundNumber:          roundNum,
		PairCount:            len(pairs),
		BaseModel:            dpoBaseModel(),
		FineTuneJobID:        &jobID,
		Status:               "running",
		QualityScoreBaseline: &baseline,
	})
	if err != nil {
		return DPORound{}, err
	}

	pairIDs := make([]uuid.UUID, len(pairs))
	for i, p := range pairs {
		pairIDs[i] = p.ID
	}
	return round, s.repo.MarkPairsUsed(ctx, pairIDs, roundNum)
}

// PollUntilComplete polls fine-tune API until job succeeds/fails (6h timeout).
func (s *Service) PollUntilComplete(ctx context.Context, round DPORound) (string, error) {
	if round.FineTuneJobID == nil || s.ftc == nil {
		return "", fmt.Errorf("dpo.Service.PollUntilComplete: no job ID or client")
	}
	deadline := time.Now().Add(6 * time.Hour)
	for {
		if time.Now().After(deadline) {
			return "", fmt.Errorf("dpo.Service.PollUntilComplete: timed out after 6h")
		}
		select {
		case <-ctx.Done():
			return "", ctx.Err()
		case <-time.After(60 * time.Second):
		}
		status, err := s.ftc.PollJobStatus(ctx, *round.FineTuneJobID)
		if err != nil {
			continue
		}
		switch status.Status {
		case "succeeded":
			if status.CheckpointID == nil {
				return "", fmt.Errorf("dpo.Service.PollUntilComplete: succeeded but no checkpoint ID")
			}
			return *status.CheckpointID, nil
		case "failed":
			errMsg := "unknown"
			if status.Error != nil {
				errMsg = *status.Error
			}
			return "", fmt.Errorf("dpo.Service.PollUntilComplete: fine-tune failed: %s", errMsg)
		}
	}
}

func (s *Service) RecordRoundComplete(ctx context.Context, roundID uuid.UUID, checkpointID string) error {
	if s.repo == nil {
		return nil
	}
	return s.repo.UpdateDPORound(ctx, roundID, "completed", &checkpointID, nil, nil)
}

func (s *Service) RecordRoundFailed(ctx context.Context, roundID uuid.UUID, errMsg string) error {
	if s.repo == nil {
		return nil
	}
	return s.repo.UpdateDPORound(ctx, roundID, "failed", nil, nil, &errMsg)
}

func (s *Service) RecordQualityAfter(ctx context.Context, roundID uuid.UUID, qualityAfter float64) error {
	if s.repo == nil {
		return nil
	}
	return s.repo.UpdateDPORound(ctx, roundID, "completed", nil, &qualityAfter, nil)
}
