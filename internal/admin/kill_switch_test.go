package admin

import "testing"

func TestKillSwitchActivateHappyPath(t *testing.T) {
	t.Parallel()
	svc := NewKillSwitchService()
	ks, err := svc.Activate("ws1", "", "admin@brev.io", "runaway agent")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if ks.ID == "" {
		t.Fatal("expected generated ID")
	}
	if !svc.IsActive("ws1", "") {
		t.Fatal("expected kill switch to be active")
	}
}

func TestKillSwitchActivateMissingWorkspace(t *testing.T) {
	t.Parallel()
	svc := NewKillSwitchService()
	_, err := svc.Activate("", "", "admin", "reason")
	if err == nil {
		t.Fatal("expected error for missing workspace_id")
	}
}

func TestKillSwitchActivateMissingActivatedBy(t *testing.T) {
	t.Parallel()
	svc := NewKillSwitchService()
	_, err := svc.Activate("ws1", "", "", "reason")
	if err == nil {
		t.Fatal("expected error for missing activated_by")
	}
}

func TestKillSwitchDoubleActivate(t *testing.T) {
	t.Parallel()
	svc := NewKillSwitchService()
	_, _ = svc.Activate("ws1", "", "admin", "reason")
	_, err := svc.Activate("ws1", "", "admin", "reason again")
	if err == nil {
		t.Fatal("expected error for double activation")
	}
}

func TestKillSwitchDeactivate(t *testing.T) {
	t.Parallel()
	svc := NewKillSwitchService()
	_, _ = svc.Activate("ws1", "", "admin", "reason")
	if !svc.IsActive("ws1", "") {
		t.Fatal("expected active")
	}

	err := svc.Deactivate("ws1", "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if svc.IsActive("ws1", "") {
		t.Fatal("expected inactive after deactivation")
	}
}

func TestKillSwitchDeactivateNotFound(t *testing.T) {
	t.Parallel()
	svc := NewKillSwitchService()
	err := svc.Deactivate("ws1", "")
	if err == nil {
		t.Fatal("expected error for not found")
	}
}

func TestKillSwitchDoubleDeactivate(t *testing.T) {
	t.Parallel()
	svc := NewKillSwitchService()
	_, _ = svc.Activate("ws1", "", "admin", "reason")
	_ = svc.Deactivate("ws1", "")
	err := svc.Deactivate("ws1", "")
	if err == nil {
		t.Fatal("expected error for double deactivation")
	}
}

func TestKillSwitchUserLevel(t *testing.T) {
	t.Parallel()
	svc := NewKillSwitchService()
	_, _ = svc.Activate("ws1", "u1", "admin", "user-specific kill")

	if !svc.IsActive("ws1", "u1") {
		t.Fatal("expected active for user u1")
	}
	if svc.IsActive("ws1", "u2") {
		t.Fatal("expected inactive for user u2")
	}
}

func TestKillSwitchWorkspaceLevelBlocksAllUsers(t *testing.T) {
	t.Parallel()
	svc := NewKillSwitchService()
	_, _ = svc.Activate("ws1", "", "admin", "workspace-wide kill")

	if !svc.IsActive("ws1", "u1") {
		t.Fatal("expected workspace kill switch to block user u1")
	}
	if !svc.IsActive("ws1", "u2") {
		t.Fatal("expected workspace kill switch to block user u2")
	}
}

func TestKillSwitchGetAll(t *testing.T) {
	t.Parallel()
	svc := NewKillSwitchService()
	_, _ = svc.Activate("ws1", "", "admin", "reason1")
	_, _ = svc.Activate("ws2", "u1", "admin", "reason2")

	all := svc.GetAll()
	if len(all) != 2 {
		t.Fatalf("expected 2 kill switches, got %d", len(all))
	}
}
