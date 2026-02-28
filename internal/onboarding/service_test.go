package onboarding

import "testing"

func fixtureAnswers() map[string]map[string]string {
	return map[string]map[string]string{
		"operator_profile_intake_v1": {
			"role":               "CTO",
			"goals":              "Ship features faster",
			"industry":           "SaaS",
			"team_size":          "42",
			"timezone":           "America/New_York",
			"decision_style":     "data-driven",
			"communication_pref": "concise",
			"kpi_primary":        "weekly active users",
		},
		"behavior_policy_calibration_v1": {
			"tone":                "direct",
			"risk_tolerance":      "moderate",
			"autonomy_preference": "A2",
			"approval_threshold":  "critical_only",
			"proactive_mode":      "enabled",
			"notification_window": "09:00-18:00",
			"initiative_level":    "high",
		},
		"codebase_map_ingestion_v1": {
			"repo":             "github.com/brevio/brevio",
			"stack":            "go",
			"planning_horizon": "quarterly",
			"meeting_load":     "medium",
			"focus_mode":       "async_blocks",
		},
		"system_map_ingestion_v1": {
			"integrations":     "slack, github",
			"sla":              "99.9",
			"escalation_path":  "ops-oncall",
			"privacy_mode":     "strict",
			"audit_strictness": "high",
			"delivery_cadence": "weekly",
			"context_budget":   "balanced",
			"write_actions":    "confirm_before_send",
			"language":         "en-US",
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
	if profile.Dimensions["role"] != "CTO" {
		t.Fatalf("unexpected profile role: %s", profile.Dimensions["role"])
	}
	if persona.Persona["tone"] != "direct" {
		t.Fatalf("unexpected persona tone: %s", persona.Persona["tone"])
	}
	if policy.Policy["write_actions"] != "confirm_before_send" {
		t.Fatalf("unexpected behavior policy write_actions: %s", policy.Policy["write_actions"])
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
