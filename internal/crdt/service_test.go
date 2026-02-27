package crdt

import "testing"

func TestCRDTConflictLifecycle(t *testing.T) {
	s := NewService()
	if _, conflict := s.Apply("memory_1", "actor_a", 1, "alpha"); conflict {
		t.Fatalf("unexpected conflict on first write")
	}
	if _, conflict := s.Apply("memory_1", "actor_a", 1, "beta"); !conflict {
		t.Fatalf("expected conflict for stale write")
	}
	conflicts := s.ListConflicts()
	if len(conflicts) != 1 {
		t.Fatalf("expected one conflict, got %d", len(conflicts))
	}
	state, ok := s.ResolveConflict(conflicts[0].ID, "resolved_value")
	if !ok {
		t.Fatalf("expected conflict resolution success")
	}
	if state.Value != "resolved_value" {
		t.Fatalf("unexpected resolved state: %#v", state)
	}
}
