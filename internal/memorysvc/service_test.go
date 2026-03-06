package memorysvc

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	runtimeserver "github.com/brevio/brevio/internal/runtime"
)

func newTestService() (*Service, *http.ServeMux) {
	logger := runtimeserver.NewJSONLogger("memorysvc-test", "test")
	svc := NewService(Config{DatabaseURL: "postgres://test", RedisURL: "redis://test"}, logger)
	mux := http.NewServeMux()
	svc.RegisterRoutes(mux)
	return svc, mux
}

func TestMemorySvcRouteRegistration(t *testing.T) {
	t.Parallel()
	_, mux := newTestService()

	routes := []struct {
		method string
		path   string
		status int
	}{
		{"POST", "/api/v1/memory/documents", http.StatusCreated},
		{"GET", "/api/v1/memory/documents", http.StatusOK},
		{"GET", "/api/v1/memory/documents/doc1", http.StatusOK},
		{"DELETE", "/api/v1/memory/documents/doc1", http.StatusNoContent},
		{"POST", "/api/v1/memory/search", http.StatusOK},
		{"POST", "/api/v1/memory/recall", http.StatusOK},
		{"POST", "/api/v1/memory/summarize", http.StatusAccepted},
		{"GET", "/api/v1/memory/conversations", http.StatusOK},
		{"GET", "/api/v1/memory/conversations/conv1", http.StatusOK},
		{"POST", "/api/v1/memory/facts", http.StatusCreated},
		{"GET", "/api/v1/memory/facts", http.StatusOK},
		{"DELETE", "/api/v1/memory/facts/f1", http.StatusNoContent},
		{"POST", "/api/v1/memory/knowledge-graph/triples", http.StatusCreated},
		{"GET", "/api/v1/memory/knowledge-graph/query", http.StatusOK},
		{"GET", "/api/v1/memory/knowledge-graph/entity/person", http.StatusOK},
		{"POST", "/api/v1/memory/forget", http.StatusOK},
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

func TestMemorySvcGetDocumentReturnsID(t *testing.T) {
	t.Parallel()
	_, mux := newTestService()

	req := httptest.NewRequest("GET", "/api/v1/memory/documents/doc-abc", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	var body map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if body["id"] != "doc-abc" {
		t.Fatalf("expected id doc-abc, got %v", body["id"])
	}
}

func TestMemorySvcGetEntityReturnsEntity(t *testing.T) {
	t.Parallel()
	_, mux := newTestService()

	req := httptest.NewRequest("GET", "/api/v1/memory/knowledge-graph/entity/john-doe", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	var body map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if body["entity"] != "john-doe" {
		t.Fatalf("expected entity john-doe, got %v", body["entity"])
	}
}

func TestMemorySvcJSONContentType(t *testing.T) {
	t.Parallel()
	_, mux := newTestService()

	req := httptest.NewRequest("GET", "/api/v1/memory/facts", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	ct := rec.Header().Get("Content-Type")
	if !strings.Contains(ct, "application/json") {
		t.Fatalf("expected application/json, got %s", ct)
	}
}

func TestMemorySvcListEndpointsReturnArrays(t *testing.T) {
	t.Parallel()
	_, mux := newTestService()

	listEndpoints := []struct {
		path     string
		arrayKey string
	}{
		{"/api/v1/memory/documents", "documents"},
		{"/api/v1/memory/conversations", "conversations"},
		{"/api/v1/memory/facts", "facts"},
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

func TestMemorySvcSearchReturnsResults(t *testing.T) {
	t.Parallel()
	_, mux := newTestService()

	req := httptest.NewRequest("POST", "/api/v1/memory/search", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	var body map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if _, ok := body["results"]; !ok {
		t.Fatal("missing results key in search response")
	}
}

func TestMemorySvcRecallReturnsMemories(t *testing.T) {
	t.Parallel()
	_, mux := newTestService()

	req := httptest.NewRequest("POST", "/api/v1/memory/recall", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	var body map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if _, ok := body["memories"]; !ok {
		t.Fatal("missing memories key in recall response")
	}
}

func TestMemorySvcForgetReturnsStatus(t *testing.T) {
	t.Parallel()
	_, mux := newTestService()

	req := httptest.NewRequest("POST", "/api/v1/memory/forget", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	var body map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if body["status"] != "forgotten" {
		t.Fatalf("expected status forgotten, got %v", body["status"])
	}
}

func TestMemorySvcNewServiceNotNil(t *testing.T) {
	t.Parallel()
	logger := runtimeserver.NewJSONLogger("test", "test")
	svc := NewService(Config{}, logger)
	if svc == nil {
		t.Fatal("NewService returned nil")
	}
}
