package control

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestDocsEndpointStagingOnly(t *testing.T) {
	t.Run("staging enabled", func(t *testing.T) {
		t.Setenv("APP_ENV", "staging")
		mux := NewMux(NewService("docs-secret"))

		req := httptest.NewRequest(http.MethodGet, "/docs", nil)
		rec := httptest.NewRecorder()
		mux.ServeHTTP(rec, req)
		if rec.Code != http.StatusOK {
			t.Fatalf("unexpected docs status: %d", rec.Code)
		}
		if !strings.Contains(rec.Body.String(), "SwaggerUIBundle") {
			t.Fatalf("expected swagger ui payload, got: %s", rec.Body.String())
		}

		specReq := httptest.NewRequest(http.MethodGet, "/docs/openapi", nil)
		specRec := httptest.NewRecorder()
		mux.ServeHTTP(specRec, specReq)
		if specRec.Code != http.StatusOK {
			t.Fatalf("unexpected openapi status: %d", specRec.Code)
		}
		if !strings.Contains(specRec.Body.String(), "openapi:") {
			t.Fatalf("expected openapi payload, got: %s", specRec.Body.String())
		}
	})

	t.Run("non-staging disabled", func(t *testing.T) {
		t.Setenv("APP_ENV", "production")
		mux := NewMux(NewService("docs-secret"))

		req := httptest.NewRequest(http.MethodGet, "/docs", nil)
		rec := httptest.NewRecorder()
		mux.ServeHTTP(rec, req)
		if rec.Code != http.StatusNotFound {
			t.Fatalf("expected 404 outside staging, got %d", rec.Code)
		}
	})
}
