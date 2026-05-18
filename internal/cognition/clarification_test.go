package cognition

import (
	"strings"
	"testing"
)

func TestGenerateCandidates(t *testing.T) {
	s := NewClarificationService()
	candidates := s.GenerateCandidates("deploy the thing", []string{"which environment?", "when?"})
	if len(candidates) != 2 {
		t.Fatalf("expected 2 candidates, got %d", len(candidates))
	}
}

func TestGenerateCandidatesEmpty(t *testing.T) {
	s := NewClarificationService()
	candidates := s.GenerateCandidates("context", []string{})
	if len(candidates) != 0 {
		t.Fatalf("expected 0 candidates, got %d", len(candidates))
	}
}

func TestGenerateCandidatesWithContextQuestion(t *testing.T) {
	s := NewClarificationService()
	ambiguities := []string{"a", "b", "c"}
	candidates := s.GenerateCandidates("some context", ambiguities)
	// Should include context-level question since ambiguities > 2
	if len(candidates) != 4 { // 3 ambiguities + 1 context question
		t.Fatalf("expected 4 candidates, got %d", len(candidates))
	}
}

func TestSelectOptimal(t *testing.T) {
	s := NewClarificationService()
	candidates := []ClarificationCandidate{
		{Question: "low", InformationGain: 0.3, Urgency: 0.5},
		{Question: "high", InformationGain: 0.9, Urgency: 0.9},
		{Question: "medium", InformationGain: 0.5, Urgency: 0.7},
	}
	best := s.SelectOptimal(candidates)
	if best.Question != "high" {
		t.Fatalf("expected 'high', got %s", best.Question)
	}
}

func TestSelectOptimalEmpty(t *testing.T) {
	s := NewClarificationService()
	best := s.SelectOptimal(nil)
	if best != nil {
		t.Fatal("expected nil for empty candidates")
	}
}

func TestShouldClarify(t *testing.T) {
	s := NewClarificationService()
	if !s.ShouldClarify(0.4, 1) {
		t.Fatal("expected clarification for low confidence")
	}
	if !s.ShouldClarify(0.8, 3) {
		t.Fatal("expected clarification for many ambiguities")
	}
	if s.ShouldClarify(0.8, 1) {
		t.Fatal("did not expect clarification")
	}
}

func TestFormatClarification(t *testing.T) {
	s := NewClarificationService()
	c := &ClarificationCandidate{
		Question:        "What do you mean?",
		InformationGain: 0.8,
		Category:        "general",
	}
	formatted := s.FormatClarification(c)
	if !strings.Contains(formatted, "GENERAL") {
		t.Fatalf("expected category in formatted output, got %s", formatted)
	}
}

func TestFormatClarificationNil(t *testing.T) {
	s := NewClarificationService()
	if s.FormatClarification(nil) != "" {
		t.Fatal("expected empty string for nil candidate")
	}
}

func TestCategorizeAmbiguity(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"when should it run?", "temporal"},
		{"who is responsible?", "identity"},
		{"how to deploy?", "method"},
		{"what version?", "specification"},
		{"something else", "general"},
	}
	for _, tc := range tests {
		got := categorizeAmbiguity(tc.input)
		if got != tc.expected {
			t.Errorf("categorizeAmbiguity(%q) = %s, want %s", tc.input, got, tc.expected)
		}
	}
}
