package identity

import "testing"

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
