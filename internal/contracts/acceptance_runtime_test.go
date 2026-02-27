package contracts

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/brevio/brevio/internal/control"
)

func TestAcceptanceRuntimeV91(t *testing.T) {
	t.Parallel()
	mux := control.NewMux(control.NewService("dev-secret"))

	goalResp := callJSON(t, mux, http.MethodPost, "/v1/goals", map[string]any{
		"workspace_id": "ws_runtime_v91",
		"title":        "Runtime acceptance",
		"status":       "active",
		"priority":     "high",
	}, http.StatusCreated)
	goalID := mustString(t, goalResp, "id")

	callJSON(t, mux, http.MethodPost, "/v1/goals/"+goalID+"/milestones", map[string]any{
		"title":  "Create milestone",
		"status": "pending",
	}, http.StatusCreated)
	callJSON(t, mux, http.MethodGet, "/v1/goals/"+goalID+"/progress", nil, http.StatusOK)

	callJSON(t, mux, http.MethodPut, "/v1/mission-control/config?workspace_id=ws_runtime_v91", map[string]any{
		"refresh_cadence_minutes": 25,
	}, http.StatusOK)
	callJSON(t, mux, http.MethodGet, "/v1/mission-control/snapshot?workspace_id=ws_runtime_v91", nil, http.StatusOK)

	promotions := callJSON(t, mux, http.MethodGet, "/v1/autonomy/promotions", nil, http.StatusOK)
	promotionsList, ok := promotions["promotions"].([]any)
	if !ok || len(promotionsList) == 0 {
		t.Fatalf("expected promotions payload, got %v", promotions)
	}
	firstPromotion, ok := promotionsList[0].(map[string]any)
	if !ok {
		t.Fatalf("unexpected promotion payload: %v", promotionsList[0])
	}
	promotionID, _ := firstPromotion["id"].(string)
	callJSON(t, mux, http.MethodPost, "/v1/autonomy/promotions/"+promotionID+"/decide", map[string]any{
		"decision": "approve",
	}, http.StatusOK)

	callJSON(t, mux, http.MethodPost, "/v1/learning/feedback?workspace_id=ws_runtime_v91", map[string]any{
		"feedback_type": "positive",
		"content":       "runtime gate pass",
	}, http.StatusCreated)
	callJSON(t, mux, http.MethodGet, "/v1/learning/lessons?workspace_id=ws_runtime_v91", nil, http.StatusOK)

	callJSON(t, mux, http.MethodGet, "/v1/captures/daily?workspace_id=ws_runtime_v91", nil, http.StatusOK)
	callJSON(t, mux, http.MethodGet, "/v1/capabilities/recommendations?workspace_id=ws_runtime_v91", nil, http.StatusOK)
	callJSON(t, mux, http.MethodPost, "/v1/codebase/context-export?workspace_id=ws_runtime_v91", map[string]any{
		"format": "markdown",
	}, http.StatusCreated)
	callJSON(t, mux, http.MethodPut, "/v1/self-modification/policy?workspace_id=ws_runtime_v91", map[string]any{
		"enabled":          true,
		"require_approval": true,
		"max_allowed_risk": "elevated",
	}, http.StatusOK)
}

func TestAcceptanceRuntimeV92(t *testing.T) {
	t.Parallel()
	mux := control.NewMux(control.NewService("dev-secret"))

	callJSON(t, mux, http.MethodPut, "/v1/context/budget", map[string]any{
		"workspace_id":  "ws_runtime_v92",
		"budget_tokens": 2048,
		"status":        "active",
		"allocations": map[string]int{
			"history": 1024,
		},
	}, http.StatusOK)

	collectionResp := callJSON(t, mux, http.MethodPost, "/v1/rag/collections", map[string]any{
		"workspace_id": "ws_runtime_v92",
		"name":         "runtime",
	}, http.StatusCreated)
	collectionID := mustString(t, collectionResp, "id")
	callJSON(t, mux, http.MethodPost, "/v1/rag/collections/"+collectionID+"/ingest", map[string]any{
		"documents": []string{"runtime rag validation content"},
	}, http.StatusAccepted)
	callJSON(t, mux, http.MethodPost, "/v1/rag/search", map[string]any{
		"workspace_id":   "ws_runtime_v92",
		"turn_id":        "turn_runtime_v92",
		"query_text":     "validation content",
		"collection_ids": []string{collectionID},
		"max_results":    1,
	}, http.StatusOK)

	callJSON(t, mux, http.MethodGet, "/v1/sessions/session_runtime?workspace_id=ws_runtime_v92&user_id=user_runtime", nil, http.StatusOK)
	callJSON(t, mux, http.MethodPost, "/v1/temporal/resolve", map[string]any{
		"workspace_id":   "ws_runtime_v92",
		"expression":     "tomorrow",
		"reference_date": "2026-02-27",
	}, http.StatusOK)

	callJSON(t, mux, http.MethodPut, "/v1/guardrails/config", map[string]any{
		"workspace_id":               "ws_runtime_v92",
		"enable_pii_redaction":       true,
		"enable_jailbreak_detection": true,
		"block_threshold":            85,
	}, http.StatusOK)
	callJSON(t, mux, http.MethodGet, "/v1/tools/health/calendar.create_event?workspace_id=ws_runtime_v92", nil, http.StatusOK)

	callJSON(t, mux, http.MethodPost, "/v1/flags", map[string]any{
		"key":       "runtime_flag",
		"flag_type": "boolean",
		"enabled":   true,
	}, http.StatusAccepted)
	callJSON(t, mux, http.MethodPost, "/v1/flags/runtime_flag/evaluate", map[string]any{
		"attributes": map[string]string{"workspace": "ws_runtime_v92"},
	}, http.StatusOK)

	callJSON(t, mux, http.MethodPut, "/v1/streaming/config", map[string]any{
		"workspace_id":           "ws_runtime_v92",
		"first_byte_sla_ms":      450,
		"chunk_size_bytes":       2048,
		"ack_enabled":            true,
		"typing_indicator":       true,
		"progressive_disclosure": true,
	}, http.StatusOK)
	callJSON(t, mux, http.MethodGet, "/v1/errors/taxonomy", nil, http.StatusOK)

	callJSON(t, mux, http.MethodPost, "/v1/event-schemas/BREVIO.runtime.event.v1/versions", map[string]any{
		"schema": "{\"type\":\"object\"}",
		"status": "active",
	}, http.StatusCreated)
	callJSON(t, mux, http.MethodPost, "/v1/event-schemas/BREVIO.runtime.event.v1/validate", map[string]any{
		"event": map[string]any{
			"type":    "BREVIO.runtime.event.v1",
			"version": 1,
		},
	}, http.StatusOK)

	callJSON(t, mux, http.MethodPost, "/v1/compliance/frameworks", map[string]any{
		"workspace_id": "ws_runtime_v92",
		"key":          "soc2",
	}, http.StatusCreated)
	callJSON(t, mux, http.MethodGet, "/v1/compliance/evidence?workspace_id=ws_runtime_v92", nil, http.StatusOK)

	callJSON(t, mux, http.MethodPost, "/v1/cache/policies", map[string]any{
		"workspace_id": "ws_runtime_v92",
		"cache_key":    "compiled_context",
		"ttl_seconds":  600,
		"max_bytes":    1048576,
		"enabled":      true,
	}, http.StatusCreated)
	callJSON(t, mux, http.MethodGet, "/v1/cache/stats?workspace_id=ws_runtime_v92", nil, http.StatusOK)

	callJSON(t, mux, http.MethodPost, "/v1/model-tiers/policies?workspace_id=ws_runtime_v92", map[string]any{
		"workspace_id": "ws_runtime_v92",
		"tier":         "T3",
		"enabled":      true,
	}, http.StatusCreated)
	callJSON(t, mux, http.MethodGet, "/v1/model-tiers/overrides?workspace_id=ws_runtime_v92", nil, http.StatusOK)
	callJSON(t, mux, http.MethodGet, "/v1/admin/operations/dashboard", nil, http.StatusOK)
}

func callJSON(t *testing.T, mux *http.ServeMux, method, path string, payload any, expectedStatus int) map[string]any {
	t.Helper()
	var body []byte
	if payload != nil {
		encoded, err := json.Marshal(payload)
		if err != nil {
			t.Fatalf("marshal payload: %v", err)
		}
		body = encoded
	}
	req := httptest.NewRequest(method, path, bytes.NewReader(body))
	resp := httptest.NewRecorder()
	mux.ServeHTTP(resp, req)
	if resp.Code != expectedStatus {
		t.Fatalf("unexpected status for %s %s: got=%d want=%d body=%s", method, path, resp.Code, expectedStatus, resp.Body.String())
	}

	var out map[string]any
	if resp.Body.Len() == 0 {
		return map[string]any{}
	}
	if err := json.Unmarshal(resp.Body.Bytes(), &out); err != nil {
		t.Fatalf("decode response for %s %s: %v body=%s", method, path, err, resp.Body.String())
	}
	return out
}

func mustString(t *testing.T, payload map[string]any, key string) string {
	t.Helper()
	value, ok := payload[key].(string)
	if !ok || value == "" {
		t.Fatalf("missing string field %q in payload: %v", key, payload)
	}
	return value
}
