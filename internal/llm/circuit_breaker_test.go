package llm

import (
	"context"
	"fmt"
	"testing"
	"time"
)

type mockCBLLM struct {
	response string
	err      error
	called   int
}

func (m *mockCBLLM) Complete(_ context.Context, _, _ string) (string, error) {
	m.called++
	return m.response, m.err
}

func TestCircuitBreaker_ClosedState_PassesThrough(t *testing.T) {
	inner := &mockCBLLM{response: "ok"}
	cb := NewLLMCircuitBreaker(inner, "test", DefaultCircuitBreakerConfig(), nil)

	result, err := cb.Complete(context.Background(), "sys", "user")
	if err != nil {
		t.Fatal(err)
	}
	if result != "ok" {
		t.Fatalf("expected 'ok', got %q", result)
	}
	if inner.called != 1 {
		t.Fatalf("expected inner called once, got %d", inner.called)
	}
}

func TestCircuitBreaker_ThreeFailures_Opens(t *testing.T) {
	inner := &mockCBLLM{err: fmt.Errorf("fail")}
	cfg := DefaultCircuitBreakerConfig()
	cfg.FailureThreshold = 3
	cb := NewLLMCircuitBreaker(inner, "test", cfg, nil)

	for i := 0; i < 3; i++ {
		cb.Complete(context.Background(), "s", "u")
	}

	// 4th call should fast-fail
	_, err := cb.Complete(context.Background(), "s", "u")
	if !IsCircuitOpen(err) {
		t.Fatalf("expected ErrCircuitOpen, got: %v", err)
	}
	if inner.called != 3 {
		t.Fatalf("inner should have been called 3 times (not 4), got %d", inner.called)
	}
}

func TestCircuitBreaker_OpenState_FastFails(t *testing.T) {
	inner := &mockCBLLM{err: fmt.Errorf("fail")}
	cfg := DefaultCircuitBreakerConfig()
	cfg.FailureThreshold = 1
	cb := NewLLMCircuitBreaker(inner, "test", cfg, nil)

	cb.Complete(context.Background(), "s", "u") // fail → open

	inner.called = 0
	_, err := cb.Complete(context.Background(), "s", "u")
	if !IsCircuitOpen(err) {
		t.Fatal("expected fast-fail")
	}
	if inner.called != 0 {
		t.Fatal("inner should NOT be called when circuit is open")
	}
}

func TestCircuitBreaker_RecoveryTimeout_TransitionsToHalfOpen(t *testing.T) {
	inner := &mockCBLLM{err: fmt.Errorf("fail")}
	cfg := CircuitBreakerConfig{
		FailureThreshold: 1,
		RecoveryTimeout:  50 * time.Millisecond,
		SuccessThreshold: 1,
	}
	cb := NewLLMCircuitBreaker(inner, "test", cfg, nil)

	cb.Complete(context.Background(), "s", "u") // fail → open

	time.Sleep(60 * time.Millisecond) // wait past recovery timeout

	inner.response = "recovered"
	inner.err = nil
	inner.called = 0

	result, err := cb.Complete(context.Background(), "s", "u")
	if err != nil {
		t.Fatalf("expected success on probe, got: %v", err)
	}
	if result != "recovered" {
		t.Fatalf("expected 'recovered', got %q", result)
	}
	if inner.called != 1 {
		t.Fatal("inner should be called for half-open probe")
	}
}

func TestCircuitBreaker_HalfOpen_Success_Closes(t *testing.T) {
	inner := &mockCBLLM{err: fmt.Errorf("fail")}
	cfg := CircuitBreakerConfig{
		FailureThreshold: 1,
		RecoveryTimeout:  10 * time.Millisecond,
		SuccessThreshold: 2,
	}
	cb := NewLLMCircuitBreaker(inner, "test", cfg, nil)

	cb.Complete(context.Background(), "s", "u") // fail → open
	time.Sleep(15 * time.Millisecond)           // → half-open

	inner.err = nil
	inner.response = "ok"
	cb.Complete(context.Background(), "s", "u") // success 1
	cb.Complete(context.Background(), "s", "u") // success 2 → closed

	if cb.State() != StateClosed {
		t.Fatalf("expected closed after 2 successes, got %s", cb.State())
	}
}

func TestCircuitBreaker_HalfOpen_Failure_Reopens(t *testing.T) {
	inner := &mockCBLLM{err: fmt.Errorf("fail")}
	cfg := CircuitBreakerConfig{
		FailureThreshold: 1,
		RecoveryTimeout:  10 * time.Millisecond,
		SuccessThreshold: 2,
	}
	cb := NewLLMCircuitBreaker(inner, "test", cfg, nil)

	cb.Complete(context.Background(), "s", "u") // fail → open
	time.Sleep(15 * time.Millisecond)           // → half-open

	// Probe fails → back to open
	cb.Complete(context.Background(), "s", "u")

	if cb.State() != StateOpen {
		t.Fatalf("expected open after half-open failure, got %s", cb.State())
	}
}

func TestIsCircuitOpen_TrueForErrCircuitOpen(t *testing.T) {
	if !IsCircuitOpen(ErrCircuitOpen) {
		t.Fatal("expected true")
	}
}

func TestIsCircuitOpen_FalseForOtherErrors(t *testing.T) {
	if IsCircuitOpen(fmt.Errorf("other error")) {
		t.Fatal("expected false for non-circuit error")
	}
}

func TestIsLLMTimeout_TrueForDeadlineExceeded(t *testing.T) {
	if !IsLLMTimeout(context.DeadlineExceeded) {
		t.Fatal("expected true for DeadlineExceeded")
	}
}

func TestIsLLMTimeout_FalseForOtherErrors(t *testing.T) {
	if IsLLMTimeout(fmt.Errorf("API error")) {
		t.Fatal("expected false")
	}
}
