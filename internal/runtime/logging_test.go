package runtime

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestParseTraceparent(t *testing.T) {
	t.Parallel()

	traceID, spanID := parseTraceparent("00-4bf92f3577b34da6a3ce929d0e0e4736-00f067aa0ba902b7-01")
	if traceID != "4bf92f3577b34da6a3ce929d0e0e4736" {
		t.Fatalf("unexpected trace id: %s", traceID)
	}
	if spanID != "00f067aa0ba902b7" {
		t.Fatalf("unexpected span id: %s", spanID)
	}

	traceID, spanID = parseTraceparent("invalid")
	if traceID != "" || spanID != "" {
		t.Fatalf("expected empty trace/span for invalid header: trace=%s span=%s", traceID, spanID)
	}
}

func TestJSONLoggerMiddlewareLogsCorrelationFields(t *testing.T) {
	t.Parallel()

	logger := NewJSONLogger("gateway", "production")
	fixedNow := time.Date(2026, time.March, 3, 12, 0, 0, 0, time.UTC)
	logger.SetNowForTest(func() time.Time { return fixedNow })

	var out bytes.Buffer
	logger.SetOutput(&out)

	handler := logger.Middleware(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusCreated)
	}))

	req := httptest.NewRequest(http.MethodPost, "/webhooks/whatsapp", nil)
	req.Header.Set("traceparent", "00-4bf92f3577b34da6a3ce929d0e0e4736-00f067aa0ba902b7-01")
	req.Header.Set("X-User-Id", "user-123")
	req.Header.Set("X-Request-Id", "req-1")

	resp := httptest.NewRecorder()
	handler.ServeHTTP(resp, req)

	if resp.Code != http.StatusCreated {
		t.Fatalf("unexpected response code: %d", resp.Code)
	}
	if got := resp.Header().Get("X-Request-Id"); got != "req-1" {
		t.Fatalf("unexpected request id header: %s", got)
	}

	lines := bytes.Split(bytes.TrimSpace(out.Bytes()), []byte("\n"))
	if len(lines) != 1 {
		t.Fatalf("unexpected log line count: %d", len(lines))
	}

	var payload map[string]any
	if err := json.Unmarshal(lines[0], &payload); err != nil {
		t.Fatalf("decode log payload: %v", err)
	}
	if payload["service"] != "gateway" {
		t.Fatalf("unexpected service field: %v", payload["service"])
	}
	if payload["env"] != "production" {
		t.Fatalf("unexpected env field: %v", payload["env"])
	}
	if payload["trace_id"] != "4bf92f3577b34da6a3ce929d0e0e4736" {
		t.Fatalf("unexpected trace id: %v", payload["trace_id"])
	}
	if payload["span_id"] != "00f067aa0ba902b7" {
		t.Fatalf("unexpected span id: %v", payload["span_id"])
	}
	if payload["user_id"] != "user-123" {
		t.Fatalf("unexpected user id: %v", payload["user_id"])
	}
	if payload["event"] != "http_request" {
		t.Fatalf("unexpected event: %v", payload["event"])
	}
}
