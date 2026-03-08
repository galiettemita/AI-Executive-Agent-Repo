package admin

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func newTestServer() (*http.ServeMux, *Service) {
	svc := NewService()
	mux := http.NewServeMux()
	RegisterRoutes(mux, svc)
	return mux, svc
}

func adminRequest(method, path string, body []byte) *http.Request {
	var req *http.Request
	if body != nil {
		req = httptest.NewRequest(method, path, bytes.NewReader(body))
	} else {
		req = httptest.NewRequest(method, path, nil)
	}
	req.Header.Set("X-User-Role", "admin")
	return req
}

func nonAdminRequest(method, path string) *http.Request {
	req := httptest.NewRequest(method, path, nil)
	req.Header.Set("X-User-Role", "operator")
	return req
}

func TestForbiddenWithoutAdminRole(t *testing.T) {
	t.Parallel()
	mux, _ := newTestServer()

	endpoints := []string{
		"/v1/admin/operations/dashboard",
		"/v1/admin/operations/workflows",
		"/v1/admin/operations/queues",
		"/v1/admin/costs/summary",
		"/v1/admin/costs/anomalies",
		"/v1/admin/costs/budgets",
		"/v1/admin/security/audit-log",
		"/v1/admin/security/trust-scores",
		"/v1/admin/security/failed-auth",
		"/v1/admin/config/system",
		"/v1/admin/alerts/rules",
		"/v1/admin/alerts/channels",
		"/v1/admin/alerts/history",
		"/v1/admin/users",
		"/v1/admin/mcp-servers",
	}
	for _, ep := range endpoints {
		w := httptest.NewRecorder()
		mux.ServeHTTP(w, nonAdminRequest("GET", ep))
		if w.Code != http.StatusForbidden {
			t.Fatalf("endpoint %s: expected 403, got %d", ep, w.Code)
		}
	}
}

func TestDashboardEndpoint(t *testing.T) {
	t.Parallel()
	mux, _ := newTestServer()

	w := httptest.NewRecorder()
	mux.ServeHTTP(w, adminRequest("GET", "/v1/admin/operations/dashboard", nil))
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}

	var body map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}
	if _, ok := body["active_workflows"]; !ok {
		t.Fatal("expected active_workflows in dashboard response")
	}
}

func TestWorkflowsEndpoint(t *testing.T) {
	t.Parallel()
	mux, _ := newTestServer()

	w := httptest.NewRecorder()
	mux.ServeHTTP(w, adminRequest("GET", "/v1/admin/operations/workflows", nil))
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}

	var body []map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}
	if len(body) == 0 {
		t.Fatal("expected non-empty workflows list")
	}
}

func TestQueuesEndpoint(t *testing.T) {
	t.Parallel()
	mux, _ := newTestServer()

	w := httptest.NewRecorder()
	mux.ServeHTTP(w, adminRequest("GET", "/v1/admin/operations/queues", nil))
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
}

func TestCostSummaryEndpoint(t *testing.T) {
	t.Parallel()
	mux, _ := newTestServer()

	w := httptest.NewRecorder()
	mux.ServeHTTP(w, adminRequest("GET", "/v1/admin/costs/summary", nil))
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}

	var body map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}
	if _, ok := body["monthly_cap"]; !ok {
		t.Fatal("expected monthly_cap in cost summary")
	}
}

func TestCostAnomaliesEndpoint(t *testing.T) {
	t.Parallel()
	mux, _ := newTestServer()

	w := httptest.NewRecorder()
	mux.ServeHTTP(w, adminRequest("GET", "/v1/admin/costs/anomalies", nil))
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
}

func TestBudgetsEndpoint(t *testing.T) {
	t.Parallel()
	mux, _ := newTestServer()

	w := httptest.NewRecorder()
	mux.ServeHTTP(w, adminRequest("GET", "/v1/admin/costs/budgets", nil))
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
}

func TestAuditLogPaginated(t *testing.T) {
	t.Parallel()
	mux, svc := newTestServer()

	// Seed some alert events to populate the audit log.
	svc.UpsertAlertRule(AlertRule{Name: "test", Metric: "cpu", Threshold: 0.5, Comparator: ">", Enabled: true})
	svc.EvaluateAlertRules(map[string]float64{"cpu": 0.9})

	w := httptest.NewRecorder()
	mux.ServeHTTP(w, adminRequest("GET", "/v1/admin/security/audit-log?page=1&page_size=10", nil))
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}

	var body map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}
	if body["page"] != float64(1) {
		t.Fatalf("expected page 1, got %v", body["page"])
	}
}

func TestTrustScoresEndpoint(t *testing.T) {
	t.Parallel()
	mux, _ := newTestServer()

	w := httptest.NewRecorder()
	mux.ServeHTTP(w, adminRequest("GET", "/v1/admin/security/trust-scores", nil))
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
}

func TestFailedAuthEndpoint(t *testing.T) {
	t.Parallel()
	mux, _ := newTestServer()

	w := httptest.NewRecorder()
	mux.ServeHTTP(w, adminRequest("GET", "/v1/admin/security/failed-auth", nil))
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
}

func TestSystemConfigEndpoint(t *testing.T) {
	t.Parallel()
	mux, _ := newTestServer()

	w := httptest.NewRecorder()
	mux.ServeHTTP(w, adminRequest("GET", "/v1/admin/config/system", nil))
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}

	var body map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}
	if _, ok := body["dashboard"]; !ok {
		t.Fatal("expected dashboard in system config")
	}
	if _, ok := body["budget"]; !ok {
		t.Fatal("expected budget in system config")
	}
}

func TestListAlertRulesEndpoint(t *testing.T) {
	t.Parallel()
	mux, _ := newTestServer()

	w := httptest.NewRecorder()
	mux.ServeHTTP(w, adminRequest("GET", "/v1/admin/alerts/rules", nil))
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
}

func TestCreateAlertRuleEndpoint(t *testing.T) {
	t.Parallel()
	mux, _ := newTestServer()

	body, _ := json.Marshal(AlertRule{
		Name:      "high_error_rate",
		Metric:    "error_rate_pct",
		Threshold: 5.0,
		Enabled:   true,
	})

	w := httptest.NewRecorder()
	mux.ServeHTTP(w, adminRequest("POST", "/v1/admin/alerts/rules", body))
	if w.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d", w.Code)
	}

	var created AlertRule
	if err := json.Unmarshal(w.Body.Bytes(), &created); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}
	if created.ID == "" {
		t.Fatal("expected non-empty ID")
	}
	if created.Name != "high_error_rate" {
		t.Fatalf("unexpected name: %s", created.Name)
	}
}

func TestCreateAlertRuleInvalidBody(t *testing.T) {
	t.Parallel()
	mux, _ := newTestServer()

	w := httptest.NewRecorder()
	mux.ServeHTTP(w, adminRequest("POST", "/v1/admin/alerts/rules", []byte("not json")))
	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", w.Code)
	}
}

func TestListAlertChannelsEndpoint(t *testing.T) {
	t.Parallel()
	mux, _ := newTestServer()

	w := httptest.NewRecorder()
	mux.ServeHTTP(w, adminRequest("GET", "/v1/admin/alerts/channels", nil))
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
}

func TestAlertHistoryEndpoint(t *testing.T) {
	t.Parallel()
	mux, _ := newTestServer()

	w := httptest.NewRecorder()
	mux.ServeHTTP(w, adminRequest("GET", "/v1/admin/alerts/history", nil))
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
}

func TestListUsersEndpointPaginated(t *testing.T) {
	t.Parallel()
	mux, svc := newTestServer()

	svc.UpsertUser(User{Email: "a@b.com", Role: "admin"})
	svc.UpsertUser(User{Email: "c@d.com", Role: "operator"})

	w := httptest.NewRecorder()
	mux.ServeHTTP(w, adminRequest("GET", "/v1/admin/users?page=1&page_size=1", nil))
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}

	var body map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}
	if body["total"] != float64(2) {
		t.Fatalf("expected total 2, got %v", body["total"])
	}
	users := body["users"].([]any)
	if len(users) != 1 {
		t.Fatalf("expected 1 user per page, got %d", len(users))
	}
}

func TestMCPServersEndpoint(t *testing.T) {
	t.Parallel()
	mux, _ := newTestServer()

	w := httptest.NewRecorder()
	mux.ServeHTTP(w, adminRequest("GET", "/v1/admin/mcp-servers", nil))
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}

	var body []MCPServerHealth
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}
	if len(body) < 2 {
		t.Fatalf("expected at least 2 MCP servers, got %d", len(body))
	}
}

func TestNoRoleHeaderGets403(t *testing.T) {
	t.Parallel()
	mux, _ := newTestServer()

	req := httptest.NewRequest("GET", "/v1/admin/operations/dashboard", nil)
	// No X-User-Role header set.
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)
	if w.Code != http.StatusForbidden {
		t.Fatalf("expected 403 with no role header, got %d", w.Code)
	}
}
