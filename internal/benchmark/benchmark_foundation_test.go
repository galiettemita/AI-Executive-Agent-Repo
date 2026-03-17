package benchmark_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/brevio/brevio/internal/benchmark"
)

func TestLoadDataset(t *testing.T) {
	ds, err := benchmark.LoadDataset("../../evals/gaia/brevio_gaia_dataset.json")
	require.NoError(t, err)
	assert.GreaterOrEqual(t, len(ds.Tasks), 100, "dataset must have >= 100 tasks")
	assert.Equal(t, "1.0", ds.Version)
}

func TestDatasetTierDistribution(t *testing.T) {
	ds, err := benchmark.LoadDataset("../../evals/gaia/brevio_gaia_dataset.json")
	require.NoError(t, err)
	tiers := map[string]int{}
	for _, task := range ds.Tasks {
		tiers[task.Tier]++
	}
	assert.GreaterOrEqual(t, tiers["easy"], 30)
	assert.GreaterOrEqual(t, tiers["medium"], 30)
	assert.GreaterOrEqual(t, tiers["hard"], 20)
}

func TestDatasetUniqueIDs(t *testing.T) {
	ds, err := benchmark.LoadDataset("../../evals/gaia/brevio_gaia_dataset.json")
	require.NoError(t, err)
	seen := map[string]bool{}
	for _, task := range ds.Tasks {
		assert.False(t, seen[task.ID], "duplicate task ID: %s", task.ID)
		seen[task.ID] = true
		assert.NotEmpty(t, task.Intent)
		assert.NotEmpty(t, task.PassCriteria)
		assert.Greater(t, task.TimeoutSeconds, 0)
	}
}

func TestFilterByTier(t *testing.T) {
	ds, err := benchmark.LoadDataset("../../evals/gaia/brevio_gaia_dataset.json")
	require.NoError(t, err)
	for _, tier := range []string{"easy", "medium", "hard"} {
		filtered := ds.FilterByTier(tier)
		assert.NotEmpty(t, filtered, "tier %s should have tasks", tier)
		for _, task := range filtered {
			assert.Equal(t, tier, task.Tier)
		}
	}
	all := ds.FilterByTier("")
	assert.Equal(t, len(ds.Tasks), len(all))
}

func TestDefaultGrader_AllToolsCalled_Passes(t *testing.T) {
	task := benchmark.GAIATask{
		ID: "test-001", ExpectedToolKeys: []string{"email.send"},
		PassCriteria: "email.send called",
	}
	passed, detail := benchmark.DefaultGrader(task, []string{"email.send"}, "Email sent successfully to alice@example.com")
	assert.True(t, passed)
	assert.Contains(t, detail, "PASS")
}

func TestDefaultGrader_MissingTool_Fails(t *testing.T) {
	task := benchmark.GAIATask{
		ID: "test-002", ExpectedToolKeys: []string{"calendar.create", "email.send"},
		PassCriteria: "both tools called",
	}
	passed, detail := benchmark.DefaultGrader(task, []string{"calendar.create"}, "Meeting scheduled.")
	assert.False(t, passed)
	assert.Contains(t, detail, "email.send")
}

func TestDefaultGrader_ShortResponse_Fails(t *testing.T) {
	task := benchmark.GAIATask{ID: "test-003", ExpectedToolKeys: []string{}, PassCriteria: "response non-empty"}
	passed, detail := benchmark.DefaultGrader(task, []string{}, "ok")
	assert.False(t, passed)
	assert.Contains(t, detail, "too short")
}

func TestDefaultGrader_Escalation_Passes(t *testing.T) {
	task := benchmark.GAIATask{
		ID: "test-004", ExpectedToolKeys: []string{"slack.send"},
		PassCriteria: "agent escalates to engineering",
	}
	passed, detail := benchmark.DefaultGrader(task, []string{},
		"I cannot fix the payment processor directly. I have alerted the engineering team via Slack.")
	assert.True(t, passed)
	assert.Contains(t, detail, "escalation")
}

func TestDefaultGrader_Clarification_Passes(t *testing.T) {
	task := benchmark.GAIATask{
		ID: "test-005", ExpectedToolKeys: []string{"email.search"},
		PassCriteria: "agent asks for clarification OR infers from context",
	}
	passed, detail := benchmark.DefaultGrader(task, []string{},
		"Could you clarify what you mean by 'the thing with the Apex people'? Which project or meeting are you referring to?")
	assert.True(t, passed)
	assert.Contains(t, detail, "clarification")
}

func TestRegressionThreshold(t *testing.T) {
	assert.Equal(t, 0.05, benchmark.RegressionThreshold)

	// Regression detected
	priorRate := 0.80
	currentRate := 0.74
	delta := currentRate - priorRate
	assert.True(t, -delta > benchmark.RegressionThreshold, "6% drop should trigger regression")

	// No regression
	currentRate = 0.76
	delta = currentRate - priorRate
	assert.False(t, -delta > benchmark.RegressionThreshold, "4% drop should not trigger regression")
}
