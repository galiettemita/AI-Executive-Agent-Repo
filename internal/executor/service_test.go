package executor

import (
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
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

func TestEmitSynthesisEvidenceCreatesReceiptAndAudit(t *testing.T) {
	t.Parallel()

	svc := NewService()
	receipt, items, err := svc.EmitSynthesisEvidence("ws1", "turn_1", []string{"source:doc_1", "source:doc_2"})
	if err != nil {
		t.Fatalf("emit synthesis evidence: %v", err)
	}
	if receipt.ID == uuid.Nil {
		t.Fatal("expected synthesis receipt id")
	}
	if len(items) != 2 {
		t.Fatalf("expected 2 synthesis evidence items, got %d", len(items))
	}
	if svc.SynthesisReceiptCount() != 1 {
		t.Fatalf("expected synthesis receipt count=1, got %d", svc.SynthesisReceiptCount())
	}
	assertAuditHasEvent(t, svc.AuditEntries(), "BREVIO.trust.synthesis_evidence.created.v1")
}

func TestCacheLayerPromotion(t *testing.T) {
	t.Parallel()

	svc := NewService()
	svc.CachePut("key1", "value1")
	delete(svc.l1Cache, "key1")

	value, hit, layer := svc.CacheGet("key1")
	if !hit || value != "value1" || layer != "L2" {
		t.Fatalf("expected L2 hit with value1, got hit=%v value=%s layer=%s", hit, value, layer)
	}

	delete(svc.l1Cache, "key1")
	delete(svc.l2Cache, "key1")
	value, hit, layer = svc.CacheGet("key1")
	if !hit || value != "value1" || layer != "L3" {
		t.Fatalf("expected L3 hit with value1, got hit=%v value=%s layer=%s", hit, value, layer)
	}
}

func TestExecuteWithCircuitFallbackWhenOpen(t *testing.T) {
	t.Parallel()

	svc := NewService()
	now := time.Now().UTC()
	svc.nowFunc = func() time.Time { return now }
	for i := 0; i < 5; i++ {
		svc.RecordProviderFailure("ws1", "providerA")
	}

	status, err := svc.ExecuteWithCircuitProtection(ExecutionRequest{
		WorkspaceID: "ws1",
		ToolKey:     "gmail.send",
		Action:      "send",
		Provider:    "providerA",
		TargetURL:   "https://api.example.com",
	})
	if err != nil {
		t.Fatalf("execute with circuit protection: %v", err)
	}
	if status != "fallback_response" {
		t.Fatalf("expected fallback response when circuit open, got %s", status)
	}
}

func TestAuditPayloadMinimization(t *testing.T) {
	t.Parallel()

	svc := NewService()
	svc.emitAudit("custom.event", "Bearer secret_token user@example.com")
	entries := svc.AuditEntries()
	if len(entries) != 1 {
		t.Fatalf("expected one audit entry, got %d", len(entries))
	}
	if entries[0].Payload == "Bearer secret_token user@example.com" {
		t.Fatal("expected payload to be minimized")
	}
	if contains(entries[0].Payload, "example.com") {
		t.Fatalf("expected email redaction in payload: %s", entries[0].Payload)
	}
}

func contains(s, sub string) bool {
	return strings.Contains(s, sub)
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
