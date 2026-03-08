package cognition

import (
	"testing"
)

func TestCreateGraph(t *testing.T) {
	e := NewGoTEngine()
	g, err := e.CreateGraph("What is the best approach?")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if g.ID == "" || g.RootID == "" {
		t.Fatal("expected non-empty IDs")
	}
	if len(g.Nodes) != 1 {
		t.Fatalf("expected 1 node, got %d", len(g.Nodes))
	}
}

func TestCreateGraphEmptyContent(t *testing.T) {
	e := NewGoTEngine()
	_, err := e.CreateGraph("")
	if err == nil {
		t.Fatal("expected error for empty content")
	}
}

func TestBranch(t *testing.T) {
	e := NewGoTEngine()
	g, _ := e.CreateGraph("root")
	node, err := e.Branch(g.ID, g.RootID, "hypothesis A", "hypothesis")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if node.Type != "hypothesis" {
		t.Fatalf("expected type hypothesis, got %s", node.Type)
	}
	if len(node.Parents) != 1 || node.Parents[0] != g.RootID {
		t.Fatal("expected parent to be root")
	}
}

func TestBranchInvalidType(t *testing.T) {
	e := NewGoTEngine()
	g, _ := e.CreateGraph("root")
	_, err := e.Branch(g.ID, g.RootID, "test", "invalid_type")
	if err == nil {
		t.Fatal("expected error for invalid thought type")
	}
}

func TestBranchFromPrunedNode(t *testing.T) {
	e := NewGoTEngine()
	g, _ := e.CreateGraph("root")
	child, _ := e.Branch(g.ID, g.RootID, "child", "hypothesis")
	_ = e.Prune(g.ID, child.ID)
	_, err := e.Branch(g.ID, child.ID, "grandchild", "evidence")
	if err == nil {
		t.Fatal("expected error branching from pruned node")
	}
}

func TestMerge(t *testing.T) {
	e := NewGoTEngine()
	g, _ := e.CreateGraph("root")
	n1, _ := e.Branch(g.ID, g.RootID, "branch A", "evidence")
	n2, _ := e.Branch(g.ID, g.RootID, "branch B", "evidence")

	merged, err := e.Merge(g.ID, []string{n1.ID, n2.ID}, "synthesis of A and B")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if merged.Type != "conclusion" {
		t.Fatalf("expected conclusion type, got %s", merged.Type)
	}
	if len(merged.Parents) != 2 {
		t.Fatalf("expected 2 parents, got %d", len(merged.Parents))
	}
}

func TestMergeTooFewNodes(t *testing.T) {
	e := NewGoTEngine()
	g, _ := e.CreateGraph("root")
	n1, _ := e.Branch(g.ID, g.RootID, "only one", "evidence")
	_, err := e.Merge(g.ID, []string{n1.ID}, "not enough")
	if err == nil {
		t.Fatal("expected error for fewer than 2 nodes")
	}
}

func TestPrune(t *testing.T) {
	e := NewGoTEngine()
	g, _ := e.CreateGraph("root")
	child, _ := e.Branch(g.ID, g.RootID, "child", "hypothesis")
	_, _ = e.Branch(g.ID, child.ID, "grandchild", "evidence")

	err := e.Prune(g.ID, child.ID)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Both child and grandchild should be pruned
	e.mu.Lock()
	graph := e.graphs[g.ID]
	prunedCount := 0
	for _, n := range graph.Nodes {
		if n.Status == "pruned" {
			prunedCount++
		}
	}
	e.mu.Unlock()

	if prunedCount != 2 {
		t.Fatalf("expected 2 pruned nodes, got %d", prunedCount)
	}
}

func TestPruneRoot(t *testing.T) {
	e := NewGoTEngine()
	g, _ := e.CreateGraph("root")
	err := e.Prune(g.ID, g.RootID)
	if err == nil {
		t.Fatal("expected error pruning root")
	}
}

func TestEvaluate(t *testing.T) {
	e := NewGoTEngine()
	g, _ := e.CreateGraph("root")
	_, _ = e.Branch(g.ID, g.RootID, "A", "hypothesis")
	_, _ = e.Branch(g.ID, g.RootID, "B", "hypothesis")

	eval, err := e.Evaluate(g.ID)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if eval.BranchCount < 1 {
		t.Fatal("expected at least 1 branch")
	}
	if eval.Depth < 1 {
		t.Fatal("expected depth >= 1")
	}
}

func TestGetBestConclusion(t *testing.T) {
	e := NewGoTEngine()
	g, _ := e.CreateGraph("root")
	n1, _ := e.Branch(g.ID, g.RootID, "evidence 1", "evidence")
	n2, _ := e.Branch(g.ID, g.RootID, "evidence 2", "evidence")
	_, _ = e.Merge(g.ID, []string{n1.ID, n2.ID}, "final conclusion")

	best, err := e.GetBestConclusion(g.ID)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if best.Type != "conclusion" {
		t.Fatalf("expected conclusion, got %s", best.Type)
	}
}

func TestGetBestConclusionNone(t *testing.T) {
	e := NewGoTEngine()
	g, _ := e.CreateGraph("root")
	_, err := e.GetBestConclusion(g.ID)
	if err == nil {
		t.Fatal("expected error when no conclusions exist")
	}
}
