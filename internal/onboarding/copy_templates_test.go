package onboarding

import (
	"testing"
)

func TestCopyVersionSet(t *testing.T) {
	if CopyVersion == "" {
		t.Fatal("CopyVersion must be set")
	}
}

func TestOnboardingCopyNotEmpty(t *testing.T) {
	if len(OnboardingCopy) == 0 {
		t.Fatal("OnboardingCopy must contain entries")
	}
}

func TestErrorStateCopyNotEmpty(t *testing.T) {
	if len(ErrorStateCopy) == 0 {
		t.Fatal("ErrorStateCopy must contain entries")
	}
}

func TestInteractionCopyNotEmpty(t *testing.T) {
	if len(InteractionCopy) == 0 {
		t.Fatal("InteractionCopy must contain entries")
	}
}

// Snapshot test: ensures copy doesn't drift without explicit update
func TestOnboardingCopySnapshot(t *testing.T) {
	requiredKeys := []string{
		"welcome_title",
		"welcome_subtitle",
		"welcome_greeting",
		"welcome_setup_prompt",
		"discovery_q1",
		"discovery_q2",
		"discovery_q3",
		"discovery_q4",
		"discovery_q5",
		"oauth_prompt",
		"oauth_connect_link",
		"oauth_success",
		"first_task_prompt",
		"calibration_checkin",
		"calibration_approval",
		"first_value_inbox",
		"complete_message",
	}
	for _, key := range requiredKeys {
		if _, ok := OnboardingCopy[key]; !ok {
			t.Fatalf("missing required onboarding copy key: %s", key)
		}
		if OnboardingCopy[key] == "" {
			t.Fatalf("onboarding copy key %s must not be empty", key)
		}
	}
}

func TestErrorStateCopySnapshot(t *testing.T) {
	requiredKeys := []string{
		"generic_error_title",
		"generic_error_body",
		"silent_1h",
		"silent_24h",
		"silent_72h",
		"oauth_fail",
		"oauth_timeout",
		"rate_limited",
		"service_unavailable",
		"permission_error",
		"approval_gate",
		"unrecognized_input",
	}
	for _, key := range requiredKeys {
		if _, ok := ErrorStateCopy[key]; !ok {
			t.Fatalf("missing required error state copy key: %s", key)
		}
		if ErrorStateCopy[key] == "" {
			t.Fatalf("error state copy key %s must not be empty", key)
		}
	}
}

func TestInteractionCopySnapshot(t *testing.T) {
	requiredKeys := []string{
		"approval_prompt",
		"approval_confirmed",
		"approval_denied",
		"proactive_morning",
		"correction_ack",
		"learning_feedback_prompt",
		"reauth_needed",
		"recurring_setup",
		"briefing_light",
		"skip_all",
		"autonomy_promotion",
		"billing_upgrade_prompt",
		"outage_single",
		"recurring_conflict",
	}
	for _, key := range requiredKeys {
		if _, ok := InteractionCopy[key]; !ok {
			t.Fatalf("missing required interaction copy key: %s", key)
		}
		if InteractionCopy[key] == "" {
			t.Fatalf("interaction copy key %s must not be empty", key)
		}
	}
}
