package dpo_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/brevio/brevio/internal/dpo"
)

func TestHashPrompt_Deterministic(t *testing.T) {
	h1 := dpo.HashPrompt("schedule a meeting with Alice")
	h2 := dpo.HashPrompt("schedule a meeting with Alice")
	h3 := dpo.HashPrompt("different prompt")
	assert.Equal(t, h1, h2)
	assert.NotEqual(t, h1, h3)
	assert.Len(t, h1, 64)
}

func TestConstants(t *testing.T) {
	assert.Equal(t, 50, dpo.MinPairsForDPO)
	assert.Equal(t, "claude-haiku-4-5-20251001", dpo.DPOBaseModel)
	assert.Equal(t, 0.02, dpo.QualityRollbackThreshold)
}

func TestServiceIngestFeedback_RejectsPositiveSignal(t *testing.T) {
	svc := dpo.NewService(nil, nil)
	_, err := svc.IngestFeedback(context.Background(), dpo.FeedbackIngestionInput{
		WorkspaceID: "00000000-0000-0000-0000-000000000001",
		UserID:      "00000000-0000-0000-0000-000000000002",
		SignalType:  "click",
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "not a correction signal")
}

func TestServiceIngestFeedback_RejectsIdenticalResponses(t *testing.T) {
	svc := dpo.NewService(nil, nil)
	_, err := svc.IngestFeedback(context.Background(), dpo.FeedbackIngestionInput{
		WorkspaceID:       "00000000-0000-0000-0000-000000000001",
		UserID:            "00000000-0000-0000-0000-000000000002",
		SignalType:        "edit",
		OriginalResponse:  "same",
		CorrectedResponse: "same",
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "identical")
}

func TestServiceIngestFeedback_ValidCorrection(t *testing.T) {
	svc := dpo.NewService(nil, nil) // nil repo = returns in-memory pair
	pair, err := svc.IngestFeedback(context.Background(), dpo.FeedbackIngestionInput{
		WorkspaceID:       "00000000-0000-0000-0000-000000000001",
		UserID:            "00000000-0000-0000-0000-000000000002",
		WorkflowRunID:     "run-1",
		PromptText:        "schedule meeting",
		OriginalResponse:  "Meeting at 2pm",
		CorrectedResponse: "Meeting at 3pm",
		SignalType:        "edit",
	})
	require.NoError(t, err)
	assert.Equal(t, "Meeting at 3pm", pair.ChosenResponse)
	assert.Equal(t, "Meeting at 2pm", pair.RejectedResponse)
	assert.Equal(t, "edit", pair.SignalType)
	assert.NotEmpty(t, pair.PromptHash)
}

func TestFineTuneClientConstructor_MissingAPIKey(t *testing.T) {
	t.Setenv("ANTHROPIC_API_KEY", "")
	_, err := dpo.NewFineTuneClient()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "ANTHROPIC_API_KEY")
}

func TestQualityRollbackLogic(t *testing.T) {
	cases := []struct {
		baseline float64
		actual   float64
		rollback bool
	}{
		{0.80, 0.77, true},
		{0.80, 0.79, false},
		{0.80, 0.85, false},
	}
	for _, tc := range cases {
		delta := tc.actual - tc.baseline
		shouldRollback := delta < -dpo.QualityRollbackThreshold
		assert.Equal(t, tc.rollback, shouldRollback,
			"baseline=%.2f actual=%.2f delta=%.3f", tc.baseline, tc.actual, delta)
	}
}

func TestServicePairsReadyForRound_NilRepo(t *testing.T) {
	svc := dpo.NewService(nil, nil)
	ready, count, err := svc.PairsReadyForRound(context.Background(), nil)
	require.NoError(t, err)
	assert.False(t, ready)
	assert.Equal(t, 0, count)
}
