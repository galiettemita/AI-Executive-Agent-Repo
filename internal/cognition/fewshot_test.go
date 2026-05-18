package cognition

import (
	"strings"
	"testing"
)

func TestAddExemplar(t *testing.T) {
	b := NewFewShotBuilder()
	err := b.AddExemplar(Exemplar{Input: "hello", Output: "hi", Domain: "greetings", Quality: 0.9})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestAddExemplarValidation(t *testing.T) {
	b := NewFewShotBuilder()
	err := b.AddExemplar(Exemplar{Input: "", Output: "hi"})
	if err == nil {
		t.Fatal("expected error for empty input")
	}
	err = b.AddExemplar(Exemplar{Input: "hello", Output: ""})
	if err == nil {
		t.Fatal("expected error for empty output")
	}
}

func TestSelectExemplars(t *testing.T) {
	b := NewFewShotBuilder()
	_ = b.AddExemplar(Exemplar{Input: "translate hello to french", Output: "bonjour", Domain: "translation", Quality: 0.9})
	_ = b.AddExemplar(Exemplar{Input: "translate goodbye to french", Output: "au revoir", Domain: "translation", Quality: 0.8})
	_ = b.AddExemplar(Exemplar{Input: "code a function in go", Output: "func main() {}", Domain: "coding", Quality: 0.7})

	selected := b.SelectExemplars("translate thanks to french", "translation", 2)
	if len(selected) != 2 {
		t.Fatalf("expected 2 exemplars, got %d", len(selected))
	}
}

func TestSelectExemplarsDomainFilter(t *testing.T) {
	b := NewFewShotBuilder()
	_ = b.AddExemplar(Exemplar{Input: "test", Output: "out", Domain: "domain_a", Quality: 0.9})
	_ = b.AddExemplar(Exemplar{Input: "test2", Output: "out2", Domain: "domain_b", Quality: 0.8})

	selected := b.SelectExemplars("test", "domain_a", 5)
	if len(selected) != 1 {
		t.Fatalf("expected 1 exemplar from domain_a, got %d", len(selected))
	}
}

func TestSelectExemplarsEmpty(t *testing.T) {
	b := NewFewShotBuilder()
	selected := b.SelectExemplars("test", "", 5)
	if len(selected) != 0 {
		t.Fatalf("expected 0 for empty pool, got %d", len(selected))
	}
}

func TestBuildPrompt(t *testing.T) {
	b := NewFewShotBuilder()
	exemplars := []Exemplar{
		{Input: "2+2", Output: "4"},
		{Input: "3+3", Output: "6"},
	}

	prompt := b.BuildPrompt("You are a calculator.", exemplars, "5+5")
	if !strings.Contains(prompt, "You are a calculator.") {
		t.Fatal("expected system prompt in output")
	}
	if !strings.Contains(prompt, "Example 1:") {
		t.Fatal("expected examples in output")
	}
	if !strings.Contains(prompt, "5+5") {
		t.Fatal("expected query in output")
	}
}

func TestKeywordSimilarityFunc(t *testing.T) {
	sim := KeywordSimilarity("the quick brown fox", "the quick red fox")
	if sim <= 0 || sim >= 1 {
		t.Fatalf("expected similarity between 0 and 1, got %f", sim)
	}

	simSame := KeywordSimilarity("hello world", "hello world")
	if simSame != 1.0 {
		t.Fatalf("expected similarity 1.0 for identical strings, got %f", simSame)
	}
}

func TestMMRDiversity(t *testing.T) {
	b := NewFewShotBuilder()
	// Add 3 very similar exemplars and 1 diverse one
	_ = b.AddExemplar(Exemplar{Input: "translate hello", Output: "bonjour", Domain: "t", Quality: 0.9})
	_ = b.AddExemplar(Exemplar{Input: "translate hello please", Output: "bonjour svp", Domain: "t", Quality: 0.8})
	_ = b.AddExemplar(Exemplar{Input: "translate hello now", Output: "bonjour maintenant", Domain: "t", Quality: 0.7})
	_ = b.AddExemplar(Exemplar{Input: "code function algorithm", Output: "func algo()", Domain: "t", Quality: 0.6})

	selected := b.SelectExemplars("translate hello", "t", 2)
	if len(selected) != 2 {
		t.Fatalf("expected 2, got %d", len(selected))
	}
	// The MMR algorithm should pick the most relevant first, then diversify
}
