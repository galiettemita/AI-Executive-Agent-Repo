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
	CreatedAt        time.Time
	ExpiresAt        *time.Time
}

type PairingInvitation struct {
	ID           uuid.UUID
	WorkspaceID  uuid.UUID
	OwnerUserID  uuid.UUID
	InviteCode   string
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
	_, allowed := grant.ToolAllowlist[toolKey]
	return allowed, nil
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
		CreatedAt:   time.Now().UTC(),
		ExpiresAt:   expiresAt,
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.invitations[code] = invitation
	return invitation, nil
}
