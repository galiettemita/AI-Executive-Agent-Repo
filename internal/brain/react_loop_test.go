package brain

import (
	"context"
	"fmt"
	"testing"

	"github.com/brevio/brevio/internal/llm"
)

type mockReActLLM struct {
	responses []*llm.GenerateResponse
	callIdx   int
	err       error
	captured  []llm.GenerateRequest
}

func (m *mockReActLLM) Generate(_ context.Context, req llm.GenerateRequest) (*llm.GenerateResponse, *llm.Usage, error) {
	m.captured = append(m.captured, req)
	if m.err != nil {
		return nil, nil, m.err
	}
	idx := m.callIdx
	m.callIdx++
	if idx < len(m.responses) {
		return m.responses[idx], &llm.Usage{InputTokens: 10, OutputTokens: 5}, nil
	}
	return &llm.GenerateResponse{Content: "fallback"}, &llm.Usage{}, nil
}

func (m *mockReActLLM) Stream(_ context.Context, _ llm.GenerateRequest, out chan<- llm.StreamChunk) {
	defer close(out)
}

func TestReactLoop_FinalAnswerOnFirstTurn(t *testing.T) {
	mock := &mockReActLLM{responses: []*llm.GenerateResponse{
		{ToolCalls: []llm.ToolCall{{Name: "final_answer", ID: "X", Input: map[string]any{"answer": "Done."}}}},
	}}
	loop := NewReActLoop(ReactLoopConfig{LLMClient: mock, MaxTurns: 5})
	result, err := loop.Run(context.Background(), &ReasoningContext{Intent: "test", WorkspaceID: "ws-1"})
	if err != nil {
		t.Fatal(err)
	}
	if result.FinalAnswer != "Done." {
		t.Fatalf("expected 'Done.', got %q", result.FinalAnswer)
	}
	if len(result.Turns) != 1 {
		t.Fatalf("expected 1 turn, got %d", len(result.Turns))
	}
}

func TestReactLoop_ToolCallThenFinal(t *testing.T) {
	mock := &mockReActLLM{responses: []*llm.GenerateResponse{
		{ToolCalls: []llm.ToolCall{{Name: "calendar_read", ID: "c1", Input: map[string]any{"workspace_id": "ws1"}}}},
		{ToolCalls: []llm.ToolCall{{Name: "final_answer", ID: "f1", Input: map[string]any{"answer": "3 meetings"}}}},
	}}
	loop := NewReActLoop(ReactLoopConfig{LLMClient: mock, MaxTurns: 5})
	result, err := loop.Run(context.Background(), &ReasoningContext{Intent: "check calendar", WorkspaceID: "ws-1"})
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Turns) != 2 {
		t.Fatalf("expected 2 turns, got %d", len(result.Turns))
	}
	if result.FinalAnswer != "3 meetings" {
		t.Fatalf("expected '3 meetings', got %q", result.FinalAnswer)
	}
	if result.Turns[0].ToolCallID != "c1" {
		t.Fatalf("expected tool call ID c1, got %q", result.Turns[0].ToolCallID)
	}
	if result.Turns[0].Observation == "" {
		t.Fatal("expected non-empty observation")
	}
}

func TestReactLoop_PriorToolCallPassedToNextRequest(t *testing.T) {
	mock := &mockReActLLM{responses: []*llm.GenerateResponse{
		{ToolCalls: []llm.ToolCall{{Name: "email_read", ID: "e1", Input: map[string]any{"q": "test"}}}},
		{Content: "Done reading"},
	}}
	loop := NewReActLoop(ReactLoopConfig{LLMClient: mock, MaxTurns: 5})
	_, _ = loop.Run(context.Background(), &ReasoningContext{Intent: "read email", WorkspaceID: "ws-1"})

	if len(mock.captured) < 2 {
		t.Fatalf("expected at least 2 LLM calls, got %d", len(mock.captured))
	}
	req2 := mock.captured[1]
	if len(req2.PriorAssistantToolCalls) == 0 {
		t.Fatal("expected PriorAssistantToolCalls on second request")
	}
	if req2.PriorAssistantToolCalls[0].ID != "e1" {
		t.Fatalf("expected prior tool call ID e1, got %q", req2.PriorAssistantToolCalls[0].ID)
	}
	if len(req2.ToolResults) == 0 {
		t.Fatal("expected ToolResults on second request")
	}
	if req2.ToolResults[0].ToolCallID != "e1" {
		t.Fatalf("expected tool result ID e1, got %q", req2.ToolResults[0].ToolCallID)
	}
}

func TestReactLoop_TextResponseNoTool_IsFinal(t *testing.T) {
	mock := &mockReActLLM{responses: []*llm.GenerateResponse{
		{Content: "Hello there"},
	}}
	loop := NewReActLoop(ReactLoopConfig{LLMClient: mock, MaxTurns: 5})
	result, err := loop.Run(context.Background(), &ReasoningContext{Intent: "greet", WorkspaceID: "ws-1"})
	if err != nil {
		t.Fatal(err)
	}
	if result.FinalAnswer != "Hello there" {
		t.Fatalf("expected 'Hello there', got %q", result.FinalAnswer)
	}
	if !result.Turns[0].IsFinal {
		t.Fatal("expected IsFinal=true")
	}
}

func TestReactLoop_ExceedsMaxTurns(t *testing.T) {
	// Always returns a non-final tool call
	mock := &mockReActLLM{responses: []*llm.GenerateResponse{
		{ToolCalls: []llm.ToolCall{{Name: "web_search", ID: "w1", Input: map[string]any{"q": "x"}}}},
		{ToolCalls: []llm.ToolCall{{Name: "web_search", ID: "w2", Input: map[string]any{"q": "y"}}}},
		{ToolCalls: []llm.ToolCall{{Name: "web_search", ID: "w3", Input: map[string]any{"q": "z"}}}},
	}}
	loop := NewReActLoop(ReactLoopConfig{LLMClient: mock, MaxTurns: 3})
	result, err := loop.Run(context.Background(), &ReasoningContext{Intent: "search", WorkspaceID: "ws-1"})
	if err == nil {
		t.Fatal("expected error for exceeding max turns")
	}
	if len(result.Turns) != 3 {
		t.Fatalf("expected 3 turns, got %d", len(result.Turns))
	}
}

func TestReactLoop_NilLLMClient(t *testing.T) {
	loop := NewReActLoop(ReactLoopConfig{MaxTurns: 3})
	_, err := loop.Run(context.Background(), &ReasoningContext{Intent: "test"})
	if err == nil {
		t.Fatal("expected error for nil client")
	}
}

func TestRunReActLoop_NilClient(t *testing.T) {
	rl := NewReasoningLoop(ReasoningLoopConfig{QualityTarget: 0.8})
	_, err := rl.RunReActLoop(context.Background(), &ReasoningContext{Intent: "test"})
	if err == nil {
		t.Fatal("expected error")
	}
}

// Unused import guard
var _ = fmt.Sprintf
