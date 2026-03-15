package cognition

import (
	"context"
	"fmt"
	"testing"

	"github.com/brevio/brevio/internal/llm"
)

type mockGoTLLM struct {
	responses []string
	callIdx   int
	err       error
}

func (m *mockGoTLLM) Generate(_ context.Context, _ llm.GenerateRequest) (*llm.GenerateResponse, *llm.Usage, error) {
	if m.err != nil {
		return nil, nil, m.err
	}
	idx := m.callIdx
	m.callIdx++
	if idx < len(m.responses) {
		return &llm.GenerateResponse{Content: m.responses[idx]}, &llm.Usage{}, nil
	}
	return &llm.GenerateResponse{Content: m.responses[0]}, &llm.Usage{}, nil
}

func (m *mockGoTLLM) Stream(_ context.Context, _ llm.GenerateRequest, out chan<- llm.StreamChunk) {
	defer close(out)
}

func TestGoTLLMEngine_Run_ProducesConclusion(t *testing.T) {
	mock := &mockGoTLLM{responses: []string{
		`[{"content":"approach A: direct","score":0.8},{"content":"approach B: indirect","score":0.6}]`,
		`X is the best approach combining direct and indirect methods.`,
	}}
	engine := NewGoTLLMEngine(GoTLLMConfig{
		LLMClient:      mock,
		MaxBranches:    2,
		PruneThreshold: 0.35,
		EnableMerge:    true,
	})

	result, err := engine.Run(context.Background(), GoTRunRequest{
		Question: "How should I approach this project?",
		MaxSteps: 1,
	})
	if err != nil {
		t.Fatal(err)
	}
	if result.Conclusion == "" {
		t.Fatal("expected non-empty conclusion")
	}
	if result.Confidence <= 0 {
		t.Fatalf("expected positive confidence, got %v", result.Confidence)
	}
}

func TestGoTLLMEngine_Run_PrunesLowConfidence(t *testing.T) {
	mock := &mockGoTLLM{responses: []string{
		`[{"content":"good","score":0.8},{"content":"bad","score":0.2},{"content":"worse","score":0.1}]`,
	}}
	engine := NewGoTLLMEngine(GoTLLMConfig{
		LLMClient:      mock,
		MaxBranches:    3,
		PruneThreshold: 0.35,
	})

	result, err := engine.Run(context.Background(), GoTRunRequest{
		Question: "test pruning",
		MaxSteps: 1,
	})
	if err != nil {
		t.Fatal(err)
	}
	if result.PrunedCount < 2 {
		t.Fatalf("expected >= 2 pruned, got %d", result.PrunedCount)
	}
}

func TestGoTLLMEngine_Run_FallbackOnLLMFailure(t *testing.T) {
	mock := &mockGoTLLM{err: fmt.Errorf("LLM down")}
	engine := NewGoTLLMEngine(GoTLLMConfig{LLMClient: mock})

	result, err := engine.Run(context.Background(), GoTRunRequest{
		Question: "test failure handling",
		MaxSteps: 1,
	})
	if err != nil {
		t.Fatal(err)
	}
	if result == nil {
		t.Fatal("expected non-nil result even on LLM failure")
	}
	// Should fall back to root content
	if result.Conclusion == "" {
		t.Fatal("expected root content as fallback conclusion")
	}
}

func TestGoTEngine_GetNode_Found(t *testing.T) {
	engine := NewGoTEngine()
	graph, _ := engine.CreateGraph("test root")
	node, ok := engine.GetNode(graph.ID, graph.RootID)
	if !ok {
		t.Fatal("expected root node found")
	}
	if node.Content != "test root" {
		t.Fatalf("expected 'test root', got %q", node.Content)
	}
}

func TestGoTEngine_GetNode_NotFound(t *testing.T) {
	engine := NewGoTEngine()
	graph, _ := engine.CreateGraph("test")
	_, ok := engine.GetNode(graph.ID, "nonexistent")
	if ok {
		t.Fatal("expected not found")
	}
}

func TestGoTEngine_SetNodeConfidence_Valid(t *testing.T) {
	engine := NewGoTEngine()
	graph, _ := engine.CreateGraph("test")
	err := engine.SetNodeConfidence(graph.ID, graph.RootID, 0.9)
	if err != nil {
		t.Fatal(err)
	}
	node, _ := engine.GetNode(graph.ID, graph.RootID)
	if node.Confidence != 0.9 {
		t.Fatalf("expected 0.9, got %v", node.Confidence)
	}
}

func TestGoTEngine_SetNodeConfidence_OutOfRange(t *testing.T) {
	engine := NewGoTEngine()
	graph, _ := engine.CreateGraph("test")
	err := engine.SetNodeConfidence(graph.ID, graph.RootID, 1.5)
	if err == nil {
		t.Fatal("expected error for confidence > 1")
	}
}

func TestGoTEngine_SetNodeConfidence_PrunedNode(t *testing.T) {
	engine := NewGoTEngine()
	graph, _ := engine.CreateGraph("test")
	node, _ := engine.Branch(graph.ID, graph.RootID, "branch", "hypothesis")
	_ = engine.Prune(graph.ID, node.ID)
	err := engine.SetNodeConfidence(graph.ID, node.ID, 0.5)
	if err == nil {
		t.Fatal("expected error for pruned node")
	}
}

func TestGoTLLMEngine_TopNodes_SortsByConfidence(t *testing.T) {
	engine := NewGoTLLMEngine(GoTLLMConfig{LLMClient: &mockGoTLLM{responses: []string{"[]"}}})
	inner := engine.inner
	graph, _ := inner.CreateGraph("test")

	n1, _ := inner.Branch(graph.ID, graph.RootID, "low", "hypothesis")
	_ = inner.SetNodeConfidence(graph.ID, n1.ID, 0.3)
	n2, _ := inner.Branch(graph.ID, graph.RootID, "high", "hypothesis")
	_ = inner.SetNodeConfidence(graph.ID, n2.ID, 0.9)
	n3, _ := inner.Branch(graph.ID, graph.RootID, "mid", "hypothesis")
	_ = inner.SetNodeConfidence(graph.ID, n3.ID, 0.6)

	top := engine.TopNodes(graph.ID, []string{n1.ID, n2.ID, n3.ID}, 2)
	if len(top) != 2 {
		t.Fatalf("expected 2 top nodes, got %d", len(top))
	}
	if top[0] != n2.ID {
		t.Fatalf("expected highest conf node first, got %s", top[0])
	}
	if top[1] != n3.ID {
		t.Fatalf("expected second highest conf node second, got %s", top[1])
	}
}
