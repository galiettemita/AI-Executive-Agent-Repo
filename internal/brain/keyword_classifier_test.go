package brain

import "testing"

func TestKeywordClassifierAddAndClassify(t *testing.T) {
	t.Parallel()

	kc := NewKeywordClassifier()
	kc.AddRule([]string{"email", "send", "mail"}, "send_email", 0.85)
	kc.AddRule([]string{"schedule", "meeting", "calendar"}, "schedule_event", 0.80)

	intent, confidence := kc.Classify("please send an email to Alice")
	if intent != "send_email" {
		t.Fatalf("expected send_email, got %s", intent)
	}
	if confidence <= 0 {
		t.Fatalf("expected positive confidence, got %f", confidence)
	}
}

func TestKeywordClassifierBestMatch(t *testing.T) {
	t.Parallel()

	kc := NewKeywordClassifier()
	kc.AddRule([]string{"email", "send"}, "send_email", 0.85)
	kc.AddRule([]string{"schedule", "meeting"}, "schedule_event", 0.80)

	intent, _ := kc.Classify("schedule a meeting please")
	if intent != "schedule_event" {
		t.Fatalf("expected schedule_event, got %s", intent)
	}
}

func TestKeywordClassifierNoMatch(t *testing.T) {
	t.Parallel()

	kc := NewKeywordClassifier()
	kc.AddRule([]string{"email", "send"}, "send_email", 0.85)

	intent, confidence := kc.Classify("what is the weather today")
	if intent != "unknown" {
		t.Fatalf("expected unknown, got %s", intent)
	}
	if confidence != 0 {
		t.Fatalf("expected 0 confidence, got %f", confidence)
	}
}

func TestKeywordClassifierEmptyInput(t *testing.T) {
	t.Parallel()

	kc := NewKeywordClassifier()
	intent, confidence := kc.Classify("")
	if intent != "unknown" {
		t.Fatalf("expected unknown, got %s", intent)
	}
	if confidence != 0 {
		t.Fatalf("expected 0 confidence, got %f", confidence)
	}
}

func TestMatchFunction(t *testing.T) {
	t.Parallel()

	if !Match("Send an Email to Bob", []string{"email"}) {
		t.Fatal("expected match for 'email' in input")
	}
	if Match("hello world", []string{"email", "send"}) {
		t.Fatal("expected no match")
	}
}
