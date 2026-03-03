package audit

import (
	"testing"
	"time"
)

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
