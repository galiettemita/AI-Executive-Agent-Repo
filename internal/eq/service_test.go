package eq

import "testing"

func TestSetAndGetProfile(t *testing.T) {
	t.Parallel()
	s := NewEQService()

	err := s.SetProfile("ws1", CommunicationProfile{
		Formality:  "formal",
		Verbosity:  "concise",
		EmojiUse:   false,
		Humor:      false,
		Directness: 0.8,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	p, err := s.GetProfile("ws1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if p.Formality != "formal" {
		t.Fatalf("expected formal, got %s", p.Formality)
	}
}

func TestSetProfileInvalidFormality(t *testing.T) {
	t.Parallel()
	s := NewEQService()

	err := s.SetProfile("ws1", CommunicationProfile{
		Formality:  "extreme",
		Verbosity:  "balanced",
		Directness: 0.5,
	})
	if err == nil {
		t.Fatal("expected error for invalid formality")
	}
}

func TestSetProfileInvalidDirectness(t *testing.T) {
	t.Parallel()
	s := NewEQService()

	err := s.SetProfile("ws1", CommunicationProfile{
		Formality:  "balanced",
		Verbosity:  "balanced",
		Directness: 1.5,
	})
	if err == nil {
		t.Fatal("expected error for out-of-range directness")
	}
}

func TestGetProfileNotFound(t *testing.T) {
	t.Parallel()
	s := NewEQService()

	_, err := s.GetProfile("nonexistent")
	if err == nil {
		t.Fatal("expected error for missing profile")
	}
}

func TestDetectEmotionPositive(t *testing.T) {
	t.Parallel()
	s := NewEQService()

	state, err := s.DetectEmotion("I am so happy and this is great news!")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if state.DetectedEmotion != "positive" {
		t.Fatalf("expected positive, got %s", state.DetectedEmotion)
	}
	if state.Valence <= 0 {
		t.Fatalf("expected positive valence, got %f", state.Valence)
	}
}

func TestDetectEmotionNegative(t *testing.T) {
	t.Parallel()
	s := NewEQService()

	state, err := s.DetectEmotion("I am frustrated and upset about this terrible outcome")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if state.DetectedEmotion != "negative" {
		t.Fatalf("expected negative, got %s", state.DetectedEmotion)
	}
	if state.Valence >= 0 {
		t.Fatalf("expected negative valence, got %f", state.Valence)
	}
}

func TestDetectEmotionNeutral(t *testing.T) {
	t.Parallel()
	s := NewEQService()

	state, err := s.DetectEmotion("Please send me the report for Q4")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if state.DetectedEmotion != "neutral" {
		t.Fatalf("expected neutral, got %s", state.DetectedEmotion)
	}
}

func TestDetectEmotionUrgent(t *testing.T) {
	t.Parallel()
	s := NewEQService()

	state, err := s.DetectEmotion("This is urgent, I need help immediately!")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if state.Arousal < 0.8 {
		t.Fatalf("expected high arousal for urgent text, got %f", state.Arousal)
	}
}

func TestDetectEmotionEmpty(t *testing.T) {
	t.Parallel()
	s := NewEQService()

	_, err := s.DetectEmotion("")
	if err == nil {
		t.Fatal("expected error for empty text")
	}
}

func TestLogAndGetEmotionalHistory(t *testing.T) {
	t.Parallel()
	s := NewEQService()

	_ = s.LogEmotionalState(EmotionalState{WorkspaceID: "ws1", DetectedEmotion: "positive", Valence: 0.5})
	_ = s.LogEmotionalState(EmotionalState{WorkspaceID: "ws1", DetectedEmotion: "neutral", Valence: 0.0})
	_ = s.LogEmotionalState(EmotionalState{WorkspaceID: "ws1", DetectedEmotion: "negative", Valence: -0.5})

	history := s.GetEmotionalHistory("ws1", 2)
	if len(history) != 2 {
		t.Fatalf("expected 2 entries, got %d", len(history))
	}
}

func TestAdaptResponseFormal(t *testing.T) {
	t.Parallel()
	s := NewEQService()

	_ = s.SetProfile("ws1", CommunicationProfile{
		Formality:  "formal",
		Verbosity:  "balanced",
		Directness: 0.5,
	})

	adapted, err := s.AdaptResponse("ws1", "Here is the report")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if adapted[len(adapted)-1] != '.' {
		t.Fatal("expected formal closing period")
	}
}

func TestAdaptResponseNoProfile(t *testing.T) {
	t.Parallel()
	s := NewEQService()

	adapted, err := s.AdaptResponse("ws_unknown", "Some response")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if adapted != "Some response" {
		t.Fatal("expected unchanged response when no profile exists")
	}
}

func TestDefaultProfile(t *testing.T) {
	t.Parallel()

	p := DefaultProfile()
	if p.Formality != "balanced" {
		t.Fatalf("expected balanced formality, got %s", p.Formality)
	}
	if p.Verbosity != "balanced" {
		t.Fatalf("expected balanced verbosity, got %s", p.Verbosity)
	}
}
