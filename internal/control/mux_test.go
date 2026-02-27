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

func TestControlMuxRAGFlow(t *testing.T) {
	t.Parallel()

	mux := NewMux(NewService("dev-secret"))

	postCollectionBody := []byte(`{"workspace_id":"ws_1","name":"knowledge","description":"workspace docs"}`)
	postCollectionReq := httptest.NewRequest(http.MethodPost, "/v1/rag/collections", bytes.NewReader(postCollectionBody))
	postCollectionResp := httptest.NewRecorder()
	mux.ServeHTTP(postCollectionResp, postCollectionReq)
	if postCollectionResp.Code != http.StatusCreated {
		t.Fatalf("unexpected create collection status: %d", postCollectionResp.Code)
	}
	var collectionPayload map[string]any
	if err := json.Unmarshal(postCollectionResp.Body.Bytes(), &collectionPayload); err != nil {
		t.Fatalf("decode collection payload: %v", err)
	}
	collectionID, ok := collectionPayload["id"].(string)
	if !ok || collectionID == "" {
		t.Fatalf("missing collection id payload: %v", collectionPayload)
	}

	postIngestBody := []byte(`{"documents":["alpha policy requires recipient verification","beta policy requires confirmation"]}`)
	postIngestReq := httptest.NewRequest(http.MethodPost, "/v1/rag/collections/"+collectionID+"/ingest", bytes.NewReader(postIngestBody))
	postIngestResp := httptest.NewRecorder()
	mux.ServeHTTP(postIngestResp, postIngestReq)
	if postIngestResp.Code != http.StatusAccepted {
		t.Fatalf("unexpected ingest status: %d", postIngestResp.Code)
	}

	postSearchBody := []byte(`{"workspace_id":"ws_1","turn_id":"turn_1","query_text":"recipient verification","collection_ids":["` + collectionID + `"],"max_results":2}`)
	postSearchReq := httptest.NewRequest(http.MethodPost, "/v1/rag/search", bytes.NewReader(postSearchBody))
	postSearchResp := httptest.NewRecorder()
	mux.ServeHTTP(postSearchResp, postSearchReq)
	if postSearchResp.Code != http.StatusOK {
		t.Fatalf("unexpected search status: %d", postSearchResp.Code)
	}
	var searchPayload map[string]any
	if err := json.Unmarshal(postSearchResp.Body.Bytes(), &searchPayload); err != nil {
		t.Fatalf("decode search payload: %v", err)
	}
	results, ok := searchPayload["results"].([]any)
	if !ok || len(results) == 0 {
		t.Fatalf("expected non-empty retrieval results: %v", searchPayload)
	}

	getRetrievalReq := httptest.NewRequest(http.MethodGet, "/v1/rag/retrievals/turn_1", nil)
	getRetrievalResp := httptest.NewRecorder()
	mux.ServeHTTP(getRetrievalResp, getRetrievalReq)
	if getRetrievalResp.Code != http.StatusOK {
		t.Fatalf("unexpected get retrieval status: %d", getRetrievalResp.Code)
	}

	getScoresReq := httptest.NewRequest(http.MethodGet, "/v1/rag/eval/scores?workspace_id=ws_1", nil)
	getScoresResp := httptest.NewRecorder()
	mux.ServeHTTP(getScoresResp, getScoresReq)
	if getScoresResp.Code != http.StatusOK {
		t.Fatalf("unexpected eval scores status: %d", getScoresResp.Code)
	}

	putCollectionBody := []byte(`{"workspace_id":"ws_1","name":"knowledge-v2","description":"updated docs"}`)
	putCollectionReq := httptest.NewRequest(http.MethodPut, "/v1/rag/collections/"+collectionID, bytes.NewReader(putCollectionBody))
	putCollectionResp := httptest.NewRecorder()
	mux.ServeHTTP(putCollectionResp, putCollectionReq)
	if putCollectionResp.Code != http.StatusOK {
		t.Fatalf("unexpected update collection status: %d", putCollectionResp.Code)
	}

	deleteCollectionReq := httptest.NewRequest(http.MethodDelete, "/v1/rag/collections/"+collectionID, nil)
	deleteCollectionResp := httptest.NewRecorder()
	mux.ServeHTTP(deleteCollectionResp, deleteCollectionReq)
	if deleteCollectionResp.Code != http.StatusOK {
		t.Fatalf("unexpected delete collection status: %d", deleteCollectionResp.Code)
	}
}

func TestControlMuxGuardrailsFlow(t *testing.T) {
	t.Parallel()

	mux := NewMux(NewService("dev-secret"))

	putConfigBody := []byte(`{"workspace_id":"ws_1","enable_pii_redaction":true,"enable_jailbreak_detection":true,"block_threshold":85}`)
	putConfigReq := httptest.NewRequest(http.MethodPut, "/v1/guardrails/config", bytes.NewReader(putConfigBody))
	putConfigResp := httptest.NewRecorder()
	mux.ServeHTTP(putConfigResp, putConfigReq)
	if putConfigResp.Code != http.StatusOK {
		t.Fatalf("unexpected guardrails config status: %d", putConfigResp.Code)
	}

	postRuleSetBody := []byte(`{"workspace_id":"ws_1","name":"injection_patterns","mode":"block","patterns":["ignore previous instructions","system prompt"],"enabled":true}`)
	postRuleSetReq := httptest.NewRequest(http.MethodPost, "/v1/guardrails/rule-sets", bytes.NewReader(postRuleSetBody))
	postRuleSetResp := httptest.NewRecorder()
	mux.ServeHTTP(postRuleSetResp, postRuleSetReq)
	if postRuleSetResp.Code != http.StatusCreated {
		t.Fatalf("unexpected create guardrails rule-set status: %d", postRuleSetResp.Code)
	}
	var ruleSetPayload map[string]any
	if err := json.Unmarshal(postRuleSetResp.Body.Bytes(), &ruleSetPayload); err != nil {
		t.Fatalf("decode guardrails ruleset payload: %v", err)
	}
	ruleSetID, ok := ruleSetPayload["id"].(string)
	if !ok || ruleSetID == "" {
		t.Fatalf("missing ruleset id payload: %v", ruleSetPayload)
	}

	getRuleSetsReq := httptest.NewRequest(http.MethodGet, "/v1/guardrails/rule-sets?workspace_id=ws_1", nil)
	getRuleSetsResp := httptest.NewRecorder()
	mux.ServeHTTP(getRuleSetsResp, getRuleSetsReq)
	if getRuleSetsResp.Code != http.StatusOK {
		t.Fatalf("unexpected list guardrails rule-sets status: %d", getRuleSetsResp.Code)
	}

	putRuleSetBody := []byte(`{"workspace_id":"ws_1","name":"injection_patterns_v2","mode":"warn","patterns":["exfiltrate","override"],"enabled":true}`)
	putRuleSetReq := httptest.NewRequest(http.MethodPut, "/v1/guardrails/rule-sets/"+ruleSetID, bytes.NewReader(putRuleSetBody))
	putRuleSetResp := httptest.NewRecorder()
	mux.ServeHTTP(putRuleSetResp, putRuleSetReq)
	if putRuleSetResp.Code != http.StatusOK {
		t.Fatalf("unexpected update guardrails rule-set status: %d", putRuleSetResp.Code)
	}

	getEventsReq := httptest.NewRequest(http.MethodGet, "/v1/guardrails/events?workspace_id=ws_1", nil)
	getEventsResp := httptest.NewRecorder()
	mux.ServeHTTP(getEventsResp, getEventsReq)
	if getEventsResp.Code != http.StatusOK {
		t.Fatalf("unexpected guardrails events status: %d", getEventsResp.Code)
	}
	var eventsPayload map[string]any
	if err := json.Unmarshal(getEventsResp.Body.Bytes(), &eventsPayload); err != nil {
		t.Fatalf("decode guardrails events payload: %v", err)
	}
	events, ok := eventsPayload["events"].([]any)
	if !ok || len(events) < 2 {
		t.Fatalf("expected at least 2 guardrails events, got %v", eventsPayload)
	}

	deleteRuleSetReq := httptest.NewRequest(http.MethodDelete, "/v1/guardrails/rule-sets/"+ruleSetID, nil)
	deleteRuleSetResp := httptest.NewRecorder()
	mux.ServeHTTP(deleteRuleSetResp, deleteRuleSetReq)
	if deleteRuleSetResp.Code != http.StatusOK {
		t.Fatalf("unexpected delete guardrails rule-set status: %d", deleteRuleSetResp.Code)
	}
}

func TestControlMuxToolHealthFlow(t *testing.T) {
	t.Parallel()

	mux := NewMux(NewService("dev-secret"))

	getToolReq := httptest.NewRequest(http.MethodGet, "/v1/tools/health/calendar.create_event?workspace_id=ws_1", nil)
	getToolResp := httptest.NewRecorder()
	mux.ServeHTTP(getToolResp, getToolReq)
	if getToolResp.Code != http.StatusOK {
		t.Fatalf("unexpected initial tool health status: %d", getToolResp.Code)
	}

	postRuleBody := []byte(`{"workspace_id":"ws_1","tool_key":"calendar.create_event","min_score":0.6,"max_failures":4,"enabled":true}`)
	postRuleReq := httptest.NewRequest(http.MethodPost, "/v1/tools/quarantine/rules?workspace_id=ws_1", bytes.NewReader(postRuleBody))
	postRuleResp := httptest.NewRecorder()
	mux.ServeHTTP(postRuleResp, postRuleReq)
	if postRuleResp.Code != http.StatusCreated {
		t.Fatalf("unexpected create quarantine rule status: %d", postRuleResp.Code)
	}

	postOverrideBody := []byte(`{"workspace_id":"ws_1","status":"quarantined"}`)
	postOverrideReq := httptest.NewRequest(http.MethodPost, "/v1/tools/quarantine/calendar.create_event/override?workspace_id=ws_1", bytes.NewReader(postOverrideBody))
	postOverrideResp := httptest.NewRecorder()
	mux.ServeHTTP(postOverrideResp, postOverrideReq)
	if postOverrideResp.Code != http.StatusOK {
		t.Fatalf("unexpected quarantine override status: %d", postOverrideResp.Code)
	}

	getToolAfterReq := httptest.NewRequest(http.MethodGet, "/v1/tools/health/calendar.create_event?workspace_id=ws_1", nil)
	getToolAfterResp := httptest.NewRecorder()
	mux.ServeHTTP(getToolAfterResp, getToolAfterReq)
	if getToolAfterResp.Code != http.StatusOK {
		t.Fatalf("unexpected tool health status after override: %d", getToolAfterResp.Code)
	}
	var toolPayload map[string]any
	if err := json.Unmarshal(getToolAfterResp.Body.Bytes(), &toolPayload); err != nil {
		t.Fatalf("decode tool payload: %v", err)
	}
	if toolPayload["status"] != "quarantined" {
		t.Fatalf("expected quarantined tool status, got %v", toolPayload)
	}

	postRecoverBody := []byte(`{"workspace_id":"ws_1","status":"healthy"}`)
	postRecoverReq := httptest.NewRequest(http.MethodPost, "/v1/tools/quarantine/calendar.create_event/override?workspace_id=ws_1", bytes.NewReader(postRecoverBody))
	postRecoverResp := httptest.NewRecorder()
	mux.ServeHTTP(postRecoverResp, postRecoverReq)
	if postRecoverResp.Code != http.StatusOK {
		t.Fatalf("unexpected healthy override status: %d", postRecoverResp.Code)
	}

	getRulesReq := httptest.NewRequest(http.MethodGet, "/v1/tools/quarantine/rules?workspace_id=ws_1", nil)
	getRulesResp := httptest.NewRecorder()
	mux.ServeHTTP(getRulesResp, getRulesReq)
	if getRulesResp.Code != http.StatusOK {
		t.Fatalf("unexpected list quarantine rules status: %d", getRulesResp.Code)
	}
}

func TestControlMuxSessionsFlow(t *testing.T) {
	t.Parallel()

	mux := NewMux(NewService("dev-secret"))

	getSessionReq := httptest.NewRequest(http.MethodGet, "/v1/sessions/session_1?workspace_id=ws_1&user_id=user_1", nil)
	getSessionResp := httptest.NewRecorder()
	mux.ServeHTTP(getSessionResp, getSessionReq)
	if getSessionResp.Code != http.StatusOK {
		t.Fatalf("unexpected get session status: %d", getSessionResp.Code)
	}

	var sessionPayload map[string]any
	if err := json.Unmarshal(getSessionResp.Body.Bytes(), &sessionPayload); err != nil {
		t.Fatalf("decode session payload: %v", err)
	}
	if sessionPayload["id"] != "session_1" {
		t.Fatalf("unexpected session payload: %v", sessionPayload)
	}

	getActiveReq := httptest.NewRequest(http.MethodGet, "/v1/sessions/active?workspace_id=ws_1", nil)
	getActiveResp := httptest.NewRecorder()
	mux.ServeHTTP(getActiveResp, getActiveReq)
	if getActiveResp.Code != http.StatusOK {
		t.Fatalf("unexpected get active sessions status: %d", getActiveResp.Code)
	}
	var activePayload map[string]any
	if err := json.Unmarshal(getActiveResp.Body.Bytes(), &activePayload); err != nil {
		t.Fatalf("decode active payload: %v", err)
	}
	sessionsAny, ok := activePayload["sessions"].([]any)
	if !ok || len(sessionsAny) != 1 {
		t.Fatalf("unexpected active sessions payload: %v", activePayload)
	}

	getEntitiesReq := httptest.NewRequest(http.MethodGet, "/v1/sessions/session_1/entities", nil)
	getEntitiesResp := httptest.NewRecorder()
	mux.ServeHTTP(getEntitiesResp, getEntitiesReq)
	if getEntitiesResp.Code != http.StatusOK {
		t.Fatalf("unexpected get session entities status: %d", getEntitiesResp.Code)
	}
}

func TestControlMuxTemporalReasoningFlow(t *testing.T) {
	t.Parallel()

	mux := NewMux(NewService("dev-secret"))

	putConfigBody := []byte(`{"workspace_id":"ws_1","default_timezone":"UTC","max_horizon_days":180,"conflict_priority_threshold":70,"travel_speed_kph":60}`)
	putConfigReq := httptest.NewRequest(http.MethodPut, "/v1/temporal/config", bytes.NewReader(putConfigBody))
	putConfigResp := httptest.NewRecorder()
	mux.ServeHTTP(putConfigResp, putConfigReq)
	if putConfigResp.Code != http.StatusOK {
		t.Fatalf("unexpected put temporal config status: %d", putConfigResp.Code)
	}

	getConfigReq := httptest.NewRequest(http.MethodGet, "/v1/temporal/config?workspace_id=ws_1", nil)
	getConfigResp := httptest.NewRecorder()
	mux.ServeHTTP(getConfigResp, getConfigReq)
	if getConfigResp.Code != http.StatusOK {
		t.Fatalf("unexpected get temporal config status: %d", getConfigResp.Code)
	}

	postConstraintBody := []byte(`{"workspace_id":"ws_1","subject":"focus","starts_at":"2026-02-27T10:00:00Z","ends_at":"2026-02-27T11:00:00Z","priority":95}`)
	postConstraintReq := httptest.NewRequest(http.MethodPost, "/v1/temporal/constraints", bytes.NewReader(postConstraintBody))
	postConstraintResp := httptest.NewRecorder()
	mux.ServeHTTP(postConstraintResp, postConstraintReq)
	if postConstraintResp.Code != http.StatusCreated {
		t.Fatalf("unexpected create temporal constraint status: %d", postConstraintResp.Code)
	}
	var constraintPayload map[string]any
	if err := json.Unmarshal(postConstraintResp.Body.Bytes(), &constraintPayload); err != nil {
		t.Fatalf("decode constraint payload: %v", err)
	}
	constraintID, ok := constraintPayload["id"].(string)
	if !ok || constraintID == "" {
		t.Fatalf("missing constraint id payload: %v", constraintPayload)
	}

	getConstraintsReq := httptest.NewRequest(http.MethodGet, "/v1/temporal/constraints?workspace_id=ws_1", nil)
	getConstraintsResp := httptest.NewRecorder()
	mux.ServeHTTP(getConstraintsResp, getConstraintsReq)
	if getConstraintsResp.Code != http.StatusOK {
		t.Fatalf("unexpected list temporal constraints status: %d", getConstraintsResp.Code)
	}

	postConflictsBody := []byte(`{"workspace_id":"ws_1","proposed_start":"2026-02-27T10:30:00Z","proposed_end":"2026-02-27T10:40:00Z"}`)
	postConflictsReq := httptest.NewRequest(http.MethodPost, "/v1/temporal/conflicts", bytes.NewReader(postConflictsBody))
	postConflictsResp := httptest.NewRecorder()
	mux.ServeHTTP(postConflictsResp, postConflictsReq)
	if postConflictsResp.Code != http.StatusOK {
		t.Fatalf("unexpected temporal conflicts status: %d", postConflictsResp.Code)
	}
	var conflictsPayload map[string]any
	if err := json.Unmarshal(postConflictsResp.Body.Bytes(), &conflictsPayload); err != nil {
		t.Fatalf("decode conflicts payload: %v", err)
	}
	conflicts, ok := conflictsPayload["conflicts"].([]any)
	if !ok || len(conflicts) != 1 {
		t.Fatalf("unexpected conflicts payload: %v", conflictsPayload)
	}

	postResolveBody := []byte(`{"workspace_id":"ws_1","expression":"tomorrow morning","reference_date":"2026-02-27"}`)
	postResolveReq := httptest.NewRequest(http.MethodPost, "/v1/temporal/resolve", bytes.NewReader(postResolveBody))
	postResolveResp := httptest.NewRecorder()
	mux.ServeHTTP(postResolveResp, postResolveReq)
	if postResolveResp.Code != http.StatusOK {
		t.Fatalf("unexpected temporal resolve status: %d", postResolveResp.Code)
	}
	var resolvePayload map[string]any
	if err := json.Unmarshal(postResolveResp.Body.Bytes(), &resolvePayload); err != nil {
		t.Fatalf("decode resolve payload: %v", err)
	}
	if resolvePayload["resolved_date"] != "2026-02-28" {
		t.Fatalf("unexpected resolved date payload: %v", resolvePayload)
	}

	postTravelBody := []byte(`{"workspace_id":"ws_1","origin":"hq","destination":"airport","distance_km":30}`)
	postTravelReq := httptest.NewRequest(http.MethodPost, "/v1/temporal/travel-time", bytes.NewReader(postTravelBody))
	postTravelResp := httptest.NewRecorder()
	mux.ServeHTTP(postTravelResp, postTravelReq)
	if postTravelResp.Code != http.StatusOK {
		t.Fatalf("unexpected temporal travel-time status: %d", postTravelResp.Code)
	}
	var travelPayload map[string]any
	if err := json.Unmarshal(postTravelResp.Body.Bytes(), &travelPayload); err != nil {
		t.Fatalf("decode travel payload: %v", err)
	}
	if int(travelPayload["minutes"].(float64)) != 30 {
		t.Fatalf("unexpected travel-time payload: %v", travelPayload)
	}

	deleteConstraintReq := httptest.NewRequest(http.MethodDelete, "/v1/temporal/constraints/"+constraintID+"?workspace_id=ws_1", nil)
	deleteConstraintResp := httptest.NewRecorder()
	mux.ServeHTTP(deleteConstraintResp, deleteConstraintReq)
	if deleteConstraintResp.Code != http.StatusOK {
		t.Fatalf("unexpected delete temporal constraint status: %d", deleteConstraintResp.Code)
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
