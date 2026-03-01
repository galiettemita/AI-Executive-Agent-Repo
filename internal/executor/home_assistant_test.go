package executor

import (
	"reflect"
	"testing"
	"time"
)

func TestHomeAssistantPolicyHelpers(t *testing.T) {
	t.Parallel()

	if HomeAssistantRateLimitPerMinute() != 30 {
		t.Fatalf("unexpected home assistant rate limit: %d", HomeAssistantRateLimitPerMinute())
	}
	if HomeAssistantEntityCacheRefreshInterval() != 60*time.Second {
		t.Fatalf("unexpected entity cache refresh interval: %s", HomeAssistantEntityCacheRefreshInterval())
	}
	if !CanRunEnvironmentProactiveAction(true, "A2") {
		t.Fatal("expected proactive action for A2+ with consent")
	}
	if CanRunEnvironmentProactiveAction(false, "A4") {
		t.Fatal("did not expect proactive action without consent")
	}
	if got := NormalizeEnvironmentSignalType("MOTION"); got != "motion" {
		t.Fatalf("unexpected normalized signal type: %s", got)
	}
}

func TestFilterAllowedHomeAssistantActions(t *testing.T) {
	t.Parallel()

	filtered := FilterAllowedHomeAssistantActions([]string{"scene.turn_on", "bad.action", "light.turn_on"})
	expected := []string{"light.turn_on", "scene.turn_on"}
	if !reflect.DeepEqual(filtered, expected) {
		t.Fatalf("unexpected filtered actions: got=%v want=%v", filtered, expected)
	}
}
