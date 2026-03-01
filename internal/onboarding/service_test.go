package onboarding

import (
	"strings"
	"testing"
)

func fixtureAnswers() map[string]map[string]string {
	return map[string]map[string]string{
		"operator_profile_intake_v1": {
			"OPI-001": "Taylor Smith",
			"OPI-002": "CTO at Brevio",
			"OPI-003": "America/New_York",
			"OPI-004": "09:00-18:00",
			"OPI-005": "taylor@brevio.app",
			"OPI-006": "Google Calendar",
			"OPI-007": "Slack, WhatsApp",
			"OPI-008": "Linear, Asana",
			"OPI-009": "42",
			"OPI-010": "SaaS",
		},
		"behavior_policy_calibration_v1": {
			"BPC-001": "Suggest best option",
			"BPC-002": "Always ask for approval",
			"BPC-003": "Always confirm",
			"BPC-004": "Proactively notify me",
			"BPC-005": "Ask me each time",
			"BPC-006": "Brief",
			"BPC-007": "Yes",
			"BPC-008": "Ask first",
		},
		"codebase_map_ingestion_v1": {
			"CBI-001": "https://github.com/brevio/brevio",
			"CBI-002": "Go",
			"CBI-003": "EKS",
			"CBI-004": "trunk-based",
			"CBI-005": "GitHub Actions",
		},
		"system_map_ingestion_v1": {
			"SMI-001": "AWS",
			"SMI-002": "high",
			"SMI-003": "Internal admin tools",
			"SMI-004": "Salesforce",
			"SMI-005": "SOC2",
		},
	}
}

func TestCompleteAllStagesWithFixtureAnswers(t *testing.T) {
	t.Parallel()

	svc := NewService()
	workspaceID := NewWorkspaceID()
	answers := fixtureAnswers()

	if err := svc.CompleteOnboarding(workspaceID, answers); err != nil {
		t.Fatalf("complete onboarding: %v", err)
	}

	profile, persona, policy, err := svc.WorkspaceState(workspaceID)
	if err != nil {
		t.Fatalf("workspace state: %v", err)
	}
	if profile.VersionInt < 1 || persona.VersionInt < 1 || policy.VersionInt < 1 {
		t.Fatal("expected all version_int values >= 1")
	}
	if len(profile.Dimensions) != 13 {
		t.Fatalf("expected 13 profile dimensions, got %d", len(profile.Dimensions))
	}
	if len(policy.Policy) != 10 {
		t.Fatalf("expected 10 behavior-policy dimensions, got %d", len(policy.Policy))
	}
	if profile.Dimensions["role"] != "CTO at Brevio" {
		t.Fatalf("unexpected profile role: %s", profile.Dimensions["role"])
	}
	if persona.Persona["tone"] != "Brief" {
		t.Fatalf("unexpected persona tone: %s", persona.Persona["tone"])
	}
	if policy.Policy["write_actions"] != "Always ask for approval" {
		t.Fatalf("unexpected behavior policy write_actions: %s", policy.Policy["write_actions"])
	}
	followups := svc.ListAdaptiveQuestions(workspaceID)
	if len(followups) == 0 {
		t.Fatal("expected adaptive followup questions after onboarding completion")
	}
}

func TestRunStageReplayLockedExtraction(t *testing.T) {
	t.Parallel()

	svc := NewService()
	workspaceID := NewWorkspaceID()
	answers := fixtureAnswers()["behavior_policy_calibration_v1"]

	first, err := svc.RunStage(workspaceID, "behavior_policy_calibration_v1", answers)
	if err != nil {
		t.Fatalf("run stage first pass: %v", err)
	}
	second, err := svc.RunStage(workspaceID, "behavior_policy_calibration_v1", answers)
	if err != nil {
		t.Fatalf("run stage second pass: %v", err)
	}
	if len(first.Extracted) != len(second.Extracted) {
		t.Fatalf("replay extraction mismatch: %v vs %v", first.Extracted, second.Extracted)
	}
	for key, value := range first.Extracted {
		if second.Extracted[key] != value {
			t.Fatalf("replay output mismatch for key %s: %s vs %s", key, value, second.Extracted[key])
		}
	}
}

func TestQuestionSetHasFixedRequiredQuestions(t *testing.T) {
	t.Parallel()

	svc := NewService()
	questions, err := svc.QuestionSet("operator_profile_intake_v1")
	if err != nil {
		t.Fatalf("question set: %v", err)
	}
	if len(questions) < 8 {
		t.Fatalf("expected fixed operator question set, got %d", len(questions))
	}
}

func TestAdaptiveDiscoveryFollowupLifecycle(t *testing.T) {
	t.Parallel()

	svc := NewService()
	workspaceID := NewWorkspaceID()
	rule := svc.UpsertFollowupRule(workspaceID, FollowupRule{
		Trigger:  "onboarding_completed",
		Question: "What recurring report should be fully automated first?",
		Status:   "active",
	})
	if rule.RuleID == "" {
		t.Fatal("expected persisted followup rule id")
	}

	answers := fixtureAnswers()
	answers["behavior_policy_calibration_v1"]["BPC-002"] = "Always ask for approval"
	answers["system_map_ingestion_v1"]["SMI-002"] = "high"
	if err := svc.CompleteOnboarding(workspaceID, answers); err != nil {
		t.Fatalf("complete onboarding with adaptive trigger answers: %v", err)
	}

	followups := svc.ListAdaptiveQuestions(workspaceID)
	if len(followups) < 2 {
		t.Fatalf("expected multiple adaptive questions, got %+v", followups)
	}

	answered, ok, err := svc.AnswerAdaptiveQuestion(workspaceID, followups[0].FollowupID, "Automate morning KPI summary first.")
	if err != nil {
		t.Fatalf("answer followup: %v", err)
	}
	if !ok {
		t.Fatalf("expected followup answer acceptance")
	}
	if answered.Status != "answered" {
		t.Fatalf("unexpected answered followup status: %+v", answered)
	}
}

func TestConnectionTemplatesContainOnboardingButtons(t *testing.T) {
	t.Parallel()

	svc := NewService()
	templates := svc.ListConnectionTemplates("whatsapp")
	if len(templates) == 0 {
		t.Fatal("expected whatsapp onboarding templates")
	}
	template := templates[0]
	if template.TemplateKey == "" || template.Channel != "whatsapp" {
		t.Fatalf("unexpected template metadata: %+v", template)
	}
	if len(template.Buttons) < 2 {
		t.Fatalf("expected onboarding template buttons, got %+v", template.Buttons)
	}
	for _, button := range template.Buttons {
		if strings.TrimSpace(button.ButtonID) == "" || strings.TrimSpace(button.Label) == "" || strings.TrimSpace(button.Action) == "" {
			t.Fatalf("expected non-empty onboarding button fields: %+v", button)
		}
	}
}

func TestRenderConnectionTemplateSubstitutesParams(t *testing.T) {
	t.Parallel()

	svc := NewService()
	rendered, err := svc.RenderConnectionTemplate("whatsapp", "ecosystem_detect_v1", map[string]string{
		"app_name":       "Google Calendar",
		"ecosystem_hint": "your team schedules many recurring meetings",
	})
	if err != nil {
		t.Fatalf("render connection template: %v", err)
	}
	if strings.Contains(rendered.Title, "{{") || strings.Contains(rendered.Body, "{{") {
		t.Fatalf("expected rendered template placeholders to be replaced: %+v", rendered)
	}
	if !strings.Contains(rendered.Body, "Google Calendar") {
		t.Fatalf("expected rendered body to include app_name substitution: %+v", rendered)
	}
}
