package admin

import (
	"fmt"
	"sync"
	"time"

	"github.com/google/uuid"
)

// KillSwitch represents an agent kill switch record.
type KillSwitch struct {
	ID            string     `json:"id"`
	WorkspaceID   string     `json:"workspace_id"`
	UserID        string     `json:"user_id"`
	ActivatedBy   string     `json:"activated_by"`
	Reason        string     `json:"reason"`
	ActivatedAt   time.Time  `json:"activated_at"`
	DeactivatedAt *time.Time `json:"deactivated_at,omitempty"`
}

// KillSwitchService manages agent kill switches. Kill switches must be
// checked FIRST in the execution gate chain — when active, all agent
// execution for the workspace (or user) is halted.
type KillSwitchService struct {
	mu       sync.Mutex
	switches map[string]*KillSwitch // key: workspaceID or workspaceID:userID
	now      func() time.Time
}

// NewKillSwitchService creates a new KillSwitchService.
func NewKillSwitchService() *KillSwitchService {
	return &KillSwitchService{
		switches: map[string]*KillSwitch{},
		now:      func() time.Time { return time.Now().UTC() },
	}
}

func killSwitchKey(workspaceID, userID string) string {
	if userID == "" {
		return workspaceID
	}
	return workspaceID + ":" + userID
}

// Activate activates a kill switch for a workspace (and optionally a specific user).
func (s *KillSwitchService) Activate(workspaceID, userID, activatedBy, reason string) (KillSwitch, error) {
	if workspaceID == "" {
		return KillSwitch{}, fmt.Errorf("workspace_id is required")
	}
	if activatedBy == "" {
		return KillSwitch{}, fmt.Errorf("activated_by is required")
	}
	s.mu.Lock()
	defer s.mu.Unlock()

	key := killSwitchKey(workspaceID, userID)
	if existing, ok := s.switches[key]; ok && existing.DeactivatedAt == nil {
		return KillSwitch{}, fmt.Errorf("kill switch already active for %s", key)
	}

	ks := &KillSwitch{
		ID:          uuid.Must(uuid.NewV7()).String(),
		WorkspaceID: workspaceID,
		UserID:      userID,
		ActivatedBy: activatedBy,
		Reason:      reason,
		ActivatedAt: s.now(),
	}
	s.switches[key] = ks
	return *ks, nil
}

// Deactivate deactivates a kill switch.
func (s *KillSwitchService) Deactivate(workspaceID, userID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	key := killSwitchKey(workspaceID, userID)
	ks, ok := s.switches[key]
	if !ok {
		return fmt.Errorf("no kill switch found for %s", key)
	}
	if ks.DeactivatedAt != nil {
		return fmt.Errorf("kill switch already deactivated for %s", key)
	}
	now := s.now()
	ks.DeactivatedAt = &now
	return nil
}

// IsActive returns true if a kill switch is active for the workspace or user.
// This should be checked FIRST in the execution gate chain.
func (s *KillSwitchService) IsActive(workspaceID, userID string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Check workspace-level kill switch first
	if ks, ok := s.switches[workspaceID]; ok && ks.DeactivatedAt == nil {
		return true
	}
	// Check user-level kill switch
	if userID != "" {
		key := killSwitchKey(workspaceID, userID)
		if ks, ok := s.switches[key]; ok && ks.DeactivatedAt == nil {
			return true
		}
	}
	return false
}

// GetAll returns all kill switch records.
func (s *KillSwitchService) GetAll() []KillSwitch {
	s.mu.Lock()
	defer s.mu.Unlock()

	result := make([]KillSwitch, 0, len(s.switches))
	for _, ks := range s.switches {
		result = append(result, *ks)
	}
	return result
}
