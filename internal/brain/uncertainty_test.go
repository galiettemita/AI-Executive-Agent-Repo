package brain

import "testing"

func TestUncertaintyHighConfidence(t *testing.T) {
	t.Parallel()

	us := NewUncertaintyService()
	level := us.Quantify(0.95)
	if level.Label != "high_confidence" {
		t.Fatalf("expected high_confidence, got %s", level.Label)
	}
	if level.ShouldQualify {
		t.Fatal("expected no qualifier for high confidence")
	}
	if level.QualifierPhrase != "" {
		t.Fatalf("expected empty qualifier, got %s", level.QualifierPhrase)
	}
}

func TestUncertaintyModerate(t *testing.T) {
	t.Parallel()

	us := NewUncertaintyService()
	level := us.Quantify(0.75)
	if level.Label != "moderate" {
		t.Fatalf("expected moderate, got %s", level.Label)
	}
	if !level.ShouldQualify {
		t.Fatal("expected qualifier for moderate confidence")
	}
	if level.QualifierPhrase != "I believe..." {
		t.Fatalf("expected 'I believe...', got %s", level.QualifierPhrase)
	}
}

func TestUncertaintyLow(t *testing.T) {
	t.Parallel()

	us := NewUncertaintyService()
	level := us.Quantify(0.55)
	if level.Label != "low" {
		t.Fatalf("expected low, got %s", level.Label)
	}
	if !level.ShouldQualify {
		t.Fatal("expected qualifier for low confidence")
	}
	if level.QualifierPhrase != "I'm not certain but..." {
		t.Fatalf("expected 'I'm not certain but...', got %s", level.QualifierPhrase)
	}
}

func TestUncertaintyVeryLow(t *testing.T) {
	t.Parallel()

	us := NewUncertaintyService()
	level := us.Quantify(0.3)
	if level.Label != "very_low" {
		t.Fatalf("expected very_low, got %s", level.Label)
	}
	if !level.ShouldQualify {
		t.Fatal("expected qualifier for very low confidence")
	}
	if level.QualifierPhrase != "You may want to verify..." {
		t.Fatalf("expected 'You may want to verify...', got %s", level.QualifierPhrase)
	}
}

func TestUncertaintyBoundaryValues(t *testing.T) {
	t.Parallel()

	us := NewUncertaintyService()

	// Exact boundary at 0.9
	level := us.Quantify(0.9)
	if level.Label != "high_confidence" {
		t.Fatalf("expected high_confidence at 0.9, got %s", level.Label)
	}

	// Exact boundary at 0.7
	level = us.Quantify(0.7)
	if level.Label != "moderate" {
		t.Fatalf("expected moderate at 0.7, got %s", level.Label)
	}

	// Exact boundary at 0.5
	level = us.Quantify(0.5)
	if level.Label != "low" {
		t.Fatalf("expected low at 0.5, got %s", level.Label)
	}

	// Clamped negative
	level = us.Quantify(-0.5)
	if level.Score != 0 {
		t.Fatalf("expected clamped score 0, got %f", level.Score)
	}

	// Clamped above 1
	level = us.Quantify(1.5)
	if level.Score != 1 {
		t.Fatalf("expected clamped score 1, got %f", level.Score)
	}
}
