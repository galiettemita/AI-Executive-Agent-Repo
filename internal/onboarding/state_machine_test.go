package onboarding

import "testing"

func TestStartSession(t *testing.T) {
	t.Parallel()
	s := NewOnboardingService()

	session, err := s.StartSession("ws1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if session.CurrentStage != StageWelcome {
		t.Fatalf("expected welcome stage, got %s", session.CurrentStage)
	}
}

func TestStartSessionDuplicate(t *testing.T) {
	t.Parallel()
	s := NewOnboardingService()

	_, _ = s.StartSession("ws1")
	_, err := s.StartSession("ws1")
	if err == nil {
		t.Fatal("expected error for duplicate session")
	}
}

func TestStartSessionEmptyWorkspace(t *testing.T) {
	t.Parallel()
	s := NewOnboardingService()

	_, err := s.StartSession("")
	if err == nil {
		t.Fatal("expected error for empty workspace")
	}
}

func TestAdvanceStage(t *testing.T) {
	t.Parallel()
	s := NewOnboardingService()

	session, _ := s.StartSession("ws1")
	err := s.AdvanceStage(session.ID, map[string]string{"name": "Alice"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	status, _ := s.GetStatus("ws1")
	if status.CurrentStage != StageDiscovery {
		t.Fatalf("expected discovery stage, got %s", status.CurrentStage)
	}
	if len(status.CompletedStages) != 1 {
		t.Fatalf("expected 1 completed stage, got %d", len(status.CompletedStages))
	}
}

func TestSkipStage(t *testing.T) {
	t.Parallel()
	s := NewOnboardingService()

	session, _ := s.StartSession("ws1")
	_ = s.SkipStage(session.ID)

	status, _ := s.GetStatus("ws1")
	if status.CurrentStage != StageDiscovery {
		t.Fatalf("expected discovery stage, got %s", status.CurrentStage)
	}
	if len(status.SkippedStages) != 1 {
		t.Fatalf("expected 1 skipped stage, got %d", len(status.SkippedStages))
	}
}

func TestFullOnboardingFlow(t *testing.T) {
	t.Parallel()
	s := NewOnboardingService()

	session, _ := s.StartSession("ws1")
	for _, stage := range AllStages() {
		_ = stage
		if err := s.AdvanceStage(session.ID, nil); err != nil {
			t.Fatalf("unexpected error advancing: %v", err)
		}
	}

	if !s.IsComplete(session.ID) {
		t.Fatal("expected session to be complete")
	}
}

func TestIsCompleteNotFound(t *testing.T) {
	t.Parallel()
	s := NewOnboardingService()

	if s.IsComplete("nonexistent") {
		t.Fatal("expected false for unknown session")
	}
}

func TestGetStatusNotFound(t *testing.T) {
	t.Parallel()
	s := NewOnboardingService()

	_, err := s.GetStatus("nonexistent")
	if err == nil {
		t.Fatal("expected error for unknown workspace")
	}
}

func TestAdvanceAfterComplete(t *testing.T) {
	t.Parallel()
	s := NewOnboardingService()

	session, _ := s.StartSession("ws1")
	for range AllStages() {
		_ = s.AdvanceStage(session.ID, nil)
	}

	err := s.AdvanceStage(session.ID, nil)
	if err == nil {
		t.Fatal("expected error advancing completed session")
	}
}

func TestAllStages(t *testing.T) {
	t.Parallel()

	stages := AllStages()
	if len(stages) != 6 {
		t.Fatalf("expected 6 stages, got %d", len(stages))
	}
}

func TestIsValidStage(t *testing.T) {
	t.Parallel()

	if !IsValidStage("welcome") {
		t.Fatal("expected welcome to be valid")
	}
	if IsValidStage("nonexistent") {
		t.Fatal("expected nonexistent to be invalid")
	}
}
