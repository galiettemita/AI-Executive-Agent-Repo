package llm

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestAnthropicClientStream_AssemblesDeltaChunks(t *testing.T) {
	t.Parallel()
	sseData := strings.Join([]string{
		`data: {"type":"content_block_delta","delta":{"type":"text_delta","text":"Hello"}}`,
		`data: {"type":"content_block_delta","delta":{"type":"text_delta","text":" world"}}`,
		`data: {"type":"message_delta","delta":{"stop_reason":"end_turn"},"usage":{"input_tokens":10,"output_tokens":5}}`,
		"",
	}, "\n")

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		fmt.Fprint(w, sseData)
	}))
	defer ts.Close()

	client, _ := NewAnthropicClient(AnthropicConfig{
		APIKey: "test-key", BaseURL: ts.URL, Timeout: 5 * time.Second,
	})
	out := make(chan StreamChunk, 16)
	go client.Stream(context.Background(), GenerateRequest{
		Model:     ModelAnthropicSonnet,
		MaxTokens: 512,
		Messages:  []ChatMsg{{Role: "user", Content: "hi"}},
	}, out)

	var assembled string
	var sawDone bool
	for chunk := range out {
		if chunk.Error != nil {
			t.Fatalf("unexpected error: %v", chunk.Error)
		}
		assembled += chunk.Delta
		if chunk.Done {
			sawDone = true
			if chunk.Usage == nil {
				t.Error("expected Usage on Done chunk")
			}
		}
	}
	if assembled != "Hello world" {
		t.Errorf("assembled text: got %q, want %q", assembled, "Hello world")
	}
	if !sawDone {
		t.Error("expected Done=true chunk")
	}
}

func TestAnthropicClientStream_HTTPError_ReturnsErrorChunk(t *testing.T) {
	t.Parallel()
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		fmt.Fprint(w, `{"error":{"type":"server_error","message":"internal"}}`)
	}))
	defer ts.Close()

	client, _ := NewAnthropicClient(AnthropicConfig{
		APIKey: "test-key", BaseURL: ts.URL, Timeout: 5 * time.Second,
	})

	out := make(chan StreamChunk, 4)
	go client.Stream(context.Background(), GenerateRequest{
		Model:     ModelAnthropicSonnet,
		MaxTokens: 512,
		Messages:  []ChatMsg{{Role: "user", Content: "hi"}},
	}, out)

	var gotError bool
	for chunk := range out {
		if chunk.Error != nil {
			gotError = true
			if !strings.Contains(chunk.Error.Error(), "status 500") {
				t.Errorf("expected status 500 error, got: %v", chunk.Error)
			}
		}
	}
	if !gotError {
		t.Error("expected error chunk for HTTP 500")
	}
}

func TestFailoverClientStream_FallsBackOnPrimaryError(t *testing.T) {
	t.Parallel()
	primary := &mockStreamingClient{chunks: []StreamChunk{{Error: fmt.Errorf("primary down")}}}
	fallback := &mockStreamingClient{chunks: []StreamChunk{
		{Delta: "fallback response"},
		{Done: true, FinishReason: "end_turn"},
	}}
	fc := &FailoverClient{
		Primary: primary, Fallback: fallback,
		PrimaryID: "primary", FallbackID: "fallback",
	}

	out := make(chan StreamChunk, 8)
	go fc.Stream(context.Background(), GenerateRequest{}, out)

	var text string
	for chunk := range out {
		if chunk.Error != nil {
			t.Fatalf("unexpected error: %v", chunk.Error)
		}
		text += chunk.Delta
	}
	if text != "fallback response" {
		t.Errorf("expected fallback text, got %q", text)
	}
}

func TestRateLimitedClientStream_PassesThrough(t *testing.T) {
	t.Parallel()
	inner := &mockStreamingClient{chunks: []StreamChunk{
		{Delta: "hello"},
		{Done: true, FinishReason: "end_turn"},
	}}
	rl := &RateLimitedClient{inner: inner}

	out := make(chan StreamChunk, 8)
	go rl.Stream(context.Background(), GenerateRequest{}, out)

	var text string
	for chunk := range out {
		if chunk.Error != nil {
			t.Fatalf("unexpected error: %v", chunk.Error)
		}
		text += chunk.Delta
	}
	if text != "hello" {
		t.Errorf("expected passthrough text, got %q", text)
	}
}

// mockStreamingClient implements Client for streaming tests.
type mockStreamingClient struct{ chunks []StreamChunk }

func (m *mockStreamingClient) Generate(_ context.Context, _ GenerateRequest) (*GenerateResponse, *Usage, error) {
	return &GenerateResponse{Content: "mock"}, &Usage{}, nil
}

func (m *mockStreamingClient) Stream(_ context.Context, _ GenerateRequest, out chan<- StreamChunk) {
	defer close(out)
	for _, c := range m.chunks {
		out <- c
	}
}
