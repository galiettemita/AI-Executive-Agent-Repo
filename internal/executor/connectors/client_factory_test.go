package connectors

import (
	"context"
	"testing"
)

func TestConnectorFactoryAndRegistry(t *testing.T) {
	t.Parallel()

	registry := NewRegistry()
	registry.Upsert(Definition{ConnectorKey: "google_calendar", Provider: "google", AuthType: "oauth"})
	if _, ok := registry.Get("google_calendar"); !ok {
		t.Fatal("expected connector definition in registry")
	}

	factory := NewClientFactory(registry)
	if err := factory.RegisterClient("google_calendar", NoopClient{}); err != nil {
		t.Fatalf("register client: %v", err)
	}
	client, err := factory.Resolve("google_calendar")
	if err != nil {
		t.Fatalf("resolve client: %v", err)
	}
	simulated, err := client.Simulate(context.Background(), ToolCallRequest{ToolKey: "calendar.list_events"})
	if err != nil || !simulated.Success {
		t.Fatalf("simulate failed: err=%v result=%+v", err, simulated)
	}
}

func TestDefaultConnectorPolicies(t *testing.T) {
	t.Parallel()

	if retry := DefaultRetryPolicy(); retry.MaxAttempts != 5 || retry.BaseDelay.Seconds() != 1 {
		t.Fatalf("unexpected retry policy: %+v", retry)
	}
	if cb := DefaultCircuitBreakerPolicy(); cb.FailureThreshold != 5 || cb.HalfOpenAfter.Seconds() != 300 {
		t.Fatalf("unexpected circuit breaker policy: %+v", cb)
	}
	if timeout := DefaultTimeoutPolicy(); timeout.Default.Seconds() != 30 || timeout.FileOps.Seconds() != 120 || timeout.Health.Seconds() != 5 {
		t.Fatalf("unexpected timeout policy: %+v", timeout)
	}
}
