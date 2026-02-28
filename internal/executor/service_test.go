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
	assertAuditHasEvent(t, svc.AuditEntries(), "BREVIO.hands.tool.simulated.v1")
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
	assertAuditHasEvent(t, svc.AuditEntries(), "BREVIO.hands.tool.committed.v1")
	assertAuditHasEvent(t, svc.AuditEntries(), "BREVIO.trust.receipt.created.v1")
	assertAuditHasEvent(t, svc.AuditEntries(), "BREVIO.trust.evidence.attached.v1")
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

func TestCommitIdempotencyAvoidsDuplicateSideEffectsAndReceipts(t *testing.T) {
	t.Parallel()

	svc := NewService()
	req := ExecutionRequest{
		WorkspaceID: "ws_idem",
		ToolKey:     "gmail.send",
		Action:      "send email",
		TargetURL:   "https://api.example.com",
	}

	firstExec, firstReceipt, err := svc.Commit(req)
	if err != nil {
		t.Fatalf("first commit failed: %v", err)
	}
	secondExec, secondReceipt, err := svc.Commit(req)
	if err != nil {
		t.Fatalf("second commit failed: %v", err)
	}

	if firstExec.ID != secondExec.ID {
		t.Fatalf("expected idempotent execution id reuse, got %s vs %s", firstExec.ID, secondExec.ID)
	}
	if firstReceipt.ID != secondReceipt.ID {
		t.Fatalf("expected idempotent receipt id reuse, got %s vs %s", firstReceipt.ID, secondReceipt.ID)
	}
	if got := svc.SideEffectCount("ws_idem", "gmail.send"); got != 1 {
		t.Fatalf("expected one side effect for idempotent commit, got %d", got)
	}
	if got := svc.TrustReceiptCount(); got != 1 {
		t.Fatalf("expected one trust receipt for idempotent commit, got %d", got)
	}
}

func assertAuditHasEvent(t *testing.T, entries []AuditLogEntry, eventType string) {
	t.Helper()
	for _, entry := range entries {
		if entry.EventType == eventType {
			return
		}
	}
	t.Fatalf("expected audit event %s in entries=%v", eventType, entries)
}
