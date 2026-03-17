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

func TestCheckOriginAllowlist(t *testing.T) {
	cases := []struct {
		name      string
		env       string
		allowlist string
		origin    string
		wantAllow bool
	}{
		{"local_open", "local", "", "http://evil.com", true},
		{"prod_no_list_deny", "production", "", "http://evil.com", false},
		{"prod_match", "production", "https://app.brevio.ai", "https://app.brevio.ai", true},
		{"prod_mismatch", "production", "https://app.brevio.ai", "http://evil.com", false},
		{"prod_multi_match", "production", "https://a.com,https://b.com", "https://b.com", true},
		{"blank_env_open", "", "", "http://any.com", true},
		{"prod_no_origin_same_origin", "production", "https://app.brevio.ai", "", true},
		{"test_env_open", "test", "", "http://evil.com", true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Setenv("BREVIO_ENV", tc.env)
			t.Setenv("CANVAS_ALLOWED_ORIGINS", tc.allowlist)

			svc := NewService(&InMemoryInjector{})
			req, _ := http.NewRequest(http.MethodGet, "/ws", nil)
			if tc.origin != "" {
				req.Header.Set("Origin", tc.origin)
			}
			got := svc.upgrader.CheckOrigin(req)
			if got != tc.wantAllow {
				t.Errorf("env=%q allowlist=%q origin=%q: got %v want %v",
					tc.env, tc.allowlist, tc.origin, got, tc.wantAllow)
			}
		})
	}
}

func TestParseAllowedOrigins(t *testing.T) {
	cases := []struct {
		input string
		want  int
	}{
		{"", 0},
		{"https://a.com", 1},
		{"https://a.com,https://b.com", 2},
		{"https://a.com, https://b.com , https://c.com", 3},
		{",,,", 0},
	}
	for _, tc := range cases {
		got := parseAllowedOrigins(tc.input)
		if len(got) != tc.want {
			t.Errorf("parseAllowedOrigins(%q) = %d entries, want %d", tc.input, len(got), tc.want)
		}
	}
}

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
