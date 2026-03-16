package gateway

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var deepgramHappyResponse = `{
  "metadata": {"request_id": "abc123", "duration": 3.5, "channels": 1},
  "results": {
    "channels": [{
      "alternatives": [{
        "transcript": "Hello world this is a test.",
        "confidence": 0.98,
        "words": [
          {"word": "Hello",  "start": 0.0, "end": 0.4, "confidence": 0.99},
          {"word": "world",  "start": 0.4, "end": 0.8, "confidence": 0.98},
          {"word": "this",   "start": 0.9, "end": 1.1, "confidence": 0.97},
          {"word": "is",     "start": 1.1, "end": 1.3, "confidence": 0.99},
          {"word": "a",      "start": 1.3, "end": 1.4, "confidence": 0.99},
          {"word": "test",   "start": 1.5, "end": 2.1, "confidence": 0.96}
        ]
      }]
    }]
  }
}`

var deepgramDiarizeResponse = `{
  "metadata": {"request_id": "def456", "duration": 5.0, "channels": 1},
  "results": {
    "channels": [{
      "alternatives": [{
        "transcript": "Hello there how are you",
        "confidence": 0.95,
        "words": [
          {"word": "Hello",  "start": 0.0, "end": 0.4, "confidence": 0.99, "speaker": 0, "speaker_confidence": 0.92},
          {"word": "there",  "start": 0.4, "end": 0.8, "confidence": 0.98, "speaker": 0, "speaker_confidence": 0.91},
          {"word": "how",    "start": 1.0, "end": 1.3, "confidence": 0.97, "speaker": 1, "speaker_confidence": 0.88},
          {"word": "are",    "start": 1.3, "end": 1.5, "confidence": 0.98, "speaker": 1, "speaker_confidence": 0.89},
          {"word": "you",    "start": 1.5, "end": 1.8, "confidence": 0.99, "speaker": 1, "speaker_confidence": 0.90}
        ]
      }]
    }]
  }
}`

func newTestDeepgramProvider(t *testing.T, serverURL string) *DeepgramProvider {
	t.Helper()
	p, err := NewDeepgramProvider(DeepgramProviderConfig{
		APIKey:  "test-key",
		BaseURL: serverURL,
		Timeout: 5 * time.Second,
	})
	require.NoError(t, err)
	return p
}

func TestNewDeepgramProvider_EmptyAPIKey(t *testing.T) {
	_, err := NewDeepgramProvider(DeepgramProviderConfig{})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "APIKey is required")
}

func TestDeepgramProvider_Name(t *testing.T) {
	p, err := NewDeepgramProvider(DeepgramProviderConfig{APIKey: "k"})
	require.NoError(t, err)
	assert.Equal(t, "deepgram_nova3", p.Name())
}

func TestDeepgramProvider_Transcribe_InvalidURL(t *testing.T) {
	p, _ := NewDeepgramProvider(DeepgramProviderConfig{APIKey: "k"})
	_, err := p.Transcribe(context.Background(), "", STTOptions{})
	require.Error(t, err)
	assert.True(t, errors.Is(err, ErrSTTInvalidAudio))
}

func TestDeepgramProvider_Transcribe_401(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		w.Write([]byte(`{"err_msg":"Invalid credentials"}`))
	}))
	defer srv.Close()

	p := newTestDeepgramProvider(t, srv.URL)
	_, err := p.Transcribe(context.Background(), "https://example.com/audio.mp3", STTOptions{})
	require.Error(t, err)
	assert.True(t, errors.Is(err, ErrSTTProviderError))
	assert.Contains(t, err.Error(), "invalid API key")
}

func TestDeepgramProvider_Transcribe_Non200(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusServiceUnavailable)
		w.Write([]byte(`service unavailable`))
	}))
	defer srv.Close()

	p := newTestDeepgramProvider(t, srv.URL)
	_, err := p.Transcribe(context.Background(), "https://example.com/audio.mp3", STTOptions{})
	require.Error(t, err)
	assert.True(t, errors.Is(err, ErrSTTProviderError))
}

func TestDeepgramProvider_Transcribe_InvalidJSON(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{bad json`))
	}))
	defer srv.Close()

	p := newTestDeepgramProvider(t, srv.URL)
	_, err := p.Transcribe(context.Background(), "https://example.com/audio.mp3", STTOptions{})
	require.Error(t, err)
	assert.True(t, errors.Is(err, ErrSTTProviderError))
	assert.Contains(t, err.Error(), "unmarshal")
}

func TestDeepgramProvider_Transcribe_EmptyChannels(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"metadata":{},"results":{"channels":[]}}`))
	}))
	defer srv.Close()

	p := newTestDeepgramProvider(t, srv.URL)
	_, err := p.Transcribe(context.Background(), "https://example.com/audio.mp3", STTOptions{})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "no transcription results")
}

func TestDeepgramProvider_Transcribe_HappyPath(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodPost, r.Method)
		assert.Equal(t, "application/json", r.Header.Get("Content-Type"))
		assert.Contains(t, r.Header.Get("Authorization"), "Token ")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(deepgramHappyResponse))
	}))
	defer srv.Close()

	p := newTestDeepgramProvider(t, srv.URL)
	result, err := p.Transcribe(context.Background(), "https://example.com/audio.mp3", STTOptions{})
	require.NoError(t, err)
	assert.Equal(t, "Hello world this is a test.", result.Text)
	assert.InDelta(t, 0.98, result.Confidence, 0.01)
	assert.Len(t, result.Segments, 6)
	assert.Equal(t, int64(3500), result.DurationMs)
	// Verify no speaker labels when diarize is off.
	assert.Empty(t, result.Segments[0].Speaker)
}

func TestDeepgramProvider_Transcribe_WithDiarization(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(deepgramDiarizeResponse))
	}))
	defer srv.Close()

	p, err := NewDeepgramProvider(DeepgramProviderConfig{
		APIKey:  "test-key",
		BaseURL: srv.URL,
		Diarize: true,
	})
	require.NoError(t, err)

	result, err := p.Transcribe(context.Background(), "https://example.com/audio.mp3", STTOptions{})
	require.NoError(t, err)
	assert.Equal(t, "Speaker 1", result.Segments[0].Speaker)
	assert.Equal(t, "Speaker 2", result.Segments[2].Speaker)
}

func TestDeepgramProvider_Transcribe_ContextTimeout(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(2 * time.Second)
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(deepgramHappyResponse))
	}))
	defer srv.Close()

	p, err := NewDeepgramProvider(DeepgramProviderConfig{
		APIKey:  "test-key",
		BaseURL: srv.URL,
		Timeout: 5 * time.Second,
	})
	require.NoError(t, err)

	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()

	_, err = p.Transcribe(ctx, "https://example.com/audio.mp3", STTOptions{})
	require.Error(t, err)
	assert.True(t, errors.Is(err, ErrSTTTimeout))
}
