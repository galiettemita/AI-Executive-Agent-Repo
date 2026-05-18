package memory

import (
	"fmt"
	"strings"
	"sync"
)

// ColdStartProfile holds bootstrap configuration for new workspaces/users.
type ColdStartProfile struct {
	WorkspaceID        string
	UserID             string
	DefaultPreferences map[string]string
	BootstrapComplete  bool
}

// ColdStartService manages cold-start mitigation for new users.
type ColdStartService struct {
	mu       sync.Mutex
	profiles map[string]ColdStartProfile // keyed by workspace_id::user_id
}

// NewColdStartService creates a new ColdStartService.
func NewColdStartService() *ColdStartService {
	return &ColdStartService{
		profiles: map[string]ColdStartProfile{},
	}
}

func coldStartKey(workspaceID, userID string) string {
	return workspaceID + "::" + userID
}

// InitializeDefaults creates a cold-start profile with default preferences.
func (cs *ColdStartService) InitializeDefaults(workspaceID, userID string) (ColdStartProfile, error) {
	if strings.TrimSpace(workspaceID) == "" {
		return ColdStartProfile{}, fmt.Errorf("workspace_id is required")
	}
	if strings.TrimSpace(userID) == "" {
		return ColdStartProfile{}, fmt.Errorf("user_id is required")
	}

	cs.mu.Lock()
	defer cs.mu.Unlock()

	key := coldStartKey(workspaceID, userID)
	if existing, ok := cs.profiles[key]; ok {
		return existing, nil
	}

	profile := ColdStartProfile{
		WorkspaceID: workspaceID,
		UserID:      userID,
		DefaultPreferences: map[string]string{
			"response_length": "medium",
			"formality":       "professional",
			"detail_level":    "moderate",
		},
		BootstrapComplete: false,
	}
	cs.profiles[key] = profile
	return profile, nil
}

// IsBootstrapComplete returns whether the bootstrap process is complete for a user.
func (cs *ColdStartService) IsBootstrapComplete(workspaceID, userID string) bool {
	cs.mu.Lock()
	defer cs.mu.Unlock()
	profile, ok := cs.profiles[coldStartKey(workspaceID, userID)]
	if !ok {
		return false
	}
	return profile.BootstrapComplete
}

// GetDefaultPreferences returns the default preferences for a user.
func (cs *ColdStartService) GetDefaultPreferences(workspaceID, userID string) map[string]string {
	cs.mu.Lock()
	defer cs.mu.Unlock()
	profile, ok := cs.profiles[coldStartKey(workspaceID, userID)]
	if !ok {
		return nil
	}
	out := make(map[string]string, len(profile.DefaultPreferences))
	for k, v := range profile.DefaultPreferences {
		out[k] = v
	}
	return out
}

// MarkBootstrapComplete marks the bootstrap process as complete for a user.
func (cs *ColdStartService) MarkBootstrapComplete(workspaceID, userID string) error {
	cs.mu.Lock()
	defer cs.mu.Unlock()
	key := coldStartKey(workspaceID, userID)
	profile, ok := cs.profiles[key]
	if !ok {
		return fmt.Errorf("profile not found for %s", key)
	}
	profile.BootstrapComplete = true
	cs.profiles[key] = profile
	return nil
}
