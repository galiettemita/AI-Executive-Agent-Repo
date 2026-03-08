package worker

import (
	"fmt"
	"testing"
)

type mockSTTProvider struct {
	result *STTResult
	err    error
}

func (m *mockSTTProvider) Transcribe(_ []byte) (*STTResult, error) {
	return m.result, m.err
}

func TestSTTTranscribeSuccess(t *testing.T) {
	t.Parallel()

	primary := &mockSTTProvider{result: &STTResult{Text: "hello world", Confidence: 0.95, Language: "en", DurationMs: 1200}}
	fallback := &mockSTTProvider{result: &STTResult{Text: "hello", Confidence: 0.80, Language: "en"}}

	svc := NewSTTService(primary, fallback)
	result, err := svc.Transcribe([]byte("audio-data"))
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if result.Text != "hello world" {
		t.Fatalf("expected 'hello world', got %s", result.Text)
	}
	if svc.IsUsingFallback() {
		t.Fatal("expected to use primary provider")
	}
}

func TestSTTTranscribeEmptyAudio(t *testing.T) {
	t.Parallel()

	svc := NewSTTService(&mockSTTProvider{}, nil)
	_, err := svc.Transcribe([]byte{})
	if err == nil {
		t.Fatal("expected error for empty audio data")
	}
}

func TestSTTFailoverAfterConsecutiveErrors(t *testing.T) {
	t.Parallel()

	primary := &mockSTTProvider{err: fmt.Errorf("primary down")}
	fallback := &mockSTTProvider{result: &STTResult{Text: "fallback result", Confidence: 0.85, Language: "en"}}

	svc := NewSTTService(primary, fallback)

	// First two errors should not trigger failover.
	svc.Transcribe([]byte("audio"))
	svc.Transcribe([]byte("audio"))

	if svc.IsUsingFallback() {
		t.Fatal("expected primary still active after 2 errors")
	}

	// Third error triggers failover.
	result, err := svc.Transcribe([]byte("audio"))
	if err != nil {
		t.Fatalf("expected fallback to succeed, got %v", err)
	}
	if result.Text != "fallback result" {
		t.Fatalf("expected fallback result, got %s", result.Text)
	}
	if !svc.IsUsingFallback() {
		t.Fatal("expected fallback to be active")
	}
}

func TestSTTResetToPrimary(t *testing.T) {
	t.Parallel()

	primary := &mockSTTProvider{err: fmt.Errorf("down")}
	fallback := &mockSTTProvider{result: &STTResult{Text: "ok"}}

	svc := NewSTTService(primary, fallback)

	// Trigger failover.
	for i := 0; i < 3; i++ {
		svc.Transcribe([]byte("audio"))
	}

	svc.ResetToTPrimary()
	if svc.IsUsingFallback() {
		t.Fatal("expected primary after reset")
	}
}

func TestSTTNoFallbackProvider(t *testing.T) {
	t.Parallel()

	primary := &mockSTTProvider{err: fmt.Errorf("error")}
	svc := NewSTTService(primary, nil)

	_, err := svc.Transcribe([]byte("audio"))
	if err == nil {
		t.Fatal("expected error when no fallback is available")
	}
}
