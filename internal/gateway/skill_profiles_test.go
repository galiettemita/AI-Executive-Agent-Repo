package gateway

import "testing"

func TestGatewaySkillProfilesCoverage(t *testing.T) {
	t.Parallel()

	profiles := GatewaySkillProfiles()
	if len(profiles) != 8 {
		t.Fatalf("unexpected gateway skill profile count: %d", len(profiles))
	}

	requiredSkills := []string{
		"asr",
		"gemini-stt",
		"openai-tts",
		"sag",
		"voice-wake-say",
		"whatsapp-styling-guide",
		"vocal-chat",
		"autoresponder",
	}
	for _, skill := range requiredSkills {
		profile, ok := profiles[skill]
		if !ok {
			t.Fatalf("missing gateway skill profile: %s", skill)
		}
		if profile.LatencyBudgetMs <= 0 {
			t.Fatalf("missing latency budget for %s: %+v", skill, profile)
		}
		if profile.WhyGatewayNotHands == "" {
			t.Fatalf("missing gateway rationale for %s: %+v", skill, profile)
		}
	}

	if !profiles["autoresponder"].DelegatesToBrain {
		t.Fatalf("autoresponder must delegate to brain")
	}
}
