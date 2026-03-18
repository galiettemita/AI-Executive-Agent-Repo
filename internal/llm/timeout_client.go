package llm

import (
	"context"
	"errors"
	"fmt"
	"time"
)

// LLMCompleter is the minimal LLM interface used across the codebase.
type LLMCompleter interface {
	Complete(ctx context.Context, system, user string) (string, error)
}

// TimeoutMetrics records timeout events.
type TimeoutMetrics interface {
	IncTimeout(clientName string)
	IncSuccess(clientName string)
	ObserveDuration(clientName string, dur time.Duration)
}

// NoopTimeoutMetrics is a no-op implementation.
type NoopTimeoutMetrics struct{}

func (n *NoopTimeoutMetrics) IncTimeout(_ string)                          {}
func (n *NoopTimeoutMetrics) IncSuccess(_ string)                          {}
func (n *NoopTimeoutMetrics) ObserveDuration(_ string, _ time.Duration)    {}

// TimeoutLLMClient wraps any LLMCompleter with a hard per-call deadline.
type TimeoutLLMClient struct {
	inner   LLMCompleter
	timeout time.Duration
	name    string
	metrics TimeoutMetrics
}

func NewTimeoutLLMClient(inner LLMCompleter, timeout time.Duration, name string, metrics TimeoutMetrics) (*TimeoutLLMClient, error) {
	if inner == nil {
		return nil, fmt.Errorf("llm.NewTimeoutLLMClient: inner must not be nil")
	}
	if timeout <= 0 {
		timeout = 2 * time.Second
	}
	if metrics == nil {
		metrics = &NoopTimeoutMetrics{}
	}
	return &TimeoutLLMClient{inner: inner, timeout: timeout, name: name, metrics: metrics}, nil
}

// Complete calls the underlying LLM client with a hard deadline.
func (c *TimeoutLLMClient) Complete(ctx context.Context, system, user string) (string, error) {
	deadline := time.Now().Add(c.timeout)
	if parentDeadline, ok := ctx.Deadline(); ok && parentDeadline.Before(deadline) {
		deadline = parentDeadline
	}

	callCtx, cancel := context.WithDeadline(ctx, deadline)
	defer cancel()

	start := time.Now()

	type result struct {
		text string
		err  error
	}
	ch := make(chan result, 1)

	go func() {
		text, err := c.inner.Complete(callCtx, system, user)
		ch <- result{text, err}
	}()

	select {
	case r := <-ch:
		elapsed := time.Since(start)
		c.metrics.ObserveDuration(c.name, elapsed)
		if r.err != nil {
			if isTimeoutErr(r.err) {
				c.metrics.IncTimeout(c.name)
				return "", fmt.Errorf("llm_timeout[%s]: %w", c.name, r.err)
			}
			return "", r.err
		}
		c.metrics.IncSuccess(c.name)
		return r.text, nil

	case <-callCtx.Done():
		elapsed := time.Since(start)
		c.metrics.ObserveDuration(c.name, elapsed)
		c.metrics.IncTimeout(c.name)
		return "", fmt.Errorf("llm_timeout[%s]: deadline exceeded after %v: %w",
			c.name, elapsed, callCtx.Err())
	}
}

// IsLLMTimeout returns true if the error is a timeout from a TimeoutLLMClient.
func IsLLMTimeout(err error) bool {
	if err == nil {
		return false
	}
	return isTimeoutErr(err)
}

func isTimeoutErr(err error) bool {
	return errors.Is(err, context.DeadlineExceeded) || errors.Is(err, context.Canceled)
}
