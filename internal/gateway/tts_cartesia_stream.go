package gateway

import (
	"bufio"
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
)

type sseChunk struct {
	Type    string `json:"type"`
	Data    string `json:"data,omitempty"`
	Message string `json:"message,omitempty"`
}

// SynthesizeStream calls POST /tts/sse and writes base64-decoded audio chunks to sink.
// sink is closed before the method returns.
func (c *CartesiaTTSProvider) SynthesizeStream(
	ctx context.Context,
	text string,
	opts TTSOptions,
	sink chan<- []byte,
) error {
	defer close(sink)

	text = strings.TrimSpace(text)
	if text == "" {
		return fmt.Errorf("%w: empty text", ErrTTSInvalidInput)
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

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/tts/sse", bytes.NewReader(payload))
	if err != nil {
		return fmt.Errorf("cartesia-stream: build request: %w", err)
	}
	req.Header.Set("X-API-Key", c.apiKey)
	req.Header.Set("Cartesia-Version", cartesiaVersion)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "text/event-stream")

	resp, err := c.client.Do(req)
	if err != nil {
		if ctx.Err() != nil {
			return fmt.Errorf("cartesia-stream: timed out: %w", ctx.Err())
		}
		return fmt.Errorf("%w: cartesia-stream: %v", ErrTTSProviderError, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		b, _ := io.ReadAll(io.LimitReader(resp.Body, 512))
		return fmt.Errorf("%w: cartesia-stream: status %d: %s", ErrTTSProviderError, resp.StatusCode, string(b))
	}

	scanner := bufio.NewScanner(resp.Body)
	for scanner.Scan() {
		line := scanner.Text()
		if !strings.HasPrefix(line, "data: ") {
			continue
		}
		jsonData := strings.TrimPrefix(line, "data: ")
		if jsonData == "[DONE]" {
			return nil
		}
		var chunk sseChunk
		if err := json.Unmarshal([]byte(jsonData), &chunk); err != nil {
			continue
		}
		switch chunk.Type {
		case "chunk":
			audio, err := base64.StdEncoding.DecodeString(chunk.Data)
			if err != nil {
				continue
			}
			select {
			case sink <- audio:
			case <-ctx.Done():
				return ctx.Err()
			}
		case "done":
			return nil
		case "error":
			return fmt.Errorf("%w: cartesia-stream: %s", ErrTTSProviderError, chunk.Message)
		}
	}
	if err := scanner.Err(); err != nil {
		return fmt.Errorf("cartesia-stream: scan error: %w", err)
	}
	return nil
}

// Ensure CartesiaTTSProvider satisfies StreamingTTSProvider.
var _ StreamingTTSProvider = (*CartesiaTTSProvider)(nil)
