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

const ollamaDefaultBaseURL = "http://localhost:11434"

// OllamaClient implements Client for local Ollama inference.
// Used when PRIVACY_MODE=strict or OLLAMA_BASE_URL is set.
type OllamaClient struct {
	baseURL    string
	model      string
	httpClient *http.Client
}

// NewOllamaClient creates an Ollama client. Always returns non-nil.
func NewOllamaClient(defaultModel string) *OllamaClient {
	baseURL := os.Getenv("OLLAMA_BASE_URL")
	if baseURL == "" {
		baseURL = ollamaDefaultBaseURL
	}
	if defaultModel == "" {
		defaultModel = "llama3.2"
	}
	return &OllamaClient{
		baseURL:    baseURL,
		model:      defaultModel,
		httpClient: &http.Client{Timeout: 120 * time.Second},
	}
}

// ProviderName returns "ollama".
func (o *OllamaClient) ProviderName() string { return "ollama" }

// Generate implements Client.Generate for Ollama.
func (o *OllamaClient) Generate(ctx context.Context, req GenerateRequest) (*GenerateResponse, *Usage, error) {
	model := req.Model
	if model == "" {
		model = o.model
	}

	prompt := ""
	for _, m := range req.Messages {
		if m.Role == "user" {
			prompt += m.Content + "\n"
		}
	}
	if prompt == "" {
		prompt = "Hello"
	}

	body := map[string]interface{}{
		"model":  model,
		"prompt": prompt,
		"system": req.System,
		"stream": false,
	}
	bodyBytes, _ := json.Marshal(body)
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, o.baseURL+"/api/generate", bytes.NewReader(bodyBytes))
	if err != nil {
		return nil, nil, fmt.Errorf("ollama request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")

	resp, err := o.httpClient.Do(httpReq)
	if err != nil {
		return nil, nil, fmt.Errorf("ollama http: %w", err)
	}
	defer resp.Body.Close()
	respBytes, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		return nil, nil, fmt.Errorf("ollama status %d: %s", resp.StatusCode, string(respBytes))
	}

	var ollamaResp struct {
		Response        string `json:"response"`
		PromptEvalCount int    `json:"prompt_eval_count"`
		EvalCount       int    `json:"eval_count"`
	}
	if err := json.Unmarshal(respBytes, &ollamaResp); err != nil {
		return nil, nil, fmt.Errorf("ollama parse: %w", err)
	}

	usage := &Usage{
		InputTokens:  ollamaResp.PromptEvalCount,
		OutputTokens: ollamaResp.EvalCount,
	}
	return &GenerateResponse{
		Content:    ollamaResp.Response,
		Model:      model,
		ProviderID: "ollama",
	}, usage, nil
}

// Stream implements Client.Stream (delegates to non-streaming Generate).
func (o *OllamaClient) Stream(ctx context.Context, req GenerateRequest, out chan<- StreamChunk) {
	resp, usage, err := o.Generate(ctx, req)
	if err != nil {
		out <- StreamChunk{Error: err}
		close(out)
		return
	}
	out <- StreamChunk{Delta: resp.Content, Done: true, Usage: usage}
	close(out)
}

// Compile-time interface check.
var _ Client = (*OllamaClient)(nil)
