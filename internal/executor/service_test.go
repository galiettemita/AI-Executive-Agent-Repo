package executor

import (
	"testing"
	"time"
)

func TestSimulateHasNoSideEffects(t *testing.T) {
	t.Parallel()

	svc := NewService()
	_, err := svc.Simulate(ExecutionRequest{WorkspaceID: "ws1", ToolKey: "gmail.send", Action: "send email", TargetURL: "https://api.example.com"})
	if err != nil {
		t.Fatalf("simulate failed: %v", err)
	}
	if got := svc.SideEffectCount("ws1", "gmail.send"); got != 0 {
		t.Fatalf("expected no side effects, got %d", got)
	}
}

func TestCommitCreatesSideEffectTrustReceiptAndAudit(t *testing.T) {
	t.Parallel()

	svc := NewService()
	_, _, err := svc.Commit(ExecutionRequest{WorkspaceID: "ws1", ToolKey: "gmail.send", Action: "send email", TargetURL: "https://api.example.com"})
	if err != nil {
		t.Fatalf("commit failed: %v", err)
	}
	if got := svc.SideEffectCount("ws1", "gmail.send"); got != 1 {
		t.Fatalf("expected 1 side effect, got %d", got)
	}
	if svc.TrustReceiptCount() != 1 {
		t.Fatalf("expected trust receipt")
	}
	if svc.AuditCount() == 0 {
		t.Fatalf("expected audit log entries")
	}
}

func TestSSRFBlocked(t *testing.T) {
	t.Parallel()

	svc := NewService()
	_, err := svc.Simulate(ExecutionRequest{WorkspaceID: "ws1", ToolKey: "web.fetch", Action: "fetch", TargetURL: "http://169.254.169.254/latest/meta-data"})
	if err == nil {
		t.Fatal("expected ssrf block error")
	}
}

func TestCircuitBreakerOpensAfter5Failures(t *testing.T) {
	t.Parallel()

	svc := NewService()
	now := time.Now().UTC()
	svc.nowFunc = func() time.Time { return now }

	for i := 0; i < 5; i++ {
		svc.RecordProviderFailure("ws1", "providerA")
	}
	if !svc.CircuitOpen("ws1", "providerA") {
		t.Fatal("expected circuit to be open")
	}

	now = now.Add(301 * time.Second)
	if svc.CircuitOpen("ws1", "providerA") {
		t.Fatal("expected circuit to transition to closed after cooldown")
	}
}
