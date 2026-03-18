package llm

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"time"
)

const (
	mistralDefaultModel = "mistral-large-2411"
	mistralBaseURL      = "https://api.mistral.ai/v1/chat/completions"
)

// MistralClient implements Client for the Mistral AI API.
// Mistral offers EU data residency, making it the preferred provider for EU workspaces.
type MistralClient struct {
	apiKey     string
	httpClient *http.Client
}

// NewMistralClient creates a Mistral client using MISTRAL_API_KEY from the environment.
func NewMistralClient() (*MistralClient, error) {
	key := os.Getenv("MISTRAL_API_KEY")
	if key == "" {
		return nil, fmt.Errorf("MISTRAL_API_KEY not set")
	}
	return &MistralClient{apiKey: key, httpClient: &http.Client{Timeout: 60 * time.Second}}, nil
}

// ProviderName returns "mistral".
func (m *MistralClient) ProviderName() string { return "mistral" }

// Generate implements Client.Generate for Mistral AI.
func (m *MistralClient) Generate(ctx context.Context, req GenerateRequest) (*GenerateResponse, *Usage, error) {
	model := req.Model
	if model == "" {
		model = mistralDefaultModel
	}

	type message struct {
		Role    string `json:"role"`
		Content string `json:"content"`
	}
	msgs := []message{}
	if req.System != "" {
		msgs = append(msgs, message{Role: "system", Content: req.System})
	}
	for _, m := range req.Messages {
		msgs = append(msgs, message{Role: m.Role, Content: m.Content})
	}
	if len(msgs) == 0 {
		msgs = append(msgs, message{Role: "user", Content: "Hello"})
	}

	maxTokens := req.MaxTokens
	if maxTokens == 0 {
		maxTokens = 2048
	}

	body := map[string]interface{}{
		"model":      model,
		"messages":   msgs,
		"max_tokens": maxTokens,
	}
	if req.Temperature > 0 {
		body["temperature"] = req.Temperature
	}

	bodyBytes, _ := json.Marshal(body)
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, mistralBaseURL, bytes.NewReader(bodyBytes))
	if err != nil {
		return nil, nil, fmt.Errorf("mistral request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Authorization", "Bearer "+m.apiKey)

	resp, err := m.httpClient.Do(httpReq)
	if err != nil {
		return nil, nil, fmt.Errorf("mistral http: %w", err)
	}
	defer resp.Body.Close()
	respBytes, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		return nil, nil, fmt.Errorf("mistral status %d: %s", resp.StatusCode, string(respBytes))
	}

	var mistralResp struct {
		Choices []struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
		} `json:"choices"`
		Usage struct {
			PromptTokens     int `json:"prompt_tokens"`
			CompletionTokens int `json:"completion_tokens"`
		} `json:"usage"`
	}
	if err := json.Unmarshal(respBytes, &mistralResp); err != nil {
		return nil, nil, fmt.Errorf("mistral parse: %w", err)
	}
	if len(mistralResp.Choices) == 0 {
		return nil, nil, fmt.Errorf("mistral: empty choices")
	}

	usage := &Usage{
		InputTokens:  mistralResp.Usage.PromptTokens,
		OutputTokens: mistralResp.Usage.CompletionTokens,
	}
	return &GenerateResponse{
		Content:    mistralResp.Choices[0].Message.Content,
		Model:      model,
		ProviderID: "mistral",
	}, usage, nil
}

// Stream implements Client.Stream (delegates to non-streaming Generate).
func (m *MistralClient) Stream(ctx context.Context, req GenerateRequest, out chan<- StreamChunk) {
	resp, usage, err := m.Generate(ctx, req)
	if err != nil {
		out <- StreamChunk{Error: err}
		close(out)
		return
	}
	out <- StreamChunk{Delta: resp.Content, Done: true, Usage: usage}
	close(out)
}

// Compile-time interface check.
var _ Client = (*MistralClient)(nil)
