package call

import (
	"bytes"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func computeHMAC(body []byte, secret string) string {
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write(body)
	return hex.EncodeToString(mac.Sum(nil))
}

func TestValidateSignature(t *testing.T) {
	t.Parallel()

	wh := NewWebhookHandler("my-secret", nil)

	body := []byte(`{"type":"call.started","call_id":"123"}`)
	sig := computeHMAC(body, "my-secret")

	if !wh.ValidateSignature(body, sig) {
		t.Fatal("expected valid signature")
	}

	if wh.ValidateSignature(body, "invalid-signature") {
		t.Fatal("expected invalid signature to fail")
	}

	if wh.ValidateSignature(body, "") {
		t.Fatal("expected empty signature to fail")
	}
}

func TestValidateSignatureEmptySecret(t *testing.T) {
	t.Parallel()

	wh := NewWebhookHandler("", nil)
	body := []byte(`{"test":true}`)
	if wh.ValidateSignature(body, "any-sig") {
		t.Fatal("expected failure when secret is empty")
	}
}

func TestHandleWebhookCallStarted(t *testing.T) {
	t.Parallel()

	secret := "test-secret"
	primary := &mockProvider{
		name:       "p",
		createResp: &CallResponse{CallID: "prov-event-1", Status: "queued"},
	}
	svc := NewCallService(primary, nil)
	svc.InitiateCall(nil, "ws1", CallRequest{PhoneNumber: "+1", CallType: "reservation"})

	wh := NewWebhookHandler(secret, svc)

	event := WebhookEvent{EventType: "call.started", CallID: "prov-event-1"}
	body, _ := json.Marshal(event)
	sig := computeHMAC(body, secret)

	req := httptest.NewRequest(http.MethodPost, "/webhook", bytes.NewReader(body))
	req.Header.Set("X-Vapi-Signature", sig)
	rec := httptest.NewRecorder()

	wh.HandleWebhook(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
}

func TestHandleWebhookInvalidSignature(t *testing.T) {
	t.Parallel()

	wh := NewWebhookHandler("secret", nil)

	body := []byte(`{"type":"call.started","call_id":"123"}`)
	req := httptest.NewRequest(http.MethodPost, "/webhook", bytes.NewReader(body))
	req.Header.Set("X-Vapi-Signature", "wrong-sig")
	rec := httptest.NewRecorder()

	wh.HandleWebhook(rec, req)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", rec.Code)
	}
}

func TestHandleWebhookInvalidJSON(t *testing.T) {
	t.Parallel()

	secret := "secret"
	body := []byte(`not valid json`)
	sig := computeHMAC(body, secret)

	wh := NewWebhookHandler(secret, nil)
	req := httptest.NewRequest(http.MethodPost, "/webhook", bytes.NewReader(body))
	req.Header.Set("X-Vapi-Signature", sig)
	rec := httptest.NewRecorder()

	wh.HandleWebhook(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", rec.Code)
	}
}
