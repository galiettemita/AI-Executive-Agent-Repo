package brain

import (
	"context"
	"fmt"
	"testing"

	"github.com/brevio/brevio/internal/llm"
)

type mockPlannerLLM struct {
	response string
	err      error
	temps    []float64 // records temperatures from each call
}

func (m *mockPlannerLLM) Generate(_ context.Context, req llm.GenerateRequest) (*llm.GenerateResponse, *llm.Usage, error) {
	m.temps = append(m.temps, req.Temperature)
	if m.err != nil {
		return nil, nil, m.err
	}
	return &llm.GenerateResponse{Content: m.response}, &llm.Usage{}, nil
}

func (m *mockPlannerLLM) Stream(_ context.Context, _ llm.GenerateRequest, out chan<- llm.StreamChunk) {
	defer close(out)
}

func TestCallLLMPlanner_ParsesValidPlan(t *testing.T) {
	mock := &mockPlannerLLM{response: `{"steps":[{"tool_key":"email_read","parameters":{"workspace_id":"ws1","query":"test"},"phase":"gather"},{"tool_key":"verify_output","parameters":{"workspace_id":"ws1"},"phase":"verify","depends_on":[0]}],"risk_level":"low","reasoning":"read then verify"}`}
	plan, _, err := callLLMPlanner(context.Background(), mock, LLMPlannerInput{
		WorkspaceID: "ws1", Intent: "read emails",
		Tools: DefaultToolRegistry(),
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(plan.Steps) != 2 {
		t.Fatalf("expected 2 steps, got %d", len(plan.Steps))
	}
}

func TestCallLLMPlanner_EmptySteps(t *testing.T) {
	mock := &mockPlannerLLM{response: `{"steps":[],"risk_level":"low"}`}
	_, _, err := callLLMPlanner(context.Background(), mock, LLMPlannerInput{
		WorkspaceID: "ws1", Intent: "test",
	})
	if err == nil {
		t.Fatal("expected error for empty plan")
	}
}

func TestCallLLMPlanner_NilClient(t *testing.T) {
	_, _, err := callLLMPlanner(context.Background(), nil, LLMPlannerInput{})
	if err == nil {
		t.Fatal("expected error for nil client")
	}
}

func TestCallLLMPlanner_InvalidJSON(t *testing.T) {
	mock := &mockPlannerLLM{response: "not json at all"}
	_, _, err := callLLMPlanner(context.Background(), mock, LLMPlannerInput{
		WorkspaceID: "ws1", Intent: "test",
	})
	if err == nil {
		t.Fatal("expected parse error")
	}
}

func TestPlannerStep_HeuristicFallback(t *testing.T) {
	loop := NewReasoningLoop(ReasoningLoopConfig{
		QualityTarget: 0.8,
		MaxIterations: 3,
		// LLMClient nil → heuristic fallback
	})
	rc := &ReasoningContext{
		WorkspaceID: "ws1",
		Intent:      "send email to Alice",
	}
	plan, err := loop.PlannerStep(context.Background(), rc)
	if err != nil {
		t.Fatal(err)
	}
	if plan == nil || len(plan.Steps) == 0 {
		t.Fatal("expected non-nil plan from heuristic fallback")
	}
}

func TestPlanSignature_Deterministic(t *testing.T) {
	plan := &Plan{Steps: []PlanStep{
		{ToolKey: "email_read", Phase: "gather"},
		{ToolKey: "verify_output", Phase: "verify"},
	}}
	sig1 := PlanSignature(plan)
	sig2 := PlanSignature(plan)
	if sig1 != sig2 {
		t.Fatal("signatures should be identical for same plan")
	}
	if sig1 != "gather:email_read|verify:verify_output" {
		t.Fatalf("unexpected signature: %q", sig1)
	}
}

func TestSelfConsistencyPlanner_MajorityWins(t *testing.T) {
	// Return plan A twice, plan B once → A wins
	planA := `{"steps":[{"tool_key":"email_read","parameters":{},"phase":"gather"}],"risk_level":"low"}`
	planB := `{"steps":[{"tool_key":"web_search","parameters":{},"phase":"gather"}],"risk_level":"low"}`
	multiMock := &multiResponseLLM{responses: []string{planA, planB, planA}}
	sc := NewSelfConsistencyPlanner(multiMock, 3)
	plan, err := sc.SelectPlan(context.Background(), LLMPlannerInput{
		WorkspaceID: "ws1", Intent: "test",
	})
	if err != nil {
		t.Fatal(err)
	}
	if plan.Steps[0].ToolKey != "email_read" {
		t.Fatalf("expected majority plan (email_read), got %s", plan.Steps[0].ToolKey)
	}
}

func TestSelfConsistencyPlanner_VariesTemperature(t *testing.T) {
	mock := &mockPlannerLLM{response: `{"steps":[{"tool_key":"email_read","parameters":{},"phase":"gather"}],"risk_level":"low"}`}
	sc := NewSelfConsistencyPlanner(mock, 3)
	_, _ = sc.SelectPlan(context.Background(), LLMPlannerInput{
		WorkspaceID: "ws1", Intent: "test",
	})

	uniqueTemps := make(map[float64]bool)
	for _, t := range mock.temps {
		uniqueTemps[t] = true
	}
	if len(uniqueTemps) < 2 {
		t.Fatalf("expected at least 2 unique temperatures, got %d from %v", len(uniqueTemps), mock.temps)
	}
}

func TestSelfConsistencyPlanner_AllFailed(t *testing.T) {
	mock := &mockPlannerLLM{err: fmt.Errorf("LLM down")}
	sc := NewSelfConsistencyPlanner(mock, 3)
	_, err := sc.SelectPlan(context.Background(), LLMPlannerInput{
		WorkspaceID: "ws1", Intent: "test",
	})
	if err == nil {
		t.Fatal("expected error when all samples fail")
	}
}

func TestDefaultToolRegistry_Returns8Tools(t *testing.T) {
	tools := DefaultToolRegistry()
	if len(tools) != 8 {
		t.Fatalf("expected 8 tools, got %d", len(tools))
	}
}

// multiResponseLLM returns different responses for each call.
type multiResponseLLM struct {
	responses []string
	callIdx   int
}

func (m *multiResponseLLM) Generate(_ context.Context, _ llm.GenerateRequest) (*llm.GenerateResponse, *llm.Usage, error) {
	idx := m.callIdx
	m.callIdx++
	if idx < len(m.responses) {
		return &llm.GenerateResponse{Content: m.responses[idx]}, &llm.Usage{}, nil
	}
	return &llm.GenerateResponse{Content: m.responses[0]}, &llm.Usage{}, nil
}

func (m *multiResponseLLM) Stream(_ context.Context, _ llm.GenerateRequest, out chan<- llm.StreamChunk) {
	defer close(out)
}
