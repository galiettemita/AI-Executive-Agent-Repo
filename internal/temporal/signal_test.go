package temporal

import (
	"testing"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"
	"go.temporal.io/sdk/testsuite"
)

type SignalTestSuite struct {
	suite.Suite
	testsuite.WorkflowTestSuite
}

func TestSignalTestSuite(t *testing.T) {
	suite.Run(t, new(SignalTestSuite))
}

// Test 1: VoiceSession signal — end signal delivers transcript and triggers extraction.
func (s *SignalTestSuite) TestVoiceSession_EndSignal_TriggersExtraction() {
	env := s.NewTestWorkflowEnvironment()
	var a *Activities

	env.OnActivity(a.InitVoiceSessionActivity, mock.Anything, mock.Anything).Return(
		&VoiceInitResult{Token: "tok", RoomName: "voice-room"}, nil,
	)
	env.OnActivity(a.ExtractVoiceTasksActivity, mock.Anything, mock.Anything).Return(
		&VoiceTaskExtractResult{Tasks: []string{"call Alice"}}, nil,
	)
	env.OnActivity(a.AnalyseSentimentActivity, mock.Anything, mock.Anything).Return(
		&AnalyseSentimentResult{Summary: "neutral", OverallLabel: "neutral", OverallScore: 0.5}, nil,
	)

	env.RegisterDelayedCallback(func() {
		env.SignalWorkflow("voice_session_end", VoiceEndSignal{
			Transcript: "Please call Alice tomorrow.",
			DurationMs: 30000,
		})
	}, 0)

	env.ExecuteWorkflow(VoiceSessionWorkflow, VoiceSessionWorkflowInput{
		SessionID: "sess-sig-001", WorkspaceID: "ws-test", UserID: "u1", ChannelType: "livekit",
	})

	s.True(env.IsWorkflowCompleted())
	s.NoError(env.GetWorkflowError())

	var result VoiceSessionWorkflowResult
	s.NoError(env.GetWorkflowResult(&result))
	s.Equal("COMPLETED", result.TerminalState)
	s.Equal(int64(30000), result.Duration)
	s.Len(result.TasksExtracted, 1)
}

// Test 2: VoiceSession — init failure returns INIT_FAILED.
func (s *SignalTestSuite) TestVoiceSession_InitFailure_ReturnsInitFailed() {
	env := s.NewTestWorkflowEnvironment()
	var a *Activities

	env.OnActivity(a.InitVoiceSessionActivity, mock.Anything, mock.Anything).Return(
		nil, errFromString("livekit connect failed"),
	)

	env.ExecuteWorkflow(VoiceSessionWorkflow, VoiceSessionWorkflowInput{
		SessionID: "sess-fail", WorkspaceID: "ws-test", UserID: "u1", ChannelType: "livekit",
	})

	s.True(env.IsWorkflowCompleted())
	var result VoiceSessionWorkflowResult
	s.NoError(env.GetWorkflowResult(&result))
	s.Equal("INIT_FAILED", result.TerminalState)
}

// Test 3: VoiceSession — sentiment failure is non-fatal, workflow still completes.
func (s *SignalTestSuite) TestVoiceSession_SentimentFail_StillCompletes() {
	env := s.NewTestWorkflowEnvironment()
	var a *Activities

	env.OnActivity(a.InitVoiceSessionActivity, mock.Anything, mock.Anything).Return(
		&VoiceInitResult{Token: "tok", RoomName: "voice-room"}, nil,
	)
	env.OnActivity(a.ExtractVoiceTasksActivity, mock.Anything, mock.Anything).Return(
		&VoiceTaskExtractResult{Tasks: []string{}}, nil,
	)
	env.OnActivity(a.AnalyseSentimentActivity, mock.Anything, mock.Anything).Return(
		nil, errFromString("LLM unavailable"),
	)

	env.RegisterDelayedCallback(func() {
		env.SignalWorkflow("voice_session_end", VoiceEndSignal{
			Transcript: "Quick check-in.", DurationMs: 5000,
		})
	}, 0)

	env.ExecuteWorkflow(VoiceSessionWorkflow, VoiceSessionWorkflowInput{
		SessionID: "sess-sent-fail", WorkspaceID: "ws-test", UserID: "u1", ChannelType: "livekit",
	})

	s.True(env.IsWorkflowCompleted())
	s.NoError(env.GetWorkflowError())

	var result VoiceSessionWorkflowResult
	s.NoError(env.GetWorkflowResult(&result))
	s.Equal("COMPLETED", result.TerminalState)
}
