package control

import (
	"bytes"
	"encoding/json"
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

func TestControlMuxFeatureFlagsFlow(t *testing.T) {
	t.Parallel()

	mux := NewMux(NewService("dev-secret"))

	postFlagBody := []byte(`{"key":"new_ui","flag_type":"boolean","enabled":true}`)
	postFlagReq := httptest.NewRequest(http.MethodPost, "/v1/flags", bytes.NewReader(postFlagBody))
	postFlagResp := httptest.NewRecorder()
	mux.ServeHTTP(postFlagResp, postFlagReq)
	if postFlagResp.Code != http.StatusAccepted {
		t.Fatalf("unexpected create flag status: %d", postFlagResp.Code)
	}

	postRulesBody := []byte(`{"rules":[{"match_type":"workspace","match_value":"ws_a","enabled":false}]}`)
	postRulesReq := httptest.NewRequest(http.MethodPost, "/v1/flags/new_ui/rules", bytes.NewReader(postRulesBody))
	postRulesResp := httptest.NewRecorder()
	mux.ServeHTTP(postRulesResp, postRulesReq)
	if postRulesResp.Code != http.StatusAccepted {
		t.Fatalf("unexpected create rules status: %d", postRulesResp.Code)
	}

	evaluateReq := httptest.NewRequest(http.MethodPost, "/v1/flags/new_ui/evaluate", bytes.NewReader([]byte(`{"attributes":{"workspace":"ws_a"}}`)))
	evaluateResp := httptest.NewRecorder()
	mux.ServeHTTP(evaluateResp, evaluateReq)
	if evaluateResp.Code != http.StatusOK {
		t.Fatalf("unexpected evaluate status: %d", evaluateResp.Code)
	}
	var evaluatePayload map[string]any
	if err := json.Unmarshal(evaluateResp.Body.Bytes(), &evaluatePayload); err != nil {
		t.Fatalf("decode evaluate payload: %v", err)
	}
	enabled, ok := evaluatePayload["enabled"].(bool)
	if !ok {
		t.Fatalf("missing enabled field: %v", evaluatePayload)
	}
	if enabled {
		t.Fatalf("expected evaluate deny for ws_a")
	}

	getFlagReq := httptest.NewRequest(http.MethodGet, "/v1/flags/new_ui", nil)
	getFlagResp := httptest.NewRecorder()
	mux.ServeHTTP(getFlagResp, getFlagReq)
	if getFlagResp.Code != http.StatusOK {
		t.Fatalf("unexpected get flag status: %d", getFlagResp.Code)
	}

	deleteFlagReq := httptest.NewRequest(http.MethodDelete, "/v1/flags/new_ui", nil)
	deleteFlagResp := httptest.NewRecorder()
	mux.ServeHTTP(deleteFlagResp, deleteFlagReq)
	if deleteFlagResp.Code != http.StatusOK {
		t.Fatalf("unexpected delete flag status: %d", deleteFlagResp.Code)
	}
}

func TestControlMuxContextBudgetFlow(t *testing.T) {
	t.Parallel()

	mux := NewMux(NewService("dev-secret"))

	putBudgetBody := []byte(`{"workspace_id":"ws_1","budget_tokens":2048,"status":"active","allocations":{"history":1024,"tool":512}}`)
	putBudgetReq := httptest.NewRequest(http.MethodPut, "/v1/context/budget", bytes.NewReader(putBudgetBody))
	putBudgetResp := httptest.NewRecorder()
	mux.ServeHTTP(putBudgetResp, putBudgetReq)
	if putBudgetResp.Code != http.StatusOK {
		t.Fatalf("unexpected put budget status: %d", putBudgetResp.Code)
	}

	getBudgetReq := httptest.NewRequest(http.MethodGet, "/v1/context/budget?workspace_id=ws_1", nil)
	getBudgetResp := httptest.NewRecorder()
	mux.ServeHTTP(getBudgetResp, getBudgetReq)
	if getBudgetResp.Code != http.StatusOK {
		t.Fatalf("unexpected get budget status: %d", getBudgetResp.Code)
	}
	var budgetPayload map[string]any
	if err := json.Unmarshal(getBudgetResp.Body.Bytes(), &budgetPayload); err != nil {
		t.Fatalf("decode budget payload: %v", err)
	}
	if int(budgetPayload["budget_tokens"].(float64)) != 2048 {
		t.Fatalf("unexpected budget tokens payload: %v", budgetPayload)
	}

	getAllocReq := httptest.NewRequest(http.MethodGet, "/v1/context/allocations?workspace_id=ws_1", nil)
	getAllocResp := httptest.NewRecorder()
	mux.ServeHTTP(getAllocResp, getAllocReq)
	if getAllocResp.Code != http.StatusOK {
		t.Fatalf("unexpected get allocations status: %d", getAllocResp.Code)
	}
	var allocPayload map[string]any
	if err := json.Unmarshal(getAllocResp.Body.Bytes(), &allocPayload); err != nil {
		t.Fatalf("decode allocation payload: %v", err)
	}
	allocs, ok := allocPayload["allocations"].([]any)
	if !ok {
		t.Fatalf("missing allocations payload: %v", allocPayload)
	}
	if len(allocs) != 2 {
		t.Fatalf("unexpected allocation count: %d", len(allocs))
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
