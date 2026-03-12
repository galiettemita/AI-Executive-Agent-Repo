package llm

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

const (
	anthropicDefaultBaseURL = "https://api.anthropic.com"
	anthropicMessagesPath   = "/v1/messages"
	anthropicAPIVersion     = "2023-06-01"
	anthropicMaxRetries     = 2
)

// AnthropicClient implements Client for the Anthropic Messages API.
type AnthropicClient struct {
	apiKey     string
	baseURL    string
	httpClient *http.Client
}

// AnthropicConfig holds configuration for the Anthropic client.
type AnthropicConfig struct {
	APIKey  string
	BaseURL string
	Timeout time.Duration
}

// NewAnthropicClient creates a new Anthropic Messages API client.
func NewAnthropicClient(cfg AnthropicConfig) (*AnthropicClient, error) {
	if cfg.APIKey == "" {
		return nil, fmt.Errorf("anthropic: API key is required")
	}
	baseURL := cfg.BaseURL
	if baseURL == "" {
		baseURL = anthropicDefaultBaseURL
	}
	baseURL = strings.TrimRight(baseURL, "/")
	timeout := cfg.Timeout
	if timeout <= 0 {
		timeout = 60 * time.Second
	}
	return &AnthropicClient{
		apiKey:  cfg.APIKey,
		baseURL: baseURL,
		httpClient: &http.Client{
			Timeout: timeout,
		},
	}, nil
}

type anthropicRequest struct {
	Model       string              `json:"model"`
	MaxTokens   int                 `json:"max_tokens"`
	Messages    []anthropicMessage  `json:"messages"`
	System      string              `json:"system,omitempty"`
	Temperature *float64            `json:"temperature,omitempty"`
	TopP        *float64            `json:"top_p,omitempty"`
}

type anthropicMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type anthropicResponse struct {
	ID           string                 `json:"id"`
	Type         string                 `json:"type"`
	Role         string                 `json:"role"`
	Content      []anthropicContentBlock `json:"content"`
	Model        string                 `json:"model"`
	StopReason   string                 `json:"stop_reason"`
	Usage        anthropicUsage         `json:"usage"`
}

type anthropicContentBlock struct {
	Type string `json:"type"`
	Text string `json:"text"`
}

type anthropicUsage struct {
	InputTokens  int `json:"input_tokens"`
	OutputTokens int `json:"output_tokens"`
}

type anthropicErrorResponse struct {
	Error struct {
		Type    string `json:"type"`
		Message string `json:"message"`
	} `json:"error"`
}

// Generate calls the Anthropic Messages API.
func (c *AnthropicClient) Generate(ctx context.Context, req GenerateRequest) (*GenerateResponse, *Usage, error) {
	// Separate system message from conversation messages.
	var systemPrompt string
	messages := make([]anthropicMessage, 0, len(req.Messages))
	for _, msg := range req.Messages {
		if msg.Role == "system" {
			systemPrompt = msg.Content
			continue
		}
		messages = append(messages, anthropicMessage{
			Role:    msg.Role,
			Content: msg.Content,
		})
	}

	// If structured JSON output is requested, append schema instruction to system prompt.
	if req.JSONSchema != nil {
		schemaBytes, err := json.Marshal(req.JSONSchema)
		if err != nil {
			return nil, nil, fmt.Errorf("anthropic: marshal json_schema: %w", err)
		}
		schemaInstruction := fmt.Sprintf(
			"\n\nYou MUST respond with valid JSON that conforms to this schema:\n%s\n\nRespond with ONLY the JSON object, no other text.",
			string(schemaBytes),
		)
		systemPrompt += schemaInstruction
	}

	maxTokens := req.MaxTokens
	if maxTokens <= 0 {
		maxTokens = 1024
	}

	apiReq := anthropicRequest{
		Model:     req.Model,
		MaxTokens: maxTokens,
		Messages:  messages,
		System:    systemPrompt,
	}
	if req.Temperature > 0 {
		t := req.Temperature
		apiReq.Temperature = &t
	}
	if req.TopP > 0 && req.TopP < 1.0 {
		p := req.TopP
		apiReq.TopP = &p
	}

	body, err := json.Marshal(apiReq)
	if err != nil {
		return nil, nil, fmt.Errorf("anthropic: marshal request: %w", err)
	}

	var lastErr error
	for attempt := 0; attempt <= anthropicMaxRetries; attempt++ {
		if attempt > 0 {
			select {
			case <-ctx.Done():
				return nil, nil, fmt.Errorf("anthropic: %w", ctx.Err())
			case <-time.After(time.Duration(attempt) * time.Second):
			}
		}

		resp, usage, err := c.doRequest(ctx, body)
		if err == nil {
			return resp, usage, nil
		}
		lastErr = err
		if !isRetryable(err) {
			return nil, nil, err
		}
	}
	return nil, nil, fmt.Errorf("anthropic: exhausted retries: %w", lastErr)
}

func (c *AnthropicClient) doRequest(ctx context.Context, body []byte) (*GenerateResponse, *Usage, error) {
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+anthropicMessagesPath, bytes.NewReader(body))
	if err != nil {
		return nil, nil, fmt.Errorf("anthropic: create request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("x-api-key", c.apiKey)
	httpReq.Header.Set("anthropic-version", anthropicAPIVersion)
	if reqID, ok := ctx.Value(requestIDKey{}).(string); ok && reqID != "" {
		httpReq.Header.Set("X-Request-ID", reqID)
	}

	httpResp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return nil, nil, fmt.Errorf("anthropic: http error: %w", err)
	}
	defer httpResp.Body.Close()

	respBody, err := io.ReadAll(io.LimitReader(httpResp.Body, 1<<20))
	if err != nil {
		return nil, nil, fmt.Errorf("anthropic: read response: %w", err)
	}

	if httpResp.StatusCode != http.StatusOK {
		var errResp anthropicErrorResponse
		_ = json.Unmarshal(respBody, &errResp)
		return nil, nil, fmt.Errorf("anthropic: status %d: %s: %s",
			httpResp.StatusCode, errResp.Error.Type, errResp.Error.Message)
	}

	var apiResp anthropicResponse
	if err := json.Unmarshal(respBody, &apiResp); err != nil {
		return nil, nil, fmt.Errorf("anthropic: unmarshal response: %w", err)
	}

	var contentBuilder strings.Builder
	for _, block := range apiResp.Content {
		if block.Type == "text" {
			contentBuilder.WriteString(block.Text)
		}
	}

	return &GenerateResponse{
			Content:      contentBuilder.String(),
			Model:        apiResp.Model,
			ProviderID:   "anthropic",
			FinishReason: apiResp.StopReason,
		}, &Usage{
			InputTokens:  apiResp.Usage.InputTokens,
			OutputTokens: apiResp.Usage.OutputTokens,
		}, nil
}
