package llm

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"time"
)

const (
	geminiDefaultModel = "gemini-2.0-flash"
	geminiBaseURL      = "https://generativelanguage.googleapis.com/v1beta/models"
)

// GeminiClient implements Client for Google's Gemini API.
type GeminiClient struct {
	apiKey     string
	httpClient *http.Client
	model      string
}

// NewGeminiClient creates a Gemini client using GEMINI_API_KEY from the environment.
func NewGeminiClient() (*GeminiClient, error) {
	key := os.Getenv("GEMINI_API_KEY")
	if key == "" {
		return nil, fmt.Errorf("GEMINI_API_KEY not set")
	}
	return &GeminiClient{
		apiKey:     key,
		httpClient: &http.Client{Timeout: 60 * time.Second},
		model:      geminiDefaultModel,
	}, nil
}

// ProviderName returns "gemini".
func (g *GeminiClient) ProviderName() string { return "gemini" }

// Generate implements Client.Generate for Google Gemini.
func (g *GeminiClient) Generate(ctx context.Context, req GenerateRequest) (*GenerateResponse, *Usage, error) {
	model := req.Model
	if model == "" {
		model = g.model
	}

	type part struct {
		Text       string `json:"text,omitempty"`
		InlineData *struct {
			MimeType string `json:"mimeType"`
			Data     string `json:"data"`
		} `json:"inlineData,omitempty"`
	}
	type content struct {
		Role  string `json:"role"`
		Parts []part `json:"parts"`
	}
	type reqBody struct {
		Contents          []content `json:"contents"`
		SystemInstruction *content  `json:"systemInstruction,omitempty"`
		GenerationConfig  struct {
			MaxOutputTokens int     `json:"maxOutputTokens"`
			Temperature     float64 `json:"temperature,omitempty"`
		} `json:"generationConfig"`
	}

	body := reqBody{}
	body.GenerationConfig.MaxOutputTokens = req.MaxTokens
	if req.MaxTokens == 0 {
		body.GenerationConfig.MaxOutputTokens = 2048
	}
	if req.Temperature > 0 {
		body.GenerationConfig.Temperature = req.Temperature
	}
	if req.System != "" {
		body.SystemInstruction = &content{Parts: []part{{Text: req.System}}}
	}

	// Build user message from Messages or System+Messages.
	userText := ""
	for _, m := range req.Messages {
		if m.Role == "user" {
			userText += m.Content + "\n"
		}
	}
	if userText == "" {
		userText = "Hello"
	}

	userParts := []part{{Text: userText}}
	body.Contents = []content{{Role: "user", Parts: userParts}}

	bodyBytes, _ := json.Marshal(body)
	url := fmt.Sprintf("%s/%s:generateContent?key=%s", geminiBaseURL, model, g.apiKey)
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(bodyBytes))
	if err != nil {
		return nil, nil, fmt.Errorf("gemini request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")

	resp, err := g.httpClient.Do(httpReq)
	if err != nil {
		return nil, nil, fmt.Errorf("gemini http: %w", err)
	}
	defer resp.Body.Close()

	respBytes, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		return nil, nil, fmt.Errorf("gemini status %d: %s", resp.StatusCode, string(respBytes))
	}

	var geminiResp struct {
		Candidates []struct {
			Content struct {
				Parts []struct {
					Text string `json:"text"`
				} `json:"parts"`
			} `json:"content"`
		} `json:"candidates"`
		UsageMetadata struct {
			PromptTokenCount     int `json:"promptTokenCount"`
			CandidatesTokenCount int `json:"candidatesTokenCount"`
		} `json:"usageMetadata"`
	}
	if err := json.Unmarshal(respBytes, &geminiResp); err != nil {
		return nil, nil, fmt.Errorf("gemini parse: %w", err)
	}
	if len(geminiResp.Candidates) == 0 || len(geminiResp.Candidates[0].Content.Parts) == 0 {
		return nil, nil, fmt.Errorf("gemini: empty response")
	}

	usage := &Usage{
		InputTokens:  geminiResp.UsageMetadata.PromptTokenCount,
		OutputTokens: geminiResp.UsageMetadata.CandidatesTokenCount,
	}
	return &GenerateResponse{
		Content:    geminiResp.Candidates[0].Content.Parts[0].Text,
		Model:      model,
		ProviderID: "gemini",
	}, usage, nil
}

// Stream implements Client.Stream (not supported by Gemini in this implementation).
func (g *GeminiClient) Stream(ctx context.Context, req GenerateRequest, out chan<- StreamChunk) {
	resp, usage, err := g.Generate(ctx, req)
	if err != nil {
		out <- StreamChunk{Error: err}
		close(out)
		return
	}
	out <- StreamChunk{Delta: resp.Content, Done: true, Usage: usage}
	close(out)
}

// Ensure base64 is used (for image support).
var _ = base64.StdEncoding

// Compile-time interface check.
var _ Client = (*GeminiClient)(nil)
