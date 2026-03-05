package contracts

import (
	"testing"

	"github.com/brevio/brevio/internal/gateway"
)

func TestGatewaySkillProfileClosure(t *testing.T) {
	t.Parallel()

	profiles := gateway.GatewaySkillProfiles()
	if len(profiles) != 8 {
		t.Fatalf("expected 8 gateway skill profiles, got %d", len(profiles))
	}

	expectedBudgets := map[string]int{
		"asr":                    3000,
		"gemini-stt":             5000,
		"openai-tts":             2000,
		"sag":                    3000,
		"voice-wake-say":         500,
		"whatsapp-styling-guide": 10,
		"vocal-chat":             5000,
		"autoresponder":          8000,
	}
	for skillID, expected := range expectedBudgets {
		profile, ok := profiles[skillID]
		if !ok {
			t.Fatalf("missing gateway skill profile: %s", skillID)
		}
		if profile.LatencyBudgetMs != expected {
			t.Fatalf("unexpected latency budget for %s: got=%d want=%d", skillID, profile.LatencyBudgetMs, expected)
		}
	}

	autoresponder := profiles["autoresponder"]
	if !autoresponder.DelegatesToBrain {
		t.Fatalf("autoresponder must be marked as gateway-brain hybrid delegation")
	}
	if autoresponder.ExternalAPICalled == "" {
		t.Fatalf("autoresponder external API field must be populated")
	}
}
