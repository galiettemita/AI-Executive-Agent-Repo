package temporal

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/mock"
	"go.temporal.io/sdk/testsuite"
)

func TestMessageProcessingWorkflow_SpeculativeMemoryFails(t *testing.T) {
	testSuite := &testsuite.WorkflowTestSuite{}
	env := testSuite.NewTestWorkflowEnvironment()

	activities := NewActivities()
	registerAllMessageActivities(env, activities)

	// Override RetrieveMemoryActivity to return an error.
	env.OnActivity(activities.RetrieveMemoryActivity, mock.Anything, mock.Anything).
		Return(nil, fmt.Errorf("memory service unavailable"))

	input := MessageProcessingWorkflowInput{
		MessageID:      "test-spec-mem-001",
		WorkspaceID:    "ws-001",
		ChannelType:    "web",
		RawPayload:     "check my calendar",
		IdempotencyKey: "idem-spec-mem-001",
	}

	env.ExecuteWorkflow(MessageProcessingWorkflow, input)

	if !env.IsWorkflowCompleted() {
		t.Fatal("workflow did not complete")
	}

	var result MessageProcessingWorkflowResult
	if err := env.GetWorkflowResult(&result); err != nil {
		t.Fatalf("failed to get result: %v", err)
	}
	if result.TerminalState != "COMPLETED" {
		t.Fatalf("expected COMPLETED (memory failure is non-fatal), got %s", result.TerminalState)
	}
}

func TestMessageProcessingWorkflow_SpeculativeClassifyFails(t *testing.T) {
	testSuite := &testsuite.WorkflowTestSuite{}
	env := testSuite.NewTestWorkflowEnvironment()

	activities := NewActivities()
	registerAllMessageActivities(env, activities)

	// Override ClassifyIntentActivity to return an error.
	env.OnActivity(activities.ClassifyIntentActivity, mock.Anything, mock.Anything).
		Return(nil, fmt.Errorf("classify service unavailable"))

	input := MessageProcessingWorkflowInput{
		MessageID:      "test-spec-cls-001",
		WorkspaceID:    "ws-001",
		ChannelType:    "web",
		RawPayload:     "send an email",
		IdempotencyKey: "idem-spec-cls-001",
	}

	env.ExecuteWorkflow(MessageProcessingWorkflow, input)

	if !env.IsWorkflowCompleted() {
		t.Fatal("workflow did not complete")
	}

	var result MessageProcessingWorkflowResult
	if err := env.GetWorkflowResult(&result); err != nil {
		t.Fatalf("failed to get result: %v", err)
	}
	if result.TerminalState != "FAILED" {
		t.Fatalf("expected FAILED for classify failure, got %s", result.TerminalState)
	}
	found := false
	for _, f := range result.Fallbacks {
		if f == "classify_failed" {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("expected Fallbacks to contain 'classify_failed', got %v", result.Fallbacks)
	}
}

func TestMessageProcessingWorkflow_SpeculativeAllSucceed(t *testing.T) {
	testSuite := &testsuite.WorkflowTestSuite{}
	env := testSuite.NewTestWorkflowEnvironment()

	activities := NewActivities()
	registerAllMessageActivities(env, activities)

	input := MessageProcessingWorkflowInput{
		MessageID:      "test-spec-all-001",
		WorkspaceID:    "ws-001",
		ChannelType:    "web",
		RawPayload:     "check my calendar for tomorrow",
		IdempotencyKey: "idem-spec-all-001",
	}

	env.ExecuteWorkflow(MessageProcessingWorkflow, input)

	if !env.IsWorkflowCompleted() {
		t.Fatal("workflow did not complete")
	}

	var result MessageProcessingWorkflowResult
	if err := env.GetWorkflowResult(&result); err != nil {
		t.Fatalf("failed to get result: %v", err)
	}
	if result.TerminalState != "COMPLETED" {
		t.Fatalf("expected COMPLETED, got %s", result.TerminalState)
	}
	if result.MemoryItemCount < 0 {
		t.Error("expected non-negative memory item count")
	}
	if result.RAGChunkCount < 0 {
		t.Error("expected non-negative RAG chunk count")
	}
}
