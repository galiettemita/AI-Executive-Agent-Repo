package llm

import (
	"context"
	"fmt"
	"sync"
	"time"
)

// ClientCircuitBreaker wraps a Client with circuit breaker protection.
// State machine: Closed → Open (on FailureThreshold) → HalfOpen (after RecoveryTimeout) → Closed (on SuccessThreshold).
type ClientCircuitBreaker struct {
	inner Client
	name  string
	cfg   CircuitBreakerConfig

	mu                 sync.Mutex
	state              CircuitState
	consecutiveFails   int
	consecutiveSuccess int
	lastStateChange    time.Time
}

// NewClientCircuitBreaker creates a circuit breaker wrapping a Client.
func NewClientCircuitBreaker(inner Client, name string, cfg CircuitBreakerConfig) *ClientCircuitBreaker {
	if inner == nil {
		panic("NewClientCircuitBreaker: inner must not be nil")
	}
	return &ClientCircuitBreaker{
		inner:           inner,
		name:            name,
		cfg:             cfg,
		state:           StateClosed,
		lastStateChange: time.Now(),
	}
}

// Generate routes a generation request through the circuit breaker.
func (cb *ClientCircuitBreaker) Generate(ctx context.Context, req GenerateRequest) (*GenerateResponse, *Usage, error) {
	cb.mu.Lock()
	s := cb.currentState()
	cb.mu.Unlock()

	if s == StateOpen {
		return nil, nil, ErrCircuitOpen
	}

	resp, usage, err := cb.inner.Generate(ctx, req)

	cb.mu.Lock()
	defer cb.mu.Unlock()
	if err != nil {
		cb.onFailure()
	} else {
		cb.onSuccess()
	}
	return resp, usage, err
}

// Stream routes a streaming request through the circuit breaker.
func (cb *ClientCircuitBreaker) Stream(ctx context.Context, req GenerateRequest, out chan<- StreamChunk) {
	cb.mu.Lock()
	s := cb.currentState()
	cb.mu.Unlock()

	if s == StateOpen {
		out <- StreamChunk{Error: fmt.Errorf("circuit open for provider %s", cb.name)}
		close(out)
		return
	}

	inner := make(chan StreamChunk, 16)
	go cb.inner.Stream(ctx, req, inner)

	var hadError bool
	for chunk := range inner {
		if chunk.Error != nil {
			hadError = true
		}
		out <- chunk
	}
	close(out)

	cb.mu.Lock()
	defer cb.mu.Unlock()
	if hadError {
		cb.onFailure()
	} else {
		cb.onSuccess()
	}
}

// State returns the current circuit state.
func (cb *ClientCircuitBreaker) State() CircuitState {
	cb.mu.Lock()
	defer cb.mu.Unlock()
	return cb.currentState()
}

func (cb *ClientCircuitBreaker) currentState() CircuitState {
	if cb.state == StateOpen && time.Since(cb.lastStateChange) >= cb.cfg.RecoveryTimeout {
		cb.transitionTo(StateHalfOpen)
	}
	return cb.state
}

func (cb *ClientCircuitBreaker) onFailure() {
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

func (cb *ClientCircuitBreaker) onSuccess() {
	cb.consecutiveFails = 0
	cb.consecutiveSuccess++
	if cb.state == StateHalfOpen && cb.consecutiveSuccess >= cb.cfg.SuccessThreshold {
		cb.transitionTo(StateClosed)
	}
}

func (cb *ClientCircuitBreaker) transitionTo(next CircuitState) {
	cb.state = next
	cb.lastStateChange = time.Now()
	cb.consecutiveFails = 0
	cb.consecutiveSuccess = 0
}
