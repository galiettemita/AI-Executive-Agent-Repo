package contracts

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"strings"
	"testing"

	"github.com/brevio/brevio/internal/control"
	"github.com/brevio/brevio/internal/executor"
)

// --- P6-T001: Executor pgx repositories ---

func TestToolExecutionRepositoryInterface(t *testing.T) {
	// Verify PgToolExecutionRepository implements ToolExecutionRepository.
	var _ executor.ToolExecutionRepository = (*executor.PgToolExecutionRepository)(nil)
}

func TestToolExecutionRepositoryMethods(t *testing.T) {
	typ := reflect.TypeOf((*executor.ToolExecutionRepository)(nil)).Elem()
	required := []string{
		"InsertExecution",
		"GetExecution",
		"InsertReceipt",
		"GetReceiptByExecution",
		"IncrementSideEffect",
		"GetSideEffectCount",
		"InsertAuditEntry",
	}
	for _, method := range required {
		if _, ok := typ.MethodByName(method); !ok {
			t.Errorf("ToolExecutionRepository missing method: %s", method)
		}
	}
}

func TestProdServiceEmbeddsService(t *testing.T) {
	// ProdService must embed *Service for backwards compatibility.
	typ := reflect.TypeOf(executor.ProdService{})
	found := false
	for i := 0; i < typ.NumField(); i++ {
		if typ.Field(i).Type == reflect.TypeOf((*executor.Service)(nil)) {
			found = true
			break
		}
	}
	if !found {
		t.Fatal("ProdService must embed *executor.Service")
	}
}

func TestProdServiceHasReceiptValidator(t *testing.T) {
	// ProdService must have a ReceiptValidator field (receipt enforcement).
	typ := reflect.TypeOf(executor.ProdService{})
	receiptValidatorType := reflect.TypeOf((*executor.ReceiptValidator)(nil)).Elem()
	found := false
	for i := 0; i < typ.NumField(); i++ {
		if typ.Field(i).Type.Implements(receiptValidatorType) ||
			reflect.PointerTo(typ.Field(i).Type).Implements(receiptValidatorType) {
			found = true
			break
		}
	}
	if !found {
		t.Fatal("ProdService must hold a ReceiptValidator for receipt enforcement")
	}
}

// --- P6-T002: Receipt enforcement ---

func TestReceiptValidatorInterface(t *testing.T) {
	// DurableReceiptService must satisfy ReceiptValidator.
	var _ executor.ReceiptValidator = (*control.DurableReceiptService)(nil)
}

func TestProdServiceCommitRequiresReceipt(t *testing.T) {
	// Verify ProdService.Commit method signature requires receiptID parameter.
	typ := reflect.TypeOf(&executor.ProdService{})
	method, ok := typ.MethodByName("Commit")
	if !ok {
		t.Fatal("ProdService missing Commit method")
	}
	// Method signature: Commit(ctx, req, receiptID) => (ToolExecution, TrustReceipt, error)
	// Receiver + 3 params = 4 inputs.
	if method.Type.NumIn() != 4 {
		t.Fatalf("ProdService.Commit expected 4 inputs (receiver + ctx + req + receiptID), got %d", method.Type.NumIn())
	}
	// Third param (index 3) must be string (receiptID).
	if method.Type.In(3).Kind() != reflect.String {
		t.Fatalf("ProdService.Commit 3rd param must be string (receiptID), got %s", method.Type.In(3))
	}
}

func TestReceiptServiceGateCount(t *testing.T) {
	// Control ReceiptService must evaluate 7 gates per spec.
	svc := control.NewReceiptService([]byte("test-key"))
	receipt, evals, err := svc.EvaluateAndIssue(control.ReceiptRequest{
		WorkspaceID:   "ws-test",
		WorkflowRunID: "wf-test",
		PlanID:        "plan-test",
		ToolKeys:      []string{"send_email"},
		RiskLevel:     "LOW",
		PolicyBundle:  "{}",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if receipt == nil {
		t.Fatal("expected receipt")
	}
	if len(evals) != 7 {
		t.Fatalf("expected 7 gate evaluations, got %d", len(evals))
	}
}

func TestReceiptValidation_MissingReceipt(t *testing.T) {
	svc := control.NewReceiptService([]byte("test-key"))
	err := svc.ValidateReceipt("", "ws-1", "send_email")
	if err == nil {
		t.Fatal("expected error for empty receipt")
	}
	if err != control.ErrNoReceipt {
		t.Fatalf("expected ErrNoReceipt, got %v", err)
	}
}

func TestReceiptValidation_NonexistentReceipt(t *testing.T) {
	svc := control.NewReceiptService([]byte("test-key"))
	err := svc.ValidateReceipt("nonexistent-id", "ws-1", "send_email")
	if err == nil {
		t.Fatal("expected error for nonexistent receipt")
	}
	if err != control.ErrNoReceipt {
		t.Fatalf("expected ErrNoReceipt, got %v", err)
	}
}

func TestReceiptValidation_ConsumedReceipt(t *testing.T) {
	svc := control.NewReceiptService([]byte("test-key"))
	receipt, _, err := svc.EvaluateAndIssue(control.ReceiptRequest{
		WorkspaceID:   "ws-1",
		WorkflowRunID: "wf-1",
		PlanID:        "plan-1",
		ToolKeys:      []string{"send_email"},
		RiskLevel:     "LOW",
		PolicyBundle:  "{}",
	})
	if err != nil {
		t.Fatalf("issue error: %v", err)
	}
	if err := svc.ConsumeReceipt(receipt.ID); err != nil {
		t.Fatalf("consume error: %v", err)
	}
	err = svc.ValidateReceipt(receipt.ID, "ws-1", "send_email")
	if err != control.ErrReceiptConsumed {
		t.Fatalf("expected ErrReceiptConsumed, got %v", err)
	}
}

func TestReceiptValidation_WorkspaceMismatch(t *testing.T) {
	svc := control.NewReceiptService([]byte("test-key"))
	receipt, _, err := svc.EvaluateAndIssue(control.ReceiptRequest{
		WorkspaceID:   "ws-1",
		WorkflowRunID: "wf-1",
		PlanID:        "plan-1",
		ToolKeys:      []string{"send_email"},
		RiskLevel:     "LOW",
		PolicyBundle:  "{}",
	})
	if err != nil {
		t.Fatalf("issue error: %v", err)
	}
	err = svc.ValidateReceipt(receipt.ID, "ws-WRONG", "send_email")
	if err != control.ErrReceiptMismatch {
		t.Fatalf("expected ErrReceiptMismatch, got %v", err)
	}
}

func TestReceiptValidation_ToolMismatch(t *testing.T) {
	svc := control.NewReceiptService([]byte("test-key"))
	receipt, _, err := svc.EvaluateAndIssue(control.ReceiptRequest{
		WorkspaceID:   "ws-1",
		WorkflowRunID: "wf-1",
		PlanID:        "plan-1",
		ToolKeys:      []string{"send_email"},
		RiskLevel:     "LOW",
		PolicyBundle:  "{}",
	})
	if err != nil {
		t.Fatalf("issue error: %v", err)
	}
	err = svc.ValidateReceipt(receipt.ID, "ws-1", "delete_database")
	if err != control.ErrReceiptMismatch {
		t.Fatalf("expected ErrReceiptMismatch, got %v", err)
	}
}

func TestReceiptValidation_KillSwitch(t *testing.T) {
	svc := control.NewReceiptService([]byte("test-key"))
	svc.ActivateKillSwitch("ws-1")
	_, _, err := svc.EvaluateAndIssue(control.ReceiptRequest{
		WorkspaceID:   "ws-1",
		WorkflowRunID: "wf-1",
		PlanID:        "plan-1",
		ToolKeys:      []string{"send_email"},
		RiskLevel:     "LOW",
		PolicyBundle:  "{}",
	})
	if err != control.ErrKillSwitchActive {
		t.Fatalf("expected ErrKillSwitchActive, got %v", err)
	}
}

// --- P6-T003: OpenClaw contract closure (JSON schema validation) ---

func TestToolCallSchemaAdditionalPropertiesFalse(t *testing.T) {
	schema := loadTestSchema(t, "tool_call.v9.json")
	if schema.AdditionalProperties == nil || *schema.AdditionalProperties {
		t.Fatal("tool_call.v9.json must have additionalProperties: false")
	}
}

func TestToolCallSchemaRequiredFields(t *testing.T) {
	schema := loadTestSchema(t, "tool_call.v9.json")
	required := map[string]bool{
		"tool_key":        false,
		"idempotency_key": false,
		"arguments":       false,
		"requested_risk":  false,
		"workspace_id":    false,
		"ingress_turn_id": false,
	}
	for _, r := range schema.Required {
		if _, ok := required[r]; ok {
			required[r] = true
		}
	}
	for field, found := range required {
		if !found {
			t.Errorf("tool_call.v9.json missing required field: %s", field)
		}
	}
}

func TestToolCallSchemaRequestedRiskClosed(t *testing.T) {
	schema := loadTestSchema(t, "tool_call.v9.json")
	riskProp, ok := schema.Properties["requested_risk"]
	if !ok {
		t.Fatal("tool_call.v9.json missing requested_risk property")
	}
	if riskProp.AdditionalProperties == nil || *riskProp.AdditionalProperties {
		t.Fatal("requested_risk must have additionalProperties: false")
	}
}

func TestToolExecutionResponseSchemaAdditionalPropertiesFalse(t *testing.T) {
	schema := loadTestSchema(t, "tool_execution_response.v1.json")
	if schema.AdditionalProperties == nil || *schema.AdditionalProperties {
		t.Fatal("tool_execution_response.v1.json must have additionalProperties: false")
	}
}

func TestToolExecutionResponseSchemaRequiredFields(t *testing.T) {
	schema := loadTestSchema(t, "tool_execution_response.v1.json")
	required := map[string]bool{
		"execution_id":    false,
		"phase":           false,
		"status":          false,
		"tool_key":        false,
		"idempotency_key": false,
	}
	for _, r := range schema.Required {
		if _, ok := required[r]; ok {
			required[r] = true
		}
	}
	for field, found := range required {
		if !found {
			t.Errorf("tool_execution_response.v1.json missing required field: %s", field)
		}
	}
}

func TestSchemaValidation_RejectsAdditionalProperties(t *testing.T) {
	schema := loadTestSchema(t, "tool_call.v9.json")
	execSchema := &executor.SchemaEnvelope{}
	// Parse schema data directly for the executor validator.
	schemasDir := findTestSchemasDir(t)
	data, err := os.ReadFile(filepath.Join(schemasDir, "tool_call.v9.json"))
	if err != nil {
		t.Fatalf("read schema: %v", err)
	}
	if err := json.Unmarshal(data, execSchema); err != nil {
		t.Fatalf("parse schema: %v", err)
	}
	_ = schema

	// Valid payload.
	validPayload := map[string]any{
		"tool_key":        "email.send",
		"idempotency_key": "1234567890abcdef",
		"arguments":       map[string]any{"to": "user@test.com"},
		"requested_risk":  map[string]any{"level": "LOW", "reversible": true},
		"workspace_id":    "0193b9a0-1234-7000-8000-000000000001",
		"ingress_turn_id": "0193b9a0-5678-7000-8000-000000000002",
	}
	violations := executor.ValidatePayloadAgainstSchema(execSchema, validPayload)
	if len(violations) != 0 {
		t.Fatalf("expected no violations for valid payload, got: %v", violations)
	}

	// Payload with additional property.
	invalidPayload := map[string]any{
		"tool_key":           "email.send",
		"idempotency_key":    "1234567890abcdef",
		"arguments":          map[string]any{},
		"requested_risk":     map[string]any{"level": "LOW", "reversible": true},
		"workspace_id":       "0193b9a0-1234-7000-8000-000000000001",
		"ingress_turn_id":    "0193b9a0-5678-7000-8000-000000000002",
		"unauthorized_field": "should_be_rejected",
	}
	violations = executor.ValidatePayloadAgainstSchema(execSchema, invalidPayload)
	if len(violations) == 0 {
		t.Fatal("expected violations for payload with additional properties")
	}
	found := false
	for _, v := range violations {
		if strings.Contains(v, "unauthorized_field") {
			found = true
		}
	}
	if !found {
		t.Fatalf("expected violation about unauthorized_field, got: %v", violations)
	}
}

func TestSchemaValidation_RequiredFieldsMissing(t *testing.T) {
	schemasDir := findTestSchemasDir(t)
	data, err := os.ReadFile(filepath.Join(schemasDir, "tool_call.v9.json"))
	if err != nil {
		t.Fatalf("read schema: %v", err)
	}
	var execSchema executor.SchemaEnvelope
	if err := json.Unmarshal(data, &execSchema); err != nil {
		t.Fatalf("parse schema: %v", err)
	}

	// Payload missing required fields.
	payload := map[string]any{
		"tool_key": "email.send",
	}
	violations := executor.ValidatePayloadAgainstSchema(&execSchema, payload)
	if len(violations) < 4 {
		t.Fatalf("expected at least 4 violations for missing required fields, got %d: %v", len(violations), violations)
	}
}

// --- P6: Migration table existence ---

func TestExecutorMigrationExists(t *testing.T) {
	migrationsDir := findExecutorMigrationsDir(t)
	name := findMigrationFile(t, migrationsDir, 52, "up")
	if name == "" {
		t.Fatal("migration 052 not found")
	}
	data, err := os.ReadFile(filepath.Join(migrationsDir, name))
	if err != nil {
		t.Fatalf("read migration 052: %v", err)
	}
	content := string(data)

	tables := []string{
		"tool_executions",
		"tool_execution_receipts",
		"tool_side_effects",
		"executor_audit_log",
	}
	for _, table := range tables {
		if !strings.Contains(content, table) {
			t.Errorf("migration 052 missing table: %s", table)
		}
	}

	// Verify idempotency constraint.
	if !strings.Contains(content, "uq_tool_execution_idempotency") {
		t.Error("migration 052 missing idempotency unique constraint")
	}

	// Verify one-receipt-per-execution constraint.
	if !strings.Contains(content, "uq_tool_execution_receipt") {
		t.Error("migration 052 missing one-receipt-per-execution unique constraint")
	}
}

// --- P6: Executor entrypoint production path ---

func TestExecutorEntrypointProductionPath(t *testing.T) {
	entrypoint := findProjectFile(t, "cmd/executor/main.go")
	data, err := os.ReadFile(entrypoint)
	if err != nil {
		t.Fatalf("read entrypoint: %v", err)
	}
	content := string(data)

	checks := []struct {
		needle string
		desc   string
	}{
		{"pgxpool.New", "must create pgx pool"},
		{"NewPgToolExecutionRepository", "must create pgx repository"},
		{"NewProdService", "must create ProdService"},
		{"NewDurableReceiptService", "must create durable receipt service"},
		{"DATABASE_URL", "must check DATABASE_URL"},
	}
	for _, check := range checks {
		if !strings.Contains(content, check.needle) {
			t.Errorf("executor entrypoint: %s (missing %q)", check.desc, check.needle)
		}
	}
}

// --- P6: No executor authoritative state in memory only ---

func TestProdServiceCommitSignatureRequiresContext(t *testing.T) {
	typ := reflect.TypeOf(&executor.ProdService{})
	method, ok := typ.MethodByName("Commit")
	if !ok {
		t.Fatal("ProdService missing Commit method")
	}
	// First param after receiver must be context.Context.
	contextType := reflect.TypeOf((*context.Context)(nil)).Elem()
	if !method.Type.In(1).Implements(contextType) {
		t.Fatal("ProdService.Commit first param must be context.Context")
	}
}

func TestProdServiceSimulateSignatureRequiresContext(t *testing.T) {
	typ := reflect.TypeOf(&executor.ProdService{})
	method, ok := typ.MethodByName("Simulate")
	if !ok {
		t.Fatal("ProdService missing Simulate method")
	}
	contextType := reflect.TypeOf((*context.Context)(nil)).Elem()
	if !method.Type.In(1).Implements(contextType) {
		t.Fatal("ProdService.Simulate first param must be context.Context")
	}
}

// --- helpers ---

type testSchema struct {
	AdditionalProperties *bool                     `json:"additionalProperties"`
	Required             []string                  `json:"required"`
	Properties           map[string]testSchemaProp `json:"properties"`
}

type testSchemaProp struct {
	Type                 string                    `json:"type"`
	AdditionalProperties *bool                     `json:"additionalProperties"`
	Required             []string                  `json:"required"`
	Properties           map[string]testSchemaProp `json:"properties"`
}

func loadTestSchema(t *testing.T, filename string) testSchema {
	t.Helper()
	schemasDir := findTestSchemasDir(t)
	data, err := os.ReadFile(filepath.Join(schemasDir, filename))
	if err != nil {
		t.Fatalf("failed to load schema %s: %v", filename, err)
	}
	var s testSchema
	if err := json.Unmarshal(data, &s); err != nil {
		t.Fatalf("failed to parse schema %s: %v", filename, err)
	}
	return s
}

func findTestSchemasDir(t *testing.T) string {
	t.Helper()
	_, filename, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("cannot determine caller file")
	}
	dir := filepath.Dir(filename)
	candidate := filepath.Join(dir, "..", "..", "schemas")
	if info, err := os.Stat(candidate); err == nil && info.IsDir() {
		return candidate
	}
	t.Fatalf("schemas directory not found from %s", dir)
	return ""
}

func findExecutorMigrationsDir(t *testing.T) string {
	t.Helper()
	_, callerFile, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("cannot determine caller file")
	}
	dir := filepath.Dir(callerFile)
	candidate := filepath.Join(dir, "..", "..", "migrations")
	if info, err := os.Stat(candidate); err == nil && info.IsDir() {
		return candidate
	}
	t.Fatalf("migrations directory not found from %s", dir)
	return ""
}

func findProjectFile(t *testing.T, relPath string) string {
	t.Helper()
	_, callerFile, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("cannot determine caller file")
	}
	dir := filepath.Dir(callerFile)
	candidate := filepath.Join(dir, "..", "..", relPath)
	if _, err := os.Stat(candidate); err == nil {
		return candidate
	}
	t.Fatalf("project file not found: %s", relPath)
	return ""
}
