package brain

import (
	"context"
	"fmt"
	"testing"

	"github.com/brevio/brevio/internal/llm"
)

type mockJudgeLLM struct {
	response string
	err      error
}

func (m *mockJudgeLLM) Generate(_ context.Context, _ llm.GenerateRequest) (*llm.GenerateResponse, *llm.Usage, error) {
	if m.err != nil {
		return nil, nil, m.err
	}
	return &llm.GenerateResponse{Content: m.response}, &llm.Usage{}, nil
}

func (m *mockJudgeLLM) Stream(_ context.Context, _ llm.GenerateRequest, out chan<- llm.StreamChunk) {
	defer close(out)
}

func TestSemanticCriticService_Passing(t *testing.T) {
	mock := &mockJudgeLLM{response: `{"intent_satisfied":true,"quality_score":0.9,"completeness":0.95,"accuracy":0.92,"should_retry":false,"issues":[]}`}
	svc := NewSemanticCriticService(mock, 0.75)
	score, err := svc.Evaluate(context.Background(), SemanticCriticRequest{
		OriginalIntent: "send email", Steps: []PlanStep{{ToolKey: "email_send"}},
		Results: []StepResult{{Success: true, ToolKey: "email_send"}},
	})
	if err != nil {
		t.Fatal(err)
	}
	if !score.Passed {
		t.Fatal("expected passed")
	}
	if score.QualityScore < 0.85 {
		t.Fatalf("expected high quality, got %v", score.QualityScore)
	}
}

func TestSemanticCriticService_Failing(t *testing.T) {
	mock := &mockJudgeLLM{response: `{"intent_satisfied":false,"quality_score":0.4,"completeness":0.3,"accuracy":0.5,"should_retry":true,"retry_guidance":"try different tool","issues":["wrong tool used"]}`}
	svc := NewSemanticCriticService(mock, 0.75)
	score, err := svc.Evaluate(context.Background(), SemanticCriticRequest{
		OriginalIntent: "send email", Steps: []PlanStep{{ToolKey: "web_search"}},
		Results: []StepResult{{Success: true, ToolKey: "web_search"}},
	})
	if err != nil {
		t.Fatal(err)
	}
	if score.Passed {
		t.Fatal("expected not passed")
	}
	if !score.ShouldRetry {
		t.Fatal("expected should retry")
	}
}

func TestSemanticCriticService_FallbackOnLLMError(t *testing.T) {
	mock := &mockJudgeLLM{err: fmt.Errorf("LLM down")}
	svc := NewSemanticCriticService(mock, 0.75)
	score, err := svc.Evaluate(context.Background(), SemanticCriticRequest{
		OriginalIntent: "test", Steps: []PlanStep{{ToolKey: "email_read"}},
		Results: []StepResult{{Success: true, ToolKey: "email_read"}},
	})
	if err != nil {
		t.Fatal(err)
	}
	if score == nil {
		t.Fatal("expected non-nil score from heuristic fallback")
	}
}

func TestSemanticCriticService_NilClient(t *testing.T) {
	svc := NewSemanticCriticService(nil, 0.75)
	score, err := svc.Evaluate(context.Background(), SemanticCriticRequest{
		OriginalIntent: "test", Steps: []PlanStep{{ToolKey: "email_read"}},
		Results: []StepResult{{Success: true, ToolKey: "email_read"}},
	})
	if err != nil {
		t.Fatal(err)
	}
	if score == nil {
		t.Fatal("expected heuristic fallback result")
	}
}

func TestBuildJudgePrompt_ContainsIntent(t *testing.T) {
	svc := NewSemanticCriticService(nil, 0.75)
	prompt := svc.buildPrompt(SemanticCriticRequest{
		OriginalIntent: "schedule meeting with Alice",
		Steps:          []PlanStep{{ToolKey: "calendar_write", Phase: "act"}},
		Results:        []StepResult{{Success: true, ToolKey: "calendar_write"}},
	})
	if !contains(prompt, "schedule meeting with Alice") {
		t.Fatalf("prompt missing intent: %s", prompt)
	}
}

func contains(s, sub string) bool {
	return len(s) >= len(sub) && (s == sub || len(s) > 0 && containsSub(s, sub))
}

func containsSub(s, sub string) bool {
	for i := 0; i <= len(s)-len(sub); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}
