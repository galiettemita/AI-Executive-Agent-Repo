package rbac

import (
	"fmt"
	"sync"

	"github.com/google/uuid"
)

const (
	RoleOwner    = "owner"
	RoleAdmin    = "admin"
	RoleDelegate = "delegate"
	RoleAuditor  = "auditor"
	RoleOperator = "operator"
)

var roleRank = map[string]int{
	RoleOwner:    5,
	RoleAdmin:    4,
	RoleDelegate: 3,
	RoleAuditor:  2,
	RoleOperator: 1,
}

type Binding struct {
	WorkspaceID uuid.UUID
	UserID      uuid.UUID
	Role        string
	Revoked     bool
}

type Service struct {
	mu       sync.RWMutex
	bindings map[string]Binding
}

func NewService() *Service {
	return &Service{bindings: map[string]Binding{}}
}

func bindingKey(workspaceID, userID uuid.UUID) string {
	return workspaceID.String() + "::" + userID.String()
}

func (s *Service) BindRole(workspaceID, userID uuid.UUID, role string) error {
	if _, ok := roleRank[role]; !ok {
		return fmt.Errorf("invalid role: %s", role)
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.bindings[bindingKey(workspaceID, userID)] = Binding{WorkspaceID: workspaceID, UserID: userID, Role: role}
	return nil
}

func (s *Service) RevokeRole(workspaceID, userID uuid.UUID) {
	s.mu.Lock()
	defer s.mu.Unlock()
	key := bindingKey(workspaceID, userID)
	binding, ok := s.bindings[key]
	if !ok {
		return
	}
	binding.Revoked = true
	s.bindings[key] = binding
}

func (s *Service) CheckRoleAtLeast(workspaceID, userID uuid.UUID, minimumRole string) error {
	minimumRank, ok := roleRank[minimumRole]
	if !ok {
		return fmt.Errorf("invalid minimum role: %s", minimumRole)
	}

	s.mu.RLock()
	defer s.mu.RUnlock()
	binding, ok := s.bindings[bindingKey(workspaceID, userID)]
	if !ok || binding.Revoked {
		return fmt.Errorf("role binding not found")
	}
	if roleRank[binding.Role] < minimumRank {
		return fmt.Errorf("insufficient role: have %s need %s", binding.Role, minimumRole)
	}
	return nil
}
