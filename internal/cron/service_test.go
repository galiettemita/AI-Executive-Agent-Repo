package cron

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	runtimeserver "github.com/brevio/brevio/internal/runtime"
)

func newTestService() (*Service, *http.ServeMux) {
	logger := runtimeserver.NewJSONLogger("cron-test", "test")
	svc := NewService(Config{DatabaseURL: "postgres://test", RedisURL: "redis://test"}, logger)
	mux := http.NewServeMux()
	svc.RegisterRoutes(mux)
	return svc, mux
}

func TestCronServiceRouteRegistration(t *testing.T) {
	t.Parallel()
	_, mux := newTestService()

	routes := []struct {
		method string
		path   string
		status int
	}{
		{"POST", "/api/v1/cron/jobs", http.StatusCreated},
		{"GET", "/api/v1/cron/jobs", http.StatusOK},
		{"GET", "/api/v1/cron/jobs/j1", http.StatusOK},
		{"PUT", "/api/v1/cron/jobs/j1", http.StatusOK},
		{"DELETE", "/api/v1/cron/jobs/j1", http.StatusNoContent},
		{"POST", "/api/v1/cron/jobs/j1/pause", http.StatusOK},
		{"POST", "/api/v1/cron/jobs/j1/resume", http.StatusOK},
		{"POST", "/api/v1/cron/jobs/j1/trigger", http.StatusAccepted},
		{"GET", "/api/v1/cron/executions", http.StatusOK},
		{"GET", "/api/v1/cron/executions/e1", http.StatusOK},
		{"POST", "/api/v1/cron/webhooks", http.StatusCreated},
		{"GET", "/api/v1/cron/webhooks", http.StatusOK},
		{"DELETE", "/api/v1/cron/webhooks/wh1", http.StatusNoContent},
		{"POST", "/api/v1/cron/notifications", http.StatusCreated},
		{"GET", "/api/v1/cron/notifications", http.StatusOK},
		{"GET", "/api/v1/cron/audit", http.StatusOK},
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

func TestCronGetJobReturnsID(t *testing.T) {
	t.Parallel()
	_, mux := newTestService()

	req := httptest.NewRequest("GET", "/api/v1/cron/jobs/job-xyz", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	var body map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if body["id"] != "job-xyz" {
		t.Fatalf("expected id job-xyz, got %v", body["id"])
	}
}

func TestCronPauseResumeStatuses(t *testing.T) {
	t.Parallel()
	_, mux := newTestService()

	t.Run("pause", func(t *testing.T) {
		t.Parallel()
		req := httptest.NewRequest("POST", "/api/v1/cron/jobs/j1/pause", nil)
		rec := httptest.NewRecorder()
		mux.ServeHTTP(rec, req)

		var body map[string]any
		if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
			t.Fatalf("decode: %v", err)
		}
		if body["status"] != "paused" {
			t.Fatalf("expected paused, got %v", body["status"])
		}
	})

	t.Run("resume", func(t *testing.T) {
		t.Parallel()
		req := httptest.NewRequest("POST", "/api/v1/cron/jobs/j1/resume", nil)
		rec := httptest.NewRecorder()
		mux.ServeHTTP(rec, req)

		var body map[string]any
		if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
			t.Fatalf("decode: %v", err)
		}
		if body["status"] != "active" {
			t.Fatalf("expected active, got %v", body["status"])
		}
	})
}

func TestCronTriggerReturnsAccepted(t *testing.T) {
	t.Parallel()
	_, mux := newTestService()

	req := httptest.NewRequest("POST", "/api/v1/cron/jobs/j1/trigger", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusAccepted {
		t.Fatalf("expected 202, got %d", rec.Code)
	}
	var body map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if body["status"] != "triggered" {
		t.Fatalf("expected triggered, got %v", body["status"])
	}
}

func TestCronJSONContentType(t *testing.T) {
	t.Parallel()
	_, mux := newTestService()

	req := httptest.NewRequest("GET", "/api/v1/cron/jobs", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	ct := rec.Header().Get("Content-Type")
	if !strings.Contains(ct, "application/json") {
		t.Fatalf("expected application/json, got %s", ct)
	}
}

func TestCronListEndpointsReturnArrays(t *testing.T) {
	t.Parallel()
	_, mux := newTestService()

	listEndpoints := []struct {
		path     string
		arrayKey string
	}{
		{"/api/v1/cron/jobs", "jobs"},
		{"/api/v1/cron/executions", "executions"},
		{"/api/v1/cron/webhooks", "webhooks"},
		{"/api/v1/cron/notifications", "notifications"},
		{"/api/v1/cron/audit", "audit_entries"},
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

func TestCronDeleteJobReturnsNoContent(t *testing.T) {
	t.Parallel()
	_, mux := newTestService()

	req := httptest.NewRequest("DELETE", "/api/v1/cron/jobs/del-1", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusNoContent {
		t.Fatalf("expected 204, got %d", rec.Code)
	}
}

func TestCronNewServiceNotNil(t *testing.T) {
	t.Parallel()
	logger := runtimeserver.NewJSONLogger("test", "test")
	svc := NewService(Config{}, logger)
	if svc == nil {
		t.Fatal("NewService returned nil")
	}
}
