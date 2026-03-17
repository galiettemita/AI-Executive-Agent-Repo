package temporal

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.temporal.io/sdk/testsuite"

	"github.com/brevio/brevio/internal/memory"
)

func TestKGExtractActivity_NilService_NoError(t *testing.T) {
	ts := &testsuite.WorkflowTestSuite{}
	env := ts.NewTestActivityEnvironment()
	acts := NewActivities()
	env.RegisterActivity(acts.KGExtractActivity)

	_, err := env.ExecuteActivity(acts.KGExtractActivity, KGExtractInput{
		WorkspaceID: "ws-1", TurnID: "turn-1",
		Content: "Alice Johnson is the new VP of Product at Acme Corp.",
	})
	require.NoError(t, err)
}

func TestKGExtractActivity_EmptyContent_NoError(t *testing.T) {
	ts := &testsuite.WorkflowTestSuite{}
	env := ts.NewTestActivityEnvironment()
	acts := NewActivities()
	env.RegisterActivity(acts.KGExtractActivity)

	_, err := env.ExecuteActivity(acts.KGExtractActivity, KGExtractInput{
		WorkspaceID: "ws-1", TurnID: "t-1", Content: "",
	})
	require.NoError(t, err)
}

func TestSearchRAGActivity_KGContextSnippetField(t *testing.T) {
	ts := &testsuite.WorkflowTestSuite{}
	env := ts.NewTestActivityEnvironment()
	acts := NewActivities()
	env.RegisterActivity(acts.SearchRAGActivity)

	val, err := env.ExecuteActivity(acts.SearchRAGActivity, RAGSearchInput{
		MessageID: "msg-001", WorkspaceID: "00000000-0000-0000-0000-000000000001",
		Query: "Alice's role at Acme", TopK: 3,
	})
	require.NoError(t, err)
	var result RAGSearchResult
	require.NoError(t, val.Get(&result))
	_ = result.KGContextSnippet // compile-time check
}

func TestClusterEpisodicItems_GroupsByTheme(t *testing.T) {
	items := []memory.Item{
		{Body: "scheduled a meeting with Alice", MemoryType: "episodic", Confidence: 0.7},
		{Body: "rescheduled the board meeting", MemoryType: "episodic", Confidence: 0.8},
		{Body: "blocked focus time on calendar", MemoryType: "episodic", Confidence: 0.6},
		{Body: "sent email to legal team", MemoryType: "episodic", Confidence: 0.5},
	}
	clusters := clusterEpisodicItems(items)
	assert.NotEmpty(t, clusters)
	found := false
	for _, c := range clusters {
		if c.Theme == "scheduling" {
			assert.GreaterOrEqual(t, len(c.Items), 2)
			found = true
		}
	}
	assert.True(t, found, "scheduling cluster must be present")
}

func TestAvgConfidence_ZeroDefaultsToHalf(t *testing.T) {
	items := []memory.Item{{Confidence: 0}, {Confidence: 0.8}}
	avg := avgConfidence(items)
	assert.InDelta(t, 0.65, avg, 0.01)
}

func TestBuildSemanticFact_ContainsThemeAndCount(t *testing.T) {
	cluster := EpisodicCluster{
		Theme: "scheduling",
		Items: []memory.Item{
			{Body: "scheduled meeting with Alice", Confidence: 0.8},
			{Body: "rescheduled board meeting", Confidence: 0.7},
			{Body: "blocked focus time", Confidence: 0.6},
		},
	}
	fact := buildSemanticFact(cluster)
	assert.Contains(t, fact, "scheduling")
	assert.Contains(t, fact, "3")
}

func TestConsolidationActivity_NoItems_Completes(t *testing.T) {
	ts := &testsuite.WorkflowTestSuite{}
	env := ts.NewTestActivityEnvironment()
	acts := NewActivities()
	env.RegisterActivity(acts.RunConsolidationActivity)

	val, err := env.ExecuteActivity(acts.RunConsolidationActivity, RunConsolidationInput{
		WorkspaceID: "00000000-0000-0000-0000-000000000001", RunDate: "2026-03-16",
	})
	require.NoError(t, err)
	var result RunConsolidationResult
	require.NoError(t, val.Get(&result))
	assert.Equal(t, "complete", result.Status)
	assert.Equal(t, 0, result.EpisodesAnalyzed)
}

func TestConsolidationActivity_NoSyntheticEpisodes(t *testing.T) {
	ts := &testsuite.WorkflowTestSuite{}
	env := ts.NewTestActivityEnvironment()
	acts := NewActivities()
	env.RegisterActivity(acts.RunConsolidationActivity)

	val, err := env.ExecuteActivity(acts.RunConsolidationActivity, RunConsolidationInput{
		WorkspaceID: "00000000-0000-0000-0000-000000000001",
	})
	require.NoError(t, err)
	var result RunConsolidationResult
	require.NoError(t, val.Get(&result))
	assert.Equal(t, 0, result.EpisodesAnalyzed, "no memorySvc means 0 real episodes — not 1 synthetic")
	assert.Equal(t, "complete", result.Status)
}
