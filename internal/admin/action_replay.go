package admin

import (
	"encoding/json"
	"fmt"
	"sort"
	"sync"
	"time"

	"github.com/google/uuid"
)

// ActionReplayEntry represents a recorded agent action for replay.
type ActionReplayEntry struct {
	ID            string          `json:"id"`
	WorkspaceID   string          `json:"workspace_id"`
	UserID        string          `json:"user_id"`
	ActionType    string          `json:"action_type"`
	ActionPayload json.RawMessage `json:"action_payload"`
	Timestamp     time.Time       `json:"timestamp"`
	ReplayStatus  string          `json:"replay_status"` // recorded, replaying, replayed, failed
}

// ActionReplayService provides agent action recording and replay.
type ActionReplayService struct {
	mu      sync.Mutex
	entries map[string]*ActionReplayEntry // key: ID
	now     func() time.Time
}

// NewActionReplayService creates a new ActionReplayService.
func NewActionReplayService() *ActionReplayService {
	return &ActionReplayService{
		entries: map[string]*ActionReplayEntry{},
		now:     func() time.Time { return time.Now().UTC() },
	}
}

// RecordAction records an agent action for later replay.
func (s *ActionReplayService) RecordAction(entry ActionReplayEntry) (ActionReplayEntry, error) {
	if entry.WorkspaceID == "" {
		return ActionReplayEntry{}, fmt.Errorf("workspace_id is required")
	}
	if entry.UserID == "" {
		return ActionReplayEntry{}, fmt.Errorf("user_id is required")
	}
	if entry.ActionType == "" {
		return ActionReplayEntry{}, fmt.Errorf("action_type is required")
	}
	s.mu.Lock()
	defer s.mu.Unlock()

	entry.ID = uuid.Must(uuid.NewV7()).String()
	if entry.Timestamp.IsZero() {
		entry.Timestamp = s.now()
	}
	entry.ReplayStatus = "recorded"
	s.entries[entry.ID] = &entry
	return entry, nil
}

// ReplayAction marks an action as replayed. In a real implementation this
// would re-execute the action; here we simulate by updating the status.
func (s *ActionReplayService) ReplayAction(id string) (ActionReplayEntry, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	entry, ok := s.entries[id]
	if !ok {
		return ActionReplayEntry{}, fmt.Errorf("action %s not found", id)
	}
	if entry.ReplayStatus == "replayed" {
		return ActionReplayEntry{}, fmt.Errorf("action %s already replayed", id)
	}
	entry.ReplayStatus = "replayed"
	return *entry, nil
}

// GetReplayLog returns all recorded actions for a workspace, sorted by timestamp.
func (s *ActionReplayService) GetReplayLog(workspaceID string) []ActionReplayEntry {
	s.mu.Lock()
	defer s.mu.Unlock()

	var result []ActionReplayEntry
	for _, e := range s.entries {
		if e.WorkspaceID == workspaceID {
			result = append(result, *e)
		}
	}
	sort.Slice(result, func(i, j int) bool {
		return result[i].Timestamp.Before(result[j].Timestamp)
	})
	return result
}
