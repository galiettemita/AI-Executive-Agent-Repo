package gateway

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestOpenAITTSProvider_Synthesize(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/audio/speech" {
			http.Error(w, "not found", http.StatusNotFound)
			return
		}
		if r.Header.Get("Authorization") != "Bearer test-key" {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}

		var req openAITTSRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "bad request", http.StatusBadRequest)
			return
		}
		if req.Input == "" {
			http.Error(w, "empty input", http.StatusBadRequest)
			return
		}

		resp := openAITTSResponse{
			AudioURL:   "https://cdn.example.com/speech-123.mp3",
			Format:     req.ResponseFormat,
			DurationMs: 3200,
			SizeBytes:  51200,
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}))
	defer srv.Close()

	provider := NewOpenAITTSProvider(OpenAITTSProviderConfig{
		APIKey:  "test-key",
		BaseURL: srv.URL,
	})

	result, err := provider.Synthesize(context.Background(), "Hello world", TTSOptions{
		Voice:  "nova",
		Speed:  1.0,
		Format: "mp3",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.AudioURL != "https://cdn.example.com/speech-123.mp3" {
		t.Errorf("got audio_url=%q want=%q", result.AudioURL, "https://cdn.example.com/speech-123.mp3")
	}
	if result.Format != "mp3" {
		t.Errorf("got format=%q want=%q", result.Format, "mp3")
	}
	if result.DurationMs != 3200 {
		t.Errorf("got duration_ms=%d want=%d", result.DurationMs, 3200)
	}
	if result.SizeBytes != 51200 {
		t.Errorf("got size_bytes=%d want=%d", result.SizeBytes, 51200)
	}
}

func TestOpenAITTSProvider_EmptyText(t *testing.T) {
	t.Parallel()

	provider := NewOpenAITTSProvider(OpenAITTSProviderConfig{
		APIKey:  "test-key",
		BaseURL: "http://localhost",
	})

	_, err := provider.Synthesize(context.Background(), "", TTSOptions{})
	if err == nil {
		t.Fatal("expected error for empty text")
	}
}

func TestOpenAITTSProvider_ServerError(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		http.Error(w, "internal error", http.StatusInternalServerError)
	}))
	defer srv.Close()

	provider := NewOpenAITTSProvider(OpenAITTSProviderConfig{
		APIKey:  "test-key",
		BaseURL: srv.URL,
	})

	_, err := provider.Synthesize(context.Background(), "Hello", TTSOptions{})
	if err == nil {
		t.Fatal("expected error for 500 response")
	}
}

func TestTTSService_Synthesize(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		resp := openAITTSResponse{
			AudioURL:   "https://cdn.example.com/out.opus",
			Format:     "opus",
			DurationMs: 1500,
			SizeBytes:  24000,
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}))
	defer srv.Close()

	provider := NewOpenAITTSProvider(OpenAITTSProviderConfig{
		APIKey:  "k",
		BaseURL: srv.URL,
	})

	svc, err := NewTTSService(TTSServiceConfig{Provider: provider})
	if err != nil {
		t.Fatalf("unexpected error creating service: %v", err)
	}

	result, err := svc.Synthesize(context.Background(), "Test", TTSOptions{Format: "opus"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Format != "opus" {
		t.Errorf("got format=%q want=%q", result.Format, "opus")
	}
}

func TestTTSService_EmptyText(t *testing.T) {
	t.Parallel()

	provider := NewOpenAITTSProvider(OpenAITTSProviderConfig{APIKey: "k", BaseURL: "http://localhost"})
	svc, _ := NewTTSService(TTSServiceConfig{Provider: provider})

	_, err := svc.Synthesize(context.Background(), "", TTSOptions{})
	if err == nil {
		t.Fatal("expected error for empty text")
	}
}

func TestTTSService_ExceedsMaxChars(t *testing.T) {
	t.Parallel()

	provider := NewOpenAITTSProvider(OpenAITTSProviderConfig{APIKey: "k", BaseURL: "http://localhost"})
	svc, _ := NewTTSService(TTSServiceConfig{Provider: provider, MaxChars: 10})

	_, err := svc.Synthesize(context.Background(), "This text is definitely longer than ten characters", TTSOptions{})
	if err == nil {
		t.Fatal("expected error for text exceeding max chars")
	}
}

func TestNewTTSService_NoProvider(t *testing.T) {
	t.Parallel()

	_, err := NewTTSService(TTSServiceConfig{})
	if err == nil {
		t.Fatal("expected error when no provider configured")
	}
}

func TestNormalizeAudioFormat(t *testing.T) {
	t.Parallel()

	tests := []struct {
		input string
		want  string
	}{
		{"mp3", "mp3"},
		{"MP3", "mp3"},
		{"opus", "opus"},
		{"ogg/opus", "opus"},
		{"wav", "wav"},
		{"flac", "flac"},
		{"aac", "aac"},
		{"m4a", "aac"},
		{"", "mp3"},
		{"unknown", "mp3"},
	}
	for _, tt := range tests {
		got := normalizeAudioFormat(tt.input)
		if got != tt.want {
			t.Errorf("normalizeAudioFormat(%q)=%q want=%q", tt.input, got, tt.want)
		}
	}
}

func TestOpenAITTSProvider_DefaultVoice(t *testing.T) {
	t.Parallel()

	var capturedVoice string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req openAITTSRequest
		json.NewDecoder(r.Body).Decode(&req)
		capturedVoice = req.Voice
		resp := openAITTSResponse{AudioURL: "https://cdn.example.com/out.mp3", Format: "mp3"}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}))
	defer srv.Close()

	provider := NewOpenAITTSProvider(OpenAITTSProviderConfig{
		APIKey:  "k",
		BaseURL: srv.URL,
	})

	_, err := provider.Synthesize(context.Background(), "Hello", TTSOptions{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	defaultVoice := DefaultVoicePipelineConfig().DefaultVoice
	if capturedVoice != defaultVoice {
		t.Errorf("got voice=%q want default=%q", capturedVoice, defaultVoice)
	}
}

func TestOpenAITTSProvider_OpusFormat(t *testing.T) {
	t.Parallel()

	var capturedFormat string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req openAITTSRequest
		json.NewDecoder(r.Body).Decode(&req)
		capturedFormat = req.ResponseFormat
		resp := openAITTSResponse{AudioURL: "https://cdn.example.com/out.opus", Format: "opus"}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}))
	defer srv.Close()

	provider := NewOpenAITTSProvider(OpenAITTSProviderConfig{APIKey: "k", BaseURL: srv.URL})

	_, err := provider.Synthesize(context.Background(), "Hello", TTSOptions{Format: "ogg/opus"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if capturedFormat != "opus" {
		t.Errorf("got format=%q want=%q", capturedFormat, "opus")
	}
}

func TestTTSService_WhitespaceOnlyText(t *testing.T) {
	t.Parallel()

	provider := NewOpenAITTSProvider(OpenAITTSProviderConfig{APIKey: "k", BaseURL: "http://localhost"})
	svc, _ := NewTTSService(TTSServiceConfig{Provider: provider})

	_, err := svc.Synthesize(context.Background(), "   \n\t  ", TTSOptions{})
	if err == nil {
		t.Fatal("expected error for whitespace-only text")
	}
	if !strings.Contains(err.Error(), "empty text") {
		t.Errorf("expected 'empty text' in error, got: %v", err)
	}
}
