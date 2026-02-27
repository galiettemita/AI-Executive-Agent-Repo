package control

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
)

type openapiDoc struct {
	Paths map[string]map[string]any `yaml:"paths"`
}

func TestControlMuxRespondsForOpenAPIEndpoints(t *testing.T) {
	t.Parallel()

	mux := NewMux(NewService("dev-secret"))
	doc := loadOpenAPIForControlTest(t)

	for path, methods := range doc.Paths {
		for method := range methods {
			methodUpper := strings.ToUpper(method)
			if methodUpper == "GET" && path == "/v1/canvas/ws" {
				continue
			}

			reqPath := concretePath(path)
			req := httptest.NewRequest(methodUpper, reqPath, nil)
			rec := httptest.NewRecorder()
			mux.ServeHTTP(rec, req)

			if rec.Code == http.StatusNotFound || rec.Code == http.StatusMethodNotAllowed {
				t.Fatalf("endpoint did not respond: %s %s status=%d", methodUpper, reqPath, rec.Code)
			}
		}
	}
}

func concretePath(template string) string {
	replacements := map[string]string{
		"{id}":       "11111111-1111-1111-1111-111111111111",
		"{task_id}":  "22222222-2222-2222-2222-222222222222",
		"{turn_id}":  "33333333-3333-3333-3333-333333333333",
		"{tool_key}": "connector.tool",
		"{key}":      "example_key",
		"{type}":     "BREVIO.test.event.v1",
		"{date}":     "2026-02-27",
	}
	out := template
	for key, value := range replacements {
		out = strings.ReplaceAll(out, key, value)
	}
	return out
}

func loadOpenAPIForControlTest(t *testing.T) openapiDoc {
	t.Helper()

	_, currentFile, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("unable to resolve current file path")
	}
	root := filepath.Clean(filepath.Join(filepath.Dir(currentFile), "..", ".."))
	path := filepath.Join(root, "api", "openapi", "v9.yaml")

	body, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read openapi file: %v", err)
	}

	var doc openapiDoc
	if err := yaml.Unmarshal(body, &doc); err != nil {
		t.Fatalf("parse openapi yaml: %v", err)
	}
	if len(doc.Paths) == 0 {
		t.Fatal("openapi paths are empty")
	}
	return doc
}
