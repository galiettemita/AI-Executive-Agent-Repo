package llm

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"
)

// CircuitState represents the state of the circuit breaker.
type CircuitState int

const (
	StateClosed   CircuitState = iota
	StateOpen
	StateHalfOpen
)

func (s CircuitState) String() string {
	switch s {
	case StateClosed:
		return "closed"
	case StateOpen:
		return "open"
	case StateHalfOpen:
		return "half_open"
	default:
		return "unknown"
	}
}

// ErrCircuitOpen is returned when the circuit is open (fast-fail).
var ErrCircuitOpen = fmt.Errorf("llm circuit open: service temporarily unavailable")

// CircuitBreakerConfig holds tunable parameters.
type CircuitBreakerConfig struct {
	FailureThreshold int
	RecoveryTimeout  time.Duration
	SuccessThreshold int
}

func DefaultCircuitBreakerConfig() CircuitBreakerConfig {
	return CircuitBreakerConfig{
		FailureThreshold: 3,
		RecoveryTimeout:  15 * time.Second,
		SuccessThreshold: 2,
	}
}

// CircuitBreakerMetrics records state transitions and fast-fail events.
type CircuitBreakerMetrics interface {
	RecordStateTransition(name, from, to string)
	IncFastFail(name string)
	IncProbe(name string)
}

// NoopCircuitBreakerMetrics is a no-op implementation.
type NoopCircuitBreakerMetrics struct{}

func (n *NoopCircuitBreakerMetrics) RecordStateTransition(_, _, _ string) {}
func (n *NoopCircuitBreakerMetrics) IncFastFail(_ string)                {}
func (n *NoopCircuitBreakerMetrics) IncProbe(_ string)                   {}

// LLMCircuitBreaker wraps an LLM client with a circuit breaker pattern.
type LLMCircuitBreaker struct {
	inner   LLMCompleter
	name    string
	cfg     CircuitBreakerConfig
	metrics CircuitBreakerMetrics

	mu                 sync.Mutex
	state              CircuitState
	consecutiveFails   int
	consecutiveSuccess int
	lastStateChange    time.Time
}

func NewLLMCircuitBreaker(
	inner LLMCompleter,
	name string,
	cfg CircuitBreakerConfig,
	metrics CircuitBreakerMetrics,
) (*LLMCircuitBreaker, error) {
	if inner == nil {
		return nil, fmt.Errorf("llm.NewLLMCircuitBreaker: inner must not be nil")
	}
	if metrics == nil {
		metrics = &NoopCircuitBreakerMetrics{}
	}
	return &LLMCircuitBreaker{
		inner:           inner,
		name:            name,
		cfg:             cfg,
		metrics:         metrics,
		state:           StateClosed,
		lastStateChange: time.Now(),
	}, nil
}

// Complete routes the call through the circuit breaker.
func (cb *LLMCircuitBreaker) Complete(ctx context.Context, system, user string) (string, error) {
	cb.mu.Lock()
	state := cb.currentState()
	cb.mu.Unlock()

	switch state {
	case StateOpen:
		cb.metrics.IncFastFail(cb.name)
		return "", ErrCircuitOpen
	case StateHalfOpen:
		cb.metrics.IncProbe(cb.name)
	}

	result, err := cb.inner.Complete(ctx, system, user)

	cb.mu.Lock()
	defer cb.mu.Unlock()

	if err != nil {
		cb.onFailure()
		return "", err
	}
	cb.onSuccess()
	return result, nil
}

// State returns the current circuit state.
func (cb *LLMCircuitBreaker) State() CircuitState {
	cb.mu.Lock()
	defer cb.mu.Unlock()
	return cb.currentState()
}

func (cb *LLMCircuitBreaker) currentState() CircuitState {
	if cb.state == StateOpen &&
		time.Since(cb.lastStateChange) >= cb.cfg.RecoveryTimeout {
		cb.transitionTo(StateHalfOpen)
	}
	return cb.state
}

func (cb *LLMCircuitBreaker) onFailure() {
	cb.consecutiveSuccess = 0
	cb.consecutiveFails++

	switch cb.state {
	case StateClosed:
		if cb.consecutiveFails >= cb.cfg.FailureThreshold {
			cb.transitionTo(StateOpen)
		}
	case StateHalfOpen:
		cb.transitionTo(StateOpen)
	}
}

func (cb *LLMCircuitBreaker) onSuccess() {
	cb.consecutiveFails = 0
	cb.consecutiveSuccess++

	if cb.state == StateHalfOpen &&
		cb.consecutiveSuccess >= cb.cfg.SuccessThreshold {
		cb.transitionTo(StateClosed)
	}
}

func (cb *LLMCircuitBreaker) transitionTo(next CircuitState) {
	prev := cb.state
	cb.state = next
	cb.lastStateChange = time.Now()
	cb.consecutiveFails = 0
	cb.consecutiveSuccess = 0
	cb.metrics.RecordStateTransition(cb.name, prev.String(), next.String())
}

// IsCircuitOpen returns true if the error is ErrCircuitOpen.
func IsCircuitOpen(err error) bool {
	return errors.Is(err, ErrCircuitOpen)
}
