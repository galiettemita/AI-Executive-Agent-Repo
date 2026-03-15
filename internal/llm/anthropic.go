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

type anthropicRequest struct {
	Model       string             `json:"model"`
	MaxTokens   int                `json:"max_tokens"`
	Messages    []anthropicMessage `json:"messages"`
	System      string             `json:"system,omitempty"`
	Temperature *float64           `json:"temperature,omitempty"`
	TopP        *float64           `json:"top_p,omitempty"`
	Tools       []anthropicTool    `json:"tools,omitempty"`
	ToolChoice  any                `json:"tool_choice,omitempty"`
	Thinking    *anthropicThinking `json:"thinking,omitempty"`
	Stream      bool               `json:"stream,omitempty"`
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

type anthropicMessage struct {
	Role    string `json:"role"`
	Content any    `json:"content"` // string OR []anthropicContentBlock
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
	Type     string         `json:"type"`               // text|tool_use|tool_result|thinking
	Text     string         `json:"text,omitempty"`
	ID       string         `json:"id,omitempty"`       // tool_use and tool_result
	Name     string         `json:"name,omitempty"`     // tool_use
	Input    map[string]any `json:"input,omitempty"`    // tool_use
	Content  string         `json:"content,omitempty"`  // tool_result value
	IsError  bool           `json:"is_error,omitempty"` // tool_result
	Thinking string         `json:"thinking,omitempty"` // thinking block
}

const anthropicBetaThinking = "interleaved-thinking-2025-05-14"

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

// Generate calls the Anthropic Messages API with full support for tool use and
// extended thinking.
func (c *AnthropicClient) Generate(ctx context.Context, req GenerateRequest) (*GenerateResponse, *Usage, error) {
	// Guard: thinking and JSONSchema (tool_use forcing) are incompatible.
	if req.Thinking != nil && req.Thinking.Enabled && req.JSONSchema != nil {
		return nil, nil, fmt.Errorf(
			"anthropic: Thinking and JSONSchema (tool_use) cannot both be set on the same request",
		)
	}

	maxTokens := req.MaxTokens
	if maxTokens <= 0 {
		maxTokens = 1024
	}

	// Build system prompt: prefer explicit System field, fall back to extracting from messages.
	systemPrompt := req.System
	var filteredMsgs []ChatMsg
	for _, msg := range req.Messages {
		if msg.Role == "system" {
			if systemPrompt == "" {
				systemPrompt = msg.Content
			}
			continue
		}
		filteredMsgs = append(filteredMsgs, msg)
	}

	apiReq := anthropicRequest{
		Model:     req.Model,
		MaxTokens: maxTokens,
		System:    systemPrompt,
	}
	if req.Temperature != 0 {
		t := req.Temperature
		apiReq.Temperature = &t
	}
	if req.TopP != 0 {
		p := req.TopP
		apiReq.TopP = &p
	}

	// Build messages with correct multi-turn tool use format.
	apiReq.Messages = buildAnthropicMessages(filteredMsgs, req.PriorAssistantToolCalls, req.ToolResults)

	// Tool definitions from explicit Tools field.
	if len(req.Tools) > 0 {
		apiReq.Tools = make([]anthropicTool, len(req.Tools))
		for i, t := range req.Tools {
			apiReq.Tools[i] = anthropicTool{
				Name:        t.Name,
				Description: t.Description,
				InputSchema: t.InputSchema,
			}
		}
		switch req.ToolChoice {
		case ToolChoiceAny:
			apiReq.ToolChoice = map[string]string{"type": "any"}
		case ToolChoiceNone:
			apiReq.ToolChoice = map[string]string{"type": "none"}
		default:
			tc := string(req.ToolChoice)
			if tc != "" && req.ToolChoice != ToolChoiceAuto {
				apiReq.ToolChoice = map[string]any{"type": "tool", "name": tc}
			} else {
				apiReq.ToolChoice = map[string]string{"type": "auto"}
			}
		}
	}

	// Native structured output via tool_use forcing (JSONSchema path).
	if req.JSONSchema != nil && len(req.Tools) == 0 {
		apiReq.Tools = []anthropicTool{{
			Name:        "structured_output",
			Description: "Output the result conforming exactly to the provided JSON schema.",
			InputSchema: req.JSONSchema,
		}}
		apiReq.ToolChoice = map[string]any{"type": "tool", "name": "structured_output"}
	}

	// Extended thinking.
	useThinkingBeta := false
	if req.Thinking != nil && req.Thinking.Enabled {
		budget := req.Thinking.BudgetTokens
		if budget < 1024 {
			budget = 1024
		}
		if budget > 32768 {
			budget = 32768
		}
		apiReq.Thinking = &anthropicThinking{Type: "enabled", BudgetTokens: budget}
		one := 1.0
		apiReq.Temperature = &one
		apiReq.TopP = nil
		useThinkingBeta = true
	}

	body, err := json.Marshal(apiReq)
	if err != nil {
		return nil, nil, fmt.Errorf("anthropic: marshal: %w", err)
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
		resp, usage, err := c.doRequestFull(ctx, body, useThinkingBeta)
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

// buildAnthropicMessages constructs the message array per Anthropic multi-turn spec.
func buildAnthropicMessages(msgs []ChatMsg, priorToolCalls []AssistantToolUse, toolResults []ToolResult) []anthropicMessage {
	out := make([]anthropicMessage, 0, len(msgs)+2)

	for _, m := range msgs {
		out = append(out, anthropicMessage{Role: m.Role, Content: m.Content})
	}

	// Reconstruct prior assistant turn as structured content with tool_use blocks.
	if len(priorToolCalls) > 0 {
		blocks := make([]anthropicContentBlock, 0, len(priorToolCalls)*2)
		for _, tc := range priorToolCalls {
			if tc.Text != "" {
				blocks = append(blocks, anthropicContentBlock{Type: "text", Text: tc.Text})
			}
			blocks = append(blocks, anthropicContentBlock{
				Type:  "tool_use",
				ID:    tc.ID,
				Name:  tc.Name,
				Input: tc.Input,
			})
		}
		out = append(out, anthropicMessage{Role: "assistant", Content: blocks})
	}

	// Tool results become a user message with tool_result content blocks.
	if len(toolResults) > 0 {
		blocks := make([]anthropicContentBlock, len(toolResults))
		for i, tr := range toolResults {
			blocks[i] = anthropicContentBlock{
				Type:    "tool_result",
				ID:      tr.ToolCallID,
				Content: tr.Content,
				IsError: tr.IsError,
			}
		}
		out = append(out, anthropicMessage{Role: "user", Content: blocks})
	}

	return out
}

func (c *AnthropicClient) doRequest(ctx context.Context, body []byte) (*GenerateResponse, *Usage, error) {
	return c.doRequestFull(ctx, body, false)
}

func (c *AnthropicClient) doRequestFull(ctx context.Context, body []byte, thinkingBeta bool) (*GenerateResponse, *Usage, error) {
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost,
		c.baseURL+anthropicMessagesPath, bytes.NewReader(body))
	if err != nil {
		return nil, nil, fmt.Errorf("anthropic: create request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("x-api-key", c.apiKey)
	httpReq.Header.Set("anthropic-version", anthropicAPIVersion)
	if thinkingBeta {
		httpReq.Header.Set("anthropic-beta", anthropicBetaThinking)
	}
	if reqID, ok := ctx.Value(requestIDKey{}).(string); ok && reqID != "" {
		httpReq.Header.Set("X-Request-ID", reqID)
	}

	httpResp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return nil, nil, fmt.Errorf("anthropic: http: %w", err)
	}
	defer httpResp.Body.Close()

	respBody, err := io.ReadAll(io.LimitReader(httpResp.Body, 4<<20))
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
		return nil, nil, fmt.Errorf("anthropic: unmarshal: %w", err)
	}

	var textB, thinkingB strings.Builder
	var toolCalls []ToolCall
	var structuredOutput string

	for _, block := range apiResp.Content {
		switch block.Type {
		case "text":
			textB.WriteString(block.Text)
		case "thinking":
			thinkingB.WriteString(block.Thinking)
		case "tool_use":
			toolCalls = append(toolCalls, ToolCall{
				ID:    block.ID,
				Name:  block.Name,
				Input: block.Input,
			})
			// Handle structured_output tool for JSONSchema path
			if block.Name == "structured_output" && block.Input != nil {
				raw, merr := json.Marshal(block.Input)
				if merr == nil {
					structuredOutput = string(raw)
				}
			}
		}
	}

	responseContent := textB.String()
	if structuredOutput != "" {
		responseContent = structuredOutput
	}

	return &GenerateResponse{
		Content:         responseContent,
		ThinkingContent: thinkingB.String(),
		ToolCalls:       toolCalls,
		Model:           apiResp.Model,
		ProviderID:      "anthropic",
		FinishReason:    apiResp.StopReason,
		InputTokens:     apiResp.Usage.InputTokens,
		OutputTokens:    apiResp.Usage.OutputTokens,
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

	systemPrompt := req.System
	var filteredMsgs []ChatMsg
	for _, msg := range req.Messages {
		if msg.Role == "system" {
			if systemPrompt == "" {
				systemPrompt = msg.Content
			}
			continue
		}
		filteredMsgs = append(filteredMsgs, msg)
	}

	maxTokens := req.MaxTokens
	if maxTokens <= 0 {
		maxTokens = 1024
	}

	msgs := buildAnthropicMessages(filteredMsgs, nil, nil)

	apiReq := anthropicRequest{
		Model:     req.Model,
		MaxTokens: maxTokens,
		Messages:  msgs,
		System:    systemPrompt,
		Stream:    true,
	}
	if req.Temperature > 0 {
		apiReq.Temperature = floatPtr(req.Temperature)
	}
	if req.Thinking != nil && req.Thinking.Enabled {
		budget := req.Thinking.BudgetTokens
		if budget < 1024 {
			budget = 1024
		}
		apiReq.Thinking = &anthropicThinking{
			Type:         "enabled",
			BudgetTokens: budget,
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
