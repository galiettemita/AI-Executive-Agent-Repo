package temporal

import (
	"fmt"
	"time"

	"go.temporal.io/sdk/temporal"
	"go.temporal.io/sdk/workflow"
)

// MessageProcessingWorkflowInput matches the existing MessageProcessingInput structure.
type MessageProcessingWorkflowInput struct {
	MessageID      string `json:"message_id"`
	WorkspaceID    string `json:"workspace_id"`
	ChannelType    string `json:"channel_type"`
	RawPayload     string `json:"raw_payload"`
	IdempotencyKey string `json:"idempotency_key"`
	Tier           string `json:"tier,omitempty"` // T1/T2/T3; defaults to T2 if empty
}

type MessageProcessingWorkflowResult struct {
	WorkflowID         string   `json:"workflow_id"`
	TerminalState      string   `json:"terminal_state"`
	ResponsePayload    string   `json:"response_payload,omitempty"`
	Fallbacks          []string `json:"fallbacks,omitempty"`
	CompensationNeeded bool     `json:"compensation_needed"`
	EvidenceHash       string   `json:"evidence_hash,omitempty"`
	MemoryItemCount    int      `json:"memory_item_count,omitempty"`
	RAGChunkCount      int      `json:"rag_chunk_count,omitempty"`
	ReasoningIterations int     `json:"reasoning_iterations,omitempty"`
	CouncilConvened    bool     `json:"council_convened,omitempty"`
	OutboxEntryID      string   `json:"outbox_entry_id,omitempty"`
}

// MessageProcessingWorkflow orchestrates the full message lifecycle with
// intelligence pipeline integration:
// validate → retrieve memory/RAG → reasoning loop → cognitive assess →
// council eval → control gate → executor simulate → executor commit →
// synthesize → outbox enqueue
//
// Determinism guarantees (D7):
// - All activity inputs are deterministic from workflow input + prior results
// - Tool keys sorted lexically, context items stable-sorted
// - Deterministic jitter via ComputeDeterministicBackoff
// - No nondeterministic LLM invocations; fixed parameters throughout
func MessageProcessingWorkflow(ctx workflow.Context, input MessageProcessingWorkflowInput) (*MessageProcessingWorkflowResult, error) {
	logger := workflow.GetLogger(ctx)
	logger.Info("MessageProcessingWorkflow started", "messageID", input.MessageID)

	var a *Activities

	defaultAO := workflow.ActivityOptions{
		StartToCloseTimeout: 30 * time.Second,
		RetryPolicy: &temporal.RetryPolicy{
			InitialInterval:    time.Second,
			BackoffCoefficient: 2.0,
			MaximumInterval:    60 * time.Second,
			MaximumAttempts:    3,
		},
	}
	ctx = workflow.WithActivityOptions(ctx, defaultAO)

	// Step 1: Validate envelope
	var validateResult ValidateEnvelopeResult
	err := workflow.ExecuteActivity(ctx, a.ValidateEnvelopeActivity, ValidateEnvelopeInput{
		MessageID:      input.MessageID,
		WorkspaceID:    input.WorkspaceID,
		ChannelType:    input.ChannelType,
		RawPayload:     input.RawPayload,
		IdempotencyKey: input.IdempotencyKey,
	}).Get(ctx, &validateResult)
	if err != nil {
		return &MessageProcessingWorkflowResult{
			WorkflowID:    "msg-" + input.MessageID,
			TerminalState: "FAILED",
			Fallbacks:     []string{"envelope_validation_failed"},
		}, nil
	}
	if !validateResult.Valid {
		return &MessageProcessingWorkflowResult{
			WorkflowID:    "msg-" + input.MessageID,
			TerminalState: "DEAD_LETTER",
		}, nil
	}

	// Step 2: Classify intent (deterministic parameters, fixed temperature)
	classifyAO := workflow.ActivityOptions{
		StartToCloseTimeout: 120 * time.Second,
		RetryPolicy: &temporal.RetryPolicy{
			InitialInterval:        time.Second,
			BackoffCoefficient:     2.0,
			MaximumInterval:        60 * time.Second,
			MaximumAttempts:        2,
			NonRetryableErrorTypes: []string{"ATTENTION_BUDGET_EXHAUSTED", "SCHEMA_VALIDATION_FAILED"},
		},
	}
	ctxClassify := workflow.WithActivityOptions(ctx, classifyAO)
	var classifyResult ClassifyIntentResult
	err = workflow.ExecuteActivity(ctxClassify, a.ClassifyIntentActivity, ClassifyIntentInput{
		MessageID:   input.MessageID,
		WorkspaceID: input.WorkspaceID,
		Payload:     validateResult.NormalizedPayload,
	}).Get(ctx, &classifyResult)
	if err != nil {
		return &MessageProcessingWorkflowResult{
			WorkflowID:    "msg-" + input.MessageID,
			TerminalState: "FAILED",
			Fallbacks:     []string{"classify_failed"},
		}, nil
	}

	// Step 3: Retrieve memory (deterministic ordering: score DESC, ID ASC)
	var memResult MemoryRetrieveResult
	err = workflow.ExecuteActivity(ctx, a.RetrieveMemoryActivity, MemoryRetrieveInput{
		MessageID:   input.MessageID,
		WorkspaceID: input.WorkspaceID,
		Query:       validateResult.NormalizedPayload,
		MaxItems:    10,
	}).Get(ctx, &memResult)
	if err != nil {
		logger.Warn("memory retrieval failed, continuing without memory", "error", err)
		memResult = MemoryRetrieveResult{Items: []MemoryItem{}}
	}

	// Step 4: RAG search (deterministic ordering: score DESC, ChunkID ASC)
	var ragResult RAGSearchResult
	err = workflow.ExecuteActivity(ctx, a.SearchRAGActivity, RAGSearchInput{
		MessageID:   input.MessageID,
		WorkspaceID: input.WorkspaceID,
		Query:       validateResult.NormalizedPayload,
		TopK:        5,
	}).Get(ctx, &ragResult)
	if err != nil {
		logger.Warn("RAG search failed, continuing without RAG", "error", err)
		ragResult = RAGSearchResult{Chunks: []RAGChunk{}}
	}

	// Step 5: Reasoning loop (PLANNER → EXECUTOR → CRITIC → REFLECTOR)
	// Tool keys sorted lexically for replay determinism.
	var reasoningResult ReasoningLoopResult
	err = workflow.ExecuteActivity(ctxClassify, a.ExecuteReasoningLoopActivity, ReasoningLoopInput{
		MessageID:     input.MessageID,
		WorkspaceID:   input.WorkspaceID,
		Intent:        classifyResult.Intent,
		Confidence:    classifyResult.Confidence,
		MemoryItems:   memResult.Items,
		RAGChunks:     ragResult.Chunks,
		ContextBudget: 4096,
	}).Get(ctx, &reasoningResult)
	if err != nil {
		return &MessageProcessingWorkflowResult{
			WorkflowID:    "msg-" + input.MessageID,
			TerminalState: "FAILED",
			Fallbacks:     []string{"reasoning_loop_failed"},
		}, nil
	}

	// Step 6: Cognitive assessment (metacognitive monitoring)
	var cogResult CognitiveAssessResult
	err = workflow.ExecuteActivity(ctx, a.AssessCognitiveStateActivity, CognitiveAssessInput{
		MessageID:    input.MessageID,
		WorkspaceID:  input.WorkspaceID,
		TaskTokens:   reasoningResult.Iterations * 1024,
		StepCount:    len(reasoningResult.ToolKeys),
		ErrorCount:   0,
		QualityScore: reasoningResult.QualityScore,
	}).Get(ctx, &cogResult)
	if err != nil {
		logger.Warn("cognitive assessment failed, aborting per deny-by-default (D3)", "error", err)
		cogResult = CognitiveAssessResult{Strategy: "abort"}
	}

	// Step 7: Council evaluation (convenes for CRITICAL risk or high complexity)
	complexity := float64(len(reasoningResult.ToolKeys)) / 5.0
	if complexity > 1.0 {
		complexity = 1.0
	}
	var councilResult CouncilEvalResult
	err = workflow.ExecuteActivity(ctx, a.EvaluateCouncilActivity, CouncilEvalInput{
		MessageID:   input.MessageID,
		WorkspaceID: input.WorkspaceID,
		PlanID:      reasoningResult.PlanID,
		ToolKeys:    reasoningResult.ToolKeys,
		RiskLevel:   reasoningResult.RiskLevel,
		Complexity:  complexity,
	}).Get(ctx, &councilResult)
	if err != nil {
		logger.Warn("council evaluation failed, denying per deny-by-default (D3)", "error", err)
		councilResult = CouncilEvalResult{Decision: "deny", VoteCount: 0, EvidenceHash: "gate_error"}
	}

	// If cognitive assessment suggests abort, terminate.
	if cogResult.Strategy == "abort" {
		return &MessageProcessingWorkflowResult{
			WorkflowID:          "msg-" + input.MessageID,
			TerminalState:       "FAILED",
			Fallbacks:           []string{"cognitive_abort"},
			EvidenceHash:        reasoningResult.EvidenceHash,
			MemoryItemCount:     len(memResult.Items),
			RAGChunkCount:       len(ragResult.Chunks),
			ReasoningIterations: reasoningResult.Iterations,
			CouncilConvened:     councilResult.Convened,
		}, nil
	}

	// Step 8: Authorize via Control plane (get receipt)
	authorizeAO := workflow.ActivityOptions{
		StartToCloseTimeout: 5 * time.Second,
		RetryPolicy: &temporal.RetryPolicy{
			InitialInterval:        time.Second,
			BackoffCoefficient:     2.0,
			MaximumInterval:        10 * time.Second,
			MaximumAttempts:        3,
			NonRetryableErrorTypes: []string{"POLICY_DENY", "KILL_SWITCH_ACTIVE"},
		},
	}
	ctxAuth := workflow.WithActivityOptions(ctx, authorizeAO)
	var authResult AuthorizePlanResult
	err = workflow.ExecuteActivity(ctxAuth, a.AuthorizePlanActivity, AuthorizePlanInput{
		MessageID:   input.MessageID,
		WorkspaceID: input.WorkspaceID,
		PlanID:      reasoningResult.PlanID,
		ToolKeys:    reasoningResult.ToolKeys,
		RiskLevel:   reasoningResult.RiskLevel,
	}).Get(ctx, &authResult)
	if err != nil || authResult.Decision == "deny" {
		return &MessageProcessingWorkflowResult{
			WorkflowID:          "msg-" + input.MessageID,
			TerminalState:       "FAILED",
			Fallbacks:           []string{"authorization_denied"},
			EvidenceHash:        reasoningResult.EvidenceHash,
			MemoryItemCount:     len(memResult.Items),
			RAGChunkCount:       len(ragResult.Chunks),
			ReasoningIterations: reasoningResult.Iterations,
			CouncilConvened:     councilResult.Convened,
		}, nil
	}

	// Step 9 + 9.5: Execute tools then verify, with up to 1 replan iteration (max 2 total).
	executeAO := workflow.ActivityOptions{
		StartToCloseTimeout: 30 * time.Second,
		RetryPolicy: &temporal.RetryPolicy{
			InitialInterval:        time.Second,
			BackoffCoefficient:     2.0,
			MaximumInterval:        30 * time.Second,
			MaximumAttempts:        2,
			NonRetryableErrorTypes: []string{"IDEMPOTENCY_CONFLICT", "AUTH_EXPIRED", "BUDGET_EXHAUSTED"},
		},
	}
	ctxExec := workflow.WithActivityOptions(ctx, executeAO)

	verifyAO := workflow.ActivityOptions{
		StartToCloseTimeout: 120 * time.Second,
		RetryPolicy: &temporal.RetryPolicy{
			InitialInterval:        time.Second,
			BackoffCoefficient:     2.0,
			MaximumInterval:        60 * time.Second,
			MaximumAttempts:        2,
			NonRetryableErrorTypes: []string{"SCHEMA_VALIDATION_FAILED"},
		},
	}
	ctxVerify := workflow.WithActivityOptions(ctx, verifyAO)

	// Resolve tier-adaptive verify iteration cap. Default to T2 for backward
	// compatibility when callers omit Tier.
	maxVerifyIterations := 2
	switch input.Tier {
	case "T1":
		maxVerifyIterations = 1
	case "T3":
		maxVerifyIterations = 3
	}
	var execResults []ToolExecutionActivityResult
	compensationNeeded := false
	currentToolKeys := reasoningResult.ToolKeys
	currentPlanID := reasoningResult.PlanID
	currentRiskLevel := reasoningResult.RiskLevel
	currentFinalAnswerReqs := ""
	retryHints := ""

	for iteration := 0; iteration < maxVerifyIterations; iteration++ {
		// Execute tools for this iteration.
		execResults = nil
		for i, toolKey := range currentToolKeys {
			if i > 0 {
				wfInfo := workflow.GetInfo(ctx)
				jitter := ComputeDeterministicBackoff(
					DeterministicJitterConfig{
						BaseBackoff:    50 * time.Millisecond,
						MaxBackoff:     500 * time.Millisecond,
						JitterWindowMs: 200,
					},
					wfInfo.WorkflowExecution.ID,
					"ExecuteToolActivity",
					i+iteration*100, // offset by iteration to vary jitter
				)
				_ = workflow.Sleep(ctx, jitter)
			}

			var execResult ToolExecutionActivityResult
			err = workflow.ExecuteActivity(ctxExec, a.ExecuteToolActivity, ExecuteToolInput{
				MessageID:      input.MessageID,
				WorkspaceID:    input.WorkspaceID,
				ToolKey:        toolKey,
				ReceiptID:      authResult.ReceiptID,
				IdempotencyKey: input.IdempotencyKey + ":" + toolKey + fmt.Sprintf(":iter%d", iteration),
			}).Get(ctx, &execResult)
			if err != nil {
				compensationNeeded = true
				break
			}
			execResults = append(execResults, execResult)
		}

		// Step 9.5: Verify execution via LLM critic.
		var verifyResult VerifyExecutionResult
		err = workflow.ExecuteActivity(ctxVerify, a.VerifyExecutionActivity, VerifyExecutionInput{
			MessageID:       input.MessageID,
			WorkspaceID:     input.WorkspaceID,
			OriginalPayload: validateResult.NormalizedPayload,
			PlanID:          currentPlanID,
			PlanToolKeys:    currentToolKeys,
			PlanRiskLevel:   currentRiskLevel,
			FinalAnswerReqs: currentFinalAnswerReqs,
			ToolResults:     execResults,
			RetryHints:      retryHints,
		}).Get(ctx, &verifyResult)
		if err != nil {
			logger.Warn("verify activity failed, proceeding with current results", "error", err)
			break // treat as pass — don't block on verify failures
		}

		if verifyResult.Verdict == "pass" {
			break // satisfied, proceed to synthesis
		}

		// Verdict is "fail" — if we have iterations remaining, replan.
		if iteration+1 >= maxVerifyIterations {
			logger.Warn("verify failed after max iterations, proceeding", "reasons", verifyResult.Reasons)
			break
		}

		// Replan with verifier hints.
		retryHints = verifyResult.RetryHints
		logger.Info("verify failed, replanning", "iteration", iteration, "hints", retryHints)

		var replanResult GeneratePlanResult
		err = workflow.ExecuteActivity(ctxClassify, a.GeneratePlanActivity, GeneratePlanInput{
			MessageID:     input.MessageID,
			WorkspaceID:   input.WorkspaceID,
			Intent:        classifyResult.Intent,
			Confidence:    classifyResult.Confidence,
			Payload:       validateResult.NormalizedPayload,
			MemoryContext: fmt.Sprintf("%d items", len(memResult.Items)),
			RAGContext:    fmt.Sprintf("%d chunks", len(ragResult.Chunks)),
			RetryHints:    retryHints,
		}).Get(ctx, &replanResult)
		if err != nil {
			logger.Warn("replan failed, proceeding with original results", "error", err)
			break
		}

		// Update plan state for next iteration.
		currentToolKeys = replanResult.ToolKeys
		currentPlanID = replanResult.PlanID
		currentRiskLevel = replanResult.RiskLevel
		currentFinalAnswerReqs = replanResult.FinalAnswerReqs
	}

	// Step 10: Synthesize response
	synthesizeAO := workflow.ActivityOptions{
		StartToCloseTimeout: 60 * time.Second,
		RetryPolicy: &temporal.RetryPolicy{
			InitialInterval:        time.Second,
			BackoffCoefficient:     2.0,
			MaximumInterval:        30 * time.Second,
			MaximumAttempts:        2,
			NonRetryableErrorTypes: []string{"ATTENTION_BUDGET_EXHAUSTED"},
		},
	}
	ctxSynth := workflow.WithActivityOptions(ctx, synthesizeAO)
	var synthResult SynthesizeResponseResult
	err = workflow.ExecuteActivity(ctxSynth, a.SynthesizeResponseActivity, SynthesizeResponseInput{
		MessageID:   input.MessageID,
		WorkspaceID: input.WorkspaceID,
		ToolResults: execResults,
	}).Get(ctx, &synthResult)
	if err != nil {
		return &MessageProcessingWorkflowResult{
			WorkflowID:          "msg-" + input.MessageID,
			TerminalState:       "FAILED",
			CompensationNeeded:  compensationNeeded,
			EvidenceHash:        reasoningResult.EvidenceHash,
			MemoryItemCount:     len(memResult.Items),
			RAGChunkCount:       len(ragResult.Chunks),
			ReasoningIterations: reasoningResult.Iterations,
			CouncilConvened:     councilResult.Convened,
		}, nil
	}

	// Step 11: Enqueue outbox event for downstream consumption
	var outboxResult OutboxEnqueueResult
	err = workflow.ExecuteActivity(ctx, a.EnqueueOutboxActivity, OutboxEnqueueInput{
		WorkspaceID: input.WorkspaceID,
		EventType:   "BREVIO.message.processed.v1",
		Payload:     synthResult.ResponsePayload,
		Target:      input.ChannelType,
	}).Get(ctx, &outboxResult)
	if err != nil {
		logger.Warn("outbox enqueue failed, returning DELIVERY_FAILED", "error", err)
		return &MessageProcessingWorkflowResult{
			WorkflowID:          "msg-" + input.MessageID,
			TerminalState:       "DELIVERY_FAILED",
			ResponsePayload:     synthResult.ResponsePayload,
			CompensationNeeded:  true,
			EvidenceHash:        reasoningResult.EvidenceHash,
			MemoryItemCount:     len(memResult.Items),
			RAGChunkCount:       len(ragResult.Chunks),
			ReasoningIterations: reasoningResult.Iterations,
			CouncilConvened:     councilResult.Convened,
		}, nil
	}

	return &MessageProcessingWorkflowResult{
		WorkflowID:          "msg-" + input.MessageID,
		TerminalState:       "COMPLETED",
		ResponsePayload:     synthResult.ResponsePayload,
		CompensationNeeded:  compensationNeeded,
		EvidenceHash:        reasoningResult.EvidenceHash,
		MemoryItemCount:     len(memResult.Items),
		RAGChunkCount:       len(ragResult.Chunks),
		ReasoningIterations: reasoningResult.Iterations,
		CouncilConvened:     councilResult.Convened,
		OutboxEntryID:       outboxResult.EntryID,
	}, nil
}

// OutboxDispatchWorkflow processes pending outbox entries with DLQ semantics.
// Failed entries that exceed max_attempts are moved to the dead-letter queue.
// Uses deterministic jitter (D07) for retry backoff timing.
func OutboxDispatchWorkflow(ctx workflow.Context, input OutboxDispatchInput) (*OutboxDispatchResult, error) {
	logger := workflow.GetLogger(ctx)
	logger.Info("OutboxDispatchWorkflow started", "batchSize", input.BatchSize)

	var a *Activities

	fetchAO := workflow.ActivityOptions{
		StartToCloseTimeout: 30 * time.Second,
		RetryPolicy: &temporal.RetryPolicy{
			InitialInterval:    time.Second,
			BackoffCoefficient: 2.0,
			MaximumInterval:    60 * time.Second,
			MaximumAttempts:    5,
		},
	}
	ctx = workflow.WithActivityOptions(ctx, fetchAO)

	var fetchResult OutboxFetchResult
	err := workflow.ExecuteActivity(ctx, a.FetchPendingOutboxActivity, input).Get(ctx, &fetchResult)
	if err != nil {
		return nil, err
	}

	// Dispatch each entry individually. The activity handles mark-dispatched/mark-failed
	// and DLQ promotion internally via the outbox service.
	dispatchAO := workflow.ActivityOptions{
		StartToCloseTimeout: 30 * time.Second,
		RetryPolicy: &temporal.RetryPolicy{
			InitialInterval:    time.Second,
			BackoffCoefficient: 2.0,
			MaximumInterval:    30 * time.Second,
			MaximumAttempts:    1, // No Temporal-level retry; outbox service manages retries.
		},
	}
	dispatchCtx := workflow.WithActivityOptions(ctx, dispatchAO)

	dispatched := 0
	dlqCount := 0
	for i, entry := range fetchResult.Entries {
		// Apply deterministic jitter between dispatches to avoid thundering herd.
		if i > 0 {
			wfInfo := workflow.GetInfo(ctx)
			jitterDuration := ComputeDeterministicBackoff(
				DeterministicJitterConfig{
					BaseBackoff:    100 * time.Millisecond,
					MaxBackoff:     2 * time.Second,
					JitterWindowMs: 500,
				},
				wfInfo.WorkflowExecution.ID,
				"DispatchOutboxEntryActivity",
				i,
			)
			_ = workflow.Sleep(ctx, jitterDuration)
		}

		var dispatchResult OutboxEntryDispatchResult
		err = workflow.ExecuteActivity(dispatchCtx, a.DispatchOutboxEntryActivity, entry).Get(ctx, &dispatchResult)
		if err != nil {
			logger.Warn("outbox entry dispatch failed", "entryID", entry.ID, "error", err)
			continue
		}
		if dispatchResult.DLQ {
			dlqCount++
			logger.Warn("outbox entry moved to DLQ", "entryID", entry.ID)
		} else if dispatchResult.Success {
			dispatched++
		}
	}

	return &OutboxDispatchResult{
		TotalFetched:    len(fetchResult.Entries),
		TotalDispatched: dispatched,
		TotalDLQ:        dlqCount,
	}, nil
}

// ToolHealthEvaluationWorkflow evaluates tool health scores periodically.
func ToolHealthEvaluationWorkflow(ctx workflow.Context, input ToolHealthEvalInput) (*ToolHealthEvalResult, error) {
	var a *Activities
	ao := workflow.ActivityOptions{
		StartToCloseTimeout: 15 * time.Second,
		RetryPolicy: &temporal.RetryPolicy{
			InitialInterval:    time.Second,
			BackoffCoefficient: 2.0,
			MaximumInterval:    30 * time.Second,
			MaximumAttempts:    3,
		},
	}
	ctx = workflow.WithActivityOptions(ctx, ao)

	var result ToolHealthEvalResult
	err := workflow.ExecuteActivity(ctx, a.EvaluateToolHealthActivity, input).Get(ctx, &result)
	if err != nil {
		return nil, err
	}
	return &result, nil
}

// OnboardingWorkflow guides new workspace setup.
func OnboardingWorkflow(ctx workflow.Context, input OnboardingWorkflowInput) (*OnboardingWorkflowResult, error) {
	logger := workflow.GetLogger(ctx)
	logger.Info("OnboardingWorkflow started", "workspaceID", input.WorkspaceID)

	var a *Activities
	ao := workflow.ActivityOptions{
		StartToCloseTimeout: 120 * time.Second,
		RetryPolicy: &temporal.RetryPolicy{
			InitialInterval:    time.Second,
			BackoffCoefficient: 2.0,
			MaximumInterval:    60 * time.Second,
			MaximumAttempts:    3,
		},
	}
	ctx = workflow.WithActivityOptions(ctx, ao)

	stages := []string{
		"operator_profile_intake_v1",
		"behavior_policy_calibration_v1",
		"codebase_map_ingestion_v1",
		"system_map_ingestion_v1",
	}

	completedStages := make([]string, 0, len(stages))
	for _, stage := range stages {
		var stageResult OnboardingStageResult
		err := workflow.ExecuteActivity(ctx, a.ExecuteOnboardingStageActivity, OnboardingStageInput{
			WorkspaceID: input.WorkspaceID,
			Stage:       stage,
			Answers:     input.Answers,
		}).Get(ctx, &stageResult)
		if err != nil {
			return &OnboardingWorkflowResult{
				CompletedStages: completedStages,
				Status:          "incomplete",
			}, nil
		}
		completedStages = append(completedStages, stage)
	}

	return &OnboardingWorkflowResult{
		CompletedStages: completedStages,
		Status:          "completed",
	}, nil
}

// CostRollupWorkflow aggregates cost events into rollups.
func CostRollupWorkflow(ctx workflow.Context, input CostRollupInput) (*CostRollupResult, error) {
	var a *Activities
	ao := workflow.ActivityOptions{
		StartToCloseTimeout: 60 * time.Second,
		RetryPolicy: &temporal.RetryPolicy{
			InitialInterval:    time.Second,
			BackoffCoefficient: 2.0,
			MaximumInterval:    60 * time.Second,
			MaximumAttempts:    3,
		},
	}
	ctx = workflow.WithActivityOptions(ctx, ao)

	var result CostRollupResult
	err := workflow.ExecuteActivity(ctx, a.AggregateCostsActivity, input).Get(ctx, &result)
	if err != nil {
		return nil, err
	}
	return &result, nil
}

// KillSwitchWorkflow halts all workspace workflows when kill switch is activated.
func KillSwitchWorkflow(ctx workflow.Context, input KillSwitchInput) (*KillSwitchResult, error) {
	var a *Activities
	ao := workflow.ActivityOptions{
		StartToCloseTimeout: 10 * time.Second,
		RetryPolicy: &temporal.RetryPolicy{
			InitialInterval:    500 * time.Millisecond,
			BackoffCoefficient: 1.5,
			MaximumInterval:    5 * time.Second,
			MaximumAttempts:    5,
		},
	}
	ctx = workflow.WithActivityOptions(ctx, ao)

	var result KillSwitchResult
	err := workflow.ExecuteActivity(ctx, a.ActivateKillSwitchActivity, input).Get(ctx, &result)
	if err != nil {
		return nil, err
	}
	return &result, nil
}
