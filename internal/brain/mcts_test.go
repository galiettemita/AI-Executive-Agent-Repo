package brain

import (
	"context"
	"fmt"
	"math"
	"testing"
	"time"

	"github.com/brevio/brevio/internal/llm"
)

type mockMCTSLLM struct {
	response string
	err      error
	temps    []float64
}

func (m *mockMCTSLLM) Generate(_ context.Context, req llm.GenerateRequest) (*llm.GenerateResponse, *llm.Usage, error) {
	m.temps = append(m.temps, req.Temperature)
	if m.err != nil {
		return nil, nil, m.err
	}
	return &llm.GenerateResponse{Content: m.response}, &llm.Usage{}, nil
}

func (m *mockMCTSLLM) Stream(_ context.Context, _ llm.GenerateRequest, out chan<- llm.StreamChunk) {
	defer close(out)
}

func TestMCTSNode_UCB1_UnvisitedIsInfinity(t *testing.T) {
	node := &MCTSNode{Visits: 0}
	score := node.UCB1(math.Sqrt2, 10)
	if !math.IsInf(score, 1) {
		t.Fatalf("expected +Inf for unvisited, got %v", score)
	}
}

func TestMCTSNode_UCB1_BalancesExploitExplore(t *testing.T) {
	// Low-visits child should have higher UCB1 than high-visits sibling (more explore)
	lowVisits := &MCTSNode{Visits: 2, TotalValue: 1.0}
	highVisits := &MCTSNode{Visits: 100, TotalValue: 80.0}
	parentVisits := 102
	ucbLow := lowVisits.UCB1(math.Sqrt2, parentVisits)
	ucbHigh := highVisits.UCB1(math.Sqrt2, parentVisits)
	if ucbLow <= ucbHigh {
		t.Fatalf("expected low-visits UCB1 (%v) > high-visits UCB1 (%v)", ucbLow, ucbHigh)
	}
}

func TestMCTSPlanner_Search_ReturnsPlan(t *testing.T) {
	mock := &mockMCTSLLM{response: `{"steps":[{"tool_key":"email_read","parameters":{},"phase":"gather"},{"tool_key":"verify_output","parameters":{},"phase":"verify"}],"risk_level":"low"}`}
	planner := NewMCTSPlanner(MCTSConfig{
		LLMClient:       mock,
		MaxIterations:   3,
		BranchingFactor: 2,
		TimeLimit:       5 * time.Second,
	})
	plan, err := planner.Search(context.Background(), LLMPlannerInput{
		WorkspaceID: "ws-1", Intent: "read emails",
	})
	if err != nil {
		t.Fatal(err)
	}
	if plan == nil || len(plan.Steps) == 0 {
		t.Fatal("expected non-empty plan")
	}
}

func TestMCTSPlanner_Expand_VariesTemperature(t *testing.T) {
	mock := &mockMCTSLLM{response: `{"steps":[{"tool_key":"web_search","parameters":{},"phase":"gather"}],"risk_level":"low"}`}
	planner := NewMCTSPlanner(MCTSConfig{
		LLMClient:       mock,
		MaxIterations:   2,
		BranchingFactor: 3,
		TimeLimit:       5 * time.Second,
	})
	_, _ = planner.Search(context.Background(), LLMPlannerInput{
		WorkspaceID: "ws-1", Intent: "search web",
	})
	uniqueTemps := make(map[float64]bool)
	for _, temp := range mock.temps {
		uniqueTemps[temp] = true
	}
	if len(uniqueTemps) < 2 {
		t.Fatalf("expected at least 2 distinct temperatures, got %d from %v", len(uniqueTemps), mock.temps)
	}
}

func TestMCTSPlanner_Search_FallbackOnAllFailures(t *testing.T) {
	mock := &mockMCTSLLM{err: fmt.Errorf("all fail")}
	planner := NewMCTSPlanner(MCTSConfig{
		LLMClient:       mock,
		MaxIterations:   2,
		BranchingFactor: 2,
		TimeLimit:       2 * time.Second,
	})
	_, err := planner.Search(context.Background(), LLMPlannerInput{
		WorkspaceID: "ws-1", Intent: "test",
	})
	// Should fail with error (not panic or infinite loop)
	if err == nil {
		t.Fatal("expected error when all expansions fail")
	}
}

func TestMCTSPlanner_BestPlan_DepthGuard(t *testing.T) {
	planner := NewMCTSPlanner(MCTSConfig{MaxDepth: 3})
	// Create a chain deeper than maxDepth
	root := &MCTSNode{Plan: &Plan{Steps: []PlanStep{{ToolKey: "root"}}}}
	child := &MCTSNode{Plan: &Plan{Steps: []PlanStep{{ToolKey: "child"}}}, Parent: root, Depth: 1}
	grandchild := &MCTSNode{Plan: &Plan{Steps: []PlanStep{{ToolKey: "gc"}}}, Parent: child, Depth: 2}
	root.Children = []*MCTSNode{child}
	child.Children = []*MCTSNode{grandchild}
	grandchild.Visits = 1
	grandchild.TotalValue = 0.9
	child.Visits = 1
	child.TotalValue = 0.8

	plan := planner.bestPlan(root, 0)
	if plan == nil {
		t.Fatal("expected non-nil plan")
	}
}

func TestBackprop_UpdatesAllAncestors(t *testing.T) {
	root := &MCTSNode{}
	child := &MCTSNode{Parent: root}
	grandchild := &MCTSNode{Parent: child}

	planner := NewMCTSPlanner(MCTSConfig{})
	planner.backprop(grandchild, 0.9)

	if root.Visits != 1 || root.TotalValue != 0.9 {
		t.Fatalf("root: visits=%d value=%v", root.Visits, root.TotalValue)
	}
	if child.Visits != 1 || child.TotalValue != 0.9 {
		t.Fatalf("child: visits=%d value=%v", child.Visits, child.TotalValue)
	}
	if grandchild.Visits != 1 || grandchild.TotalValue != 0.9 {
		t.Fatalf("grandchild: visits=%d value=%v", grandchild.Visits, grandchild.TotalValue)
	}
}
