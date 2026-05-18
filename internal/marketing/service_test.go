package marketing

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	runtimeserver "github.com/brevio/brevio/internal/runtime"
)

func newTestService() (*Service, *http.ServeMux) {
	logger := runtimeserver.NewJSONLogger("marketing-test", "test")
	svc := NewService(Config{DatabaseURL: "postgres://test", RedisURL: "redis://test"}, logger)
	mux := http.NewServeMux()
	svc.RegisterRoutes(mux)
	return svc, mux
}

func TestMarketingServiceRouteRegistration(t *testing.T) {
	t.Parallel()
	_, mux := newTestService()

	routes := []struct {
		method string
		path   string
		status int
	}{
		{"POST", "/api/v1/marketing/campaigns", http.StatusCreated},
		{"GET", "/api/v1/marketing/campaigns", http.StatusOK},
		{"GET", "/api/v1/marketing/campaigns/c1", http.StatusOK},
		{"PUT", "/api/v1/marketing/campaigns/c1", http.StatusOK},
		{"DELETE", "/api/v1/marketing/campaigns/c1", http.StatusNoContent},
		{"POST", "/api/v1/marketing/contacts", http.StatusCreated},
		{"GET", "/api/v1/marketing/contacts", http.StatusOK},
		{"POST", "/api/v1/marketing/contacts/import", http.StatusAccepted},
		{"POST", "/api/v1/marketing/sequences", http.StatusCreated},
		{"GET", "/api/v1/marketing/sequences", http.StatusOK},
		{"POST", "/api/v1/marketing/sequences/seq1/enroll", http.StatusAccepted},
		{"POST", "/api/v1/marketing/templates", http.StatusCreated},
		{"GET", "/api/v1/marketing/templates", http.StatusOK},
		{"POST", "/api/v1/marketing/email/send", http.StatusAccepted},
		{"POST", "/api/v1/marketing/social/post", http.StatusAccepted},
		{"POST", "/api/v1/marketing/leads/enrich", http.StatusAccepted},
		{"POST", "/api/v1/marketing/content/generate", http.StatusAccepted},
		{"POST", "/api/v1/marketing/ab-tests", http.StatusCreated},
		{"GET", "/api/v1/marketing/ab-tests/ab1", http.StatusOK},
		{"GET", "/api/v1/marketing/analytics", http.StatusOK},
		{"GET", "/api/v1/marketing/analytics/campaigns/c1", http.StatusOK},
		{"POST", "/api/v1/marketing/integrations", http.StatusCreated},
		{"GET", "/api/v1/marketing/integrations", http.StatusOK},
		{"DELETE", "/api/v1/marketing/integrations/int1", http.StatusNoContent},
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

func TestMarketingGetCampaignReturnsID(t *testing.T) {
	t.Parallel()
	_, mux := newTestService()

	req := httptest.NewRequest("GET", "/api/v1/marketing/campaigns/camp-789", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	var body map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if body["id"] != "camp-789" {
		t.Fatalf("expected id camp-789, got %v", body["id"])
	}
}

func TestMarketingJSONContentType(t *testing.T) {
	t.Parallel()
	_, mux := newTestService()

	req := httptest.NewRequest("GET", "/api/v1/marketing/campaigns", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	ct := rec.Header().Get("Content-Type")
	if !strings.Contains(ct, "application/json") {
		t.Fatalf("expected application/json, got %s", ct)
	}
}

func TestMarketingListEndpointsReturnArrays(t *testing.T) {
	t.Parallel()
	_, mux := newTestService()

	listEndpoints := []struct {
		path     string
		arrayKey string
	}{
		{"/api/v1/marketing/campaigns", "campaigns"},
		{"/api/v1/marketing/contacts", "contacts"},
		{"/api/v1/marketing/sequences", "sequences"},
		{"/api/v1/marketing/templates", "templates"},
		{"/api/v1/marketing/integrations", "integrations"},
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

func TestMarketingAnalyticsEndpoints(t *testing.T) {
	t.Parallel()
	_, mux := newTestService()

	t.Run("global analytics", func(t *testing.T) {
		t.Parallel()
		req := httptest.NewRequest("GET", "/api/v1/marketing/analytics", nil)
		rec := httptest.NewRecorder()
		mux.ServeHTTP(rec, req)

		var body map[string]any
		if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
			t.Fatalf("decode: %v", err)
		}
		if _, ok := body["analytics"]; !ok {
			t.Fatal("missing analytics key")
		}
	})

	t.Run("campaign analytics", func(t *testing.T) {
		t.Parallel()
		req := httptest.NewRequest("GET", "/api/v1/marketing/analytics/campaigns/c99", nil)
		rec := httptest.NewRecorder()
		mux.ServeHTTP(rec, req)

		var body map[string]any
		if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
			t.Fatalf("decode: %v", err)
		}
		if body["campaign_id"] != "c99" {
			t.Fatalf("expected campaign_id c99, got %v", body["campaign_id"])
		}
	})
}

func TestMarketingNewServiceNotNil(t *testing.T) {
	t.Parallel()
	logger := runtimeserver.NewJSONLogger("test", "test")
	svc := NewService(Config{}, logger)
	if svc == nil {
		t.Fatal("NewService returned nil")
	}
}
