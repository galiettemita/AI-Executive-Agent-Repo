package cognitive

import (
	"testing"
)

func TestGenerateClarificationsShortInput(t *testing.T) {
	t.Parallel()

	cs := NewClarificationService()
	candidates := cs.GenerateClarifications("ws1", "help", nil)

	if len(candidates) == 0 {
		t.Fatal("expected clarification candidates for short input")
	}

	// Short input should generate intent clarifications.
	hasIntent := false
	for _, c := range candidates {
		if c.Category == "intent" {
			hasIntent = true
			break
		}
	}
	if !hasIntent {
		t.Fatal("expected intent clarifications for short ambiguous input")
	}
}

func TestGenerateClarificationsLongInput(t *testing.T) {
	t.Parallel()

	cs := NewClarificationService()
	ctx := map[string]any{"deadline": "tomorrow", "scope": "full"}
	candidates := cs.GenerateClarifications("ws1", "I need help with my project that involves building a complete web application for the team", ctx)

	// Long input with context should produce fewer intent clarifications.
	intentCount := 0
	for _, c := range candidates {
		if c.Category == "intent" {
			intentCount++
		}
	}
	// Long input (>7 words) has ambiguityFromLength = 0.2, which is < 0.4, so no intent clarifications.
	if intentCount != 0 {
		t.Fatalf("expected no intent clarifications for long input, got %d", intentCount)
	}
}

func TestRankClarifications(t *testing.T) {
	t.Parallel()

	cs := NewClarificationService()
	candidates := []ClarificationCandidate{
		{Question: "q1", InformationGain: 0.3, Urgency: 0.3, Category: "preference"},
		{Question: "q2", InformationGain: 0.8, Urgency: 0.7, Category: "intent"},
		{Question: "q3", InformationGain: 0.6, Urgency: 0.5, Category: "scope"},
	}

	ranked := cs.RankClarifications(candidates)
	if len(ranked) != 3 {
		t.Fatalf("expected 3 ranked candidates, got %d", len(ranked))
	}
	// q2 has highest score (0.8*0.7=0.56).
	if ranked[0].Question != "q2" {
		t.Fatalf("expected q2 first, got %s", ranked[0].Question)
	}
	// q3 has second highest (0.6*0.5=0.30).
	if ranked[1].Question != "q3" {
		t.Fatalf("expected q3 second, got %s", ranked[1].Question)
	}
}

func TestShouldAskClarification(t *testing.T) {
	t.Parallel()

	cs := NewClarificationService()

	if !cs.ShouldAskClarification(0.8, 0.5) {
		t.Fatal("expected should ask when score > threshold")
	}
	if cs.ShouldAskClarification(0.3, 0.5) {
		t.Fatal("expected should not ask when score < threshold")
	}
	if cs.ShouldAskClarification(0.5, 0.5) {
		t.Fatal("expected should not ask when score == threshold")
	}
}

func TestClarificationsWithQuestionInput(t *testing.T) {
	t.Parallel()

	cs := NewClarificationService()
	// Input with a question mark should suppress preference clarifications.
	candidates := cs.GenerateClarifications("ws1", "how?", nil)

	for _, c := range candidates {
		if c.Category == "preference" {
			t.Fatal("expected no preference clarifications when input contains a question mark")
		}
	}
}
