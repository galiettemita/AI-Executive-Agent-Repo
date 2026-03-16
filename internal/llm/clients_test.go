package llm

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

// ---------------------------------------------------------------------------
// Anthropic httptest tests
// ---------------------------------------------------------------------------

func newAnthropicTestServer(handler http.HandlerFunc) (*httptest.Server, *AnthropicClient) {
	ts := httptest.NewServer(handler)
	client, _ := NewAnthropicClient(AnthropicConfig{
		APIKey:  "test-key",
		BaseURL: ts.URL,
		Timeout: 5 * time.Second,
	})
	return ts, client
}

func TestAnthropicClient_Success(t *testing.T) {
	t.Parallel()

	ts, client := newAnthropicTestServer(func(w http.ResponseWriter, r *http.Request) {
		// Verify headers.
		if r.Header.Get("x-api-key") != "test-key" {
			t.Errorf("expected x-api-key header, got %q", r.Header.Get("x-api-key"))
		}
		if r.Header.Get("anthropic-version") != "2023-06-01" {
			t.Errorf("expected anthropic-version header")
		}
		if r.Header.Get("X-Request-ID") != "req-123" {
			t.Errorf("expected X-Request-ID header, got %q", r.Header.Get("X-Request-ID"))
		}
		if r.URL.Path != "/v1/messages" {
			t.Errorf("expected path /v1/messages, got %s", r.URL.Path)
		}

		resp := anthropicResponse{
			ID:    "msg_test",
			Type:  "message",
			Role:  "assistant",
			Model: ModelAnthropicHaiku,
			Content: []anthropicContentBlock{
				{Type: "text", Text: `{"intent":"email_query","confidence":0.95}`},
			},
			StopReason: "end_turn",
			Usage:      anthropicUsage{InputTokens: 100, OutputTokens: 50},
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	})
	defer ts.Close()

	ctx := ContextWithRequestID(context.Background(), "req-123")
	resp, usage, err := client.Generate(ctx, GenerateRequest{
		Model:     ModelAnthropicHaiku,
		MaxTokens: 256,
		Messages:  []ChatMsg{{Role: "user", Content: "hello"}},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(resp.Content, "email_query") {
		t.Errorf("expected content to contain 'email_query', got %q", resp.Content)
	}
	if resp.ProviderID != "anthropic" {
		t.Errorf("expected providerID 'anthropic', got %q", resp.ProviderID)
	}
	if usage.InputTokens != 100 || usage.OutputTokens != 50 {
		t.Errorf("unexpected usage: %+v", usage)
	}
}

func TestAnthropicClient_429Retryable(t *testing.T) {
	t.Parallel()

	attempts := 0
	ts, client := newAnthropicTestServer(func(w http.ResponseWriter, r *http.Request) {
		attempts++
		w.WriteHeader(http.StatusTooManyRequests)
		json.NewEncoder(w).Encode(map[string]any{
			"error": map[string]string{"type": "rate_limit", "message": "rate limited"},
		})
	})
	defer ts.Close()

	_, _, err := client.Generate(context.Background(), GenerateRequest{
		Model:     ModelAnthropicHaiku,
		MaxTokens: 256,
		Messages:  []ChatMsg{{Role: "user", Content: "hello"}},
	})
	if err == nil {
		t.Fatal("expected error for 429")
	}
	if !strings.Contains(err.Error(), "status 429") {
		t.Errorf("expected 429 in error, got: %v", err)
	}
	// Should have retried (initial + 2 retries = 3 attempts).
	if attempts != 3 {
		t.Errorf("expected 3 attempts, got %d", attempts)
	}
}

func TestAnthropicClient_500Retryable(t *testing.T) {
	t.Parallel()

	attempts := 0
	ts, client := newAnthropicTestServer(func(w http.ResponseWriter, r *http.Request) {
		attempts++
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]any{
			"error": map[string]string{"type": "server_error", "message": "internal"},
		})
	})
	defer ts.Close()

	_, _, err := client.Generate(context.Background(), GenerateRequest{
		Model:     ModelAnthropicHaiku,
		MaxTokens: 256,
		Messages:  []ChatMsg{{Role: "user", Content: "hello"}},
	})
	if err == nil {
		t.Fatal("expected error for 500")
	}
	if !strings.Contains(err.Error(), "status 500") {
		t.Errorf("expected 500 in error, got: %v", err)
	}
	if attempts != 3 {
		t.Errorf("expected 3 attempts, got %d", attempts)
	}
}

func TestAnthropicClient_400NonRetryable(t *testing.T) {
	t.Parallel()

	attempts := 0
	ts, client := newAnthropicTestServer(func(w http.ResponseWriter, r *http.Request) {
		attempts++
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]any{
			"error": map[string]string{"type": "invalid_request", "message": "bad model"},
		})
	})
	defer ts.Close()

	_, _, err := client.Generate(context.Background(), GenerateRequest{
		Model:     "invalid-model",
		MaxTokens: 256,
		Messages:  []ChatMsg{{Role: "user", Content: "hello"}},
	})
	if err == nil {
		t.Fatal("expected error for 400")
	}
	// Should NOT retry — only 1 attempt.
	if attempts != 1 {
		t.Errorf("expected 1 attempt for non-retryable 400, got %d", attempts)
	}
}

func TestAnthropicClient_Timeout(t *testing.T) {
	t.Parallel()

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(3 * time.Second)
	}))
	defer ts.Close()

	client, _ := NewAnthropicClient(AnthropicConfig{
		APIKey:  "test-key",
		BaseURL: ts.URL,
		Timeout: 100 * time.Millisecond,
	})

	ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
	defer cancel()

	_, _, err := client.Generate(ctx, GenerateRequest{
		Model:     ModelAnthropicHaiku,
		MaxTokens: 256,
		Messages:  []ChatMsg{{Role: "user", Content: "hello"}},
	})
	if err == nil {
		t.Fatal("expected timeout error")
	}
}

func TestAnthropicNativeStructuredOutput(t *testing.T) {
	t.Parallel()

	ts, client := newAnthropicTestServer(func(w http.ResponseWriter, r *http.Request) {
		bodyBytes, _ := io.ReadAll(r.Body)
		bodyStr := string(bodyBytes)

		// Assert the request uses native tool_use, not system prompt injection.
		if !strings.Contains(bodyStr, `"tools"`) {
			t.Error("expected request body to contain tools field")
		}
		if !strings.Contains(bodyStr, `"tool_choice"`) {
			t.Error("expected request body to contain tool_choice field")
		}
		if strings.Contains(bodyStr, "You MUST respond with valid JSON") {
			t.Error("system prompt should NOT contain schema instruction when using native tool_use")
		}

		// Return a tool_use content block.
		resp := map[string]any{
			"id":          "msg_test",
			"type":        "message",
			"role":        "assistant",
			"content":     []map[string]any{{"type": "tool_use", "name": "structured_output", "input": map[string]any{"intent": "email_query", "confidence": 0.95}}},
			"model":       ModelAnthropicSonnet,
			"stop_reason": "tool_use",
			"usage":       map[string]any{"input_tokens": 100, "output_tokens": 50},
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	})
	defer ts.Close()

	resp, usage, err := client.Generate(context.Background(), GenerateRequest{
		Model:      ModelAnthropicHaiku,
		MaxTokens:  256,
		Messages:   []ChatMsg{{Role: "user", Content: "test"}},
		JSONSchema: map[string]any{"type": "object", "properties": map[string]any{"intent": map[string]any{"type": "string"}, "confidence": map[string]any{"type": "number"}}},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(resp.Content, "email_query") {
		t.Errorf("expected tool_use output with email_query, got %q", resp.Content)
	}
	if !strings.Contains(resp.Content, "0.95") {
		t.Errorf("expected tool_use output with confidence, got %q", resp.Content)
	}
	if usage.InputTokens != 100 || usage.OutputTokens != 50 {
		t.Errorf("unexpected usage: %+v", usage)
	}
}

// ---------------------------------------------------------------------------
// OpenAI httptest tests
// ---------------------------------------------------------------------------

func newOpenAITestServer(handler http.HandlerFunc) (*httptest.Server, *OpenAIClient) {
	ts := httptest.NewServer(handler)
	client, _ := NewOpenAIClient(OpenAIConfig{
		APIKey:  "test-key",
		BaseURL: ts.URL,
		Timeout: 5 * time.Second,
	})
	return ts, client
}

func TestOpenAIClient_Success(t *testing.T) {
	t.Parallel()

	ts, client := newOpenAITestServer(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer test-key" {
			t.Errorf("expected Bearer auth, got %q", r.Header.Get("Authorization"))
		}
		if r.Header.Get("X-Request-ID") != "req-456" {
			t.Errorf("expected X-Request-ID header")
		}
		if r.URL.Path != "/v1/responses" {
			t.Errorf("expected path /v1/responses, got %s", r.URL.Path)
		}

		resp := openaiResponsesResponse{
			ID:     "resp_test",
			Status: "completed",
			Model:  "gpt-4o-mini",
			Output: []openaiOutputItem{
				{
					Type: "message",
					Content: []openaiContentPart{
						{Type: "output_text", Text: `{"intent":"task_creation"}`},
					},
				},
			},
			Usage: openaiUsage{InputTokens: 80, OutputTokens: 40},
		}
		json.NewEncoder(w).Encode(resp)
	})
	defer ts.Close()

	ctx := ContextWithRequestID(context.Background(), "req-456")
	resp, usage, err := client.Generate(ctx, GenerateRequest{
		Model:     "gpt-4o-mini",
		MaxTokens: 256,
		Messages:  []ChatMsg{{Role: "system", Content: "You are a helper"}, {Role: "user", Content: "create task"}},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(resp.Content, "task_creation") {
		t.Errorf("expected content with 'task_creation', got %q", resp.Content)
	}
	if resp.ProviderID != "openai" {
		t.Errorf("expected providerID 'openai', got %q", resp.ProviderID)
	}
	if usage.InputTokens != 80 || usage.OutputTokens != 40 {
		t.Errorf("unexpected usage: %+v", usage)
	}
}

func TestOpenAIClient_429Retryable(t *testing.T) {
	t.Parallel()

	attempts := 0
	ts, client := newOpenAITestServer(func(w http.ResponseWriter, r *http.Request) {
		attempts++
		w.WriteHeader(http.StatusTooManyRequests)
		json.NewEncoder(w).Encode(map[string]any{
			"error": map[string]string{"code": "rate_limit_exceeded", "message": "rate limited"},
		})
	})
	defer ts.Close()

	_, _, err := client.Generate(context.Background(), GenerateRequest{
		Model:     "gpt-4o-mini",
		MaxTokens: 256,
		Messages:  []ChatMsg{{Role: "user", Content: "hello"}},
	})
	if err == nil {
		t.Fatal("expected error for 429")
	}
	if attempts != 3 {
		t.Errorf("expected 3 attempts, got %d", attempts)
	}
}

func TestOpenAIClient_500Retryable(t *testing.T) {
	t.Parallel()

	attempts := 0
	ts, client := newOpenAITestServer(func(w http.ResponseWriter, r *http.Request) {
		attempts++
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]any{
			"error": map[string]string{"code": "server_error", "message": "internal"},
		})
	})
	defer ts.Close()

	_, _, err := client.Generate(context.Background(), GenerateRequest{
		Model:     "gpt-4o-mini",
		MaxTokens: 256,
		Messages:  []ChatMsg{{Role: "user", Content: "hello"}},
	})
	if err == nil {
		t.Fatal("expected error for 500")
	}
	if attempts != 3 {
		t.Errorf("expected 3 attempts, got %d", attempts)
	}
}

func TestOpenAIClient_StructuredOutputFormat(t *testing.T) {
	t.Parallel()

	ts, client := newOpenAITestServer(func(w http.ResponseWriter, r *http.Request) {
		var req openaiResponsesRequest
		json.NewDecoder(r.Body).Decode(&req)
		if req.Text == nil || req.Text.Format == nil {
			t.Fatal("expected structured text format")
		}
		if req.Text.Format.Type != "json_schema" {
			t.Errorf("expected json_schema format type, got %q", req.Text.Format.Type)
		}
		if !req.Text.Format.Strict {
			t.Error("expected strict=true for structured output")
		}

		resp := openaiResponsesResponse{
			ID:     "resp_test",
			Status: "completed",
			Output: []openaiOutputItem{{Type: "message", Content: []openaiContentPart{{Type: "output_text", Text: `{"ok":true}`}}}},
			Usage:  openaiUsage{InputTokens: 10, OutputTokens: 5},
		}
		json.NewEncoder(w).Encode(resp)
	})
	defer ts.Close()

	_, _, err := client.Generate(context.Background(), GenerateRequest{
		Model:      "gpt-4o",
		MaxTokens:  256,
		Messages:   []ChatMsg{{Role: "user", Content: "test"}},
		JSONSchema: map[string]any{"type": "object", "properties": map[string]any{"ok": map[string]any{"type": "boolean"}}},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestOpenAIClient_SystemRoleRewrite(t *testing.T) {
	t.Parallel()

	ts, client := newOpenAITestServer(func(w http.ResponseWriter, r *http.Request) {
		var req openaiResponsesRequest
		json.NewDecoder(r.Body).Decode(&req)
		for _, item := range req.Input {
			if item.Role == "system" {
				t.Error("system role should be rewritten to developer")
			}
		}
		hasDeveloper := false
		for _, item := range req.Input {
			if item.Role == "developer" {
				hasDeveloper = true
			}
		}
		if !hasDeveloper {
			t.Error("expected developer role in input")
		}

		resp := openaiResponsesResponse{
			ID:     "resp_test",
			Status: "completed",
			Output: []openaiOutputItem{{Type: "message", Content: []openaiContentPart{{Type: "output_text", Text: "ok"}}}},
			Usage:  openaiUsage{InputTokens: 10, OutputTokens: 5},
		}
		json.NewEncoder(w).Encode(resp)
	})
	defer ts.Close()

	_, _, err := client.Generate(context.Background(), GenerateRequest{
		Model:    "gpt-4o-mini",
		Messages: []ChatMsg{{Role: "system", Content: "sys"}, {Role: "user", Content: "hi"}},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

// ---------------------------------------------------------------------------
// FailoverClient tests
// ---------------------------------------------------------------------------

type stubClient struct {
	resp *GenerateResponse
	err  error
}

func (s *stubClient) Generate(_ context.Context, _ GenerateRequest) (*GenerateResponse, *Usage, error) {
	return s.resp, &Usage{InputTokens: 10, OutputTokens: 5}, s.err
}

func (s *stubClient) Stream(_ context.Context, _ GenerateRequest, out chan<- StreamChunk) {
	defer close(out)
	if s.err != nil {
		out <- StreamChunk{Error: s.err}
		return
	}
	if s.resp != nil {
		out <- StreamChunk{Delta: s.resp.Content, Done: true, FinishReason: "end_turn"}
	}
}

func TestFailoverClient_PrimarySuccess(t *testing.T) {
	t.Parallel()

	fc := &FailoverClient{
		Primary:   &stubClient{resp: &GenerateResponse{Content: "primary", ProviderID: "anthropic"}},
		Fallback:  &stubClient{resp: &GenerateResponse{Content: "fallback", ProviderID: "openai"}},
		PrimaryID: "anthropic",
	}
	resp, _, err := fc.Generate(context.Background(), GenerateRequest{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.Content != "primary" {
		t.Errorf("expected primary response, got %q", resp.Content)
	}
}

func TestFailoverClient_FallbackOn429(t *testing.T) {
	t.Parallel()

	fc := &FailoverClient{
		Primary:    &stubClient{err: fmt.Errorf("status 429: rate limited")},
		Fallback:   &stubClient{resp: &GenerateResponse{Content: "fallback", ProviderID: "openai"}},
		PrimaryID:  "anthropic",
		FallbackID: "openai",
	}
	resp, _, err := fc.Generate(context.Background(), GenerateRequest{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.Content != "fallback" {
		t.Errorf("expected fallback response, got %q", resp.Content)
	}
}

func TestFailoverClient_FallbackOn500(t *testing.T) {
	t.Parallel()

	fc := &FailoverClient{
		Primary:    &stubClient{err: fmt.Errorf("status 500: internal server error")},
		Fallback:   &stubClient{resp: &GenerateResponse{Content: "fallback"}},
		PrimaryID:  "primary",
		FallbackID: "fallback",
	}
	resp, _, err := fc.Generate(context.Background(), GenerateRequest{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.Content != "fallback" {
		t.Errorf("expected fallback, got %q", resp.Content)
	}
}

func TestFailoverClient_NoFallbackOnNonRetryable(t *testing.T) {
	t.Parallel()

	fc := &FailoverClient{
		Primary:    &stubClient{err: fmt.Errorf("status 400: bad request")},
		Fallback:   &stubClient{resp: &GenerateResponse{Content: "fallback"}},
		PrimaryID:  "primary",
		FallbackID: "fallback",
	}
	_, _, err := fc.Generate(context.Background(), GenerateRequest{})
	if err == nil {
		t.Fatal("expected error for non-retryable failure")
	}
	if !strings.Contains(err.Error(), "primary provider") {
		t.Errorf("expected primary provider in error, got: %v", err)
	}
}

func TestFailoverClient_NoFallbackConfigured(t *testing.T) {
	t.Parallel()

	fc := &FailoverClient{
		Primary:   &stubClient{err: fmt.Errorf("status 500: server error")},
		Fallback:  nil,
		PrimaryID: "primary",
	}
	_, _, err := fc.Generate(context.Background(), GenerateRequest{})
	if err == nil {
		t.Fatal("expected error when no fallback configured")
	}
	if !strings.Contains(err.Error(), "no fallback configured") {
		t.Errorf("expected 'no fallback configured' in error, got: %v", err)
	}
}

func TestFailoverClient_BothFail(t *testing.T) {
	t.Parallel()

	fc := &FailoverClient{
		Primary:    &stubClient{err: fmt.Errorf("status 500: primary down")},
		Fallback:   &stubClient{err: fmt.Errorf("status 500: fallback down")},
		PrimaryID:  "primary",
		FallbackID: "fallback",
	}
	_, _, err := fc.Generate(context.Background(), GenerateRequest{})
	if err == nil {
		t.Fatal("expected error when both fail")
	}
	if !strings.Contains(err.Error(), "fallback provider") {
		t.Errorf("expected 'fallback provider' in error, got: %v", err)
	}
}

// ---------------------------------------------------------------------------
// isRetryable tests
// ---------------------------------------------------------------------------

func TestIsRetryable(t *testing.T) {
	t.Parallel()

	cases := []struct {
		err      string
		expected bool
	}{
		{"status 429: rate limited", true},
		{"status 500: server error", true},
		{"status 502: bad gateway", true},
		{"status 503: unavailable", true},
		{"timeout waiting for response", true},
		{"context deadline exceeded", true},
		{"status 400: bad request", false},
		{"status 401: unauthorized", false},
		{"status 403: forbidden", false},
		{"", false},
	}

	for _, tc := range cases {
		var err error
		if tc.err != "" {
			err = fmt.Errorf("%s", tc.err)
		}
		if got := isRetryable(err); got != tc.expected {
			t.Errorf("isRetryable(%q) = %v, want %v", tc.err, got, tc.expected)
		}
	}
}

// ---------------------------------------------------------------------------
// IdempotencyKey tests
// ---------------------------------------------------------------------------

func TestComputeIdempotencyKey(t *testing.T) {
	t.Parallel()

	ts := time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)
	key1 := ComputeIdempotencyKey("ws-1", "classify", "hello world", ts)
	key2 := ComputeIdempotencyKey("ws-1", "classify", "hello world", ts)
	key3 := ComputeIdempotencyKey("ws-2", "classify", "hello world", ts)

	if !strings.HasPrefix(key1, "idem_") {
		t.Errorf("expected idem_ prefix, got %q", key1)
	}
	if key1 != key2 {
		t.Error("same inputs should produce same key")
	}
	if key1 == key3 {
		t.Error("different workspace should produce different key")
	}
}

func TestAnthropicThinkingRequest(t *testing.T) {
	t.Parallel()
	var capturedBody []byte
	ts, client := newAnthropicTestServer(func(w http.ResponseWriter, r *http.Request) {
		capturedBody, _ = io.ReadAll(r.Body)
		resp := map[string]any{
			"id": "msg_t", "type": "message", "role": "assistant",
			"content": []map[string]any{
				{"type": "thinking", "thinking": "Let me reason step by step..."},
				{"type": "text", "text": `{"intent":"test","confidence":0.9}`},
			},
			"model":       ModelAnthropicSonnet,
			"stop_reason": "end_turn",
			"usage":       map[string]int{"input_tokens": 100, "output_tokens": 200},
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	})
	defer ts.Close()

	resp, _, err := client.Generate(context.Background(), GenerateRequest{
		Model:     ModelAnthropicSonnet,
		MaxTokens: 1024,
		Thinking:  &ThinkingConfig{Enabled: true, BudgetTokens: 1000},
		Messages:  []ChatMsg{{Role: "user", Content: "test"}},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Verify temperature was forced to 1.0
	var reqBody map[string]any
	if err := json.Unmarshal(capturedBody, &reqBody); err != nil {
		t.Fatalf("unmarshal request body: %v", err)
	}
	temp, _ := reqBody["temperature"].(float64)
	if temp != 1.0 {
		t.Errorf("expected temperature=1.0, got %v", reqBody["temperature"])
	}
	if _, ok := reqBody["thinking"]; !ok {
		t.Error("expected 'thinking' field in request body")
	}

	// ThinkingContent must be populated
	if resp.ThinkingContent != "Let me reason step by step..." {
		t.Errorf("ThinkingContent: got %q", resp.ThinkingContent)
	}
	// Content must be the text block (not the thinking block)
	if !strings.Contains(resp.Content, "intent") {
		t.Errorf("unexpected Content: %q", resp.Content)
	}
}

func TestAnthropicThinkingIncompatibleWithJSONSchema(t *testing.T) {
	t.Parallel()
	_, client := newAnthropicTestServer(func(w http.ResponseWriter, r *http.Request) {
		t.Error("server should not be called")
	})
	_, _, err := client.Generate(context.Background(), GenerateRequest{
		Model:      ModelAnthropicSonnet,
		MaxTokens:  512,
		Thinking:   &ThinkingConfig{Enabled: true, BudgetTokens: 1000},
		JSONSchema: map[string]any{"type": "object"},
		Messages:   []ChatMsg{{Role: "user", Content: "test"}},
	})
	if err == nil {
		t.Error("expected error when both Thinking and JSONSchema are set")
	}
}
