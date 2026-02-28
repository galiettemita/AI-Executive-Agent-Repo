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

func TestAccountAndUserCRUD(t *testing.T) {
	t.Parallel()

	svc := NewService()
	account, err := svc.CreateAccount("pro", "active", "cust_789")
	if err != nil {
		t.Fatalf("create account: %v", err)
	}

	plan := "enterprise"
	status := "suspended"
	billing := "cust_updated"
	updatedAccount, err := svc.UpdateAccount(account.ID, AccountUpdate{
		PlanKey:            &plan,
		Status:             &status,
		BillingCustomerRef: &billing,
	})
	if err != nil {
		t.Fatalf("update account: %v", err)
	}
	if updatedAccount.PlanKey != "enterprise" || updatedAccount.Status != "suspended" || updatedAccount.BillingCustomerRef != "cust_updated" {
		t.Fatalf("unexpected updated account: %+v", updatedAccount)
	}
	if _, err := svc.GetAccount(account.ID); err != nil {
		t.Fatalf("get account: %v", err)
	}

	user, err := svc.CreateUser(account.ID, "crud@example.com", "+15550003333", "A1", "UTC")
	if err != nil {
		t.Fatalf("create user: %v", err)
	}
	email := "crud-updated@example.com"
	phone := "+15550004444"
	autonomy := "A2"
	timezone := "America/New_York"
	userStatus := "suspended"
	updatedUser, err := svc.UpdateUser(user.ID, UserUpdate{
		Email:          &email,
		PhoneE164:      &phone,
		GlobalAutonomy: &autonomy,
		Timezone:       &timezone,
		Status:         &userStatus,
	})
	if err != nil {
		t.Fatalf("update user: %v", err)
	}
	if updatedUser.Email != email || updatedUser.PhoneE164 != phone || updatedUser.GlobalAutonomy != autonomy || updatedUser.Timezone != timezone || updatedUser.Status != userStatus {
		t.Fatalf("unexpected updated user: %+v", updatedUser)
	}
	if err := svc.DeleteUser(user.ID); err != nil {
		t.Fatalf("delete user: %v", err)
	}
	deletedUser, err := svc.GetUser(user.ID)
	if err != nil {
		t.Fatalf("get deleted user: %v", err)
	}
	if deletedUser.Status != "deleted" {
		t.Fatalf("expected deleted user status, got %s", deletedUser.Status)
	}
}

func TestArchiveWorkspaceLifecycle(t *testing.T) {
	t.Parallel()

	svc := NewService()
	account, err := svc.CreateAccount("pro", "active", "cust_lifecycle")
	if err != nil {
		t.Fatalf("create account: %v", err)
	}
	user, err := svc.CreateUser(account.ID, "owner3@example.com", "+15550005555", "A1", "UTC")
	if err != nil {
		t.Fatalf("create user: %v", err)
	}
	workspace, err := svc.CreateWorkspace(account.ID, user.ID, "ws_lifecycle", "{}", []string{"gmail"})
	if err != nil {
		t.Fatalf("create workspace: %v", err)
	}
	if err := svc.ArchiveWorkspace(workspace.ID); err != nil {
		t.Fatalf("archive workspace: %v", err)
	}
	archived, err := svc.GetWorkspace(workspace.ID)
	if err != nil {
		t.Fatalf("get workspace: %v", err)
	}
	if archived.Status != "archived" {
		t.Fatalf("expected archived workspace, got %s", archived.Status)
	}
}

func TestBindChannelRejectsUnsupportedChannel(t *testing.T) {
	t.Parallel()

	svc := NewService()
	account, err := svc.CreateAccount("pro", "active", "cust_channel")
	if err != nil {
		t.Fatalf("create account: %v", err)
	}
	user, err := svc.CreateUser(account.ID, "owner4@example.com", "+15550006666", "A1", "UTC")
	if err != nil {
		t.Fatalf("create user: %v", err)
	}
	workspace, err := svc.CreateWorkspace(account.ID, user.ID, "ws_channel", "{}", []string{"gmail"})
	if err != nil {
		t.Fatalf("create workspace: %v", err)
	}
	if _, err := svc.BindChannel(workspace.ID, "email", "owner4@example.com"); err == nil {
		t.Fatal("expected unsupported channel to fail")
	}
}
