package temporal

import (
	"time"

	"go.temporal.io/sdk/temporal"
	"go.temporal.io/sdk/workflow"
)

// VoiceSessionWorkflowInput is the input for a voice session workflow.
type VoiceSessionWorkflowInput struct {
	SessionID   string `json:"session_id"`
	WorkspaceID string `json:"workspace_id"`
	UserID      string `json:"user_id"`
	ChannelType string `json:"channel_type"` // livekit, phone
}

// VoiceSessionWorkflowResult is the output of a voice session workflow.
type VoiceSessionWorkflowResult struct {
	SessionID      string   `json:"session_id"`
	TerminalState  string   `json:"terminal_state"`
	Duration       int64    `json:"duration_ms"`
	TasksExtracted []string `json:"tasks_extracted,omitempty"`
}

// VoiceSessionWorkflow orchestrates a real-time voice session lifecycle.
func VoiceSessionWorkflow(ctx workflow.Context, input VoiceSessionWorkflowInput) (*VoiceSessionWorkflowResult, error) {
	logger := workflow.GetLogger(ctx)
	logger.Info("VoiceSessionWorkflow started", "sessionID", input.SessionID)

	ao := workflow.ActivityOptions{
		StartToCloseTimeout: 300 * time.Second, // voice sessions can be long
		HeartbeatTimeout:    30 * time.Second,
		RetryPolicy: &temporal.RetryPolicy{
			InitialInterval:    time.Second,
			BackoffCoefficient: 2.0,
			MaximumInterval:    30 * time.Second,
			MaximumAttempts:    2,
		},
	}
	ctx = workflow.WithActivityOptions(ctx, ao)

	var a *Activities

	// Step 1: Initialize voice session (get token, connect to provider)
	var initResult VoiceInitResult
	err := workflow.ExecuteActivity(ctx, a.InitVoiceSessionActivity, VoiceInitInput{
		SessionID:   input.SessionID,
		WorkspaceID: input.WorkspaceID,
		UserID:      input.UserID,
		ChannelType: input.ChannelType,
	}).Get(ctx, &initResult)
	if err != nil {
		return &VoiceSessionWorkflowResult{
			SessionID:     input.SessionID,
			TerminalState: "INIT_FAILED",
		}, nil
	}

	// Step 2: Wait for session end signal
	endCh := workflow.GetSignalChannel(ctx, "voice_session_end")
	var endSignal VoiceEndSignal
	endCh.Receive(ctx, &endSignal)

	// Step 3: Extract tasks from transcript
	extractAO := workflow.ActivityOptions{
		StartToCloseTimeout: 30 * time.Second,
		RetryPolicy: &temporal.RetryPolicy{
			InitialInterval:    time.Second,
			BackoffCoefficient: 2.0,
			MaximumInterval:    15 * time.Second,
			MaximumAttempts:    3,
		},
	}
	ctx2 := workflow.WithActivityOptions(ctx, extractAO)
	var extractResult VoiceTaskExtractResult
	err = workflow.ExecuteActivity(ctx2, a.ExtractVoiceTasksActivity, VoiceTaskExtractInput{
		SessionID:   input.SessionID,
		WorkspaceID: input.WorkspaceID,
		Transcript:  endSignal.Transcript,
	}).Get(ctx, &extractResult)
	if err != nil {
		logger.Warn("task extraction failed", "error", err)
	}

	return &VoiceSessionWorkflowResult{
		SessionID:      input.SessionID,
		TerminalState:  "COMPLETED",
		Duration:       endSignal.DurationMs,
		TasksExtracted: extractResult.Tasks,
	}, nil
}

// Voice activity types

// VoiceInitInput is the input for InitVoiceSessionActivity.
type VoiceInitInput struct {
	SessionID   string `json:"session_id"`
	WorkspaceID string `json:"workspace_id"`
	UserID      string `json:"user_id"`
	ChannelType string `json:"channel_type"`
}

// VoiceInitResult is the result of InitVoiceSessionActivity.
type VoiceInitResult struct {
	Token    string `json:"token"`
	RoomName string `json:"room_name"`
}

// VoiceEndSignal is the signal payload for ending a voice session.
type VoiceEndSignal struct {
	Transcript string `json:"transcript"`
	DurationMs int64  `json:"duration_ms"`
}

// VoiceTaskExtractInput is the input for ExtractVoiceTasksActivity.
type VoiceTaskExtractInput struct {
	SessionID   string `json:"session_id"`
	WorkspaceID string `json:"workspace_id"`
	Transcript  string `json:"transcript"`
}

// VoiceTaskExtractResult is the result of ExtractVoiceTasksActivity.
type VoiceTaskExtractResult struct {
	Tasks []string `json:"tasks"`
}
