package llm_test

import (
	"context"
	"fmt"
	"sync/atomic"
	"testing"
	"time"

	"github.com/brevio/brevio/internal/llm"
)

type cbMockClient struct {
	err   error
	calls atomic.Int64
}

func (m *cbMockClient) Generate(_ context.Context, _ llm.GenerateRequest) (*llm.GenerateResponse, *llm.Usage, error) {
	m.calls.Add(1)
	if m.err != nil {
		return nil, nil, m.err
	}
	return &llm.GenerateResponse{Content: "ok"}, &llm.Usage{}, nil
}

func (m *cbMockClient) Stream(_ context.Context, _ llm.GenerateRequest, out chan<- llm.StreamChunk) {
	defer close(out)
	if m.err != nil {
		out <- llm.StreamChunk{Error: m.err}
	} else {
		out <- llm.StreamChunk{Delta: "ok", Done: true}
	}
}

func TestClientCircuitBreaker_PassesThrough(t *testing.T) {
	t.Parallel()
	mc := &cbMockClient{}
	cb := llm.NewClientCircuitBreaker(mc, "test", llm.DefaultCircuitBreakerConfig())
	_, _, err := cb.Generate(context.Background(), llm.GenerateRequest{})
	if err != nil {
		t.Errorf("expected no error, got %v", err)
	}
}

func TestClientCircuitBreaker_OpensAfterFailures(t *testing.T) {
	t.Parallel()
	mc := &cbMockClient{err: fmt.Errorf("provider down")}
	cfg := llm.DefaultCircuitBreakerConfig()
	cb := llm.NewClientCircuitBreaker(mc, "test", cfg)

	for i := 0; i < cfg.FailureThreshold; i++ {
		cb.Generate(context.Background(), llm.GenerateRequest{})
	}

	_, _, err := cb.Generate(context.Background(), llm.GenerateRequest{})
	if err == nil {
		t.Error("expected ErrCircuitOpen after failures")
	}
}

func TestClientCircuitBreaker_RecoveryToHalfOpen(t *testing.T) {
	t.Parallel()
	mc := &cbMockClient{err: fmt.Errorf("down")}
	cfg := llm.DefaultCircuitBreakerConfig()
	cfg.RecoveryTimeout = 50 * time.Millisecond
	cb := llm.NewClientCircuitBreaker(mc, "test", cfg)

	for i := 0; i < cfg.FailureThreshold; i++ {
		cb.Generate(context.Background(), llm.GenerateRequest{})
	}

	time.Sleep(60 * time.Millisecond)
	mc.err = nil
	_, _, err := cb.Generate(context.Background(), llm.GenerateRequest{})
	if err != nil {
		t.Errorf("expected probe to succeed after recovery, got %v", err)
	}
}

func TestClientCircuitBreaker_ConcurrentSafe(t *testing.T) {
	t.Parallel()
	mc := &cbMockClient{}
	cb := llm.NewClientCircuitBreaker(mc, "concurrent", llm.DefaultCircuitBreakerConfig())
	done := make(chan struct{}, 50)
	for i := 0; i < 50; i++ {
		go func() {
			cb.Generate(context.Background(), llm.GenerateRequest{})
			done <- struct{}{}
		}()
	}
	for i := 0; i < 50; i++ {
		<-done
	}
}

func TestT3TierUsesOpusModel(t *testing.T) {
	t.Parallel()
	m := llm.DefaultTierModelMapping()
	t3 := m["T3"]
	if t3.PrimaryModel != llm.ModelAnthropicOpus {
		t.Errorf("T3 PrimaryModel = %q, want %q", t3.PrimaryModel, llm.ModelAnthropicOpus)
	}
}

func TestT2TierUsesSonnet(t *testing.T) {
	t.Parallel()
	m := llm.DefaultTierModelMapping()
	if m["T2"].PrimaryModel != llm.ModelAnthropicSonnet {
		t.Errorf("T2 PrimaryModel should be Sonnet")
	}
}
