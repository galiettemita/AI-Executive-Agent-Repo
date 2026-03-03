package audit

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
)

type MutationEntry struct {
	ID          string         `json:"id"`
	WorkspaceID string         `json:"workspace_id"`
	Actor       string         `json:"actor"`
	Action      string         `json:"action"`
	Resource    string         `json:"resource"`
	Timestamp   string         `json:"timestamp"`
	Before      map[string]any `json:"before,omitempty"`
	After       map[string]any `json:"after,omitempty"`
	PrevHash    string         `json:"prev_hash"`
	Hash        string         `json:"hash"`
}

type MutationInput struct {
	WorkspaceID string
	Actor       string
	Action      string
	Resource    string
	Before      map[string]any
	After       map[string]any
}

type Service struct {
	mu                  sync.RWMutex
	lastHashByWorkspace map[string]string
	entriesByWorkspace  map[string][]MutationEntry
	now                 func() time.Time
}

func NewService() *Service {
	return &Service{
		lastHashByWorkspace: map[string]string{},
		entriesByWorkspace:  map[string][]MutationEntry{},
		now:                 func() time.Time { return time.Now().UTC() },
	}
}

func (s *Service) AppendMutation(input MutationInput) MutationEntry {
	s.mu.Lock()
	defer s.mu.Unlock()

	workspaceID := strings.TrimSpace(input.WorkspaceID)
	if workspaceID == "" {
		workspaceID = "default"
	}
	actor := strings.TrimSpace(input.Actor)
	if actor == "" {
		actor = "system"
	}
	action := strings.TrimSpace(input.Action)
	if action == "" {
		action = "unknown"
	}
	resource := strings.TrimSpace(input.Resource)
	if resource == "" {
		resource = "unknown"
	}

	entryID := uuid.Must(uuid.NewV7())
	timestamp := s.now().UTC().Format(time.RFC3339)
	prevHash := s.lastHashByWorkspace[workspaceID]
	payload := map[string]any{
		"id":           entryID.String(),
		"workspace_id": workspaceID,
		"actor":        actor,
		"action":       action,
		"resource":     resource,
		"timestamp":    timestamp,
		"before":       input.Before,
		"after":        input.After,
		"prev_hash":    prevHash,
	}
	encoded, err := json.Marshal(payload)
	if err != nil {
		encoded = []byte(workspaceID + ":" + actor + ":" + action + ":" + resource + ":" + timestamp + ":" + prevHash)
	}
	sum := sha256.Sum256(encoded)
	hash := hex.EncodeToString(sum[:])

	entry := MutationEntry{
		ID:          entryID.String(),
		WorkspaceID: workspaceID,
		Actor:       actor,
		Action:      action,
		Resource:    resource,
		Timestamp:   timestamp,
		Before:      input.Before,
		After:       input.After,
		PrevHash:    prevHash,
		Hash:        hash,
	}
	s.entriesByWorkspace[workspaceID] = append(s.entriesByWorkspace[workspaceID], entry)
	s.lastHashByWorkspace[workspaceID] = hash
	return entry
}

func (s *Service) ListMutations(workspaceID string) []MutationEntry {
	s.mu.RLock()
	defer s.mu.RUnlock()

	workspaceID = strings.TrimSpace(workspaceID)
	if workspaceID == "" {
		workspaceID = "default"
	}

	entries := s.entriesByWorkspace[workspaceID]
	out := make([]MutationEntry, len(entries))
	copy(out, entries)
	return out
}

func (s *Service) Count(workspaceID string) int {
	s.mu.RLock()
	defer s.mu.RUnlock()

	workspaceID = strings.TrimSpace(workspaceID)
	if workspaceID == "" {
		workspaceID = "default"
	}
	return len(s.entriesByWorkspace[workspaceID])
}
