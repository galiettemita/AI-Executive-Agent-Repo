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
	Generate(ctx context.Context, req GenerateRequest) (*GenerateResponse, *Usage, error)
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
}

// ChatMsg is a single message in the conversation.
type ChatMsg struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

// GenerateResponse carries the inference result.
type GenerateResponse struct {
	Content    string `json:"content"`
	Model      string `json:"model"`
	ProviderID string `json:"provider_id"`
	FinishReason string `json:"finish_reason"`
}

// Usage carries token consumption metrics for cost tracking.
type Usage struct {
	InputTokens  int `json:"input_tokens"`
	OutputTokens int `json:"output_tokens"`
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

// isRetryable returns true for errors that warrant a failover attempt.
func isRetryable(err error) bool {
	if err == nil {
		return false
	}
	msg := err.Error()
	return strings.Contains(msg, "status 429") ||
		strings.Contains(msg, "status 5") ||
		strings.Contains(msg, "timeout") ||
		strings.Contains(msg, "context deadline exceeded")
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
