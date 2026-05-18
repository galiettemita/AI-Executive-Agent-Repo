package router

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	runtimeserver "github.com/brevio/brevio/internal/runtime"
)

func newTestService() (*Service, *http.ServeMux) {
	logger := runtimeserver.NewJSONLogger("router-test", "test")
	svc := NewService(Config{DatabaseURL: "postgres://test", RedisURL: "redis://test"}, logger)
	mux := http.NewServeMux()
	svc.RegisterRoutes(mux)
	return svc, mux
}

func TestRouterServiceRouteRegistration(t *testing.T) {
	t.Parallel()
	_, mux := newTestService()

	routes := []struct {
		method string
		path   string
		status int
	}{
		{"POST", "/api/v1/routing/select", http.StatusOK},
		{"POST", "/api/v1/routing/classify", http.StatusOK},
		{"GET", "/api/v1/routing/models", http.StatusOK},
		{"POST", "/api/v1/routing/models", http.StatusCreated},
		{"PUT", "/api/v1/routing/models/m1", http.StatusOK},
		{"GET", "/api/v1/routing/rules", http.StatusOK},
		{"POST", "/api/v1/routing/rules", http.StatusCreated},
		{"PUT", "/api/v1/routing/rules/r1", http.StatusOK},
		{"DELETE", "/api/v1/routing/rules/r1", http.StatusNoContent},
		{"GET", "/api/v1/routing/decisions", http.StatusOK},
		{"GET", "/api/v1/routing/decisions/stats", http.StatusOK},
		{"GET", "/api/v1/routing/preferences", http.StatusOK},
		{"PUT", "/api/v1/routing/preferences", http.StatusOK},
		{"GET", "/api/v1/routing/health", http.StatusOK},
		{"GET", "/api/v1/routing/costs", http.StatusOK},
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

func TestRouterSelectModelReturnsExpectedFields(t *testing.T) {
	t.Parallel()
	_, mux := newTestService()

	req := httptest.NewRequest("POST", "/api/v1/routing/select", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	var body map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode: %v", err)
	}
	for _, key := range []string{"selected_model", "provider", "reason"} {
		if _, ok := body[key]; !ok {
			t.Fatalf("missing key %s in select response", key)
		}
	}
}

func TestRouterClassifyReturnsComplexity(t *testing.T) {
	t.Parallel()
	_, mux := newTestService()

	req := httptest.NewRequest("POST", "/api/v1/routing/classify", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	var body map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if _, ok := body["complexity"]; !ok {
		t.Fatal("missing complexity in classify response")
	}
	if _, ok := body["score"]; !ok {
		t.Fatal("missing score in classify response")
	}
}

func TestRouterJSONContentType(t *testing.T) {
	t.Parallel()
	_, mux := newTestService()

	req := httptest.NewRequest("GET", "/api/v1/routing/models", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	ct := rec.Header().Get("Content-Type")
	if !strings.Contains(ct, "application/json") {
		t.Fatalf("expected application/json, got %s", ct)
	}
}

func TestRouterListEndpointsReturnArrays(t *testing.T) {
	t.Parallel()
	_, mux := newTestService()

	listEndpoints := []struct {
		path     string
		arrayKey string
	}{
		{"/api/v1/routing/models", "models"},
		{"/api/v1/routing/rules", "rules"},
		{"/api/v1/routing/decisions", "decisions"},
		{"/api/v1/routing/health", "providers"},
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

func TestRouterDeleteRuleReturnsNoContent(t *testing.T) {
	t.Parallel()
	_, mux := newTestService()

	req := httptest.NewRequest("DELETE", "/api/v1/routing/rules/rule-1", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusNoContent {
		t.Fatalf("expected 204, got %d", rec.Code)
	}
}

func TestRouterCostSummaryReturnsMap(t *testing.T) {
	t.Parallel()
	_, mux := newTestService()

	req := httptest.NewRequest("GET", "/api/v1/routing/costs", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	var body map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if _, ok := body["costs"]; !ok {
		t.Fatal("missing costs key")
	}
}

func TestRouterNewServiceNotNil(t *testing.T) {
	t.Parallel()
	logger := runtimeserver.NewJSONLogger("test", "test")
	svc := NewService(Config{}, logger)
	if svc == nil {
		t.Fatal("NewService returned nil")
	}
}
