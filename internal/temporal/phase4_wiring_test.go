package temporal

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.temporal.io/sdk/testsuite"

	"github.com/brevio/brevio/internal/delegation"
)

// ── EQ Tests ──────────────────────────────────────────────────────────────────

func TestBuildEQSystemPrompt_DefaultWhenNoEQ(t *testing.T) {
	prompt := buildEQSystemPrompt(SynthesizeResponseInput{})
	assert.Contains(t, prompt, "Brevio")
	assert.NotContains(t, prompt, "EQ MODULATION")
}

func TestBuildEQSystemPrompt_InjectsAllDirectives(t *testing.T) {
	prompt := buildEQSystemPrompt(SynthesizeResponseInput{
		EQToneDirective: "be empathetic and warm", EQFormalityLevel: 2,
		EQLengthModifier: 0.5, EQOfferHelp: true,
	})
	assert.Contains(t, prompt, "EQ MODULATION")
	assert.Contains(t, prompt, "empathetic and warm")
	assert.Contains(t, prompt, "concise")
	assert.Contains(t, prompt, "EMPATHY")
	assert.Contains(t, prompt, "friendly")
}

func TestBuildEQSystemPrompt_FormalLevel4(t *testing.T) {
	prompt := buildEQSystemPrompt(SynthesizeResponseInput{EQFormalityLevel: 5})
	assert.Contains(t, prompt, "formal professional")
}

func TestBuildEQSystemPrompt_LongFormat(t *testing.T) {
	prompt := buildEQSystemPrompt(SynthesizeResponseInput{EQLengthModifier: 1.5})
	assert.Contains(t, prompt, "detailed")
}

func TestAddUncertaintyQualifiers_PrependsPhraseOnce(t *testing.T) {
	response := "The meeting is scheduled for 3pm."
	qualified := addUncertaintyQualifiers(response)
	assert.NotEqual(t, response, qualified)
	assert.Greater(t, len(qualified), len(response))
	assert.Equal(t, qualified, addUncertaintyQualifiers(qualified))
}

func TestAddUncertaintyQualifiers_EmptyString(t *testing.T) {
	assert.NotEmpty(t, addUncertaintyQualifiers(""))
}

func TestSynthesizeEQ_NoLLM_FallbackOK(t *testing.T) {
	ts := &testsuite.WorkflowTestSuite{}
	env := ts.NewTestActivityEnvironment()
	acts := NewActivities()
	env.RegisterActivity(acts.SynthesizeResponseActivity)

	val, err := env.ExecuteActivity(acts.SynthesizeResponseActivity, SynthesizeResponseInput{
		MessageID: "msg-001", WorkspaceID: "ws-001",
		EQToneDirective: "be concise", EQFormalityLevel: 3, EQLengthModifier: 0.6,
	})
	require.NoError(t, err)
	var result SynthesizeResponseResult
	require.NoError(t, val.Get(&result))
	assert.NotEmpty(t, result.ResponsePayload)
}

func TestDetectEmotionalState_Urgent(t *testing.T) {
	assert.Equal(t, "stressed_urgent", detectEmotionalState("URGENT: please respond ASAP"))
	assert.Equal(t, "stressed_urgent", detectEmotionalState("this is an emergency"))
}

func TestDetectEmotionalState_Polite(t *testing.T) {
	assert.Equal(t, "polite_request", detectEmotionalState("Could you please schedule a meeting?"))
}

func TestDetectEmotionalState_Correction(t *testing.T) {
	assert.Equal(t, "correction_mode", detectEmotionalState("That was wrong, please cancel it"))
}

func TestDetectEmotionalState_Neutral(t *testing.T) {
	assert.Equal(t, "neutral", detectEmotionalState("schedule a meeting with Alice at 3pm"))
}

// ── Delegation Tests ──────────────────────────────────────────────────────────

func TestAuthorizePlanActivity_SameUser_Allowed(t *testing.T) {
	ts := &testsuite.WorkflowTestSuite{}
	env := ts.NewTestActivityEnvironment()
	acts := NewActivities()
	acts.delegationSvc = delegation.NewService()
	env.RegisterActivity(acts.AuthorizePlanActivity)

	ownerID := "00000000-0000-0000-0000-000000000001"
	val, err := env.ExecuteActivity(acts.AuthorizePlanActivity, AuthorizePlanInput{
		MessageID: "msg-001", WorkspaceID: "00000000-0000-0000-0000-000000000001",
		PlanID: "plan-001", ToolKeys: []string{"email.send"}, RiskLevel: "low",
		RequestingUserID: ownerID, WorkspaceOwnerID: ownerID,
	})
	require.NoError(t, err)
	var result AuthorizePlanResult
	require.NoError(t, val.Get(&result))
	assert.Equal(t, "allow", result.Decision)
}

func TestAuthorizePlanActivity_CrossUser_NoGrant_Denied(t *testing.T) {
	ts := &testsuite.WorkflowTestSuite{}
	env := ts.NewTestActivityEnvironment()
	acts := NewActivities()
	acts.delegationSvc = delegation.NewService()
	env.RegisterActivity(acts.AuthorizePlanActivity)

	val, err := env.ExecuteActivity(acts.AuthorizePlanActivity, AuthorizePlanInput{
		MessageID: "msg-002", WorkspaceID: "00000000-0000-0000-0000-000000000001",
		PlanID: "plan-002", ToolKeys: []string{"calendar.create"}, RiskLevel: "low",
		RequestingUserID: "00000000-0000-0000-0000-000000000002",
		WorkspaceOwnerID: "00000000-0000-0000-0000-000000000001",
	})
	require.NoError(t, err)
	var result AuthorizePlanResult
	require.NoError(t, val.Get(&result))
	assert.Equal(t, "deny", result.Decision)
	assert.Contains(t, result.Reason, "DELEGATION_REQUIRED")
	assert.Contains(t, result.Reason, "calendar.create")
}

func TestDelegation_NilSvc_BackwardsCompatible(t *testing.T) {
	ts := &testsuite.WorkflowTestSuite{}
	env := ts.NewTestActivityEnvironment()
	acts := NewActivities() // nil delegationSvc
	env.RegisterActivity(acts.AuthorizePlanActivity)

	val, err := env.ExecuteActivity(acts.AuthorizePlanActivity, AuthorizePlanInput{
		MessageID: "msg-003", WorkspaceID: "00000000-0000-0000-0000-000000000001",
		PlanID: "plan-003", ToolKeys: []string{"slack.send"}, RiskLevel: "low",
		RequestingUserID: "00000000-0000-0000-0000-000000000099",
		WorkspaceOwnerID: "00000000-0000-0000-0000-000000000001",
	})
	require.NoError(t, err)
	var result AuthorizePlanResult
	require.NoError(t, val.Get(&result))
	assert.Equal(t, "allow", result.Decision)
}

// ── Clarification Tests ──────────────────────────────────────────────────────

func TestClarificationCheck_LowConfidenceWriteOp_Clarifies(t *testing.T) {
	ts := &testsuite.WorkflowTestSuite{}
	env := ts.NewTestActivityEnvironment()
	acts := NewActivities()
	env.RegisterActivity(acts.ClarificationCheckActivity)

	val, err := env.ExecuteActivity(acts.ClarificationCheckActivity, ClarificationCheckInput{
		WorkspaceID:    "ws-1",
		MessageContent: "do the thing",
		IntentKey:      "calendar_write",
		ToolKeys:       []string{"calendar.create"},
		Confidence:     0.3,
	})
	require.NoError(t, err)
	var result ClarificationCheckResult
	require.NoError(t, val.Get(&result))
	assert.True(t, result.NeedsClarification)
	assert.NotEmpty(t, result.Question)
}

func TestClarificationCheck_HighConfidence_NoClarification(t *testing.T) {
	ts := &testsuite.WorkflowTestSuite{}
	env := ts.NewTestActivityEnvironment()
	acts := NewActivities()
	env.RegisterActivity(acts.ClarificationCheckActivity)

	val, err := env.ExecuteActivity(acts.ClarificationCheckActivity, ClarificationCheckInput{
		WorkspaceID:    "ws-1",
		MessageContent: "schedule a meeting with Alice",
		IntentKey:      "calendar_write",
		ToolKeys:       []string{"calendar.create"},
		Confidence:     0.85,
	})
	require.NoError(t, err)
	var result ClarificationCheckResult
	require.NoError(t, val.Get(&result))
	assert.False(t, result.NeedsClarification)
}
