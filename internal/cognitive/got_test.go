package cognitive

import (
	"context"
	"testing"
)

func TestNewGraphAndBranch(t *testing.T) {
	t.Parallel()

	got := NewGraphOfThought()
	graph := got.NewGraph("root thought", 3, 4)

	if graph.RootID == "" {
		t.Fatal("expected non-empty root ID")
	}
	if len(graph.Nodes) != 1 {
		t.Fatalf("expected 1 node, got %d", len(graph.Nodes))
	}

	nodes, err := got.Branch(graph, graph.RootID, []string{"branch A", "branch B", "branch C"})
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if len(nodes) != 3 {
		t.Fatalf("expected 3 branch nodes, got %d", len(nodes))
	}
	if len(graph.Nodes) != 4 {
		t.Fatalf("expected 4 total nodes, got %d", len(graph.Nodes))
	}
	for _, n := range nodes {
		if n.Depth != 1 {
			t.Fatalf("expected depth 1, got %d", n.Depth)
		}
		if n.ParentID != graph.RootID {
			t.Fatalf("expected parent to be root, got %s", n.ParentID)
		}
	}
}

func TestBranchRespectsMaxDepth(t *testing.T) {
	t.Parallel()

	got := NewGraphOfThought()
	graph := got.NewGraph("root", 1, 3)

	children, _ := got.Branch(graph, graph.RootID, []string{"child"})
	// children are at depth 1 == maxDepth, so branching from them should fail.
	_, err := got.Branch(graph, children[0].ID, []string{"grandchild"})
	if err == nil {
		t.Fatal("expected error when exceeding max depth")
	}
}

func TestMergeNodes(t *testing.T) {
	t.Parallel()

	got := NewGraphOfThought()
	graph := got.NewGraph("root", 3, 4)
	children, _ := got.Branch(graph, graph.RootID, []string{"A", "B", "C"})

	got.ScoreNode(graph, children[0].ID, 0.7)
	got.ScoreNode(graph, children[1].ID, 0.9)

	merged, err := got.Merge(graph, []string{children[0].ID, children[1].ID}, "synthesis of A and B")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if merged.Content != "synthesis of A and B" {
		t.Fatalf("unexpected merged content: %s", merged.Content)
	}
	// Average score.
	expectedScore := (0.7 + 0.9) / 2.0
	if merged.Score != expectedScore {
		t.Fatalf("expected score %f, got %f", expectedScore, merged.Score)
	}

	// Merge requires at least 2 nodes.
	_, err = got.Merge(graph, []string{children[2].ID}, "single")
	if err == nil {
		t.Fatal("expected error when merging fewer than 2 nodes")
	}
}

func TestPruneAndBestPath(t *testing.T) {
	t.Parallel()

	got := NewGraphOfThought()
	graph := got.NewGraph("root", 3, 4)
	children, _ := got.Branch(graph, graph.RootID, []string{"good path", "bad path"})

	got.ScoreNode(graph, children[0].ID, 0.9)
	got.ScoreNode(graph, children[1].ID, 0.1)

	got.Prune(graph, children[1].ID)

	path := got.BestPath(graph)
	if len(path) != 2 {
		t.Fatalf("expected path length 2, got %d", len(path))
	}
	if path[1].Content != "good path" {
		t.Fatalf("expected 'good path' in best path, got %s", path[1].Content)
	}

	// Cannot branch from pruned node.
	_, err := got.Branch(graph, children[1].ID, []string{"child"})
	if err == nil {
		t.Fatal("expected error branching from pruned node")
	}
}

func TestEvaluateFullCycle(t *testing.T) {
	t.Parallel()

	got := NewGraphOfThought()
	graph := got.NewGraph("What is the best approach to solve this problem?", 3, 4)
	got.Branch(graph, graph.RootID, []string{
		"Use a greedy algorithm for the optimization problem",
		"Apply dynamic programming to find optimal substructure",
		"Try brute force enumeration of all possibilities",
	})

	result, err := got.Evaluate(context.Background(), graph)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if result.FinalThought == "" {
		t.Fatal("expected non-empty final thought")
	}
	if result.NodesExplored == 0 {
		t.Fatal("expected some nodes explored")
	}

	// Nil graph.
	_, err = got.Evaluate(context.Background(), nil)
	if err == nil {
		t.Fatal("expected error for nil graph")
	}
}
