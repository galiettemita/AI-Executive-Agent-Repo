package gateway

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"net/http"
	"strings"
	"time"
)

const (
	cartesiaBaseURL  = "https://api.cartesia.ai"
	cartesiaVersion  = "2024-06-10"
	cartesiaModel    = "sonic-2"
	cartesiaLanguage = "en"
)

// AudioUploader abstracts uploading audio bytes and returning a URL.
type AudioUploader interface {
	Upload(ctx context.Context, data []byte, contentType string) (string, error)
}

// CartesiaTTSProviderConfig holds configuration for the Cartesia TTS provider.
type CartesiaTTSProviderConfig struct {
	APIKey   string
	BaseURL  string        // defaults to https://api.cartesia.ai
	Model    string        // defaults to sonic-2
	Language string        // defaults to en
	Timeout  time.Duration // defaults to 30s
	VoiceMap map[string]string // maps friendly name → Cartesia voice ID
}

// CartesiaTTSProvider calls the Cartesia TTS API.
type CartesiaTTSProvider struct {
	apiKey   string
	baseURL  string
	model    string
	language string
	client   *http.Client
	voiceMap map[string]string
	uploader AudioUploader
}

// cartesiaFormat describes the output format for Cartesia requests.
type cartesiaFormat struct {
	container  string
	encoding   string
	sampleRate int
}

// NewCartesiaTTSProvider creates a CartesiaTTSProvider. APIKey and uploader must not be empty/nil.
func NewCartesiaTTSProvider(cfg CartesiaTTSProviderConfig, uploader AudioUploader) (*CartesiaTTSProvider, error) {
	if cfg.APIKey == "" {
		return nil, fmt.Errorf("cartesia: APIKey is required")
	}
	if uploader == nil {
		return nil, fmt.Errorf("cartesia: uploader is required")
	}
	base := strings.TrimRight(cfg.BaseURL, "/")
	if base == "" {
		base = cartesiaBaseURL
	}
	model := cfg.Model
	if model == "" {
		model = cartesiaModel
	}
	lang := cfg.Language
	if lang == "" {
		lang = cartesiaLanguage
	}
	timeout := cfg.Timeout
	if timeout <= 0 {
		timeout = 30 * time.Second
	}
	vm := cfg.VoiceMap
	if vm == nil {
		vm = map[string]string{
			"nova":  "a0e99841-438c-4a64-b679-ae501e7d6091",
			"alloy": "79a125e8-cd45-4c13-8a67-188112f4dd22",
		}
	}
	return &CartesiaTTSProvider{
		apiKey:   cfg.APIKey,
		baseURL:  base,
		model:    model,
		language: lang,
		client:   &http.Client{Timeout: timeout},
		voiceMap: vm,
		uploader: uploader,
	}, nil
}

func (c *CartesiaTTSProvider) Name() string { return "cartesia_sonic" }

func (c *CartesiaTTSProvider) resolveVoiceID(voice string) string {
	if id, ok := c.voiceMap[strings.ToLower(voice)]; ok {
		return id
	}
	// If it looks like a UUID, use as-is.
	if len(voice) == 36 && strings.Count(voice, "-") == 4 {
		return voice
	}
	return c.voiceMap["nova"]
}

func normalizeCartesiaFormat(format string) cartesiaFormat {
	switch strings.ToLower(strings.TrimSpace(format)) {
	case "wav":
		return cartesiaFormat{container: "wav", encoding: "pcm_s16le", sampleRate: 44100}
	case "raw", "pcm":
		return cartesiaFormat{container: "raw", encoding: "pcm_s16le", sampleRate: 24000}
	default: // mp3
		return cartesiaFormat{container: "mp3", encoding: "mp3", sampleRate: 44100}
	}
}

func (c *CartesiaTTSProvider) Synthesize(ctx context.Context, text string, opts TTSOptions) (*AudioResult, error) {
	text = strings.TrimSpace(text)
	if text == "" {
		return nil, fmt.Errorf("%w: empty text", ErrTTSInvalidInput)
	}

	voiceID := c.resolveVoiceID(opts.Voice)
	format := normalizeCartesiaFormat(opts.Format)

	reqBody := map[string]any{
		"model_id":   c.model,
		"transcript": text,
		"voice":      map[string]string{"mode": "id", "id": voiceID},
		"output_format": map[string]any{
			"container":   format.container,
			"encoding":    format.encoding,
			"sample_rate": format.sampleRate,
		},
		"language": c.language,
	}
	payload, _ := json.Marshal(reqBody)

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/tts/bytes", bytes.NewReader(payload))
	if err != nil {
		return nil, fmt.Errorf("cartesia: build request: %w", err)
	}
	req.Header.Set("X-API-Key", c.apiKey)
	req.Header.Set("Cartesia-Version", cartesiaVersion)
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.client.Do(req)
	if err != nil {
		if ctx.Err() != nil {
			return nil, fmt.Errorf("cartesia: timed out: %w", ctx.Err())
		}
		return nil, fmt.Errorf("%w: cartesia: %v", ErrTTSProviderError, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		b, _ := io.ReadAll(io.LimitReader(resp.Body, 512))
		return nil, fmt.Errorf("%w: cartesia: status %d: %s", ErrTTSProviderError, resp.StatusCode, string(b))
	}

	audioBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("%w: cartesia: read body: %v", ErrTTSProviderError, err)
	}
	if len(audioBytes) == 0 {
		return nil, fmt.Errorf("%w: cartesia: empty audio response", ErrTTSProviderError)
	}

	contentType := "audio/mpeg"
	if format.container == "wav" {
		contentType = "audio/wav"
	}
	audioURL, err := c.uploader.Upload(ctx, audioBytes, contentType)
	if err != nil {
		return nil, fmt.Errorf("cartesia: upload failed: %w", err)
	}

	durationMs := estimateDurationMs(len(audioBytes), format)

	return &AudioResult{
		AudioURL:   audioURL,
		Format:     format.container,
		DurationMs: durationMs,
		SizeBytes:  int64(len(audioBytes)),
	}, nil
}

func estimateDurationMs(sizeBytes int, format cartesiaFormat) int64 {
	switch format.container {
	case "wav":
		bytesPerSec := float64(format.sampleRate) * 2 // 16-bit PCM
		return int64(math.Round(float64(sizeBytes) / bytesPerSec * 1000))
	default: // mp3
		return int64(math.Round(float64(sizeBytes) / 3.0)) // ~24kbps
	}
}

func clampF(v, lo, hi float64) float64 {
	if v < lo {
		return lo
	}
	if v > hi {
		return hi
	}
	return v
}

// StreamingTTSProvider extends TTSProvider with chunk-level streaming synthesis.
type StreamingTTSProvider interface {
	TTSProvider
	// SynthesizeStream writes audio chunks to sink as synthesis progresses.
	// sink is always closed before this method returns.
	SynthesizeStream(ctx context.Context, text string, opts TTSOptions, sink chan<- []byte) error
}
