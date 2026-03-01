package gateway

import "testing"

func TestResolveInteractiveIntentWithPendingOptions(t *testing.T) {
	t.Parallel()

	options := []string{"slot_9am", "slot_10am", "slot_11am"}

	intent, option, matched := ResolveInteractiveIntentWithPendingOptions("OPTION_INDEX:2", options)
	if intent != string(IntentOption) || option != "slot_10am" || !matched {
		t.Fatalf("unexpected option-index resolution: intent=%s option=%s matched=%v", intent, option, matched)
	}

	intent, option, matched = ResolveInteractiveIntentWithPendingOptions("OPTION:slot_11am", options)
	if intent != string(IntentOption) || option != "slot_11am" || !matched {
		t.Fatalf("unexpected option-id resolution: intent=%s option=%s matched=%v", intent, option, matched)
	}

	intent, option, matched = ResolveInteractiveIntentWithPendingOptions("APPROVE", options)
	if intent != string(IntentApprove) || option != "" || !matched {
		t.Fatalf("unexpected approve resolution: intent=%s option=%s matched=%v", intent, option, matched)
	}
}
