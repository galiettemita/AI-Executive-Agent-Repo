package temporal

import (
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
}

type MessageProcessingWorkflowResult struct {
	WorkflowID         string   `json:"workflow_id"`
	TerminalState      string   `json:"terminal_state"`
	ResponsePayload    string   `json:"response_payload,omitempty"`
	Fallbacks          []string `json:"fallbacks,omitempty"`
	CompensationNeeded bool     `json:"compensation_needed"`
}

// MessageProcessingWorkflow orchestrates the full message lifecycle:
// ingress -> validate -> plan -> authorize -> execute -> respond
func MessageProcessingWorkflow(ctx workflow.Context, input MessageProcessingWorkflowInput) (*MessageProcessingWorkflowResult, error) {
	logger := workflow.GetLogger(ctx)
	logger.Info("MessageProcessingWorkflow started", "messageID", input.MessageID)

	retryPolicy := &temporal.RetryPolicy{
		InitialInterval:    time.Second,
		BackoffCoefficient: 2.0,
		MaximumInterval:    60 * time.Second,
		MaximumAttempts:    3,
	}

	ao := workflow.ActivityOptions{
		StartToCloseTimeout: 30 * time.Second,
		RetryPolicy:         retryPolicy,
	}
	ctx = workflow.WithActivityOptions(ctx, ao)

	// Step 1: Validate envelope
	var validateResult ValidateEnvelopeResult
	err := workflow.ExecuteActivity(ctx, ValidateEnvelopeActivity, ValidateEnvelopeInput{
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

	// Step 2: Classify intent
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
	ctx2 := workflow.WithActivityOptions(ctx, classifyAO)
	var classifyResult ClassifyIntentResult
	err = workflow.ExecuteActivity(ctx2, ClassifyIntentActivity, ClassifyIntentInput{
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

	// Step 3: Generate plan
	var planResult GeneratePlanResult
	err = workflow.ExecuteActivity(ctx2, GeneratePlanActivity, GeneratePlanInput{
		MessageID:   input.MessageID,
		WorkspaceID: input.WorkspaceID,
		Intent:      classifyResult.Intent,
		Confidence:  classifyResult.Confidence,
	}).Get(ctx, &planResult)
	if err != nil {
		return &MessageProcessingWorkflowResult{
			WorkflowID:    "msg-" + input.MessageID,
			TerminalState: "FAILED",
			Fallbacks:     []string{"plan_generation_failed"},
		}, nil
	}

	// Step 4: Authorize via Control plane (get receipt)
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
	ctx3 := workflow.WithActivityOptions(ctx, authorizeAO)
	var authResult AuthorizePlanResult
	err = workflow.ExecuteActivity(ctx3, AuthorizePlanActivity, AuthorizePlanInput{
		MessageID:   input.MessageID,
		WorkspaceID: input.WorkspaceID,
		PlanID:      planResult.PlanID,
		ToolKeys:    planResult.ToolKeys,
		RiskLevel:   planResult.RiskLevel,
	}).Get(ctx, &authResult)
	if err != nil || authResult.Decision == "deny" {
		return &MessageProcessingWorkflowResult{
			WorkflowID:    "msg-" + input.MessageID,
			TerminalState: "FAILED",
			Fallbacks:     []string{"authorization_denied"},
		}, nil
	}

	// Step 5: Execute tools (simulate -> commit)
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
	ctx4 := workflow.WithActivityOptions(ctx, executeAO)

	var execResults []ToolExecutionActivityResult
	compensationNeeded := false
	for _, toolKey := range planResult.ToolKeys {
		var execResult ToolExecutionActivityResult
		err = workflow.ExecuteActivity(ctx4, ExecuteToolActivity, ExecuteToolInput{
			MessageID:      input.MessageID,
			WorkspaceID:    input.WorkspaceID,
			ToolKey:        toolKey,
			ReceiptID:      authResult.ReceiptID,
			IdempotencyKey: input.IdempotencyKey + ":" + toolKey,
		}).Get(ctx, &execResult)
		if err != nil {
			compensationNeeded = true
			break
		}
		execResults = append(execResults, execResult)
	}

	// Step 6: Synthesize response
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
	ctx5 := workflow.WithActivityOptions(ctx, synthesizeAO)
	var synthResult SynthesizeResponseResult
	err = workflow.ExecuteActivity(ctx5, SynthesizeResponseActivity, SynthesizeResponseInput{
		MessageID:   input.MessageID,
		WorkspaceID: input.WorkspaceID,
		ToolResults: execResults,
	}).Get(ctx, &synthResult)
	if err != nil {
		return &MessageProcessingWorkflowResult{
			WorkflowID:         "msg-" + input.MessageID,
			TerminalState:      "FAILED",
			CompensationNeeded: compensationNeeded,
		}, nil
	}

	return &MessageProcessingWorkflowResult{
		WorkflowID:         "msg-" + input.MessageID,
		TerminalState:      "COMPLETED",
		ResponsePayload:    synthResult.ResponsePayload,
		CompensationNeeded: compensationNeeded,
	}, nil
}

// OutboxDispatchWorkflow processes pending outbox entries.
func OutboxDispatchWorkflow(ctx workflow.Context, input OutboxDispatchInput) (*OutboxDispatchResult, error) {
	logger := workflow.GetLogger(ctx)
	logger.Info("OutboxDispatchWorkflow started", "batchSize", input.BatchSize)

	ao := workflow.ActivityOptions{
		StartToCloseTimeout: 30 * time.Second,
		RetryPolicy: &temporal.RetryPolicy{
			InitialInterval:    time.Second,
			BackoffCoefficient: 2.0,
			MaximumInterval:    60 * time.Second,
			MaximumAttempts:    5,
		},
	}
	ctx = workflow.WithActivityOptions(ctx, ao)

	var fetchResult OutboxFetchResult
	err := workflow.ExecuteActivity(ctx, FetchPendingOutboxActivity, input).Get(ctx, &fetchResult)
	if err != nil {
		return nil, err
	}

	dispatched := 0
	for _, entry := range fetchResult.Entries {
		var dispatchResult OutboxEntryDispatchResult
		err = workflow.ExecuteActivity(ctx, DispatchOutboxEntryActivity, entry).Get(ctx, &dispatchResult)
		if err != nil {
			logger.Warn("outbox entry dispatch failed", "entryID", entry.ID, "error", err)
			continue
		}
		dispatched++
	}

	return &OutboxDispatchResult{
		TotalFetched:    len(fetchResult.Entries),
		TotalDispatched: dispatched,
	}, nil
}

// ToolHealthEvaluationWorkflow evaluates tool health scores periodically.
func ToolHealthEvaluationWorkflow(ctx workflow.Context, input ToolHealthEvalInput) (*ToolHealthEvalResult, error) {
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
	err := workflow.ExecuteActivity(ctx, EvaluateToolHealthActivity, input).Get(ctx, &result)
	if err != nil {
		return nil, err
	}
	return &result, nil
}

// OnboardingWorkflow guides new workspace setup.
func OnboardingWorkflow(ctx workflow.Context, input OnboardingWorkflowInput) (*OnboardingWorkflowResult, error) {
	logger := workflow.GetLogger(ctx)
	logger.Info("OnboardingWorkflow started", "workspaceID", input.WorkspaceID)

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
		err := workflow.ExecuteActivity(ctx, ExecuteOnboardingStageActivity, OnboardingStageInput{
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
	err := workflow.ExecuteActivity(ctx, AggregateCostsActivity, input).Get(ctx, &result)
	if err != nil {
		return nil, err
	}
	return &result, nil
}

// KillSwitchWorkflow halts all workspace workflows when kill switch is activated.
func KillSwitchWorkflow(ctx workflow.Context, input KillSwitchInput) (*KillSwitchResult, error) {
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
	err := workflow.ExecuteActivity(ctx, ActivateKillSwitchActivity, input).Get(ctx, &result)
	if err != nil {
		return nil, err
	}
	return &result, nil
}
