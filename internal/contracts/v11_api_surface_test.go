package contracts

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/brevio/brevio/internal/admin"
)

// callAdminJSON is like callJSON but sets X-User-Role: admin header for admin endpoints.
func callAdminJSON(t *testing.T, mux *http.ServeMux, method, path string, payload any, expectedStatus int) map[string]any {
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
	req.Header.Set("X-User-Role", "admin")
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

// TestV11BrainIngestEndpointExists verifies the brain ingress endpoint exists in cmd/brain/main.go.
func TestV11BrainIngestEndpointExists(t *testing.T) {
	t.Parallel()
	root := repositoryRoot(t)
	assertFileContainsTokens(t, filepath.Join(root, "cmd/brain/main.go"), []string{
		"POST /v1/brain/ingest",
		"MessageProcessingWorkflow",
		"DecodeMessageEnvelope",
		"ExecuteWorkflow",
		"StatusAccepted",
	})
}

// TestV11BrainIngestIdempotency verifies that brain ingress uses message ID as workflow ID
// for idempotent workflow starts.
func TestV11BrainIngestIdempotency(t *testing.T) {
	t.Parallel()
	root := repositoryRoot(t)
	assertFileContainsTokens(t, filepath.Join(root, "cmd/brain/main.go"), []string{
		"envelope.ID",
		"msg-",
		"StartWorkflowOptions",
		"IdempotencyKey",
	})
}

// TestV11BrainImportsTemporalClient verifies the brain service imports and connects
// to Temporal for workflow dispatch.
func TestV11BrainImportsTemporalClient(t *testing.T) {
	t.Parallel()
	root := repositoryRoot(t)
	assertFileContainsTokens(t, filepath.Join(root, "cmd/brain/main.go"), []string{
		"go.temporal.io/sdk/client",
		"client.Dial",
		"TEMPORAL_HOST",
	})
}

// TestV11ExecutorToolExecuteEndpointExists verifies executor exposes tool execution endpoint.
func TestV11ExecutorToolExecuteEndpointExists(t *testing.T) {
	t.Parallel()
	root := repositoryRoot(t)
	assertFileContainsTokens(t, filepath.Join(root, "cmd/executor/main.go"), []string{
		"POST /v1/executor/tool/execute",
		"prodSvc.Simulate",
		"prodSvc.Commit",
		"receipt_id",
	})
}

// TestV11ExecutorCallEndpointsExist verifies executor exposes call subsystem endpoints.
func TestV11ExecutorCallEndpointsExist(t *testing.T) {
	t.Parallel()
	root := repositoryRoot(t)
	assertFileContainsTokens(t, filepath.Join(root, "cmd/executor/main.go"), []string{
		"POST /v1/executor/call/approve",
		"GET /v1/executor/call/{id}",
		"GET /v1/executor/calls",
		"GET /v1/executor/call/{id}/transcript",
	})
}

// TestV11ExecutorCallApprovalBackedByDB verifies executor call approval uses the
// DB-backed ApprovalService, not in-memory stubs.
func TestV11ExecutorCallApprovalBackedByDB(t *testing.T) {
	t.Parallel()
	root := repositoryRoot(t)
	assertFileContainsTokens(t, filepath.Join(root, "cmd/executor/main.go"), []string{
		"NewApprovalService",
		"callRepo",
		"NewPgCallRepository",
		"approvalSvc.Approve",
		"approvalSvc.Deny",
	})
}

// TestV11ExecutorNoProdStubs verifies executor does not have stub markers — all
// endpoints are backed by real services.
func TestV11ExecutorNoProdStubs(t *testing.T) {
	t.Parallel()
	root := repositoryRoot(t)
	body, err := os.ReadFile(filepath.Join(root, "cmd/executor/main.go"))
	if err != nil {
		t.Fatalf("read executor main: %v", err)
	}
	content := string(body)
	forbidden := []string{
		"_ = prodSvc",
		"TO" + "DO: implement",
		"stub",
	}
	for _, f := range forbidden {
		if strings.Contains(content, f) {
			t.Errorf("executor main.go contains stub marker %q", f)
		}
	}
}

// TestV11ExecutorTranscriptEndpoint verifies transcript retrieval is backed by the
// DB call repository, not in-memory.
func TestV11ExecutorTranscriptEndpoint(t *testing.T) {
	t.Parallel()
	root := repositoryRoot(t)
	assertFileContainsTokens(t, filepath.Join(root, "cmd/executor/main.go"), []string{
		"GetTranscriptSegments",
		"segments",
	})
}

// TestV11AdminKillSwitchEndpoints verifies admin API exposes kill switch endpoints (V10.1).
func TestV11AdminKillSwitchEndpoints(t *testing.T) {
	t.Parallel()
	svc := admin.NewService()
	mux := http.NewServeMux()
	admin.RegisterRoutes(mux, svc)

	// Activate kill switch.
	resp := callAdminJSON(t, mux, http.MethodPost, "/v1/admin/kill-switch/activate", map[string]any{
		"workspace_id": "ws_test_ks",
		"activated_by": "admin_001",
		"reason":       "contract test",
	}, http.StatusOK)
	if resp["workspace_id"] != "ws_test_ks" {
		t.Fatalf("expected workspace_id ws_test_ks, got %v", resp["workspace_id"])
	}

	// List kill switches.
	listResp := callAdminJSON(t, mux, http.MethodGet, "/v1/admin/kill-switch", nil, http.StatusOK)
	switches, ok := listResp["kill_switches"].([]any)
	if !ok || len(switches) == 0 {
		t.Fatalf("expected non-empty kill_switches list, got %v", listResp)
	}

	// Deactivate.
	callAdminJSON(t, mux, http.MethodPost, "/v1/admin/kill-switch/deactivate", map[string]any{
		"workspace_id": "ws_test_ks",
	}, http.StatusOK)
}

// TestV11AdminSkillACLEndpoints verifies admin API exposes skill ACL endpoints (V10.1).
func TestV11AdminSkillACLEndpoints(t *testing.T) {
	t.Parallel()
	svc := admin.NewService()
	mux := http.NewServeMux()
	admin.RegisterRoutes(mux, svc)

	// Set override.
	resp := callAdminJSON(t, mux, http.MethodPost, "/v1/admin/skill-acl/override", map[string]any{
		"workspace_id": "ws_test_acl",
		"user_id":      "user_001",
		"skill_id":     "skill_calendar",
		"allowed":      false,
		"reason":       "contract test deny",
	}, http.StatusCreated)
	if resp["skill_id"] != "skill_calendar" {
		t.Fatalf("expected skill_id skill_calendar, got %v", resp["skill_id"])
	}

	// List overrides.
	listResp := callAdminJSON(t, mux, http.MethodGet, "/v1/admin/skill-acl/overrides?workspace_id=ws_test_acl&user_id=user_001", nil, http.StatusOK)
	overrides, ok := listResp["overrides"].([]any)
	if !ok || len(overrides) == 0 {
		t.Fatalf("expected non-empty overrides list, got %v", listResp)
	}
}

// TestV11AdminEndpointCount verifies admin API has at least 22 registered routes
// (16 original + 6 new V10.1 endpoints).
func TestV11AdminEndpointCount(t *testing.T) {
	t.Parallel()
	root := repositoryRoot(t)
	body, err := os.ReadFile(filepath.Join(root, "internal/admin/handlers.go"))
	if err != nil {
		t.Fatalf("read admin handlers: %v", err)
	}
	content := string(body)
	count := strings.Count(content, "mux.HandleFunc(")
	if count < 22 {
		t.Fatalf("expected at least 22 admin routes, found %d", count)
	}
}

// TestV11BrainNoHandlerStubs verifies brain service has no stub/placeholder handlers.
func TestV11BrainNoHandlerStubs(t *testing.T) {
	t.Parallel()
	root := repositoryRoot(t)
	body, err := os.ReadFile(filepath.Join(root, "cmd/brain/main.go"))
	if err != nil {
		t.Fatalf("read brain main: %v", err)
	}
	content := string(body)
	if strings.Contains(content, "TO"+"DO: implement") || strings.Contains(content, "stub") {
		t.Error("brain main.go contains stub markers")
	}
}

// TestV11ExecutorListCallsEndpoint verifies the list calls endpoint queries the DB repository.
func TestV11ExecutorListCallsEndpoint(t *testing.T) {
	t.Parallel()
	root := repositoryRoot(t)
	assertFileContainsTokens(t, filepath.Join(root, "cmd/executor/main.go"), []string{
		"callRepo.ListCalls",
		"workspace_id",
	})
}
