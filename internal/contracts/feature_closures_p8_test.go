package contracts

import (
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"strings"
	"testing"

	"github.com/brevio/brevio/internal/temporal"
)

// --- P8: Feature Closures ---

func TestP8WorkflowsExist(t *testing.T) {
	// All P8 workflows must be callable functions.
	workflows := []struct {
		name string
		fn   interface{}
	}{
		{"FederationNegotiationWorkflow", temporal.FederationNegotiationWorkflow},
		{"EdgeOfflineSyncWorkflow", temporal.EdgeOfflineSyncWorkflow},
		{"BrowserAutomationWorkflow", temporal.BrowserAutomationWorkflow},
		{"FastPathPipelineWorkflow", temporal.FastPathPipelineWorkflow},
		{"ExperimentAssignmentWorkflow", temporal.ExperimentAssignmentWorkflow},
		{"OnboardingProvisioningWorkflow", temporal.OnboardingProvisioningWorkflow},
		{"BillingEnforcementWorkflow", temporal.BillingEnforcementWorkflow},
		{"LoadSheddingTierWorkflow", temporal.LoadSheddingTierWorkflow},
	}
	for _, w := range workflows {
		if w.fn == nil {
			t.Errorf("workflow %s is nil", w.name)
		}
	}
}

func TestP8ActivitiesAreMethodBased(t *testing.T) {
	typ := reflect.TypeOf(&temporal.Activities{})
	methods := []string{
		// Federation.
		"CheckFederationPolicyActivity",
		"ExecuteFederationNegotiateActivity",
		"CompensateFederationActivity",
		// Edge.
		"FetchEdgeTasksActivity",
		"DetectEdgeConflictsActivity",
		"ResolveEdgeConflictsActivity",
		"ExecuteEdgeTasksActivity",
		// Browser.
		"ValidateBrowserReceiptActivity",
		"StartBrowserSessionActivity",
		"ExecuteBrowserTaskActivity",
		"CloseBrowserSessionActivity",
		// Fast-path.
		"FastPathMatchActivity",
		"RecordFastPathMetricActivity",
		// Experiments.
		"CheckExistingAssignmentActivity",
		"DeterministicAssignActivity",
		"PersistAssignmentActivity",
		// Onboarding.
		"InitOnboardingSessionActivity",
		"ExecuteProvisioningStageActivity",
		"FinalizeOnboardingActivity",
		// Billing.
		"IngestBillingWebhookActivity",
		"UpdateBillingLedgerActivity",
		"EnforceBillingPolicyActivity",
		// Load shedding.
		"EvaluateLoadSheddingTierActivity",
		"PropagateLoadSheddingTierActivity",
	}
	for _, method := range methods {
		if _, ok := typ.MethodByName(method); !ok {
			t.Errorf("Activities missing method: %s", method)
		}
	}
}

func TestP8WorkerRegistersAllWorkflows(t *testing.T) {
	workerFile := findP8ProjectFile(t, "internal/temporal/worker.go")
	data, err := os.ReadFile(workerFile)
	if err != nil {
		t.Fatalf("read worker.go: %v", err)
	}
	content := string(data)

	workflows := []string{
		"FederationNegotiationWorkflow",
		"EdgeOfflineSyncWorkflow",
		"BrowserAutomationWorkflow",
		"FastPathPipelineWorkflow",
		"ExperimentAssignmentWorkflow",
		"OnboardingProvisioningWorkflow",
		"BillingEnforcementWorkflow",
		"LoadSheddingTierWorkflow",
	}
	for _, wf := range workflows {
		if !strings.Contains(content, wf) {
			t.Errorf("worker.go missing workflow registration: %s", wf)
		}
	}
}

func TestP8WorkerRegistersAllActivities(t *testing.T) {
	workerFile := findP8ProjectFile(t, "internal/temporal/worker.go")
	data, err := os.ReadFile(workerFile)
	if err != nil {
		t.Fatalf("read worker.go: %v", err)
	}
	content := string(data)

	activities := []string{
		// Federation.
		"CheckFederationPolicyActivity",
		"ExecuteFederationNegotiateActivity",
		"CompensateFederationActivity",
		// Edge.
		"FetchEdgeTasksActivity",
		"DetectEdgeConflictsActivity",
		"ResolveEdgeConflictsActivity",
		"ExecuteEdgeTasksActivity",
		// Browser.
		"ValidateBrowserReceiptActivity",
		"StartBrowserSessionActivity",
		"ExecuteBrowserTaskActivity",
		"CloseBrowserSessionActivity",
		// Fast-path.
		"FastPathMatchActivity",
		"RecordFastPathMetricActivity",
		// Experiments.
		"CheckExistingAssignmentActivity",
		"DeterministicAssignActivity",
		"PersistAssignmentActivity",
		// Onboarding.
		"InitOnboardingSessionActivity",
		"ExecuteProvisioningStageActivity",
		"FinalizeOnboardingActivity",
		// Billing.
		"IngestBillingWebhookActivity",
		"UpdateBillingLedgerActivity",
		"EnforceBillingPolicyActivity",
		// Load shedding.
		"EvaluateLoadSheddingTierActivity",
		"PropagateLoadSheddingTierActivity",
	}
	for _, activity := range activities {
		if !strings.Contains(content, activity) {
			t.Errorf("worker.go missing activity registration: %s", activity)
		}
	}
}

func TestP8FederationWorkflowHasPolicyGate(t *testing.T) {
	wfFile := findP8ProjectFile(t, "internal/temporal/workflows_p8.go")
	data, err := os.ReadFile(wfFile)
	if err != nil {
		t.Fatalf("read workflows_p8.go: %v", err)
	}
	content := string(data)

	required := []string{
		"CheckFederationPolicyActivity",
		"ExecuteFederationNegotiateActivity",
		"CompensateFederationActivity",
	}
	for _, needle := range required {
		if !strings.Contains(content, needle) {
			t.Errorf("FederationNegotiationWorkflow missing: %s", needle)
		}
	}
}

func TestP8EdgeWorkflowHasConflictHandling(t *testing.T) {
	wfFile := findP8ProjectFile(t, "internal/temporal/workflows_p8.go")
	data, err := os.ReadFile(wfFile)
	if err != nil {
		t.Fatalf("read workflows_p8.go: %v", err)
	}
	content := string(data)

	required := []string{
		"FetchEdgeTasksActivity",
		"DetectEdgeConflictsActivity",
		"ResolveEdgeConflictsActivity",
		"ExecuteEdgeTasksActivity",
	}
	for _, needle := range required {
		if !strings.Contains(content, needle) {
			t.Errorf("EdgeOfflineSyncWorkflow missing: %s", needle)
		}
	}
}

func TestP8BrowserWorkflowHasReceiptEnforcement(t *testing.T) {
	wfFile := findP8ProjectFile(t, "internal/temporal/workflows_p8.go")
	data, err := os.ReadFile(wfFile)
	if err != nil {
		t.Fatalf("read workflows_p8.go: %v", err)
	}
	content := string(data)

	required := []string{
		"ValidateBrowserReceiptActivity",
		"StartBrowserSessionActivity",
		"ExecuteBrowserTaskActivity",
		"CloseBrowserSessionActivity",
	}
	for _, needle := range required {
		if !strings.Contains(content, needle) {
			t.Errorf("BrowserAutomationWorkflow missing: %s", needle)
		}
	}
}

func TestP8FastPathWorkflowHasLatencyBudget(t *testing.T) {
	wfFile := findP8ProjectFile(t, "internal/temporal/workflows_p8.go")
	data, err := os.ReadFile(wfFile)
	if err != nil {
		t.Fatalf("read workflows_p8.go: %v", err)
	}
	content := string(data)

	if !strings.Contains(content, "LatencyBudgetMs") {
		t.Error("FastPathPipelineWorkflow missing LatencyBudgetMs")
	}
	if !strings.Contains(content, "MaximumAttempts: 1") {
		t.Error("fast-path should have MaximumAttempts: 1 (no retries)")
	}
}

func TestP8ExperimentWorkflowHasDeterministicAssign(t *testing.T) {
	wfFile := findP8ProjectFile(t, "internal/temporal/workflows_p8.go")
	data, err := os.ReadFile(wfFile)
	if err != nil {
		t.Fatalf("read workflows_p8.go: %v", err)
	}
	content := string(data)

	if !strings.Contains(content, "DeterministicAssignActivity") {
		t.Error("ExperimentAssignmentWorkflow missing DeterministicAssignActivity")
	}
	if !strings.Contains(content, "CheckExistingAssignmentActivity") {
		t.Error("ExperimentAssignmentWorkflow missing idempotent existing check")
	}
}

func TestP8OnboardingWorkflowHasFirstValueVerification(t *testing.T) {
	wfFile := findP8ProjectFile(t, "internal/temporal/workflows_p8.go")
	data, err := os.ReadFile(wfFile)
	if err != nil {
		t.Fatalf("read workflows_p8.go: %v", err)
	}
	content := string(data)

	if !strings.Contains(content, "first_value") {
		t.Error("OnboardingProvisioningWorkflow missing first_value stage")
	}
	if !strings.Contains(content, "FirstValueVerified") {
		t.Error("OnboardingProvisioningWorkflow missing FirstValueVerified")
	}
}

func TestP8BillingWorkflowHasWebhookIngestionAndPolicyGate(t *testing.T) {
	wfFile := findP8ProjectFile(t, "internal/temporal/workflows_p8.go")
	data, err := os.ReadFile(wfFile)
	if err != nil {
		t.Fatalf("read workflows_p8.go: %v", err)
	}
	content := string(data)

	required := []string{
		"IngestBillingWebhookActivity",
		"UpdateBillingLedgerActivity",
		"EnforceBillingPolicyActivity",
		"customer.subscription.deleted",
		"invoice.payment_failed",
	}
	for _, needle := range required {
		if !strings.Contains(content, needle) {
			t.Errorf("BillingEnforcementWorkflow missing: %s", needle)
		}
	}
}

func TestP8LoadSheddingTiers(t *testing.T) {
	actFile := findP8ProjectFile(t, "internal/temporal/activities_p8.go")
	data, err := os.ReadFile(actFile)
	if err != nil {
		t.Fatalf("read activities_p8.go: %v", err)
	}
	content := string(data)

	tiers := []string{"D0", "D1", "D2", "D3", "D4"}
	for _, tier := range tiers {
		if !strings.Contains(content, `"`+tier+`"`) {
			t.Errorf("activities_p8.go missing tier: %s", tier)
		}
	}
}

func TestP8MigrationExists(t *testing.T) {
	_, callerFile, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("cannot determine caller")
	}
	dir := filepath.Dir(callerFile)
	migDir := filepath.Join(dir, "..", "..", "migrations")

	upFile := filepath.Join(migDir, "053_feature_closures_p8.up.sql")
	downFile := filepath.Join(migDir, "053_feature_closures_p8.down.sql")

	if _, err := os.Stat(upFile); err != nil {
		t.Fatalf("migration up file missing: %s", upFile)
	}
	if _, err := os.Stat(downFile); err != nil {
		t.Fatalf("migration down file missing: %s", downFile)
	}

	data, err := os.ReadFile(upFile)
	if err != nil {
		t.Fatalf("read migration: %v", err)
	}
	content := string(data)

	tables := []string{
		"federation_sync_log",
		"edge_sync_tasks",
		"browser_sessions",
		"experiment_definitions",
		"experiment_assignments",
		"experiment_conversions",
		"fast_path_routes",
		"billing_webhook_events",
		"billing_ledger_entries",
		"onboarding_sessions",
		"load_shedding_state",
	}
	for _, table := range tables {
		if !strings.Contains(content, table) {
			t.Errorf("migration missing table: %s", table)
		}
	}

	// All tables must have RLS.
	rlsTables := []string{
		"federation_sync_log",
		"edge_sync_tasks",
		"browser_sessions",
		"experiment_definitions",
		"experiment_assignments",
		"experiment_conversions",
		"fast_path_routes",
		"billing_webhook_events",
		"billing_ledger_entries",
		"onboarding_sessions",
		"load_shedding_state",
	}
	for _, table := range rlsTables {
		if !strings.Contains(content, "ENABLE ROW LEVEL SECURITY") {
			t.Errorf("migration missing RLS for: %s", table)
			break // All-or-nothing check.
		}
		rlsPolicy := table + "_workspace_isolation"
		if !strings.Contains(content, rlsPolicy) {
			t.Errorf("migration missing RLS policy: %s", rlsPolicy)
		}
	}
}

func TestP8ActivityTypes(t *testing.T) {
	// All P8 activity input/output types must be defined.
	types := []struct {
		name string
		inst interface{}
	}{
		{"FederationPolicyCheckInput", temporal.FederationPolicyCheckInput{}},
		{"FederationPolicyCheckResult", temporal.FederationPolicyCheckResult{}},
		{"FederationNegotiateInput", temporal.FederationNegotiateInput{}},
		{"FederationNegotiateResult", temporal.FederationNegotiateResult{}},
		{"FederationCompensateInput", temporal.FederationCompensateInput{}},
		{"FederationCompensateResult", temporal.FederationCompensateResult{}},
		{"EdgeFetchTasksInput", temporal.EdgeFetchTasksInput{}},
		{"EdgeFetchTasksResult", temporal.EdgeFetchTasksResult{}},
		{"EdgeTask", temporal.EdgeTask{}},
		{"EdgeConflict", temporal.EdgeConflict{}},
		{"BrowserReceiptCheckInput", temporal.BrowserReceiptCheckInput{}},
		{"BrowserSessionInput", temporal.BrowserSessionInput{}},
		{"BrowserTaskInput", temporal.BrowserTaskInput{}},
		{"FastPathMatchInput", temporal.FastPathMatchInput{}},
		{"FastPathMatchResult", temporal.FastPathMatchResult{}},
		{"ExperimentDeterministicInput", temporal.ExperimentDeterministicInput{}},
		{"ExperimentDeterministicResult", temporal.ExperimentDeterministicResult{}},
		{"OnboardingInitInput", temporal.OnboardingInitInput{}},
		{"OnboardingStageExecInput", temporal.OnboardingStageExecInput{}},
		{"BillingIngestInput", temporal.BillingIngestInput{}},
		{"BillingLedgerInput", temporal.BillingLedgerInput{}},
		{"BillingPolicyInput", temporal.BillingPolicyInput{}},
		{"LoadSheddingEvalInput", temporal.LoadSheddingEvalInput{}},
		{"LoadSheddingEvalResult", temporal.LoadSheddingEvalResult{}},
	}
	for _, tt := range types {
		if reflect.TypeOf(tt.inst).Kind() != reflect.Struct {
			t.Errorf("type %s is not a struct", tt.name)
		}
	}
}

func TestP8FederationCompensationInWorkflow(t *testing.T) {
	wfFile := findP8ProjectFile(t, "internal/temporal/workflows_p8.go")
	data, err := os.ReadFile(wfFile)
	if err != nil {
		t.Fatalf("read workflows_p8.go: %v", err)
	}
	content := string(data)

	if !strings.Contains(content, "CompensateFederationActivity") {
		t.Error("FederationNegotiationWorkflow missing compensation step")
	}
	if !strings.Contains(content, "sync_failure") {
		t.Error("FederationNegotiationWorkflow missing sync_failure compensation reason")
	}
}

func TestP8EdgeIdempotencyInActivities(t *testing.T) {
	actFile := findP8ProjectFile(t, "internal/temporal/activities_p8.go")
	data, err := os.ReadFile(actFile)
	if err != nil {
		t.Fatalf("read activities_p8.go: %v", err)
	}
	content := string(data)

	if !strings.Contains(content, "idempotency_key") {
		t.Error("edge activities missing idempotency_key handling")
	}
	if !strings.Contains(content, "idempotency_duplicate") {
		t.Error("edge conflict detection missing idempotency_duplicate type")
	}
}

// --- helpers ---

func findP8ProjectFile(t *testing.T, relPath string) string {
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
