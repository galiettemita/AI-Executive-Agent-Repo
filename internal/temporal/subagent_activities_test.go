package temporal

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.temporal.io/sdk/testsuite"

	"github.com/brevio/brevio/internal/subagent"
	"github.com/brevio/brevio/internal/trust"
)

func TestCheckSubAgentAutonomyActivity_A2_DeniedForA3(t *testing.T) {
	ts := &testsuite.WorkflowTestSuite{}
	env := ts.NewTestActivityEnvironment()
	acts := NewActivities()
	svc := trust.NewService()
	svc.UpsertScore(trust.TrustScore{WorkspaceID: "ws-a2", CurrentAutonomy: "A2"})
	acts.trustSvc = svc
	env.RegisterActivity(acts.CheckSubAgentAutonomyActivity)

	val, err := env.ExecuteActivity(acts.CheckSubAgentAutonomyActivity,
		subagent.CheckAutonomyInput{WorkspaceID: "ws-a2", RequiredTier: "A3"})
	require.NoError(t, err)
	var result subagent.CheckAutonomyResult
	require.NoError(t, val.Get(&result))
	assert.False(t, result.Permitted)
	assert.Equal(t, "A2", result.CurrentTier)
}

func TestCheckSubAgentAutonomyActivity_A3_Permitted(t *testing.T) {
	ts := &testsuite.WorkflowTestSuite{}
	env := ts.NewTestActivityEnvironment()
	acts := NewActivities()
	svc := trust.NewService()
	svc.UpsertScore(trust.TrustScore{WorkspaceID: "ws-a3", CurrentAutonomy: "A3"})
	acts.trustSvc = svc
	env.RegisterActivity(acts.CheckSubAgentAutonomyActivity)

	val, err := env.ExecuteActivity(acts.CheckSubAgentAutonomyActivity,
		subagent.CheckAutonomyInput{WorkspaceID: "ws-a3", RequiredTier: "A3"})
	require.NoError(t, err)
	var result subagent.CheckAutonomyResult
	require.NoError(t, val.Get(&result))
	assert.True(t, result.Permitted)
}

func TestCheckSubAgentAutonomyActivity_A4_SatisfiesA3(t *testing.T) {
	ts := &testsuite.WorkflowTestSuite{}
	env := ts.NewTestActivityEnvironment()
	acts := NewActivities()
	svc := trust.NewService()
	svc.UpsertScore(trust.TrustScore{WorkspaceID: "ws-a4", CurrentAutonomy: "A4"})
	acts.trustSvc = svc
	env.RegisterActivity(acts.CheckSubAgentAutonomyActivity)

	val, err := env.ExecuteActivity(acts.CheckSubAgentAutonomyActivity,
		subagent.CheckAutonomyInput{WorkspaceID: "ws-a4", RequiredTier: "A3"})
	require.NoError(t, err)
	var result subagent.CheckAutonomyResult
	require.NoError(t, val.Get(&result))
	assert.True(t, result.Permitted)
}

func TestCheckSubAgentAutonomyActivity_NoTrustSvc_Denied(t *testing.T) {
	ts := &testsuite.WorkflowTestSuite{}
	env := ts.NewTestActivityEnvironment()
	acts := NewActivities()
	env.RegisterActivity(acts.CheckSubAgentAutonomyActivity)

	val, err := env.ExecuteActivity(acts.CheckSubAgentAutonomyActivity,
		subagent.CheckAutonomyInput{WorkspaceID: "ws-unknown", RequiredTier: "A3"})
	require.NoError(t, err)
	var result subagent.CheckAutonomyResult
	require.NoError(t, val.Get(&result))
	assert.False(t, result.Permitted)
	assert.Equal(t, "A1", result.CurrentTier)
}

func TestCheckSubAgentAutonomyActivity_EmptyWorkspaceID_Error(t *testing.T) {
	ts := &testsuite.WorkflowTestSuite{}
	env := ts.NewTestActivityEnvironment()
	acts := NewActivities()
	env.RegisterActivity(acts.CheckSubAgentAutonomyActivity)

	_, err := env.ExecuteActivity(acts.CheckSubAgentAutonomyActivity,
		subagent.CheckAutonomyInput{WorkspaceID: "", RequiredTier: "A3"})
	require.Error(t, err)
}

func TestDecomposeSubTasksActivity_MultiDomain(t *testing.T) {
	ts := &testsuite.WorkflowTestSuite{}
	env := ts.NewTestActivityEnvironment()
	acts := NewActivities()
	env.RegisterActivity(acts.DecomposeSubTasksActivity)

	val, err := env.ExecuteActivity(acts.DecomposeSubTasksActivity,
		subagent.DecomposeInput{Intent: "research and schedule", ToolKeys: []string{"web.search", "calendar.create"}})
	require.NoError(t, err)
	var result subagent.DecompositionResult
	require.NoError(t, val.Get(&result))
	assert.True(t, result.CanParallelize)
	assert.Len(t, result.SubTasks, 2)
}

func TestDecomposeSubTasksActivity_EmptyIntent_Error(t *testing.T) {
	ts := &testsuite.WorkflowTestSuite{}
	env := ts.NewTestActivityEnvironment()
	acts := NewActivities()
	env.RegisterActivity(acts.DecomposeSubTasksActivity)

	_, err := env.ExecuteActivity(acts.DecomposeSubTasksActivity,
		subagent.DecomposeInput{Intent: "", ToolKeys: []string{"email.send"}})
	require.Error(t, err)
}
