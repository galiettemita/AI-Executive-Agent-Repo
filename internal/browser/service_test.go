package browser

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	runtimeserver "github.com/brevio/brevio/internal/runtime"
)

func newTestService() (*Service, *http.ServeMux) {
	logger := runtimeserver.NewJSONLogger("browser-test", "test")
	svc := NewService(Config{DatabaseURL: "postgres://test", RedisURL: "redis://test"}, logger)
	mux := http.NewServeMux()
	svc.RegisterRoutes(mux)
	return svc, mux
}

func TestBrowserServiceRouteRegistration(t *testing.T) {
	t.Parallel()
	_, mux := newTestService()

	routes := []struct {
		method string
		path   string
		status int
	}{
		{"POST", "/api/v1/browser/sessions", http.StatusCreated},
		{"GET", "/api/v1/browser/sessions/test-id", http.StatusOK},
		{"DELETE", "/api/v1/browser/sessions/test-id", http.StatusNoContent},
		{"POST", "/api/v1/browser/sessions/s1/tasks", http.StatusCreated},
		{"GET", "/api/v1/browser/sessions/s1/tasks", http.StatusOK},
		{"GET", "/api/v1/browser/sessions/s1/tasks/t1", http.StatusOK},
		{"POST", "/api/v1/browser/scrape", http.StatusAccepted},
		{"POST", "/api/v1/browser/screenshot", http.StatusAccepted},
		{"POST", "/api/v1/browser/form-fill", http.StatusAccepted},
		{"GET", "/api/v1/browser/fingerprints", http.StatusOK},
		{"POST", "/api/v1/browser/fingerprints", http.StatusCreated},
		{"DELETE", "/api/v1/browser/fingerprints/fp1", http.StatusNoContent},
		{"GET", "/api/v1/browser/proxies", http.StatusOK},
		{"POST", "/api/v1/browser/proxies", http.StatusCreated},
		{"DELETE", "/api/v1/browser/proxies/px1", http.StatusNoContent},
		{"POST", "/api/v1/browser/captcha/solve", http.StatusAccepted},
		{"GET", "/api/v1/browser/cookies/example.com", http.StatusOK},
		{"POST", "/api/v1/browser/cookies", http.StatusCreated},
	}

	for _, tc := range routes {
		t.Run(tc.method+" "+tc.path, func(t *testing.T) {
			t.Parallel()
			req := httptest.NewRequest(tc.method, tc.path, nil)
			rec := httptest.NewRecorder()
			mux.ServeHTTP(rec, req)
			if rec.Code != tc.status {
				t.Fatalf("expected status %d, got %d", tc.status, rec.Code)
			}
		})
	}
}

func TestBrowserGetSessionReturnsID(t *testing.T) {
	t.Parallel()
	_, mux := newTestService()

	req := httptest.NewRequest("GET", "/api/v1/browser/sessions/abc-123", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	var body map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if body["id"] != "abc-123" {
		t.Fatalf("expected id abc-123, got %v", body["id"])
	}
}

func TestBrowserGetTaskReturnsTaskID(t *testing.T) {
	t.Parallel()
	_, mux := newTestService()

	req := httptest.NewRequest("GET", "/api/v1/browser/sessions/s1/tasks/task-456", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	var body map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if body["id"] != "task-456" {
		t.Fatalf("expected id task-456, got %v", body["id"])
	}
}

func TestBrowserJSONContentType(t *testing.T) {
	t.Parallel()
	_, mux := newTestService()

	req := httptest.NewRequest("GET", "/api/v1/browser/proxies", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	ct := rec.Header().Get("Content-Type")
	if !strings.Contains(ct, "application/json") {
		t.Fatalf("expected application/json content type, got %s", ct)
	}
}

func TestBrowserDeleteReturnsNoBody(t *testing.T) {
	t.Parallel()
	_, mux := newTestService()

	req := httptest.NewRequest("DELETE", "/api/v1/browser/sessions/del-1", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusNoContent {
		t.Fatalf("expected 204, got %d", rec.Code)
	}
	if rec.Body.Len() != 0 {
		t.Fatalf("expected empty body on 204, got %d bytes", rec.Body.Len())
	}
}

func TestBrowserNewServiceNotNil(t *testing.T) {
	t.Parallel()
	logger := runtimeserver.NewJSONLogger("test", "test")
	svc := NewService(Config{}, logger)
	if svc == nil {
		t.Fatal("NewService returned nil")
	}
}

func TestBrowserListEndpointsReturnArrays(t *testing.T) {
	t.Parallel()
	_, mux := newTestService()

	listEndpoints := []struct {
		path     string
		arrayKey string
	}{
		{"/api/v1/browser/sessions/s1/tasks", "tasks"},
		{"/api/v1/browser/fingerprints", "fingerprints"},
		{"/api/v1/browser/proxies", "proxies"},
		{"/api/v1/browser/cookies/example.com", "cookies"},
	}

	for _, tc := range listEndpoints {
		t.Run(tc.path, func(t *testing.T) {
			t.Parallel()
			req := httptest.NewRequest("GET", tc.path, nil)
			rec := httptest.NewRecorder()
			mux.ServeHTTP(rec, req)

			var body map[string]any
			if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
				t.Fatalf("decode: %v", err)
			}
			arr, ok := body[tc.arrayKey].([]any)
			if !ok {
				t.Fatalf("expected %s to be array, got %T", tc.arrayKey, body[tc.arrayKey])
			}
			if arr == nil {
				t.Fatalf("expected non-nil array for %s", tc.arrayKey)
			}
		})
	}
}
