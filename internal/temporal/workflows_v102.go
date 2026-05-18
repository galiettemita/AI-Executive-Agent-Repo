package temporal

import (
	"time"

	"go.temporal.io/sdk/temporal"
	"go.temporal.io/sdk/workflow"
)

// IntelligenceProcessingWorkflow orchestrates the v10.2 intelligence pipeline:
// 1. Classify multi-intent
// 2. Assess uncertainty (linked to calibration)
// 3. Apply EQ strategy
// 4. Run reasoning loop with critic/reflector persistence
// 5. Evaluate interruptions
func IntelligenceProcessingWorkflow(ctx workflow.Context, input struct {
	WorkspaceID   string  `json:"workspace_id"`
	UserID        string  `json:"user_id"`
	SessionID     string  `json:"session_id"`
	IngressTurnID string  `json:"ingress_turn_id"`
	RawInput      string  `json:"raw_input"`
	DetectedState string  `json:"detected_state"`
	CommStyle     string  `json:"comm_style"`
	Intent        string  `json:"intent"`
	RawConfidence float64 `json:"raw_confidence"`
}) error {
	ao := workflow.ActivityOptions{
		StartToCloseTimeout: 30 * time.Second,
		RetryPolicy: &temporal.RetryPolicy{
			MaximumAttempts: 2,
		},
	}
	ctx = workflow.WithActivityOptions(ctx, ao)

	workflowRunID := workflow.GetInfo(ctx).WorkflowExecution.RunID

	// Step 1: Multi-intent classification + persistence.
	var multiIntentResult ClassifyMultiIntentResult
	if err := workflow.ExecuteActivity(ctx, (*Activities).ClassifyMultiIntentActivity, ClassifyMultiIntentInput{
		WorkspaceID:   input.WorkspaceID,
		IngressTurnID: input.IngressTurnID,
		RawInput:      input.RawInput,
	}).Get(ctx, &multiIntentResult); err != nil {
		return err
	}

	// Step 2: Uncertainty assessment + calibration link.
	var uqResult AssessUncertaintyResult
	if err := workflow.ExecuteActivity(ctx, (*Activities).AssessUncertaintyActivity, AssessUncertaintyInput{
		WorkspaceID:   input.WorkspaceID,
		IngressTurnID: input.IngressTurnID,
		RawConfidence: input.RawConfidence,
		Domain:        "general",
	}).Get(ctx, &uqResult); err != nil {
		return err
	}

	// Step 3: EQ strategy application + logging.
	var eqResult ApplyEQStrategyResult
	if err := workflow.ExecuteActivity(ctx, (*Activities).ApplyEQStrategyActivity, ApplyEQStrategyInput{
		WorkspaceID:   input.WorkspaceID,
		UserID:        input.UserID,
		SessionID:     input.SessionID,
		DetectedState: input.DetectedState,
		CommStyle:     input.CommStyle,
	}).Get(ctx, &eqResult); err != nil {
		return err
	}

	// Step 4: Reasoning loop with critic/reflector persistence.
	type reasoningInput struct {
		WorkspaceID   string  `json:"workspace_id"`
		WorkflowRunID string  `json:"workflow_run_id"`
		Intent        string  `json:"intent"`
		Confidence    float64 `json:"confidence"`
	}
	if err := workflow.ExecuteActivity(ctx, (*Activities).V102ReasoningLoopActivity, reasoningInput{
		WorkspaceID:   input.WorkspaceID,
		WorkflowRunID: workflowRunID,
		Intent:        input.Intent,
		Confidence:    uqResult.CalibratedConfidence,
	}).Get(ctx, nil); err != nil {
		// Reasoning loop failure is non-fatal for the workflow.
		_ = err
	}

	// Step 5: Evaluate interruptions.
	var interruptResult EvaluateInterruptionsResult
	if err := workflow.ExecuteActivity(ctx, (*Activities).EvaluateInterruptionsActivity, EvaluateInterruptionsInput{
		WorkspaceID: input.WorkspaceID,
		ContextStr:  input.RawInput,
	}).Get(ctx, &interruptResult); err != nil {
		_ = err
	}

	return nil
}

// AutonomyDemotionWorkflow evaluates demotion for a workspace/domain and
// records calibration outcomes from the reasoning loop results.
func AutonomyDemotionWorkflow(ctx workflow.Context, input struct {
	WorkspaceID         string  `json:"workspace_id"`
	Domain              string  `json:"domain"`
	TrustScore          float64 `json:"trust_score"`
	FailureCount        int     `json:"failure_count"`
	PredictedConfidence float64 `json:"predicted_confidence"`
	WasCorrect          bool    `json:"was_correct"`
}) error {
	ao := workflow.ActivityOptions{
		StartToCloseTimeout: 15 * time.Second,
		RetryPolicy: &temporal.RetryPolicy{
			MaximumAttempts: 2,
		},
	}
	ctx = workflow.WithActivityOptions(ctx, ao)

	// Evaluate autonomy demotion.
	var demotionResult EvaluateAutonomyDemotionResult
	if err := workflow.ExecuteActivity(ctx, (*Activities).EvaluateAutonomyDemotionActivity, EvaluateAutonomyDemotionInput{
		WorkspaceID:  input.WorkspaceID,
		Domain:       input.Domain,
		TrustScore:   input.TrustScore,
		FailureCount: input.FailureCount,
	}).Get(ctx, &demotionResult); err != nil {
		return err
	}

	// Record calibration outcome.
	if err := workflow.ExecuteActivity(ctx, (*Activities).RecordCalibrationOutcomeActivity, RecordCalibrationOutcomeInput{
		WorkspaceID:         input.WorkspaceID,
		Domain:              input.Domain,
		PredictedConfidence: input.PredictedConfidence,
		WasCorrect:          input.WasCorrect,
	}).Get(ctx, nil); err != nil {
		return err
	}

	return nil
}
