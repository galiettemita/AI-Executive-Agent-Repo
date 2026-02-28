package crdt

import (
	"fmt"
	"sort"
	"strings"
	"sync"
	"time"
)

type VectorClock map[string]int

type ItemState struct {
	ItemID      string      `json:"item_id"`
	WorkspaceID string      `json:"workspace_id"`
	EntityKey   string      `json:"entity_key"`
	VectorClock VectorClock `json:"vector_clock"`
	Value       string      `json:"value"`
	UpdatedAt   time.Time   `json:"updated_at"`
}

type Conflict struct {
	ID                   string      `json:"id"`
	WorkspaceID          string      `json:"workspace_id"`
	ItemID               string      `json:"item_id"`
	EntityKey            string      `json:"entity_key"`
	Reason               string      `json:"reason"`
	LocalValue           string      `json:"local_value"`
	RemoteValue          string      `json:"remote_value"`
	LocalClock           VectorClock `json:"local_clock"`
	RemoteClock          VectorClock `json:"remote_clock"`
	ResolutionStrategy   string      `json:"resolution_strategy"`
	RequiresManualReview bool        `json:"requires_manual_review"`
	Status               string      `json:"status"`
	CreatedAt            time.Time   `json:"created_at"`
	ResolvedAt           time.Time   `json:"resolved_at,omitempty"`
}

type ConflictReport struct {
	ConflictID           string `json:"conflict_id"`
	WorkspaceID          string `json:"workspace_id"`
	EntityKey            string `json:"entity_key"`
	ResolutionStrategy   string `json:"resolution_strategy"`
	RequiresManualReview bool   `json:"requires_manual_review"`
}

type Service struct {
	mu        sync.RWMutex
	nextID    int
	items     map[string]ItemState
	conflicts map[string]Conflict
	now       func() time.Time
}

func NewService() *Service {
	return &Service{
		nextID:    1,
		items:     map[string]ItemState{},
		conflicts: map[string]Conflict{},
		now:       func() time.Time { return time.Now().UTC() },
	}
}

// Apply preserves the original API and defaults to workspace=default with the
// last_writer_wins strategy.
func (s *Service) Apply(itemID, actorID string, counter int, value string) (ItemState, bool) {
	state, conflict := s.ApplyWithStrategy("default", itemID, VectorClock{actorID: counter}, value, "last_writer_wins")
	return state, conflict != nil
}

func (s *Service) ApplyWithStrategy(workspaceID, entityKey string, incomingClock VectorClock, value, strategy string) (ItemState, *Conflict) {
	s.mu.Lock()
	defer s.mu.Unlock()

	workspaceID = normalizeWorkspaceID(workspaceID)
	entityKey = normalizeEntityKey(entityKey)
	incomingClock = normalizeClock(incomingClock)
	itemKey := itemStoreKey(workspaceID, entityKey)

	state, exists := s.items[itemKey]
	if !exists {
		state = ItemState{
			ItemID:      entityKey,
			WorkspaceID: workspaceID,
			EntityKey:   entityKey,
			VectorClock: copyClock(incomingClock),
			Value:       value,
			UpdatedAt:   s.now(),
		}
		s.items[itemKey] = state
		return copyState(state), nil
	}

	resolvedStrategy := resolveStrategy(compareClocks(state.VectorClock, incomingClock), strategy)
	clockRelation := compareClocks(state.VectorClock, incomingClock)

	switch clockRelation {
	case relationRemoteDominates:
		state.VectorClock = mergeClock(state.VectorClock, incomingClock)
		state.Value = value
		state.UpdatedAt = s.now()
		s.items[itemKey] = state
		return copyState(state), nil
	case relationEqual:
		if state.Value == value {
			return copyState(state), nil
		}
		fallthrough
	case relationConcurrent:
		if resolvedStrategy == "merge_concat" {
			state.VectorClock = mergeClock(state.VectorClock, incomingClock)
			state.Value = mergeValues(state.Value, value)
			state.UpdatedAt = s.now()
			s.items[itemKey] = state
			return copyState(state), nil
		}
		conflict := s.createConflictLocked(state, value, incomingClock, resolvedStrategy, true, conflictReason(clockRelation))
		return copyState(state), &conflict
	case relationLocalDominates:
		if state.Value == value {
			return copyState(state), nil
		}
		conflict := s.createConflictLocked(state, value, incomingClock, resolvedStrategy, false, "VECTOR_CLOCK_STALE_WRITE")
		return copyState(state), &conflict
	default:
		return copyState(state), nil
	}
}

func (s *Service) ListConflicts() []Conflict {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]Conflict, 0, len(s.conflicts))
	for _, conflict := range s.conflicts {
		out = append(out, copyConflict(conflict))
	}
	sort.Slice(out, func(i, j int) bool {
		return out[i].ID < out[j].ID
	})
	return out
}

func (s *Service) ListConflictReports(workspaceID string) []ConflictReport {
	s.mu.RLock()
	defer s.mu.RUnlock()

	workspaceID = normalizeWorkspaceID(workspaceID)
	reports := make([]ConflictReport, 0, len(s.conflicts))
	for _, conflict := range s.conflicts {
		if conflict.WorkspaceID != workspaceID {
			continue
		}
		reports = append(reports, ConflictReport{
			ConflictID:           conflict.ID,
			WorkspaceID:          conflict.WorkspaceID,
			EntityKey:            conflict.EntityKey,
			ResolutionStrategy:   conflict.ResolutionStrategy,
			RequiresManualReview: conflict.RequiresManualReview,
		})
	}
	sort.Slice(reports, func(i, j int) bool {
		return reports[i].ConflictID < reports[j].ConflictID
	})
	return reports
}

func (s *Service) ResolveConflict(conflictID, resolutionValue string) (ItemState, bool) {
	return s.ResolveConflictWithStrategy(conflictID, "manual_review", resolutionValue)
}

func (s *Service) ResolveConflictWithStrategy(conflictID, strategy, resolutionValue string) (ItemState, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()

	conflict, ok := s.conflicts[conflictID]
	if !ok {
		return ItemState{}, false
	}
	itemKey := itemStoreKey(conflict.WorkspaceID, conflict.EntityKey)
	state, ok := s.items[itemKey]
	if !ok {
		return ItemState{}, false
	}

	resolvedStrategy := resolveStrategy(compareClocks(conflict.LocalClock, conflict.RemoteClock), strategy)
	switch resolvedStrategy {
	case "merge_concat":
		state.Value = mergeValues(conflict.LocalValue, conflict.RemoteValue)
	case "last_writer_wins":
		if strings.TrimSpace(resolutionValue) != "" {
			state.Value = resolutionValue
		} else {
			state.Value = conflict.LocalValue
		}
	default:
		if strings.TrimSpace(resolutionValue) == "" {
			state.Value = conflict.LocalValue
		} else {
			state.Value = resolutionValue
		}
	}

	state.VectorClock = mergeClock(state.VectorClock, conflict.RemoteClock)
	state.UpdatedAt = s.now()
	s.items[itemKey] = state

	conflict.Status = "resolved"
	conflict.RequiresManualReview = false
	conflict.ResolutionStrategy = resolvedStrategy
	conflict.ResolvedAt = s.now()
	s.conflicts[conflictID] = conflict

	return copyState(state), true
}

type clockRelation string

const (
	relationEqual           clockRelation = "equal"
	relationLocalDominates  clockRelation = "local_dominates"
	relationRemoteDominates clockRelation = "remote_dominates"
	relationConcurrent      clockRelation = "concurrent"
)

func compareClocks(local, remote VectorClock) clockRelation {
	allKeys := map[string]struct{}{}
	for actor := range local {
		allKeys[actor] = struct{}{}
	}
	for actor := range remote {
		allKeys[actor] = struct{}{}
	}

	localGE, remoteGE := true, true
	localGT, remoteGT := false, false
	for actor := range allKeys {
		localValue := local[actor]
		remoteValue := remote[actor]
		if localValue < remoteValue {
			localGE = false
			remoteGT = true
		}
		if remoteValue < localValue {
			remoteGE = false
			localGT = true
		}
	}

	switch {
	case localGE && remoteGE:
		return relationEqual
	case localGE && localGT:
		return relationLocalDominates
	case remoteGE && remoteGT:
		return relationRemoteDominates
	default:
		return relationConcurrent
	}
}

func resolveStrategy(relation clockRelation, requested string) string {
	normalized := strings.TrimSpace(strings.ToLower(requested))
	switch normalized {
	case "last_writer_wins", "merge_concat", "manual_review":
		return normalized
	}
	if relation == relationLocalDominates {
		return "last_writer_wins"
	}
	if relation == relationConcurrent || relation == relationEqual {
		return "manual_review"
	}
	return "last_writer_wins"
}

func conflictReason(relation clockRelation) string {
	switch relation {
	case relationEqual:
		return "VECTOR_CLOCK_EQUAL_DIVERGENT_VALUE"
	case relationConcurrent:
		return "VECTOR_CLOCK_CONCURRENT_WRITE"
	default:
		return "VECTOR_CLOCK_CONFLICT"
	}
}

func (s *Service) createConflictLocked(state ItemState, remoteValue string, remoteClock VectorClock, strategy string, requiresManualReview bool, reason string) Conflict {
	conflict := Conflict{
		ID:                   fmt.Sprintf("conflict_%06d", s.nextID),
		WorkspaceID:          state.WorkspaceID,
		ItemID:               state.ItemID,
		EntityKey:            state.EntityKey,
		Reason:               reason,
		LocalValue:           state.Value,
		RemoteValue:          remoteValue,
		LocalClock:           copyClock(state.VectorClock),
		RemoteClock:          copyClock(remoteClock),
		ResolutionStrategy:   strategy,
		RequiresManualReview: requiresManualReview,
		Status:               "open",
		CreatedAt:            s.now(),
	}
	s.nextID++
	s.conflicts[conflict.ID] = conflict
	return copyConflict(conflict)
}

func normalizeWorkspaceID(workspaceID string) string {
	if strings.TrimSpace(workspaceID) == "" {
		return "default"
	}
	return workspaceID
}

func normalizeEntityKey(entityKey string) string {
	if strings.TrimSpace(entityKey) == "" {
		return "entity_default"
	}
	return entityKey
}

func normalizeClock(clock VectorClock) VectorClock {
	if len(clock) == 0 {
		return VectorClock{"system": 1}
	}
	out := make(VectorClock, len(clock))
	for actor, counter := range clock {
		if strings.TrimSpace(actor) == "" {
			continue
		}
		if counter < 0 {
			counter = 0
		}
		out[actor] = counter
	}
	if len(out) == 0 {
		out["system"] = 1
	}
	return out
}

func mergeClock(local, remote VectorClock) VectorClock {
	merged := copyClock(local)
	for actor, counter := range remote {
		if counter > merged[actor] {
			merged[actor] = counter
		}
	}
	return merged
}

func mergeValues(localValue, remoteValue string) string {
	if strings.TrimSpace(localValue) == "" {
		return remoteValue
	}
	if strings.TrimSpace(remoteValue) == "" {
		return localValue
	}
	if localValue == remoteValue {
		return localValue
	}
	parts := []string{localValue, remoteValue}
	sort.Strings(parts)
	return parts[0] + " | " + parts[1]
}

func itemStoreKey(workspaceID, entityKey string) string {
	return workspaceID + "|" + entityKey
}

func copyClock(in VectorClock) VectorClock {
	out := make(VectorClock, len(in))
	for actor, counter := range in {
		out[actor] = counter
	}
	return out
}

func copyState(in ItemState) ItemState {
	out := in
	out.VectorClock = copyClock(in.VectorClock)
	return out
}

func copyConflict(in Conflict) Conflict {
	out := in
	out.LocalClock = copyClock(in.LocalClock)
	out.RemoteClock = copyClock(in.RemoteClock)
	return out
}
