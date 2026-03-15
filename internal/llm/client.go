package llm

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"strings"
	"time"
)

// requestIDKey is the context key for propagating request IDs to provider calls.
type requestIDKey struct{}

// ContextWithRequestID returns a context carrying a request ID for provider traceability.
func ContextWithRequestID(ctx context.Context, id string) context.Context {
	return context.WithValue(ctx, requestIDKey{}, id)
}

// Client is the provider-agnostic interface for LLM inference.
// All implementations must be safe for concurrent use.
type Client interface {
	// Generate performs a complete (non-streaming) inference call.
	Generate(ctx context.Context, req GenerateRequest) (*GenerateResponse, *Usage, error)

	// Stream sends incremental StreamChunks to out as they arrive from the provider.
	// The implementor MUST close out when done (on Done=true chunk or error chunk).
	// Callers MUST drain out even after context cancellation to avoid goroutine leaks.
	Stream(ctx context.Context, req GenerateRequest, out chan<- StreamChunk)
}

// ToolDefinition describes a callable tool the LLM may invoke.
type ToolDefinition struct {
	Name        string         `json:"name"`
	Description string         `json:"description"`
	InputSchema map[string]any `json:"input_schema"`
}

// ToolCall is a single tool invocation the model requests.
type ToolCall struct {
	ID    string         `json:"id"`
	Name  string         `json:"name"`
	Input map[string]any `json:"input"`
}

// ToolResult carries the result of an executed tool back to the model.
type ToolResult struct {
	ToolCallID string `json:"tool_call_id"`
	Content    string `json:"content"`
	IsError    bool   `json:"is_error,omitempty"`
}

// ToolChoice controls model tool selection.
type ToolChoice string

const (
	ToolChoiceAuto ToolChoice = "auto"
	ToolChoiceAny  ToolChoice = "any"
	ToolChoiceNone ToolChoice = "none"
)

// ThinkingConfig enables Anthropic extended thinking.
type ThinkingConfig struct {
	Enabled      bool `json:"enabled"`
	BudgetTokens int  `json:"budget_tokens"` // 1024–32768
}

// AssistantToolUse reconstructs a prior assistant turn that made tool calls.
type AssistantToolUse struct {
	ID    string         `json:"id"`
	Name  string         `json:"name"`
	Input map[string]any `json:"input"`
	Text  string         `json:"text,omitempty"`
}

// GenerateRequest carries all parameters for an LLM inference call.
type GenerateRequest struct {
	Model       string    `json:"model"`
	Messages    []ChatMsg `json:"messages"`
	MaxTokens   int       `json:"max_tokens"`
	Temperature float64   `json:"temperature"`
	TopP        float64   `json:"top_p"`
	// JSONSchema, when non-nil, instructs the provider to enforce structured output.
	JSONSchema map[string]any `json:"json_schema,omitempty"`
	// IdempotencyKey is used for request deduplication at the provider level.
	IdempotencyKey string `json:"idempotency_key,omitempty"`
	// Thinking enables extended chain-of-thought.
	Thinking *ThinkingConfig `json:"thinking,omitempty"`
	// System prompt sent to the model.
	System string `json:"system,omitempty"`
	// Tools available to the model.
	Tools []ToolDefinition `json:"tools,omitempty"`
	// ToolChoice controls tool selection.
	ToolChoice ToolChoice `json:"tool_choice,omitempty"`
	// ToolResults from a PREVIOUS round of tool calls.
	ToolResults []ToolResult `json:"tool_results,omitempty"`
	// PriorAssistantToolCalls from the PREVIOUS assistant turn.
	PriorAssistantToolCalls []AssistantToolUse `json:"prior_assistant_tool_calls,omitempty"`
}

// ChatMsg is a single message in the conversation.
type ChatMsg struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

// GenerateResponse carries the inference result.
type GenerateResponse struct {
	Content      string `json:"content"`
	Model        string `json:"model"`
	ProviderID   string `json:"provider_id"`
	FinishReason string `json:"finish_reason"`
	// ThinkingContent is the extended reasoning trace.
	ThinkingContent string `json:"thinking_content,omitempty"`
	// ToolCalls are tool invocations the model wants to execute.
	ToolCalls []ToolCall `json:"tool_calls,omitempty"`
	// InputTokens mirrors Usage.InputTokens.
	InputTokens int `json:"input_tokens,omitempty"`
	// OutputTokens mirrors Usage.OutputTokens.
	OutputTokens int `json:"output_tokens,omitempty"`
}

// Usage carries token consumption metrics for cost tracking.
type Usage struct {
	InputTokens         int `json:"input_tokens"`
	OutputTokens        int `json:"output_tokens"`
	CacheCreationTokens int `json:"cache_creation_input_tokens"` // tokens written to cache
	CacheReadTokens     int `json:"cache_read_input_tokens"`     // tokens read from cache
}

// StreamChunk is a single incremental event from a streaming LLM response.
type StreamChunk struct {
	Delta         string // incremental text token(s); empty for metadata-only events
	ThinkingDelta string // extended thinking token(s) — Anthropic only
	Done          bool   // true on the final event; no further chunks will be sent
	FinishReason  string // populated when Done=true ("end_turn", "max_tokens", etc.)
	Usage         *Usage // populated when Done=true
	Error         error  // non-nil if streaming failed; channel is closed immediately after
}

// FailoverClient wraps a primary and fallback Client. If the primary call
// fails with a retryable error, the fallback is tried exactly once.
type FailoverClient struct {
	Primary    Client
	Fallback   Client
	PrimaryID  string
	FallbackID string
}

// Generate tries the primary provider, falling back on retryable failures.
func (fc *FailoverClient) Generate(ctx context.Context, req GenerateRequest) (*GenerateResponse, *Usage, error) {
	resp, usage, err := fc.Primary.Generate(ctx, req)
	if err == nil {
		return resp, usage, nil
	}
	if !isRetryable(err) {
		return nil, nil, fmt.Errorf("primary provider %s: %w", fc.PrimaryID, err)
	}
	if fc.Fallback == nil {
		return nil, nil, fmt.Errorf("primary provider %s failed and no fallback configured: %w", fc.PrimaryID, err)
	}
	resp, usage, fallbackErr := fc.Fallback.Generate(ctx, req)
	if fallbackErr != nil {
		return nil, nil, fmt.Errorf("fallback provider %s also failed: %w (primary: %v)", fc.FallbackID, fallbackErr, err)
	}
	return resp, usage, nil
}

// Stream implements Client. Tries the primary provider first.
// Falls back to the secondary if the primary errors on its first chunk.
func (fc *FailoverClient) Stream(ctx context.Context, req GenerateRequest, out chan<- StreamChunk) {
	primaryOut := make(chan StreamChunk, 1)
	go func() { fc.Primary.Stream(ctx, req, primaryOut) }()

	first, ok := <-primaryOut
	if !ok {
		if fc.Fallback != nil {
			fc.Fallback.Stream(ctx, req, out)
		} else {
			close(out)
		}
		return
	}
	if first.Error != nil {
		if fc.Fallback != nil {
			fc.Fallback.Stream(ctx, req, out)
		} else {
			out <- first
			close(out)
		}
		return
	}

	out <- first
	for chunk := range primaryOut {
		out <- chunk
	}
	close(out)
}

// ComputeIdempotencyKey generates a deterministic key from the request
// parameters for request-level deduplication.
func ComputeIdempotencyKey(workspaceID, promptKey, input string, timestamp time.Time) string {
	h := sha256.Sum256([]byte(strings.Join([]string{
		workspaceID,
		promptKey,
		input,
		fmt.Sprintf("%d", timestamp.UnixMilli()),
	}, "::")))
	return "idem_" + hex.EncodeToString(h[:16])
}
