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

func TestCreatePairingInvitation(t *testing.T) {
	t.Parallel()

	svc := NewService()
	_, err := svc.CreatePairingInvitation(uuid.New(), uuid.New(), "PAIR-123", time.Now().Add(time.Hour))
	if err != nil {
		t.Fatalf("create pairing invitation: %v", err)
	}
}
