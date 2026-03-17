package temporal

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.temporal.io/sdk/testsuite"

	"github.com/brevio/brevio/internal/dpo"
)

func TestDPOFeedbackIngestionActivity_NilService_NoError(t *testing.T) {
	ts := &testsuite.WorkflowTestSuite{}
	env := ts.NewTestActivityEnvironment()
	acts := NewActivities()
	env.RegisterActivity(acts.FeedbackIngestionActivity)

	val, err := env.ExecuteActivity(acts.FeedbackIngestionActivity, dpo.FeedbackIngestionInput{
		WorkspaceID: "ws-1", UserID: "u-1", SignalType: "edit",
	})
	require.NoError(t, err)
	var pair dpo.PreferencePair
	require.NoError(t, val.Get(&pair))
	assert.Equal(t, dpo.PreferencePair{}, pair)
}

func TestDPODatasetReadyActivity_NilService(t *testing.T) {
	ts := &testsuite.WorkflowTestSuite{}
	env := ts.NewTestActivityEnvironment()
	acts := NewActivities()
	env.RegisterActivity(acts.DPODatasetReadyActivity)

	val, err := env.ExecuteActivity(acts.DPODatasetReadyActivity, "")
	require.NoError(t, err)
	var result DPOReadinessResult
	require.NoError(t, val.Get(&result))
	assert.False(t, result.Ready)
}

func TestDPOQualityDeltaMonitorActivity_NoRollback(t *testing.T) {
	ts := &testsuite.WorkflowTestSuite{}
	env := ts.NewTestActivityEnvironment()
	acts := NewActivities()
	env.RegisterActivity(acts.QualityDeltaMonitorActivity)

	val, err := env.ExecuteActivity(acts.QualityDeltaMonitorActivity, dpo.QualityDeltaInput{
		WorkspaceID:    "ws-1",
		BaselineScore:  0.80,
		EvalWindowDays: 7,
	})
	require.NoError(t, err)
	var result DPOQualityDeltaResult
	require.NoError(t, val.Get(&result))
	assert.False(t, result.RolledBack)
}

func TestDPOCheckpointDeployActivity_NilService(t *testing.T) {
	ts := &testsuite.WorkflowTestSuite{}
	env := ts.NewTestActivityEnvironment()
	acts := NewActivities()
	env.RegisterActivity(acts.CheckpointDeployActivity)

	_, err := env.ExecuteActivity(acts.CheckpointDeployActivity, dpo.CheckpointDeployInput{
		WorkspaceID: "ws-1", CheckpointID: "cp-123", RoundNumber: 1, BaselineScore: 0.80,
	})
	require.NoError(t, err)
}
