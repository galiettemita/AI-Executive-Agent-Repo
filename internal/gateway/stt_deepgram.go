package gateway

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// Compile-time check that DeepgramProvider satisfies STTProvider.
var _ STTProvider = (*DeepgramProvider)(nil)

const (
	deepgramBaseURL        = "https://api.deepgram.com"
	deepgramDefaultModel   = "nova-3"
	deepgramDefaultLang    = "en-US"
	deepgramRequestTimeout = 30 * time.Second
)

// DeepgramProviderConfig holds configuration for the Deepgram STT provider.
type DeepgramProviderConfig struct {
	APIKey      string
	BaseURL     string        // defaults to https://api.deepgram.com
	Model       string        // defaults to nova-3
	Timeout     time.Duration // defaults to 30s
	Diarize     bool          // enable speaker diarization
	SmartFormat bool          // enable smart punctuation/formatting
}

// DeepgramProvider calls the Deepgram REST transcription API.
type DeepgramProvider struct {
	apiKey      string
	baseURL     string
	model       string
	timeout     time.Duration
	diarize     bool
	smartFormat bool
	client      *http.Client
}

// NewDeepgramProvider creates a DeepgramProvider. APIKey must not be empty.
func NewDeepgramProvider(cfg DeepgramProviderConfig) (*DeepgramProvider, error) {
	if cfg.APIKey == "" {
		return nil, fmt.Errorf("deepgram: APIKey is required")
	}
	base := strings.TrimRight(cfg.BaseURL, "/")
	if base == "" {
		base = deepgramBaseURL
	}
	model := cfg.Model
	if model == "" {
		model = deepgramDefaultModel
	}
	timeout := cfg.Timeout
	if timeout <= 0 {
		timeout = deepgramRequestTimeout
	}
	return &DeepgramProvider{
		apiKey:      cfg.APIKey,
		baseURL:     base,
		model:       model,
		timeout:     timeout,
		diarize:     cfg.Diarize,
		smartFormat: cfg.SmartFormat,
		client:      &http.Client{Timeout: timeout},
	}, nil
}

func (d *DeepgramProvider) Name() string { return "deepgram_nova3" }

func (d *DeepgramProvider) Transcribe(ctx context.Context, audioURL string, opts STTOptions) (*TranscriptResult, error) {
	if err := ValidateAudioURL(audioURL); err != nil {
		return nil, err
	}

	// Build query parameters.
	params := url.Values{}
	params.Set("model", d.resolveModel(opts.Model))
	lang := opts.Language
	if lang == "" {
		lang = deepgramDefaultLang
	}
	params.Set("language", lang)
	params.Set("punctuate", "true")
	params.Set("utterances", "false")
	params.Set("paragraphs", "false")
	params.Set("words", "true")
	if d.diarize || (opts.Diarization != nil && opts.Diarization.Enabled) {
		params.Set("diarize", "true")
		params.Set("diarize_version", "3")
		if opts.Diarization != nil && opts.Diarization.MaxSpeakers > 0 {
			params.Set("max_speakers", fmt.Sprintf("%d", opts.Diarization.MaxSpeakers))
		}
	}
	if d.smartFormat {
		params.Set("smart_format", "true")
	}

	endpoint := d.baseURL + "/v1/listen?" + params.Encode()

	// Body: JSON with audio URL.
	reqBody, err := json.Marshal(map[string]string{"url": audioURL})
	if err != nil {
		return nil, fmt.Errorf("deepgram: marshal request body: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(reqBody))
	if err != nil {
		return nil, fmt.Errorf("deepgram: build request: %w", err)
	}
	req.Header.Set("Authorization", "Token "+d.apiKey)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")

	resp, err := d.client.Do(req)
	if err != nil {
		if ctx.Err() != nil {
			return nil, ErrSTTTimeout
		}
		return nil, fmt.Errorf("%w: %v", ErrSTTProviderError, err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("%w: read body: %v", ErrSTTProviderError, err)
	}

	if resp.StatusCode == http.StatusUnauthorized {
		return nil, fmt.Errorf("%w: deepgram: invalid API key (401)", ErrSTTProviderError)
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("%w: deepgram: status %d: %s", ErrSTTProviderError, resp.StatusCode, truncate(string(body), 300))
	}

	return d.parseResponse(body, lang)
}

// deepgramResponse is the top-level response from Deepgram's /v1/listen endpoint.
type deepgramResponse struct {
	Metadata deepgramMetadata `json:"metadata"`
	Results  deepgramResults  `json:"results"`
}

type deepgramMetadata struct {
	RequestID string  `json:"request_id"`
	Duration  float64 `json:"duration"`
	Channels  int     `json:"channels"`
}

type deepgramResults struct {
	Channels []deepgramChannel `json:"channels"`
}

type deepgramChannel struct {
	Alternatives []deepgramAlternative `json:"alternatives"`
}

type deepgramAlternative struct {
	Transcript string         `json:"transcript"`
	Confidence float64        `json:"confidence"`
	Words      []deepgramWord `json:"words"`
}

type deepgramWord struct {
	Word              string  `json:"word"`
	Start             float64 `json:"start"`
	End               float64 `json:"end"`
	Confidence        float64 `json:"confidence"`
	Speaker           *int    `json:"speaker,omitempty"`
	SpeakerConfidence float64 `json:"speaker_confidence,omitempty"`
}

func (d *DeepgramProvider) parseResponse(body []byte, lang string) (*TranscriptResult, error) {
	var dr deepgramResponse
	if err := json.Unmarshal(body, &dr); err != nil {
		return nil, fmt.Errorf("%w: unmarshal deepgram response: %v", ErrSTTProviderError, err)
	}

	if len(dr.Results.Channels) == 0 || len(dr.Results.Channels[0].Alternatives) == 0 {
		return nil, fmt.Errorf("%w: deepgram: no transcription results in response", ErrSTTProviderError)
	}

	alt := dr.Results.Channels[0].Alternatives[0]

	// Build segments from per-word results.
	segments := make([]TranscriptSegment, 0, len(alt.Words))
	for _, w := range alt.Words {
		segments = append(segments, TranscriptSegment{
			StartMs:    int64(w.Start * 1000),
			EndMs:      int64(w.End * 1000),
			Text:       w.Word,
			Confidence: w.Confidence,
			Speaker:    speakerLabel(w.Speaker),
		})
	}

	confidence := alt.Confidence
	if confidence == 0 && len(segments) > 0 {
		// Average per-word confidence if overall is missing.
		var sum float64
		for _, s := range segments {
			sum += s.Confidence
		}
		confidence = sum / float64(len(segments))
	}

	return &TranscriptResult{
		Text:       alt.Transcript,
		Confidence: confidence,
		Language:   lang,
		DurationMs: int64(dr.Metadata.Duration * 1000),
		Segments:   segments,
	}, nil
}

func speakerLabel(sp *int) string {
	if sp == nil {
		return ""
	}
	return fmt.Sprintf("Speaker %d", *sp+1)
}

func (d *DeepgramProvider) resolveModel(override string) string {
	if override != "" {
		return override
	}
	return d.model
}

// truncate returns s truncated to n bytes with "..." suffix if truncated.
func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "..."
}
