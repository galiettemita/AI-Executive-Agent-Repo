package llm

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestBuildAnthropicMessages_PlainHistory(t *testing.T) {
	msgs := []ChatMsg{
		{Role: "user", Content: "hello"},
		{Role: "assistant", Content: "hi"},
	}
	out := buildAnthropicMessages(msgs, nil, nil)
	if len(out) != 2 {
		t.Fatalf("expected 2, got %d", len(out))
	}
	if out[0].Role != "user" || out[1].Role != "assistant" {
		t.Fatal("roles not preserved")
	}
}

func TestBuildAnthropicMessages_WithPriorToolCalls(t *testing.T) {
	msgs := []ChatMsg{{Role: "user", Content: "do something"}}
	prior := []AssistantToolUse{{ID: "X", Name: "foo", Input: map[string]any{"k": "v"}}}
	out := buildAnthropicMessages(msgs, prior, nil)
	if len(out) != 2 {
		t.Fatalf("expected 2, got %d", len(out))
	}
	if out[1].Role != "assistant" {
		t.Fatalf("expected assistant, got %s", out[1].Role)
	}
	blocks, ok := out[1].Content.([]anthropicContentBlock)
	if !ok {
		t.Fatal("assistant content should be []anthropicContentBlock")
	}
	found := false
	for _, b := range blocks {
		if b.Type == "tool_use" && b.ID == "X" && b.Name == "foo" {
			found = true
		}
	}
	if !found {
		t.Fatal("tool_use block not found")
	}
}

func TestBuildAnthropicMessages_WithToolResults(t *testing.T) {
	msgs := []ChatMsg{{Role: "user", Content: "do something"}}
	prior := []AssistantToolUse{{ID: "X", Name: "foo", Input: map[string]any{"k": "v"}}}
	results := []ToolResult{{ToolCallID: "X", Content: "result"}}
	out := buildAnthropicMessages(msgs, prior, results)
	if len(out) != 3 {
		t.Fatalf("expected 3, got %d", len(out))
	}
	if out[2].Role != "user" {
		t.Fatalf("expected user for tool_result, got %s", out[2].Role)
	}
	blocks, ok := out[2].Content.([]anthropicContentBlock)
	if !ok {
		t.Fatal("tool_result content should be []anthropicContentBlock")
	}
	if blocks[0].Type != "tool_result" {
		t.Fatalf("expected tool_result, got %s", blocks[0].Type)
	}
}

func TestBuildAnthropicMessages_ToolResultIDMatchesPriorCall(t *testing.T) {
	prior := []AssistantToolUse{{ID: "abc", Name: "tool1", Input: map[string]any{}}}
	results := []ToolResult{{ToolCallID: "abc", Content: "done"}}
	out := buildAnthropicMessages(nil, prior, results)
	// Last message (user with tool_result) should have ID matching "abc"
	userBlocks := out[len(out)-1].Content.([]anthropicContentBlock)
	if userBlocks[0].ID != "abc" {
		t.Fatalf("expected ID 'abc', got %q", userBlocks[0].ID)
	}
}

func TestGenerateRequest_ThinkingSerialized(t *testing.T) {
	req := GenerateRequest{
		Thinking: &ThinkingConfig{Enabled: true, BudgetTokens: 4096},
	}
	data, err := json.Marshal(req)
	if err != nil {
		t.Fatal(err)
	}
	s := string(data)
	if !strings.Contains(s, `"thinking"`) {
		t.Fatal("missing thinking key")
	}
	if !strings.Contains(s, `"enabled":true`) {
		t.Fatal("missing enabled:true")
	}
	if !strings.Contains(s, `"budget_tokens":4096`) {
		t.Fatal("missing budget_tokens")
	}
}

func TestAnthropicClient_Generate_ToolCallParsed(t *testing.T) {
	t.Parallel()
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		resp := map[string]any{
			"id": "msg_1", "type": "message", "role": "assistant",
			"content": []map[string]any{
				{"type": "tool_use", "id": "call_1", "name": "calendar_read",
					"input": map[string]any{"workspace_id": "ws1"}},
			},
			"model":       ModelAnthropicSonnet,
			"stop_reason": "tool_use",
			"usage":       map[string]int{"input_tokens": 100, "output_tokens": 50},
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}))
	defer ts.Close()

	client, _ := NewAnthropicClient(AnthropicConfig{
		APIKey: "test-key", BaseURL: ts.URL, Timeout: 5 * time.Second,
	})
	resp, _, err := client.Generate(context.Background(), GenerateRequest{
		Model:     ModelAnthropicSonnet,
		MaxTokens: 512,
		Messages:  []ChatMsg{{Role: "user", Content: "read my calendar"}},
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(resp.ToolCalls) != 1 {
		t.Fatalf("expected 1 tool call, got %d", len(resp.ToolCalls))
	}
	if resp.ToolCalls[0].ID != "call_1" {
		t.Fatalf("ID: got %q", resp.ToolCalls[0].ID)
	}
	if resp.ToolCalls[0].Name != "calendar_read" {
		t.Fatalf("Name: got %q", resp.ToolCalls[0].Name)
	}
}

func TestAnthropicClient_Generate_ThinkingParsed(t *testing.T) {
	t.Parallel()
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		resp := map[string]any{
			"id": "msg_1", "type": "message", "role": "assistant",
			"content": []map[string]any{
				{"type": "thinking", "thinking": "I should check..."},
				{"type": "text", "text": "Here is the result"},
			},
			"model":       ModelAnthropicSonnet,
			"stop_reason": "end_turn",
			"usage":       map[string]int{"input_tokens": 100, "output_tokens": 200},
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}))
	defer ts.Close()

	client, _ := NewAnthropicClient(AnthropicConfig{
		APIKey: "test-key", BaseURL: ts.URL, Timeout: 5 * time.Second,
	})
	resp, _, err := client.Generate(context.Background(), GenerateRequest{
		Model:     ModelAnthropicSonnet,
		MaxTokens: 1024,
		Thinking:  &ThinkingConfig{Enabled: true, BudgetTokens: 4096},
		Messages:  []ChatMsg{{Role: "user", Content: "plan this"}},
	})
	if err != nil {
		t.Fatal(err)
	}
	if resp.ThinkingContent != "I should check..." {
		t.Fatalf("ThinkingContent: got %q", resp.ThinkingContent)
	}
	if !strings.Contains(resp.Content, "Here is the result") {
		t.Fatalf("Content: got %q", resp.Content)
	}
}

func TestAnthropicClient_Generate_ThinkingSetsTemperature1(t *testing.T) {
	t.Parallel()
	var capturedBody []byte
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedBody, _ = io.ReadAll(r.Body)
		resp := map[string]any{
			"id": "msg_1", "type": "message", "role": "assistant",
			"content":     []map[string]any{{"type": "text", "text": "ok"}},
			"model":       ModelAnthropicSonnet,
			"stop_reason": "end_turn",
			"usage":       map[string]int{"input_tokens": 10, "output_tokens": 5},
		}
		json.NewEncoder(w).Encode(resp)
	}))
	defer ts.Close()

	client, _ := NewAnthropicClient(AnthropicConfig{
		APIKey: "test-key", BaseURL: ts.URL, Timeout: 5 * time.Second,
	})
	_, _, err := client.Generate(context.Background(), GenerateRequest{
		Model:     ModelAnthropicSonnet,
		MaxTokens: 1024,
		Thinking:  &ThinkingConfig{Enabled: true, BudgetTokens: 4096},
		Messages:  []ChatMsg{{Role: "user", Content: "test"}},
	})
	if err != nil {
		t.Fatal(err)
	}

	var body map[string]any
	json.Unmarshal(capturedBody, &body)
	temp, _ := body["temperature"].(float64)
	if temp != 1.0 {
		t.Fatalf("expected temperature=1.0, got %v", temp)
	}
	thinking, _ := body["thinking"].(map[string]any)
	if thinking["type"] != "enabled" {
		t.Fatalf("expected thinking.type=enabled, got %v", thinking["type"])
	}
}

func TestAnthropicClient_Generate_ThinkingSendsBetaHeader(t *testing.T) {
	t.Parallel()
	var capturedBeta string
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedBeta = r.Header.Get("anthropic-beta")
		resp := map[string]any{
			"id": "msg_1", "type": "message", "role": "assistant",
			"content":     []map[string]any{{"type": "text", "text": "ok"}},
			"model":       ModelAnthropicSonnet,
			"stop_reason": "end_turn",
			"usage":       map[string]int{"input_tokens": 10, "output_tokens": 5},
		}
		json.NewEncoder(w).Encode(resp)
	}))
	defer ts.Close()

	client, _ := NewAnthropicClient(AnthropicConfig{
		APIKey: "test-key", BaseURL: ts.URL, Timeout: 5 * time.Second,
	})
	_, _, _ = client.Generate(context.Background(), GenerateRequest{
		Model:     ModelAnthropicSonnet,
		MaxTokens: 1024,
		Thinking:  &ThinkingConfig{Enabled: true, BudgetTokens: 4096},
		Messages:  []ChatMsg{{Role: "user", Content: "test"}},
	})

	if capturedBeta != anthropicBetaThinking {
		t.Fatalf("expected beta header %q, got %q", anthropicBetaThinking, capturedBeta)
	}
}
