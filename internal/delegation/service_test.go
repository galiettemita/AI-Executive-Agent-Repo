package delegation

import (
	"testing"
	"time"

	"github.com/google/uuid"
)

func TestGrantDelegationToolAllowlist(t *testing.T) {
	t.Parallel()

	svc := NewService()
	grant, err := svc.GrantDelegation(uuid.New(), uuid.New(), uuid.New(), []string{"gmail.send", "calendar.create_event"}, []string{"contacts"})
	if err != nil {
		t.Fatalf("grant delegation: %v", err)
	}
	allowed, err := svc.CanUseTool(grant.ID, "gmail.send")
	if err != nil {
		t.Fatalf("can use tool: %v", err)
	}
	if !allowed {
		t.Fatal("expected tool to be allowed")
	}
	allowed, err = svc.CanUseTool(grant.ID, "bank.transfer")
	if err != nil {
		t.Fatalf("can use tool: %v", err)
	}
	if allowed {
		t.Fatal("expected tool to be denied")
	}
}

func TestGranteeToolVisibilityAndRevoke(t *testing.T) {
	t.Parallel()

	svc := NewService()
	workspaceID := uuid.New()
	ownerID := uuid.New()
	granteeID := uuid.New()
	grant, err := svc.GrantDelegation(workspaceID, ownerID, granteeID, []string{"gmail.send", "calendar.create_event"}, []string{"contacts"})
	if err != nil {
		t.Fatalf("grant delegation: %v", err)
	}

	if !svc.CanGranteeUseTool(workspaceID, granteeID, "gmail.send") {
		t.Fatal("expected grantee to see allowed tool")
	}
	if svc.CanGranteeUseTool(workspaceID, granteeID, "bank.transfer") {
		t.Fatal("expected grantee to not see disallowed tool")
	}

	if err := svc.RevokeGrant(grant.ID); err != nil {
		t.Fatalf("revoke grant: %v", err)
	}
	if svc.CanGranteeUseTool(workspaceID, granteeID, "gmail.send") {
		t.Fatal("expected revoked grant to block tool visibility")
	}
}

func TestCreatePairingInvitation(t *testing.T) {
	t.Parallel()

	svc := NewService()
	_, err := svc.CreatePairingInvitation(uuid.New(), uuid.New(), "PAIR-123", time.Now().Add(time.Hour))
	if err != nil {
		t.Fatalf("create pairing invitation: %v", err)
	}
}

func TestAcceptPairingInvitation(t *testing.T) {
	t.Parallel()

	svc := NewService()
	invitation, err := svc.CreatePairingInvitation(uuid.New(), uuid.New(), "PAIR-456", time.Now().Add(time.Hour))
	if err != nil {
		t.Fatalf("create pairing invitation: %v", err)
	}

	acceptedBy := uuid.New()
	accepted, err := svc.AcceptPairingInvitation(invitation.InviteCode, acceptedBy, time.Now())
	if err != nil {
		t.Fatalf("accept pairing invitation: %v", err)
	}
	if accepted.Status != "accepted" {
		t.Fatalf("expected accepted status, got %s", accepted.Status)
	}
	if accepted.AcceptedByID != acceptedBy {
		t.Fatalf("accepted_by mismatch: got=%s want=%s", accepted.AcceptedByID, acceptedBy)
	}
}
