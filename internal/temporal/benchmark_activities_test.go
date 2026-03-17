package temporal

import (
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.temporal.io/sdk/testsuite"

	"github.com/brevio/brevio/internal/benchmark"
)

func TestBenchmarkInitRunActivity_NilRepo(t *testing.T) {
	ts := &testsuite.WorkflowTestSuite{}
	env := ts.NewTestActivityEnvironment()
	acts := NewActivities()
	env.RegisterActivity(acts.InitBenchmarkRunActivity)

	_, err := env.ExecuteActivity(acts.InitBenchmarkRunActivity, benchmark.GAIARunnerInput{
		DatasetPath: "../../evals/gaia/brevio_gaia_dataset.json",
		TriggeredBy: "test",
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "NOT_CONFIGURED")
}

func TestBenchmarkRunTaskActivity_EasyTask(t *testing.T) {
	ts := &testsuite.WorkflowTestSuite{}
	env := ts.NewTestActivityEnvironment()
	acts := NewActivities()
	env.RegisterActivity(acts.RunBenchmarkTaskActivity)

	val, err := env.ExecuteActivity(acts.RunBenchmarkTaskActivity, benchmark.TaskRunInput{
		RunID: uuid.New(),
		Task: benchmark.GAIATask{
			ID: "brevio-001", Tier: "easy", Category: "email",
			Intent: "How many unread emails do I have?",
			ExpectedToolKeys: []string{"email.read"},
			PassCriteria: "response contains a number", TimeoutSeconds: 30,
		},
		WorkspaceID: "ws-test",
	})
	require.NoError(t, err)
	var result benchmark.TaskResult
	require.NoError(t, val.Get(&result))
	assert.Equal(t, "brevio-001", result.TaskID)
	assert.Equal(t, "easy", result.Tier)
	assert.NotNil(t, result.LatencyMs)
}

func TestBenchmarkFinalizeActivity_NilRepo_NoError(t *testing.T) {
	ts := &testsuite.WorkflowTestSuite{}
	env := ts.NewTestActivityEnvironment()
	acts := NewActivities()
	env.RegisterActivity(acts.FinalizeBenchmarkRunActivity)

	priorRate := 0.80
	var results []benchmark.TaskResult
	for i := 0; i < 10; i++ {
		results = append(results, benchmark.TaskResult{Tier: "easy", Passed: i < 6})
	}

	_, err := env.ExecuteActivity(acts.FinalizeBenchmarkRunActivity, BenchmarkFinalizeInput{
		RunID: uuid.New(), Results: results,
		StartedAt: time.Now().Add(-10 * time.Minute), PriorPassRate: &priorRate,
	})
	require.NoError(t, err)
}

func TestBenchmarkRegressionDetection_Threshold(t *testing.T) {
	cases := []struct {
		prior, current float64
		expectAlert    bool
	}{
		{0.80, 0.74, true},
		{0.80, 0.76, false},
		{0.80, 0.85, false},
		{0.60, 0.54, true},
	}
	for _, tc := range cases {
		drop := tc.prior - tc.current
		got := drop > benchmark.RegressionThreshold
		assert.Equal(t, tc.expectAlert, got,
			"prior=%.2f current=%.2f", tc.prior, tc.current)
	}
}
