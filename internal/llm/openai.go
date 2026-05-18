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
	openaiDefaultBaseURL  = "https://api.openai.com"
	openaiResponsesPath   = "/v1/responses"
	openaiMaxRetries      = 2
)

// OpenAIClient implements Client for the OpenAI Responses API.
type OpenAIClient struct {
	apiKey     string
	baseURL    string
	httpClient *http.Client
}

// OpenAIConfig holds configuration for the OpenAI client.
type OpenAIConfig struct {
	APIKey  string
	BaseURL string
	Timeout time.Duration
}

// NewOpenAIClient creates a new OpenAI Responses API client.
func NewOpenAIClient(cfg OpenAIConfig) (*OpenAIClient, error) {
	if cfg.APIKey == "" {
		return nil, fmt.Errorf("openai: API key is required")
	}
	baseURL := cfg.BaseURL
	if baseURL == "" {
		baseURL = openaiDefaultBaseURL
	}
	baseURL = strings.TrimRight(baseURL, "/")
	timeout := cfg.Timeout
	if timeout <= 0 {
		timeout = 60 * time.Second
	}
	return &OpenAIClient{
		apiKey:  cfg.APIKey,
		baseURL: baseURL,
		httpClient: &http.Client{
			Timeout: timeout,
		},
	}, nil
}

// openaiResponsesRequest is the body for POST /v1/responses.
type openaiResponsesRequest struct {
	Model        string              `json:"model"`
	Input        []openaiInputItem   `json:"input"`
	MaxTokens    int                 `json:"max_output_tokens,omitempty"`
	Temperature  *float64            `json:"temperature,omitempty"`
	TopP         *float64            `json:"top_p,omitempty"`
	Text         *openaiTextConfig   `json:"text,omitempty"`
}

type openaiInputItem struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type openaiTextConfig struct {
	Format *openaiTextFormat `json:"format,omitempty"`
}

type openaiTextFormat struct {
	Type   string         `json:"type"`
	Name   string         `json:"name,omitempty"`
	Schema map[string]any `json:"schema,omitempty"`
	Strict bool           `json:"strict,omitempty"`
}

type openaiResponsesResponse struct {
	ID            string             `json:"id"`
	Status        string             `json:"status"`
	Output        []openaiOutputItem `json:"output"`
	Model         string             `json:"model"`
	Usage         openaiUsage        `json:"usage"`
	Error         *openaiError       `json:"error,omitempty"`
}

type openaiOutputItem struct {
	Type    string              `json:"type"`
	ID      string              `json:"id"`
	Content []openaiContentPart `json:"content,omitempty"`
}

type openaiContentPart struct {
	Type string `json:"type"`
	Text string `json:"text"`
}

type openaiUsage struct {
	InputTokens  int `json:"input_tokens"`
	OutputTokens int `json:"output_tokens"`
}

type openaiError struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

// Generate calls the OpenAI Responses API (POST /v1/responses).
func (c *OpenAIClient) Generate(ctx context.Context, req GenerateRequest) (*GenerateResponse, *Usage, error) {
	input := make([]openaiInputItem, 0, len(req.Messages))
	for _, msg := range req.Messages {
		role := msg.Role
		if role == "system" {
			role = "developer"
		}
		input = append(input, openaiInputItem{
			Role:    role,
			Content: msg.Content,
		})
	}

	apiReq := openaiResponsesRequest{
		Model: req.Model,
		Input: input,
	}
	if req.MaxTokens > 0 {
		apiReq.MaxTokens = req.MaxTokens
	}
	if req.Temperature > 0 {
		t := req.Temperature
		apiReq.Temperature = &t
	}
	if req.TopP > 0 && req.TopP < 1.0 {
		p := req.TopP
		apiReq.TopP = &p
	}

	// Structured output via json_schema format.
	if req.JSONSchema != nil {
		apiReq.Text = &openaiTextConfig{
			Format: &openaiTextFormat{
				Type:   "json_schema",
				Name:   "structured_output",
				Schema: req.JSONSchema,
				Strict: true,
			},
		}
	}

	body, err := json.Marshal(apiReq)
	if err != nil {
		return nil, nil, fmt.Errorf("openai: marshal request: %w", err)
	}

	var lastErr error
	for attempt := 0; attempt <= openaiMaxRetries; attempt++ {
		if attempt > 0 {
			select {
			case <-ctx.Done():
				return nil, nil, fmt.Errorf("openai: %w", ctx.Err())
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
	return nil, nil, fmt.Errorf("openai: exhausted retries: %w", lastErr)
}

func (c *OpenAIClient) doRequest(ctx context.Context, body []byte) (*GenerateResponse, *Usage, error) {
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+openaiResponsesPath, bytes.NewReader(body))
	if err != nil {
		return nil, nil, fmt.Errorf("openai: create request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Authorization", "Bearer "+c.apiKey)
	if reqID, ok := ctx.Value(requestIDKey{}).(string); ok && reqID != "" {
		httpReq.Header.Set("X-Request-ID", reqID)
	}

	httpResp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return nil, nil, fmt.Errorf("openai: http error: %w", err)
	}
	defer httpResp.Body.Close()

	respBody, err := io.ReadAll(io.LimitReader(httpResp.Body, 1<<20))
	if err != nil {
		return nil, nil, fmt.Errorf("openai: read response: %w", err)
	}

	if httpResp.StatusCode != http.StatusOK {
		var errData struct {
			Error openaiError `json:"error"`
		}
		_ = json.Unmarshal(respBody, &errData)
		return nil, nil, fmt.Errorf("openai: status %d: %s: %s",
			httpResp.StatusCode, errData.Error.Code, errData.Error.Message)
	}

	var apiResp openaiResponsesResponse
	if err := json.Unmarshal(respBody, &apiResp); err != nil {
		return nil, nil, fmt.Errorf("openai: unmarshal response: %w", err)
	}

	if apiResp.Error != nil {
		return nil, nil, fmt.Errorf("openai: api error: %s: %s", apiResp.Error.Code, apiResp.Error.Message)
	}

	// Extract text content from output items.
	var contentBuilder strings.Builder
	for _, item := range apiResp.Output {
		if item.Type == "message" {
			for _, part := range item.Content {
				if part.Type == "output_text" {
					contentBuilder.WriteString(part.Text)
				}
			}
		}
	}

	finishReason := "completed"
	if apiResp.Status != "" {
		finishReason = apiResp.Status
	}

	return &GenerateResponse{
			Content:      contentBuilder.String(),
			Model:        apiResp.Model,
			ProviderID:   "openai",
			FinishReason: finishReason,
		}, &Usage{
			InputTokens:  apiResp.Usage.InputTokens,
			OutputTokens: apiResp.Usage.OutputTokens,
		}, nil
}
