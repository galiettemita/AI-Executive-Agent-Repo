package temporal

import (
	"testing"

	"go.temporal.io/sdk/testsuite"
)

func TestMessageProcessingWorkflow_HappyPath(t *testing.T) {
	testSuite := &testsuite.WorkflowTestSuite{}
	env := testSuite.NewTestWorkflowEnvironment()

	activities := NewActivities()
	env.RegisterActivity(activities.ValidateEnvelopeActivity)
	env.RegisterActivity(activities.ClassifyIntentActivity)
	env.RegisterActivity(activities.GeneratePlanActivity)
	env.RegisterActivity(activities.AuthorizePlanActivity)
	env.RegisterActivity(activities.ExecuteToolActivity)
	env.RegisterActivity(activities.SynthesizeResponseActivity)

	input := MessageProcessingWorkflowInput{
		MessageID:      "test-msg-001",
		WorkspaceID:    "ws-001",
		ChannelType:    "web",
		RawPayload:     "hello world",
		IdempotencyKey: "idem-001",
	}

	env.ExecuteWorkflow(MessageProcessingWorkflow, input)

	if !env.IsWorkflowCompleted() {
		t.Fatal("workflow did not complete")
	}
	if err := env.GetWorkflowError(); err != nil {
		t.Fatalf("workflow error: %v", err)
	}

	var result MessageProcessingWorkflowResult
	if err := env.GetWorkflowResult(&result); err != nil {
		t.Fatalf("failed to get result: %v", err)
	}
	if result.TerminalState != "COMPLETED" {
		t.Fatalf("expected COMPLETED, got %s", result.TerminalState)
	}
}

func TestMessageProcessingWorkflow_InvalidEnvelope(t *testing.T) {
	testSuite := &testsuite.WorkflowTestSuite{}
	env := testSuite.NewTestWorkflowEnvironment()

	activities := NewActivities()
	env.RegisterActivity(activities.ValidateEnvelopeActivity)
	env.RegisterActivity(activities.ClassifyIntentActivity)
	env.RegisterActivity(activities.GeneratePlanActivity)
	env.RegisterActivity(activities.AuthorizePlanActivity)
	env.RegisterActivity(activities.ExecuteToolActivity)
	env.RegisterActivity(activities.SynthesizeResponseActivity)

	input := MessageProcessingWorkflowInput{
		MessageID:   "test-msg-002",
		WorkspaceID: "ws-001",
		ChannelType: "web",
		RawPayload:  "", // empty payload = invalid
	}

	env.ExecuteWorkflow(MessageProcessingWorkflow, input)

	if !env.IsWorkflowCompleted() {
		t.Fatal("workflow did not complete")
	}

	var result MessageProcessingWorkflowResult
	if err := env.GetWorkflowResult(&result); err != nil {
		t.Fatalf("failed to get result: %v", err)
	}
	if result.TerminalState != "DEAD_LETTER" {
		t.Fatalf("expected DEAD_LETTER for invalid envelope, got %s", result.TerminalState)
	}
}

func TestOnboardingWorkflow_Complete(t *testing.T) {
	testSuite := &testsuite.WorkflowTestSuite{}
	env := testSuite.NewTestWorkflowEnvironment()

	activities := NewActivities()
	env.RegisterActivity(activities.ExecuteOnboardingStageActivity)

	input := OnboardingWorkflowInput{
		WorkspaceID: "ws-001",
		Answers: map[string]string{
			"operator_profile_intake_v1":     "done",
			"behavior_policy_calibration_v1": "done",
			"codebase_map_ingestion_v1":      "done",
			"system_map_ingestion_v1":        "done",
		},
	}

	env.ExecuteWorkflow(OnboardingWorkflow, input)

	if !env.IsWorkflowCompleted() {
		t.Fatal("workflow did not complete")
	}

	var result OnboardingWorkflowResult
	if err := env.GetWorkflowResult(&result); err != nil {
		t.Fatalf("failed to get result: %v", err)
	}
	if result.Status != "completed" {
		t.Fatalf("expected completed, got %s", result.Status)
	}
	if len(result.CompletedStages) != 4 {
		t.Fatalf("expected 4 stages, got %d", len(result.CompletedStages))
	}
}

func TestOnboardingWorkflow_Incomplete(t *testing.T) {
	testSuite := &testsuite.WorkflowTestSuite{}
	env := testSuite.NewTestWorkflowEnvironment()

	activities := NewActivities()
	env.RegisterActivity(activities.ExecuteOnboardingStageActivity)

	input := OnboardingWorkflowInput{
		WorkspaceID: "ws-001",
		Answers: map[string]string{
			"operator_profile_intake_v1": "done",
			// Missing other stages
		},
	}

	env.ExecuteWorkflow(OnboardingWorkflow, input)

	if !env.IsWorkflowCompleted() {
		t.Fatal("workflow did not complete")
	}

	var result OnboardingWorkflowResult
	if err := env.GetWorkflowResult(&result); err != nil {
		t.Fatalf("failed to get result: %v", err)
	}
	if result.Status != "incomplete" {
		t.Fatalf("expected incomplete, got %s", result.Status)
	}
}

func TestOutboxDispatchWorkflow(t *testing.T) {
	testSuite := &testsuite.WorkflowTestSuite{}
	env := testSuite.NewTestWorkflowEnvironment()

	activities := NewActivities()
	env.RegisterActivity(activities.FetchPendingOutboxActivity)
	env.RegisterActivity(activities.DispatchOutboxEntryActivity)

	input := OutboxDispatchInput{BatchSize: 10}
	env.ExecuteWorkflow(OutboxDispatchWorkflow, input)

	if !env.IsWorkflowCompleted() {
		t.Fatal("workflow did not complete")
	}
	if err := env.GetWorkflowError(); err != nil {
		t.Fatalf("workflow error: %v", err)
	}
}

func TestToolHealthEvaluationWorkflow(t *testing.T) {
	testSuite := &testsuite.WorkflowTestSuite{}
	env := testSuite.NewTestWorkflowEnvironment()

	activities := NewActivities()
	env.RegisterActivity(activities.EvaluateToolHealthActivity)

	input := ToolHealthEvalInput{ToolKey: "send_email", WindowSeconds: 60}
	env.ExecuteWorkflow(ToolHealthEvaluationWorkflow, input)

	if !env.IsWorkflowCompleted() {
		t.Fatal("workflow did not complete")
	}

	var result ToolHealthEvalResult
	if err := env.GetWorkflowResult(&result); err != nil {
		t.Fatalf("failed to get result: %v", err)
	}
	if result.Status != "healthy" {
		t.Fatalf("expected healthy, got %s", result.Status)
	}
}
