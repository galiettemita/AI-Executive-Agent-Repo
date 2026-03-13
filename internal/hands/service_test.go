package hands

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"

	"github.com/brevio/brevio/internal/connectors"
)

func newTestHandsService(t *testing.T) (*Service, *FakeMCPClient) {
	t.Helper()
	kp := connectors.NewInMemoryKeyProvider("v1", make([]byte, 32))
	connSvc := connectors.NewService(kp)
	if err := connSvc.LoadSeedFile(filepath.Join("..", "connectors", "seeds", "connectors.yaml")); err != nil {
		t.Fatalf("load seed file: %v", err)
	}
	fakeMCP := NewFakeMCPClient()
	svc := NewService(connSvc, fakeMCP)
	return svc, fakeMCP
}

// ---------------------------------------------------------------------------
// Service: ListSkills
// ---------------------------------------------------------------------------

func TestListSkills_PopulatedFromRegistry(t *testing.T) {
	t.Parallel()
	svc, _ := newTestHandsService(t)

	skills := svc.ListSkills()
	if len(skills) == 0 {
		t.Fatal("expected at least one skill from registry")
	}
	t.Logf("ListSkills returned %d skills", len(skills))
}

// ---------------------------------------------------------------------------
// Service: GetSchema
// ---------------------------------------------------------------------------

func TestGetSchema_Found(t *testing.T) {
	t.Parallel()
	svc, _ := newTestHandsService(t)

	skills := svc.ListSkills()
	if len(skills) == 0 {
		t.Skip("no skills")
	}

	schema, ok := svc.GetSchema(skills[0].ID)
	if !ok {
		t.Fatalf("expected to find schema for %s", skills[0].ID)
	}
	if schema.ID != skills[0].ID {
		t.Errorf("schema ID mismatch: %s vs %s", schema.ID, skills[0].ID)
	}
}

func TestGetSchema_NotFound(t *testing.T) {
	t.Parallel()
	svc, _ := newTestHandsService(t)

	_, ok := svc.GetSchema("nonexistent.tool")
	if ok {
		t.Fatal("expected not found")
	}
}

// ---------------------------------------------------------------------------
// Service: Execute
// ---------------------------------------------------------------------------

func TestExecute_SuccessWithFakeMCP(t *testing.T) {
	t.Parallel()
	svc, fakeMCP := newTestHandsService(t)

	skills := svc.ListSkills()
	if len(skills) == 0 {
		t.Skip("no skills")
	}

	skillID := skills[0].ID
	fakeMCP.SetResponse(skillID, map[string]any{
		"result":  "meeting scheduled",
		"tool_key": skillID,
	})

	result := svc.Execute(context.Background(), ExecuteRequest{
		SkillID:        skillID,
		WorkspaceID:    "ws-test",
		ReceiptID:      "receipt-001",
		IdempotencyKey: "idem-001",
		Mode:           "commit",
		Args:           map[string]interface{}{"title": "Standup"},
	})

	if result.Status != "SUCCESS" {
		t.Fatalf("expected SUCCESS, got %s (error: %+v)", result.Status, result.Error)
	}
	if result.SkillID != skillID {
		t.Errorf("skill ID mismatch: %s vs %s", result.SkillID, skillID)
	}
	if result.LatencyMs < 0 {
		t.Error("expected non-negative latency")
	}

	calls := fakeMCP.Calls()
	if len(calls) != 1 {
		t.Fatalf("expected 1 MCP call, got %d", len(calls))
	}
	if calls[0].ToolKey != skillID {
		t.Errorf("MCP call tool_key mismatch: %s", calls[0].ToolKey)
	}

	t.Logf("Execute SUCCESS: skill=%s latency=%dms", result.SkillID, result.LatencyMs)
}

func TestExecute_SkillNotFound(t *testing.T) {
	t.Parallel()
	svc, _ := newTestHandsService(t)

	result := svc.Execute(context.Background(), ExecuteRequest{
		SkillID:     "nonexistent.tool",
		WorkspaceID: "ws-test",
		ReceiptID:   "receipt-001",
		Mode:        "commit",
	})

	if result.Status != "FAILED" {
		t.Fatalf("expected FAILED, got %s", result.Status)
	}
	if result.Error == nil || result.Error.Code != "SKILL_NOT_FOUND" {
		t.Error("expected SKILL_NOT_FOUND error")
	}
}

func TestExecute_MissingReceipt(t *testing.T) {
	t.Parallel()
	svc, _ := newTestHandsService(t)

	skills := svc.ListSkills()
	if len(skills) == 0 {
		t.Skip("no skills")
	}

	result := svc.Execute(context.Background(), ExecuteRequest{
		SkillID:     skills[0].ID,
		WorkspaceID: "ws-test",
		ReceiptID:   "",
		Mode:        "commit",
	})

	if result.Status != "FAILED" {
		t.Fatalf("expected FAILED, got %s", result.Status)
	}
	if result.Error == nil || result.Error.Code != "AUTHORIZATION_REQUIRED" {
		t.Errorf("expected AUTHORIZATION_REQUIRED error, got %+v", result.Error)
	}
}

func TestExecute_MCPError_Retryable(t *testing.T) {
	t.Parallel()
	svc, fakeMCP := newTestHandsService(t)

	skills := svc.ListSkills()
	if len(skills) == 0 {
		t.Skip("no skills")
	}

	fakeMCP.SetError(errors.New("connection refused"))

	result := svc.Execute(context.Background(), ExecuteRequest{
		SkillID:     skills[0].ID,
		WorkspaceID: "ws-test",
		ReceiptID:   "receipt-001",
		Mode:        "commit",
	})

	if result.Status != "FAILED" {
		t.Fatalf("expected FAILED, got %s", result.Status)
	}
	if result.Error == nil || !result.Error.Retryable {
		t.Error("expected retryable error for connection refused")
	}
}

func TestExecute_PayloadTooLarge(t *testing.T) {
	t.Parallel()
	svc, _ := newTestHandsService(t)

	skills := svc.ListSkills()
	if len(skills) == 0 {
		t.Skip("no skills")
	}

	// Build args larger than 512KB.
	bigValue := strings.Repeat("x", 600*1024)
	result := svc.Execute(context.Background(), ExecuteRequest{
		SkillID:     skills[0].ID,
		WorkspaceID: "ws-test",
		ReceiptID:   "receipt-001",
		Mode:        "commit",
		Args:        map[string]interface{}{"data": bigValue},
	})

	if result.Status != "FAILED" {
		t.Fatalf("expected FAILED, got %s", result.Status)
	}
	if result.Error == nil || result.Error.Code != "PAYLOAD_TOO_LARGE" {
		t.Errorf("expected PAYLOAD_TOO_LARGE error, got %+v", result.Error)
	}
}

func TestExecute_TimeoutErrorCode(t *testing.T) {
	t.Parallel()
	svc, fakeMCP := newTestHandsService(t)

	skills := svc.ListSkills()
	if len(skills) == 0 {
		t.Skip("no skills")
	}

	// Simulate a timeout error from the MCP client.
	fakeMCP.SetError(errors.New("context deadline exceeded (timeout)"))

	// Use an already-cancelled context to trigger deadline exceeded.
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	result := svc.Execute(ctx, ExecuteRequest{
		SkillID:     skills[0].ID,
		WorkspaceID: "ws-test",
		ReceiptID:   "receipt-001",
		Mode:        "commit",
		Args:        map[string]interface{}{"query": "test"},
	})

	if result.Status != "FAILED" {
		t.Fatalf("expected FAILED, got %s", result.Status)
	}
	if result.Error == nil {
		t.Fatal("expected error")
	}
	// The error should be retryable (contains "timeout").
	if !result.Error.Retryable {
		t.Error("expected timeout error to be retryable")
	}
}

func TestExecute_MCPError_NonRetryable(t *testing.T) {
	t.Parallel()
	svc, fakeMCP := newTestHandsService(t)

	skills := svc.ListSkills()
	if len(skills) == 0 {
		t.Skip("no skills")
	}

	fakeMCP.SetError(errors.New("invalid tool configuration"))

	result := svc.Execute(context.Background(), ExecuteRequest{
		SkillID:     skills[0].ID,
		WorkspaceID: "ws-test",
		ReceiptID:   "receipt-001",
		Mode:        "commit",
	})

	if result.Status != "FAILED" {
		t.Fatalf("expected FAILED, got %s", result.Status)
	}
	if result.Error == nil || result.Error.Code != "MCP_EXECUTION_FAILED" {
		t.Errorf("expected MCP_EXECUTION_FAILED, got %+v", result.Error)
	}
	if result.Error.Retryable {
		t.Error("expected non-retryable error")
	}
}

// ---------------------------------------------------------------------------
// ExecutorAdapter
// ---------------------------------------------------------------------------

func TestExecutorAdapter_Success(t *testing.T) {
	t.Parallel()
	svc, fakeMCP := newTestHandsService(t)

	skills := svc.ListSkills()
	if len(skills) == 0 {
		t.Skip("no skills")
	}
	skillID := skills[0].ID
	fakeMCP.SetResponse(skillID, map[string]any{"ok": true})

	adapter := NewExecutorAdapter(svc)
	success, output, err := adapter.ExecuteTool(
		context.Background(), skillID, "ws-test", "receipt-001", "idem-001", "commit", nil,
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !success {
		t.Error("expected success")
	}
	if output == nil {
		t.Error("expected non-nil output")
	}
}

func TestExecutorAdapter_Failure(t *testing.T) {
	t.Parallel()
	svc, _ := newTestHandsService(t)

	adapter := NewExecutorAdapter(svc)
	success, _, err := adapter.ExecuteTool(
		context.Background(), "nonexistent.tool", "ws-test", "receipt-001", "idem-001", "commit", nil,
	)
	if err == nil {
		t.Fatal("expected error")
	}
	if success {
		t.Error("expected failure")
	}
}

// ---------------------------------------------------------------------------
// HTTP Handlers
// ---------------------------------------------------------------------------

func TestHandler_ListSkills(t *testing.T) {
	t.Parallel()
	svc, _ := newTestHandsService(t)

	mux := http.NewServeMux()
	svc.RegisterRoutes(mux)

	req := httptest.NewRequest("GET", "/v1/skills", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}

	body, _ := io.ReadAll(rec.Body)
	var resp map[string]any
	json.Unmarshal(body, &resp)

	count, ok := resp["count"].(float64)
	if !ok || count == 0 {
		t.Error("expected non-zero skill count")
	}
	t.Logf("GET /v1/skills: %d skills", int(count))
}

func TestHandler_GetSchema(t *testing.T) {
	t.Parallel()
	svc, _ := newTestHandsService(t)

	mux := http.NewServeMux()
	svc.RegisterRoutes(mux)

	skills := svc.ListSkills()
	if len(skills) == 0 {
		t.Skip("no skills")
	}

	req := httptest.NewRequest("GET", "/v1/skills/"+skills[0].ID+"/schema", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
}

func TestHandler_GetSchema_NotFound(t *testing.T) {
	t.Parallel()
	svc, _ := newTestHandsService(t)

	mux := http.NewServeMux()
	svc.RegisterRoutes(mux)

	req := httptest.NewRequest("GET", "/v1/skills/nonexistent.tool/schema", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", rec.Code)
	}
}

func TestHandler_Execute(t *testing.T) {
	t.Parallel()
	svc, fakeMCP := newTestHandsService(t)

	skills := svc.ListSkills()
	if len(skills) == 0 {
		t.Skip("no skills")
	}
	skillID := skills[0].ID
	fakeMCP.SetResponse(skillID, map[string]any{"done": true})

	mux := http.NewServeMux()
	svc.RegisterRoutes(mux)

	payload := `{"workspace_id":"ws-test","receipt_id":"receipt-001","idempotency_key":"idem-001","mode":"commit","args":{"title":"test"}}`
	req := httptest.NewRequest("POST", "/v1/skills/"+skillID+"/execute", strings.NewReader(payload))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		body, _ := io.ReadAll(rec.Body)
		t.Fatalf("expected 200, got %d: %s", rec.Code, string(body))
	}

	body, _ := io.ReadAll(rec.Body)
	var result ExecuteResult
	json.Unmarshal(body, &result)
	if result.Status != "SUCCESS" {
		t.Errorf("expected SUCCESS, got %s", result.Status)
	}
	if result.SkillID != skillID {
		t.Errorf("skill ID mismatch: %s", result.SkillID)
	}

	t.Logf("POST /v1/skills/%s/execute: status=%s latency=%dms", skillID, result.Status, result.LatencyMs)
}

func TestHandler_Execute_MissingReceipt(t *testing.T) {
	t.Parallel()
	svc, _ := newTestHandsService(t)

	skills := svc.ListSkills()
	if len(skills) == 0 {
		t.Skip("no skills")
	}

	mux := http.NewServeMux()
	svc.RegisterRoutes(mux)

	payload := `{"workspace_id":"ws-test","mode":"commit"}`
	req := httptest.NewRequest("POST", "/v1/skills/"+skills[0].ID+"/execute", strings.NewReader(payload))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnprocessableEntity {
		t.Fatalf("expected 422, got %d", rec.Code)
	}
}

func TestHandler_Liveness(t *testing.T) {
	t.Parallel()
	svc, _ := newTestHandsService(t)

	mux := http.NewServeMux()
	svc.RegisterRoutes(mux)

	req := httptest.NewRequest("GET", "/healthz/live", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
}

func TestHandler_Readiness(t *testing.T) {
	t.Parallel()
	svc, _ := newTestHandsService(t)

	mux := http.NewServeMux()
	svc.RegisterRoutes(mux)

	req := httptest.NewRequest("GET", "/healthz/ready", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}

	body, _ := io.ReadAll(rec.Body)
	var resp map[string]any
	json.Unmarshal(body, &resp)
	count, _ := resp["skill_count"].(float64)
	if count == 0 {
		t.Error("expected non-zero skill count in readiness")
	}
}

// ---------------------------------------------------------------------------
// FakeMCPClient
// ---------------------------------------------------------------------------

func TestFakeMCPClient_DefaultResponse(t *testing.T) {
	t.Parallel()
	fake := NewFakeMCPClient()

	result, err := fake.Execute(context.Background(), "https://mcp.test.internal/test", "test.tool", nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result == nil {
		t.Fatal("expected non-nil result")
	}
	if len(fake.Calls()) != 1 {
		t.Errorf("expected 1 call, got %d", len(fake.Calls()))
	}
}

func TestFakeMCPClient_CustomError(t *testing.T) {
	t.Parallel()
	fake := NewFakeMCPClient()
	fake.SetError(errors.New("test error"))

	_, err := fake.Execute(context.Background(), "https://mcp.test.internal/test", "test.tool", nil)
	if err == nil {
		t.Fatal("expected error")
	}
}

// ---------------------------------------------------------------------------
// Integration: Seeded registry → Execute → MCP fake → structured output
// ---------------------------------------------------------------------------

func TestIntegration_SeedToExecute_EndToEnd(t *testing.T) {
	t.Parallel()
	svc, fakeMCP := newTestHandsService(t)

	// Verify registry was loaded.
	skills := svc.ListSkills()
	if len(skills) < 40 {
		t.Fatalf("expected at least 40 skills from registry, got %d", len(skills))
	}

	// Pick a safe read-only tool.
	var safeSkill SkillMetadata
	for _, sk := range skills {
		if !sk.WriteCapable {
			safeSkill = sk
			break
		}
	}
	if safeSkill.ID == "" {
		t.Fatal("no read-only skill found")
	}

	// Configure fake MCP to return structured output.
	fakeMCP.SetResponse(safeSkill.ID, map[string]any{
		"status":    "ok",
		"tool_key":  safeSkill.ID,
		"connector": safeSkill.ConnectorKey,
		"results":   []any{"item1", "item2"},
	})

	// Execute through the adapter (same path as Temporal activity).
	adapter := NewExecutorAdapter(svc)
	success, output, err := adapter.ExecuteTool(
		context.Background(),
		safeSkill.ID,
		"ws-integration-test",
		"receipt-integration",
		"idem-integration",
		"commit",
		map[string]interface{}{"query": "test"},
	)

	if err != nil {
		t.Fatalf("integration execution failed: %v", err)
	}
	if !success {
		t.Fatal("expected success")
	}
	if output == nil {
		t.Fatal("expected structured output")
	}

	// Verify MCP was called with correct parameters.
	calls := fakeMCP.Calls()
	if len(calls) != 1 {
		t.Fatalf("expected 1 MCP call, got %d", len(calls))
	}
	if calls[0].ToolKey != safeSkill.ID {
		t.Errorf("MCP call tool_key: %s, expected %s", calls[0].ToolKey, safeSkill.ID)
	}

	// Verify output is structured.
	outputMap, ok := output.(map[string]any)
	if !ok {
		t.Fatalf("expected map output, got %T", output)
	}
	if outputMap["tool_key"] != safeSkill.ID {
		t.Errorf("output tool_key mismatch")
	}

	t.Logf("Integration test PASS: skill=%s connector=%s domain=%s",
		safeSkill.ID, safeSkill.ConnectorKey, safeSkill.Domain)
	t.Logf("  MCP call: url=%s tool_key=%s", calls[0].ServerURL, calls[0].ToolKey)
	t.Logf("  Output: %+v", outputMap)
}
