package gateway

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

var (
	ErrSTTTimeout       = errors.New("stt: transcription timed out")
	ErrSTTInvalidAudio  = errors.New("stt: invalid audio URL")
	ErrSTTProviderError = errors.New("stt: provider returned error")
	ErrSTTNoProviders   = errors.New("stt: no providers configured")
)

// STTOptions controls transcription behaviour.
type STTOptions struct {
	Language       string // BCP-47 language code, e.g. "en", "de"
	MaxDurationSec int    // max audio length the provider should accept
	Model          string // provider-specific model identifier
}

// TranscriptSegment represents a time-aligned word/phrase segment.
type TranscriptSegment struct {
	StartMs    int64   `json:"start_ms"`
	EndMs      int64   `json:"end_ms"`
	Text       string  `json:"text"`
	Confidence float64 `json:"confidence"`
}

// TranscriptResult is the output of a successful transcription.
type TranscriptResult struct {
	Text       string             `json:"text"`
	Confidence float64            `json:"confidence"`
	Language   string             `json:"language"`
	DurationMs int64              `json:"duration_ms"`
	Segments   []TranscriptSegment `json:"segments,omitempty"`
}

// STTProvider is the interface every speech-to-text backend must implement.
type STTProvider interface {
	Transcribe(ctx context.Context, audioURL string, opts STTOptions) (*TranscriptResult, error)
	Name() string
}

// ---------------------------------------------------------------------------
// WhisperProvider — OpenAI Whisper API
// ---------------------------------------------------------------------------

// WhisperProvider calls the OpenAI Whisper API for transcription.
type WhisperProvider struct {
	apiKey  string
	baseURL string
	client  *http.Client
}

// WhisperProviderConfig holds configuration for the Whisper provider.
type WhisperProviderConfig struct {
	APIKey  string
	BaseURL string // defaults to "https://api.openai.com"
	Timeout time.Duration
}

// NewWhisperProvider creates a WhisperProvider with the given config.
func NewWhisperProvider(cfg WhisperProviderConfig) *WhisperProvider {
	base := strings.TrimRight(cfg.BaseURL, "/")
	if base == "" {
		base = "https://api.openai.com"
	}
	timeout := cfg.Timeout
	if timeout <= 0 {
		timeout = 30 * time.Second
	}
	return &WhisperProvider{
		apiKey:  cfg.APIKey,
		baseURL: base,
		client:  &http.Client{Timeout: timeout},
	}
}

func (w *WhisperProvider) Name() string { return "openai_whisper" }

// whisperRequest is the JSON body sent to the Whisper transcription endpoint.
type whisperRequest struct {
	AudioURL string `json:"audio_url"`
	Model    string `json:"model"`
	Language string `json:"language,omitempty"`
}

// whisperResponse is the expected JSON response from the Whisper API.
type whisperResponse struct {
	Text     string  `json:"text"`
	Language string  `json:"language"`
	Duration float64 `json:"duration"`
	Segments []struct {
		Start float64 `json:"start"`
		End   float64 `json:"end"`
		Text  string  `json:"text"`
	} `json:"segments"`
}

func (w *WhisperProvider) Transcribe(ctx context.Context, audioURL string, opts STTOptions) (*TranscriptResult, error) {
	model := opts.Model
	if model == "" {
		model = "whisper-1"
	}

	reqBody := whisperRequest{
		AudioURL: audioURL,
		Model:    model,
		Language: opts.Language,
	}
	bodyBytes, err := json.Marshal(reqBody)
	if err != nil {
		return nil, fmt.Errorf("stt: marshal request: %w", err)
	}

	endpoint := w.baseURL + "/v1/audio/transcriptions"
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(bodyBytes))
	if err != nil {
		return nil, fmt.Errorf("stt: build request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+w.apiKey)

	resp, err := w.client.Do(req)
	if err != nil {
		if ctx.Err() != nil {
			return nil, ErrSTTTimeout
		}
		return nil, fmt.Errorf("%w: %v", ErrSTTProviderError, err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("%w: read response: %v", ErrSTTProviderError, err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("%w: status %d: %s", ErrSTTProviderError, resp.StatusCode, string(respBody))
	}

	var wr whisperResponse
	if err := json.Unmarshal(respBody, &wr); err != nil {
		return nil, fmt.Errorf("%w: unmarshal response: %v", ErrSTTProviderError, err)
	}

	segments := make([]TranscriptSegment, 0, len(wr.Segments))
	for _, seg := range wr.Segments {
		segments = append(segments, TranscriptSegment{
			StartMs:    int64(seg.Start * 1000),
			EndMs:      int64(seg.End * 1000),
			Text:       seg.Text,
			Confidence: 0.9, // Whisper does not return per-segment confidence
		})
	}

	lang := wr.Language
	if lang == "" {
		lang = opts.Language
	}

	return &TranscriptResult{
		Text:       wr.Text,
		Confidence: 0.9, // Whisper does not return a global confidence score
		Language:   lang,
		DurationMs: int64(wr.Duration * 1000),
		Segments:   segments,
	}, nil
}

// ---------------------------------------------------------------------------
// GoogleSTTProvider — Google Cloud Speech-to-Text V2 (fallback)
// ---------------------------------------------------------------------------

// GoogleSTTProvider calls the Google Cloud Speech-to-Text API.
type GoogleSTTProvider struct {
	apiKey  string
	baseURL string
	client  *http.Client
}

// GoogleSTTProviderConfig holds configuration for the Google STT provider.
type GoogleSTTProviderConfig struct {
	APIKey  string
	BaseURL string // defaults to "https://speech.googleapis.com"
	Timeout time.Duration
}

// NewGoogleSTTProvider creates a GoogleSTTProvider with the given config.
func NewGoogleSTTProvider(cfg GoogleSTTProviderConfig) *GoogleSTTProvider {
	base := strings.TrimRight(cfg.BaseURL, "/")
	if base == "" {
		base = "https://speech.googleapis.com"
	}
	timeout := cfg.Timeout
	if timeout <= 0 {
		timeout = 30 * time.Second
	}
	return &GoogleSTTProvider{
		apiKey:  cfg.APIKey,
		baseURL: base,
		client:  &http.Client{Timeout: timeout},
	}
}

func (g *GoogleSTTProvider) Name() string { return "google_speech_v2" }

type googleSTTRequest struct {
	Config struct {
		LanguageCodes       []string `json:"languageCodes"`
		Model               string   `json:"model"`
		MaxAlternatives     int      `json:"maxAlternatives"`
		EnableWordTimeStamp bool     `json:"enableWordTimeOffsets"`
	} `json:"config"`
	URI string `json:"uri"`
}

type googleSTTResponse struct {
	Results []struct {
		Alternatives []struct {
			Transcript string  `json:"transcript"`
			Confidence float64 `json:"confidence"`
			Words      []struct {
				StartOffset string `json:"startOffset"`
				EndOffset   string `json:"endOffset"`
				Word        string `json:"word"`
			} `json:"words"`
		} `json:"alternatives"`
		ResultEndOffset string `json:"resultEndOffset"`
	} `json:"results"`
	TotalBilledDuration string `json:"totalBilledDuration"`
}

func (g *GoogleSTTProvider) Transcribe(ctx context.Context, audioURL string, opts STTOptions) (*TranscriptResult, error) {
	var reqPayload googleSTTRequest
	lang := opts.Language
	if lang == "" {
		lang = "en-US"
	}
	reqPayload.Config.LanguageCodes = []string{lang}
	reqPayload.Config.Model = "long"
	if opts.Model != "" {
		reqPayload.Config.Model = opts.Model
	}
	reqPayload.Config.MaxAlternatives = 1
	reqPayload.Config.EnableWordTimeStamp = true
	reqPayload.URI = audioURL

	bodyBytes, err := json.Marshal(reqPayload)
	if err != nil {
		return nil, fmt.Errorf("stt: marshal google request: %w", err)
	}

	endpoint := g.baseURL + "/v2/speech:recognize"
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(bodyBytes))
	if err != nil {
		return nil, fmt.Errorf("stt: build google request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Goog-Api-Key", g.apiKey)

	resp, err := g.client.Do(req)
	if err != nil {
		if ctx.Err() != nil {
			return nil, ErrSTTTimeout
		}
		return nil, fmt.Errorf("%w: %v", ErrSTTProviderError, err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("%w: read google response: %v", ErrSTTProviderError, err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("%w: google status %d: %s", ErrSTTProviderError, resp.StatusCode, string(respBody))
	}

	var gr googleSTTResponse
	if err := json.Unmarshal(respBody, &gr); err != nil {
		return nil, fmt.Errorf("%w: unmarshal google response: %v", ErrSTTProviderError, err)
	}

	var fullText strings.Builder
	var confidence float64
	var segments []TranscriptSegment
	altCount := 0

	for _, result := range gr.Results {
		if len(result.Alternatives) == 0 {
			continue
		}
		alt := result.Alternatives[0]
		if fullText.Len() > 0 {
			fullText.WriteString(" ")
		}
		fullText.WriteString(alt.Transcript)
		confidence += alt.Confidence
		altCount++

		for _, w := range alt.Words {
			segments = append(segments, TranscriptSegment{
				StartMs:    parseDurationMs(w.StartOffset),
				EndMs:      parseDurationMs(w.EndOffset),
				Text:       w.Word,
				Confidence: alt.Confidence,
			})
		}
	}

	if altCount > 0 {
		confidence /= float64(altCount)
	}

	return &TranscriptResult{
		Text:       fullText.String(),
		Confidence: confidence,
		Language:   lang,
		DurationMs: 0, // filled from TotalBilledDuration if available
		Segments:   segments,
	}, nil
}

// parseDurationMs converts a protobuf-style duration string like "1.500s" to milliseconds.
func parseDurationMs(s string) int64 {
	s = strings.TrimSuffix(s, "s")
	d, err := time.ParseDuration(s + "s")
	if err != nil {
		return 0
	}
	return d.Milliseconds()
}

// ---------------------------------------------------------------------------
// STTService — orchestrates primary/fallback provider chain
// ---------------------------------------------------------------------------

// STTService manages STT providers with primary/fallback logic.
type STTService struct {
	primary  STTProvider
	fallback STTProvider
	timeout  time.Duration
}

// STTServiceConfig holds configuration for the STT service.
type STTServiceConfig struct {
	Primary  STTProvider
	Fallback STTProvider
	Timeout  time.Duration // global timeout wrapping all attempts
}

// NewSTTService creates an STTService. At least a primary provider is required.
func NewSTTService(cfg STTServiceConfig) (*STTService, error) {
	if cfg.Primary == nil {
		return nil, ErrSTTNoProviders
	}
	timeout := cfg.Timeout
	if timeout <= 0 {
		timeout = 60 * time.Second
	}
	return &STTService{
		primary:  cfg.Primary,
		fallback: cfg.Fallback,
		timeout:  timeout,
	}, nil
}

// ValidateAudioURL performs basic validation on the audio URL.
func ValidateAudioURL(audioURL string) error {
	if strings.TrimSpace(audioURL) == "" {
		return fmt.Errorf("%w: empty URL", ErrSTTInvalidAudio)
	}
	u, err := url.Parse(audioURL)
	if err != nil {
		return fmt.Errorf("%w: %v", ErrSTTInvalidAudio, err)
	}
	if u.Scheme != "https" && u.Scheme != "http" {
		return fmt.Errorf("%w: unsupported scheme %q", ErrSTTInvalidAudio, u.Scheme)
	}
	if u.Host == "" {
		return fmt.Errorf("%w: missing host", ErrSTTInvalidAudio)
	}
	return nil
}

// Transcribe runs the audio URL through the primary provider, falling back to
// the secondary provider on error.
func (s *STTService) Transcribe(ctx context.Context, audioURL string, opts STTOptions) (*TranscriptResult, error) {
	if err := ValidateAudioURL(audioURL); err != nil {
		return nil, err
	}

	ctx, cancel := context.WithTimeout(ctx, s.timeout)
	defer cancel()

	result, err := s.primary.Transcribe(ctx, audioURL, opts)
	if err == nil {
		return result, nil
	}

	if s.fallback == nil {
		return nil, fmt.Errorf("stt: primary provider %s failed: %w", s.primary.Name(), err)
	}

	fallbackResult, fallbackErr := s.fallback.Transcribe(ctx, audioURL, opts)
	if fallbackErr != nil {
		return nil, fmt.Errorf("stt: all providers failed: primary=%v fallback=%v", err, fallbackErr)
	}
	return fallbackResult, nil
}
