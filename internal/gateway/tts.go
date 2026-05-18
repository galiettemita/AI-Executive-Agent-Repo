package gateway

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

var (
	ErrTTSProviderError = errors.New("tts: provider returned error")
	ErrTTSNoProviders   = errors.New("tts: no providers configured")
	ErrTTSInvalidInput  = errors.New("tts: invalid input")
)

// TTSOptions controls speech synthesis behaviour.
type TTSOptions struct {
	Voice  string  // provider-specific voice identifier (e.g. "nova", "alloy")
	Speed  float64 // speech rate multiplier, 1.0 = normal
	Format string  // output format: "mp3", "opus", "wav"
}

// AudioResult is the output of a successful synthesis.
type AudioResult struct {
	AudioURL   string `json:"audio_url"`
	Format     string `json:"format"`
	DurationMs int64  `json:"duration_ms"`
	SizeBytes  int64  `json:"size_bytes"`
}

// TTSProvider is the interface every text-to-speech backend must implement.
type TTSProvider interface {
	Synthesize(ctx context.Context, text string, opts TTSOptions) (*AudioResult, error)
	Name() string
}

// ---------------------------------------------------------------------------
// OpenAITTSProvider — OpenAI TTS API
// ---------------------------------------------------------------------------

// OpenAITTSProvider calls the OpenAI text-to-speech API.
type OpenAITTSProvider struct {
	apiKey  string
	baseURL string
	model   string
	client  *http.Client
}

// OpenAITTSProviderConfig holds configuration for the OpenAI TTS provider.
type OpenAITTSProviderConfig struct {
	APIKey  string
	BaseURL string // defaults to "https://api.openai.com"
	Model   string // defaults to "tts-1"
	Timeout time.Duration
}

// NewOpenAITTSProvider creates an OpenAITTSProvider with the given config.
func NewOpenAITTSProvider(cfg OpenAITTSProviderConfig) *OpenAITTSProvider {
	base := strings.TrimRight(cfg.BaseURL, "/")
	if base == "" {
		base = "https://api.openai.com"
	}
	model := cfg.Model
	if model == "" {
		model = "tts-1"
	}
	timeout := cfg.Timeout
	if timeout <= 0 {
		timeout = 30 * time.Second
	}
	return &OpenAITTSProvider{
		apiKey:  cfg.APIKey,
		baseURL: base,
		model:   model,
		client:  &http.Client{Timeout: timeout},
	}
}

func (o *OpenAITTSProvider) Name() string { return "openai_tts" }

type openAITTSRequest struct {
	Model          string  `json:"model"`
	Input          string  `json:"input"`
	Voice          string  `json:"voice"`
	ResponseFormat string  `json:"response_format,omitempty"`
	Speed          float64 `json:"speed,omitempty"`
}

type openAITTSResponse struct {
	AudioURL   string `json:"audio_url"`
	Format     string `json:"format"`
	DurationMs int64  `json:"duration_ms"`
	SizeBytes  int64  `json:"size_bytes"`
}

func (o *OpenAITTSProvider) Synthesize(ctx context.Context, text string, opts TTSOptions) (*AudioResult, error) {
	text = strings.TrimSpace(text)
	if text == "" {
		return nil, fmt.Errorf("%w: empty text", ErrTTSInvalidInput)
	}

	voice := opts.Voice
	if voice == "" {
		voice = DefaultVoicePipelineConfig().DefaultVoice
	}
	speed := opts.Speed
	if speed <= 0 {
		speed = 1.0
	}
	format := normalizeAudioFormat(opts.Format)

	reqBody := openAITTSRequest{
		Model:          o.model,
		Input:          text,
		Voice:          voice,
		ResponseFormat: format,
		Speed:          speed,
	}
	bodyBytes, err := json.Marshal(reqBody)
	if err != nil {
		return nil, fmt.Errorf("tts: marshal request: %w", err)
	}

	endpoint := o.baseURL + "/v1/audio/speech"
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(bodyBytes))
	if err != nil {
		return nil, fmt.Errorf("tts: build request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+o.apiKey)

	resp, err := o.client.Do(req)
	if err != nil {
		if ctx.Err() != nil {
			return nil, fmt.Errorf("tts: request timed out")
		}
		return nil, fmt.Errorf("%w: %v", ErrTTSProviderError, err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("%w: read response: %v", ErrTTSProviderError, err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("%w: status %d: %s", ErrTTSProviderError, resp.StatusCode, string(respBody))
	}

	var ttsResp openAITTSResponse
	if err := json.Unmarshal(respBody, &ttsResp); err != nil {
		return nil, fmt.Errorf("%w: unmarshal response: %v", ErrTTSProviderError, err)
	}

	resultFormat := ttsResp.Format
	if resultFormat == "" {
		resultFormat = format
	}

	return &AudioResult{
		AudioURL:   ttsResp.AudioURL,
		Format:     resultFormat,
		DurationMs: ttsResp.DurationMs,
		SizeBytes:  ttsResp.SizeBytes,
	}, nil
}

// normalizeAudioFormat maps user-facing format names to API format values.
func normalizeAudioFormat(format string) string {
	switch strings.ToLower(strings.TrimSpace(format)) {
	case "mp3":
		return "mp3"
	case "opus", "ogg/opus", "ogg":
		return "opus"
	case "wav":
		return "wav"
	case "flac":
		return "flac"
	case "aac", "m4a":
		return "aac"
	default:
		return "mp3"
	}
}

// ---------------------------------------------------------------------------
// TTSService — provider selection and synthesis orchestration
// ---------------------------------------------------------------------------

// TTSService manages TTS providers.
type TTSService struct {
	provider TTSProvider
	timeout  time.Duration
	maxChars int
}

// TTSServiceConfig holds configuration for the TTS service.
type TTSServiceConfig struct {
	Provider TTSProvider
	Timeout  time.Duration
	MaxChars int // maximum text length, defaults to 4096
}

// NewTTSService creates a TTSService. A provider is required.
func NewTTSService(cfg TTSServiceConfig) (*TTSService, error) {
	if cfg.Provider == nil {
		return nil, ErrTTSNoProviders
	}
	timeout := cfg.Timeout
	if timeout <= 0 {
		timeout = 30 * time.Second
	}
	maxChars := cfg.MaxChars
	if maxChars <= 0 {
		maxChars = DefaultVoicePipelineConfig().MaxResponseChars
	}
	return &TTSService{
		provider: cfg.Provider,
		timeout:  timeout,
		maxChars: maxChars,
	}, nil
}

// Synthesize converts text to speech, enforcing length limits and timeout.
func (s *TTSService) Synthesize(ctx context.Context, text string, opts TTSOptions) (*AudioResult, error) {
	text = strings.TrimSpace(text)
	if text == "" {
		return nil, fmt.Errorf("%w: empty text", ErrTTSInvalidInput)
	}
	if len(text) > s.maxChars {
		return nil, fmt.Errorf("%w: text exceeds maximum length of %d characters", ErrTTSInvalidInput, s.maxChars)
	}

	ctx, cancel := context.WithTimeout(ctx, s.timeout)
	defer cancel()

	return s.provider.Synthesize(ctx, text, opts)
}
