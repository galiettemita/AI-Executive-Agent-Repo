package federation

import (
	"context"
	"testing"
)

func TestCreateFederation(t *testing.T) {
	t.Parallel()
	s := NewFederationService()

	peer, err := s.CreateFederation(context.Background(), "ws1", "ws2", []string{"read", "write"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if peer.Status != "pending" {
		t.Fatalf("expected pending status, got %s", peer.Status)
	}
	if len(peer.SharedCapabilities) != 2 {
		t.Fatalf("expected 2 capabilities, got %d", len(peer.SharedCapabilities))
	}
}

func TestCreateFederationSelfError(t *testing.T) {
	t.Parallel()
	s := NewFederationService()

	_, err := s.CreateFederation(context.Background(), "ws1", "ws1", nil)
	if err == nil {
		t.Fatal("expected error for self-federation")
	}
}

func TestCreateFederationDuplicateError(t *testing.T) {
	t.Parallel()
	s := NewFederationService()

	_, _ = s.CreateFederation(context.Background(), "ws1", "ws2", nil)
	_, err := s.CreateFederation(context.Background(), "ws1", "ws2", nil)
	if err == nil {
		t.Fatal("expected error for duplicate federation")
	}
}

func TestAcceptFederation(t *testing.T) {
	t.Parallel()
	s := NewFederationService()
	ctx := context.Background()

	peer, _ := s.CreateFederation(ctx, "ws1", "ws2", []string{"read"})
	if err := s.AcceptFederation(ctx, peer.ID); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	list := s.ListFederations(ctx, "ws1")
	if len(list) != 1 || list[0].Status != "active" {
		t.Fatal("expected active federation")
	}
	if list[0].AcceptedAt.IsZero() {
		t.Fatal("expected non-zero AcceptedAt")
	}
}

func TestAcceptNonPendingError(t *testing.T) {
	t.Parallel()
	s := NewFederationService()
	ctx := context.Background()

	peer, _ := s.CreateFederation(ctx, "ws1", "ws2", nil)
	_ = s.AcceptFederation(ctx, peer.ID)
	err := s.AcceptFederation(ctx, peer.ID)
	if err == nil {
		t.Fatal("expected error accepting already active federation")
	}
}

func TestRejectFederation(t *testing.T) {
	t.Parallel()
	s := NewFederationService()
	ctx := context.Background()

	peer, _ := s.CreateFederation(ctx, "ws1", "ws2", nil)
	if err := s.RejectFederation(ctx, peer.ID); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	list := s.ListFederations(ctx, "ws1")
	if len(list) != 1 || list[0].Status != "revoked" {
		t.Fatal("expected revoked federation")
	}
}

func TestRejectNonPendingError(t *testing.T) {
	t.Parallel()
	s := NewFederationService()
	ctx := context.Background()

	peer, _ := s.CreateFederation(ctx, "ws1", "ws2", nil)
	_ = s.AcceptFederation(ctx, peer.ID)
	err := s.RejectFederation(ctx, peer.ID)
	if err == nil {
		t.Fatal("expected error rejecting active federation")
	}
}

func TestSuspendFederation(t *testing.T) {
	t.Parallel()
	s := NewFederationService()
	ctx := context.Background()

	peer, _ := s.CreateFederation(ctx, "ws1", "ws2", []string{"read"})
	_ = s.AcceptFederation(ctx, peer.ID)
	if err := s.SuspendFederation(ctx, peer.ID); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	list := s.ListFederations(ctx, "ws1")
	if len(list) != 1 || list[0].Status != "suspended" {
		t.Fatal("expected suspended federation")
	}
}

func TestSuspendNonActiveError(t *testing.T) {
	t.Parallel()
	s := NewFederationService()
	ctx := context.Background()

	peer, _ := s.CreateFederation(ctx, "ws1", "ws2", nil)
	err := s.SuspendFederation(ctx, peer.ID)
	if err == nil {
		t.Fatal("expected error suspending pending federation")
	}
}

func TestProposeNegotiation(t *testing.T) {
	t.Parallel()
	s := NewFederationService()
	ctx := context.Background()

	peer, _ := s.CreateFederation(ctx, "ws1", "ws2", []string{"read"})
	_ = s.AcceptFederation(ctx, peer.ID)

	neg, err := s.ProposeNegotiation(ctx, peer.ID, []string{"write", "execute"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if neg.Status != "proposed" {
		t.Fatalf("expected proposed status, got %s", neg.Status)
	}
	if len(neg.ProposedCapabilities) != 2 {
		t.Fatalf("expected 2 proposed capabilities, got %d", len(neg.ProposedCapabilities))
	}
	if neg.ExpiresAt.IsZero() {
		t.Fatal("expected non-zero ExpiresAt")
	}
}

func TestProposeNegotiationEmptyCaps(t *testing.T) {
	t.Parallel()
	s := NewFederationService()
	ctx := context.Background()

	peer, _ := s.CreateFederation(ctx, "ws1", "ws2", []string{"read"})
	_ = s.AcceptFederation(ctx, peer.ID)

	_, err := s.ProposeNegotiation(ctx, peer.ID, []string{})
	if err == nil {
		t.Fatal("expected error for empty capabilities")
	}
}

func TestRespondToNegotiationAccept(t *testing.T) {
	t.Parallel()
	s := NewFederationService()
	ctx := context.Background()

	peer, _ := s.CreateFederation(ctx, "ws1", "ws2", []string{"read"})
	_ = s.AcceptFederation(ctx, peer.ID)
	neg, _ := s.ProposeNegotiation(ctx, peer.ID, []string{"write", "execute"})

	err := s.RespondToNegotiation(ctx, neg.ID, true, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Capabilities should be updated on the federation peer.
	list := s.ListFederations(ctx, "ws1")
	if len(list) != 1 {
		t.Fatalf("expected 1 federation, got %d", len(list))
	}
	if len(list[0].SharedCapabilities) != 2 {
		t.Fatalf("expected 2 capabilities after negotiation, got %d", len(list[0].SharedCapabilities))
	}
}

func TestRespondToNegotiationReject(t *testing.T) {
	t.Parallel()
	s := NewFederationService()
	ctx := context.Background()

	peer, _ := s.CreateFederation(ctx, "ws1", "ws2", []string{"read"})
	_ = s.AcceptFederation(ctx, peer.ID)
	neg, _ := s.ProposeNegotiation(ctx, peer.ID, []string{"write"})

	err := s.RespondToNegotiation(ctx, neg.ID, false, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestRespondToNegotiationCounterPropose(t *testing.T) {
	t.Parallel()
	s := NewFederationService()
	ctx := context.Background()

	peer, _ := s.CreateFederation(ctx, "ws1", "ws2", []string{"read"})
	_ = s.AcceptFederation(ctx, peer.ID)
	neg, _ := s.ProposeNegotiation(ctx, peer.ID, []string{"write", "execute"})

	err := s.RespondToNegotiation(ctx, neg.ID, false, []string{"write"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestValidateCapability(t *testing.T) {
	t.Parallel()

	peer := &FederationPeer{
		Status:             "active",
		SharedCapabilities: []string{"read", "write"},
	}
	if !ValidateCapability(peer, "read") {
		t.Fatal("expected read to be valid")
	}
	if ValidateCapability(peer, "delete") {
		t.Fatal("expected delete to be invalid")
	}
	if ValidateCapability(nil, "read") {
		t.Fatal("expected nil peer to be invalid")
	}

	pendingPeer := &FederationPeer{
		Status:             "pending",
		SharedCapabilities: []string{"read"},
	}
	if ValidateCapability(pendingPeer, "read") {
		t.Fatal("expected pending peer to be invalid")
	}
}

func TestRevokeFederation(t *testing.T) {
	t.Parallel()
	s := NewFederationService()
	ctx := context.Background()

	peer, _ := s.CreateFederation(ctx, "ws1", "ws2", nil)
	_ = s.AcceptFederation(ctx, peer.ID)
	if err := s.RevokeFederation(peer.ID); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestRevokeAlreadyRevokedError(t *testing.T) {
	t.Parallel()
	s := NewFederationService()
	ctx := context.Background()

	peer, _ := s.CreateFederation(ctx, "ws1", "ws2", nil)
	_ = s.RevokeFederation(peer.ID)
	err := s.RevokeFederation(peer.ID)
	if err == nil {
		t.Fatal("expected error revoking already revoked federation")
	}
}

func TestNegotiateCapabilities(t *testing.T) {
	t.Parallel()
	s := NewFederationService()
	ctx := context.Background()

	peer, _ := s.CreateFederation(ctx, "ws1", "ws2", []string{"read", "write", "execute"})
	_ = s.AcceptFederation(ctx, peer.ID)

	result, err := s.NegotiateCapabilities(peer.ID, []string{"read", "delete"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(result.Accepted) != 1 || result.Accepted[0] != "read" {
		t.Fatalf("unexpected accepted: %v", result.Accepted)
	}
	if len(result.Denied) != 1 || result.Denied[0] != "delete" {
		t.Fatalf("unexpected denied: %v", result.Denied)
	}
}

func TestSendFederatedMessage(t *testing.T) {
	t.Parallel()
	s := NewFederationService()
	ctx := context.Background()

	peer, _ := s.CreateFederation(ctx, "ws1", "ws2", []string{"read"})
	_ = s.AcceptFederation(ctx, peer.ID)

	err := s.SendFederatedMessage(peer.ID, FederatedMessage{
		SenderWorkspace: "ws1",
		Intent:          "query",
		Payload:         map[string]any{"q": "hello"},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestSendMessageInactiveFederationError(t *testing.T) {
	t.Parallel()
	s := NewFederationService()
	ctx := context.Background()

	peer, _ := s.CreateFederation(ctx, "ws1", "ws2", nil)
	err := s.SendFederatedMessage(peer.ID, FederatedMessage{Intent: "query"})
	if err == nil {
		t.Fatal("expected error sending to pending federation")
	}
}

func TestCanExecute(t *testing.T) {
	t.Parallel()
	s := NewFederationService()
	ctx := context.Background()

	peer, _ := s.CreateFederation(ctx, "ws1", "ws2", []string{"read", "write"})
	_ = s.AcceptFederation(ctx, peer.ID)

	if !s.CanExecute(peer.ID, "read") {
		t.Fatal("expected read to be executable")
	}
	if s.CanExecute(peer.ID, "delete") {
		t.Fatal("expected delete to not be executable")
	}
}
