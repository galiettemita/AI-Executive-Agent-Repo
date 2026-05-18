package temporal

import (
	"fmt"
	"time"

	"go.temporal.io/sdk/temporal"
	"go.temporal.io/sdk/workflow"
)

// ===================== FEDERATION NEGOTIATION & SYNC =====================

// FederationNegotiationInput drives federation negotiation workflow.
type FederationNegotiationInput struct {
	WorkspaceID         string   `json:"workspace_id"`
	PeerWorkspaceID     string   `json:"peer_workspace_id"`
	ProposedPermissions []string `json:"proposed_permissions"`
}

// FederationNegotiationResult captures the negotiation outcome.
type FederationNegotiationResult struct {
	NegotiationID       string   `json:"negotiation_id"`
	Status              string   `json:"status"` // accepted, rejected, expired
	AcceptedPermissions []string `json:"accepted_permissions"`
	CompensationNeeded  bool     `json:"compensation_needed"`
	EvidenceHash        string   `json:"evidence_hash"`
}

// FederationNegotiationWorkflow orchestrates federation negotiation with
// policy enforcement and compensation handling.
func FederationNegotiationWorkflow(ctx workflow.Context, input FederationNegotiationInput) (*FederationNegotiationResult, error) {
	logger := workflow.GetLogger(ctx)
	logger.Info("FederationNegotiationWorkflow started",
		"workspace", input.WorkspaceID,
		"peer", input.PeerWorkspaceID)

	ao := workflow.ActivityOptions{
		StartToCloseTimeout: 60 * time.Second,
		RetryPolicy: &temporal.RetryPolicy{
			InitialInterval:    2 * time.Second,
			BackoffCoefficient: 2.0,
			MaximumInterval:    60 * time.Second,
			MaximumAttempts:    3,
		},
	}
	ctx = workflow.WithActivityOptions(ctx, ao)

	var a *Activities

	// Step 1: Validate federation permissions via policy gate.
	var policyResult FederationPolicyCheckResult
	err := workflow.ExecuteActivity(ctx, a.CheckFederationPolicyActivity, FederationPolicyCheckInput{
		WorkspaceID:         input.WorkspaceID,
		PeerWorkspaceID:     input.PeerWorkspaceID,
		RequestedPermissions: input.ProposedPermissions,
	}).Get(ctx, &policyResult)
	if err != nil {
		return &FederationNegotiationResult{Status: "FAILED", EvidenceHash: "policy_error"}, nil
	}
	if !policyResult.Allowed {
		return &FederationNegotiationResult{
			Status:       "DENIED",
			EvidenceHash: policyResult.EvidenceHash,
		}, nil
	}

	// Step 2: Execute negotiation with peer.
	var negotiationResult FederationNegotiateResult
	err = workflow.ExecuteActivity(ctx, a.ExecuteFederationNegotiateActivity, FederationNegotiateInput{
		WorkspaceID:         input.WorkspaceID,
		PeerWorkspaceID:     input.PeerWorkspaceID,
		AllowedPermissions:  policyResult.AllowedPermissions,
	}).Get(ctx, &negotiationResult)
	if err != nil {
		return &FederationNegotiationResult{Status: "FAILED"}, nil
	}

	// Step 3: If accepted, execute sync.
	if negotiationResult.Status == "accepted" {
		var syncResult FederationSyncResult
		err = workflow.ExecuteActivity(ctx, a.ExecuteFederationSyncActivity, FederationSyncInput{
			SourceWorkspaceID: input.WorkspaceID,
			TargetWorkspaceID: input.PeerWorkspaceID,
			SyncType:          "full",
		}).Get(ctx, &syncResult)
		if err != nil || syncResult.Status == "FAILED" {
			// Step 4: Compensate on sync failure.
			var compResult FederationCompensateResult
			_ = workflow.ExecuteActivity(ctx, a.CompensateFederationActivity, FederationCompensateInput{
				WorkspaceID:     input.WorkspaceID,
				PeerWorkspaceID: input.PeerWorkspaceID,
				NegotiationID:   negotiationResult.NegotiationID,
				Reason:          "sync_failure",
			}).Get(ctx, &compResult)
			return &FederationNegotiationResult{
				NegotiationID:      negotiationResult.NegotiationID,
				Status:             "compensated",
				CompensationNeeded: true,
				EvidenceHash:       negotiationResult.EvidenceHash,
			}, nil
		}
	}

	return &FederationNegotiationResult{
		NegotiationID:       negotiationResult.NegotiationID,
		Status:              negotiationResult.Status,
		AcceptedPermissions: negotiationResult.AcceptedPermissions,
		EvidenceHash:        negotiationResult.EvidenceHash,
	}, nil
}

// ===================== EDGE OFFLINE TASK SYNC =====================

// EdgeSyncInput drives the edge offline task sync workflow.
type EdgeSyncInput struct {
	WorkspaceID string `json:"workspace_id"`
	AgentID     string `json:"agent_id"`
	BatchSize   int    `json:"batch_size"`
}

// EdgeSyncResult captures the sync outcome.
type EdgeSyncResult struct {
	TasksSynced      int    `json:"tasks_synced"`
	TasksFailed      int    `json:"tasks_failed"`
	ConflictsFound   int    `json:"conflicts_found"`
	ConflictsResolved int   `json:"conflicts_resolved"`
	Status           string `json:"status"`
	EvidenceHash     string `json:"evidence_hash"`
}

// EdgeOfflineSyncWorkflow syncs queued offline tasks from an edge agent
// with conflict detection and idempotent execution.
func EdgeOfflineSyncWorkflow(ctx workflow.Context, input EdgeSyncInput) (*EdgeSyncResult, error) {
	logger := workflow.GetLogger(ctx)
	logger.Info("EdgeOfflineSyncWorkflow started",
		"workspace", input.WorkspaceID,
		"agent", input.AgentID)

	ao := workflow.ActivityOptions{
		StartToCloseTimeout: 120 * time.Second,
		RetryPolicy: &temporal.RetryPolicy{
			InitialInterval:    2 * time.Second,
			BackoffCoefficient: 2.0,
			MaximumInterval:    60 * time.Second,
			MaximumAttempts:    3,
		},
	}
	ctx = workflow.WithActivityOptions(ctx, ao)

	var a *Activities

	// Step 1: Fetch pending edge tasks.
	var fetchResult EdgeFetchTasksResult
	err := workflow.ExecuteActivity(ctx, a.FetchEdgeTasksActivity, EdgeFetchTasksInput{
		WorkspaceID: input.WorkspaceID,
		AgentID:     input.AgentID,
		BatchSize:   input.BatchSize,
	}).Get(ctx, &fetchResult)
	if err != nil {
		return &EdgeSyncResult{Status: "FETCH_FAILED"}, nil
	}
	if len(fetchResult.Tasks) == 0 {
		return &EdgeSyncResult{Status: "NO_TASKS"}, nil
	}

	// Step 2: Detect conflicts.
	var conflictResult EdgeConflictDetectResult
	err = workflow.ExecuteActivity(ctx, a.DetectEdgeConflictsActivity, EdgeConflictDetectInput{
		WorkspaceID: input.WorkspaceID,
		Tasks:       fetchResult.Tasks,
	}).Get(ctx, &conflictResult)
	if err != nil {
		return &EdgeSyncResult{Status: "CONFLICT_DETECT_FAILED"}, nil
	}

	// Step 3: Resolve conflicts.
	resolvedCount := 0
	if conflictResult.ConflictsFound > 0 {
		var resolveResult EdgeConflictResolveResult
		err = workflow.ExecuteActivity(ctx, a.ResolveEdgeConflictsActivity, EdgeConflictResolveInput{
			WorkspaceID: input.WorkspaceID,
			Conflicts:   conflictResult.Conflicts,
		}).Get(ctx, &resolveResult)
		if err != nil {
			return &EdgeSyncResult{Status: "CONFLICT_RESOLVE_FAILED", ConflictsFound: conflictResult.ConflictsFound}, nil
		}
		resolvedCount = resolveResult.Resolved
	}

	// Step 4: Execute synced tasks with idempotency.
	var execResult EdgeExecuteTasksResult
	err = workflow.ExecuteActivity(ctx, a.ExecuteEdgeTasksActivity, EdgeExecuteTasksInput{
		WorkspaceID:  input.WorkspaceID,
		Tasks:        fetchResult.Tasks,
		ResolvedKeys: conflictResult.ResolvedKeys,
	}).Get(ctx, &execResult)
	if err != nil {
		return &EdgeSyncResult{Status: "EXECUTE_FAILED"}, nil
	}

	return &EdgeSyncResult{
		TasksSynced:       execResult.Executed,
		TasksFailed:       execResult.Failed,
		ConflictsFound:    conflictResult.ConflictsFound,
		ConflictsResolved: resolvedCount,
		Status:            "COMPLETED",
		EvidenceHash:      execResult.EvidenceHash,
	}, nil
}

// ===================== BROWSER AUTOMATION =====================

// BrowserAutomationInput drives browser automation as a Temporal workflow.
type BrowserAutomationInput struct {
	WorkspaceID string `json:"workspace_id"`
	SessionType string `json:"session_type"` // scrape, form_fill, booking, price_watch, screenshot
	URL         string `json:"url"`
	Parameters  string `json:"parameters"` // JSON-encoded session-specific params
	ReceiptID   string `json:"receipt_id"`
}

// BrowserAutomationResult captures the automation outcome.
type BrowserAutomationResult struct {
	SessionID    string `json:"session_id"`
	Status       string `json:"status"`
	Result       string `json:"result"` // JSON-encoded result
	EvidenceHash string `json:"evidence_hash"`
}

// BrowserAutomationWorkflow orchestrates browser automation through Temporal
// with policy gate verification and receipt enforcement.
func BrowserAutomationWorkflow(ctx workflow.Context, input BrowserAutomationInput) (*BrowserAutomationResult, error) {
	logger := workflow.GetLogger(ctx)
	logger.Info("BrowserAutomationWorkflow started",
		"workspace", input.WorkspaceID,
		"type", input.SessionType)

	ao := workflow.ActivityOptions{
		StartToCloseTimeout: 300 * time.Second,
		RetryPolicy: &temporal.RetryPolicy{
			InitialInterval:    5 * time.Second,
			BackoffCoefficient: 2.0,
			MaximumInterval:    120 * time.Second,
			MaximumAttempts:    2,
		},
	}
	ctx = workflow.WithActivityOptions(ctx, ao)

	var a *Activities

	// Step 1: Validate receipt for browser automation.
	var receiptResult BrowserReceiptCheckResult
	err := workflow.ExecuteActivity(ctx, a.ValidateBrowserReceiptActivity, BrowserReceiptCheckInput{
		WorkspaceID: input.WorkspaceID,
		ReceiptID:   input.ReceiptID,
		SessionType: input.SessionType,
	}).Get(ctx, &receiptResult)
	if err != nil || !receiptResult.Valid {
		reason := "receipt_validation_error"
		if err == nil {
			reason = receiptResult.Reason
		}
		return &BrowserAutomationResult{Status: "DENIED", EvidenceHash: reason}, nil
	}

	// Step 2: Start browser session.
	var sessionResult BrowserSessionResult
	err = workflow.ExecuteActivity(ctx, a.StartBrowserSessionActivity, BrowserSessionInput{
		WorkspaceID: input.WorkspaceID,
		SessionType: input.SessionType,
		URL:         input.URL,
		Parameters:  input.Parameters,
	}).Get(ctx, &sessionResult)
	if err != nil {
		return &BrowserAutomationResult{Status: "SESSION_FAILED"}, nil
	}

	// Step 3: Execute browser task.
	var taskResult BrowserTaskResult
	err = workflow.ExecuteActivity(ctx, a.ExecuteBrowserTaskActivity, BrowserTaskInput{
		WorkspaceID: input.WorkspaceID,
		SessionID:   sessionResult.SessionID,
		SessionType: input.SessionType,
		URL:         input.URL,
		Parameters:  input.Parameters,
	}).Get(ctx, &taskResult)
	if err != nil {
		// Cleanup session on failure.
		_ = workflow.ExecuteActivity(ctx, a.CloseBrowserSessionActivity, BrowserCloseInput{
			SessionID: sessionResult.SessionID,
		}).Get(ctx, nil)
		return &BrowserAutomationResult{
			SessionID: sessionResult.SessionID,
			Status:    "TASK_FAILED",
		}, nil
	}

	// Step 4: Close browser session.
	_ = workflow.ExecuteActivity(ctx, a.CloseBrowserSessionActivity, BrowserCloseInput{
		SessionID: sessionResult.SessionID,
	}).Get(ctx, nil)

	return &BrowserAutomationResult{
		SessionID:    sessionResult.SessionID,
		Status:       "COMPLETED",
		Result:       taskResult.Result,
		EvidenceHash: taskResult.EvidenceHash,
	}, nil
}

// ===================== FAST-PATH DETERMINISTIC PIPELINE =====================

// FastPathInput drives the fast-path pipeline evaluation.
type FastPathInput struct {
	WorkspaceID   string `json:"workspace_id"`
	MessageID     string `json:"message_id"`
	Payload       string `json:"payload"`
	LatencyBudgetMs int  `json:"latency_budget_ms"`
}

// FastPathResult captures the fast-path outcome with latency metrics.
type FastPathResult struct {
	Matched        bool    `json:"matched"`
	Response       string  `json:"response,omitempty"`
	RouteID        string  `json:"route_id,omitempty"`
	LatencyMs      float64 `json:"latency_ms"`
	Confidence     float64 `json:"confidence"`
	BudgetExceeded bool    `json:"budget_exceeded"`
	EvidenceHash   string  `json:"evidence_hash"`
}

// FastPathPipelineWorkflow attempts fast-path matching before falling back
// to the full intelligence pipeline. Enforces latency budget.
func FastPathPipelineWorkflow(ctx workflow.Context, input FastPathInput) (*FastPathResult, error) {
	logger := workflow.GetLogger(ctx)
	logger.Info("FastPathPipelineWorkflow started",
		"workspace", input.WorkspaceID,
		"budget_ms", input.LatencyBudgetMs)

	shortAO := workflow.ActivityOptions{
		StartToCloseTimeout: time.Duration(input.LatencyBudgetMs) * time.Millisecond,
		RetryPolicy: &temporal.RetryPolicy{
			MaximumAttempts: 1, // No retries on fast-path — fail fast.
		},
	}
	ctx = workflow.WithActivityOptions(ctx, shortAO)

	var a *Activities

	// Step 1: Attempt fast-path match.
	var matchResult FastPathMatchResult
	err := workflow.ExecuteActivity(ctx, a.FastPathMatchActivity, FastPathMatchInput{
		WorkspaceID:     input.WorkspaceID,
		MessageID:       input.MessageID,
		Payload:         input.Payload,
		LatencyBudgetMs: input.LatencyBudgetMs,
	}).Get(ctx, &matchResult)
	if err != nil {
		return &FastPathResult{
			Matched:        false,
			BudgetExceeded: true,
			EvidenceHash:   "fast_path_timeout",
		}, nil
	}

	if matchResult.Matched {
		// Step 2: Record fast-path hit metric.
		_ = workflow.ExecuteActivity(ctx, a.RecordFastPathMetricActivity, FastPathMetricInput{
			WorkspaceID: input.WorkspaceID,
			RouteID:     matchResult.RouteID,
			LatencyMs:   matchResult.LatencyMs,
			Hit:         true,
		}).Get(ctx, nil)

		return &FastPathResult{
			Matched:      true,
			Response:     matchResult.Response,
			RouteID:      matchResult.RouteID,
			LatencyMs:    matchResult.LatencyMs,
			Confidence:   matchResult.Confidence,
			EvidenceHash: matchResult.EvidenceHash,
		}, nil
	}

	// Fast-path miss — caller should fall through to full pipeline.
	return &FastPathResult{
		Matched:      false,
		LatencyMs:    matchResult.LatencyMs,
		EvidenceHash: matchResult.EvidenceHash,
	}, nil
}

// ===================== EXPERIMENTS WORKFLOW =====================

// ExperimentAssignInput drives deterministic experiment assignment.
type ExperimentAssignInput struct {
	WorkspaceID  string `json:"workspace_id"`
	ExperimentID string `json:"experiment_id"`
	SubjectID    string `json:"subject_id"`
}

// ExperimentAssignResult captures the assignment outcome.
type ExperimentAssignResult struct {
	AssignmentID string `json:"assignment_id"`
	VariantID    string `json:"variant_id"`
	VariantName  string `json:"variant_name"`
	Persisted    bool   `json:"persisted"`
	EvidenceHash string `json:"evidence_hash"`
}

// ExperimentAssignmentWorkflow deterministically assigns a subject to an
// experiment variant and persists the assignment.
func ExperimentAssignmentWorkflow(ctx workflow.Context, input ExperimentAssignInput) (*ExperimentAssignResult, error) {
	logger := workflow.GetLogger(ctx)
	logger.Info("ExperimentAssignmentWorkflow started",
		"experiment", input.ExperimentID,
		"subject", input.SubjectID)

	ao := workflow.ActivityOptions{
		StartToCloseTimeout: 30 * time.Second,
		RetryPolicy: &temporal.RetryPolicy{
			InitialInterval:    time.Second,
			BackoffCoefficient: 2.0,
			MaximumAttempts:    3,
		},
	}
	ctx = workflow.WithActivityOptions(ctx, ao)

	var a *Activities

	// Step 1: Check for existing assignment (idempotent).
	var existing ExperimentExistingResult
	err := workflow.ExecuteActivity(ctx, a.CheckExistingAssignmentActivity, ExperimentExistingInput{
		WorkspaceID:  input.WorkspaceID,
		ExperimentID: input.ExperimentID,
		SubjectID:    input.SubjectID,
	}).Get(ctx, &existing)
	if err != nil {
		return nil, fmt.Errorf("check existing assignment: %w", err)
	}
	if existing.Found {
		return &ExperimentAssignResult{
			AssignmentID: existing.AssignmentID,
			VariantID:    existing.VariantID,
			VariantName:  existing.VariantName,
			Persisted:    true,
			EvidenceHash: existing.EvidenceHash,
		}, nil
	}

	// Step 2: Deterministic variant assignment.
	var assignResult ExperimentDeterministicResult
	err = workflow.ExecuteActivity(ctx, a.DeterministicAssignActivity, ExperimentDeterministicInput{
		WorkspaceID:  input.WorkspaceID,
		ExperimentID: input.ExperimentID,
		SubjectID:    input.SubjectID,
	}).Get(ctx, &assignResult)
	if err != nil {
		return nil, fmt.Errorf("deterministic assign: %w", err)
	}

	// Step 3: Persist assignment.
	var persistResult ExperimentPersistResult
	err = workflow.ExecuteActivity(ctx, a.PersistAssignmentActivity, ExperimentPersistInput{
		WorkspaceID:  input.WorkspaceID,
		ExperimentID: input.ExperimentID,
		SubjectID:    input.SubjectID,
		VariantID:    assignResult.VariantID,
		VariantName:  assignResult.VariantName,
	}).Get(ctx, &persistResult)
	if err != nil {
		return nil, fmt.Errorf("persist assignment: %w", err)
	}

	return &ExperimentAssignResult{
		AssignmentID: persistResult.AssignmentID,
		VariantID:    assignResult.VariantID,
		VariantName:  assignResult.VariantName,
		Persisted:    persistResult.Persisted,
		EvidenceHash: assignResult.EvidenceHash,
	}, nil
}

// ===================== ONBOARDING PROVISIONING =====================

// OnboardingProvisionInput drives workspace provisioning.
type OnboardingProvisionInput struct {
	WorkspaceID string `json:"workspace_id"`
	PlanID      string `json:"plan_id"`
	OperatorID  string `json:"operator_id"`
}

// OnboardingProvisionResult captures provisioning outcome.
type OnboardingProvisionResult struct {
	SessionID          string   `json:"session_id"`
	Status             string   `json:"status"`
	CompletedStages    []string `json:"completed_stages"`
	FirstValueVerified bool     `json:"first_value_verified"`
	EvidenceHash       string   `json:"evidence_hash"`
}

// OnboardingProvisioningWorkflow orchestrates workspace setup through
// staged provisioning with policy gates and first-value verification.
func OnboardingProvisioningWorkflow(ctx workflow.Context, input OnboardingProvisionInput) (*OnboardingProvisionResult, error) {
	logger := workflow.GetLogger(ctx)
	logger.Info("OnboardingProvisioningWorkflow started",
		"workspace", input.WorkspaceID)

	ao := workflow.ActivityOptions{
		StartToCloseTimeout: 120 * time.Second,
		RetryPolicy: &temporal.RetryPolicy{
			InitialInterval:    2 * time.Second,
			BackoffCoefficient: 2.0,
			MaximumInterval:    60 * time.Second,
			MaximumAttempts:    3,
		},
	}
	ctx = workflow.WithActivityOptions(ctx, ao)

	var a *Activities

	// Step 1: Validate plan and create session.
	var initResult OnboardingInitResult
	err := workflow.ExecuteActivity(ctx, a.InitOnboardingSessionActivity, OnboardingInitInput{
		WorkspaceID: input.WorkspaceID,
		PlanID:      input.PlanID,
		OperatorID:  input.OperatorID,
	}).Get(ctx, &initResult)
	if err != nil {
		return &OnboardingProvisionResult{Status: "INIT_FAILED"}, nil
	}

	// Step 2: Execute provisioning stages.
	stages := []string{"workspace_setup", "policy_defaults", "integration_check", "first_value"}
	var completed []string
	for _, stage := range stages {
		var stageResult OnboardingStageExecResult
		err := workflow.ExecuteActivity(ctx, a.ExecuteProvisioningStageActivity, OnboardingStageExecInput{
			WorkspaceID: input.WorkspaceID,
			SessionID:   initResult.SessionID,
			Stage:       stage,
			PlanID:      input.PlanID,
		}).Get(ctx, &stageResult)
		if err != nil {
			logger.Warn("provisioning stage failed", "stage", stage, "error", err)
			continue
		}
		if stageResult.Success {
			completed = append(completed, stage)
		}
	}

	// Step 3: Verify first value was delivered.
	firstValueVerified := false
	for _, s := range completed {
		if s == "first_value" {
			firstValueVerified = true
		}
	}

	// Step 4: Finalize onboarding session.
	status := "completed"
	if len(completed) < len(stages) {
		status = "partial"
	}
	var finalResult OnboardingFinalizeResult
	_ = workflow.ExecuteActivity(ctx, a.FinalizeOnboardingActivity, OnboardingFinalizeInput{
		WorkspaceID:        input.WorkspaceID,
		SessionID:          initResult.SessionID,
		CompletedStages:    completed,
		FirstValueVerified: firstValueVerified,
		Status:             status,
	}).Get(ctx, &finalResult)

	return &OnboardingProvisionResult{
		SessionID:          initResult.SessionID,
		Status:             status,
		CompletedStages:    completed,
		FirstValueVerified: firstValueVerified,
		EvidenceHash:       finalResult.EvidenceHash,
	}, nil
}

// ===================== BILLING ENFORCEMENT =====================

// BillingWebhookInput drives billing webhook processing.
type BillingWebhookInput struct {
	WorkspaceID string `json:"workspace_id"`
	Provider    string `json:"provider"` // stripe, manual
	EventType   string `json:"event_type"`
	EventID     string `json:"event_id"`
	Payload     string `json:"payload"`
}

// BillingWebhookResult captures webhook processing outcome.
type BillingWebhookResult struct {
	Status        string `json:"status"`
	LedgerEntryID string `json:"ledger_entry_id,omitempty"`
	PolicyGated   bool   `json:"policy_gated"`
	EvidenceHash  string `json:"evidence_hash"`
}

// BillingEnforcementWorkflow processes billing webhooks, updates ledger,
// and enforces policy gating on plan changes.
func BillingEnforcementWorkflow(ctx workflow.Context, input BillingWebhookInput) (*BillingWebhookResult, error) {
	logger := workflow.GetLogger(ctx)
	logger.Info("BillingEnforcementWorkflow started",
		"workspace", input.WorkspaceID,
		"event", input.EventType)

	ao := workflow.ActivityOptions{
		StartToCloseTimeout: 60 * time.Second,
		RetryPolicy: &temporal.RetryPolicy{
			InitialInterval:    2 * time.Second,
			BackoffCoefficient: 2.0,
			MaximumInterval:    60 * time.Second,
			MaximumAttempts:    3,
		},
	}
	ctx = workflow.WithActivityOptions(ctx, ao)

	var a *Activities

	// Step 1: Ingest webhook event (idempotent).
	var ingestResult BillingIngestResult
	err := workflow.ExecuteActivity(ctx, a.IngestBillingWebhookActivity, BillingIngestInput{
		WorkspaceID: input.WorkspaceID,
		Provider:    input.Provider,
		EventType:   input.EventType,
		EventID:     input.EventID,
		Payload:     input.Payload,
	}).Get(ctx, &ingestResult)
	if err != nil {
		return &BillingWebhookResult{Status: "INGEST_FAILED"}, nil
	}
	if ingestResult.Duplicate {
		return &BillingWebhookResult{Status: "DUPLICATE", EvidenceHash: ingestResult.EvidenceHash}, nil
	}

	// Step 2: Update billing ledger.
	var ledgerResult BillingLedgerResult
	err = workflow.ExecuteActivity(ctx, a.UpdateBillingLedgerActivity, BillingLedgerInput{
		WorkspaceID: input.WorkspaceID,
		EventType:   input.EventType,
		EventID:     input.EventID,
		Payload:     input.Payload,
	}).Get(ctx, &ledgerResult)
	if err != nil {
		return &BillingWebhookResult{Status: "LEDGER_FAILED"}, nil
	}

	// Step 3: Enforce policy gate on plan-affecting events.
	policyGated := false
	if input.EventType == "customer.subscription.deleted" || input.EventType == "invoice.payment_failed" {
		var policyResult BillingPolicyResult
		err = workflow.ExecuteActivity(ctx, a.EnforceBillingPolicyActivity, BillingPolicyInput{
			WorkspaceID: input.WorkspaceID,
			EventType:   input.EventType,
			AmountCents: ledgerResult.AmountCents,
		}).Get(ctx, &policyResult)
		if err == nil {
			policyGated = policyResult.Enforced
		}
	}

	return &BillingWebhookResult{
		Status:        "PROCESSED",
		LedgerEntryID: ledgerResult.EntryID,
		PolicyGated:   policyGated,
		EvidenceHash:  ledgerResult.EvidenceHash,
	}, nil
}

// ===================== LOAD SHEDDING TIER WORKFLOW =====================

// LoadSheddingInput drives load shedding tier propagation.
type LoadSheddingInput struct {
	WorkspaceID string  `json:"workspace_id"`
	CPUPercent  float64 `json:"cpu_percent"`
	ErrorRate   float64 `json:"error_rate"`
	DBPoolUsage float64 `json:"db_pool_usage"`
}

// LoadSheddingResult captures the tier determination.
type LoadSheddingResult struct {
	CurrentTier  string `json:"current_tier"`
	PreviousTier string `json:"previous_tier"`
	Escalated    bool   `json:"escalated"`
	Reason       string `json:"reason"`
	EvidenceHash string `json:"evidence_hash"`
}

// LoadSheddingTierWorkflow evaluates system metrics and propagates
// load shedding tier changes through the workflow system.
func LoadSheddingTierWorkflow(ctx workflow.Context, input LoadSheddingInput) (*LoadSheddingResult, error) {
	logger := workflow.GetLogger(ctx)
	logger.Info("LoadSheddingTierWorkflow started",
		"workspace", input.WorkspaceID,
		"cpu", input.CPUPercent)

	ao := workflow.ActivityOptions{
		StartToCloseTimeout: 15 * time.Second,
		RetryPolicy: &temporal.RetryPolicy{
			MaximumAttempts: 2,
		},
	}
	ctx = workflow.WithActivityOptions(ctx, ao)

	var a *Activities

	// Step 1: Evaluate tier from metrics.
	var evalResult LoadSheddingEvalResult
	err := workflow.ExecuteActivity(ctx, a.EvaluateLoadSheddingTierActivity, LoadSheddingEvalInput{
		WorkspaceID: input.WorkspaceID,
		CPUPercent:  input.CPUPercent,
		ErrorRate:   input.ErrorRate,
		DBPoolUsage: input.DBPoolUsage,
	}).Get(ctx, &evalResult)
	if err != nil {
		return &LoadSheddingResult{CurrentTier: "D0", Reason: "eval_error"}, nil
	}

	// Step 2: Propagate tier change if needed.
	if evalResult.Changed {
		var propResult LoadSheddingPropagateResult
		_ = workflow.ExecuteActivity(ctx, a.PropagateLoadSheddingTierActivity, LoadSheddingPropagateInput{
			WorkspaceID:  input.WorkspaceID,
			NewTier:      evalResult.NewTier,
			PreviousTier: evalResult.PreviousTier,
			Reason:       evalResult.Reason,
		}).Get(ctx, &propResult)
	}

	return &LoadSheddingResult{
		CurrentTier:  evalResult.NewTier,
		PreviousTier: evalResult.PreviousTier,
		Escalated:    evalResult.Changed && evalResult.NewTier > evalResult.PreviousTier,
		Reason:       evalResult.Reason,
		EvidenceHash: evalResult.EvidenceHash,
	}, nil
}
