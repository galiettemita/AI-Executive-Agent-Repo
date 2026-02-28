package gateway

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/google/uuid"
)

func signedRequestBody(secret string, payload []byte) *http.Request {
	req := httptest.NewRequest(http.MethodPost, "/v1/gateway/webhook/whatsapp", bytes.NewReader(payload))
	req.Header.Set("X-Signature", signatureFor([]byte(secret), payload))
	return req
}

func TestInvalidSignatureReturns401AndAuditEntry(t *testing.T) {
	t.Parallel()

	svc := NewService("test-secret")
	svc.router.Bind("whatsapp", "+15550001111", uuid.MustParse("018f3f6a-9a0f-7cc6-8f2f-1f0f2d2f2d2f"))
	payload := []byte(`{"channel":"whatsapp","channel_identifier":"+15550001111","user_channel_id":"u1","nonce":"n1","message":"hello"}`)

	req := httptest.NewRequest(http.MethodPost, "/v1/gateway/webhook/whatsapp", bytes.NewReader(payload))
	req.Header.Set("X-Signature", "deadbeef")
	resp := httptest.NewRecorder()

	svc.HandleInbound(resp, req)
	if resp.Code != http.StatusUnauthorized {
		t.Fatalf("unexpected status: %d", resp.Code)
	}
	if svc.audit.Count() != 1 {
		t.Fatalf("expected 1 audit entry, got %d", svc.audit.Count())
	}
	if !containsString(svc.AuditEntries(), "BREVIO.security.webhook.signature_invalid.v1") {
		t.Fatalf("expected invalid signature event in audit log, got %v", svc.AuditEntries())
	}
}

func TestReplayNonceRejected(t *testing.T) {
	t.Parallel()

	svc := NewService("test-secret")
	svc.router.Bind("whatsapp", "+15550001111", uuid.MustParse("018f3f6a-9a0f-7cc6-8f2f-1f0f2d2f2d2f"))
	payload := []byte(`{"channel":"whatsapp","channel_identifier":"+15550001111","user_channel_id":"u1","nonce":"n1","message":"hello"}`)

	resp1 := httptest.NewRecorder()
	svc.HandleInbound(resp1, signedRequestBody("test-secret", payload))
	if resp1.Code != http.StatusAccepted {
		t.Fatalf("unexpected initial status: %d", resp1.Code)
	}

	resp2 := httptest.NewRecorder()
	svc.HandleInbound(resp2, signedRequestBody("test-secret", payload))
	if resp2.Code != http.StatusConflict {
		t.Fatalf("expected replay conflict, got %d", resp2.Code)
	}
	if !containsString(svc.AuditEntries(), "BREVIO.security.webhook.replay_blocked.v1") {
		t.Fatalf("expected replay-blocked event in audit log, got %v", svc.AuditEntries())
	}
}

func TestValidMessageCreatesIngressAndQueue(t *testing.T) {
	t.Parallel()

	svc := NewService("test-secret")
	svc.router.Bind("whatsapp", "+15550001111", uuid.MustParse("018f3f6a-9a0f-7cc6-8f2f-1f0f2d2f2d2f"))
	payload := []byte(`{"channel":"whatsapp","channel_identifier":"+15550001111","user_channel_id":"u1","nonce":"n2","message":"hello"}`)

	resp := httptest.NewRecorder()
	svc.HandleInbound(resp, signedRequestBody("test-secret", payload))
	if resp.Code != http.StatusAccepted {
		t.Fatalf("unexpected status: %d", resp.Code)
	}
	if svc.store.TurnCount() != 1 {
		t.Fatalf("expected 1 ingress turn, got %d", svc.store.TurnCount())
	}
	if svc.queue.Count() != 1 {
		t.Fatalf("expected 1 queue message, got %d", svc.queue.Count())
	}
	if !containsString(svc.AuditEntries(), "BREVIO.ingress.received.v1") {
		t.Fatalf("expected ingress received event in audit log, got %v", svc.AuditEntries())
	}
}

func TestRateLimitedUserGets429(t *testing.T) {
	t.Parallel()

	svc := NewService("test-secret")
	svc.router.Bind("whatsapp", "+15550001111", uuid.MustParse("018f3f6a-9a0f-7cc6-8f2f-1f0f2d2f2d2f"))

	for i := 0; i < 5; i++ {
		payload := []byte(`{"channel":"whatsapp","channel_identifier":"+15550001111","user_channel_id":"u1","nonce":"n` + string(rune('a'+i)) + `","message":"hello"}`)
		resp := httptest.NewRecorder()
		svc.HandleInbound(resp, signedRequestBody("test-secret", payload))
		if resp.Code != http.StatusAccepted {
			t.Fatalf("unexpected status at iteration %d: %d", i, resp.Code)
		}
	}

	payload := []byte(`{"channel":"whatsapp","channel_identifier":"+15550001111","user_channel_id":"u1","nonce":"n_limit","message":"hello"}`)
	resp := httptest.NewRecorder()
	svc.HandleInbound(resp, signedRequestBody("test-secret", payload))
	if resp.Code != http.StatusTooManyRequests {
		t.Fatalf("expected 429, got %d", resp.Code)
	}
}

func TestInjectToolCallAccepted(t *testing.T) {
	t.Parallel()

	svc := NewService("test-secret")
	mux := NewMux(svc)

	payload := []byte(`{"workspace_id":"018f3f6a-9a0f-7cc6-8f2f-1f0f2d2f2d2f","ingress_turn_id":"018f3f6a-9a0f-7cc6-8f2f-1f0f2d2f2d2f","tool_key":"calendar.create_event","arguments":{"title":"Standup"}}`)
	req := httptest.NewRequest(http.MethodPost, "/v1/gateway/inject/tool_call", bytes.NewReader(payload))
	resp := httptest.NewRecorder()

	mux.ServeHTTP(resp, req)
	if resp.Code != http.StatusAccepted {
		t.Fatalf("unexpected status: %d", resp.Code)
	}
	if svc.InjectedToolCallCount() != 1 {
		t.Fatalf("expected one injected tool call, got %d", svc.InjectedToolCallCount())
	}
}

func TestDuplicateIngressDropsWithCanonicalEvent(t *testing.T) {
	t.Parallel()

	svc := NewService("test-secret")
	workspaceID := uuid.MustParse("018f3f6a-9a0f-7cc6-8f2f-1f0f2d2f2d2f")
	svc.router.Bind("whatsapp", "+15550001111", workspaceID)

	payload := []byte(`{"channel":"whatsapp","channel_identifier":"+15550001111","user_channel_id":"u1","nonce":"n_dedup","message":"hello"}`)
	svc.store.InsertIngressTurn(IngressTurn{
		ID:            uuid.MustParse("018f3f6a-9a0f-7cc6-8f2f-1f0f2d2f2d2e"),
		WorkspaceID:   workspaceID,
		UserChannelID: "u1",
		DedupHash:     dedupHash(payload),
		Payload:       payload,
		CreatedAt:     time.Now().UTC(),
	})

	resp := httptest.NewRecorder()
	svc.HandleInbound(resp, signedRequestBody("test-secret", payload))
	if resp.Code != http.StatusOK {
		t.Fatalf("expected dedup drop status 200, got %d", resp.Code)
	}
	if !containsString(svc.AuditEntries(), "BREVIO.ingress.duplicate_dropped.v1") {
		t.Fatalf("expected duplicate-dropped event in audit log, got %v", svc.AuditEntries())
	}
}

func TestHealthEndpoints(t *testing.T) {
	t.Parallel()

	svc := NewService("test-secret")
	mux := NewMux(svc)

	for _, path := range []string{"/healthz/ready", "/healthz/live"} {
		req := httptest.NewRequest(http.MethodGet, path, nil)
		resp := httptest.NewRecorder()
		mux.ServeHTTP(resp, req)
		if resp.Code != http.StatusOK {
			t.Fatalf("unexpected status for %s: %d", path, resp.Code)
		}
	}
}

func containsString(items []string, needle string) bool {
	for _, item := range items {
		if item == needle {
			return true
		}
	}
	return false
}
