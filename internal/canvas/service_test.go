package canvas

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gorilla/websocket"
)

func TestWebSocketConnectionAndToolInjection(t *testing.T) {
	t.Parallel()

	injector := &InMemoryInjector{}
	svc := NewService(injector)
	server := httptest.NewServer(NewMux(svc))
	defer server.Close()

	wsURL := "ws" + strings.TrimPrefix(server.URL, "http") + "/v1/canvas/ws?session_id=s1"
	conn, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
	if err != nil {
		t.Fatalf("dial websocket: %v", err)
	}
	defer conn.Close()

	message := `{"type":"click","payload":"open task panel"}`
	if err := conn.WriteMessage(websocket.TextMessage, []byte(message)); err != nil {
		t.Fatalf("write message: %v", err)
	}

	_, _, err = conn.ReadMessage()
	if err != nil {
		t.Fatalf("read ack: %v", err)
	}

	if injector.Count() != 1 {
		t.Fatalf("expected 1 injected tool call, got %d", injector.Count())
	}
	if svc.InteractionCount() != 1 {
		t.Fatalf("expected interaction log count 1, got %d", svc.InteractionCount())
	}
}

func TestA2UISurfaceEndpoint(t *testing.T) {
	t.Parallel()

	svc := NewService(&InMemoryInjector{})
	mux := NewMux(svc)
	req := httptest.NewRequest(http.MethodGet, "/v1/canvas/surfaces/mission_control", nil)
	resp := httptest.NewRecorder()
	mux.ServeHTTP(resp, req)
	if resp.Code != http.StatusOK {
		t.Fatalf("unexpected status: %d", resp.Code)
	}
	var payload map[string]any
	if err := json.Unmarshal(resp.Body.Bytes(), &payload); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if payload["surface_id"] != "mission_control" {
		t.Fatalf("unexpected surface_id: %v", payload["surface_id"])
	}
}

func TestCanvasFetchSSRFBlocked(t *testing.T) {
	t.Parallel()

	svc := NewService(&InMemoryInjector{})
	mux := NewMux(svc)

	blockedReq := httptest.NewRequest(http.MethodPost, "/v1/canvas/fetch", bytes.NewReader([]byte(`{"url":"http://169.254.169.254/latest/meta-data"}`)))
	blockedResp := httptest.NewRecorder()
	mux.ServeHTTP(blockedResp, blockedReq)
	if blockedResp.Code != http.StatusForbidden {
		t.Fatalf("expected ssrf blocked status 403, got %d", blockedResp.Code)
	}

	allowedReq := httptest.NewRequest(http.MethodPost, "/v1/canvas/fetch", bytes.NewReader([]byte(`{"url":"https://example.com/docs"}`)))
	allowedResp := httptest.NewRecorder()
	mux.ServeHTTP(allowedResp, allowedReq)
	if allowedResp.Code != http.StatusOK {
		t.Fatalf("expected allowed fetch status 200, got %d", allowedResp.Code)
	}
}
