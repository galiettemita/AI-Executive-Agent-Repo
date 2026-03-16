package gateway

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// mockUploader is used by both tts_cartesia_test.go and voice_stream_test.go.
type mockUploader struct {
	returnURL string
	err       error
}

func (m *mockUploader) Upload(_ context.Context, _ []byte, _ string) (string, error) {
	return m.returnURL, m.err
}

func TestNewCartesiaTTSProvider_EmptyAPIKey(t *testing.T) {
	_, err := NewCartesiaTTSProvider(CartesiaTTSProviderConfig{}, &mockUploader{})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "APIKey is required")
}

func TestNewCartesiaTTSProvider_NilUploader(t *testing.T) {
	_, err := NewCartesiaTTSProvider(CartesiaTTSProviderConfig{APIKey: "k"}, nil)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "uploader is required")
}

func TestCartesiaTTSProvider_Name(t *testing.T) {
	up := &mockUploader{returnURL: "https://s3.example.com/test.mp3"}
	p, err := NewCartesiaTTSProvider(CartesiaTTSProviderConfig{APIKey: "k"}, up)
	require.NoError(t, err)
	assert.Equal(t, "cartesia_sonic", p.Name())
}

func TestCartesiaTTSProvider_Synthesize_HappyPath(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/tts/bytes", r.URL.Path)
		assert.Equal(t, "test-key", r.Header.Get("X-API-Key"))

		var body map[string]any
		json.NewDecoder(r.Body).Decode(&body)
		assert.Equal(t, "Hello world", body["transcript"])

		w.WriteHeader(200)
		w.Write(make([]byte, 9000)) // fake audio
	}))
	defer srv.Close()

	up := &mockUploader{returnURL: "https://s3.example.com/out.mp3"}
	p, err := NewCartesiaTTSProvider(CartesiaTTSProviderConfig{
		APIKey: "test-key", BaseURL: srv.URL,
	}, up)
	require.NoError(t, err)

	result, err := p.Synthesize(context.Background(), "Hello world", TTSOptions{})
	require.NoError(t, err)
	assert.Equal(t, "https://s3.example.com/out.mp3", result.AudioURL)
	assert.True(t, result.DurationMs > 0)
}

func TestCartesiaTTSProvider_Synthesize_EmptyText(t *testing.T) {
	up := &mockUploader{returnURL: "https://s3.example.com/test.mp3"}
	p, _ := NewCartesiaTTSProvider(CartesiaTTSProviderConfig{
		APIKey: "k", BaseURL: "http://localhost",
	}, up)
	_, err := p.Synthesize(context.Background(), "  ", TTSOptions{})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "empty text")
}
