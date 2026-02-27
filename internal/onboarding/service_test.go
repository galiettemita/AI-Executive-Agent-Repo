package onboarding

import "testing"

func TestCompleteAllStagesWithFixtureAnswers(t *testing.T) {
	t.Parallel()

	svc := NewService()
	workspaceID := NewWorkspaceID()
	answers := map[string]map[string]string{
		"operator_profile_intake_v1": {
			"role":  "CTO",
			"goals": "Ship features faster",
		},
		"behavior_policy_calibration_v1": {
			"tone": "concise",
			"risk": "moderate",
		},
		"codebase_map_ingestion_v1": {
			"repo":  "github.com/brevio/brevio",
			"stack": "go",
		},
		"system_map_ingestion_v1": {
			"integrations": "slack, github",
			"sla":          "99.9",
		},
	}

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
	if profile.Dimensions["role"] != "CTO" {
		t.Fatalf("unexpected profile role: %s", profile.Dimensions["role"])
	}
	if persona.Persona["tone"] != "concise" {
		t.Fatalf("unexpected persona tone: %s", persona.Persona["tone"])
	}
	if policy.Policy["integrations"] == "" {
		t.Fatal("expected behavior policy integrations to be set")
	}
}
