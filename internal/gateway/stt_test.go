package gateway

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestValidateAudioURL(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		url     string
		wantErr bool
	}{
		{name: "valid https", url: "https://storage.example.com/audio.ogg", wantErr: false},
		{name: "valid http", url: "http://localhost:9000/audio.wav", wantErr: false},
		{name: "empty", url: "", wantErr: true},
		{name: "whitespace only", url: "   ", wantErr: true},
		{name: "ftp scheme", url: "ftp://files.example.com/audio.mp3", wantErr: true},
		{name: "no scheme", url: "storage.example.com/audio.ogg", wantErr: true},
		{name: "no host", url: "https:///audio.ogg", wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			err := ValidateAudioURL(tt.url)
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidateAudioURL(%q) err=%v wantErr=%v", tt.url, err, tt.wantErr)
			}
		})
	}
}

func TestWhisperProvider_Transcribe(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/audio/transcriptions" {
			http.Error(w, "not found", http.StatusNotFound)
			return
		}
		if r.Header.Get("Authorization") != "Bearer test-key" {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}

		resp := whisperResponse{
			Text:     "Hello world",
			Language: "en",
			Duration: 2.5,
			Segments: []struct {
				Start float64 `json:"start"`
				End   float64 `json:"end"`
				Text  string  `json:"text"`
			}{
				{Start: 0, End: 1.2, Text: "Hello"},
				{Start: 1.2, End: 2.5, Text: "world"},
			},
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}))
	defer srv.Close()

	provider := NewWhisperProvider(WhisperProviderConfig{
		APIKey:  "test-key",
		BaseURL: srv.URL,
	})

	result, err := provider.Transcribe(context.Background(), "https://example.com/audio.ogg", STTOptions{Language: "en"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Text != "Hello world" {
		t.Errorf("got text=%q want=%q", result.Text, "Hello world")
	}
	if result.Language != "en" {
		t.Errorf("got language=%q want=%q", result.Language, "en")
	}
	if result.DurationMs != 2500 {
		t.Errorf("got duration_ms=%d want=%d", result.DurationMs, 2500)
	}
	if len(result.Segments) != 2 {
		t.Fatalf("got %d segments want 2", len(result.Segments))
	}
	if result.Segments[0].Text != "Hello" {
		t.Errorf("segment[0].Text=%q want=%q", result.Segments[0].Text, "Hello")
	}
}

func TestWhisperProvider_TranscribeError(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		http.Error(w, `{"error":"rate limited"}`, http.StatusTooManyRequests)
	}))
	defer srv.Close()

	provider := NewWhisperProvider(WhisperProviderConfig{
		APIKey:  "test-key",
		BaseURL: srv.URL,
	})

	_, err := provider.Transcribe(context.Background(), "https://example.com/audio.ogg", STTOptions{})
	if err == nil {
		t.Fatal("expected error for 429 response")
	}
}

func TestGoogleSTTProvider_Transcribe(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v2/speech:recognize" {
			http.Error(w, "not found", http.StatusNotFound)
			return
		}

		resp := googleSTTResponse{
			Results: []struct {
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
			}{
				{
					Alternatives: []struct {
						Transcript string  `json:"transcript"`
						Confidence float64 `json:"confidence"`
						Words      []struct {
							StartOffset string `json:"startOffset"`
							EndOffset   string `json:"endOffset"`
							Word        string `json:"word"`
						} `json:"words"`
					}{
						{
							Transcript: "Guten Tag",
							Confidence: 0.95,
							Words: []struct {
								StartOffset string `json:"startOffset"`
								EndOffset   string `json:"endOffset"`
								Word        string `json:"word"`
							}{
								{StartOffset: "0s", EndOffset: "0.5s", Word: "Guten"},
								{StartOffset: "0.5s", EndOffset: "1s", Word: "Tag"},
							},
						},
					},
				},
			},
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}))
	defer srv.Close()

	provider := NewGoogleSTTProvider(GoogleSTTProviderConfig{
		APIKey:  "test-google-key",
		BaseURL: srv.URL,
	})

	result, err := provider.Transcribe(context.Background(), "https://example.com/audio.ogg", STTOptions{Language: "de-DE"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Text != "Guten Tag" {
		t.Errorf("got text=%q want=%q", result.Text, "Guten Tag")
	}
	if result.Confidence != 0.95 {
		t.Errorf("got confidence=%f want=%f", result.Confidence, 0.95)
	}
	if len(result.Segments) != 2 {
		t.Fatalf("got %d segments want 2", len(result.Segments))
	}
}

func TestSTTService_Fallback(t *testing.T) {
	t.Parallel()

	failServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		http.Error(w, "internal error", http.StatusInternalServerError)
	}))
	defer failServer.Close()

	okServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		resp := whisperResponse{Text: "fallback result", Language: "en", Duration: 1.0}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}))
	defer okServer.Close()

	primary := NewWhisperProvider(WhisperProviderConfig{APIKey: "k", BaseURL: failServer.URL})
	fallback := NewWhisperProvider(WhisperProviderConfig{APIKey: "k", BaseURL: okServer.URL})

	svc, err := NewSTTService(STTServiceConfig{Primary: primary, Fallback: fallback})
	if err != nil {
		t.Fatalf("unexpected error creating service: %v", err)
	}

	result, err := svc.Transcribe(context.Background(), "https://example.com/audio.ogg", STTOptions{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Text != "fallback result" {
		t.Errorf("got text=%q want=%q", result.Text, "fallback result")
	}
}

func TestSTTService_InvalidURL(t *testing.T) {
	t.Parallel()

	provider := NewWhisperProvider(WhisperProviderConfig{APIKey: "k", BaseURL: "http://localhost"})
	svc, _ := NewSTTService(STTServiceConfig{Primary: provider})

	_, err := svc.Transcribe(context.Background(), "", STTOptions{})
	if err == nil {
		t.Fatal("expected error for empty URL")
	}
}

func TestNewSTTService_NoProviders(t *testing.T) {
	t.Parallel()

	_, err := NewSTTService(STTServiceConfig{})
	if err == nil {
		t.Fatal("expected error when no providers configured")
	}
}

func TestParseDurationMs(t *testing.T) {
	t.Parallel()

	tests := []struct {
		input string
		want  int64
	}{
		{"0s", 0},
		{"1.5s", 1500},
		{"0.25s", 250},
		{"invalid", 0},
	}
	for _, tt := range tests {
		got := parseDurationMs(tt.input)
		if got != tt.want {
			t.Errorf("parseDurationMs(%q)=%d want=%d", tt.input, got, tt.want)
		}
	}
}
