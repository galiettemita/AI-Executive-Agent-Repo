package memory

import "testing"

func TestColdStartInitializeDefaults(t *testing.T) {
	t.Parallel()

	cs := NewColdStartService()
	profile, err := cs.InitializeDefaults("ws1", "u1")
	if err != nil {
		t.Fatalf("initialize defaults: %v", err)
	}
	if profile.WorkspaceID != "ws1" {
		t.Fatalf("expected workspace ws1, got %s", profile.WorkspaceID)
	}
	if profile.BootstrapComplete {
		t.Fatal("expected bootstrap not complete")
	}
	if profile.DefaultPreferences["response_length"] != "medium" {
		t.Fatalf("expected response_length=medium, got %s", profile.DefaultPreferences["response_length"])
	}
	if profile.DefaultPreferences["formality"] != "professional" {
		t.Fatalf("expected formality=professional, got %s", profile.DefaultPreferences["formality"])
	}
	if profile.DefaultPreferences["detail_level"] != "moderate" {
		t.Fatalf("expected detail_level=moderate, got %s", profile.DefaultPreferences["detail_level"])
	}
}

func TestColdStartIdempotentInitialize(t *testing.T) {
	t.Parallel()

	cs := NewColdStartService()
	p1, _ := cs.InitializeDefaults("ws1", "u1")
	p2, _ := cs.InitializeDefaults("ws1", "u1")
	if p1.WorkspaceID != p2.WorkspaceID || p1.UserID != p2.UserID {
		t.Fatal("expected idempotent profile creation")
	}
}

func TestColdStartBootstrapLifecycle(t *testing.T) {
	t.Parallel()

	cs := NewColdStartService()
	_, _ = cs.InitializeDefaults("ws1", "u1")

	if cs.IsBootstrapComplete("ws1", "u1") {
		t.Fatal("expected bootstrap not complete initially")
	}

	err := cs.MarkBootstrapComplete("ws1", "u1")
	if err != nil {
		t.Fatalf("mark complete: %v", err)
	}

	if !cs.IsBootstrapComplete("ws1", "u1") {
		t.Fatal("expected bootstrap complete after marking")
	}
}

func TestColdStartGetDefaultPreferences(t *testing.T) {
	t.Parallel()

	cs := NewColdStartService()
	prefs := cs.GetDefaultPreferences("ws_missing", "u_missing")
	if prefs != nil {
		t.Fatal("expected nil preferences for missing profile")
	}

	_, _ = cs.InitializeDefaults("ws1", "u1")
	prefs = cs.GetDefaultPreferences("ws1", "u1")
	if len(prefs) != 3 {
		t.Fatalf("expected 3 default preferences, got %d", len(prefs))
	}
}

func TestColdStartValidation(t *testing.T) {
	t.Parallel()

	cs := NewColdStartService()
	_, err := cs.InitializeDefaults("", "u1")
	if err == nil {
		t.Fatal("expected error for empty workspace_id")
	}
	_, err = cs.InitializeDefaults("ws1", "")
	if err == nil {
		t.Fatal("expected error for empty user_id")
	}

	err = cs.MarkBootstrapComplete("ws_missing", "u_missing")
	if err == nil {
		t.Fatal("expected error for marking non-existent profile")
	}
}
