package llm

import (
	"bufio"
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

// anthropicSystemBlock is one element of a multi-block system prompt.
// Setting CacheControl instructs the Anthropic server to cache the KV state of
// this block's prefix across requests, saving cost and reducing TTFT.
type anthropicSystemBlock struct {
	Type         string                 `json:"type"` // always "text"
	Text         string                 `json:"text"`
	CacheControl *anthropicCacheControl `json:"cache_control,omitempty"`
}

type anthropicCacheControl struct {
	Type string `json:"type"` // "ephemeral"
}

type anthropicRequest struct {
	Model       string                 `json:"model"`
	MaxTokens   int                    `json:"max_tokens"`
	Messages    []anthropicMessage     `json:"messages"`
	System      []anthropicSystemBlock `json:"system,omitempty"`
	Temperature *float64               `json:"temperature,omitempty"`
	TopP        *float64               `json:"top_p,omitempty"`
	// Forward-compat placeholders — fully wired in later prompts:
	Thinking   *anthropicThinking   `json:"thinking,omitempty"`    // wired in Prompt 3
	Stream     bool                 `json:"stream,omitempty"`      // wired in Prompt 5
	Tools      []anthropicTool      `json:"tools,omitempty"`       // wired in Prompt 2
	ToolChoice *anthropicToolChoice `json:"tool_choice,omitempty"` // wired in Prompt 2
}

// anthropicThinking placeholder — fully defined in Prompt 3.
type anthropicThinking struct {
	Type         string `json:"type"`
	BudgetTokens int    `json:"budget_tokens"`
}

type anthropicTool struct {
	Name        string         `json:"name"`
	Description string         `json:"description"`
	InputSchema map[string]any `json:"input_schema"`
}

type anthropicToolChoice struct {
	Type string `json:"type"` // "tool"
	Name string `json:"name"`
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
	Text string `json:"text,omitempty"`
	// tool_use fields (populated when Type == "tool_use"):
	Name  string         `json:"name,omitempty"`
	Input map[string]any `json:"input,omitempty"`
	// thinking field (populated when Type == "thinking", wired fully in Prompt 3):
	Thinking string `json:"thinking,omitempty"`
}

type anthropicUsage struct {
	InputTokens              int `json:"input_tokens"`
	OutputTokens             int `json:"output_tokens"`
	CacheCreationInputTokens int `json:"cache_creation_input_tokens"`
	CacheReadInputTokens     int `json:"cache_read_input_tokens"`
}

type anthropicStreamEvent struct {
	Type  string                `json:"type"`
	Delta *anthropicStreamDelta `json:"delta,omitempty"`
	Usage *anthropicUsage       `json:"usage,omitempty"`
}

type anthropicStreamDelta struct {
	Type       string `json:"type"`        // "text_delta" or "thinking_delta"
	Text       string `json:"text"`
	Thinking   string `json:"thinking"`
	StopReason string `json:"stop_reason"`
}

type anthropicErrorResponse struct {
	Error struct {
		Type    string `json:"type"`
		Message string `json:"message"`
	} `json:"error"`
}

func floatPtr(f float64) *float64 { return &f }

// Generate calls the Anthropic Messages API.
func (c *AnthropicClient) Generate(ctx context.Context, req GenerateRequest) (*GenerateResponse, *Usage, error) {
	// Guard: thinking and tool_use are incompatible in the same request.
	if req.Thinking != nil && req.JSONSchema != nil {
		return nil, nil, fmt.Errorf(
			"anthropic: Thinking and JSONSchema (tool_use) cannot both be set on the same request",
		)
	}

	// Extract system message from messages slice.
	var staticSystemText string
	messages := make([]anthropicMessage, 0, len(req.Messages))
	for _, msg := range req.Messages {
		if msg.Role == "system" {
			staticSystemText = msg.Content
			continue
		}
		messages = append(messages, anthropicMessage{Role: msg.Role, Content: msg.Content})
	}

	// Build system blocks. Static system prompt is marked for server-side caching.
	// Dynamic content (JSONSchema instruction) is appended as a separate uncached block.
	var sysBlocks []anthropicSystemBlock
	if staticSystemText != "" {
		sysBlocks = append(sysBlocks, anthropicSystemBlock{
			Type:         "text",
			Text:         staticSystemText,
			CacheControl: &anthropicCacheControl{Type: "ephemeral"},
		})
	}
	maxTokens := req.MaxTokens
	if maxTokens <= 0 {
		maxTokens = 1024
	}

	apiReq := anthropicRequest{
		Model:     req.Model,
		MaxTokens: maxTokens,
		Messages:  messages,
	}

	// Native structured output via tool_use forcing.
	if req.JSONSchema != nil {
		apiReq.Tools = []anthropicTool{{
			Name:        "structured_output",
			Description: "Output the result conforming exactly to the provided JSON schema.",
			InputSchema: req.JSONSchema,
		}}
		apiReq.ToolChoice = &anthropicToolChoice{
			Type: "tool",
			Name: "structured_output",
		}
	}
	apiReq.System = sysBlocks

	// Wire extended thinking if requested.
	if req.Thinking != nil {
		apiReq.Thinking = &anthropicThinking{
			Type:         req.Thinking.Type,
			BudgetTokens: req.Thinking.BudgetTokens,
		}
		// Anthropic API requires temperature=1.0 when thinking is enabled.
		apiReq.Temperature = floatPtr(1.0)
	} else {
		if req.Temperature > 0 {
			t := req.Temperature
			apiReq.Temperature = &t
		}
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
			case <-time.After(RetryBackoff(attempt, time.Second, 30*time.Second)):
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
	httpReq.Header.Set("anthropic-beta", "prompt-caching-2024-07-31")
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
	var toolUseOutput string
	var thinkingContent string

	for _, block := range apiResp.Content {
		switch block.Type {
		case "thinking":
			thinkingContent = block.Thinking
		case "tool_use":
			if block.Name == "structured_output" && block.Input != nil {
				raw, err := json.Marshal(block.Input)
				if err == nil {
					toolUseOutput = string(raw)
				}
			}
		case "text":
			contentBuilder.WriteString(block.Text)
		}
	}

	// Prefer tool_use output (structured output path) over text.
	responseContent := contentBuilder.String()
	if toolUseOutput != "" {
		responseContent = toolUseOutput
	}

	return &GenerateResponse{
			Content:         responseContent,
			Model:           apiResp.Model,
			ProviderID:      "anthropic",
			FinishReason:    apiResp.StopReason,
			ThinkingContent: thinkingContent,
		}, &Usage{
			InputTokens:         apiResp.Usage.InputTokens,
			OutputTokens:        apiResp.Usage.OutputTokens,
			CacheCreationTokens: apiResp.Usage.CacheCreationInputTokens,
			CacheReadTokens:     apiResp.Usage.CacheReadInputTokens,
		}, nil
}

// Stream implements Client for Anthropic SSE streaming.
func (c *AnthropicClient) Stream(ctx context.Context, req GenerateRequest, out chan<- StreamChunk) {
	defer close(out)

	var staticSystemText string
	messages := make([]anthropicMessage, 0, len(req.Messages))
	for _, msg := range req.Messages {
		if msg.Role == "system" {
			staticSystemText = msg.Content
			continue
		}
		messages = append(messages, anthropicMessage{Role: msg.Role, Content: msg.Content})
	}

	var sysBlocks []anthropicSystemBlock
	if staticSystemText != "" {
		sysBlocks = append(sysBlocks, anthropicSystemBlock{
			Type:         "text",
			Text:         staticSystemText,
			CacheControl: &anthropicCacheControl{Type: "ephemeral"},
		})
	}

	maxTokens := req.MaxTokens
	if maxTokens <= 0 {
		maxTokens = 1024
	}

	apiReq := anthropicRequest{
		Model:     req.Model,
		MaxTokens: maxTokens,
		Messages:  messages,
		System:    sysBlocks,
		Stream:    true,
	}
	if req.Temperature > 0 {
		apiReq.Temperature = floatPtr(req.Temperature)
	}
	if req.Thinking != nil {
		apiReq.Thinking = &anthropicThinking{
			Type:         req.Thinking.Type,
			BudgetTokens: req.Thinking.BudgetTokens,
		}
		apiReq.Temperature = floatPtr(1.0)
	}

	body, err := json.Marshal(apiReq)
	if err != nil {
		out <- StreamChunk{Error: fmt.Errorf("anthropic stream: marshal: %w", err)}
		return
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost,
		c.baseURL+anthropicMessagesPath, bytes.NewReader(body))
	if err != nil {
		out <- StreamChunk{Error: fmt.Errorf("anthropic stream: build request: %w", err)}
		return
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("x-api-key", c.apiKey)
	httpReq.Header.Set("anthropic-version", anthropicAPIVersion)
	httpReq.Header.Set("anthropic-beta", "prompt-caching-2024-07-31")
	if reqID, ok := ctx.Value(requestIDKey{}).(string); ok && reqID != "" {
		httpReq.Header.Set("X-Request-ID", reqID)
	}

	streamClient := &http.Client{Timeout: 0}
	resp, err := streamClient.Do(httpReq)
	if err != nil {
		out <- StreamChunk{Error: fmt.Errorf("anthropic stream: http: %w", err)}
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		b, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
		out <- StreamChunk{Error: fmt.Errorf("anthropic stream: status %d: %s", resp.StatusCode, b)}
		return
	}

	var cumulativeUsage Usage
	scanner := bufio.NewScanner(resp.Body)
	for scanner.Scan() {
		if ctx.Err() != nil {
			out <- StreamChunk{Error: fmt.Errorf("anthropic stream: %w", ctx.Err())}
			return
		}
		line := scanner.Text()
		if !strings.HasPrefix(line, "data: ") {
			continue
		}
		data := strings.TrimPrefix(line, "data: ")
		if data == "[DONE]" {
			break
		}
		var event anthropicStreamEvent
		if err := json.Unmarshal([]byte(data), &event); err != nil {
			continue
		}
		switch event.Type {
		case "content_block_delta":
			if event.Delta == nil {
				continue
			}
			switch event.Delta.Type {
			case "text_delta":
				out <- StreamChunk{Delta: event.Delta.Text}
			case "thinking_delta":
				out <- StreamChunk{ThinkingDelta: event.Delta.Thinking}
			}
		case "message_delta":
			if event.Delta != nil && event.Delta.StopReason != "" {
				if event.Usage != nil {
					cumulativeUsage.InputTokens = event.Usage.InputTokens
					cumulativeUsage.OutputTokens = event.Usage.OutputTokens
				}
				out <- StreamChunk{
					Done:         true,
					FinishReason: event.Delta.StopReason,
					Usage:        &cumulativeUsage,
				}
				return
			}
		case "message_stop":
			out <- StreamChunk{Done: true, FinishReason: "end_turn", Usage: &cumulativeUsage}
			return
		}
	}
	if err := scanner.Err(); err != nil {
		out <- StreamChunk{Error: fmt.Errorf("anthropic stream: scanner: %w", err)}
		return
	}
	out <- StreamChunk{Done: true, FinishReason: "end_turn", Usage: &cumulativeUsage}
}
