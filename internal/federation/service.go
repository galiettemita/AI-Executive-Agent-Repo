package federation

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/google/uuid"
)

// FederationPeer represents a federation link between two workspaces.
type FederationPeer struct {
	ID                 string    `json:"id"`
	WorkspaceID        string    `json:"workspace_id"`
	PeerWorkspaceID    string    `json:"peer_workspace_id"`
	Status             string    `json:"status"` // pending, active, suspended, revoked
	SharedCapabilities []string  `json:"shared_capabilities"`
	CreatedAt          time.Time `json:"created_at"`
	AcceptedAt         time.Time `json:"accepted_at,omitempty"`
}

// FederationNegotiation tracks a capability negotiation between federated peers.
type FederationNegotiation struct {
	ID                   string    `json:"id"`
	FederationID         string    `json:"federation_id"`
	ProposerID           string    `json:"proposer_id"`
	ResponderID          string    `json:"responder_id"`
	ProposedCapabilities []string  `json:"proposed_capabilities"`
	AcceptedCapabilities []string  `json:"accepted_capabilities"`
	Status               string    `json:"status"` // proposed, accepted, rejected, counter_proposed
	ExpiresAt            time.Time `json:"expires_at"`
}

// NegotiationResult captures the outcome of capability negotiation.
type NegotiationResult struct {
	Accepted []string `json:"accepted"`
	Denied   []string `json:"denied"`
	Reason   string   `json:"reason"`
}

// FederatedMessage is a message sent between federated workspaces.
type FederatedMessage struct {
	SenderWorkspace string         `json:"sender_workspace"`
	Intent          string         `json:"intent"`
	Payload         map[string]any `json:"payload"`
}

// FederationService manages multi-agent federation.
type FederationService struct {
	mu           sync.Mutex
	peers        map[string]FederationPeer
	negotiations map[string]FederationNegotiation
	messages     map[string][]FederatedMessage
	now          func() time.Time
}

// NewFederationService creates a new FederationService.
func NewFederationService() *FederationService {
	return &FederationService{
		peers:        map[string]FederationPeer{},
		negotiations: map[string]FederationNegotiation{},
		messages:     map[string][]FederatedMessage{},
		now:          func() time.Time { return time.Now().UTC() },
	}
}

// CreateFederation creates a new federation link between two workspaces.
func (s *FederationService) CreateFederation(_ context.Context, workspaceID, peerWorkspaceID string, capabilities []string) (*FederationPeer, error) {
	if workspaceID == "" {
		return nil, fmt.Errorf("workspace ID is required")
	}
	if peerWorkspaceID == "" {
		return nil, fmt.Errorf("peer workspace ID is required")
	}
	if workspaceID == peerWorkspaceID {
		return nil, fmt.Errorf("cannot federate with self")
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	// Check for existing federation
	for _, p := range s.peers {
		if p.WorkspaceID == workspaceID && p.PeerWorkspaceID == peerWorkspaceID && p.Status != "revoked" {
			return nil, fmt.Errorf("federation already exists: %s", p.ID)
		}
	}

	caps := make([]string, len(capabilities))
	copy(caps, capabilities)

	peer := FederationPeer{
		ID:                 uuid.Must(uuid.NewV7()).String(),
		WorkspaceID:        workspaceID,
		PeerWorkspaceID:    peerWorkspaceID,
		Status:             "pending",
		SharedCapabilities: caps,
		CreatedAt:          s.now(),
	}
	s.peers[peer.ID] = peer
	return &peer, nil
}

// AcceptFederation moves a federation from pending to active.
func (s *FederationService) AcceptFederation(_ context.Context, federationID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	peer, ok := s.peers[federationID]
	if !ok {
		return fmt.Errorf("federation not found: %s", federationID)
	}
	if peer.Status != "pending" {
		return fmt.Errorf("federation is not pending: %s", peer.Status)
	}
	peer.Status = "active"
	peer.AcceptedAt = s.now()
	s.peers[federationID] = peer
	return nil
}

// RejectFederation rejects a pending federation.
func (s *FederationService) RejectFederation(_ context.Context, federationID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	peer, ok := s.peers[federationID]
	if !ok {
		return fmt.Errorf("federation not found: %s", federationID)
	}
	if peer.Status != "pending" {
		return fmt.Errorf("federation is not pending: %s", peer.Status)
	}
	peer.Status = "revoked"
	s.peers[federationID] = peer
	return nil
}

// SuspendFederation suspends an active federation.
func (s *FederationService) SuspendFederation(_ context.Context, federationID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	peer, ok := s.peers[federationID]
	if !ok {
		return fmt.Errorf("federation not found: %s", federationID)
	}
	if peer.Status != "active" {
		return fmt.Errorf("federation is not active: %s", peer.Status)
	}
	peer.Status = "suspended"
	s.peers[federationID] = peer
	return nil
}

// ProposeNegotiation creates a new capability negotiation on a federation link.
func (s *FederationService) ProposeNegotiation(_ context.Context, federationID string, capabilities []string) (*FederationNegotiation, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	peer, ok := s.peers[federationID]
	if !ok {
		return nil, fmt.Errorf("federation not found: %s", federationID)
	}
	if peer.Status != "active" && peer.Status != "pending" {
		return nil, fmt.Errorf("federation is not active or pending: %s", peer.Status)
	}
	if len(capabilities) == 0 {
		return nil, fmt.Errorf("at least one capability is required")
	}

	caps := make([]string, len(capabilities))
	copy(caps, capabilities)

	neg := FederationNegotiation{
		ID:                   uuid.Must(uuid.NewV7()).String(),
		FederationID:         federationID,
		ProposerID:           peer.WorkspaceID,
		ResponderID:          peer.PeerWorkspaceID,
		ProposedCapabilities: caps,
		AcceptedCapabilities: []string{},
		Status:               "proposed",
		ExpiresAt:            s.now().Add(72 * time.Hour),
	}
	s.negotiations[neg.ID] = neg
	return &neg, nil
}

// RespondToNegotiation accepts or rejects (or counter-proposes) a negotiation.
func (s *FederationService) RespondToNegotiation(_ context.Context, negotiationID string, accept bool, counterCapabilities []string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	neg, ok := s.negotiations[negotiationID]
	if !ok {
		return fmt.Errorf("negotiation not found: %s", negotiationID)
	}
	if neg.Status != "proposed" && neg.Status != "counter_proposed" {
		return fmt.Errorf("negotiation is not open: %s", neg.Status)
	}

	if accept {
		neg.Status = "accepted"
		accepted := make([]string, len(neg.ProposedCapabilities))
		copy(accepted, neg.ProposedCapabilities)
		neg.AcceptedCapabilities = accepted

		// Update federation capabilities.
		if peer, peerOk := s.peers[neg.FederationID]; peerOk {
			peer.SharedCapabilities = accepted
			s.peers[neg.FederationID] = peer
		}
	} else if len(counterCapabilities) > 0 {
		neg.Status = "counter_proposed"
		counter := make([]string, len(counterCapabilities))
		copy(counter, counterCapabilities)
		neg.ProposedCapabilities = counter
	} else {
		neg.Status = "rejected"
	}

	s.negotiations[negotiationID] = neg
	return nil
}

// ListFederations lists all federation peers for a workspace.
func (s *FederationService) ListFederations(_ context.Context, workspaceID string) []FederationPeer {
	s.mu.Lock()
	defer s.mu.Unlock()

	var out []FederationPeer
	for _, p := range s.peers {
		if p.WorkspaceID == workspaceID || p.PeerWorkspaceID == workspaceID {
			cp := p
			caps := make([]string, len(cp.SharedCapabilities))
			copy(caps, cp.SharedCapabilities)
			cp.SharedCapabilities = caps
			out = append(out, cp)
		}
	}
	return out
}

// ValidateCapability checks whether a given capability is allowed on a federation peer.
func ValidateCapability(peer *FederationPeer, capability string) bool {
	if peer == nil || peer.Status != "active" {
		return false
	}
	for _, c := range peer.SharedCapabilities {
		if c == capability {
			return true
		}
	}
	return false
}

// RevokeFederation revokes an existing federation.
func (s *FederationService) RevokeFederation(federationID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	peer, ok := s.peers[federationID]
	if !ok {
		return fmt.Errorf("federation not found: %s", federationID)
	}
	if peer.Status == "revoked" {
		return fmt.Errorf("federation already revoked")
	}
	peer.Status = "revoked"
	s.peers[federationID] = peer
	return nil
}

// NegotiateCapabilities evaluates requested capabilities against shared ones.
func (s *FederationService) NegotiateCapabilities(federationID string, requested []string) (*NegotiationResult, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	peer, ok := s.peers[federationID]
	if !ok {
		return nil, fmt.Errorf("federation not found: %s", federationID)
	}
	if peer.Status != "active" {
		return nil, fmt.Errorf("federation is not active: %s", peer.Status)
	}

	sharedSet := map[string]bool{}
	for _, c := range peer.SharedCapabilities {
		sharedSet[c] = true
	}

	result := &NegotiationResult{
		Accepted: []string{},
		Denied:   []string{},
		Reason:   "negotiation complete",
	}
	for _, r := range requested {
		if sharedSet[r] {
			result.Accepted = append(result.Accepted, r)
		} else {
			result.Denied = append(result.Denied, r)
		}
	}
	return result, nil
}

// SendFederatedMessage sends a message over an active federation link.
func (s *FederationService) SendFederatedMessage(federationID string, msg FederatedMessage) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	peer, ok := s.peers[federationID]
	if !ok {
		return fmt.Errorf("federation not found: %s", federationID)
	}
	if peer.Status != "active" {
		return fmt.Errorf("federation is not active: %s", peer.Status)
	}
	if msg.Intent == "" {
		return fmt.Errorf("message intent is required")
	}

	s.messages[federationID] = append(s.messages[federationID], msg)
	return nil
}

// CanExecute checks whether a given capability is allowed on a federation link.
func (s *FederationService) CanExecute(federationID, capability string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()

	peer, ok := s.peers[federationID]
	if !ok {
		return false
	}
	if peer.Status != "active" {
		return false
	}
	for _, c := range peer.SharedCapabilities {
		if c == capability {
			return true
		}
	}
	return false
}
