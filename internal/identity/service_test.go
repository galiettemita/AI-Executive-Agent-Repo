package identity

import (
	"encoding/json"
	"testing"
)

func TestCreateUserWorkspaceAndChannelBinding(t *testing.T) {
	t.Parallel()

	svc := NewService()
	account, err := svc.CreateAccount("pro", "active", "cust_123")
	if err != nil {
		t.Fatalf("create account: %v", err)
	}

	user, err := svc.CreateUser(account.ID, "owner@example.com", "+15550001111", "A1", "UTC")
	if err != nil {
		t.Fatalf("create user: %v", err)
	}

	workspace, err := svc.CreateWorkspace(account.ID, user.ID, "ws_owner_example", "{}", []string{"gmail", "calendar"})
	if err != nil {
		t.Fatalf("create workspace: %v", err)
	}

	_, err = svc.BindChannel(workspace.ID, "whatsapp", "+15550001111")
	if err != nil {
		t.Fatalf("bind channel: %v", err)
	}

	resolved, err := svc.ResolveWorkspaceByChannel("whatsapp", "+15550001111")
	if err != nil {
		t.Fatalf("resolve workspace: %v", err)
	}
	if resolved != workspace.ID {
		t.Fatalf("workspace mismatch: got %s want %s", resolved, workspace.ID)
	}
}

func TestCreateWorkspaceSeedsDefaultDomainAutonomy(t *testing.T) {
	t.Parallel()

	svc := NewService()
	account, err := svc.CreateAccount("pro", "active", "cust_456")
	if err != nil {
		t.Fatalf("create account: %v", err)
	}
	user, err := svc.CreateUser(account.ID, "owner2@example.com", "+15550002222", "A1", "UTC")
	if err != nil {
		t.Fatalf("create user: %v", err)
	}

	workspace, err := svc.CreateWorkspace(account.ID, user.ID, "ws_default_autonomy", "", []string{"gmail"})
	if err != nil {
		t.Fatalf("create workspace: %v", err)
	}

	var defaults map[string]string
	if err := json.Unmarshal([]byte(workspace.DomainAutonomyJSON), &defaults); err != nil {
		t.Fatalf("decode default autonomy json: %v", err)
	}

	expected := map[string]string{
		"calendar":    "A2",
		"email":       "A1",
		"messaging":   "A1",
		"tasks":       "A2",
		"documents":   "A1",
		"crm":         "A1",
		"travel":      "A2",
		"financial":   "A1",
		"health":      "A0",
		"environment": "A1",
		"web":         "A3",
	}
	if len(defaults) != len(expected) {
		t.Fatalf("default autonomy size mismatch: got=%d want=%d", len(defaults), len(expected))
	}
	for domain, level := range expected {
		if defaults[domain] != level {
			t.Fatalf("default autonomy mismatch for %s: got=%s want=%s", domain, defaults[domain], level)
		}
	}
}
