package llm

import (
	"context"
	"fmt"
	"testing"
	"time"
)

type slowLLM struct {
	delay    time.Duration
	response string
	err      error
}

func (s *slowLLM) Complete(ctx context.Context, _, _ string) (string, error) {
	select {
	case <-time.After(s.delay):
		return s.response, s.err
	case <-ctx.Done():
		return "", ctx.Err()
	}
}

func TestTimeoutClient_FastCall_ReturnsResult(t *testing.T) {
	inner := &slowLLM{delay: 50 * time.Millisecond, response: "hello"}
	client := NewTimeoutLLMClient(inner, 1*time.Second, "test", nil)

	result, err := client.Complete(context.Background(), "sys", "user")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result != "hello" {
		t.Fatalf("expected 'hello', got %q", result)
	}
}

func TestTimeoutClient_SlowCall_TimesOut(t *testing.T) {
	inner := &slowLLM{delay: 2 * time.Second, response: "late"}
	client := NewTimeoutLLMClient(inner, 200*time.Millisecond, "test", nil)

	start := time.Now()
	_, err := client.Complete(context.Background(), "sys", "user")
	elapsed := time.Since(start)

	if err == nil {
		t.Fatal("expected timeout error")
	}
	if !IsLLMTimeout(err) {
		t.Fatalf("expected IsLLMTimeout=true, got error: %v", err)
	}
	if elapsed > 500*time.Millisecond {
		t.Fatalf("expected fast return, took %v", elapsed)
	}
}

func TestTimeoutClient_RespectsParentContextDeadline(t *testing.T) {
	inner := &slowLLM{delay: 5 * time.Second, response: "late"}
	client := NewTimeoutLLMClient(inner, 2*time.Second, "test", nil)

	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	start := time.Now()
	_, err := client.Complete(ctx, "sys", "user")
	elapsed := time.Since(start)

	if err == nil {
		t.Fatal("expected error")
	}
	if elapsed > 300*time.Millisecond {
		t.Fatalf("should respect parent deadline, took %v", elapsed)
	}
}

func TestTimeoutClient_ErrorFromInner_Propagated(t *testing.T) {
	inner := &slowLLM{delay: 10 * time.Millisecond, err: fmt.Errorf("API error")}
	client := NewTimeoutLLMClient(inner, 1*time.Second, "test", nil)

	_, err := client.Complete(context.Background(), "sys", "user")
	if err == nil {
		t.Fatal("expected error")
	}
	if IsLLMTimeout(err) {
		t.Fatal("should NOT be a timeout error")
	}
}
