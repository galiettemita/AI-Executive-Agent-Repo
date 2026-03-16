package temporal

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Test 1: ValidateEnvelopeActivity — empty message ID returns invalid.
func TestValidateEnvelopeActivity_EmptyMessageID(t *testing.T) {
	t.Parallel()
	acts := NewActivities()
	result, err := acts.ValidateEnvelopeActivity(context.Background(), ValidateEnvelopeInput{
		WorkspaceID: "ws-test", RawPayload: "hello",
	})
	require.NoError(t, err)
	assert.False(t, result.Valid)
}

// Test 2: ValidateEnvelopeActivity — valid input returns valid.
func TestValidateEnvelopeActivity_ValidInput(t *testing.T) {
	t.Parallel()
	acts := NewActivities()
	result, err := acts.ValidateEnvelopeActivity(context.Background(), ValidateEnvelopeInput{
		MessageID: "msg-001", WorkspaceID: "ws-test", RawPayload: "hello",
	})
	require.NoError(t, err)
	assert.True(t, result.Valid)
	assert.Equal(t, "hello", result.NormalizedPayload)
}

// Test 3: AuthorizePlanActivity — nil OPA evaluator allows in degraded mode.
func TestAuthorizePlanActivity_NilOPA_AllowsDegraded(t *testing.T) {
	t.Parallel()
	acts := NewActivities()
	result, err := acts.AuthorizePlanActivity(context.Background(), AuthorizePlanInput{
		WorkspaceID: "ws-test",
		PlanID:      "plan-001",
		ToolKeys:    []string{"google_calendar.read_events"},
		RiskLevel:   "low",
	})
	require.NoError(t, err)
	assert.Equal(t, "allow", result.Decision)
	assert.NotEmpty(t, result.ReceiptID)
}

// Test 4: ExecuteToolActivity — missing receipt is always rejected.
func TestExecuteToolActivity_MissingReceipt_Rejected(t *testing.T) {
	t.Parallel()
	acts := NewActivities()
	_, err := acts.ExecuteToolActivity(context.Background(), ExecuteToolInput{
		WorkspaceID: "ws-test",
		ToolKey:     "google_calendar.create_event",
		ReceiptID:   "", // no receipt
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "AUTHORIZATION_REQUIRED")
}

// Test 5: SynthesizeResponseActivity — no LLM returns deterministic template.
func TestSynthesizeResponseActivity_NoLLM_ReturnsTemplate(t *testing.T) {
	t.Parallel()
	acts := NewActivities()
	result, err := acts.SynthesizeResponseActivity(context.Background(), SynthesizeResponseInput{
		MessageID:   "msg-001",
		WorkspaceID: "ws-test",
		ToolResults: []ToolExecutionActivityResult{{Success: true, ToolOutput: `{"id":"1"}`}},
	})
	require.NoError(t, err)
	assert.NotNil(t, result)
	assert.NotEmpty(t, result.ResponsePayload)
}

// Test 6: ActivateKillSwitchActivity — returns activated result.
func TestActivateKillSwitchActivity_DegradedMode(t *testing.T) {
	t.Parallel()
	acts := NewActivities()
	result, err := acts.ActivateKillSwitchActivity(context.Background(), KillSwitchInput{
		WorkspaceID: "ws-kill",
		Reason:      "security",
		ActivatedBy: "admin-1",
	})
	require.NoError(t, err)
	assert.NotNil(t, result)
}

// Test 7: AggregateCostsActivity — degraded mode returns empty result.
func TestAggregateCostsActivity_DegradedMode(t *testing.T) {
	t.Parallel()
	acts := NewActivities()
	result, err := acts.AggregateCostsActivity(context.Background(), CostRollupInput{
		WorkspaceID: "ws-cost",
	})
	require.NoError(t, err)
	assert.NotNil(t, result)
}

// Test 8: AnalyseSentimentActivity — empty transcript returns neutral.
func TestAnalyseSentimentActivity_EmptyTranscript(t *testing.T) {
	t.Parallel()
	acts := NewActivities()
	result, err := acts.AnalyseSentimentActivity(context.Background(), AnalyseSentimentInput{
		Transcript: "",
	})
	require.NoError(t, err)
	assert.Equal(t, "neutral", result.OverallLabel)
}

// Test 9: AnalyseSentimentActivity — no LLM returns unavailable message.
func TestAnalyseSentimentActivity_NoLLM_ReturnsUnavailable(t *testing.T) {
	t.Parallel()
	acts := NewActivities()
	result, err := acts.AnalyseSentimentActivity(context.Background(), AnalyseSentimentInput{
		Transcript: "Great meeting today!",
	})
	require.NoError(t, err)
	assert.Equal(t, "neutral", result.OverallLabel)
	assert.Contains(t, result.Summary, "unavailable")
}

// Test 10: GeneratePlanActivity — degraded mode returns deterministic plan.
func TestGeneratePlanActivity_DegradedMode(t *testing.T) {
	t.Parallel()
	acts := NewActivities()
	result, err := acts.GeneratePlanActivity(context.Background(), GeneratePlanInput{
		MessageID:   "msg-plan",
		WorkspaceID: "ws-test",
		Intent:      "google_calendar.create_event",
		Confidence:  0.9,
		Payload:     "schedule a meeting",
	})
	require.NoError(t, err)
	assert.NotNil(t, result)
	assert.NotEmpty(t, result.PlanID)
}
