package control

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"sort"
	"strings"
	"sync"
	"time"
)

// OPAClientConfig configures the OPA HTTP sidecar client.
type OPAClientConfig struct {
	// BaseURL is the OPA sidecar endpoint (e.g. "http://localhost:8181").
	BaseURL string

	// Timeout per individual OPA request.
	Timeout time.Duration

	// MaxRetries for transient failures (5xx, timeout).
	MaxRetries int

	// RetryBackoff is the base delay between retries (doubled each attempt).
	RetryBackoff time.Duration

	// CircuitBreakerThreshold is the number of consecutive failures before
	// the circuit opens.
	CircuitBreakerThreshold int

	// CircuitBreakerCooldown is how long the circuit stays open before
	// allowing a probe request.
	CircuitBreakerCooldown time.Duration
}

// DefaultOPAClientConfig returns production defaults.
func DefaultOPAClientConfig() OPAClientConfig {
	return OPAClientConfig{
		BaseURL:                 "http://localhost:8181",
		Timeout:                 2 * time.Second,
		MaxRetries:              2,
		RetryBackoff:            100 * time.Millisecond,
		CircuitBreakerThreshold: 5,
		CircuitBreakerCooldown:  30 * time.Second,
	}
}

// circuitState tracks circuit breaker state.
type circuitState struct {
	mu                sync.Mutex
	consecutiveErrors int
	threshold         int
	cooldown          time.Duration
	openedAt          time.Time
}

func newCircuitState(threshold int, cooldown time.Duration) *circuitState {
	return &circuitState{
		threshold: threshold,
		cooldown:  cooldown,
	}
}

func (cs *circuitState) isOpen() bool {
	cs.mu.Lock()
	defer cs.mu.Unlock()
	if cs.consecutiveErrors < cs.threshold {
		return false
	}
	// Allow probe after cooldown.
	if time.Since(cs.openedAt) > cs.cooldown {
		return false
	}
	return true
}

func (cs *circuitState) recordSuccess() {
	cs.mu.Lock()
	defer cs.mu.Unlock()
	cs.consecutiveErrors = 0
}

func (cs *circuitState) recordFailure() {
	cs.mu.Lock()
	defer cs.mu.Unlock()
	cs.consecutiveErrors++
	if cs.consecutiveErrors >= cs.threshold {
		cs.openedAt = time.Now()
	}
}

// ConsecutiveErrors returns the current consecutive error count (for testing/metrics).
func (cs *circuitState) ConsecutiveErrors() int {
	cs.mu.Lock()
	defer cs.mu.Unlock()
	return cs.consecutiveErrors
}

// OPAClient is an HTTP client for the OPA sidecar.
type OPAClient struct {
	httpClient *http.Client
	baseURL    string
	maxRetries int
	retryDelay time.Duration
	circuit    *circuitState
}

// NewOPAClient creates an OPA HTTP client with the given config.
func NewOPAClient(cfg OPAClientConfig) *OPAClient {
	return &OPAClient{
		httpClient: &http.Client{Timeout: cfg.Timeout},
		baseURL:    strings.TrimRight(cfg.BaseURL, "/"),
		maxRetries: cfg.MaxRetries,
		retryDelay: cfg.RetryBackoff,
		circuit:    newCircuitState(cfg.CircuitBreakerThreshold, cfg.CircuitBreakerCooldown),
	}
}

// ErrOPAUnavailable indicates OPA is unreachable or circuit is open.
var ErrOPAUnavailable = fmt.Errorf("OPA_UNAVAILABLE: policy engine unreachable, deny-by-default")

// ErrOPADeny indicates OPA returned an explicit deny.
var ErrOPADeny = fmt.Errorf("OPA_DENY: policy evaluation denied the request")

// OPAResponse represents the JSON response from OPA's /v1/data endpoint.
type OPAResponse struct {
	Result map[string]any `json:"result"`
}

// EvaluatePolicy sends a policy query to OPA and returns the decision.
// On any failure (timeout, circuit open, error), returns deny-by-default.
func (c *OPAClient) EvaluatePolicy(ctx context.Context, packagePath string, input PolicyInput) (*PolicyDecision, error) {
	if c.circuit.isOpen() {
		return &PolicyDecision{
			Allowed: false,
			Reason:  "opa_circuit_open",
		}, ErrOPAUnavailable
	}

	canonicalInput, err := CanonicalizeInput(input)
	if err != nil {
		return &PolicyDecision{
			Allowed: false,
			Reason:  "input_canonicalization_failed",
		}, fmt.Errorf("opa: canonicalize input: %w", err)
	}

	reqBody, err := json.Marshal(map[string]json.RawMessage{
		"input": canonicalInput,
	})
	if err != nil {
		return &PolicyDecision{
			Allowed: false,
			Reason:  "input_marshal_failed",
		}, fmt.Errorf("opa: marshal request: %w", err)
	}

	endpoint := fmt.Sprintf("%s/v1/data/%s", c.baseURL, strings.ReplaceAll(packagePath, ".", "/"))

	var lastErr error
	for attempt := 0; attempt <= c.maxRetries; attempt++ {
		if attempt > 0 {
			delay := c.retryDelay * (1 << (attempt - 1))
			select {
			case <-ctx.Done():
				c.circuit.recordFailure()
				return &PolicyDecision{
					Allowed: false,
					Reason:  "opa_context_cancelled",
				}, ctx.Err()
			case <-time.After(delay):
			}
		}

		decision, err := c.doRequest(ctx, endpoint, reqBody)
		if err == nil {
			c.circuit.recordSuccess()
			return decision, nil
		}
		lastErr = err
	}

	c.circuit.recordFailure()
	return &PolicyDecision{
		Allowed: false,
		Reason:  "opa_unavailable",
	}, fmt.Errorf("opa: all retries exhausted: %w", lastErr)
}

func (c *OPAClient) doRequest(ctx context.Context, endpoint string, body []byte) (*PolicyDecision, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("http request: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20)) // 1MB limit
	if err != nil {
		return nil, fmt.Errorf("read response: %w", err)
	}

	if resp.StatusCode >= 500 {
		return nil, fmt.Errorf("opa server error: status %d", resp.StatusCode)
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("opa unexpected status: %d body=%s", resp.StatusCode, string(respBody))
	}

	var opaResp OPAResponse
	if err := json.Unmarshal(respBody, &opaResp); err != nil {
		return nil, fmt.Errorf("unmarshal opa response: %w", err)
	}

	return mapOPAResult(opaResp.Result), nil
}

// mapOPAResult converts OPA's result map into a PolicyDecision.
// OPA Rego policies should return:
//
//	{
//	  "allow": true/false,
//	  "deny": ["reason1", "reason2"],
//	  "require_approval": true/false,
//	  "constraints": { ... }
//	}
func mapOPAResult(result map[string]any) *PolicyDecision {
	if result == nil {
		return &PolicyDecision{
			Allowed: false,
			Reason:  "opa_empty_result",
		}
	}

	decision := &PolicyDecision{
		Constraints: make(map[string]any),
	}

	// Check deny set first (deny-by-default).
	if denySet, ok := result["deny"]; ok {
		if reasons, ok := denySet.([]any); ok && len(reasons) > 0 {
			msgs := make([]string, 0, len(reasons))
			for _, r := range reasons {
				if s, ok := r.(string); ok {
					msgs = append(msgs, s)
				}
			}
			decision.Allowed = false
			decision.Reason = strings.Join(msgs, "; ")
			return decision
		}
	}

	// Check allow.
	if allow, ok := result["allow"]; ok {
		if b, ok := allow.(bool); ok {
			decision.Allowed = b
		}
	}

	if !decision.Allowed {
		decision.Reason = "policy_default_deny"
		return decision
	}

	// Check require_approval.
	if ra, ok := result["require_approval"]; ok {
		if b, ok := ra.(bool); ok && b {
			decision.Allowed = false
			decision.RequiresApproval = true
			decision.Reason = "policy_requires_approval"
		}
	}

	// Check receipt_required.
	if rr, ok := result["receipt_required"]; ok {
		if b, ok := rr.(bool); ok {
			decision.ReceiptRequired = b
		}
	}

	// Copy constraints.
	if constraints, ok := result["constraints"]; ok {
		if m, ok := constraints.(map[string]any); ok {
			decision.Constraints = m
		}
	}

	if decision.Reason == "" {
		decision.Reason = "policy_allow"
	}

	return decision
}

// CanonicalizeInput produces deterministic JSON encoding of a PolicyInput.
// Keys are sorted alphabetically for stable hashing and reproducible OPA evaluation.
func CanonicalizeInput(input PolicyInput) (json.RawMessage, error) {
	// Marshal to map for sorted key output.
	raw, err := json.Marshal(input)
	if err != nil {
		return nil, err
	}

	var m map[string]any
	if err := json.Unmarshal(raw, &m); err != nil {
		return nil, err
	}

	return marshalSorted(m)
}

// marshalSorted produces JSON with keys sorted alphabetically at all levels.
func marshalSorted(v any) ([]byte, error) {
	switch val := v.(type) {
	case map[string]any:
		keys := make([]string, 0, len(val))
		for k := range val {
			keys = append(keys, k)
		}
		sort.Strings(keys)

		var buf bytes.Buffer
		buf.WriteByte('{')
		for i, k := range keys {
			if i > 0 {
				buf.WriteByte(',')
			}
			keyBytes, err := json.Marshal(k)
			if err != nil {
				return nil, err
			}
			buf.Write(keyBytes)
			buf.WriteByte(':')
			valBytes, err := marshalSorted(val[k])
			if err != nil {
				return nil, err
			}
			buf.Write(valBytes)
		}
		buf.WriteByte('}')
		return buf.Bytes(), nil
	case []any:
		var buf bytes.Buffer
		buf.WriteByte('[')
		for i, item := range val {
			if i > 0 {
				buf.WriteByte(',')
			}
			itemBytes, err := marshalSorted(item)
			if err != nil {
				return nil, err
			}
			buf.Write(itemBytes)
		}
		buf.WriteByte(']')
		return buf.Bytes(), nil
	default:
		return json.Marshal(v)
	}
}

// CircuitBreaker returns the circuit breaker state (for testing/monitoring).
func (c *OPAClient) CircuitBreaker() *circuitState {
	return c.circuit
}

// Health checks if OPA is reachable by querying /health.
func (c *OPAClient) Health(ctx context.Context) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.baseURL+"/health", nil)
	if err != nil {
		return err
	}
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("opa health check: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("opa health check: status %d", resp.StatusCode)
	}
	return nil
}
