package audit

import (
	"context"
	"errors"
	"testing"
	"time"
)

type fakeSink struct {
	entries []MutationEntry
	err     error
	closed  bool
}

func (s *fakeSink) PersistMutation(_ context.Context, entry MutationEntry) error {
	s.entries = append(s.entries, entry)
	return s.err
}

func (s *fakeSink) Close() error {
	s.closed = true
	return nil
}

func TestAppendMutationUsesDefaultsAndHashChain(t *testing.T) {
	t.Parallel()

	svc := NewService()
	now := time.Date(2026, 3, 3, 15, 0, 0, 0, time.UTC)
	svc.now = func() time.Time { return now }

	first := svc.AppendMutation(MutationInput{
		Action:   "compliance.dsr.create",
		Resource: "compliance.dsr",
	})
	second := svc.AppendMutation(MutationInput{
		Action:   "compliance.dsr.update",
		Resource: "compliance.dsr",
	})

	if first.WorkspaceID != "default" {
		t.Fatalf("expected default workspace, got %s", first.WorkspaceID)
	}
	if first.Actor != "system" {
		t.Fatalf("expected default actor, got %s", first.Actor)
	}
	if first.ID == "" || second.ID == "" {
		t.Fatal("expected UUID ids")
	}
	if first.Hash == "" || second.Hash == "" {
		t.Fatal("expected hash values")
	}
	if second.PrevHash != first.Hash {
		t.Fatalf("expected chained prev hash, got=%s want=%s", second.PrevHash, first.Hash)
	}
	if svc.Count("default") != 2 {
		t.Fatalf("expected mutation count=2, got=%d", svc.Count("default"))
	}
}

func TestListMutationsIsWorkspaceScoped(t *testing.T) {
	t.Parallel()

	svc := NewService()
	svc.AppendMutation(MutationInput{
		WorkspaceID: "ws_1",
		Actor:       "user_1",
		Action:      "feature_flag.upsert",
		Resource:    "feature_flag",
	})
	svc.AppendMutation(MutationInput{
		WorkspaceID: "ws_2",
		Actor:       "user_2",
		Action:      "compliance.framework.upsert",
		Resource:    "compliance.framework",
	})

	ws1 := svc.ListMutations("ws_1")
	ws2 := svc.ListMutations("ws_2")
	if len(ws1) != 1 || ws1[0].WorkspaceID != "ws_1" {
		t.Fatalf("unexpected ws_1 entries: %+v", ws1)
	}
	if len(ws2) != 1 || ws2[0].WorkspaceID != "ws_2" {
		t.Fatalf("unexpected ws_2 entries: %+v", ws2)
	}
}

func TestAppendMutationPersistsToSink(t *testing.T) {
	t.Parallel()

	sink := &fakeSink{}
	svc := NewService(WithSink(sink))
	entry := svc.AppendMutation(MutationInput{
		WorkspaceID: "ws_1",
		Actor:       "user_1",
		Action:      "feature_flag.upsert",
		Resource:    "feature_flag",
	})

	if len(sink.entries) != 1 {
		t.Fatalf("expected sink entry count=1, got=%d", len(sink.entries))
	}
	if sink.entries[0].ID != entry.ID {
		t.Fatalf("expected sink entry id=%s got=%s", entry.ID, sink.entries[0].ID)
	}
}

func TestAppendMutationRetainsSinkErrors(t *testing.T) {
	t.Parallel()

	sink := &fakeSink{err: errors.New("sink failed")}
	svc := NewService(WithSink(sink))
	svc.AppendMutation(MutationInput{
		Action:   "compliance.dsr.create",
		Resource: "compliance.dsr",
	})

	errs := svc.PersistErrors()
	if len(errs) != 1 {
		t.Fatalf("expected one persist error, got=%d errs=%v", len(errs), errs)
	}
}

func TestCloseClosesSink(t *testing.T) {
	t.Parallel()

	sink := &fakeSink{}
	svc := NewService(WithSink(sink))
	if err := svc.Close(); err != nil {
		t.Fatalf("unexpected close error: %v", err)
	}
	if !sink.closed {
		t.Fatal("expected sink to be closed")
	}
}
