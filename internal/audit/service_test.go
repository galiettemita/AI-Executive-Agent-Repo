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

func TestHMACChainProducesDifferentHashes(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 3, 3, 15, 0, 0, 0, time.UTC)

	plain := NewService()
	plain.now = func() time.Time { return now }

	hmacSvc := NewService(WithHMACSecret([]byte("test-secret-key-32bytes-long!!")))
	hmacSvc.now = func() time.Time { return now }

	input := MutationInput{
		WorkspaceID: "ws_hmac",
		Actor:       "user_1",
		Action:      "token.rotate",
		Resource:    "oauth_token",
	}

	plainEntry := plain.AppendMutation(input)
	hmacEntry := hmacSvc.AppendMutation(input)

	if plainEntry.Hash == hmacEntry.Hash {
		t.Fatal("HMAC hash should differ from plain SHA256 hash")
	}
	if hmacEntry.Hash == "" {
		t.Fatal("HMAC entry should have a hash")
	}
}

func TestVerifyChainIntact(t *testing.T) {
	t.Parallel()

	svc := NewService(WithHMACSecret([]byte("chain-verify-secret")))
	now := time.Date(2026, 3, 3, 15, 0, 0, 0, time.UTC)
	svc.now = func() time.Time { return now }

	for i := 0; i < 5; i++ {
		svc.AppendMutation(MutationInput{
			WorkspaceID: "ws_verify",
			Actor:       "user_1",
			Action:      "record.create",
			Resource:    "record",
		})
	}

	valid, brokenAt, err := svc.VerifyChain("ws_verify")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !valid {
		t.Fatalf("chain should be valid, brokenAt=%d", brokenAt)
	}
}

func TestVerifyChainDetectsTampering(t *testing.T) {
	t.Parallel()

	svc := NewService(WithHMACSecret([]byte("tamper-detect-secret")))
	now := time.Date(2026, 3, 3, 15, 0, 0, 0, time.UTC)
	svc.now = func() time.Time { return now }

	for i := 0; i < 5; i++ {
		svc.AppendMutation(MutationInput{
			WorkspaceID: "ws_tamper",
			Actor:       "user_1",
			Action:      "record.create",
			Resource:    "record",
		})
	}

	// Tamper with the third entry's hash.
	svc.mu.Lock()
	svc.entriesByWorkspace["ws_tamper"][2].Hash = "tampered_hash_value"
	svc.mu.Unlock()

	valid, brokenAt, err := svc.VerifyChain("ws_tamper")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if valid {
		t.Fatal("chain should be invalid after tampering")
	}
	if brokenAt != 2 {
		t.Fatalf("expected brokenAt=2 got=%d", brokenAt)
	}
}

func TestVerifyChainDetectsPrevHashTampering(t *testing.T) {
	t.Parallel()

	svc := NewService(WithHMACSecret([]byte("prev-hash-tamper")))
	now := time.Date(2026, 3, 3, 15, 0, 0, 0, time.UTC)
	svc.now = func() time.Time { return now }

	for i := 0; i < 3; i++ {
		svc.AppendMutation(MutationInput{
			WorkspaceID: "ws_prev",
			Actor:       "user_1",
			Action:      "record.create",
			Resource:    "record",
		})
	}

	// Tamper with prev_hash of the second entry.
	svc.mu.Lock()
	svc.entriesByWorkspace["ws_prev"][1].PrevHash = "wrong_prev_hash"
	svc.mu.Unlock()

	valid, brokenAt, _ := svc.VerifyChain("ws_prev")
	if valid {
		t.Fatal("chain should be invalid after prev_hash tampering")
	}
	if brokenAt != 1 {
		t.Fatalf("expected brokenAt=1 got=%d", brokenAt)
	}
}

func TestVerifyChainEmptyWorkspace(t *testing.T) {
	t.Parallel()

	svc := NewService()
	valid, brokenAt, err := svc.VerifyChain("nonexistent")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !valid {
		t.Fatalf("empty chain should be valid, brokenAt=%d", brokenAt)
	}
}
