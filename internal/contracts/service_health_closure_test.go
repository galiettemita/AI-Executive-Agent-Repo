package contracts

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"
	"time"

	"github.com/brevio/brevio/internal/canvas"
	"github.com/brevio/brevio/internal/control"
	"github.com/brevio/brevio/internal/gateway"
	"github.com/brevio/brevio/internal/identity"
)

func TestServiceHealthEndpointClosure(t *testing.T) {
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

	for _, path := range []string{"/healthz/ready", "/healthz/live", "/health"} {
		req := httptest.NewRequest(http.MethodGet, path, nil)
		rec := httptest.NewRecorder()
		mux.ServeHTTP(rec, req)
		if rec.Code != http.StatusOK {
			t.Fatalf("unexpected health status for %s: got=%d want=%d", path, rec.Code, http.StatusOK)
		}
		if path == "/health" {
			var payload map[string]any
			if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
				t.Fatalf("health endpoint %s did not return JSON: %v", path, err)
			}
			if payload["status"] != "healthy" {
				t.Fatalf("unexpected health payload status for %s: %#v", path, payload)
			}
		}
	}

	unauthorized := httptest.NewRequest(http.MethodGet, "/health/deep", nil)
	unauthorizedRec := httptest.NewRecorder()
	mux.ServeHTTP(unauthorizedRec, unauthorized)
	if unauthorizedRec.Code != http.StatusUnauthorized {
		t.Fatalf("unexpected health status for /health/deep without auth: got=%d want=%d", unauthorizedRec.Code, http.StatusUnauthorized)
	}

	authorized := adminHealthRequest(t, "/health/deep")
	authorizedRec := httptest.NewRecorder()
	mux.ServeHTTP(authorizedRec, authorized)
	if authorizedRec.Code != http.StatusOK {
		t.Fatalf("unexpected health status for /health/deep with admin auth: got=%d want=%d", authorizedRec.Code, http.StatusOK)
	}
	var payload map[string]any
	if err := json.Unmarshal(authorizedRec.Body.Bytes(), &payload); err != nil {
		t.Fatalf("health endpoint /health/deep did not return JSON: %v", err)
	}
	if payload["status"] != "healthy" {
		t.Fatalf("unexpected health payload status for /health/deep: %#v", payload)
	}
}

func adminHealthRequest(t *testing.T, path string) *http.Request {
	t.Helper()

	privateKey, err := identity.GenerateJWTSigningKey()
	if err != nil {
		t.Fatalf("generate admin jwt key: %v", err)
	}
	publicKeyPEM, err := identity.MarshalRSAPublicKeyPEM(&privateKey.PublicKey)
	if err != nil {
		t.Fatalf("marshal public key: %v", err)
	}
	t.Setenv("BREVIO_AUTH_ACCESS_PUBLIC_KEY", publicKeyPEM)
	t.Setenv("BREVIO_AUTH_ACCESS_ISSUER", "https://auth.brevio.internal")

	token, err := identity.NewJWTSigner(privateKey).IssueAdminJWT(identity.AdminJWTClaims{
		UserJWTClaims: identity.UserJWTClaims{
			Version:  2,
			Sub:      "admin-user",
			Iss:      "https://auth.brevio.internal",
			Aud:      identity.AdminJWTAudience(),
			TokenUse: "admin_access",
		},
		AdminLevel:  "ops",
		AdminScopes: []string{"health:read"},
	}, time.Now().UTC())
	if err != nil {
		t.Fatalf("issue admin jwt: %v", err)
	}

	req := httptest.NewRequest(http.MethodGet, path, nil)
	req.Header.Set("Authorization", "Bearer "+token)
	return req
}
