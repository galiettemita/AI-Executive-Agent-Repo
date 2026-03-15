package brain

import (
	"context"
	"testing"

	"github.com/brevio/brevio/internal/llm"
)

type mockStepBackLLM struct {
	responseContent string
	err             error
	called          bool
}

func (m *mockStepBackLLM) Generate(_ context.Context, _ llm.GenerateRequest) (*llm.GenerateResponse, *llm.Usage, error) {
	m.called = true
	if m.err != nil {
		return nil, nil, m.err
	}
	return &llm.GenerateResponse{Content: m.responseContent}, &llm.Usage{}, nil
}

func (m *mockStepBackLLM) Stream(_ context.Context, _ llm.GenerateRequest, out chan<- llm.StreamChunk) {
	defer close(out)
}

func TestStepBackService_SkipsWhenConfidenceHigh(t *testing.T) {
	mock := &mockStepBackLLM{responseContent: `{"abstract_goal":"test","confidence":0.9}`}
	svc := NewStepBackService(mock)
	result, err := svc.Infer(context.Background(), StepBackRequest{
		WorkspaceID:      "ws-1",
		RawMessage:       "schedule a meeting",
		IntentConfidence: 0.85, // above threshold 0.75
	})
	if err != nil {
		t.Fatal(err)
	}
	if !result.Skipped {
		t.Fatal("expected skipped when confidence >= threshold")
	}
	if mock.called {
		t.Fatal("LLM should NOT be called when confidence is high")
	}
}

func TestStepBackService_RunsWhenLow(t *testing.T) {
	mock := &mockStepBackLLM{responseContent: `{"abstract_goal":"user wants to organize their schedule","confidence":0.88}`}
	svc := NewStepBackService(mock)
	result, err := svc.Infer(context.Background(), StepBackRequest{
		WorkspaceID:      "ws-1",
		RawMessage:       "can you maybe help me with that thing about the calendar",
		IntentConfidence: 0.60, // below threshold
	})
	if err != nil {
		t.Fatal(err)
	}
	if result.Skipped {
		t.Fatal("expected NOT skipped when confidence is low")
	}
	if result.AbstractGoal != "user wants to organize their schedule" {
		t.Fatalf("unexpected goal: %q", result.AbstractGoal)
	}
}

func TestStepBackService_GracefulOnBadJSON(t *testing.T) {
	mock := &mockStepBackLLM{responseContent: "this is not json at all"}
	svc := NewStepBackService(mock)
	result, err := svc.Infer(context.Background(), StepBackRequest{
		WorkspaceID:      "ws-1",
		RawMessage:       "something ambiguous",
		IntentConfidence: 0.50,
	})
	if err != nil {
		t.Fatal(err)
	}
	if !result.Skipped {
		t.Fatal("expected skipped on bad JSON")
	}
}

func TestStepBackService_GracefulOnNilClient(t *testing.T) {
	svc := NewStepBackService(nil)
	result, err := svc.Infer(context.Background(), StepBackRequest{
		WorkspaceID:      "ws-1",
		RawMessage:       "test",
		IntentConfidence: 0.50,
	})
	if err != nil {
		t.Fatal(err)
	}
	if !result.Skipped {
		t.Fatal("expected skipped with nil client")
	}
}

func TestStepBackService_GracefulOnEmptyMessage(t *testing.T) {
	svc := NewStepBackService(&mockStepBackLLM{})
	result, err := svc.Infer(context.Background(), StepBackRequest{
		WorkspaceID:      "ws-1",
		RawMessage:       "",
		IntentConfidence: 0.50,
	})
	if err != nil {
		t.Fatal(err)
	}
	if !result.Skipped {
		t.Fatal("expected skipped on empty message")
	}
}
