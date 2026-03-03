package contracts

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"

	"github.com/brevio/brevio/internal/canvas"
	"github.com/brevio/brevio/internal/control"
	"github.com/brevio/brevio/internal/gateway"
)

func TestServiceHealthEndpointClosure(t *testing.T) {
	t.Parallel()

	root := repositoryRoot(t)
	requiredHealthTokens := []string{
		"GET /healthz/ready",
		"GET /healthz/live",
	}
	requiredAPIHealthTokens := []string{
		"GET /health",
		"GET /health/deep",
	}

	assertFileContainsTokens(t, filepath.Join(root, "cmd", "brain", "main.go"), requiredHealthTokens)
	assertFileContainsTokens(t, filepath.Join(root, "cmd", "brain", "main.go"), requiredAPIHealthTokens)
	assertFileContainsTokens(t, filepath.Join(root, "cmd", "executor", "main.go"), requiredHealthTokens)
	assertFileContainsTokens(t, filepath.Join(root, "cmd", "executor", "main.go"), requiredAPIHealthTokens)
	assertFileContainsTokens(t, filepath.Join(root, "cmd", "temporal-worker", "main.go"), requiredHealthTokens)
	assertFileContainsTokens(t, filepath.Join(root, "cmd", "temporal-worker", "main.go"), requiredAPIHealthTokens)
	assertFileContainsTokens(t, filepath.Join(root, "internal", "gateway", "server.go"), requiredHealthTokens)
	assertFileContainsTokens(t, filepath.Join(root, "internal", "gateway", "server.go"), requiredAPIHealthTokens)
	assertFileContainsTokens(t, filepath.Join(root, "internal", "control", "mux.go"), requiredHealthTokens)
	assertFileContainsTokens(t, filepath.Join(root, "internal", "control", "mux.go"), requiredAPIHealthTokens)
	assertFileContainsTokens(t, filepath.Join(root, "internal", "canvas", "service.go"), requiredHealthTokens)
	assertFileContainsTokens(t, filepath.Join(root, "internal", "canvas", "service.go"), requiredAPIHealthTokens)

	t.Run("gateway_runtime_health_endpoints", func(t *testing.T) {
		svc := gateway.NewService("health-secret")
		mux := gateway.NewMux(svc)
		assertRuntimeHealthEndpoints(t, mux)
	})

	t.Run("control_runtime_health_endpoints", func(t *testing.T) {
		svc := control.NewService("health-secret")
		mux := control.NewMux(svc)
		assertRuntimeHealthEndpoints(t, mux)
	})

	t.Run("canvas_runtime_health_endpoints", func(t *testing.T) {
		svc := canvas.NewService(&canvas.InMemoryInjector{})
		mux := canvas.NewMux(svc)
		assertRuntimeHealthEndpoints(t, mux)
	})
}

func assertRuntimeHealthEndpoints(t *testing.T, mux *http.ServeMux) {
	t.Helper()

	for _, path := range []string{"/healthz/ready", "/healthz/live", "/health", "/health/deep"} {
		req := httptest.NewRequest(http.MethodGet, path, nil)
		rec := httptest.NewRecorder()
		mux.ServeHTTP(rec, req)
		if rec.Code != http.StatusOK {
			t.Fatalf("unexpected health status for %s: got=%d want=%d", path, rec.Code, http.StatusOK)
		}
		if path == "/health" || path == "/health/deep" {
			var payload map[string]any
			if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
				t.Fatalf("health endpoint %s did not return JSON: %v", path, err)
			}
			if payload["status"] != "healthy" {
				t.Fatalf("unexpected health payload status for %s: %#v", path, payload)
			}
		}
	}
}
