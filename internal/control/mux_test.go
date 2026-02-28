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
	controlPrefixes := controlEndpointPrefixes()

	for path, methods := range doc.Paths {
		if !hasPrefix(path, controlPrefixes) {
			continue
		}
		for method := range methods {
			methodUpper := strings.ToUpper(method)
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

func TestControlMuxRejectsNonControlOpenAPIEndpoints(t *testing.T) {
	t.Parallel()

	mux := NewMux(NewService("dev-secret"))
	doc := loadOpenAPIForControlTest(t)

	nonControlPrefixes := []string{
		"/v1/gateway",
		"/v1/canvas",
	}

	for path, methods := range doc.Paths {
		if !hasPrefix(path, nonControlPrefixes) {
			continue
		}
		for method := range methods {
			methodUpper := strings.ToUpper(method)
			reqPath := concretePath(path)
			req := httptest.NewRequest(methodUpper, reqPath, nil)
			rec := httptest.NewRecorder()
			mux.ServeHTTP(rec, req)

			if rec.Code != http.StatusNotFound {
				t.Fatalf("non-control endpoint should 404 in control mux: %s %s status=%d body=%s", methodUpper, reqPath, rec.Code, rec.Body.String())
			}
		}
	}
}

func TestControlMuxSpecializedV91V92Endpoints(t *testing.T) {
	t.Parallel()

	mux := NewMux(NewService("dev-secret"))
	doc := loadOpenAPIForControlTest(t)
	requiredPrefixes := specializedControlPrefixes()

	for path, methods := range doc.Paths {
		if !hasPrefix(path, requiredPrefixes) {
			continue
		}
		for method := range methods {
			methodUpper := strings.ToUpper(method)
			reqPath := concretePath(path)
			var body *bytes.Reader
			if methodUpper == http.MethodPost || methodUpper == http.MethodPut {
				body = bytes.NewReader([]byte(`{}`))
			} else {
				body = bytes.NewReader(nil)
			}
			req := httptest.NewRequest(methodUpper, reqPath, body)
			rec := httptest.NewRecorder()
			mux.ServeHTTP(rec, req)

			if rec.Code == http.StatusNotFound || rec.Code == http.StatusMethodNotAllowed {
				t.Fatalf("specialized endpoint unresolved: %s %s status=%d", methodUpper, reqPath, rec.Code)
			}
			bodyText := rec.Body.String()
			if strings.Contains(bodyText, `"service":"control"`) && strings.Contains(bodyText, `"status":"accepted"`) {
				t.Fatalf("endpoint fell through generic handler: %s %s body=%s", methodUpper, reqPath, bodyText)
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

func TestControlMuxErrorsFlow(t *testing.T) {
	t.Parallel()

	mux := NewMux(NewService("dev-secret"))

	getTaxonomyReq := httptest.NewRequest(http.MethodGet, "/v1/errors/taxonomy", nil)
	getTaxonomyResp := httptest.NewRecorder()
	mux.ServeHTTP(getTaxonomyResp, getTaxonomyReq)
	if getTaxonomyResp.Code != http.StatusOK {
		t.Fatalf("unexpected get taxonomy status: %d", getTaxonomyResp.Code)
	}
	var taxonomyPayload map[string]any
	if err := json.Unmarshal(getTaxonomyResp.Body.Bytes(), &taxonomyPayload); err != nil {
		t.Fatalf("decode taxonomy payload: %v", err)
	}
	errorsAny, ok := taxonomyPayload["errors"].([]any)
	if !ok || len(errorsAny) == 0 {
		t.Fatalf("expected taxonomy entries: %v", taxonomyPayload)
	}

	postTemplateBody := []byte(`{"workspace_id":"ws_1","persona":"executive","code_pattern":"BUDGET_*","template":"Budget exhausted. Approval required.","status":"active"}`)
	postTemplateReq := httptest.NewRequest(http.MethodPost, "/v1/errors/templates", bytes.NewReader(postTemplateBody))
	postTemplateResp := httptest.NewRecorder()
	mux.ServeHTTP(postTemplateResp, postTemplateReq)
	if postTemplateResp.Code != http.StatusCreated {
		t.Fatalf("unexpected create template status: %d", postTemplateResp.Code)
	}

	getTemplatesReq := httptest.NewRequest(http.MethodGet, "/v1/errors/templates?workspace_id=ws_1", nil)
	getTemplatesResp := httptest.NewRecorder()
	mux.ServeHTTP(getTemplatesResp, getTemplatesReq)
	if getTemplatesResp.Code != http.StatusOK {
		t.Fatalf("unexpected list templates status: %d", getTemplatesResp.Code)
	}
}

func TestControlMuxCachingFlow(t *testing.T) {
	t.Parallel()

	mux := NewMux(NewService("dev-secret"))

	postPolicyBody := []byte(`{"workspace_id":"ws_1","cache_key":"compiled_context","ttl_seconds":600,"max_bytes":1048576,"enabled":true}`)
	postPolicyReq := httptest.NewRequest(http.MethodPost, "/v1/cache/policies", bytes.NewReader(postPolicyBody))
	postPolicyResp := httptest.NewRecorder()
	mux.ServeHTTP(postPolicyResp, postPolicyReq)
	if postPolicyResp.Code != http.StatusCreated {
		t.Fatalf("unexpected create cache policy status: %d", postPolicyResp.Code)
	}

	getPoliciesReq := httptest.NewRequest(http.MethodGet, "/v1/cache/policies?workspace_id=ws_1", nil)
	getPoliciesResp := httptest.NewRecorder()
	mux.ServeHTTP(getPoliciesResp, getPoliciesReq)
	if getPoliciesResp.Code != http.StatusOK {
		t.Fatalf("unexpected list cache policies status: %d", getPoliciesResp.Code)
	}

	getStatsReq := httptest.NewRequest(http.MethodGet, "/v1/cache/stats?workspace_id=ws_1", nil)
	getStatsResp := httptest.NewRecorder()
	mux.ServeHTTP(getStatsResp, getStatsReq)
	if getStatsResp.Code != http.StatusOK {
		t.Fatalf("unexpected cache stats status: %d", getStatsResp.Code)
	}
	var statsPayload map[string]any
	if err := json.Unmarshal(getStatsResp.Body.Bytes(), &statsPayload); err != nil {
		t.Fatalf("decode stats payload: %v", err)
	}
	if int(statsPayload["entries"].(float64)) != 1 {
		t.Fatalf("expected seeded cache entry count, got %v", statsPayload)
	}

	postInvalidateBody := []byte(`{"workspace_id":"ws_1","cache_key":"compiled_context"}`)
	postInvalidateReq := httptest.NewRequest(http.MethodPost, "/v1/cache/invalidate", bytes.NewReader(postInvalidateBody))
	postInvalidateResp := httptest.NewRecorder()
	mux.ServeHTTP(postInvalidateResp, postInvalidateReq)
	if postInvalidateResp.Code != http.StatusOK {
		t.Fatalf("unexpected cache invalidate status: %d", postInvalidateResp.Code)
	}
}

func TestControlMuxEventSchemasFlow(t *testing.T) {
	t.Parallel()

	mux := NewMux(NewService("dev-secret"))

	postVersionBody := []byte(`{"schema":"{\"type\":\"object\"}","status":"active"}`)
	postVersionReq := httptest.NewRequest(http.MethodPost, "/v1/event-schemas/BREVIO.test.event.v1/versions", bytes.NewReader(postVersionBody))
	postVersionResp := httptest.NewRecorder()
	mux.ServeHTTP(postVersionResp, postVersionReq)
	if postVersionResp.Code != http.StatusCreated {
		t.Fatalf("unexpected register schema version status: %d", postVersionResp.Code)
	}

	getTypesReq := httptest.NewRequest(http.MethodGet, "/v1/event-schemas", nil)
	getTypesResp := httptest.NewRecorder()
	mux.ServeHTTP(getTypesResp, getTypesReq)
	if getTypesResp.Code != http.StatusOK {
		t.Fatalf("unexpected list event schema types status: %d", getTypesResp.Code)
	}

	getVersionsReq := httptest.NewRequest(http.MethodGet, "/v1/event-schemas/BREVIO.test.event.v1/versions", nil)
	getVersionsResp := httptest.NewRecorder()
	mux.ServeHTTP(getVersionsResp, getVersionsReq)
	if getVersionsResp.Code != http.StatusOK {
		t.Fatalf("unexpected list event schema versions status: %d", getVersionsResp.Code)
	}

	postValidateBody := []byte(`{"event":{"type":"BREVIO.test.event.v1","version":1}}`)
	postValidateReq := httptest.NewRequest(http.MethodPost, "/v1/event-schemas/BREVIO.test.event.v1/validate", bytes.NewReader(postValidateBody))
	postValidateResp := httptest.NewRecorder()
	mux.ServeHTTP(postValidateResp, postValidateReq)
	if postValidateResp.Code != http.StatusOK {
		t.Fatalf("unexpected event schema validate status: %d", postValidateResp.Code)
	}
	var validatePayload map[string]any
	if err := json.Unmarshal(postValidateResp.Body.Bytes(), &validatePayload); err != nil {
		t.Fatalf("decode validate payload: %v", err)
	}
	if valid, ok := validatePayload["valid"].(bool); !ok || !valid {
		t.Fatalf("expected successful event validation, got %v", validatePayload)
	}
}

func TestControlMuxModelTiersFlow(t *testing.T) {
	t.Parallel()

	mux := NewMux(NewService("dev-secret"))

	postPolicyBody := []byte(`{"workspace_id":"ws_1","tier":"T3","enabled":true}`)
	postPolicyReq := httptest.NewRequest(http.MethodPost, "/v1/model-tiers/policies?workspace_id=ws_1", bytes.NewReader(postPolicyBody))
	postPolicyResp := httptest.NewRecorder()
	mux.ServeHTTP(postPolicyResp, postPolicyReq)
	if postPolicyResp.Code != http.StatusCreated {
		t.Fatalf("unexpected model tier policy create status: %d", postPolicyResp.Code)
	}

	getPoliciesReq := httptest.NewRequest(http.MethodGet, "/v1/model-tiers/policies?workspace_id=ws_1", nil)
	getPoliciesResp := httptest.NewRecorder()
	mux.ServeHTTP(getPoliciesResp, getPoliciesReq)
	if getPoliciesResp.Code != http.StatusOK {
		t.Fatalf("unexpected model tier policies status: %d", getPoliciesResp.Code)
	}

	getOverridesReq := httptest.NewRequest(http.MethodGet, "/v1/model-tiers/overrides?workspace_id=ws_1", nil)
	getOverridesResp := httptest.NewRecorder()
	mux.ServeHTTP(getOverridesResp, getOverridesReq)
	if getOverridesResp.Code != http.StatusOK {
		t.Fatalf("unexpected model tier overrides status: %d", getOverridesResp.Code)
	}
	var overridesPayload map[string]any
	if err := json.Unmarshal(getOverridesResp.Body.Bytes(), &overridesPayload); err != nil {
		t.Fatalf("decode overrides payload: %v", err)
	}
	overrides, ok := overridesPayload["overrides"].([]any)
	if !ok || len(overrides) == 0 {
		t.Fatalf("expected at least one override payload: %v", overridesPayload)
	}
}

func TestControlMuxStreamingFlow(t *testing.T) {
	t.Parallel()

	mux := NewMux(NewService("dev-secret"))

	putBody := []byte(`{"workspace_id":"ws_1","ack_enabled":true,"typing_indicator":true,"first_byte_sla_ms":450,"chunk_size_bytes":4096,"progressive_disclosure":true}`)
	putReq := httptest.NewRequest(http.MethodPut, "/v1/streaming/config", bytes.NewReader(putBody))
	putResp := httptest.NewRecorder()
	mux.ServeHTTP(putResp, putReq)
	if putResp.Code != http.StatusOK {
		t.Fatalf("unexpected streaming put status: %d", putResp.Code)
	}

	getReq := httptest.NewRequest(http.MethodGet, "/v1/streaming/config?workspace_id=ws_1", nil)
	getResp := httptest.NewRecorder()
	mux.ServeHTTP(getResp, getReq)
	if getResp.Code != http.StatusOK {
		t.Fatalf("unexpected streaming get status: %d", getResp.Code)
	}
	var payload map[string]any
	if err := json.Unmarshal(getResp.Body.Bytes(), &payload); err != nil {
		t.Fatalf("decode streaming payload: %v", err)
	}
	if int(payload["first_byte_sla_ms"].(float64)) != 450 {
		t.Fatalf("unexpected streaming config payload: %v", payload)
	}
}

func TestControlMuxComplianceFlow(t *testing.T) {
	t.Parallel()

	mux := NewMux(NewService("dev-secret"))

	postFrameworkBody := []byte(`{"workspace_id":"ws_1","key":"soc2","status":"active","version_int":1}`)
	postFrameworkReq := httptest.NewRequest(http.MethodPost, "/v1/compliance/frameworks", bytes.NewReader(postFrameworkBody))
	postFrameworkResp := httptest.NewRecorder()
	mux.ServeHTTP(postFrameworkResp, postFrameworkReq)
	if postFrameworkResp.Code != http.StatusCreated {
		t.Fatalf("unexpected compliance framework create status: %d", postFrameworkResp.Code)
	}

	getFrameworksReq := httptest.NewRequest(http.MethodGet, "/v1/compliance/frameworks?workspace_id=ws_1", nil)
	getFrameworksResp := httptest.NewRecorder()
	mux.ServeHTTP(getFrameworksResp, getFrameworksReq)
	if getFrameworksResp.Code != http.StatusOK {
		t.Fatalf("unexpected compliance frameworks get status: %d", getFrameworksResp.Code)
	}

	getEvidenceReq := httptest.NewRequest(http.MethodGet, "/v1/compliance/evidence?workspace_id=ws_1", nil)
	getEvidenceResp := httptest.NewRecorder()
	mux.ServeHTTP(getEvidenceResp, getEvidenceReq)
	if getEvidenceResp.Code != http.StatusOK {
		t.Fatalf("unexpected compliance evidence get status: %d", getEvidenceResp.Code)
	}

	postDSRBody := []byte(`{"workspace_id":"ws_1","user_id":"user_1","request_type":"deletion","status":"received","deadline_date":"2026-03-31"}`)
	postDSRReq := httptest.NewRequest(http.MethodPost, "/v1/compliance/dsr", bytes.NewReader(postDSRBody))
	postDSRResp := httptest.NewRecorder()
	mux.ServeHTTP(postDSRResp, postDSRReq)
	if postDSRResp.Code != http.StatusCreated {
		t.Fatalf("unexpected compliance dsr create status: %d", postDSRResp.Code)
	}
	var dsrPayload map[string]any
	if err := json.Unmarshal(postDSRResp.Body.Bytes(), &dsrPayload); err != nil {
		t.Fatalf("decode dsr payload: %v", err)
	}
	dsrID, ok := dsrPayload["id"].(string)
	if !ok || dsrID == "" {
		t.Fatalf("missing dsr id payload: %v", dsrPayload)
	}

	getDSRReq := httptest.NewRequest(http.MethodGet, "/v1/compliance/dsr/"+dsrID, nil)
	getDSRResp := httptest.NewRecorder()
	mux.ServeHTTP(getDSRResp, getDSRReq)
	if getDSRResp.Code != http.StatusOK {
		t.Fatalf("unexpected compliance dsr get status: %d", getDSRResp.Code)
	}

	putDSRBody := []byte(`{"status":"in_progress"}`)
	putDSRReq := httptest.NewRequest(http.MethodPut, "/v1/compliance/dsr/"+dsrID, bytes.NewReader(putDSRBody))
	putDSRResp := httptest.NewRecorder()
	mux.ServeHTTP(putDSRResp, putDSRReq)
	if putDSRResp.Code != http.StatusOK {
		t.Fatalf("unexpected compliance dsr update status: %d", putDSRResp.Code)
	}

	getDSRListReq := httptest.NewRequest(http.MethodGet, "/v1/compliance/dsr?workspace_id=ws_1", nil)
	getDSRListResp := httptest.NewRecorder()
	mux.ServeHTTP(getDSRListResp, getDSRListReq)
	if getDSRListResp.Code != http.StatusOK {
		t.Fatalf("unexpected compliance dsr list status: %d", getDSRListResp.Code)
	}
}

func TestControlMuxAdminFlow(t *testing.T) {
	t.Parallel()

	mux := NewMux(NewService("dev-secret"))

	trustReq := httptest.NewRequest(http.MethodPost, "/v1/admin/trust-scores/recalculate", nil)
	trustResp := httptest.NewRecorder()
	mux.ServeHTTP(trustResp, trustReq)
	if trustResp.Code != http.StatusAccepted {
		t.Fatalf("unexpected admin trust recalc status: %d", trustResp.Code)
	}

	lessonsReq := httptest.NewRequest(http.MethodPost, "/v1/admin/learning/lessons/bulk-retire", nil)
	lessonsResp := httptest.NewRecorder()
	mux.ServeHTTP(lessonsResp, lessonsReq)
	if lessonsResp.Code != http.StatusAccepted {
		t.Fatalf("unexpected admin lessons bulk-retire status: %d", lessonsResp.Code)
	}

	putUserBody := []byte(`{"email":"operator@brev.io","role":"operator","status":"active"}`)
	putUserReq := httptest.NewRequest(http.MethodPut, "/v1/admin/users/user_1", bytes.NewReader(putUserBody))
	putUserResp := httptest.NewRecorder()
	mux.ServeHTTP(putUserResp, putUserReq)
	if putUserResp.Code != http.StatusOK {
		t.Fatalf("unexpected admin put user status: %d", putUserResp.Code)
	}

	getUsersReq := httptest.NewRequest(http.MethodGet, "/v1/admin/users", nil)
	getUsersResp := httptest.NewRecorder()
	mux.ServeHTTP(getUsersResp, getUsersReq)
	if getUsersResp.Code != http.StatusOK {
		t.Fatalf("unexpected admin users list status: %d", getUsersResp.Code)
	}

	getUserReq := httptest.NewRequest(http.MethodGet, "/v1/admin/users/user_1", nil)
	getUserResp := httptest.NewRecorder()
	mux.ServeHTTP(getUserResp, getUserReq)
	if getUserResp.Code != http.StatusOK {
		t.Fatalf("unexpected admin get user status: %d", getUserResp.Code)
	}

	getUserSessionsReq := httptest.NewRequest(http.MethodGet, "/v1/admin/users/user_1/sessions", nil)
	getUserSessionsResp := httptest.NewRecorder()
	mux.ServeHTTP(getUserSessionsResp, getUserSessionsReq)
	if getUserSessionsResp.Code != http.StatusOK {
		t.Fatalf("unexpected admin user sessions status: %d", getUserSessionsResp.Code)
	}

	for _, path := range []string{
		"/v1/admin/operations/dashboard",
		"/v1/admin/operations/workflows",
		"/v1/admin/operations/queues",
		"/v1/admin/costs/summary",
		"/v1/admin/costs/anomalies",
		"/v1/admin/costs/budgets",
		"/v1/admin/kpi/report",
	} {
		req := httptest.NewRequest(http.MethodGet, path, nil)
		resp := httptest.NewRecorder()
		mux.ServeHTTP(resp, req)
		if resp.Code != http.StatusOK {
			t.Fatalf("unexpected admin get status for %s: %d", path, resp.Code)
		}
	}

	putBudgetBody := []byte(`{"workspace_id":"default","monthly_cap":2000,"current_cost":300,"currency":"USD"}`)
	putBudgetReq := httptest.NewRequest(http.MethodPut, "/v1/admin/costs/budgets", bytes.NewReader(putBudgetBody))
	putBudgetResp := httptest.NewRecorder()
	mux.ServeHTTP(putBudgetResp, putBudgetReq)
	if putBudgetResp.Code != http.StatusOK {
		t.Fatalf("unexpected admin put budget status: %d", putBudgetResp.Code)
	}

	postRuleBody := []byte(`{"name":"error_spike","metric":"error_rate_pct","threshold":1.0,"comparator":">","enabled":true}`)
	postRuleReq := httptest.NewRequest(http.MethodPost, "/v1/admin/alerts/rules", bytes.NewReader(postRuleBody))
	postRuleResp := httptest.NewRecorder()
	mux.ServeHTTP(postRuleResp, postRuleReq)
	if postRuleResp.Code != http.StatusCreated {
		t.Fatalf("unexpected admin create alert rule status: %d", postRuleResp.Code)
	}
	var rulePayload map[string]any
	if err := json.Unmarshal(postRuleResp.Body.Bytes(), &rulePayload); err != nil {
		t.Fatalf("decode rule payload: %v", err)
	}
	ruleID, ok := rulePayload["id"].(string)
	if !ok || ruleID == "" {
		t.Fatalf("missing alert rule id payload: %v", rulePayload)
	}

	putRuleBody := []byte(`{"name":"error_spike_v2","metric":"error_rate_pct","threshold":1.2,"comparator":">","enabled":true}`)
	putRuleReq := httptest.NewRequest(http.MethodPut, "/v1/admin/alerts/rules/"+ruleID, bytes.NewReader(putRuleBody))
	putRuleResp := httptest.NewRecorder()
	mux.ServeHTTP(putRuleResp, putRuleReq)
	if putRuleResp.Code != http.StatusOK {
		t.Fatalf("unexpected admin update alert rule status: %d", putRuleResp.Code)
	}

	deleteRuleReq := httptest.NewRequest(http.MethodDelete, "/v1/admin/alerts/rules/"+ruleID, nil)
	deleteRuleResp := httptest.NewRecorder()
	mux.ServeHTTP(deleteRuleResp, deleteRuleReq)
	if deleteRuleResp.Code != http.StatusOK {
		t.Fatalf("unexpected admin delete alert rule status: %d", deleteRuleResp.Code)
	}

	postChannelBody := []byte(`{"type":"email","target":"ops@brev.io","enabled":true}`)
	postChannelReq := httptest.NewRequest(http.MethodPost, "/v1/admin/alerts/channels", bytes.NewReader(postChannelBody))
	postChannelResp := httptest.NewRecorder()
	mux.ServeHTTP(postChannelResp, postChannelReq)
	if postChannelResp.Code != http.StatusCreated {
		t.Fatalf("unexpected admin create alert channel status: %d", postChannelResp.Code)
	}

	getChannelsReq := httptest.NewRequest(http.MethodGet, "/v1/admin/alerts/channels", nil)
	getChannelsResp := httptest.NewRecorder()
	mux.ServeHTTP(getChannelsResp, getChannelsReq)
	if getChannelsResp.Code != http.StatusOK {
		t.Fatalf("unexpected admin list alert channels status: %d", getChannelsResp.Code)
	}
}

func TestControlMuxV91CoreFlow(t *testing.T) {
	t.Parallel()

	mux := NewMux(NewService("dev-secret"))

	postGoalBody := []byte(`{"workspace_id":"ws_1","title":"Close v9.1 gaps","status":"active","priority":"high"}`)
	postGoalReq := httptest.NewRequest(http.MethodPost, "/v1/goals", bytes.NewReader(postGoalBody))
	postGoalResp := httptest.NewRecorder()
	mux.ServeHTTP(postGoalResp, postGoalReq)
	if postGoalResp.Code != http.StatusCreated {
		t.Fatalf("unexpected goals create status: %d", postGoalResp.Code)
	}
	var goalPayload map[string]any
	if err := json.Unmarshal(postGoalResp.Body.Bytes(), &goalPayload); err != nil {
		t.Fatalf("decode goal payload: %v", err)
	}
	goalID, ok := goalPayload["id"].(string)
	if !ok || goalID == "" {
		t.Fatalf("missing goal id payload: %v", goalPayload)
	}

	postMilestoneBody := []byte(`{"title":"Wire endpoints","status":"pending"}`)
	postMilestoneReq := httptest.NewRequest(http.MethodPost, "/v1/goals/"+goalID+"/milestones", bytes.NewReader(postMilestoneBody))
	postMilestoneResp := httptest.NewRecorder()
	mux.ServeHTTP(postMilestoneResp, postMilestoneReq)
	if postMilestoneResp.Code != http.StatusCreated {
		t.Fatalf("unexpected milestone create status: %d", postMilestoneResp.Code)
	}

	getProgressReq := httptest.NewRequest(http.MethodGet, "/v1/goals/"+goalID+"/progress", nil)
	getProgressResp := httptest.NewRecorder()
	mux.ServeHTTP(getProgressResp, getProgressReq)
	if getProgressResp.Code != http.StatusOK {
		t.Fatalf("unexpected goal progress status: %d", getProgressResp.Code)
	}

	putMCConfigBody := []byte(`{"refresh_cadence_minutes":20}`)
	putMCConfigReq := httptest.NewRequest(http.MethodPut, "/v1/mission-control/config?workspace_id=ws_1", bytes.NewReader(putMCConfigBody))
	putMCConfigResp := httptest.NewRecorder()
	mux.ServeHTTP(putMCConfigResp, putMCConfigReq)
	if putMCConfigResp.Code != http.StatusOK {
		t.Fatalf("unexpected mission-control config status: %d", putMCConfigResp.Code)
	}

	putMCWidgetsBody := []byte(`{"widgets":[{"widget_key":"goals_overview","enabled":true,"position":1}]}`)
	putMCWidgetsReq := httptest.NewRequest(http.MethodPut, "/v1/mission-control/widgets?workspace_id=ws_1", bytes.NewReader(putMCWidgetsBody))
	putMCWidgetsResp := httptest.NewRecorder()
	mux.ServeHTTP(putMCWidgetsResp, putMCWidgetsReq)
	if putMCWidgetsResp.Code != http.StatusOK {
		t.Fatalf("unexpected mission-control widgets status: %d", putMCWidgetsResp.Code)
	}

	getMCSnapshotReq := httptest.NewRequest(http.MethodGet, "/v1/mission-control/snapshot?workspace_id=ws_1", nil)
	getMCSnapshotResp := httptest.NewRecorder()
	mux.ServeHTTP(getMCSnapshotResp, getMCSnapshotReq)
	if getMCSnapshotResp.Code != http.StatusOK {
		t.Fatalf("unexpected mission-control snapshot status: %d", getMCSnapshotResp.Code)
	}

	getTrustReq := httptest.NewRequest(http.MethodGet, "/v1/autonomy/trust-scores", nil)
	getTrustResp := httptest.NewRecorder()
	mux.ServeHTTP(getTrustResp, getTrustReq)
	if getTrustResp.Code != http.StatusOK {
		t.Fatalf("unexpected trust scores status: %d", getTrustResp.Code)
	}

	getPromotionsReq := httptest.NewRequest(http.MethodGet, "/v1/autonomy/promotions", nil)
	getPromotionsResp := httptest.NewRecorder()
	mux.ServeHTTP(getPromotionsResp, getPromotionsReq)
	if getPromotionsResp.Code != http.StatusOK {
		t.Fatalf("unexpected promotions status: %d", getPromotionsResp.Code)
	}
	var promotionsPayload map[string]any
	if err := json.Unmarshal(getPromotionsResp.Body.Bytes(), &promotionsPayload); err != nil {
		t.Fatalf("decode promotions payload: %v", err)
	}
	promotions, ok := promotionsPayload["promotions"].([]any)
	if !ok || len(promotions) == 0 {
		t.Fatalf("expected promotion payload: %v", promotionsPayload)
	}
	firstPromotion, ok := promotions[0].(map[string]any)
	if !ok {
		t.Fatalf("unexpected promotion item payload: %v", promotions[0])
	}
	promotionID, _ := firstPromotion["id"].(string)

	postDecisionBody := []byte(`{"decision":"approve"}`)
	postDecisionReq := httptest.NewRequest(http.MethodPost, "/v1/autonomy/promotions/"+promotionID+"/decide", bytes.NewReader(postDecisionBody))
	postDecisionResp := httptest.NewRecorder()
	mux.ServeHTTP(postDecisionResp, postDecisionReq)
	if postDecisionResp.Code != http.StatusOK {
		t.Fatalf("unexpected promotion decide status: %d", postDecisionResp.Code)
	}

	putLearningCfgBody := []byte(`{"max_active_lessons":10,"auto_apply_lessons":false}`)
	putLearningCfgReq := httptest.NewRequest(http.MethodPut, "/v1/learning/config?workspace_id=ws_1", bytes.NewReader(putLearningCfgBody))
	putLearningCfgResp := httptest.NewRecorder()
	mux.ServeHTTP(putLearningCfgResp, putLearningCfgReq)
	if putLearningCfgResp.Code != http.StatusOK {
		t.Fatalf("unexpected learning config status: %d", putLearningCfgResp.Code)
	}

	postFeedbackBody := []byte(`{"feedback_type":"positive","content":"Great execution quality"}`)
	postFeedbackReq := httptest.NewRequest(http.MethodPost, "/v1/learning/feedback?workspace_id=ws_1", bytes.NewReader(postFeedbackBody))
	postFeedbackResp := httptest.NewRecorder()
	mux.ServeHTTP(postFeedbackResp, postFeedbackReq)
	if postFeedbackResp.Code != http.StatusCreated {
		t.Fatalf("unexpected learning feedback status: %d", postFeedbackResp.Code)
	}

	getLessonsReq := httptest.NewRequest(http.MethodGet, "/v1/learning/lessons?workspace_id=ws_1", nil)
	getLessonsResp := httptest.NewRecorder()
	mux.ServeHTTP(getLessonsResp, getLessonsReq)
	if getLessonsResp.Code != http.StatusOK {
		t.Fatalf("unexpected lessons list status: %d", getLessonsResp.Code)
	}
	var lessonsPayload map[string]any
	if err := json.Unmarshal(getLessonsResp.Body.Bytes(), &lessonsPayload); err != nil {
		t.Fatalf("decode lessons payload: %v", err)
	}
	lessons, ok := lessonsPayload["lessons"].([]any)
	if !ok || len(lessons) == 0 {
		t.Fatalf("expected lessons payload: %v", lessonsPayload)
	}
	firstLesson, ok := lessons[0].(map[string]any)
	if !ok {
		t.Fatalf("unexpected lesson payload: %v", lessons[0])
	}
	lessonID, _ := firstLesson["id"].(string)

	confirmReq := httptest.NewRequest(http.MethodPost, "/v1/learning/lessons/"+lessonID+"/confirm", nil)
	confirmResp := httptest.NewRecorder()
	mux.ServeHTTP(confirmResp, confirmReq)
	if confirmResp.Code != http.StatusOK {
		t.Fatalf("unexpected lesson confirm status: %d", confirmResp.Code)
	}

	retireReq := httptest.NewRequest(http.MethodPost, "/v1/learning/lessons/"+lessonID+"/retire", nil)
	retireResp := httptest.NewRecorder()
	mux.ServeHTTP(retireResp, retireReq)
	if retireResp.Code != http.StatusOK {
		t.Fatalf("unexpected lesson retire status: %d", retireResp.Code)
	}

	getCapturesReq := httptest.NewRequest(http.MethodGet, "/v1/captures/daily?workspace_id=ws_1", nil)
	getCapturesResp := httptest.NewRecorder()
	mux.ServeHTTP(getCapturesResp, getCapturesReq)
	if getCapturesResp.Code != http.StatusOK {
		t.Fatalf("unexpected captures list status: %d", getCapturesResp.Code)
	}

	getCaptureByDateReq := httptest.NewRequest(http.MethodGet, "/v1/captures/daily/2026-02-27?workspace_id=ws_1", nil)
	getCaptureByDateResp := httptest.NewRecorder()
	mux.ServeHTTP(getCaptureByDateResp, getCaptureByDateReq)
	if getCaptureByDateResp.Code != http.StatusOK {
		t.Fatalf("unexpected capture-by-date status: %d", getCaptureByDateResp.Code)
	}
	var captureByDate map[string]any
	if err := json.Unmarshal(getCaptureByDateResp.Body.Bytes(), &captureByDate); err != nil {
		t.Fatalf("decode capture-by-date payload: %v", err)
	}
	if captureByDate["capture_date"] != "2026-02-27" {
		t.Fatalf("unexpected capture date payload: %v", captureByDate)
	}
	if _, ok := captureByDate["wins"].([]any); !ok {
		t.Fatalf("expected wins array payload: %v", captureByDate)
	}
}

func TestControlMuxV91CodebaseFlow(t *testing.T) {
	t.Parallel()

	mux := NewMux(NewService("dev-secret"))

	for _, path := range []string{"/v1/codebase/dependencies", "/v1/codebase/patterns", "/v1/codebase/debt"} {
		req := httptest.NewRequest(http.MethodGet, path+"?workspace_id=ws_1", nil)
		resp := httptest.NewRecorder()
		mux.ServeHTTP(resp, req)
		if resp.Code != http.StatusOK {
			t.Fatalf("unexpected status for %s: %d", path, resp.Code)
		}
	}

	putDebtBody := []byte(`{"title":"refactor handlers","severity":"high","status":"open"}`)
	putDebtReq := httptest.NewRequest(http.MethodPut, "/v1/codebase/debt/debt_1?workspace_id=ws_1", bytes.NewReader(putDebtBody))
	putDebtResp := httptest.NewRecorder()
	mux.ServeHTTP(putDebtResp, putDebtReq)
	if putDebtResp.Code != http.StatusOK {
		t.Fatalf("unexpected debt upsert status: %d", putDebtResp.Code)
	}

	postTaskBody := []byte(`{"title":"extract shared parsing","status":"open"}`)
	postTaskReq := httptest.NewRequest(http.MethodPost, "/v1/codebase/debt/debt_1/tasks?workspace_id=ws_1", bytes.NewReader(postTaskBody))
	postTaskResp := httptest.NewRecorder()
	mux.ServeHTTP(postTaskResp, postTaskReq)
	if postTaskResp.Code != http.StatusCreated {
		t.Fatalf("unexpected debt task create status: %d", postTaskResp.Code)
	}
	var taskPayload map[string]any
	if err := json.Unmarshal(postTaskResp.Body.Bytes(), &taskPayload); err != nil {
		t.Fatalf("decode task payload: %v", err)
	}
	taskID, ok := taskPayload["id"].(string)
	if !ok || taskID == "" {
		t.Fatalf("missing debt task id payload: %v", taskPayload)
	}

	getTaskReq := httptest.NewRequest(http.MethodGet, "/v1/codebase/debt/debt_1/tasks/"+taskID+"?workspace_id=ws_1", nil)
	getTaskResp := httptest.NewRecorder()
	mux.ServeHTTP(getTaskResp, getTaskReq)
	if getTaskResp.Code != http.StatusOK {
		t.Fatalf("unexpected debt task get status: %d", getTaskResp.Code)
	}

	putTaskBody := []byte(`{"title":"extract shared parsing","status":"completed"}`)
	putTaskReq := httptest.NewRequest(http.MethodPut, "/v1/codebase/debt/debt_1/tasks/"+taskID+"?workspace_id=ws_1", bytes.NewReader(putTaskBody))
	putTaskResp := httptest.NewRecorder()
	mux.ServeHTTP(putTaskResp, putTaskReq)
	if putTaskResp.Code != http.StatusOK {
		t.Fatalf("unexpected debt task update status: %d", putTaskResp.Code)
	}

	postTemplateBody := []byte(`{"name":"go_service_template","status":"active"}`)
	postTemplateReq := httptest.NewRequest(http.MethodPost, "/v1/codebase/templates?workspace_id=ws_1", bytes.NewReader(postTemplateBody))
	postTemplateResp := httptest.NewRecorder()
	mux.ServeHTTP(postTemplateResp, postTemplateReq)
	if postTemplateResp.Code != http.StatusCreated {
		t.Fatalf("unexpected template create status: %d", postTemplateResp.Code)
	}

	postExportBody := []byte(`{"format":"markdown","status":"completed"}`)
	postExportReq := httptest.NewRequest(http.MethodPost, "/v1/codebase/context-export?workspace_id=ws_1", bytes.NewReader(postExportBody))
	postExportResp := httptest.NewRecorder()
	mux.ServeHTTP(postExportResp, postExportReq)
	if postExportResp.Code != http.StatusCreated {
		t.Fatalf("unexpected context export create status: %d", postExportResp.Code)
	}
	var exportPayload map[string]any
	if err := json.Unmarshal(postExportResp.Body.Bytes(), &exportPayload); err != nil {
		t.Fatalf("decode context export payload: %v", err)
	}
	exportID, ok := exportPayload["id"].(string)
	if !ok || exportID == "" {
		t.Fatalf("missing context export id payload: %v", exportPayload)
	}

	getExportReq := httptest.NewRequest(http.MethodGet, "/v1/codebase/context-export/"+exportID+"?workspace_id=ws_1", nil)
	getExportResp := httptest.NewRecorder()
	mux.ServeHTTP(getExportResp, getExportReq)
	if getExportResp.Code != http.StatusOK {
		t.Fatalf("unexpected context export get status: %d", getExportResp.Code)
	}

	getRecsReq := httptest.NewRequest(http.MethodGet, "/v1/capabilities/recommendations?workspace_id=ws_1", nil)
	getRecsResp := httptest.NewRecorder()
	mux.ServeHTTP(getRecsResp, getRecsReq)
	if getRecsResp.Code != http.StatusOK {
		t.Fatalf("unexpected capability recommendations status: %d", getRecsResp.Code)
	}
	var recsPayload map[string]any
	if err := json.Unmarshal(getRecsResp.Body.Bytes(), &recsPayload); err != nil {
		t.Fatalf("decode recommendations payload: %v", err)
	}
	recs, ok := recsPayload["recommendations"].([]any)
	if !ok || len(recs) == 0 {
		t.Fatalf("expected recommendations payload: %v", recsPayload)
	}
	firstRec, ok := recs[0].(map[string]any)
	if !ok {
		t.Fatalf("unexpected recommendation payload: %v", recs[0])
	}
	recID, _ := firstRec["id"].(string)

	postRecDecisionBody := []byte(`{"decision":"accept"}`)
	postRecDecisionReq := httptest.NewRequest(http.MethodPost, "/v1/capabilities/recommendations/"+recID+"/decide", bytes.NewReader(postRecDecisionBody))
	postRecDecisionResp := httptest.NewRecorder()
	mux.ServeHTTP(postRecDecisionResp, postRecDecisionReq)
	if postRecDecisionResp.Code != http.StatusOK {
		t.Fatalf("unexpected recommendation decide status: %d", postRecDecisionResp.Code)
	}

	putPolicyBody := []byte(`{"enabled":true,"require_approval":true,"max_allowed_risk":"elevated"}`)
	putPolicyReq := httptest.NewRequest(http.MethodPut, "/v1/self-modification/policy?workspace_id=ws_1", bytes.NewReader(putPolicyBody))
	putPolicyResp := httptest.NewRecorder()
	mux.ServeHTTP(putPolicyResp, putPolicyReq)
	if putPolicyResp.Code != http.StatusOK {
		t.Fatalf("unexpected self-mod policy put status: %d", putPolicyResp.Code)
	}

	getPolicyReq := httptest.NewRequest(http.MethodGet, "/v1/self-modification/policy?workspace_id=ws_1", nil)
	getPolicyResp := httptest.NewRecorder()
	mux.ServeHTTP(getPolicyResp, getPolicyReq)
	if getPolicyResp.Code != http.StatusOK {
		t.Fatalf("unexpected self-mod policy get status: %d", getPolicyResp.Code)
	}

	invalidPolicyBody := []byte(`{"enabled":true,"require_approval":false,"max_allowed_risk":"unknown"}`)
	invalidPolicyReq := httptest.NewRequest(http.MethodPut, "/v1/self-modification/policy?workspace_id=ws_1", bytes.NewReader(invalidPolicyBody))
	invalidPolicyResp := httptest.NewRecorder()
	mux.ServeHTTP(invalidPolicyResp, invalidPolicyReq)
	if invalidPolicyResp.Code != http.StatusBadRequest {
		t.Fatalf("expected invalid risk rejection, got status: %d", invalidPolicyResp.Code)
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

func hasPrefix(path string, prefixes []string) bool {
	for _, prefix := range prefixes {
		if strings.HasPrefix(path, prefix) {
			return true
		}
	}
	return false
}

func specializedControlPrefixes() []string {
	return []string{
		"/v1/goals",
		"/v1/mission-control",
		"/v1/autonomy",
		"/v1/learning",
		"/v1/captures",
		"/v1/codebase",
		"/v1/capabilities",
		"/v1/self-modification",
		"/v1/context",
		"/v1/rag",
		"/v1/sessions",
		"/v1/temporal",
		"/v1/guardrails",
		"/v1/tools",
		"/v1/flags",
		"/v1/streaming",
		"/v1/errors",
		"/v1/compliance",
		"/v1/cache",
		"/v1/model-tiers",
		"/v1/event-schemas",
		"/v1/admin",
	}
}

func controlEndpointPrefixes() []string {
	return append([]string{"/healthz/ready", "/healthz/live"}, specializedControlPrefixes()...)
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
