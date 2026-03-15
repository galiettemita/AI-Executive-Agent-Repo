package brain

import (
	"context"
	"fmt"
	"testing"

	"github.com/brevio/brevio/internal/llm"
)

type mockPRMLLM struct {
	response string
	err      error
	called   bool
}

func (m *mockPRMLLM) Generate(_ context.Context, _ llm.GenerateRequest) (*llm.GenerateResponse, *llm.Usage, error) {
	m.called = true
	if m.err != nil {
		return nil, nil, m.err
	}
	return &llm.GenerateResponse{Content: m.response}, &llm.Usage{}, nil
}

func (m *mockPRMLLM) Stream(_ context.Context, _ llm.GenerateRequest, out chan<- llm.StreamChunk) {
	defer close(out)
}

func TestPRM_HighScore_Continues(t *testing.T) {
	mock := &mockPRMLLM{response: `{"score":4,"is_plausible":true,"reasoning":"good step"}`}
	prm := NewProcessRewardModel(PRMConfig{LLMClient: mock, MinStepScore: 2.0, Enabled: true})
	reward, shouldContinue, err := prm.ScoreStep(context.Background(), "test intent",
		[]PlanStep{{ToolKey: "email_read"}},
		[]StepResult{{Success: true, ToolKey: "email_read"}},
		StepResult{StepIndex: 0, ToolKey: "email_read", Success: true},
	)
	if err != nil {
		t.Fatal(err)
	}
	if !shouldContinue {
		t.Fatal("expected shouldContinue=true for high score")
	}
	if reward.Score != 4 {
		t.Fatalf("expected score 4, got %v", reward.Score)
	}
}

func TestPRM_LowScore_Terminates(t *testing.T) {
	mock := &mockPRMLLM{response: `{"score":1,"is_plausible":false,"reasoning":"wrong approach"}`}
	prm := NewProcessRewardModel(PRMConfig{LLMClient: mock, MinStepScore: 2.0, Enabled: true})
	_, shouldContinue, _ := prm.ScoreStep(context.Background(), "test",
		[]PlanStep{{ToolKey: "web_search"}},
		[]StepResult{{Success: true, ToolKey: "web_search"}},
		StepResult{StepIndex: 0, ToolKey: "web_search", Success: true},
	)
	if shouldContinue {
		t.Fatal("expected shouldContinue=false for low score")
	}
}

func TestPRM_Disabled_AlwaysContinues(t *testing.T) {
	mock := &mockPRMLLM{}
	prm := NewProcessRewardModel(PRMConfig{LLMClient: mock, Enabled: false})
	_, shouldContinue, _ := prm.ScoreStep(context.Background(), "test",
		nil, nil, StepResult{},
	)
	if !shouldContinue {
		t.Fatal("expected shouldContinue=true when disabled")
	}
	if mock.called {
		t.Fatal("LLM should NOT be called when PRM disabled")
	}
}

func TestPRM_LLMFailure_FailOpen(t *testing.T) {
	mock := &mockPRMLLM{err: fmt.Errorf("LLM down")}
	prm := NewProcessRewardModel(PRMConfig{LLMClient: mock, MinStepScore: 2.0, Enabled: true})
	_, shouldContinue, _ := prm.ScoreStep(context.Background(), "test",
		nil, nil, StepResult{ToolKey: "test"},
	)
	if !shouldContinue {
		t.Fatal("expected fail-open: shouldContinue=true on LLM failure")
	}
}

func TestPRM_NilClient_FailOpen(t *testing.T) {
	prm := NewProcessRewardModel(PRMConfig{LLMClient: nil, Enabled: true})
	_, shouldContinue, _ := prm.ScoreStep(context.Background(), "test",
		nil, nil, StepResult{},
	)
	if !shouldContinue {
		t.Fatal("expected fail-open with nil client")
	}
}
