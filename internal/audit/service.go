package audit

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
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

type MutationSink interface {
	PersistMutation(ctx context.Context, entry MutationEntry) error
	Close() error
}

type Option func(*Service)

type Service struct {
	mu                  sync.RWMutex
	lastHashByWorkspace map[string]string
	entriesByWorkspace  map[string][]MutationEntry
	sinks               []MutationSink
	persistErrors       []string
	now                 func() time.Time
	hmacSecret          []byte
}

func NewService(opts ...Option) *Service {
	svc := &Service{
		lastHashByWorkspace: map[string]string{},
		entriesByWorkspace:  map[string][]MutationEntry{},
		now:                 func() time.Time { return time.Now().UTC() },
	}
	for _, opt := range opts {
		if opt != nil {
			opt(svc)
		}
	}
	return svc
}

func WithSink(sink MutationSink) Option {
	return func(s *Service) {
		if sink == nil {
			return
		}
		s.sinks = append(s.sinks, sink)
	}
}

func WithHMACSecret(secret []byte) Option {
	return func(s *Service) {
		if len(secret) == 0 {
			return
		}
		cp := make([]byte, len(secret))
		copy(cp, secret)
		s.hmacSecret = cp
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
	hash := s.computeHash(encoded)

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
	for _, sink := range s.sinks {
		if sink == nil {
			continue
		}
		if err := sink.PersistMutation(context.Background(), entry); err != nil {
			s.recordPersistErrorLocked(entry.ID, err)
		}
	}
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

func (s *Service) PersistErrors() []string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]string, len(s.persistErrors))
	copy(out, s.persistErrors)
	return out
}

func (s *Service) Close() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	var errs []error
	for _, sink := range s.sinks {
		if sink == nil {
			continue
		}
		if err := sink.Close(); err != nil {
			errs = append(errs, err)
		}
	}
	if len(errs) == 0 {
		return nil
	}
	parts := make([]string, 0, len(errs))
	for _, err := range errs {
		parts = append(parts, err.Error())
	}
	return fmt.Errorf("close mutation sinks: %s", strings.Join(parts, "; "))
}

// computeHash returns an HMAC-SHA256 hex digest when an hmacSecret is configured,
// otherwise falls back to plain SHA256.
func (s *Service) computeHash(data []byte) string {
	if len(s.hmacSecret) > 0 {
		mac := hmac.New(sha256.New, s.hmacSecret)
		mac.Write(data)
		return hex.EncodeToString(mac.Sum(nil))
	}
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}

// VerifyChain re-computes every hash in a workspace's audit log and verifies
// chain integrity. It returns valid=true if the full chain is intact. If
// tampered, brokenAt indicates the zero-based index of the first broken entry.
func (s *Service) VerifyChain(workspaceID string) (valid bool, brokenAt int, err error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	workspaceID = strings.TrimSpace(workspaceID)
	if workspaceID == "" {
		workspaceID = "default"
	}

	entries := s.entriesByWorkspace[workspaceID]
	if len(entries) == 0 {
		return true, -1, nil
	}

	prevHash := ""
	for i, entry := range entries {
		if entry.PrevHash != prevHash {
			return false, i, nil
		}
		payload := map[string]any{
			"id":           entry.ID,
			"workspace_id": entry.WorkspaceID,
			"actor":        entry.Actor,
			"action":       entry.Action,
			"resource":     entry.Resource,
			"timestamp":    entry.Timestamp,
			"before":       entry.Before,
			"after":        entry.After,
			"prev_hash":    entry.PrevHash,
		}
		encoded, jsonErr := json.Marshal(payload)
		if jsonErr != nil {
			encoded = []byte(entry.WorkspaceID + ":" + entry.Actor + ":" + entry.Action + ":" + entry.Resource + ":" + entry.Timestamp + ":" + entry.PrevHash)
		}
		computed := s.computeHash(encoded)
		if computed != entry.Hash {
			return false, i, nil
		}
		prevHash = entry.Hash
	}
	return true, -1, nil
}

func (s *Service) recordPersistErrorLocked(entryID string, err error) {
	if err == nil {
		return
	}
	s.persistErrors = append(s.persistErrors, entryID+": "+err.Error())
	const maxErrors = 50
	if len(s.persistErrors) > maxErrors {
		s.persistErrors = s.persistErrors[len(s.persistErrors)-maxErrors:]
	}
}
