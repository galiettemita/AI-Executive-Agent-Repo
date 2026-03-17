package temporal

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.temporal.io/sdk/testsuite"

	"github.com/brevio/brevio/internal/preference"
)

func TestPreferenceUpdateActivity_ValidSignal(t *testing.T) {
	ts := &testsuite.WorkflowTestSuite{}
	env := ts.NewTestActivityEnvironment()

	acts := NewActivities()
	env.RegisterActivity(acts.PreferenceUpdateActivity)

	signals := []string{"undo", "edit", "retry", "skip", "explicit_thumbsdown"}
	for _, sig := range signals {
		t.Run(sig, func(t *testing.T) {
			val, err := env.ExecuteActivity(acts.PreferenceUpdateActivity, preference.UpdateInput{
				Signal: preference.PreferenceSignal{
					WorkspaceID:       "ws-test-001",
					UserID:            "u-test-001",
					WorkflowRunID:     "run-test-" + sig,
					OriginalResponse:  "I scheduled the meeting for 2pm",
					CorrectedResponse: "I scheduled the meeting for 3pm instead",
					OriginalIntent:    "schedule meeting",
					SignalType:        sig,
					ToolKeyUsed:       "calendar.create",
				},
			})
			require.NoError(t, err)
			var fact preference.PreferenceFact
			require.NoError(t, val.Get(&fact))
			assert.Equal(t, 0.95, fact.Confidence, "correction signal must have confidence=0.95")
			assert.NotEmpty(t, fact.Category)
			assert.NotEmpty(t, fact.Preference)
		})
	}
}

func TestPreferenceUpdateActivity_RejectsPositiveSignal(t *testing.T) {
	ts := &testsuite.WorkflowTestSuite{}
	env := ts.NewTestActivityEnvironment()
	acts := NewActivities()
	env.RegisterActivity(acts.PreferenceUpdateActivity)

	_, err := env.ExecuteActivity(acts.PreferenceUpdateActivity, preference.UpdateInput{
		Signal: preference.PreferenceSignal{
			WorkspaceID: "ws-1",
			UserID:      "u-1",
			SignalType:  "click",
		},
	})
	require.Error(t, err, "positive signal should be rejected")
}

func TestPreferenceRetrievalActivity_NoRetrieverConfigured(t *testing.T) {
	ts := &testsuite.WorkflowTestSuite{}
	env := ts.NewTestActivityEnvironment()
	acts := NewActivities()
	env.RegisterActivity(acts.PreferenceRetrievalActivity)

	val, err := env.ExecuteActivity(acts.PreferenceRetrievalActivity, preference.RetrievalInput{
		WorkspaceID: "ws-1",
		UserID:      "u-1",
		Intent:      "schedule meeting",
	})
	require.NoError(t, err, "missing retriever should not error")
	var ctx preference.PreferenceContext
	require.NoError(t, val.Get(&ctx))
	assert.Empty(t, ctx.Facts)
}

func TestPreferenceExtractFact_EmailCategory(t *testing.T) {
	sig := preference.PreferenceSignal{
		WorkspaceID:       "ws-1",
		UserID:            "u-1",
		WorkflowRunID:     "run-1",
		OriginalResponse:  "Dear Alice, I hope this email finds you well. I wanted to follow up on our conversation from last week about the project timeline and deliverables.",
		CorrectedResponse: "Alice — quick update: on track for April 15.",
		OriginalIntent:    "send email to Alice",
		SignalType:        "edit",
		ToolKeyUsed:       "email.send",
	}
	fact := preference.ExtractFact(sig)
	assert.Equal(t, "email_style", fact.Category)
	assert.Equal(t, 0.95, fact.Confidence)
	assert.Contains(t, fact.Preference, "concise")
}

func TestPreferenceFormatForLLM_EmptyFacts(t *testing.T) {
	result := preference.FormatForLLM(nil)
	assert.Empty(t, result)
}

func TestPreferenceFormatForLLM_WithFacts(t *testing.T) {
	facts := []preference.PreferenceFact{
		{Category: "email_style", Preference: "User prefers short emails under 3 sentences"},
		{Category: "scheduling", Preference: "User prefers meetings in afternoon only"},
	}
	result := preference.FormatForLLM(facts)
	assert.Contains(t, result, "USER PREFERENCE CONTEXT")
	assert.Contains(t, result, "email_style")
	assert.Contains(t, result, "scheduling")
}
