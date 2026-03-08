package admin

import (
	"fmt"
	"sync"
	"time"

	"github.com/google/uuid"
)

// SkillACLOverride represents a per-user, per-skill access control override.
type SkillACLOverride struct {
	ID          string    `json:"id"`
	WorkspaceID string    `json:"workspace_id"`
	UserID      string    `json:"user_id"`
	SkillID     string    `json:"skill_id"`
	Allowed     bool      `json:"allowed"`
	Reason      string    `json:"reason"`
	CreatedAt   time.Time `json:"created_at"`
}

// SkillACLOverrideService manages per-user skill ACL overrides.
type SkillACLOverrideService struct {
	mu        sync.Mutex
	overrides map[string]*SkillACLOverride // key: workspaceID:userID:skillID
	now       func() time.Time
}

// NewSkillACLOverrideService creates a new SkillACLOverrideService.
func NewSkillACLOverrideService() *SkillACLOverrideService {
	return &SkillACLOverrideService{
		overrides: map[string]*SkillACLOverride{},
		now:       func() time.Time { return time.Now().UTC() },
	}
}

func skillACLOverrideKey(workspaceID, userID, skillID string) string {
	return workspaceID + ":" + userID + ":" + skillID
}

// SetOverride creates or updates a skill ACL override.
func (s *SkillACLOverrideService) SetOverride(override SkillACLOverride) (SkillACLOverride, error) {
	if override.WorkspaceID == "" {
		return SkillACLOverride{}, fmt.Errorf("workspace_id is required")
	}
	if override.UserID == "" {
		return SkillACLOverride{}, fmt.Errorf("user_id is required")
	}
	if override.SkillID == "" {
		return SkillACLOverride{}, fmt.Errorf("skill_id is required")
	}
	s.mu.Lock()
	defer s.mu.Unlock()

	key := skillACLOverrideKey(override.WorkspaceID, override.UserID, override.SkillID)
	override.ID = uuid.Must(uuid.NewV7()).String()
	if override.CreatedAt.IsZero() {
		override.CreatedAt = s.now()
	}
	s.overrides[key] = &override
	return override, nil
}

// RemoveOverride removes a skill ACL override.
func (s *SkillACLOverrideService) RemoveOverride(workspaceID, userID, skillID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	key := skillACLOverrideKey(workspaceID, userID, skillID)
	if _, ok := s.overrides[key]; !ok {
		return fmt.Errorf("override not found for %s", key)
	}
	delete(s.overrides, key)
	return nil
}

// IsSkillAllowed checks whether a skill is allowed for a user. Returns
// (allowed, hasOverride). If no override exists, returns (true, false)
// indicating default allow with no override present.
func (s *SkillACLOverrideService) IsSkillAllowed(workspaceID, userID, skillID string) (bool, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()

	key := skillACLOverrideKey(workspaceID, userID, skillID)
	override, ok := s.overrides[key]
	if !ok {
		return true, false // default allow, no override
	}
	return override.Allowed, true
}

// GetUserOverrides returns all overrides for a user in a workspace.
func (s *SkillACLOverrideService) GetUserOverrides(workspaceID, userID string) []SkillACLOverride {
	s.mu.Lock()
	defer s.mu.Unlock()

	var result []SkillACLOverride
	for _, o := range s.overrides {
		if o.WorkspaceID == workspaceID && o.UserID == userID {
			result = append(result, *o)
		}
	}
	return result
}
