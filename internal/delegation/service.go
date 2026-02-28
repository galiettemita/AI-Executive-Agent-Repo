package delegation

import (
	"fmt"
	"sync"
	"time"

	"github.com/google/uuid"

	"github.com/brevio/brevio/internal/determinism"
)

type Grant struct {
	ID               uuid.UUID
	WorkspaceID      uuid.UUID
	OwnerUserID      uuid.UUID
	GranteeUserID    uuid.UUID
	ToolAllowlist    map[string]struct{}
	SharedMemoryKeys []string
	Status           string
	CreatedAt        time.Time
	ExpiresAt        *time.Time
	RevokedAt        *time.Time
}

type PairingInvitation struct {
	ID           uuid.UUID
	WorkspaceID  uuid.UUID
	OwnerUserID  uuid.UUID
	InviteCode   string
	Status       string
	CreatedAt    time.Time
	ExpiresAt    time.Time
	AcceptedByID uuid.UUID
}

type Service struct {
	mu          sync.RWMutex
	grants      map[uuid.UUID]Grant
	invitations map[string]PairingInvitation
}

func NewService() *Service {
	return &Service{
		grants:      map[uuid.UUID]Grant{},
		invitations: map[string]PairingInvitation{},
	}
}

func (s *Service) GrantDelegation(workspaceID, ownerUserID, granteeUserID uuid.UUID, toolAllowlist, sharedMemoryKeys []string) (Grant, error) {
	if workspaceID == uuid.Nil || ownerUserID == uuid.Nil || granteeUserID == uuid.Nil {
		return Grant{}, fmt.Errorf("workspace_id, owner_user_id, and grantee_user_id are required")
	}
	id, err := determinism.NewUUIDv7()
	if err != nil {
		return Grant{}, err
	}
	allowlist := make(map[string]struct{}, len(toolAllowlist))
	for _, tool := range toolAllowlist {
		allowlist[tool] = struct{}{}
	}
	grant := Grant{
		ID:               id,
		WorkspaceID:      workspaceID,
		OwnerUserID:      ownerUserID,
		GranteeUserID:    granteeUserID,
		ToolAllowlist:    allowlist,
		SharedMemoryKeys: sharedMemoryKeys,
		Status:           "active",
		CreatedAt:        time.Now().UTC(),
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.grants[grant.ID] = grant
	return grant, nil
}

func (s *Service) CanUseTool(grantID uuid.UUID, toolKey string) (bool, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	grant, ok := s.grants[grantID]
	if !ok {
		return false, fmt.Errorf("grant not found")
	}
	if grant.Status != "active" {
		return false, nil
	}
	_, allowed := grant.ToolAllowlist[toolKey]
	return allowed, nil
}

func (s *Service) RevokeGrant(grantID uuid.UUID) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	grant, ok := s.grants[grantID]
	if !ok {
		return fmt.Errorf("grant not found")
	}
	now := time.Now().UTC()
	grant.Status = "revoked"
	grant.RevokedAt = &now
	s.grants[grantID] = grant
	return nil
}

func (s *Service) CanGranteeUseTool(workspaceID, granteeUserID uuid.UUID, toolKey string) bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	for _, grant := range s.grants {
		if grant.WorkspaceID != workspaceID || grant.GranteeUserID != granteeUserID {
			continue
		}
		if grant.Status != "active" {
			continue
		}
		if grant.ExpiresAt != nil && grant.ExpiresAt.Before(time.Now().UTC()) {
			continue
		}
		if _, ok := grant.ToolAllowlist[toolKey]; ok {
			return true
		}
	}
	return false
}

func (s *Service) CreatePairingInvitation(workspaceID, ownerUserID uuid.UUID, code string, expiresAt time.Time) (PairingInvitation, error) {
	if code == "" {
		return PairingInvitation{}, fmt.Errorf("invite code is required")
	}
	id, err := determinism.NewUUIDv7()
	if err != nil {
		return PairingInvitation{}, err
	}
	invitation := PairingInvitation{
		ID:          id,
		WorkspaceID: workspaceID,
		OwnerUserID: ownerUserID,
		InviteCode:  code,
		Status:      "pending",
		CreatedAt:   time.Now().UTC(),
		ExpiresAt:   expiresAt,
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.invitations[code] = invitation
	return invitation, nil
}

func (s *Service) AcceptPairingInvitation(code string, acceptedByID uuid.UUID, now time.Time) (PairingInvitation, error) {
	if code == "" {
		return PairingInvitation{}, fmt.Errorf("invite code is required")
	}
	if acceptedByID == uuid.Nil {
		return PairingInvitation{}, fmt.Errorf("accepted_by_id is required")
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	invitation, ok := s.invitations[code]
	if !ok {
		return PairingInvitation{}, fmt.Errorf("invitation not found")
	}
	if invitation.Status != "pending" {
		return PairingInvitation{}, fmt.Errorf("invitation is not pending")
	}
	if now.After(invitation.ExpiresAt) {
		invitation.Status = "expired"
		s.invitations[code] = invitation
		return PairingInvitation{}, fmt.Errorf("invitation expired")
	}

	invitation.Status = "accepted"
	invitation.AcceptedByID = acceptedByID
	s.invitations[code] = invitation
	return invitation, nil
}
