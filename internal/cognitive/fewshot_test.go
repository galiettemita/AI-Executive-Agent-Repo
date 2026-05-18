package cognitive

import (
	"strings"
	"testing"
)

func TestAddExemplarValidation(t *testing.T) {
	t.Parallel()

	fb := NewFewShotBuilder()

	err := fb.AddExemplar(Exemplar{Input: "hello", Output: "world", Task: "greet", Quality: 0.9})
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	err = fb.AddExemplar(Exemplar{Input: "", Output: "world", Task: "greet"})
	if err == nil {
		t.Fatal("expected error for empty input")
	}

	err = fb.AddExemplar(Exemplar{Input: "hello", Output: "", Task: "greet"})
	if err == nil {
		t.Fatal("expected error for empty output")
	}

	err = fb.AddExemplar(Exemplar{Input: "hello", Output: "world", Task: ""})
	if err == nil {
		t.Fatal("expected error for empty task")
	}
}

func TestSelectExemplarsMMR(t *testing.T) {
	t.Parallel()

	fb := NewFewShotBuilder()
	fb.AddExemplar(Exemplar{Input: "translate hello", Output: "hola", Task: "translate", Quality: 0.9})
	fb.AddExemplar(Exemplar{Input: "translate goodbye", Output: "adios", Task: "translate", Quality: 0.8})
	fb.AddExemplar(Exemplar{Input: "translate thanks", Output: "gracias", Task: "translate", Quality: 0.7})
	fb.AddExemplar(Exemplar{Input: "summarize text", Output: "summary", Task: "summarize", Quality: 0.9})

	selected := fb.SelectExemplars("translate", "translate hello world", 2)
	if len(selected) != 2 {
		t.Fatalf("expected 2 selected exemplars, got %d", len(selected))
	}

	// Should not include exemplars from other tasks.
	for _, s := range selected {
		if s.Task != "translate" {
			t.Fatalf("expected translate task, got %s", s.Task)
		}
	}
}

func TestSelectExemplarsNoMatch(t *testing.T) {
	t.Parallel()

	fb := NewFewShotBuilder()
	fb.AddExemplar(Exemplar{Input: "hello", Output: "world", Task: "greet", Quality: 0.9})

	selected := fb.SelectExemplars("nonexistent_task", "input", 3)
	if selected != nil {
		t.Fatalf("expected nil for no matching task, got %d results", len(selected))
	}
}

func TestBuildPrompt(t *testing.T) {
	t.Parallel()

	fb := NewFewShotBuilder()
	fb.AddExemplar(Exemplar{Input: "2+2", Output: "4", Task: "math", Quality: 0.9})
	fb.AddExemplar(Exemplar{Input: "3+3", Output: "6", Task: "math", Quality: 0.8})

	prompt := fb.BuildPrompt("math", "5+5", 2)

	if !strings.Contains(prompt, "Task: math") {
		t.Fatal("expected prompt to contain task header")
	}
	if !strings.Contains(prompt, "Example 1:") {
		t.Fatal("expected prompt to contain example 1")
	}
	if !strings.Contains(prompt, "Input: 5+5") {
		t.Fatal("expected prompt to contain query input")
	}
	if !strings.Contains(prompt, "Output:") {
		t.Fatal("expected prompt to end with Output:")
	}
}

func TestCosineSimilarityBow(t *testing.T) {
	t.Parallel()

	sim := cosineSimilarityBow("hello world", "hello world")
	if sim < 0.999 || sim > 1.001 {
		t.Fatalf("expected similarity ~1.0 for identical strings, got %f", sim)
	}

	sim = cosineSimilarityBow("hello world", "foo bar")
	if sim != 0.0 {
		t.Fatalf("expected similarity 0.0 for disjoint strings, got %f", sim)
	}

	sim = cosineSimilarityBow("", "hello")
	if sim != 0.0 {
		t.Fatalf("expected similarity 0.0 for empty string, got %f", sim)
	}
}
