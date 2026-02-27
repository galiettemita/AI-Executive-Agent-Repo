package memory

import "testing"

func TestWriteGateEnforcesExclusionRules(t *testing.T) {
	t.Parallel()

	svc := NewService()
	svc.AddExclusionRule("ws1", "u1", "do not store")
	_, err := svc.Write("ws1", "u1", "preference", "This says do not store secret")
	if err == nil {
		t.Fatal("expected exclusion rule to block write")
	}
}

func TestRetrievalWorkspaceIsolation(t *testing.T) {
	t.Parallel()

	svc := NewService()
	if _, err := svc.Write("ws1", "u1", "semantic", "alpha"); err != nil {
		t.Fatalf("write ws1: %v", err)
	}
	if _, err := svc.Write("ws2", "u2", "semantic", "beta"); err != nil {
		t.Fatalf("write ws2: %v", err)
	}

	items := svc.Retrieve("ws1")
	if len(items) != 1 {
		t.Fatalf("expected 1 item for ws1, got %d", len(items))
	}
	if items[0].WorkspaceID != "ws1" {
		t.Fatalf("unexpected workspace in retrieval: %s", items[0].WorkspaceID)
	}
}

func TestConsolidationMergesDuplicates(t *testing.T) {
	t.Parallel()

	svc := NewService()
	_, _ = svc.Write("ws1", "u1", "semantic", "buy milk")
	_, _ = svc.Write("ws1", "u1", "semantic", "Buy Milk")
	_, _ = svc.Write("ws1", "u1", "semantic", "buy eggs")

	consolidated := svc.Consolidate("ws1")
	if len(consolidated) != 2 {
		t.Fatalf("expected 2 unique items, got %d", len(consolidated))
	}
}
