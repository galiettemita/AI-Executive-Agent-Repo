package contracts

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/brevio/brevio/internal/canvas"
	"github.com/brevio/brevio/internal/control"
	"github.com/brevio/brevio/internal/gateway"
)

func TestOpenAPIServiceOwnershipClosure(t *testing.T) {
	t.Parallel()

	doc := loadOpenAPIDoc(t)
	services := map[string]http.Handler{
		"gateway": gateway.NewMux(gateway.NewService("dev-secret")),
		"control": control.NewMux(control.NewService("dev-secret")),
		"canvas":  canvas.NewMux(canvas.NewService(&canvas.InMemoryInjector{})),
	}

	for path, operations := range doc.Paths {
		owner := openAPIPathOwner(t, path)
		concretePath := concreteOpenAPIPath(path)

		for method := range operations {
			methodUpper := strings.ToUpper(method)
			for serviceName, handler := range services {
				req := httptest.NewRequest(methodUpper, concretePath, bodyForMethod(methodUpper))
				rec := httptest.NewRecorder()
				handler.ServeHTTP(rec, req)

				if owner == "shared" || serviceName == owner {
					if rec.Code == http.StatusNotFound || rec.Code == http.StatusMethodNotAllowed {
						t.Fatalf("owner endpoint unresolved: owner=%s service=%s method=%s path=%s status=%d body=%s", owner, serviceName, methodUpper, concretePath, rec.Code, rec.Body.String())
					}
					continue
				}

				if rec.Code != http.StatusNotFound {
					t.Fatalf("non-owner endpoint unexpectedly resolved: owner=%s service=%s method=%s path=%s status=%d body=%s", owner, serviceName, methodUpper, concretePath, rec.Code, rec.Body.String())
				}
			}
		}
	}
}

func openAPIPathOwner(t *testing.T, path string) string {
	t.Helper()
	switch {
	case path == "/healthz/ready", path == "/healthz/live":
		return "shared"
	case strings.HasPrefix(path, "/v1/gateway/"):
		return "gateway"
	case strings.HasPrefix(path, "/v1/canvas/"):
		return "canvas"
	case strings.HasPrefix(path, "/v1/"):
		return "control"
	default:
		t.Fatalf("unable to map openapi path to owner: %s", path)
		return ""
	}
}

func concreteOpenAPIPath(path string) string {
	replacer := strings.NewReplacer(
		"{id}", "11111111-1111-1111-1111-111111111111",
		"{task_id}", "22222222-2222-2222-2222-222222222222",
		"{turn_id}", "33333333-3333-3333-3333-333333333333",
		"{tool_key}", "connector.tool",
		"{key}", "example_key",
		"{type}", "BREVIO.test.event.v1",
		"{date}", "2026-02-27",
	)
	return replacer.Replace(path)
}

func bodyForMethod(method string) *bytes.Reader {
	switch method {
	case http.MethodPost, http.MethodPut, http.MethodPatch:
		return bytes.NewReader([]byte(`{}`))
	default:
		return bytes.NewReader(nil)
	}
}
