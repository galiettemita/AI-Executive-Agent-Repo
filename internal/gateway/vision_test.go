package gateway

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func geminiEnvelope(jsonBody string) string {
	env := map[string]any{
		"candidates": []map[string]any{{
			"content": map[string]any{
				"parts": []map[string]any{{"text": jsonBody}},
			},
		}},
	}
	b, _ := json.Marshal(env)
	return string(b)
}

func newTestVisionProvider(t *testing.T, handler http.HandlerFunc) *GeminiVisionProvider {
	t.Helper()
	srv := httptest.NewServer(handler)
	t.Cleanup(srv.Close)
	p, err := NewGeminiVisionProvider(GeminiVisionProviderConfig{
		APIKey: "test-key", BaseURL: srv.URL,
	})
	require.NoError(t, err)
	return p
}

func TestNewGeminiVisionProvider_EmptyAPIKey(t *testing.T) {
	_, err := NewGeminiVisionProvider(GeminiVisionProviderConfig{})
	require.Error(t, err)
}

func TestGeminiVisionProvider_Name(t *testing.T) {
	p, _ := NewGeminiVisionProvider(GeminiVisionProviderConfig{APIKey: "k"})
	assert.Equal(t, "gemini_vision", p.Name())
}

func TestGeminiVisionProvider_EmptyImage(t *testing.T) {
	p, _ := NewGeminiVisionProvider(GeminiVisionProviderConfig{APIKey: "k"})
	_, err := p.Analyse(context.Background(), VisionInput{MIMEType: "image/jpeg"})
	assert.ErrorIs(t, err, ErrVisionEmptyImage)
}

func TestGeminiVisionProvider_TooLarge(t *testing.T) {
	p, _ := NewGeminiVisionProvider(GeminiVisionProviderConfig{APIKey: "k"})
	_, err := p.Analyse(context.Background(), VisionInput{
		ImageData: make([]byte, MaxVisionImageBytes+1), MIMEType: "image/jpeg",
	})
	assert.ErrorIs(t, err, ErrVisionImageTooLarge)
}

func TestGeminiVisionProvider_UnsupportedMIME(t *testing.T) {
	p, _ := NewGeminiVisionProvider(GeminiVisionProviderConfig{APIKey: "k"})
	_, err := p.Analyse(context.Background(), VisionInput{
		ImageData: []byte{0xFF}, MIMEType: "image/bmp",
	})
	assert.ErrorIs(t, err, ErrVisionUnsupported)
}

func TestGeminiVisionProvider_401(t *testing.T) {
	p := newTestVisionProvider(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(401)
	})
	_, err := p.Analyse(context.Background(), VisionInput{ImageData: []byte{0xFF}, MIMEType: "image/jpeg"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "invalid API key")
}

func TestGeminiVisionProvider_Non200(t *testing.T) {
	p := newTestVisionProvider(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(503)
	})
	_, err := p.Analyse(context.Background(), VisionInput{ImageData: []byte{0xFF}, MIMEType: "image/jpeg"})
	assert.ErrorIs(t, err, ErrVisionProviderError)
}

func TestGeminiVisionProvider_EmptyCandidates(t *testing.T) {
	p := newTestVisionProvider(t, func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, `{"candidates":[]}`)
	})
	_, err := p.Analyse(context.Background(), VisionInput{ImageData: []byte{0xFF}, MIMEType: "image/jpeg"})
	assert.ErrorIs(t, err, ErrVisionProviderError)
}

func TestGeminiVisionProvider_HappyPath(t *testing.T) {
	vr := map[string]any{
		"extracted_text":  "Invoice #123",
		"summary":         "A scanned invoice document.",
		"data_points":     []string{"Total: $500"},
		"tags":            []string{"invoice", "finance"},
		"confidence_hint": 0.95,
	}
	vrJSON, _ := json.Marshal(vr)
	p := newTestVisionProvider(t, func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, geminiEnvelope(string(vrJSON)))
	})
	result, err := p.Analyse(context.Background(), VisionInput{
		ImageData: []byte{0xFF, 0xD8, 0xFF}, MIMEType: "image/jpeg",
	})
	require.NoError(t, err)
	assert.Equal(t, "gemini_vision", result.Provider)
	assert.NotEmpty(t, result.Summary)
	assert.NotNil(t, result.DataPoints)
	assert.NotNil(t, result.Tags)
}

func TestGeminiVisionProvider_NonJSONFallback(t *testing.T) {
	p := newTestVisionProvider(t, func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, geminiEnvelope("This is a receipt for office supplies."))
	})
	result, err := p.Analyse(context.Background(), VisionInput{
		ImageData: []byte{0xFF}, MIMEType: "image/jpeg",
	})
	require.NoError(t, err)
	assert.Contains(t, result.Summary, "receipt")
}

func TestGeminiVisionProvider_ContextTimeout(t *testing.T) {
	block := make(chan struct{})
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		<-block
	}))
	defer func() {
		close(block)
		srv.Close()
	}()

	p, err := NewGeminiVisionProvider(GeminiVisionProviderConfig{
		APIKey: "test-key", BaseURL: srv.URL,
	})
	require.NoError(t, err)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Millisecond)
	defer cancel()
	_, err = p.Analyse(ctx, VisionInput{ImageData: []byte{0xFF}, MIMEType: "image/jpeg"})
	require.Error(t, err)
}

func TestShouldRouteToVision_Image(t *testing.T) {
	assert.True(t, ShouldRouteToVision("image/jpeg"))
	assert.True(t, ShouldRouteToVision("image/png"))
}

func TestShouldRouteToVision_Audio(t *testing.T) {
	assert.False(t, ShouldRouteToVision("audio/ogg"))
}

func TestMaxAttachmentBytesForMime_Image20MB(t *testing.T) {
	assert.Equal(t, int64(20*1024*1024), maxAttachmentBytesForMime("image/png"))
}
