package gateway

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

var (
	ErrVisionProviderError = errors.New("vision: provider error")
	ErrVisionUnsupported   = errors.New("vision: unsupported image type")
	ErrVisionImageTooLarge = errors.New("vision: image exceeds maximum size")
	ErrVisionEmptyImage    = errors.New("vision: image data is empty")
)

// MaxVisionImageBytes is the maximum image size accepted for vision analysis.
const MaxVisionImageBytes = 20 * 1024 * 1024 // 20 MB

var visionSupportedMIME = map[string]bool{
	"image/jpeg": true,
	"image/png":  true,
	"image/webp": true,
	"image/gif":  true,
}

const defaultVisionPrompt = `Analyse this image and extract the following as JSON:
{
  "extracted_text": "<all visible text verbatim>",
  "summary": "<1-2 sentence description>",
  "data_points": ["<key fact or number>"],
  "tags": ["<category tag>"]
}
Return ONLY valid JSON, no markdown, no explanation.`

// VisionInput is the input to a vision analysis request.
type VisionInput struct {
	ImageData []byte
	MIMEType  string
	Prompt    string // if empty, defaultVisionPrompt is used
	MaxChars  int    // max response chars; default 4096
}

// VisionResult is the structured output of vision analysis.
type VisionResult struct {
	ExtractedText  string   `json:"extracted_text"`
	Summary        string   `json:"summary"`
	DataPoints     []string `json:"data_points"`
	Tags           []string `json:"tags"`
	Provider       string   `json:"provider"`
	ConfidenceHint float64  `json:"confidence_hint"`
}

// VisionProvider analyses images and extracts structured information.
type VisionProvider interface {
	Analyse(ctx context.Context, input VisionInput) (*VisionResult, error)
	Name() string
}

// GeminiVisionProviderConfig configures the Gemini vision provider.
type GeminiVisionProviderConfig struct {
	APIKey  string
	BaseURL string        // defaults to https://generativelanguage.googleapis.com
	Model   string        // defaults to gemini-2.5-flash
	Timeout time.Duration // defaults to 30s
}

// GeminiVisionProvider analyses images using Gemini 2.5 Flash.
type GeminiVisionProvider struct {
	apiKey  string
	baseURL string
	model   string
	timeout time.Duration
	client  *http.Client
}

// NewGeminiVisionProvider creates a GeminiVisionProvider.
func NewGeminiVisionProvider(cfg GeminiVisionProviderConfig) (*GeminiVisionProvider, error) {
	if cfg.APIKey == "" {
		return nil, fmt.Errorf("gemini vision: APIKey is required")
	}
	base := strings.TrimRight(cfg.BaseURL, "/")
	if base == "" {
		base = "https://generativelanguage.googleapis.com"
	}
	model := cfg.Model
	if model == "" {
		model = "gemini-2.5-flash"
	}
	timeout := cfg.Timeout
	if timeout <= 0 {
		timeout = 30 * time.Second
	}
	return &GeminiVisionProvider{
		apiKey:  cfg.APIKey,
		baseURL: base,
		model:   model,
		timeout: timeout,
		client:  &http.Client{Timeout: timeout},
	}, nil
}

func (g *GeminiVisionProvider) Name() string { return "gemini_vision" }

func (g *GeminiVisionProvider) Analyse(ctx context.Context, input VisionInput) (*VisionResult, error) {
	if len(input.ImageData) == 0 {
		return nil, ErrVisionEmptyImage
	}
	if len(input.ImageData) > MaxVisionImageBytes {
		return nil, ErrVisionImageTooLarge
	}
	mime := strings.ToLower(strings.TrimSpace(input.MIMEType))
	if !visionSupportedMIME[mime] {
		return nil, fmt.Errorf("%w: %s", ErrVisionUnsupported, mime)
	}

	prompt := input.Prompt
	if prompt == "" {
		prompt = defaultVisionPrompt
	}
	maxChars := input.MaxChars
	if maxChars <= 0 {
		maxChars = 4096
	}

	imgB64 := base64.StdEncoding.EncodeToString(input.ImageData)
	reqBody := map[string]any{
		"contents": []map[string]any{{
			"parts": []map[string]any{
				{"inline_data": map[string]string{"mime_type": mime, "data": imgB64}},
				{"text": prompt},
			},
		}},
		"generationConfig": map[string]any{
			"responseMimeType": "application/json",
			"temperature":      0,
			"maxOutputTokens":  maxChars / 4,
		},
	}

	payload, err := json.Marshal(reqBody)
	if err != nil {
		return nil, fmt.Errorf("gemini vision: marshal request: %w", err)
	}

	endpoint := fmt.Sprintf("%s/v1beta/models/%s:generateContent?key=%s",
		g.baseURL, g.model, g.apiKey)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(payload))
	if err != nil {
		return nil, fmt.Errorf("gemini vision: build request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := g.client.Do(req)
	if err != nil {
		if ctx.Err() != nil {
			return nil, fmt.Errorf("gemini vision: timed out: %w", ctx.Err())
		}
		return nil, fmt.Errorf("%w: %v", ErrVisionProviderError, err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return nil, fmt.Errorf("%w: read response: %v", ErrVisionProviderError, err)
	}
	if resp.StatusCode == http.StatusUnauthorized {
		return nil, fmt.Errorf("%w: invalid API key (401)", ErrVisionProviderError)
	}
	if resp.StatusCode != http.StatusOK {
		preview := string(body)
		if len(preview) > 300 {
			preview = preview[:300]
		}
		return nil, fmt.Errorf("%w: status %d: %s", ErrVisionProviderError, resp.StatusCode, preview)
	}

	var envelope struct {
		Candidates []struct {
			Content struct {
				Parts []struct {
					Text string `json:"text"`
				} `json:"parts"`
			} `json:"content"`
		} `json:"candidates"`
	}
	if err := json.Unmarshal(body, &envelope); err != nil {
		return nil, fmt.Errorf("%w: unmarshal envelope: %v", ErrVisionProviderError, err)
	}
	if len(envelope.Candidates) == 0 || len(envelope.Candidates[0].Content.Parts) == 0 {
		return nil, fmt.Errorf("%w: no content in response", ErrVisionProviderError)
	}

	rawText := strings.TrimSpace(envelope.Candidates[0].Content.Parts[0].Text)
	rawText = strings.TrimPrefix(rawText, "```json")
	rawText = strings.TrimPrefix(rawText, "```")
	rawText = strings.TrimSuffix(rawText, "```")
	rawText = strings.TrimSpace(rawText)

	var vr VisionResult
	if err := json.Unmarshal([]byte(rawText), &vr); err != nil {
		// Non-JSON response: return raw text in summary.
		return &VisionResult{
			Summary: rawText, Provider: g.Name(), ConfidenceHint: 0.5,
			DataPoints: []string{}, Tags: []string{},
		}, nil
	}

	vr.Provider = g.Name()
	vr.ConfidenceHint = clampF(vr.ConfidenceHint, 0, 1) // clampF from tts_cartesia.go
	if vr.DataPoints == nil {
		vr.DataPoints = []string{}
	}
	if vr.Tags == nil {
		vr.Tags = []string{}
	}
	return &vr, nil
}

// Compile-time check.
var _ VisionProvider = (*GeminiVisionProvider)(nil)
