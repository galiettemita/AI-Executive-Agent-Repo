package control

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestOPAClient_EvaluatePolicy_Allow(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("expected POST, got %s", r.Method)
		}
		if !strings.HasSuffix(r.URL.Path, "/v1/data/brevio/v9") {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(OPAResponse{
			Result: map[string]any{
				"allow": true,
				"deny":  []any{},
			},
		})
	}))
	defer srv.Close()

	client := NewOPAClient(OPAClientConfig{
		BaseURL:                 srv.URL,
		Timeout:                 2 * time.Second,
		MaxRetries:              0,
		CircuitBreakerThreshold: 5,
		CircuitBreakerCooldown:  30 * time.Second,
	})

	decision, err := client.EvaluatePolicy(context.Background(), "brevio.v9", PolicyInput{
		AutonomyLevel:          "A3",
		FirewallAllowed:        true,
		SemanticVerifierPassed: true,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !decision.Allowed {
		t.Errorf("expected allowed=true, got reason=%s", decision.Reason)
	}
}

func TestOPAClient_EvaluatePolicy_Deny(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(OPAResponse{
			Result: map[string]any{
				"allow": false,
				"deny":  []any{"budget_exhausted", "rate_limit_exceeded"},
			},
		})
	}))
	defer srv.Close()

	client := NewOPAClient(OPAClientConfig{
		BaseURL:                 srv.URL,
		Timeout:                 2 * time.Second,
		MaxRetries:              0,
		CircuitBreakerThreshold: 5,
		CircuitBreakerCooldown:  30 * time.Second,
	})

	decision, err := client.EvaluatePolicy(context.Background(), "brevio.v9", PolicyInput{
		BudgetExhausted: true,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if decision.Allowed {
		t.Fatal("expected denied")
	}
	if !strings.Contains(decision.Reason, "budget_exhausted") {
		t.Errorf("reason should contain budget_exhausted, got %q", decision.Reason)
	}
}

func TestOPAClient_EvaluatePolicy_RequireApproval(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(OPAResponse{
			Result: map[string]any{
				"allow":            true,
				"deny":             []any{},
				"require_approval": true,
			},
		})
	}))
	defer srv.Close()

	client := NewOPAClient(OPAClientConfig{
		BaseURL:                 srv.URL,
		Timeout:                 2 * time.Second,
		MaxRetries:              0,
		CircuitBreakerThreshold: 5,
		CircuitBreakerCooldown:  30 * time.Second,
	})

	decision, err := client.EvaluatePolicy(context.Background(), "brevio.v9", PolicyInput{
		IsWrite:       true,
		AutonomyLevel: "A1",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !decision.RequiresApproval {
		t.Error("expected requires_approval=true")
	}
}

func TestOPAClient_DenyByDefault_ServerUnavailable(t *testing.T) {
	t.Parallel()

	client := NewOPAClient(OPAClientConfig{
		BaseURL:                 "http://127.0.0.1:1", // nothing listening
		Timeout:                 100 * time.Millisecond,
		MaxRetries:              0,
		CircuitBreakerThreshold: 5,
		CircuitBreakerCooldown:  30 * time.Second,
	})

	decision, err := client.EvaluatePolicy(context.Background(), "brevio.v9", PolicyInput{})
	if err == nil {
		t.Fatal("expected error for unavailable server")
	}
	if decision.Allowed {
		t.Fatal("expected deny-by-default when OPA unavailable")
	}
	if decision.Reason != "opa_unavailable" {
		t.Errorf("got reason=%q want=%q", decision.Reason, "opa_unavailable")
	}
}

func TestOPAClient_DenyByDefault_Server500(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()

	client := NewOPAClient(OPAClientConfig{
		BaseURL:                 srv.URL,
		Timeout:                 2 * time.Second,
		MaxRetries:              1,
		RetryBackoff:            10 * time.Millisecond,
		CircuitBreakerThreshold: 5,
		CircuitBreakerCooldown:  30 * time.Second,
	})

	decision, err := client.EvaluatePolicy(context.Background(), "brevio.v9", PolicyInput{})
	if err == nil {
		t.Fatal("expected error for 500")
	}
	if decision.Allowed {
		t.Fatal("expected deny-by-default on server error")
	}
}

func TestOPAClient_CircuitBreaker_OpensAfterThreshold(t *testing.T) {
	t.Parallel()

	callCount := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		callCount++
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()

	client := NewOPAClient(OPAClientConfig{
		BaseURL:                 srv.URL,
		Timeout:                 2 * time.Second,
		MaxRetries:              0,
		CircuitBreakerThreshold: 3,
		CircuitBreakerCooldown:  1 * time.Hour, // won't expire during test
	})

	// Trip the circuit breaker.
	for i := 0; i < 3; i++ {
		client.EvaluatePolicy(context.Background(), "brevio.v9", PolicyInput{})
	}

	if !client.circuit.isOpen() {
		t.Fatal("circuit should be open after threshold failures")
	}

	// Next call should fail immediately without hitting server.
	beforeCount := callCount
	decision, err := client.EvaluatePolicy(context.Background(), "brevio.v9", PolicyInput{})
	if err == nil {
		t.Fatal("expected error with open circuit")
	}
	if decision.Reason != "opa_circuit_open" {
		t.Errorf("got reason=%q want=%q", decision.Reason, "opa_circuit_open")
	}
	if callCount != beforeCount {
		t.Error("server should not have been called with open circuit")
	}
}

func TestOPAClient_CircuitBreaker_ResetsOnSuccess(t *testing.T) {
	t.Parallel()

	failCount := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		failCount++
		if failCount <= 2 {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(OPAResponse{
			Result: map[string]any{"allow": true, "deny": []any{}},
		})
	}))
	defer srv.Close()

	client := NewOPAClient(OPAClientConfig{
		BaseURL:                 srv.URL,
		Timeout:                 2 * time.Second,
		MaxRetries:              0,
		CircuitBreakerThreshold: 5,
		CircuitBreakerCooldown:  30 * time.Second,
	})

	// Two failures.
	client.EvaluatePolicy(context.Background(), "brevio.v9", PolicyInput{})
	client.EvaluatePolicy(context.Background(), "brevio.v9", PolicyInput{})
	if client.circuit.ConsecutiveErrors() != 2 {
		t.Fatalf("expected 2 consecutive errors, got %d", client.circuit.ConsecutiveErrors())
	}

	// Success resets counter.
	decision, err := client.EvaluatePolicy(context.Background(), "brevio.v9", PolicyInput{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !decision.Allowed {
		t.Error("expected allowed after success")
	}
	if client.circuit.ConsecutiveErrors() != 0 {
		t.Errorf("expected 0 consecutive errors after success, got %d", client.circuit.ConsecutiveErrors())
	}
}

func TestOPAClient_RetryOnTransientError(t *testing.T) {
	t.Parallel()

	callCount := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		callCount++
		if callCount <= 1 {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(OPAResponse{
			Result: map[string]any{"allow": true, "deny": []any{}},
		})
	}))
	defer srv.Close()

	client := NewOPAClient(OPAClientConfig{
		BaseURL:                 srv.URL,
		Timeout:                 2 * time.Second,
		MaxRetries:              2,
		RetryBackoff:            5 * time.Millisecond,
		CircuitBreakerThreshold: 5,
		CircuitBreakerCooldown:  30 * time.Second,
	})

	decision, err := client.EvaluatePolicy(context.Background(), "brevio.v9", PolicyInput{})
	if err != nil {
		t.Fatalf("expected success after retry, got: %v", err)
	}
	if !decision.Allowed {
		t.Error("expected allowed after retry succeeds")
	}
	if callCount != 2 {
		t.Errorf("expected 2 calls (1 fail + 1 success), got %d", callCount)
	}
}

func TestOPAClient_EmptyResultDenies(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(OPAResponse{Result: nil})
	}))
	defer srv.Close()

	client := NewOPAClient(OPAClientConfig{
		BaseURL:                 srv.URL,
		Timeout:                 2 * time.Second,
		MaxRetries:              0,
		CircuitBreakerThreshold: 5,
		CircuitBreakerCooldown:  30 * time.Second,
	})

	decision, err := client.EvaluatePolicy(context.Background(), "brevio.v9", PolicyInput{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if decision.Allowed {
		t.Fatal("empty result should deny")
	}
	if decision.Reason != "opa_empty_result" {
		t.Errorf("got reason=%q want=%q", decision.Reason, "opa_empty_result")
	}
}

func TestOPAClient_Health(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/health" {
			w.WriteHeader(http.StatusOK)
			return
		}
		w.WriteHeader(http.StatusNotFound)
	}))
	defer srv.Close()

	client := NewOPAClient(OPAClientConfig{
		BaseURL:                 srv.URL,
		Timeout:                 2 * time.Second,
		CircuitBreakerThreshold: 5,
		CircuitBreakerCooldown:  30 * time.Second,
	})

	if err := client.Health(context.Background()); err != nil {
		t.Fatalf("health check should pass: %v", err)
	}
}

func TestCanonicalizeInput_Deterministic(t *testing.T) {
	t.Parallel()

	input := PolicyInput{
		AutonomyLevel:          "A3",
		ToolRiskLevel:          "LOW",
		IsWrite:                true,
		RateLimited:            false,
		BudgetExhausted:        false,
		FirewallAllowed:        true,
		SemanticVerifierPassed: true,
		BlockedTool:            false,
		WorkspacePlan:          "pro",
		Domain:                 "calendar",
		ToolKey:                "calendar_write",
		UserRole:               "admin",
		Timestamp:              1700000000,
	}

	result1, err := CanonicalizeInput(input)
	if err != nil {
		t.Fatal(err)
	}
	result2, err := CanonicalizeInput(input)
	if err != nil {
		t.Fatal(err)
	}

	if string(result1) != string(result2) {
		t.Fatalf("canonicalization not deterministic:\n  %s\n  %s", result1, result2)
	}

	// Verify keys are sorted.
	s := string(result1)
	aIdx := strings.Index(s, `"autonomy_level"`)
	bIdx := strings.Index(s, `"blocked_tool"`)
	if aIdx == -1 || bIdx == -1 || aIdx >= bIdx {
		t.Errorf("keys should be alphabetically sorted in output")
	}
}

func TestOPAEvaluator_DenyByDefault_WithOPAClient(t *testing.T) {
	t.Parallel()

	// OPA client pointing at unreachable server.
	client := NewOPAClient(OPAClientConfig{
		BaseURL:                 "http://127.0.0.1:1",
		Timeout:                 50 * time.Millisecond,
		MaxRetries:              0,
		CircuitBreakerThreshold: 5,
		CircuitBreakerCooldown:  30 * time.Second,
	})

	svc := NewService("test-secret")
	evaluator := NewOPAEvaluator(svc)
	evaluator.SetOPAClient(client)

	// With OPA client set, EvaluateGateWithOPA must deny-by-default.
	output := evaluator.EvaluateGateWithOPA(context.Background(), PolicyInput{
		FirewallAllowed:        true,
		SemanticVerifierPassed: true,
		IsWrite:                false,
	})
	if output.Decision != "deny" {
		t.Errorf("expected deny-by-default when OPA unavailable, got %q reason=%q", output.Decision, output.ReasonCode)
	}
	if output.ReasonCode != "OPA_UNAVAILABLE_DENY_BY_DEFAULT" {
		t.Errorf("got reason=%q want=%q", output.ReasonCode, "OPA_UNAVAILABLE_DENY_BY_DEFAULT")
	}
}

func TestOPAEvaluator_FallbackOnlyWithoutOPAClient(t *testing.T) {
	t.Parallel()

	svc := NewService("test-secret")
	evaluator := NewOPAEvaluator(svc)
	// No OPA client set — should use embedded gate logic.

	output := evaluator.EvaluateGateWithOPA(context.Background(), PolicyInput{
		FirewallAllowed:        true,
		SemanticVerifierPassed: true,
		IsWrite:                false,
	})
	if output.Decision != "allow" {
		t.Errorf("expected allow via embedded gates, got %q reason=%q", output.Decision, output.ReasonCode)
	}
}

func TestMapOPAResult_DefaultDeny(t *testing.T) {
	t.Parallel()

	decision := mapOPAResult(map[string]any{
		"allow": false,
	})
	if decision.Allowed {
		t.Fatal("expected deny by default")
	}
	if decision.Reason != "policy_default_deny" {
		t.Errorf("got reason=%q want=%q", decision.Reason, "policy_default_deny")
	}
}
