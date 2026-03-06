package agents

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	runtimeserver "github.com/brevio/brevio/internal/runtime"
)

func newTestService() (*Service, *http.ServeMux) {
	logger := runtimeserver.NewJSONLogger("agents-test", "test")
	svc := NewService(Config{
		DatabaseURL:  "postgres://test",
		RedisURL:     "redis://test",
		TemporalHost: "localhost:7233",
	}, logger)
	mux := http.NewServeMux()
	svc.RegisterRoutes(mux)
	return svc, mux
}

func TestAgentsServiceRouteRegistration(t *testing.T) {
	t.Parallel()
	_, mux := newTestService()

	routes := []struct {
		method string
		path   string
		status int
	}{
		{"POST", "/api/v1/agents/definitions", http.StatusCreated},
		{"GET", "/api/v1/agents/definitions", http.StatusOK},
		{"GET", "/api/v1/agents/definitions/def1", http.StatusOK},
		{"PUT", "/api/v1/agents/definitions/def1", http.StatusOK},
		{"DELETE", "/api/v1/agents/definitions/def1", http.StatusNoContent},
		{"POST", "/api/v1/agents/execute", http.StatusAccepted},
		{"GET", "/api/v1/agents/executions", http.StatusOK},
		{"GET", "/api/v1/agents/executions/exec1", http.StatusOK},
		{"POST", "/api/v1/agents/executions/exec1/cancel", http.StatusOK},
		{"GET", "/api/v1/agents/executions/exec1/messages", http.StatusOK},
		{"GET", "/api/v1/agents/tools", http.StatusOK},
		{"POST", "/api/v1/agents/tools", http.StatusCreated},
		{"POST", "/api/v1/agents/delegation-rules", http.StatusCreated},
		{"GET", "/api/v1/agents/delegation-rules", http.StatusOK},
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

func TestAgentsGetDefinitionReturnsID(t *testing.T) {
	t.Parallel()
	_, mux := newTestService()

	req := httptest.NewRequest("GET", "/api/v1/agents/definitions/agent-def-1", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	var body map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if body["id"] != "agent-def-1" {
		t.Fatalf("expected id agent-def-1, got %v", body["id"])
	}
}

func TestAgentsGetExecutionReturnsID(t *testing.T) {
	t.Parallel()
	_, mux := newTestService()

	req := httptest.NewRequest("GET", "/api/v1/agents/executions/exec-42", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	var body map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if body["id"] != "exec-42" {
		t.Fatalf("expected id exec-42, got %v", body["id"])
	}
}

func TestAgentsJSONContentType(t *testing.T) {
	t.Parallel()
	_, mux := newTestService()

	req := httptest.NewRequest("GET", "/api/v1/agents/tools", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	ct := rec.Header().Get("Content-Type")
	if !strings.Contains(ct, "application/json") {
		t.Fatalf("expected application/json, got %s", ct)
	}
}

func TestAgentsListEndpointsReturnArrays(t *testing.T) {
	t.Parallel()
	_, mux := newTestService()

	listEndpoints := []struct {
		path     string
		arrayKey string
	}{
		{"/api/v1/agents/definitions", "definitions"},
		{"/api/v1/agents/executions", "executions"},
		{"/api/v1/agents/executions/e1/messages", "messages"},
		{"/api/v1/agents/tools", "tools"},
		{"/api/v1/agents/delegation-rules", "rules"},
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

func TestAgentsCancelExecutionReturnsStatus(t *testing.T) {
	t.Parallel()
	_, mux := newTestService()

	req := httptest.NewRequest("POST", "/api/v1/agents/executions/e1/cancel", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	var body map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if body["status"] != "cancelled" {
		t.Fatalf("expected cancelled status, got %v", body["status"])
	}
}

func TestAgentsNewServiceNotNil(t *testing.T) {
	t.Parallel()
	logger := runtimeserver.NewJSONLogger("test", "test")
	svc := NewService(Config{}, logger)
	if svc == nil {
		t.Fatal("NewService returned nil")
	}
}
