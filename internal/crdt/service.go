package crdt

import (
	"fmt"
	"sort"
	"sync"
)

type VectorClock map[string]int

type ItemState struct {
	ItemID      string      `json:"item_id"`
	VectorClock VectorClock `json:"vector_clock"`
	Value       string      `json:"value"`
}

type Conflict struct {
	ID          string `json:"id"`
	ItemID      string `json:"item_id"`
	Reason      string `json:"reason"`
	LocalValue  string `json:"local_value"`
	RemoteValue string `json:"remote_value"`
	Status      string `json:"status"`
}

type Service struct {
	mu        sync.RWMutex
	nextID    int
	items     map[string]ItemState
	conflicts map[string]Conflict
}

func NewService() *Service {
	return &Service{
		nextID:    1,
		items:     map[string]ItemState{},
		conflicts: map[string]Conflict{},
	}
}

func (s *Service) Apply(itemID, actorID string, counter int, value string) (ItemState, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()

	state, ok := s.items[itemID]
	if !ok {
		state = ItemState{
			ItemID:      itemID,
			VectorClock: VectorClock{},
			Value:       value,
		}
	}

	currentCounter := state.VectorClock[actorID]
	if counter <= currentCounter && value != state.Value {
		conflict := Conflict{
			ID:          fmt.Sprintf("conflict_%06d", s.nextID),
			ItemID:      itemID,
			Reason:      "VECTOR_CLOCK_STALE_WRITE",
			LocalValue:  state.Value,
			RemoteValue: value,
			Status:      "open",
		}
		s.nextID++
		s.conflicts[conflict.ID] = conflict
		return state, true
	}

	state.VectorClock[actorID] = counter
	state.Value = value
	s.items[itemID] = state
	return state, false
}

func (s *Service) ListConflicts() []Conflict {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]Conflict, 0, len(s.conflicts))
	for _, conflict := range s.conflicts {
		out = append(out, conflict)
	}
	sort.Slice(out, func(i, j int) bool {
		return out[i].ID < out[j].ID
	})
	return out
}

func (s *Service) ResolveConflict(conflictID, resolutionValue string) (ItemState, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	conflict, ok := s.conflicts[conflictID]
	if !ok {
		return ItemState{}, false
	}
	state := s.items[conflict.ItemID]
	state.Value = resolutionValue
	s.items[conflict.ItemID] = state
	conflict.Status = "resolved"
	s.conflicts[conflictID] = conflict
	return state, true
}
