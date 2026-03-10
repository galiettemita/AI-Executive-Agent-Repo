package temporal

import (
	"testing"

	"go.temporal.io/sdk/testsuite"
)

func registerP8Activities(env *testsuite.TestWorkflowEnvironment, activities *Activities) {
	// Federation.
	env.RegisterActivity(activities.CheckFederationPolicyActivity)
	env.RegisterActivity(activities.ExecuteFederationNegotiateActivity)
	env.RegisterActivity(activities.ExecuteFederationSyncActivity)
	env.RegisterActivity(activities.CompensateFederationActivity)
	// Edge.
	env.RegisterActivity(activities.FetchEdgeTasksActivity)
	env.RegisterActivity(activities.DetectEdgeConflictsActivity)
	env.RegisterActivity(activities.ResolveEdgeConflictsActivity)
	env.RegisterActivity(activities.ExecuteEdgeTasksActivity)
	// Browser.
	env.RegisterActivity(activities.ValidateBrowserReceiptActivity)
	env.RegisterActivity(activities.StartBrowserSessionActivity)
	env.RegisterActivity(activities.ExecuteBrowserTaskActivity)
	env.RegisterActivity(activities.CloseBrowserSessionActivity)
	// Fast-path.
	env.RegisterActivity(activities.FastPathMatchActivity)
	env.RegisterActivity(activities.RecordFastPathMetricActivity)
	// Experiments.
	env.RegisterActivity(activities.CheckExistingAssignmentActivity)
	env.RegisterActivity(activities.DeterministicAssignActivity)
	env.RegisterActivity(activities.PersistAssignmentActivity)
	// Onboarding.
	env.RegisterActivity(activities.InitOnboardingSessionActivity)
	env.RegisterActivity(activities.ExecuteProvisioningStageActivity)
	env.RegisterActivity(activities.FinalizeOnboardingActivity)
	// Billing.
	env.RegisterActivity(activities.IngestBillingWebhookActivity)
	env.RegisterActivity(activities.UpdateBillingLedgerActivity)
	env.RegisterActivity(activities.EnforceBillingPolicyActivity)
	// Load shedding.
	env.RegisterActivity(activities.EvaluateLoadSheddingTierActivity)
	env.RegisterActivity(activities.PropagateLoadSheddingTierActivity)
}

// ===================== FEDERATION TESTS =====================

func TestFederationNegotiationWorkflow_HappyPath(t *testing.T) {
	testSuite := &testsuite.WorkflowTestSuite{}
	env := testSuite.NewTestWorkflowEnvironment()

	activities := NewActivities()
	registerP8Activities(env, activities)

	input := FederationNegotiationInput{
		WorkspaceID:         "ws-001",
		PeerWorkspaceID:     "ws-002",
		ProposedPermissions: []string{"calendar_query", "status_query"},
	}

	env.ExecuteWorkflow(FederationNegotiationWorkflow, input)

	if !env.IsWorkflowCompleted() {
		t.Fatal("workflow did not complete")
	}
	if err := env.GetWorkflowError(); err != nil {
		t.Fatalf("workflow error: %v", err)
	}

	var result FederationNegotiationResult
	if err := env.GetWorkflowResult(&result); err != nil {
		t.Fatalf("failed to get result: %v", err)
	}
	if result.Status != "accepted" {
		t.Fatalf("expected accepted, got %s", result.Status)
	}
	if result.EvidenceHash == "" {
		t.Fatal("missing evidence hash")
	}
	if len(result.AcceptedPermissions) != 2 {
		t.Fatalf("expected 2 accepted permissions, got %d", len(result.AcceptedPermissions))
	}
}

func TestFederationNegotiationWorkflow_PolicyDenied(t *testing.T) {
	testSuite := &testsuite.WorkflowTestSuite{}
	env := testSuite.NewTestWorkflowEnvironment()

	activities := NewActivities()
	registerP8Activities(env, activities)

	// Request only invalid permissions — should be denied.
	input := FederationNegotiationInput{
		WorkspaceID:         "ws-001",
		PeerWorkspaceID:     "ws-002",
		ProposedPermissions: []string{"admin_override"},
	}

	env.ExecuteWorkflow(FederationNegotiationWorkflow, input)

	if !env.IsWorkflowCompleted() {
		t.Fatal("workflow did not complete")
	}

	var result FederationNegotiationResult
	if err := env.GetWorkflowResult(&result); err != nil {
		t.Fatalf("failed to get result: %v", err)
	}
	if result.Status != "DENIED" {
		t.Fatalf("expected DENIED for invalid permissions, got %s", result.Status)
	}
}

// ===================== EDGE SYNC TESTS =====================

func TestEdgeOfflineSyncWorkflow_HappyPath(t *testing.T) {
	testSuite := &testsuite.WorkflowTestSuite{}
	env := testSuite.NewTestWorkflowEnvironment()

	activities := NewActivities()
	registerP8Activities(env, activities)

	input := EdgeSyncInput{
		WorkspaceID: "ws-001",
		AgentID:     "agent-001",
		BatchSize:   50,
	}

	env.ExecuteWorkflow(EdgeOfflineSyncWorkflow, input)

	if !env.IsWorkflowCompleted() {
		t.Fatal("workflow did not complete")
	}

	var result EdgeSyncResult
	if err := env.GetWorkflowResult(&result); err != nil {
		t.Fatalf("failed to get result: %v", err)
	}
	// Degraded mode with no pool returns NO_TASKS.
	if result.Status != "NO_TASKS" {
		t.Fatalf("expected NO_TASKS in degraded mode, got %s", result.Status)
	}
}

// ===================== BROWSER AUTOMATION TESTS =====================

func TestBrowserAutomationWorkflow_HappyPath(t *testing.T) {
	testSuite := &testsuite.WorkflowTestSuite{}
	env := testSuite.NewTestWorkflowEnvironment()

	activities := NewActivities()
	registerP8Activities(env, activities)

	input := BrowserAutomationInput{
		WorkspaceID: "ws-001",
		SessionType: "scrape",
		URL:         "https://example.com",
		Parameters:  `{"selector":".content"}`,
		ReceiptID:   "receipt-001",
	}

	env.ExecuteWorkflow(BrowserAutomationWorkflow, input)

	if !env.IsWorkflowCompleted() {
		t.Fatal("workflow did not complete")
	}

	var result BrowserAutomationResult
	if err := env.GetWorkflowResult(&result); err != nil {
		t.Fatalf("failed to get result: %v", err)
	}
	if result.Status != "COMPLETED" {
		t.Fatalf("expected COMPLETED, got %s", result.Status)
	}
	if result.SessionID == "" {
		t.Fatal("missing session ID")
	}
	if result.EvidenceHash == "" {
		t.Fatal("missing evidence hash")
	}
}

func TestBrowserAutomationWorkflow_MissingReceipt(t *testing.T) {
	testSuite := &testsuite.WorkflowTestSuite{}
	env := testSuite.NewTestWorkflowEnvironment()

	activities := NewActivities()
	registerP8Activities(env, activities)

	input := BrowserAutomationInput{
		WorkspaceID: "ws-001",
		SessionType: "scrape",
		URL:         "https://example.com",
		ReceiptID:   "", // Missing receipt.
	}

	env.ExecuteWorkflow(BrowserAutomationWorkflow, input)

	if !env.IsWorkflowCompleted() {
		t.Fatal("workflow did not complete")
	}

	var result BrowserAutomationResult
	if err := env.GetWorkflowResult(&result); err != nil {
		t.Fatalf("failed to get result: %v", err)
	}
	if result.Status != "DENIED" {
		t.Fatalf("expected DENIED for missing receipt, got %s", result.Status)
	}
}

// ===================== FAST-PATH TESTS =====================

func TestFastPathPipelineWorkflow_Miss(t *testing.T) {
	testSuite := &testsuite.WorkflowTestSuite{}
	env := testSuite.NewTestWorkflowEnvironment()

	activities := NewActivities()
	registerP8Activities(env, activities)

	input := FastPathInput{
		WorkspaceID:     "ws-001",
		MessageID:       "msg-001",
		Payload:         "what time is it?",
		LatencyBudgetMs: 100,
	}

	env.ExecuteWorkflow(FastPathPipelineWorkflow, input)

	if !env.IsWorkflowCompleted() {
		t.Fatal("workflow did not complete")
	}

	var result FastPathResult
	if err := env.GetWorkflowResult(&result); err != nil {
		t.Fatalf("failed to get result: %v", err)
	}
	if result.Matched {
		t.Fatal("expected no match in degraded mode")
	}
	if result.EvidenceHash == "" {
		t.Fatal("missing evidence hash")
	}
}

// ===================== EXPERIMENT TESTS =====================

func TestExperimentAssignmentWorkflow_HappyPath(t *testing.T) {
	testSuite := &testsuite.WorkflowTestSuite{}
	env := testSuite.NewTestWorkflowEnvironment()

	activities := NewActivities()
	registerP8Activities(env, activities)

	input := ExperimentAssignInput{
		WorkspaceID:  "ws-001",
		ExperimentID: "exp-001",
		SubjectID:    "user-001",
	}

	env.ExecuteWorkflow(ExperimentAssignmentWorkflow, input)

	if !env.IsWorkflowCompleted() {
		t.Fatal("workflow did not complete")
	}

	var result ExperimentAssignResult
	if err := env.GetWorkflowResult(&result); err != nil {
		t.Fatalf("failed to get result: %v", err)
	}
	if result.VariantID == "" {
		t.Fatal("missing variant ID")
	}
	if result.EvidenceHash == "" {
		t.Fatal("missing evidence hash")
	}
}

func TestExperimentAssignment_Deterministic(t *testing.T) {
	// Same input must produce same variant.
	activities := NewActivities()
	input := ExperimentDeterministicInput{
		WorkspaceID:  "ws-001",
		ExperimentID: "exp-001",
		SubjectID:    "user-001",
	}

	r1, err := activities.DeterministicAssignActivity(nil, input)
	if err != nil {
		t.Fatal(err)
	}
	r2, err := activities.DeterministicAssignActivity(nil, input)
	if err != nil {
		t.Fatal(err)
	}

	if r1.VariantID != r2.VariantID {
		t.Fatalf("determinism violated: %s != %s", r1.VariantID, r2.VariantID)
	}
	if r1.EvidenceHash != r2.EvidenceHash {
		t.Fatalf("evidence hash mismatch: %s != %s", r1.EvidenceHash, r2.EvidenceHash)
	}
}

// ===================== ONBOARDING TESTS =====================

func TestOnboardingProvisioningWorkflow_Complete(t *testing.T) {
	testSuite := &testsuite.WorkflowTestSuite{}
	env := testSuite.NewTestWorkflowEnvironment()

	activities := NewActivities()
	registerP8Activities(env, activities)

	input := OnboardingProvisionInput{
		WorkspaceID: "ws-001",
		PlanID:      "plan_pro",
		OperatorID:  "op-001",
	}

	env.ExecuteWorkflow(OnboardingProvisioningWorkflow, input)

	if !env.IsWorkflowCompleted() {
		t.Fatal("workflow did not complete")
	}

	var result OnboardingProvisionResult
	if err := env.GetWorkflowResult(&result); err != nil {
		t.Fatalf("failed to get result: %v", err)
	}
	if result.Status != "completed" {
		t.Fatalf("expected completed, got %s", result.Status)
	}
	if len(result.CompletedStages) != 4 {
		t.Fatalf("expected 4 completed stages, got %d", len(result.CompletedStages))
	}
	if !result.FirstValueVerified {
		t.Fatal("expected first value to be verified")
	}
	if result.EvidenceHash == "" {
		t.Fatal("missing evidence hash")
	}
}

// ===================== BILLING TESTS =====================

func TestBillingEnforcementWorkflow_InvoicePaid(t *testing.T) {
	testSuite := &testsuite.WorkflowTestSuite{}
	env := testSuite.NewTestWorkflowEnvironment()

	activities := NewActivities()
	registerP8Activities(env, activities)

	input := BillingWebhookInput{
		WorkspaceID: "ws-001",
		Provider:    "stripe",
		EventType:   "invoice.paid",
		EventID:     "evt-001",
		Payload:     `{"amount":2900}`,
	}

	env.ExecuteWorkflow(BillingEnforcementWorkflow, input)

	if !env.IsWorkflowCompleted() {
		t.Fatal("workflow did not complete")
	}

	var result BillingWebhookResult
	if err := env.GetWorkflowResult(&result); err != nil {
		t.Fatalf("failed to get result: %v", err)
	}
	if result.Status != "PROCESSED" {
		t.Fatalf("expected PROCESSED, got %s", result.Status)
	}
	if result.PolicyGated {
		t.Fatal("invoice.paid should not trigger policy gating")
	}
	if result.EvidenceHash == "" {
		t.Fatal("missing evidence hash")
	}
}

func TestBillingEnforcementWorkflow_SubscriptionDeleted(t *testing.T) {
	testSuite := &testsuite.WorkflowTestSuite{}
	env := testSuite.NewTestWorkflowEnvironment()

	activities := NewActivities()
	registerP8Activities(env, activities)

	input := BillingWebhookInput{
		WorkspaceID: "ws-001",
		Provider:    "stripe",
		EventType:   "customer.subscription.deleted",
		EventID:     "evt-002",
		Payload:     `{"subscription_id":"sub-001"}`,
	}

	env.ExecuteWorkflow(BillingEnforcementWorkflow, input)

	if !env.IsWorkflowCompleted() {
		t.Fatal("workflow did not complete")
	}

	var result BillingWebhookResult
	if err := env.GetWorkflowResult(&result); err != nil {
		t.Fatalf("failed to get result: %v", err)
	}
	if result.Status != "PROCESSED" {
		t.Fatalf("expected PROCESSED, got %s", result.Status)
	}
	if !result.PolicyGated {
		t.Fatal("subscription deletion should trigger policy gating")
	}
}

// ===================== LOAD SHEDDING TESTS =====================

func TestLoadSheddingTierWorkflow_Nominal(t *testing.T) {
	testSuite := &testsuite.WorkflowTestSuite{}
	env := testSuite.NewTestWorkflowEnvironment()

	activities := NewActivities()
	registerP8Activities(env, activities)

	input := LoadSheddingInput{
		WorkspaceID: "ws-001",
		CPUPercent:  50,
		ErrorRate:   0.5,
		DBPoolUsage: 30,
	}

	env.ExecuteWorkflow(LoadSheddingTierWorkflow, input)

	if !env.IsWorkflowCompleted() {
		t.Fatal("workflow did not complete")
	}

	var result LoadSheddingResult
	if err := env.GetWorkflowResult(&result); err != nil {
		t.Fatalf("failed to get result: %v", err)
	}
	if result.CurrentTier != "D0" {
		t.Fatalf("expected D0 for nominal metrics, got %s", result.CurrentTier)
	}
}

func TestLoadSheddingTierWorkflow_Critical(t *testing.T) {
	testSuite := &testsuite.WorkflowTestSuite{}
	env := testSuite.NewTestWorkflowEnvironment()

	activities := NewActivities()
	registerP8Activities(env, activities)

	input := LoadSheddingInput{
		WorkspaceID: "ws-001",
		CPUPercent:  96,
		ErrorRate:   12,
		DBPoolUsage: 95,
	}

	env.ExecuteWorkflow(LoadSheddingTierWorkflow, input)

	if !env.IsWorkflowCompleted() {
		t.Fatal("workflow did not complete")
	}

	var result LoadSheddingResult
	if err := env.GetWorkflowResult(&result); err != nil {
		t.Fatalf("failed to get result: %v", err)
	}
	if result.CurrentTier != "D4" {
		t.Fatalf("expected D4 for critical metrics, got %s", result.CurrentTier)
	}
	if result.Reason == "" {
		t.Fatal("missing tier reason")
	}
}

// ===================== ACTIVITY UNIT TESTS =====================

func TestCheckFederationPolicyActivity_ValidPermissions(t *testing.T) {
	activities := NewActivities()
	result, err := activities.CheckFederationPolicyActivity(nil, FederationPolicyCheckInput{
		WorkspaceID:          "ws-001",
		PeerWorkspaceID:      "ws-002",
		RequestedPermissions: []string{"calendar_query", "task_delegate", "invalid_perm"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if !result.Allowed {
		t.Fatal("expected allowed with valid permissions")
	}
	if len(result.AllowedPermissions) != 2 {
		t.Fatalf("expected 2 allowed, got %d", len(result.AllowedPermissions))
	}
	if len(result.DeniedPermissions) != 1 {
		t.Fatalf("expected 1 denied, got %d", len(result.DeniedPermissions))
	}
}

func TestValidateBrowserReceiptActivity_MissingReceipt(t *testing.T) {
	activities := NewActivities()
	result, err := activities.ValidateBrowserReceiptActivity(nil, BrowserReceiptCheckInput{
		WorkspaceID: "ws-001",
		ReceiptID:   "",
		SessionType: "scrape",
	})
	if err != nil {
		t.Fatal(err)
	}
	if result.Valid {
		t.Fatal("expected invalid for missing receipt")
	}
	if result.Reason != "missing_receipt" {
		t.Fatalf("expected missing_receipt reason, got %s", result.Reason)
	}
}

func TestEvaluateLoadSheddingTierActivity_Thresholds(t *testing.T) {
	activities := NewActivities()

	tests := []struct {
		cpu, err, db float64
		expected     string
	}{
		{50, 1, 30, "D0"},
		{81, 1, 30, "D1"},
		{86, 1, 30, "D2"},
		{91, 1, 30, "D3"},
		{96, 1, 30, "D4"},
	}

	for _, tt := range tests {
		result, err := activities.EvaluateLoadSheddingTierActivity(nil, LoadSheddingEvalInput{
			WorkspaceID: "ws-001",
			CPUPercent:  tt.cpu,
			ErrorRate:   tt.err,
			DBPoolUsage: tt.db,
		})
		if err != nil {
			t.Fatal(err)
		}
		if result.NewTier != tt.expected {
			t.Errorf("cpu=%.0f: expected %s, got %s", tt.cpu, tt.expected, result.NewTier)
		}
	}
}

func TestIngestBillingWebhookActivity_MissingEventID(t *testing.T) {
	activities := NewActivities()
	_, err := activities.IngestBillingWebhookActivity(nil, BillingIngestInput{
		WorkspaceID: "ws-001",
		Provider:    "stripe",
		EventType:   "invoice.paid",
		EventID:     "",
	})
	if err == nil {
		t.Fatal("expected error for missing event ID")
	}
}
