package cognitive

import (
	"strings"
	"testing"
)

func TestUpdateModelAndInferKnowledge(t *testing.T) {
	t.Parallel()

	svc := NewTheoryOfMindService()

	svc.UpdateModel("ws1", "u1", "golang", true)
	svc.UpdateModel("ws1", "u1", "golang", true)
	svc.UpdateModel("ws1", "u1", "golang", true)

	familiarity := svc.InferKnowledge("ws1", "u1", "golang")
	if familiarity <= 0 {
		t.Fatalf("expected positive familiarity, got %f", familiarity)
	}

	// Unknown topic.
	unknown := svc.InferKnowledge("ws1", "u1", "quantum_physics")
	if unknown != 0 {
		t.Fatalf("expected 0 for unknown topic, got %f", unknown)
	}
}

func TestUpdateModelDecreaseFamiliarity(t *testing.T) {
	t.Parallel()

	svc := NewTheoryOfMindService()
	svc.UpdateModel("ws1", "u1", "python", true)
	after1 := svc.InferKnowledge("ws1", "u1", "python")

	svc.UpdateModel("ws1", "u1", "python", false)
	after2 := svc.InferKnowledge("ws1", "u1", "python")

	if after2 >= after1 {
		t.Fatalf("expected familiarity to decrease after negative signal, was %f now %f", after1, after2)
	}
}

func TestShouldExplain(t *testing.T) {
	t.Parallel()

	svc := NewTheoryOfMindService()

	// Unknown topic should need explanation.
	if !svc.ShouldExplain("ws1", "u1", "unknown_topic") {
		t.Fatal("expected should explain for unknown topic")
	}

	// Build up familiarity.
	for i := 0; i < 20; i++ {
		svc.UpdateModel("ws1", "u1", "familiar_topic", true)
	}
	if svc.ShouldExplain("ws1", "u1", "familiar_topic") {
		t.Fatal("expected should not explain for familiar topic")
	}
}

func TestAdaptExplanation(t *testing.T) {
	t.Parallel()

	svc := NewTheoryOfMindService()

	// Expert user gets brief explanations.
	for i := 0; i < 30; i++ {
		svc.UpdateModel("ws1", "expert", "topic_a", true)
	}

	content := "This is a detailed explanation. It has multiple sentences. And keeps going with more details about the subject."
	adapted := svc.AdaptExplanation("ws1", "expert", content)
	if len(adapted) >= len(content) {
		t.Fatal("expected brief adaptation to be shorter")
	}

	// Beginner user gets detailed explanations.
	svc.UpdateModel("ws1", "beginner", "topic_b", false)
	adapted = svc.AdaptExplanation("ws1", "beginner", content)
	if !strings.Contains(adapted, "Additional context") {
		t.Fatal("expected detailed adaptation with additional context")
	}

	// Unknown user gets original content.
	orig := svc.AdaptExplanation("ws1", "unknown_user", content)
	if orig != content {
		t.Fatal("expected original content for unknown user")
	}
}

func TestDetectKnowledgeGap(t *testing.T) {
	t.Parallel()

	svc := NewTheoryOfMindService()

	// Build up knowledge on some topics.
	for i := 0; i < 20; i++ {
		svc.UpdateModel("ws1", "u1", "known_topic", true)
	}

	gaps := svc.DetectKnowledgeGap("ws1", "u1", []string{"known_topic", "unknown_topic", "another_unknown"})
	if len(gaps) != 2 {
		t.Fatalf("expected 2 gaps, got %d", len(gaps))
	}
}
