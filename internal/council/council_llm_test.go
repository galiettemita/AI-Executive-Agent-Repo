package council

import (
	"context"
	"fmt"
	"testing"

	"github.com/brevio/brevio/internal/llm"
)

type mockCouncilLLM struct {
	response string
	err      error
}

func (m *mockCouncilLLM) Generate(_ context.Context, _ llm.GenerateRequest) (*llm.GenerateResponse, *llm.Usage, error) {
	if m.err != nil {
		return nil, nil, m.err
	}
	return &llm.GenerateResponse{Content: m.response}, &llm.Usage{}, nil
}

func (m *mockCouncilLLM) Stream(_ context.Context, _ llm.GenerateRequest, out chan<- llm.StreamChunk) {
	defer close(out)
}

func TestDeliberateAndVote_AllAgentsApprove(t *testing.T) {
	mock := &mockCouncilLLM{response: `{"vote":"approve","confidence":0.9,"justification":"Looks safe"}`}
	svc := NewCouncilServiceWithLLM(mock)
	for _, ag := range DefaultCouncilAgents() {
		svc.RegisterAgent(ag)
	}
	council, err := svc.ConveneCouncil("ws-1", "Send email to client about project update", CouncilPolicy{
		MinAgents: 2, MaxAgents: 4, VotingStrategy: "majority",
		RequiredCapabilities: []string{}, ConveneThreshold: 0.0,
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := svc.DeliberateAndVote(context.Background(), council.ID, "Send email with project update"); err != nil {
		t.Fatal(err)
	}
	decision, err := svc.ResolveCouncil(council.ID)
	if err != nil {
		t.Fatal(err)
	}
	if decision.Decision != "approved" {
		t.Fatalf("expected approved, got %s", decision.Decision)
	}
}

func TestDeliberateAndVote_NilClient_AbstainsAll(t *testing.T) {
	svc := NewCouncilService() // nil LLM client
	for _, ag := range DefaultCouncilAgents() {
		svc.RegisterAgent(ag)
	}
	council, _ := svc.ConveneCouncil("ws-1", "Test topic for abstain voting strategy", CouncilPolicy{
		MinAgents: 2, MaxAgents: 4, VotingStrategy: "majority",
		ConveneThreshold: 0.0,
	})
	err := svc.DeliberateAndVote(context.Background(), council.ID, "test action")
	if err != nil {
		t.Fatal(err)
	}
	// All should be abstain
	svc.mu.RLock()
	c := svc.councils[council.ID]
	for _, v := range c.Votes {
		if v.Vote != "abstain" {
			t.Fatalf("expected abstain for nil client, got %s", v.Vote)
		}
	}
	svc.mu.RUnlock()
}

func TestDeliberateAndVote_LLMError_AbstainsAgent(t *testing.T) {
	mock := &mockCouncilLLM{err: fmt.Errorf("LLM down")}
	svc := NewCouncilServiceWithLLM(mock)
	svc.RegisterAgent(CouncilAgent{ID: "test-agent", Name: "Test", Capabilities: []string{}})
	council, _ := svc.ConveneCouncil("ws-1", "Test topic for LLM error handling test", CouncilPolicy{
		MinAgents: 1, MaxAgents: 1, VotingStrategy: "majority",
		ConveneThreshold: 0.0,
	})
	err := svc.DeliberateAndVote(context.Background(), council.ID, "test")
	if err != nil {
		t.Fatal(err)
	}
	svc.mu.RLock()
	c := svc.councils[council.ID]
	if len(c.Votes) == 0 {
		t.Fatal("expected at least one vote")
	}
	if c.Votes[0].Vote != "abstain" {
		t.Fatalf("expected abstain on LLM error, got %s", c.Votes[0].Vote)
	}
	svc.mu.RUnlock()
}

func TestDefaultCouncilAgents_Count(t *testing.T) {
	agents := DefaultCouncilAgents()
	if len(agents) != 4 {
		t.Fatalf("expected 4 agents, got %d", len(agents))
	}
	ids := make(map[string]bool)
	for _, a := range agents {
		if ids[a.ID] {
			t.Fatalf("duplicate ID: %s", a.ID)
		}
		ids[a.ID] = true
		if len(a.Capabilities) == 0 {
			t.Fatalf("agent %s has empty capabilities", a.ID)
		}
	}
}
