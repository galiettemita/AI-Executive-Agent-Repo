package temporal

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"go.temporal.io/sdk/testsuite"

	"github.com/brevio/brevio/internal/subagent"
)

func TestSubAgentOrchestratorWorkflow_AutonomyDenied(t *testing.T) {
	ts := &testsuite.WorkflowTestSuite{}
	env := ts.NewTestWorkflowEnvironment()
	acts := NewActivities()
	env.RegisterActivity(acts.CheckSubAgentAutonomyActivity)
	env.RegisterActivity(acts.DecomposeSubTasksActivity)

	env.OnActivity(acts.CheckSubAgentAutonomyActivity, mock.Anything, mock.Anything).
		Return(subagent.CheckAutonomyResult{CurrentTier: "A2", Permitted: false, Reason: "below A3"}, nil)

	env.ExecuteWorkflow(SubAgentOrchestratorWorkflow, subagent.OrchestratorInput{
		MessageID: "msg-001", WorkspaceID: "ws-1", Intent: "research and schedule",
		ToolKeys: []string{"web.search", "calendar.create"},
	})
	require.True(t, env.IsWorkflowCompleted())
	require.NoError(t, env.GetWorkflowError())

	var result subagent.OrchestratorResult
	require.NoError(t, env.GetWorkflowResult(&result))
	assert.Contains(t, result.TerminalState, "SEQUENTIAL_FALLBACK")
	assert.Equal(t, 0, result.SubTasksLaunched)
}

func TestSubAgentOrchestratorWorkflow_SingleDomain_SequentialPreferred(t *testing.T) {
	ts := &testsuite.WorkflowTestSuite{}
	env := ts.NewTestWorkflowEnvironment()
	acts := NewActivities()
	env.RegisterActivity(acts.CheckSubAgentAutonomyActivity)
	env.RegisterActivity(acts.DecomposeSubTasksActivity)

	env.OnActivity(acts.CheckSubAgentAutonomyActivity, mock.Anything, mock.Anything).
		Return(subagent.CheckAutonomyResult{CurrentTier: "A3", Permitted: true}, nil)
	env.OnActivity(acts.DecomposeSubTasksActivity, mock.Anything, mock.Anything).
		Return(subagent.DecompositionResult{CanParallelize: false, Reason: "same domain"}, nil)

	env.ExecuteWorkflow(SubAgentOrchestratorWorkflow, subagent.OrchestratorInput{
		MessageID: "msg-002", WorkspaceID: "ws-1", Intent: "send emails",
		ToolKeys: []string{"email.send", "email.reply"},
	})
	require.True(t, env.IsWorkflowCompleted())
	require.NoError(t, env.GetWorkflowError())

	var result subagent.OrchestratorResult
	require.NoError(t, env.GetWorkflowResult(&result))
	assert.Contains(t, result.TerminalState, "SEQUENTIAL_PREFERRED")
}

func TestSubAgentOrchestratorWorkflow_TwoDomains_FansOut(t *testing.T) {
	ts := &testsuite.WorkflowTestSuite{}
	env := ts.NewTestWorkflowEnvironment()
	acts := NewActivities()
	env.RegisterActivity(acts.CheckSubAgentAutonomyActivity)
	env.RegisterActivity(acts.DecomposeSubTasksActivity)

	env.OnActivity(acts.CheckSubAgentAutonomyActivity, mock.Anything, mock.Anything).
		Return(subagent.CheckAutonomyResult{CurrentTier: "A3", Permitted: true}, nil)
	env.OnActivity(acts.DecomposeSubTasksActivity, mock.Anything, mock.Anything).
		Return(subagent.DecompositionResult{
			CanParallelize: true, Reason: "2 domains",
			SubTasks: []subagent.SubTask{
				{ID: "sub-0", Domain: subagent.DomainResearch, Priority: 0},
				{ID: "sub-1", Domain: subagent.DomainSchedule, Priority: 1},
			},
		}, nil)

	env.OnWorkflow(MessageProcessingWorkflow, mock.Anything, mock.Anything).Return(
		&MessageProcessingWorkflowResult{TerminalState: "COMPLETED", ResponsePayload: "done"}, nil)

	env.ExecuteWorkflow(SubAgentOrchestratorWorkflow, subagent.OrchestratorInput{
		MessageID: "msg-003", WorkspaceID: "ws-1", Intent: "research and schedule",
		ToolKeys: []string{"web.search", "calendar.create"},
	})
	require.True(t, env.IsWorkflowCompleted())
	require.NoError(t, env.GetWorkflowError())

	var result subagent.OrchestratorResult
	require.NoError(t, env.GetWorkflowResult(&result))
	assert.Equal(t, "COMPLETED", result.TerminalState)
	assert.Equal(t, 2, result.SubTasksLaunched)
	assert.Equal(t, 2, result.SubTasksComplete)
	assert.Equal(t, 0, result.SubTasksFailed)
	assert.Contains(t, result.MergedContext, "PARALLEL SUB-AGENT RESULTS")
}

func TestSubAgentOrchestratorWorkflow_ThreeBuckets(t *testing.T) {
	ts := &testsuite.WorkflowTestSuite{}
	env := ts.NewTestWorkflowEnvironment()
	acts := NewActivities()
	env.RegisterActivity(acts.CheckSubAgentAutonomyActivity)
	env.RegisterActivity(acts.DecomposeSubTasksActivity)

	env.OnActivity(acts.CheckSubAgentAutonomyActivity, mock.Anything, mock.Anything).
		Return(subagent.CheckAutonomyResult{CurrentTier: "A3", Permitted: true}, nil)
	env.OnActivity(acts.DecomposeSubTasksActivity, mock.Anything, mock.Anything).
		Return(subagent.DecompositionResult{
			CanParallelize: true, Reason: "3 domains",
			SubTasks: []subagent.SubTask{
				{ID: "sub-0", Domain: subagent.DomainResearch, Priority: 0},
				{ID: "sub-1", Domain: subagent.DomainSchedule, Priority: 1},
				{ID: "sub-2", Domain: subagent.DomainWrite, Priority: 2},
			},
		}, nil)

	env.OnWorkflow(MessageProcessingWorkflow, mock.Anything, mock.Anything).Return(
		&MessageProcessingWorkflowResult{TerminalState: "COMPLETED", ResponsePayload: "done"}, nil)

	env.ExecuteWorkflow(SubAgentOrchestratorWorkflow, subagent.OrchestratorInput{
		MessageID: "msg-005", WorkspaceID: "ws-1", Intent: "all three",
		ToolKeys: []string{"web.search", "calendar.create", "email.send"},
	})
	require.True(t, env.IsWorkflowCompleted())
	require.NoError(t, env.GetWorkflowError())

	var result subagent.OrchestratorResult
	require.NoError(t, env.GetWorkflowResult(&result))
	assert.Equal(t, 3, result.SubTasksLaunched)
	assert.Equal(t, 3, result.SubTasksComplete)
}

func TestBuildMergedContext_AllCompleted(t *testing.T) {
	results := []subagent.SubTaskResult{
		{SubTaskID: "sub-0", Domain: "research", TerminalState: "COMPLETED", ResponsePayload: "Found 3 competitors."},
		{SubTaskID: "sub-1", Domain: "schedule", TerminalState: "COMPLETED", ResponsePayload: "Meeting booked."},
	}
	ctx := buildMergedContext(results)
	assert.Contains(t, ctx, "PARALLEL SUB-AGENT RESULTS")
	assert.Contains(t, ctx, "research/sub-0")
	assert.Contains(t, ctx, "Found 3 competitors")
}

func TestBuildMergedContext_WithFailure(t *testing.T) {
	results := []subagent.SubTaskResult{
		{SubTaskID: "sub-0", Domain: "research", TerminalState: "COMPLETED", ResponsePayload: "done"},
		{SubTaskID: "sub-1", Domain: "write", TerminalState: "FAILED", Error: "timeout"},
	}
	ctx := buildMergedContext(results)
	assert.Contains(t, ctx, "FAILED")
	assert.Contains(t, ctx, "timeout")
}

func TestBuildMergedContext_Empty(t *testing.T) {
	assert.Empty(t, buildMergedContext(nil))
	assert.Empty(t, buildMergedContext([]subagent.SubTaskResult{}))
}
