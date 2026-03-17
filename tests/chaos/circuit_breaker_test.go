//go:build chaos

package chaos

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

type circuitBreaker struct {
	mu           sync.Mutex
	failureCount int
	threshold    int
	open         bool
	openedAt     time.Time
	resetTimeout time.Duration
}

func newCircuitBreaker(threshold int, resetTimeout time.Duration) *circuitBreaker {
	return &circuitBreaker{threshold: threshold, resetTimeout: resetTimeout}
}

func (cb *circuitBreaker) Call(fn func() error) error {
	cb.mu.Lock()
	if cb.open {
		if time.Since(cb.openedAt) > cb.resetTimeout {
			cb.open = false
			cb.failureCount = 0
			cb.mu.Unlock()
		} else {
			cb.mu.Unlock()
			return fmt.Errorf("circuit breaker OPEN — call rejected")
		}
	} else {
		cb.mu.Unlock()
	}

	err := fn()

	cb.mu.Lock()
	defer cb.mu.Unlock()
	if err != nil {
		cb.failureCount++
		if cb.failureCount >= cb.threshold {
			cb.open = true
			cb.openedAt = time.Now()
		}
	} else {
		cb.failureCount = 0
		cb.open = false
	}
	return err
}

func (cb *circuitBreaker) IsOpen() bool {
	cb.mu.Lock()
	defer cb.mu.Unlock()
	return cb.open
}

type llmService struct {
	cb         *circuitBreaker
	httpClient *http.Client
	baseURL    string
}

func newLLMService(baseURL string, cb *circuitBreaker) *llmService {
	return &llmService{
		cb:         cb,
		httpClient: &http.Client{Timeout: 3 * time.Second},
		baseURL:    baseURL,
	}
}

func (s *llmService) Complete(prompt string) error {
	return s.cb.Call(func() error {
		resp, err := s.httpClient.Get(s.baseURL + "/llm/complete?q=" + prompt)
		if err != nil {
			return fmt.Errorf("llm http request failed: %w", err)
		}
		defer resp.Body.Close()
		if resp.StatusCode != http.StatusOK {
			return fmt.Errorf("llm returned HTTP %d", resp.StatusCode)
		}
		return nil
	})
}

func TestChaos_CircuitBreakerTrips(t *testing.T) {
	var requestCount int64
	server := httptest.NewServer(http.HandlerFunc(
		func(w http.ResponseWriter, r *http.Request) {
			count := atomic.AddInt64(&requestCount, 1)
			if count <= 5 {
				w.WriteHeader(http.StatusServiceUnavailable)
				_, _ = w.Write([]byte(`{"error":"service unavailable"}`))
				return
			}
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"status":"ok"}`))
		}))
	defer server.Close()

	cb := newCircuitBreaker(5, 10*time.Second)
	llm := newLLMService(server.URL, cb)

	for i := 0; i < 5; i++ {
		err := llm.Complete("chaos-probe")
		t.Logf("LLM call %d: err=%v  circuitOpen=%v", i+1, err, cb.IsOpen())
	}

	if !cb.IsOpen() {
		t.Errorf("circuit breaker should be OPEN after 5 consecutive LLM " +
			"failures but is CLOSED")
	} else {
		t.Logf("Circuit breaker opened after 5 consecutive LLM failures")
	}

	countBefore := atomic.LoadInt64(&requestCount)
	rejectedErr := llm.Complete("should-be-rejected")
	countAfter := atomic.LoadInt64(&requestCount)

	if rejectedErr == nil {
		t.Errorf("expected open circuit breaker to reject call but got nil error")
	}
	if countAfter != countBefore {
		t.Errorf("circuit breaker made HTTP request while OPEN — "+
			"should have short-circuited (requests before=%d after=%d)",
			countBefore, countAfter)
	}
	t.Logf("Open circuit rejected LLM call without contacting server "+
		"(requests: %d → %d)", countBefore, countAfter)

	t.Logf("Waiting 10s for circuit breaker reset timeout...")
	time.Sleep(10 * time.Second)

	resetErr := llm.Complete("post-reset-probe")
	if resetErr != nil {
		t.Errorf("circuit breaker did not reset after 10s timeout: %v", resetErr)
	}
	if cb.IsOpen() {
		t.Errorf("circuit breaker should be CLOSED after successful LLM " +
			"call post-reset but is still OPEN")
	}
	t.Logf("Circuit breaker reset — closed after successful LLM call")
}
