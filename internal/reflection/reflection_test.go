package reflection_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/brevio/brevio/internal/reflection"
)

func TestClusterIntents_EmailTheme(t *testing.T) {
	events := []reflection.IntentEvent{
		{Intent: "send email to Alice", Outcome: "success"},
		{Intent: "reply to email from Bob", Outcome: "success"},
		{Intent: "forward the invoice email", Outcome: "failure"},
		{Intent: "schedule meeting tomorrow", Outcome: "success"},
	}
	clusters := reflection.ClusterIntents(events)
	assert.NotEmpty(t, clusters)

	found := false
	for _, c := range clusters {
		if c.Theme == "email_management" {
			assert.Equal(t, 3, c.Count)
			found = true
		}
	}
	assert.True(t, found, "email_management cluster expected")
}

func TestClusterIntents_EmptyInput(t *testing.T) {
	clusters := reflection.ClusterIntents(nil)
	assert.Empty(t, clusters)
}

func TestIdentifyFailurePatterns_HighFailRate(t *testing.T) {
	events := []reflection.ToolEvent{
		{ToolKey: "calendar.create", Success: false, ErrorCode: "UNAUTHORIZED"},
		{ToolKey: "calendar.create", Success: false, ErrorCode: "UNAUTHORIZED"},
		{ToolKey: "calendar.create", Success: false, ErrorCode: "UNAUTHORIZED"},
		{ToolKey: "calendar.create", Success: true},
		{ToolKey: "email.send", Success: true},
		{ToolKey: "email.send", Success: true},
	}
	patterns := reflection.IdentifyFailurePatterns(events)
	assert.NotEmpty(t, patterns)
	assert.Equal(t, "calendar.create", patterns[0].ToolKey)
	assert.Equal(t, 3, patterns[0].FailCount)
	assert.InDelta(t, 0.75, patterns[0].FailRate, 0.01)
	assert.Contains(t, patterns[0].RootCause, "OAuth")
}

func TestIdentifyFailurePatterns_LowFailRate_NotReported(t *testing.T) {
	events := []reflection.ToolEvent{
		{ToolKey: "email.send", Success: false},
		{ToolKey: "email.send", Success: true},
		{ToolKey: "email.send", Success: true},
		{ToolKey: "email.send", Success: true},
		{ToolKey: "email.send", Success: true},
	}
	patterns := reflection.IdentifyFailurePatterns(events)
	assert.Empty(t, patterns, "single failure below threshold should not be reported")
}

func TestGenerateInsights_ProducesRecords(t *testing.T) {
	clusters := []reflection.IntentCluster{
		{Theme: "email_management", Count: 5, SuccessRate: 0.6},
		{Theme: "calendar_scheduling", Count: 3, SuccessRate: 0.33},
	}
	failures := []reflection.ToolFailurePattern{
		{ToolKey: "calendar.create", FailCount: 3, TotalCount: 4, FailRate: 0.75, RootCause: "OAuth token expired"},
	}

	insights := reflection.GenerateInsights("ws-1", "2026-03-15", clusters, failures, 10)
	assert.NotEmpty(t, insights)

	for _, ins := range insights {
		assert.NotEmpty(t, ins.Body)
		assert.Greater(t, ins.Strength, 0.0)
		assert.LessOrEqual(t, ins.Strength, 1.0)
		assert.NotEmpty(t, ins.InsightType)
	}
}

func TestGenerateInsights_MaxInsightsRespected(t *testing.T) {
	clusters := make([]reflection.IntentCluster, 20)
	for i := range clusters {
		clusters[i] = reflection.IntentCluster{Theme: "email_management", Count: 5, SuccessRate: 0.4}
	}
	insights := reflection.GenerateInsights("ws-1", "2026-03-15", clusters, nil, 3)
	assert.LessOrEqual(t, len(insights), 3)
}

func TestGenerateInsights_EmptyLog_NoInsights(t *testing.T) {
	insights := reflection.GenerateInsights("ws-1", "2026-03-15", nil, nil, 10)
	assert.Empty(t, insights, "empty day log should produce no insights")
}
