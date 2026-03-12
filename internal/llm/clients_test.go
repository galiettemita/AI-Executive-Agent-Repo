package llm

import (
	"context"
	"encoding/json"
	"fmt"
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
			Model: "claude-haiku-4-5-20250929",
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
		Model:     "claude-haiku-4-5-20250929",
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
		Model:     "claude-haiku-4-5-20250929",
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
		Model:     "claude-haiku-4-5-20250929",
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
		Model:     "claude-haiku-4-5-20250929",
		MaxTokens: 256,
		Messages:  []ChatMsg{{Role: "user", Content: "hello"}},
	})
	if err == nil {
		t.Fatal("expected timeout error")
	}
}

func TestAnthropicClient_JSONSchemaInSystemPrompt(t *testing.T) {
	t.Parallel()

	ts, client := newAnthropicTestServer(func(w http.ResponseWriter, r *http.Request) {
		var req anthropicRequest
		json.NewDecoder(r.Body).Decode(&req)
		if !strings.Contains(req.System, "json_schema") && !strings.Contains(req.System, "JSON") {
			t.Error("expected system prompt to contain schema instruction")
		}
		resp := anthropicResponse{
			ID:      "msg_test",
			Content: []anthropicContentBlock{{Type: "text", Text: `{"result":"ok"}`}},
			Usage:   anthropicUsage{InputTokens: 10, OutputTokens: 5},
		}
		json.NewEncoder(w).Encode(resp)
	})
	defer ts.Close()

	_, _, err := client.Generate(context.Background(), GenerateRequest{
		Model:      "claude-haiku-4-5-20250929",
		MaxTokens:  256,
		Messages:   []ChatMsg{{Role: "user", Content: "test"}},
		JSONSchema: map[string]any{"type": "object", "properties": map[string]any{"result": map[string]any{"type": "string"}}},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
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
			err = fmt.Errorf(tc.err)
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
