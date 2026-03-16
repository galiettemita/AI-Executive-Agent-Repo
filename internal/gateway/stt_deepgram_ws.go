package gateway

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/gorilla/websocket"
)

// DeepgramWSConfig configures the WebSocket streaming STT client.
type DeepgramWSConfig struct {
	APIKey          string
	Language        string        // BCP-47; default "en-US"
	Model           string        // default "nova-3"
	Diarize         bool
	SmartFormat     bool
	SampleRate      int           // default 16000
	Channels        int           // default 1
	Encoding        string        // default "linear16"
	InterimResults  bool          // emit interim transcripts; default true
	EndpointingMs   int           // silence ms before final result; default 500
	ConnectTimeout  time.Duration // default 10s
	// BaseURLOverride is used only in tests to point at a local httptest.Server.
	// Leave empty for production (uses wss://api.deepgram.com).
	BaseURLOverride string
}

// deepgramWSResult is a parsed transcript message from Deepgram.
type deepgramWSResult struct {
	Type    string `json:"type"`
	IsFinal bool   `json:"is_final"`
	Channel struct {
		Alternatives []struct {
			Transcript string  `json:"transcript"`
			Confidence float64 `json:"confidence"`
			Words      []struct {
				Word    string  `json:"word"`
				Start   float64 `json:"start"`
				End     float64 `json:"end"`
				Speaker *int    `json:"speaker,omitempty"`
			} `json:"words"`
		} `json:"alternatives"`
	} `json:"channel"`
}

// DeepgramWSClient streams raw PCM audio frames to Deepgram and emits TranscriptEvents.
type DeepgramWSClient struct {
	cfg  DeepgramWSConfig
	conn *websocket.Conn
	mu   sync.Mutex
}

// NewDeepgramWSClient creates a DeepgramWSClient (does not connect).
func NewDeepgramWSClient(cfg DeepgramWSConfig) (*DeepgramWSClient, error) {
	if cfg.APIKey == "" {
		return nil, fmt.Errorf("deepgram ws: APIKey is required")
	}
	if cfg.Language == "" {
		cfg.Language = "en-US"
	}
	if cfg.Model == "" {
		cfg.Model = "nova-3"
	}
	if cfg.SampleRate <= 0 {
		cfg.SampleRate = 16000
	}
	if cfg.Channels <= 0 {
		cfg.Channels = 1
	}
	if cfg.Encoding == "" {
		cfg.Encoding = "linear16"
	}
	if cfg.EndpointingMs <= 0 {
		cfg.EndpointingMs = 500
	}
	if cfg.ConnectTimeout <= 0 {
		cfg.ConnectTimeout = 10 * time.Second
	}
	return &DeepgramWSClient{cfg: cfg}, nil
}

func (d *DeepgramWSClient) buildURL() string {
	base := "wss://api.deepgram.com/v1/listen"
	if d.cfg.BaseURLOverride != "" {
		base = strings.Replace(d.cfg.BaseURLOverride, "http://", "ws://", 1)
		base = strings.Replace(base, "https://", "wss://", 1)
		base = strings.TrimRight(base, "/") + "/v1/listen"
	}
	params := url.Values{}
	params.Set("model", d.cfg.Model)
	params.Set("language", d.cfg.Language)
	params.Set("encoding", d.cfg.Encoding)
	params.Set("sample_rate", fmt.Sprintf("%d", d.cfg.SampleRate))
	params.Set("channels", fmt.Sprintf("%d", d.cfg.Channels))
	params.Set("punctuate", "true")
	params.Set("words", "true")
	params.Set("endpointing", fmt.Sprintf("%d", d.cfg.EndpointingMs))
	if d.cfg.InterimResults {
		params.Set("interim_results", "true")
	}
	if d.cfg.Diarize {
		params.Set("diarize", "true")
		params.Set("diarize_version", "3")
	}
	if d.cfg.SmartFormat {
		params.Set("smart_format", "true")
	}
	return base + "?" + params.Encode()
}

// Connect establishes the WebSocket connection to Deepgram.
func (d *DeepgramWSClient) Connect(ctx context.Context) error {
	wsURL := d.buildURL()
	dialer := websocket.Dialer{HandshakeTimeout: d.cfg.ConnectTimeout}
	headers := http.Header{}
	headers.Set("Authorization", "Token "+d.cfg.APIKey)

	conn, resp, err := dialer.DialContext(ctx, wsURL, headers)
	if err != nil {
		if resp != nil {
			return fmt.Errorf("deepgram ws: connect failed HTTP %d: %w", resp.StatusCode, err)
		}
		return fmt.Errorf("deepgram ws: connect failed: %w", err)
	}
	d.mu.Lock()
	d.conn = conn
	d.mu.Unlock()
	return nil
}

// SendAudioFrame sends a binary PCM audio frame to Deepgram.
func (d *DeepgramWSClient) SendAudioFrame(frame []byte) error {
	d.mu.Lock()
	conn := d.conn
	d.mu.Unlock()
	if conn == nil {
		return fmt.Errorf("deepgram ws: not connected")
	}
	return conn.WriteMessage(websocket.BinaryMessage, frame)
}

// CloseStream sends the Deepgram close signal.
func (d *DeepgramWSClient) CloseStream() error {
	d.mu.Lock()
	conn := d.conn
	d.mu.Unlock()
	if conn == nil {
		return nil
	}
	return conn.WriteMessage(websocket.TextMessage, []byte(`{"type":"CloseStream"}`))
}

// Close closes the underlying WebSocket.
func (d *DeepgramWSClient) Close() error {
	d.mu.Lock()
	conn := d.conn
	d.conn = nil
	d.mu.Unlock()
	if conn == nil {
		return nil
	}
	return conn.Close()
}

// StreamTranscripts reads messages from Deepgram and emits TranscriptEvents.
// Blocks until the connection closes, ctx is cancelled, or an error occurs.
func (d *DeepgramWSClient) StreamTranscripts(ctx context.Context, events chan<- TranscriptEvent) error {
	d.mu.Lock()
	conn := d.conn
	d.mu.Unlock()
	if conn == nil {
		return fmt.Errorf("deepgram ws: not connected")
	}

	readErr := make(chan error, 1)
	go func() {
		for {
			_, msg, err := conn.ReadMessage()
			if err != nil {
				readErr <- fmt.Errorf("deepgram ws: read: %w", err)
				return
			}
			var result deepgramWSResult
			if err := json.Unmarshal(msg, &result); err != nil {
				continue
			}
			if result.Type != "Results" {
				continue
			}
			if len(result.Channel.Alternatives) == 0 {
				continue
			}
			alt := result.Channel.Alternatives[0]
			if strings.TrimSpace(alt.Transcript) == "" {
				continue
			}
			event := TranscriptEvent{
				Text:       alt.Transcript,
				IsFinal:    result.IsFinal,
				Confidence: alt.Confidence,
			}
			if len(alt.Words) > 0 && alt.Words[0].Speaker != nil {
				event.Speaker = fmt.Sprintf("Speaker %d", *alt.Words[0].Speaker+1)
			}
			select {
			case events <- event:
			case <-ctx.Done():
				readErr <- ctx.Err()
				return
			}
		}
	}()

	select {
	case err := <-readErr:
		return err
	case <-ctx.Done():
		_ = conn.Close()
		return ctx.Err()
	}
}
