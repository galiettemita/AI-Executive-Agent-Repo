package cognition

import (
	"testing"
)

func TestUpdateModel(t *testing.T) {
	s := NewTheoryOfMindService()
	err := s.UpdateModel("ws1", "u1", "golang", 0.8)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	model, err := s.GetModel("ws1", "u1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if model.KnownTopics["golang"] != 0.8 {
		t.Fatalf("expected proficiency 0.8, got %f", model.KnownTopics["golang"])
	}
}

func TestUpdateModelValidation(t *testing.T) {
	s := NewTheoryOfMindService()
	err := s.UpdateModel("", "u1", "go", 0.5)
	if err == nil {
		t.Fatal("expected error for empty workspace")
	}
	err = s.UpdateModel("ws1", "u1", "go", 1.5)
	if err == nil {
		t.Fatal("expected error for proficiency > 1")
	}
}

func TestExpertiseDomains(t *testing.T) {
	s := NewTheoryOfMindService()
	_ = s.UpdateModel("ws1", "u1", "golang", 0.9)
	_ = s.UpdateModel("ws1", "u1", "python", 0.3)

	model, _ := s.GetModel("ws1", "u1")
	found := false
	for _, d := range model.ExpertiseDomains {
		if d == "golang" {
			found = true
		}
	}
	if !found {
		t.Fatal("expected golang in expertise domains")
	}
}

func TestPreferredComplexity(t *testing.T) {
	s := NewTheoryOfMindService()
	_ = s.UpdateModel("ws1", "u1", "topic1", 0.9)
	_ = s.UpdateModel("ws1", "u1", "topic2", 0.8)

	model, _ := s.GetModel("ws1", "u1")
	if model.PreferredComplexity != "expert" {
		t.Fatalf("expected expert, got %s", model.PreferredComplexity)
	}
}

func TestInferKnowledgeGap(t *testing.T) {
	s := NewTheoryOfMindService()
	_ = s.UpdateModel("ws1", "u1", "golang", 0.9)

	model, _ := s.GetModel("ws1", "u1")
	gap := s.InferKnowledgeGap(model, "golang")
	if gap > 0.2 {
		t.Fatalf("expected small gap for known topic, got %f", gap)
	}

	gapUnknown := s.InferKnowledgeGap(model, "quantum physics")
	if gapUnknown < 0.5 {
		t.Fatalf("expected large gap for unknown topic, got %f", gapUnknown)
	}
}

func TestInferKnowledgeGapNilModel(t *testing.T) {
	s := NewTheoryOfMindService()
	gap := s.InferKnowledgeGap(nil, "anything")
	if gap != 1.0 {
		t.Fatalf("expected gap 1.0 for nil model, got %f", gap)
	}
}

func TestAdaptExplanation(t *testing.T) {
	s := NewTheoryOfMindService()
	_ = s.UpdateModel("ws1", "u1", "topic", 0.2) // simple level

	model, _ := s.GetModel("ws1", "u1")
	adapted := s.AdaptExplanation(model, "This is a complex explanation")
	if adapted == "This is a complex explanation" {
		t.Fatal("expected explanation to be adapted for simple level")
	}
}

func TestPredictUnderstanding(t *testing.T) {
	s := NewTheoryOfMindService()
	_ = s.UpdateModel("ws1", "u1", "golang", 0.9)

	model, _ := s.GetModel("ws1", "u1")
	pred := s.PredictUnderstanding(model, "golang")
	if pred < 0.8 {
		t.Fatalf("expected high understanding for known topic, got %f", pred)
	}

	predUnknown := s.PredictUnderstanding(model, "quantum")
	if predUnknown > 0.5 {
		t.Fatalf("expected low understanding for unknown topic, got %f", predUnknown)
	}
}
