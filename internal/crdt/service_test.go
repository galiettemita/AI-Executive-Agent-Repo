package crdt

import "testing"

func TestCRDTConflictLifecycle(t *testing.T) {
	t.Parallel()

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
	if conflicts[0].ResolutionStrategy != "last_writer_wins" {
		t.Fatalf("unexpected stale conflict strategy: %s", conflicts[0].ResolutionStrategy)
	}

	state, ok := s.ResolveConflict(conflicts[0].ID, "resolved_value")
	if !ok {
		t.Fatalf("expected conflict resolution success")
	}
	if state.Value != "resolved_value" {
		t.Fatalf("unexpected resolved state: %#v", state)
	}
}

func TestApplyWithStrategyMergeConcatResolvesConcurrentWrites(t *testing.T) {
	t.Parallel()

	s := NewService()
	if _, conflict := s.ApplyWithStrategy("ws_crdt", "memory_2", VectorClock{"actor_a": 1}, "alpha", "manual_review"); conflict != nil {
		t.Fatalf("unexpected conflict for first write: %+v", conflict)
	}

	state, conflict := s.ApplyWithStrategy("ws_crdt", "memory_2", VectorClock{"actor_b": 1}, "beta", "merge_concat")
	if conflict != nil {
		t.Fatalf("unexpected conflict for merge strategy: %+v", conflict)
	}
	if state.Value != "alpha | beta" {
		t.Fatalf("unexpected merged value: %s", state.Value)
	}
	if state.VectorClock["actor_a"] != 1 || state.VectorClock["actor_b"] != 1 {
		t.Fatalf("expected merged vector clock, got %+v", state.VectorClock)
	}
}

func TestApplyWithStrategyManualReviewConflictAndReport(t *testing.T) {
	t.Parallel()

	s := NewService()
	if _, conflict := s.ApplyWithStrategy("ws_crdt", "memory_3", VectorClock{"actor_a": 2}, "local", "manual_review"); conflict != nil {
		t.Fatalf("unexpected conflict on first write: %+v", conflict)
	}

	_, conflict := s.ApplyWithStrategy("ws_crdt", "memory_3", VectorClock{"actor_b": 1}, "remote", "manual_review")
	if conflict == nil {
		t.Fatal("expected conflict for concurrent write")
	}
	if !conflict.RequiresManualReview {
		t.Fatalf("expected manual review requirement: %+v", conflict)
	}
	if conflict.ResolutionStrategy != "manual_review" {
		t.Fatalf("unexpected strategy: %s", conflict.ResolutionStrategy)
	}

	reports := s.ListConflictReports("ws_crdt")
	if len(reports) != 1 {
		t.Fatalf("expected one conflict report, got %d", len(reports))
	}
	if reports[0].EntityKey != "memory_3" {
		t.Fatalf("unexpected report entity key: %+v", reports[0])
	}

	state, ok := s.ResolveConflictWithStrategy(conflict.ID, "manual_review", "resolved")
	if !ok {
		t.Fatal("expected manual-review conflict resolution success")
	}
	if state.Value != "resolved" {
		t.Fatalf("unexpected resolved value: %s", state.Value)
	}

	allConflicts := s.ListConflicts()
	if len(allConflicts) != 1 {
		t.Fatalf("expected one conflict record, got %d", len(allConflicts))
	}
	if allConflicts[0].Status != "resolved" {
		t.Fatalf("expected resolved status, got %s", allConflicts[0].Status)
	}
}
