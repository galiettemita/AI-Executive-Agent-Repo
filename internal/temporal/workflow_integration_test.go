package temporal

import (
	"testing"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"
	"go.temporal.io/sdk/testsuite"
)

// WorkflowIntegrationSuite tests full workflow execution paths using Temporal's
// in-memory test environment. All activities are mocked — no real DB or LLM calls.
type WorkflowIntegrationSuite struct {
	suite.Suite
	testsuite.WorkflowTestSuite
}

func TestWorkflowIntegrationSuite(t *testing.T) {
	suite.Run(t, new(WorkflowIntegrationSuite))
}

// mockAllMessageActivities registers mock returns for the full MessageProcessingWorkflow pipeline.
func (s *WorkflowIntegrationSuite) mockAllMessageActivities(env *testsuite.TestWorkflowEnvironment) {
	var a *Activities

	env.OnActivity(a.ValidateEnvelopeActivity, mock.Anything, mock.Anything).Return(
		&ValidateEnvelopeResult{Valid: true, NormalizedPayload: "test payload"}, nil,
	)
	env.OnActivity(a.RetrieveMemoryActivity, mock.Anything, mock.Anything).Return(
		&MemoryRetrieveResult{Items: []MemoryItem{}}, nil,
	)
	env.OnActivity(a.SearchRAGActivity, mock.Anything, mock.Anything).Return(
		&RAGSearchResult{Chunks: []RAGChunk{}}, nil,
	)
	env.OnActivity(a.ClassifyIntentActivity, mock.Anything, mock.Anything).Return(
		&ClassifyIntentResult{Intent: "calendar.create_event", Confidence: 0.95}, nil,
	)
	env.OnActivity(a.ExecuteReasoningLoopActivity, mock.Anything, mock.Anything).Return(
		&ReasoningLoopResult{
			PlanID: "plan-001", ToolKeys: []string{"google_calendar.create_event"},
			RiskLevel: "LOW", QualityScore: 0.9, Iterations: 1, EvidenceHash: "hash-001",
		}, nil,
	)
	env.OnActivity(a.AssessCognitiveStateActivity, mock.Anything, mock.Anything).Return(
		&CognitiveAssessResult{Strategy: "proceed"}, nil,
	)
	env.OnActivity(a.EvaluateCouncilActivity, mock.Anything, mock.Anything).Return(
		&CouncilEvalResult{Decision: "allow", Convened: true, VoteCount: 3, EvidenceHash: "council-hash"}, nil,
	)
	env.OnActivity(a.AuthorizePlanActivity, mock.Anything, mock.Anything).Return(
		&AuthorizePlanResult{Decision: "allow", ReceiptID: "rec-001"}, nil,
	)
	env.OnActivity(a.ExecuteToolActivity, mock.Anything, mock.Anything).Return(
		&ToolExecutionActivityResult{Success: true, ToolOutput: `{"event_id":"evt-123"}`}, nil,
	)
	env.OnActivity(a.VerifyExecutionActivity, mock.Anything, mock.Anything).Return(
		&VerifyExecutionResult{Verdict: "pass"}, nil,
	)
	env.OnActivity(a.SynthesizeResponseActivity, mock.Anything, mock.Anything).Return(
		&SynthesizeResponseResult{ResponsePayload: "Meeting created."}, nil,
	)
	env.OnActivity(a.EnqueueOutboxActivity, mock.Anything, mock.Anything).Return(
		&OutboxEnqueueResult{EntryID: "outbox-001"}, nil,
	)
}

// Test 1: Full happy path — all activities succeed, workflow completes.
func (s *WorkflowIntegrationSuite) TestMessageProcessing_FullPipeline_Completes() {
	env := s.NewTestWorkflowEnvironment()
	s.mockAllMessageActivities(env)

	env.ExecuteWorkflow(MessageProcessingWorkflow, MessageProcessingWorkflowInput{
		MessageID:      "msg-int-001",
		WorkspaceID:    "ws-test",
		ChannelType:    "web",
		RawPayload:     "schedule a meeting",
		IdempotencyKey: "idem-int-001",
	})

	s.True(env.IsWorkflowCompleted())
	s.NoError(env.GetWorkflowError())

	var result MessageProcessingWorkflowResult
	s.NoError(env.GetWorkflowResult(&result))
	s.Equal("COMPLETED", result.TerminalState)
	s.NotEmpty(result.OutboxEntryID)
}

// Test 2: OPA deny — authorization returns deny, workflow returns authorization_denied.
func (s *WorkflowIntegrationSuite) TestMessageProcessing_OPADeny_ReturnsAuthDenied() {
	env := s.NewTestWorkflowEnvironment()
	var a *Activities

	env.OnActivity(a.ValidateEnvelopeActivity, mock.Anything, mock.Anything).Return(
		&ValidateEnvelopeResult{Valid: true, NormalizedPayload: "buy stock"}, nil,
	)
	env.OnActivity(a.RetrieveMemoryActivity, mock.Anything, mock.Anything).Return(&MemoryRetrieveResult{Items: []MemoryItem{}}, nil)
	env.OnActivity(a.SearchRAGActivity, mock.Anything, mock.Anything).Return(&RAGSearchResult{Chunks: []RAGChunk{}}, nil)
	env.OnActivity(a.ClassifyIntentActivity, mock.Anything, mock.Anything).Return(
		&ClassifyIntentResult{Intent: "ibkr.place_order", Confidence: 0.9}, nil,
	)
	env.OnActivity(a.ExecuteReasoningLoopActivity, mock.Anything, mock.Anything).Return(
		&ReasoningLoopResult{PlanID: "plan-deny", ToolKeys: []string{"ibkr.place_order"}, RiskLevel: "CRITICAL", Iterations: 1, EvidenceHash: "h"}, nil,
	)
	env.OnActivity(a.AssessCognitiveStateActivity, mock.Anything, mock.Anything).Return(&CognitiveAssessResult{Strategy: "proceed"}, nil)
	env.OnActivity(a.EvaluateCouncilActivity, mock.Anything, mock.Anything).Return(&CouncilEvalResult{Decision: "allow", Convened: true}, nil)
	env.OnActivity(a.AuthorizePlanActivity, mock.Anything, mock.Anything).Return(
		&AuthorizePlanResult{Decision: "deny", Reason: "AUTONOMY_DENY"}, nil,
	)

	env.ExecuteWorkflow(MessageProcessingWorkflow, MessageProcessingWorkflowInput{
		MessageID: "msg-deny-001", WorkspaceID: "ws-test", RawPayload: "buy stock",
	})

	s.True(env.IsWorkflowCompleted())
	var result MessageProcessingWorkflowResult
	s.NoError(env.GetWorkflowResult(&result))
	s.Equal("FAILED", result.TerminalState)
	s.Contains(result.Fallbacks, "authorization_denied")
}

// Test 3: Invalid envelope → DEAD_LETTER terminal state.
func (s *WorkflowIntegrationSuite) TestMessageProcessing_InvalidEnvelope_DeadLetters() {
	env := s.NewTestWorkflowEnvironment()
	var a *Activities

	env.OnActivity(a.ValidateEnvelopeActivity, mock.Anything, mock.Anything).Return(
		&ValidateEnvelopeResult{Valid: false, Reason: "malformed"}, nil,
	)

	env.ExecuteWorkflow(MessageProcessingWorkflow, MessageProcessingWorkflowInput{
		MessageID: "msg-bad", WorkspaceID: "ws-test", RawPayload: "",
	})

	s.True(env.IsWorkflowCompleted())
	var result MessageProcessingWorkflowResult
	s.NoError(env.GetWorkflowResult(&result))
	s.Equal("DEAD_LETTER", result.TerminalState)
}

// Test 4: Cognitive abort → FAILED with cognitive_abort fallback.
func (s *WorkflowIntegrationSuite) TestMessageProcessing_CognitiveAbort_Fails() {
	env := s.NewTestWorkflowEnvironment()
	var a *Activities

	env.OnActivity(a.ValidateEnvelopeActivity, mock.Anything, mock.Anything).Return(&ValidateEnvelopeResult{Valid: true, NormalizedPayload: "x"}, nil)
	env.OnActivity(a.RetrieveMemoryActivity, mock.Anything, mock.Anything).Return(&MemoryRetrieveResult{Items: []MemoryItem{}}, nil)
	env.OnActivity(a.SearchRAGActivity, mock.Anything, mock.Anything).Return(&RAGSearchResult{Chunks: []RAGChunk{}}, nil)
	env.OnActivity(a.ClassifyIntentActivity, mock.Anything, mock.Anything).Return(&ClassifyIntentResult{Intent: "test", Confidence: 0.5}, nil)
	env.OnActivity(a.ExecuteReasoningLoopActivity, mock.Anything, mock.Anything).Return(
		&ReasoningLoopResult{PlanID: "p", ToolKeys: []string{"t"}, RiskLevel: "LOW", Iterations: 1, EvidenceHash: "e"}, nil,
	)
	env.OnActivity(a.AssessCognitiveStateActivity, mock.Anything, mock.Anything).Return(
		&CognitiveAssessResult{Strategy: "abort"}, nil,
	)
	env.OnActivity(a.EvaluateCouncilActivity, mock.Anything, mock.Anything).Return(&CouncilEvalResult{Decision: "allow"}, nil)

	env.ExecuteWorkflow(MessageProcessingWorkflow, MessageProcessingWorkflowInput{
		MessageID: "msg-abort", WorkspaceID: "ws-test", RawPayload: "x",
	})

	s.True(env.IsWorkflowCompleted())
	var result MessageProcessingWorkflowResult
	s.NoError(env.GetWorkflowResult(&result))
	s.Equal("FAILED", result.TerminalState)
	s.Contains(result.Fallbacks, "cognitive_abort")
}

// Test 5: Classify failure → FAILED with classify_failed fallback.
func (s *WorkflowIntegrationSuite) TestMessageProcessing_ClassifyFails_ReturnsFailed() {
	env := s.NewTestWorkflowEnvironment()
	var a *Activities

	env.OnActivity(a.ValidateEnvelopeActivity, mock.Anything, mock.Anything).Return(
		&ValidateEnvelopeResult{Valid: true, NormalizedPayload: "test"}, nil,
	)
	env.OnActivity(a.RetrieveMemoryActivity, mock.Anything, mock.Anything).Return(&MemoryRetrieveResult{Items: []MemoryItem{}}, nil)
	env.OnActivity(a.SearchRAGActivity, mock.Anything, mock.Anything).Return(&RAGSearchResult{Chunks: []RAGChunk{}}, nil)
	env.OnActivity(a.ClassifyIntentActivity, mock.Anything, mock.Anything).Return(
		nil, errFromString("ATTENTION_BUDGET_EXHAUSTED"),
	)

	env.ExecuteWorkflow(MessageProcessingWorkflow, MessageProcessingWorkflowInput{
		MessageID: "msg-classify-fail", WorkspaceID: "ws-test", RawPayload: "test", IdempotencyKey: "ik",
	})

	s.True(env.IsWorkflowCompleted())
	var result MessageProcessingWorkflowResult
	s.NoError(env.GetWorkflowResult(&result))
	s.Equal("FAILED", result.TerminalState)
	s.Contains(result.Fallbacks, "classify_failed")
}

// Test 6: OnboardingWorkflow — all stages complete successfully.
func (s *WorkflowIntegrationSuite) TestOnboarding_AllStagesComplete() {
	env := s.NewTestWorkflowEnvironment()
	var a *Activities

	env.OnActivity(a.ExecuteOnboardingStageActivity, mock.Anything, mock.Anything).Return(
		&OnboardingStageResult{Stage: "stage", Success: true}, nil,
	)

	env.ExecuteWorkflow(OnboardingWorkflow, OnboardingWorkflowInput{
		WorkspaceID: "ws-new",
	})

	s.True(env.IsWorkflowCompleted())
	var result OnboardingWorkflowResult
	s.NoError(env.GetWorkflowResult(&result))
	s.Equal("completed", result.Status)
	s.Len(result.CompletedStages, 4) // 4 stages in OnboardingWorkflow
}

// Test 7: OnboardingWorkflow — stage failure returns incomplete.
func (s *WorkflowIntegrationSuite) TestOnboarding_StageFailure_Incomplete() {
	env := s.NewTestWorkflowEnvironment()
	var a *Activities

	// Second stage fails — returns error after retries exhausted → incomplete status.
	env.OnActivity(a.ExecuteOnboardingStageActivity, mock.Anything, mock.Anything).Return(
		nil, errFromString("stage failed"),
	)

	env.ExecuteWorkflow(OnboardingWorkflow, OnboardingWorkflowInput{
		WorkspaceID: "ws-partial",
	})

	s.True(env.IsWorkflowCompleted())
	var result OnboardingWorkflowResult
	s.NoError(env.GetWorkflowResult(&result))
	s.Equal("incomplete", result.Status)
}

// Test 8: CostRollupWorkflow — aggregation succeeds.
func (s *WorkflowIntegrationSuite) TestCostRollup_Succeeds() {
	env := s.NewTestWorkflowEnvironment()
	var a *Activities

	env.OnActivity(a.AggregateCostsActivity, mock.Anything, mock.Anything).Return(
		&CostRollupResult{WorkspaceID: "ws-cost", TotalCostUSD: 5.00, EventCount: 10}, nil,
	)

	env.ExecuteWorkflow(CostRollupWorkflow, CostRollupInput{
		WorkspaceID: "ws-cost",
	})

	s.True(env.IsWorkflowCompleted())
	s.NoError(env.GetWorkflowError())
}

// Test 9: KillSwitchWorkflow — activates successfully.
func (s *WorkflowIntegrationSuite) TestKillSwitch_Activates() {
	env := s.NewTestWorkflowEnvironment()
	var a *Activities

	env.OnActivity(a.ActivateKillSwitchActivity, mock.Anything, mock.Anything).Return(
		&KillSwitchResult{WorkspaceID: "ws-kill", WorkflowsHalted: 5}, nil,
	)

	env.ExecuteWorkflow(KillSwitchWorkflow, KillSwitchInput{
		WorkspaceID: "ws-kill", Reason: "security_incident", ActivatedBy: "admin-1",
	})

	s.True(env.IsWorkflowCompleted())
	s.NoError(env.GetWorkflowError())
}

// Test 10: VoiceSessionWorkflow — init + signal + extract + sentiment.
func (s *WorkflowIntegrationSuite) TestVoiceSession_FullLifecycle() {
	env := s.NewTestWorkflowEnvironment()
	var a *Activities

	env.OnActivity(a.InitVoiceSessionActivity, mock.Anything, mock.Anything).Return(
		&VoiceInitResult{Token: "livekit-token-unavailable-check-env", RoomName: "voice-abc"}, nil,
	)
	env.OnActivity(a.ExtractVoiceTasksActivity, mock.Anything, mock.Anything).Return(
		&VoiceTaskExtractResult{Tasks: []string{"call John", "schedule meeting"}}, nil,
	)
	env.OnActivity(a.AnalyseSentimentActivity, mock.Anything, mock.Anything).Return(
		&AnalyseSentimentResult{Summary: "positive", OverallLabel: "positive", OverallScore: 0.8}, nil,
	)

	// Signal the end of session after init completes.
	env.RegisterDelayedCallback(func() {
		env.SignalWorkflow("voice_session_end", VoiceEndSignal{
			Transcript: "Please call John tomorrow. Also schedule a team meeting.",
			DurationMs: 45000,
		})
	}, 0)

	env.ExecuteWorkflow(VoiceSessionWorkflow, VoiceSessionWorkflowInput{
		SessionID: "sess-001", WorkspaceID: "ws-test", UserID: "user-1", ChannelType: "livekit",
	})

	s.True(env.IsWorkflowCompleted())
	s.NoError(env.GetWorkflowError())

	var result VoiceSessionWorkflowResult
	s.NoError(env.GetWorkflowResult(&result))
	s.Equal("COMPLETED", result.TerminalState)
	s.Len(result.TasksExtracted, 2)
}

// errFromString creates a simple error for test mocking.
type errString string

func (e errString) Error() string { return string(e) }
func errFromString(s string) error { return errString(s) }
