package canvas

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestValidateWSAuth_TokenNotConfigured(t *testing.T) {
	svc := &Service{wsToken: ""}
	r := httptest.NewRequest(http.MethodGet, "/v1/canvas/ws", nil)
	if !svc.validateWSAuth(r) {
		t.Fatal("expected all connections allowed when CANVAS_WS_TOKEN is not set")
	}
}

func TestValidateWSAuth_ValidBearerToken(t *testing.T) {
	svc := &Service{wsToken: "secret-token-123"}
	r := httptest.NewRequest(http.MethodGet, "/v1/canvas/ws", nil)
	r.Header.Set("Authorization", "Bearer secret-token-123")
	if !svc.validateWSAuth(r) {
		t.Fatal("expected valid Bearer token to be accepted")
	}
}

func TestValidateWSAuth_ValidQueryToken(t *testing.T) {
	svc := &Service{wsToken: "secret-token-123"}
	r := httptest.NewRequest(http.MethodGet, "/v1/canvas/ws?token=secret-token-123", nil)
	if !svc.validateWSAuth(r) {
		t.Fatal("expected valid query token to be accepted")
	}
}

func TestValidateWSAuth_MissingToken(t *testing.T) {
	svc := &Service{wsToken: "secret-token-123"}
	r := httptest.NewRequest(http.MethodGet, "/v1/canvas/ws", nil)
	if svc.validateWSAuth(r) {
		t.Fatal("expected missing token to be rejected")
	}
}

func TestValidateWSAuth_WrongToken(t *testing.T) {
	svc := &Service{wsToken: "secret-token-123"}
	r := httptest.NewRequest(http.MethodGet, "/v1/canvas/ws", nil)
	r.Header.Set("Authorization", "Bearer wrong-token")
	if svc.validateWSAuth(r) {
		t.Fatal("expected wrong token to be rejected")
	}
}

func TestValidateWSAuth_WrongQueryToken(t *testing.T) {
	svc := &Service{wsToken: "secret-token-123"}
	r := httptest.NewRequest(http.MethodGet, "/v1/canvas/ws?token=wrong-token", nil)
	if svc.validateWSAuth(r) {
		t.Fatal("expected wrong query token to be rejected")
	}
}

func TestHandleWebSocket_RejectsUnauthorized(t *testing.T) {
	svc := &Service{wsToken: "secret-token-123"}
	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "/v1/canvas/ws", nil)
	svc.HandleWebSocket(w, r)
	if w.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", w.Code)
	}
}

func TestHandleWebSocket_AllowsWhenNoToken(t *testing.T) {
	// When wsToken is empty, validateWSAuth passes. The request will then fail
	// at the WebSocket upgrade (not a real WS handshake), but the point is it
	// should NOT return 401.
	svc := &Service{wsToken: ""}
	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "/v1/canvas/ws", nil)
	svc.HandleWebSocket(w, r)
	if w.Code == http.StatusUnauthorized {
		t.Fatal("expected no 401 when CANVAS_WS_TOKEN is not configured")
	}
}
