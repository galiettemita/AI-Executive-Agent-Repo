package temporal

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDualProcessRouting_HighConfidenceReadOp_System1(t *testing.T) {
	t.Parallel()
	acts := NewActivities()
	result, err := acts.DualProcessRoutingActivity(context.Background(), DualProcessRoutingInput{
		WorkspaceID:    "ws-test",
		MessageContent: "what time is my next meeting",
		IntentKey:      "google_calendar.read_events",
		Confidence:     0.95,
	})
	require.NoError(t, err)
	assert.True(t, result.UseSystem1, "high confidence read op should use System 1")
	assert.False(t, result.UseSystem2)
}

func TestDualProcessRouting_WriteOp_System2(t *testing.T) {
	t.Parallel()
	acts := NewActivities()
	result, err := acts.DualProcessRoutingActivity(context.Background(), DualProcessRoutingInput{
		WorkspaceID: "ws-test",
		IntentKey:   "gmail.send_email",
		Confidence:  0.95,
	})
	require.NoError(t, err)
	assert.True(t, result.UseSystem2, "write op should always use System 2")
}

func TestDualProcessRouting_LowConfidence_System2(t *testing.T) {
	t.Parallel()
	acts := NewActivities()
	result, err := acts.DualProcessRoutingActivity(context.Background(), DualProcessRoutingInput{
		WorkspaceID: "ws-test",
		IntentKey:   "google_calendar.read_events",
		Confidence:  0.5,
	})
	require.NoError(t, err)
	assert.True(t, result.UseSystem2, "low confidence should use System 2")
}

func TestClarificationCheck_LowConfidenceWriteOp_NeedsClarification(t *testing.T) {
	t.Parallel()
	acts := NewActivities()
	result, err := acts.ClarificationCheckActivity(context.Background(), ClarificationCheckInput{
		WorkspaceID: "ws-test",
		IntentKey:   "gmail.send_email",
		ToolKeys:    []string{"gmail.send_email"},
		Confidence:  0.5,
	})
	require.NoError(t, err)
	assert.True(t, result.NeedsClarification)
	assert.NotEmpty(t, result.Question)
}

func TestClarificationCheck_HighConfidence_NoQuestion(t *testing.T) {
	t.Parallel()
	acts := NewActivities()
	result, err := acts.ClarificationCheckActivity(context.Background(), ClarificationCheckInput{
		WorkspaceID: "ws-test",
		IntentKey:   "google_calendar.read_events",
		ToolKeys:    []string{"google_calendar.read_events"},
		Confidence:  0.92,
	})
	require.NoError(t, err)
	assert.False(t, result.NeedsClarification)
}

func TestResponseDriftCheck_MatchingResponse_NoDrift(t *testing.T) {
	t.Parallel()
	acts := NewActivities()
	result, err := acts.ResponseDriftCheckActivity(context.Background(), ResponseDriftCheckInput{
		WorkspaceID:    "ws-test",
		OriginalIntent: "calendar create event",
		Response:       "I have created a calendar event for tomorrow at 9am.",
		IntentKey:      "google_calendar.create_event",
	})
	require.NoError(t, err)
	assert.False(t, result.DriftDetected)
}

func TestResponseDriftCheck_UnrelatedResponse_DriftDetected(t *testing.T) {
	t.Parallel()
	acts := NewActivities()
	result, err := acts.ResponseDriftCheckActivity(context.Background(), ResponseDriftCheckInput{
		WorkspaceID:    "ws-test",
		OriginalIntent: "calendar create event",
		Response:       "The weather in London today is partly cloudy with a high of 14C.",
		IntentKey:      "google_calendar.create_event",
	})
	require.NoError(t, err)
	assert.True(t, result.DriftDetected)
}

func TestResponseDriftCheck_EmptyResponse_NoDrift(t *testing.T) {
	t.Parallel()
	acts := NewActivities()
	result, err := acts.ResponseDriftCheckActivity(context.Background(), ResponseDriftCheckInput{
		WorkspaceID:    "ws-test",
		OriginalIntent: "anything",
		Response:       "",
	})
	require.NoError(t, err)
	assert.False(t, result.DriftDetected)
}
