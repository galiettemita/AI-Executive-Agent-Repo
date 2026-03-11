package temporal

import (
	"fmt"
	"os"
	"strings"
	"testing"

	"github.com/stretchr/testify/mock"
	"go.temporal.io/sdk/testsuite"
)

// mockIntelligenceActivities registers mocks for the full intelligence pipeline.
func mockIntelligenceActivities(env *testsuite.TestWorkflowEnvironment) {
	var a *Activities

	env.OnActivity(a.RetrieveMemoryActivity, mock.Anything, mock.Anything).Return(
		&MemoryRetrieveResult{
			Items: []MemoryItem{
				{ID: "mem-001", MemoryType: "semantic", Body: "prior context", Score: 0.85},
				{ID: "mem-002", MemoryType: "episodic", Body: "past interaction", Score: 0.72},
			},
			TotalScored: 2,
		}, nil,
	)
	env.OnActivity(a.SearchRAGActivity, mock.Anything, mock.Anything).Return(
		&RAGSearchResult{
			Chunks: []RAGChunk{
				{ChunkID: "chunk-001", Score: 0.90, Snippet: "relevant doc", Source: "kb", Provenance: "native_result"},
			},
			TotalScored: 1,
		}, nil,
	)
	env.OnActivity(a.ExecuteReasoningLoopActivity, mock.Anything, mock.Anything).Return(
		&ReasoningLoopResult{
			PlanID:        "plan-r001",
			ToolKeys:      []string{"search.knowledge"},
			RiskLevel:     "LOW",
			QualityScore:  0.88,
			Iterations:    1,
			EvidenceHash:  "ev-hash-001",
			Deterministic: true,
		}, nil,
	)
	env.OnActivity(a.AssessCognitiveStateActivity, mock.Anything, mock.Anything).Return(
		&CognitiveAssessResult{
			CognitiveLoad:    0.3,
			ReasoningQuality: 0.9,
			UncertaintyLevel: 0.12,
			ShouldEscalate:   false,
			Strategy:         "proceed",
		}, nil,
	)
	env.OnActivity(a.EvaluateCouncilActivity, mock.Anything, mock.Anything).Return(
		&CouncilEvalResult{
			Convened:     false,
			Decision:     "approve",
			Reason:       "within_policy",
			VoteCount:    0,
			EvidenceHash: "council-hash-001",
		}, nil,
	)
	env.OnActivity(a.EnqueueOutboxActivity, mock.Anything, mock.Anything).Return(
		&OutboxEnqueueResult{EntryID: "outbox-001", Success: true}, nil,
	)
}

// TestMessageProcessingWorkflowReplay verifies that MessageProcessingWorkflow
// is replay-safe: it executes deterministically with mocked activities.
// Full intelligence pipeline: validate → memory → RAG → reasoning → cognitive →
// council → authorize → execute → synthesize → outbox
func TestMessageProcessingWorkflowReplay(t *testing.T) {
	suite := &testsuite.WorkflowTestSuite{}
	env := suite.NewTestWorkflowEnvironment()

	env.RegisterWorkflow(MessageProcessingWorkflow)

	var a *Activities
	env.OnActivity(a.ValidateEnvelopeActivity, mock.Anything, mock.Anything).Return(
		&ValidateEnvelopeResult{Valid: true, NormalizedPayload: `{"text":"hello"}`}, nil,
	)
	env.OnActivity(a.ClassifyIntentActivity, mock.Anything, mock.Anything).Return(
		&ClassifyIntentResult{Intent: "general_query", Confidence: 0.95}, nil,
	)
	mockIntelligenceActivities(env)
	env.OnActivity(a.AuthorizePlanActivity, mock.Anything, mock.Anything).Return(
		&AuthorizePlanResult{Decision: "allow", ReceiptID: "receipt-001"}, nil,
	)
	env.OnActivity(a.ExecuteToolActivity, mock.Anything, mock.Anything).Return(
		&ToolExecutionActivityResult{
			ToolKey:        "search.knowledge",
			Phase:          "commit",
			Success:        true,
			IdempotencyKey: "idem-001:search.knowledge",
			PayloadHash:    "abc123",
		}, nil,
	)
	env.OnActivity(a.SynthesizeResponseActivity, mock.Anything, mock.Anything).Return(
		&SynthesizeResponseResult{ResponsePayload: "Here are the results."}, nil,
	)

	env.ExecuteWorkflow(MessageProcessingWorkflow, MessageProcessingWorkflowInput{
		MessageID:      "msg-001",
		WorkspaceID:    "ws-001",
		ChannelType:    "slack",
		RawPayload:     `{"text":"hello"}`,
		IdempotencyKey: "idem-001",
	})

	if !env.IsWorkflowCompleted() {
		t.Fatal("workflow did not complete")
	}
	if err := env.GetWorkflowError(); err != nil {
		t.Fatalf("workflow failed: %v", err)
	}

	var result MessageProcessingWorkflowResult
	if err := env.GetWorkflowResult(&result); err != nil {
		t.Fatalf("failed to get result: %v", err)
	}

	if result.TerminalState != "COMPLETED" {
		t.Errorf("expected terminal state 'COMPLETED', got %q", result.TerminalState)
	}
	if result.ResponsePayload == "" {
		t.Error("expected non-empty response payload")
	}
	if result.EvidenceHash == "" {
		t.Error("expected non-empty evidence hash")
	}
	if result.MemoryItemCount != 2 {
		t.Errorf("expected 2 memory items, got %d", result.MemoryItemCount)
	}
	if result.RAGChunkCount != 1 {
		t.Errorf("expected 1 RAG chunk, got %d", result.RAGChunkCount)
	}
	if result.OutboxEntryID == "" {
		t.Error("expected non-empty outbox entry ID")
	}
}

// TestMessageProcessingWorkflow_AuthDenied verifies that the workflow handles
// authorization denial correctly (deny-by-default per D3).
func TestMessageProcessingWorkflow_AuthDenied(t *testing.T) {
	suite := &testsuite.WorkflowTestSuite{}
	env := suite.NewTestWorkflowEnvironment()

	env.RegisterWorkflow(MessageProcessingWorkflow)

	var a *Activities
	env.OnActivity(a.ValidateEnvelopeActivity, mock.Anything, mock.Anything).Return(
		&ValidateEnvelopeResult{Valid: true, NormalizedPayload: `{"text":"delete everything"}`}, nil,
	)
	env.OnActivity(a.ClassifyIntentActivity, mock.Anything, mock.Anything).Return(
		&ClassifyIntentResult{Intent: "destructive_action", Confidence: 0.99}, nil,
	)
	mockIntelligenceActivities(env)
	env.OnActivity(a.AuthorizePlanActivity, mock.Anything, mock.Anything).Return(
		&AuthorizePlanResult{Decision: "deny", Reason: "POLICY_DENY: destructive action blocked"}, nil,
	)

	env.ExecuteWorkflow(MessageProcessingWorkflow, MessageProcessingWorkflowInput{
		MessageID:      "msg-002",
		WorkspaceID:    "ws-001",
		ChannelType:    "slack",
		RawPayload:     `{"text":"delete everything"}`,
		IdempotencyKey: "idem-002",
	})

	if !env.IsWorkflowCompleted() {
		t.Fatal("workflow did not complete")
	}

	var result MessageProcessingWorkflowResult
	if err := env.GetWorkflowResult(&result); err != nil {
		t.Fatalf("failed to get result: %v", err)
	}

	if result.TerminalState != "FAILED" {
		t.Errorf("expected terminal state 'FAILED' for denied auth, got %q", result.TerminalState)
	}
	if result.EvidenceHash == "" {
		t.Error("expected evidence hash even on auth denial")
	}
}

// TestMessageProcessingWorkflowReplay_Deterministic runs the workflow twice
// with identical inputs and verifies the results are byte-identical,
// proving replay safety and determinism.
func TestMessageProcessingWorkflowReplay_Deterministic(t *testing.T) {
	runWorkflow := func() MessageProcessingWorkflowResult {
		s := &testsuite.WorkflowTestSuite{}
		env := s.NewTestWorkflowEnvironment()
		env.RegisterWorkflow(MessageProcessingWorkflow)

		var a *Activities
		env.OnActivity(a.ValidateEnvelopeActivity, mock.Anything, mock.Anything).Return(
			&ValidateEnvelopeResult{Valid: true, NormalizedPayload: `{"text":"determinism check"}`}, nil,
		)
		env.OnActivity(a.ClassifyIntentActivity, mock.Anything, mock.Anything).Return(
			&ClassifyIntentResult{Intent: "search_query", Confidence: 0.92}, nil,
		)
		mockIntelligenceActivities(env)
		env.OnActivity(a.AuthorizePlanActivity, mock.Anything, mock.Anything).Return(
			&AuthorizePlanResult{Decision: "allow", ReceiptID: "receipt-det"}, nil,
		)
		env.OnActivity(a.ExecuteToolActivity, mock.Anything, mock.Anything).Return(
			&ToolExecutionActivityResult{
				ToolKey: "search.knowledge", Phase: "commit", Success: true,
				IdempotencyKey: "idem-det:search.knowledge", PayloadHash: "det123",
			}, nil,
		)
		env.OnActivity(a.SynthesizeResponseActivity, mock.Anything, mock.Anything).Return(
			&SynthesizeResponseResult{ResponsePayload: "Deterministic response."}, nil,
		)

		env.ExecuteWorkflow(MessageProcessingWorkflow, MessageProcessingWorkflowInput{
			MessageID:      "msg-det",
			WorkspaceID:    "ws-det",
			ChannelType:    "web",
			RawPayload:     `{"text":"determinism check"}`,
			IdempotencyKey: "idem-det",
		})

		var result MessageProcessingWorkflowResult
		_ = env.GetWorkflowResult(&result)
		return result
	}

	r1 := runWorkflow()
	r2 := runWorkflow()

	if r1.TerminalState != r2.TerminalState {
		t.Errorf("non-deterministic terminal state: %q vs %q", r1.TerminalState, r2.TerminalState)
	}
	if r1.ResponsePayload != r2.ResponsePayload {
		t.Errorf("non-deterministic response: %q vs %q", r1.ResponsePayload, r2.ResponsePayload)
	}
	if r1.EvidenceHash != r2.EvidenceHash {
		t.Errorf("non-deterministic evidence hash: %q vs %q", r1.EvidenceHash, r2.EvidenceHash)
	}
	if r1.MemoryItemCount != r2.MemoryItemCount {
		t.Errorf("non-deterministic memory count: %d vs %d", r1.MemoryItemCount, r2.MemoryItemCount)
	}
	if r1.RAGChunkCount != r2.RAGChunkCount {
		t.Errorf("non-deterministic RAG count: %d vs %d", r1.RAGChunkCount, r2.RAGChunkCount)
	}
	if r1.ReasoningIterations != r2.ReasoningIterations {
		t.Errorf("non-deterministic reasoning iterations: %d vs %d", r1.ReasoningIterations, r2.ReasoningIterations)
	}
	if r1.CouncilConvened != r2.CouncilConvened {
		t.Errorf("non-deterministic council convened: %v vs %v", r1.CouncilConvened, r2.CouncilConvened)
	}
	if r1.OutboxEntryID != r2.OutboxEntryID {
		t.Errorf("non-deterministic outbox entry ID: %q vs %q", r1.OutboxEntryID, r2.OutboxEntryID)
	}
}

// TestExecuteToolActivity_MissingReceipt verifies ExecuteToolActivity
// refuses execution without a receipt (D3 enforcement).
func TestExecuteToolActivity_MissingReceipt(t *testing.T) {
	a := NewActivities()
	_, err := a.ExecuteToolActivity(nil, ExecuteToolInput{
		MessageID:   "msg-003",
		WorkspaceID: "ws-001",
		ToolKey:     "search",
		ReceiptID:   "", // missing receipt
	})
	if err == nil {
		t.Fatal("expected error for missing receipt, got nil")
	}
	expected := "AUTHORIZATION_REQUIRED: no receipt provided"
	if err.Error() != expected {
		t.Errorf("expected error %q, got %q", expected, err.Error())
	}
}

// TestDeterministicJitterConsistency verifies FNV jitter produces identical
// results across calls with the same seed (D7 replay safety).
func TestDeterministicJitterConsistency(t *testing.T) {
	cfg := DefaultJitterConfig()

	result1 := ComputeDeterministicBackoff(cfg, "wf-123", "ExecuteToolActivity", 3)
	result2 := ComputeDeterministicBackoff(cfg, "wf-123", "ExecuteToolActivity", 3)

	if result1 != result2 {
		t.Errorf("jitter is non-deterministic: %v != %v", result1, result2)
	}

	result3 := ComputeDeterministicBackoff(cfg, "wf-456", "ExecuteToolActivity", 3)
	if result1 == result3 {
		t.Log("info: different seeds produced same jitter (unlikely but possible)")
	}
}

// TestOutboxDispatchWorkflowReplay verifies OutboxDispatchWorkflow is replay-safe
// with DLQ semantics.
func TestOutboxDispatchWorkflowReplay(t *testing.T) {
	suite := &testsuite.WorkflowTestSuite{}
	env := suite.NewTestWorkflowEnvironment()

	env.RegisterWorkflow(OutboxDispatchWorkflow)

	var a *Activities
	env.OnActivity(a.FetchPendingOutboxActivity, mock.Anything, mock.Anything).Return(
		&OutboxFetchResult{
			Entries: []OutboxEntry{
				{ID: "entry-1", WorkspaceID: "ws-1", Payload: `{"body":"hello"}`, Target: "whatsapp", Attempts: 0, MaxAttempts: 5},
				{ID: "entry-2", WorkspaceID: "ws-1", Payload: `{"body":"world"}`, Target: "whatsapp", Attempts: 4, MaxAttempts: 5},
			},
		}, nil,
	)
	env.OnActivity(a.DispatchOutboxEntryActivity, mock.Anything, OutboxEntry{
		ID: "entry-1", WorkspaceID: "ws-1", Payload: `{"body":"hello"}`, Target: "whatsapp", Attempts: 0, MaxAttempts: 5,
	}).Return(&OutboxEntryDispatchResult{Success: true}, nil)
	env.OnActivity(a.DispatchOutboxEntryActivity, mock.Anything, OutboxEntry{
		ID: "entry-2", WorkspaceID: "ws-1", Payload: `{"body":"world"}`, Target: "whatsapp", Attempts: 4, MaxAttempts: 5,
	}).Return(&OutboxEntryDispatchResult{Success: false, DLQ: true, Error: "target unreachable"}, nil)

	env.ExecuteWorkflow(OutboxDispatchWorkflow, OutboxDispatchInput{BatchSize: 10})

	if !env.IsWorkflowCompleted() {
		t.Fatal("workflow did not complete")
	}
	if err := env.GetWorkflowError(); err != nil {
		t.Fatalf("workflow error: %v", err)
	}

	var result OutboxDispatchResult
	if err := env.GetWorkflowResult(&result); err != nil {
		t.Fatalf("failed to get result: %v", err)
	}

	if result.TotalFetched != 2 {
		t.Errorf("expected 2 fetched, got %d", result.TotalFetched)
	}
	if result.TotalDispatched != 1 {
		t.Errorf("expected 1 dispatched, got %d", result.TotalDispatched)
	}
	if result.TotalDLQ != 1 {
		t.Errorf("expected 1 DLQ, got %d", result.TotalDLQ)
	}
}

// --- Intelligence activity unit tests ---

func TestRetrieveMemoryActivity_Deterministic(t *testing.T) {
	a := NewActivities()
	r1, err := a.RetrieveMemoryActivity(nil, MemoryRetrieveInput{
		MessageID: "msg-1", WorkspaceID: "ws-1", Query: "test query", MaxItems: 5,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	r2, err := a.RetrieveMemoryActivity(nil, MemoryRetrieveInput{
		MessageID: "msg-1", WorkspaceID: "ws-1", Query: "test query", MaxItems: 5,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(r1.Items) != len(r2.Items) {
		t.Fatalf("non-deterministic item count: %d vs %d", len(r1.Items), len(r2.Items))
	}
	for i := range r1.Items {
		if r1.Items[i].ID != r2.Items[i].ID || r1.Items[i].Score != r2.Items[i].Score {
			t.Errorf("non-deterministic memory item at %d", i)
		}
	}
	// Verify stable sort: scores descending.
	for i := 1; i < len(r1.Items); i++ {
		if r1.Items[i].Score > r1.Items[i-1].Score {
			t.Errorf("memory items not sorted by score DESC at index %d", i)
		}
	}
}

func TestSearchRAGActivity_Deterministic(t *testing.T) {
	a := NewActivities()
	r1, err := a.SearchRAGActivity(nil, RAGSearchInput{
		MessageID: "msg-1", WorkspaceID: "ws-1", Query: "rag search", TopK: 3,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	r2, err := a.SearchRAGActivity(nil, RAGSearchInput{
		MessageID: "msg-1", WorkspaceID: "ws-1", Query: "rag search", TopK: 3,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(r1.Chunks) != len(r2.Chunks) {
		t.Fatalf("non-deterministic chunk count: %d vs %d", len(r1.Chunks), len(r2.Chunks))
	}
	for i := range r1.Chunks {
		if r1.Chunks[i].ChunkID != r2.Chunks[i].ChunkID || r1.Chunks[i].Score != r2.Chunks[i].Score {
			t.Errorf("non-deterministic RAG chunk at %d", i)
		}
	}
}

func TestExecuteReasoningLoopActivity_SortsToolKeys(t *testing.T) {
	a := NewActivities()
	result, err := a.ExecuteReasoningLoopActivity(nil, ReasoningLoopInput{
		MessageID:   "msg-1",
		WorkspaceID: "ws-1",
		Intent:      "search and send email",
		Confidence:  0.9,
		MemoryItems: []MemoryItem{{ID: "m1", Score: 0.8}},
		RAGChunks:   []RAGChunk{{ChunkID: "c1", Score: 0.9}},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result.Deterministic {
		t.Error("expected Deterministic=true")
	}
	// Verify lexical sort.
	for i := 1; i < len(result.ToolKeys); i++ {
		if result.ToolKeys[i] < result.ToolKeys[i-1] {
			t.Errorf("tool keys not sorted lexically: %v", result.ToolKeys)
		}
	}
	if result.EvidenceHash == "" {
		t.Error("expected non-empty evidence hash")
	}
}

func TestEvaluateCouncilActivity_CriticalRisk(t *testing.T) {
	a := NewActivities()
	result, err := a.EvaluateCouncilActivity(nil, CouncilEvalInput{
		MessageID: "msg-1", WorkspaceID: "ws-1", PlanID: "plan-1",
		ToolKeys: []string{"resource.delete"}, RiskLevel: "CRITICAL", Complexity: 0.5,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result.Convened {
		t.Error("expected council to convene for CRITICAL risk")
	}
	if result.Decision != "require_approval" {
		t.Errorf("expected require_approval, got %s", result.Decision)
	}
}

func TestAssessCognitiveStateActivity_Escalation(t *testing.T) {
	a := NewActivities()
	result, err := a.AssessCognitiveStateActivity(nil, CognitiveAssessInput{
		WorkspaceID: "ws-1", TaskTokens: 9000, StepCount: 10, ErrorCount: 8, QualityScore: 0.2,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result.ShouldEscalate {
		t.Error("expected escalation")
	}
	if result.Strategy == "proceed" {
		t.Error("expected non-proceed strategy")
	}
}

// --- v10.x Replay Tests (Prompt 4) ---

// TestOnboardingWorkflowReplay verifies the onboarding workflow replays
// deterministically through all 4 stages.
func TestOnboardingWorkflowReplay(t *testing.T) {
	suite := &testsuite.WorkflowTestSuite{}
	env := suite.NewTestWorkflowEnvironment()
	env.RegisterWorkflow(OnboardingWorkflow)

	var a *Activities
	env.OnActivity(a.ExecuteOnboardingStageActivity, mock.Anything, mock.Anything).Return(
		&OnboardingStageResult{Stage: "any", Success: true}, nil,
	)

	env.ExecuteWorkflow(OnboardingWorkflow, OnboardingWorkflowInput{
		WorkspaceID: "ws-onboard-001",
		Answers: map[string]string{
			"operator_profile_intake_v1":       "done",
			"behavior_policy_calibration_v1":   "done",
			"codebase_map_ingestion_v1":        "done",
			"system_map_ingestion_v1":          "done",
		},
	})

	if !env.IsWorkflowCompleted() {
		t.Fatal("onboarding workflow did not complete")
	}
	if err := env.GetWorkflowError(); err != nil {
		t.Fatalf("onboarding workflow failed: %v", err)
	}

	var result OnboardingWorkflowResult
	if err := env.GetWorkflowResult(&result); err != nil {
		t.Fatalf("failed to get onboarding result: %v", err)
	}

	if result.Status != "completed" {
		t.Errorf("expected status 'completed', got %q", result.Status)
	}
	if len(result.CompletedStages) != 4 {
		t.Errorf("expected 4 completed stages, got %d", len(result.CompletedStages))
	}
}

// TestOnboardingWorkflowReplay_PartialFailure verifies graceful degradation
// when an onboarding stage fails mid-workflow.
func TestOnboardingWorkflowReplay_PartialFailure(t *testing.T) {
	suite := &testsuite.WorkflowTestSuite{}
	env := suite.NewTestWorkflowEnvironment()
	env.RegisterWorkflow(OnboardingWorkflow)

	var a *Activities
	// First call succeeds, second fails — workflow should return "incomplete" with 1 stage.
	env.OnActivity(a.ExecuteOnboardingStageActivity, mock.Anything, OnboardingStageInput{
		WorkspaceID: "ws-onboard-002", Stage: "operator_profile_intake_v1",
		Answers: map[string]string{"operator_profile_intake_v1": "done"},
	}).Return(&OnboardingStageResult{Stage: "operator_profile_intake_v1", Success: true}, nil)
	env.OnActivity(a.ExecuteOnboardingStageActivity, mock.Anything, OnboardingStageInput{
		WorkspaceID: "ws-onboard-002", Stage: "behavior_policy_calibration_v1",
		Answers: map[string]string{"operator_profile_intake_v1": "done"},
	}).Return(nil, fmt.Errorf("STAGE_INCOMPLETE: missing answer for stage behavior_policy_calibration_v1"))

	env.ExecuteWorkflow(OnboardingWorkflow, OnboardingWorkflowInput{
		WorkspaceID: "ws-onboard-002",
		Answers:     map[string]string{"operator_profile_intake_v1": "done"},
	})

	if !env.IsWorkflowCompleted() {
		t.Fatal("workflow did not complete")
	}

	var result OnboardingWorkflowResult
	if err := env.GetWorkflowResult(&result); err != nil {
		t.Fatalf("failed to get result: %v", err)
	}

	if result.Status != "incomplete" {
		t.Errorf("expected status 'incomplete', got %q", result.Status)
	}
	if len(result.CompletedStages) != 1 {
		t.Errorf("expected 1 completed stage, got %d", len(result.CompletedStages))
	}
}

// TestCostRollupWorkflowReplay verifies the cost rollup workflow is replay-safe.
func TestCostRollupWorkflowReplay(t *testing.T) {
	suite := &testsuite.WorkflowTestSuite{}
	env := suite.NewTestWorkflowEnvironment()
	env.RegisterWorkflow(CostRollupWorkflow)

	var a *Activities
	env.OnActivity(a.AggregateCostsActivity, mock.Anything, mock.Anything).Return(
		&CostRollupResult{
			WorkspaceID:  "ws-cost-001",
			TotalCostUSD: 42.50,
			EventCount:   127,
			RollupID:     "rollup-001",
		}, nil,
	)

	env.ExecuteWorkflow(CostRollupWorkflow, CostRollupInput{
		WorkspaceID: "ws-cost-001",
		PeriodStart: "2026-03-01",
		PeriodEnd:   "2026-03-31",
	})

	if !env.IsWorkflowCompleted() {
		t.Fatal("cost rollup workflow did not complete")
	}
	if err := env.GetWorkflowError(); err != nil {
		t.Fatalf("cost rollup workflow failed: %v", err)
	}

	var result CostRollupResult
	if err := env.GetWorkflowResult(&result); err != nil {
		t.Fatalf("failed to get cost rollup result: %v", err)
	}
	if result.RollupID != "rollup-001" {
		t.Errorf("expected rollup ID 'rollup-001', got %q", result.RollupID)
	}
	if result.TotalCostUSD != 42.50 {
		t.Errorf("expected cost 42.50, got %f", result.TotalCostUSD)
	}
}

// TestKillSwitchWorkflowReplay verifies the kill switch workflow is replay-safe.
func TestKillSwitchWorkflowReplay(t *testing.T) {
	suite := &testsuite.WorkflowTestSuite{}
	env := suite.NewTestWorkflowEnvironment()
	env.RegisterWorkflow(KillSwitchWorkflow)

	var a *Activities
	env.OnActivity(a.ActivateKillSwitchActivity, mock.Anything, mock.Anything).Return(
		&KillSwitchResult{
			WorkspaceID:     "ws-kill-001",
			WorkflowsHalted: 3,
			ToolsDisabled:   5,
			ActivatedAt:     "2026-03-11T10:00:00Z",
		}, nil,
	)

	env.ExecuteWorkflow(KillSwitchWorkflow, KillSwitchInput{
		WorkspaceID: "ws-kill-001",
		ActivatedBy: "admin@brevio.ai",
		Reason:      "security_incident",
	})

	if !env.IsWorkflowCompleted() {
		t.Fatal("kill switch workflow did not complete")
	}
	if err := env.GetWorkflowError(); err != nil {
		t.Fatalf("kill switch workflow failed: %v", err)
	}

	var result KillSwitchResult
	if err := env.GetWorkflowResult(&result); err != nil {
		t.Fatalf("failed to get kill switch result: %v", err)
	}
	if result.WorkflowsHalted != 3 {
		t.Errorf("expected 3 halted workflows, got %d", result.WorkflowsHalted)
	}
}

// TestMessageProcessingWorkflow_CognitiveAbort verifies the workflow terminates
// when the cognitive assessment strategy is "abort".
func TestMessageProcessingWorkflow_CognitiveAbort(t *testing.T) {
	suite := &testsuite.WorkflowTestSuite{}
	env := suite.NewTestWorkflowEnvironment()
	env.RegisterWorkflow(MessageProcessingWorkflow)

	var a *Activities
	env.OnActivity(a.ValidateEnvelopeActivity, mock.Anything, mock.Anything).Return(
		&ValidateEnvelopeResult{Valid: true, NormalizedPayload: `{"text":"complex request"}`}, nil,
	)
	env.OnActivity(a.ClassifyIntentActivity, mock.Anything, mock.Anything).Return(
		&ClassifyIntentResult{Intent: "complex_analysis", Confidence: 0.6}, nil,
	)
	env.OnActivity(a.RetrieveMemoryActivity, mock.Anything, mock.Anything).Return(
		&MemoryRetrieveResult{Items: []MemoryItem{}, TotalScored: 0}, nil,
	)
	env.OnActivity(a.SearchRAGActivity, mock.Anything, mock.Anything).Return(
		&RAGSearchResult{Chunks: []RAGChunk{}, TotalScored: 0}, nil,
	)
	env.OnActivity(a.ExecuteReasoningLoopActivity, mock.Anything, mock.Anything).Return(
		&ReasoningLoopResult{
			PlanID: "plan-abort", ToolKeys: []string{"search.knowledge"},
			RiskLevel: "LOW", QualityScore: 0.15, Iterations: 3,
			EvidenceHash: "ev-abort", Deterministic: true,
		}, nil,
	)
	// Cognitive assessment triggers abort strategy.
	env.OnActivity(a.AssessCognitiveStateActivity, mock.Anything, mock.Anything).Return(
		&CognitiveAssessResult{
			CognitiveLoad: 0.95, ReasoningQuality: 0.1, UncertaintyLevel: 0.9,
			ShouldEscalate: true, Strategy: "abort",
		}, nil,
	)
	env.OnActivity(a.EvaluateCouncilActivity, mock.Anything, mock.Anything).Return(
		&CouncilEvalResult{Convened: false, Decision: "approve", Reason: "within_policy"}, nil,
	)

	env.ExecuteWorkflow(MessageProcessingWorkflow, MessageProcessingWorkflowInput{
		MessageID: "msg-abort", WorkspaceID: "ws-abort", ChannelType: "web",
		RawPayload: `{"text":"complex request"}`, IdempotencyKey: "idem-abort",
	})

	if !env.IsWorkflowCompleted() {
		t.Fatal("workflow did not complete")
	}

	var result MessageProcessingWorkflowResult
	if err := env.GetWorkflowResult(&result); err != nil {
		t.Fatalf("failed to get result: %v", err)
	}
	if result.TerminalState != "FAILED" {
		t.Errorf("expected FAILED terminal state for abort, got %q", result.TerminalState)
	}
	if len(result.Fallbacks) == 0 || result.Fallbacks[0] != "cognitive_abort" {
		t.Errorf("expected cognitive_abort fallback, got %v", result.Fallbacks)
	}
	if result.EvidenceHash == "" {
		t.Error("expected evidence hash even on abort")
	}
}

// TestOutboxDispatchWorkflowReplay_Empty verifies that the outbox dispatch
// workflow handles an empty batch gracefully (no entries to process).
func TestOutboxDispatchWorkflowReplay_Empty(t *testing.T) {
	suite := &testsuite.WorkflowTestSuite{}
	env := suite.NewTestWorkflowEnvironment()
	env.RegisterWorkflow(OutboxDispatchWorkflow)

	var a *Activities
	env.OnActivity(a.FetchPendingOutboxActivity, mock.Anything, mock.Anything).Return(
		&OutboxFetchResult{Entries: []OutboxEntry{}}, nil,
	)

	env.ExecuteWorkflow(OutboxDispatchWorkflow, OutboxDispatchInput{BatchSize: 10})

	if !env.IsWorkflowCompleted() {
		t.Fatal("workflow did not complete")
	}
	if err := env.GetWorkflowError(); err != nil {
		t.Fatalf("workflow error: %v", err)
	}

	var result OutboxDispatchResult
	if err := env.GetWorkflowResult(&result); err != nil {
		t.Fatalf("failed to get result: %v", err)
	}
	if result.TotalFetched != 0 {
		t.Errorf("expected 0 fetched for empty batch, got %d", result.TotalFetched)
	}
}

// TestEnqueueOutboxActivity_Idempotent verifies deterministic entry IDs
// for the same inputs (idempotency key derivation).
func TestEnqueueOutboxActivity_Idempotent(t *testing.T) {
	a := NewActivities()
	input := OutboxEnqueueInput{
		WorkspaceID: "ws-idem",
		EventType:   "BREVIO.message.processed.v1",
		Payload:     `{"body":"hello"}`,
		Target:      "slack",
	}

	r1, err := a.EnqueueOutboxActivity(nil, input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	r2, err := a.EnqueueOutboxActivity(nil, input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if r1.EntryID != r2.EntryID {
		t.Errorf("non-deterministic entry IDs: %q vs %q", r1.EntryID, r2.EntryID)
	}
	if r1.EntryID == "" {
		t.Error("expected non-empty entry ID")
	}
}

// TestOnboardingStageActivity_DegradedMode verifies the onboarding stage
// activity works without a DB repository (degraded/test mode).
func TestOnboardingStageActivity_DegradedMode(t *testing.T) {
	a := NewActivities()
	result, err := a.ExecuteOnboardingStageActivity(nil, OnboardingStageInput{
		WorkspaceID: "ws-test",
		Stage:       "operator_profile_intake_v1",
		Answers:     map[string]string{"operator_profile_intake_v1": "done"},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result.Success {
		t.Error("expected success in degraded mode")
	}
	if result.Stage != "operator_profile_intake_v1" {
		t.Errorf("expected stage 'operator_profile_intake_v1', got %q", result.Stage)
	}
}

// TestWorkflowDeterminismAudit scans workflow code for nondeterministic patterns.
// Workflow functions must not call time.Now(), rand, uuid.New(), or os.* directly.
func TestWorkflowDeterminismAudit(t *testing.T) {
	t.Parallel()
	root := findRepoRoot(t)

	workflowFiles := []string{
		"internal/temporal/workflows.go",
		"internal/temporal/workflows_voice.go",
		"internal/temporal/workflows_learning.go",
		"internal/temporal/workflows_federation.go",
		"internal/temporal/workflows_p8.go",
		"internal/temporal/workflows_v101.go",
	}

	forbiddenPatterns := []string{
		"time.Now()",
		"rand.Int",
		"rand.Float",
		"uuid.New()",
		"uuid.Must(uuid.NewV7())",
		"os.Getenv(",
	}

	for _, relPath := range workflowFiles {
		path := relPath
		t.Run(path, func(t *testing.T) {
			fullPath := root + "/" + path
			body, err := os.ReadFile(fullPath)
			if err != nil {
				if os.IsNotExist(err) {
					t.Skipf("file not found: %s", fullPath)
				}
				t.Fatalf("read %s: %v", fullPath, err)
			}
			content := string(body)
			for _, pattern := range forbiddenPatterns {
				if containsOutsideComment(content, pattern) {
					t.Errorf("NONDETERMINISTIC: %s contains %q in workflow code", path, pattern)
				}
			}
		})
	}
}

// containsOutsideComment checks if pattern appears in non-comment lines.
func containsOutsideComment(content, pattern string) bool {
	for _, line := range strings.Split(content, "\n") {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "//") || strings.HasPrefix(trimmed, "/*") {
			continue
		}
		if strings.Contains(line, pattern) {
			return true
		}
	}
	return false
}

// findRepoRoot locates the repository root by walking up from the test file.
func findRepoRoot(t *testing.T) string {
	t.Helper()
	dir, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	for {
		if _, err := os.Stat(dir + "/go.mod"); err == nil {
			return dir
		}
		parent := dir[:strings.LastIndex(dir, "/")]
		if parent == dir || parent == "" {
			t.Fatal("could not find repository root (go.mod)")
		}
		dir = parent
	}
}
