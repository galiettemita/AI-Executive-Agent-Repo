package worker

import (
	"fmt"
	"testing"
)

type mockTTSProvider struct {
	result *TTSResult
	err    error
}

func (m *mockTTSProvider) Synthesize(_ TTSRequest) (*TTSResult, error) {
	return m.result, m.err
}

func TestTTSSynthesizeSuccess(t *testing.T) {
	t.Parallel()

	primary := &mockTTSProvider{result: &TTSResult{AudioData: []byte("audio"), DurationMs: 1500, Provider: "primary"}}
	fallback := &mockTTSProvider{result: &TTSResult{AudioData: []byte("fb-audio"), DurationMs: 1400, Provider: "fallback"}}

	svc := NewTTSService(primary, fallback)
	result, err := svc.Synthesize(TTSRequest{Text: "Hello world", Voice: "default", Language: "en"})
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if result.Provider != "primary" {
		t.Fatalf("expected primary provider, got %s", result.Provider)
	}
}

func TestTTSSynthesizeEmptyText(t *testing.T) {
	t.Parallel()

	svc := NewTTSService(&mockTTSProvider{}, nil)
	_, err := svc.Synthesize(TTSRequest{Text: ""})
	if err == nil {
		t.Fatal("expected error for empty text")
	}
}

func TestTTSFailoverAfterConsecutiveErrors(t *testing.T) {
	t.Parallel()

	primary := &mockTTSProvider{err: fmt.Errorf("primary down")}
	fallback := &mockTTSProvider{result: &TTSResult{AudioData: []byte("ok"), DurationMs: 1000, Provider: "fallback"}}

	svc := NewTTSService(primary, fallback)

	// First two errors.
	svc.Synthesize(TTSRequest{Text: "test"})
	svc.Synthesize(TTSRequest{Text: "test"})

	if svc.IsUsingFallback() {
		t.Fatal("expected primary still active after 2 errors")
	}

	// Third error triggers failover.
	result, err := svc.Synthesize(TTSRequest{Text: "test"})
	if err != nil {
		t.Fatalf("expected fallback to succeed, got %v", err)
	}
	if result.Provider != "fallback" {
		t.Fatalf("expected fallback provider, got %s", result.Provider)
	}
	if !svc.IsUsingFallback() {
		t.Fatal("expected fallback to be active")
	}
}

func TestTTSResetToPrimary(t *testing.T) {
	t.Parallel()

	primary := &mockTTSProvider{err: fmt.Errorf("down")}
	fallback := &mockTTSProvider{result: &TTSResult{AudioData: []byte("ok"), Provider: "fallback"}}

	svc := NewTTSService(primary, fallback)

	for i := 0; i < 3; i++ {
		svc.Synthesize(TTSRequest{Text: "test"})
	}

	svc.ResetToPrimary()
	if svc.IsUsingFallback() {
		t.Fatal("expected primary after reset")
	}
}

func TestTTSNoFallback(t *testing.T) {
	t.Parallel()

	primary := &mockTTSProvider{err: fmt.Errorf("error")}
	svc := NewTTSService(primary, nil)

	_, err := svc.Synthesize(TTSRequest{Text: "test"})
	if err == nil {
		t.Fatal("expected error when no fallback available")
	}
}
