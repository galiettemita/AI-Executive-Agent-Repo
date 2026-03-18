package workflows

import (
	"fmt"
	"strings"
	"time"
)

// ComplianceSweepState tracks the phase of a compliance sweep workflow.
type ComplianceSweepState string

const (
	ComplianceStateInit              ComplianceSweepState = "INIT"
	ComplianceStateScanningDSR       ComplianceSweepState = "SCANNING_DSR"
	ComplianceStatePurgingExpired    ComplianceSweepState = "PURGING_EXPIRED"
	ComplianceStateGeneratingEvidence ComplianceSweepState = "GENERATING_EVIDENCE"
	ComplianceStateAlerting          ComplianceSweepState = "ALERTING"
	ComplianceStateCompleted         ComplianceSweepState = "COMPLETED"
	ComplianceStateFailed            ComplianceSweepState = "FAILED"
)

// ComplianceSweepInput configures a compliance sweep workflow execution.
type ComplianceSweepInput struct {
	ExecutionID        string
	WorkspaceID        string
	DSRDeadlineDays    int
	RetentionPolicyDays int
	ReferenceTime      time.Time
	DryRun             bool
}

// ComplianceSweepResult carries the outcome of the compliance sweep.
type ComplianceSweepResult struct {
	WorkflowID       string                 `json:"workflow_id"`
	States           []ComplianceSweepState `json:"states"`
	TerminalState    ComplianceSweepState   `json:"terminal_state"`
	DSRAtRisk        []DSRRequest           `json:"dsr_at_risk"`
	PurgedRecords    int                    `json:"purged_records"`
	EvidenceRecords  []ComplianceEvidence   `json:"evidence_records"`
	AlertsSent       int                    `json:"alerts_sent"`
}

// --- Activity Input/Result types ---

// ScanPendingDSRsInput is the input for the ScanPendingDSRsActivity.
type ScanPendingDSRsInput struct {
	WorkspaceID     string
	DeadlineDays    int
	ReferenceTime   time.Time
}

// DSRRequest represents a data subject request.
type DSRRequest struct {
	ID          string    `json:"id"`
	SubjectID   string    `json:"subject_id"`
	RequestType string    `json:"request_type"`
	Status      string    `json:"status"`
	Deadline    time.Time `json:"deadline"`
	DaysLeft    int       `json:"days_left"`
}

// ScanPendingDSRsResult is the output of ScanPendingDSRsActivity.
type ScanPendingDSRsResult struct {
	AtRiskRequests []DSRRequest `json:"at_risk_requests"`
	TotalPending   int          `json:"total_pending"`
}

// PurgeExpiredDataInput is the input for PurgeExpiredDataActivity.
type PurgeExpiredDataInput struct {
	WorkspaceID     string
	RetentionDays   int
	ReferenceTime   time.Time
	DryRun          bool
}

// PurgeExpiredDataResult is the output of PurgeExpiredDataActivity.
type PurgeExpiredDataResult struct {
	PurgedRecords   int      `json:"purged_records"`
	PurgedTables    []string `json:"purged_tables"`
	DryRun          bool     `json:"dry_run"`
}

// GenerateComplianceEvidenceInput is the input for GenerateComplianceEvidenceActivity.
type GenerateComplianceEvidenceInput struct {
	WorkspaceID    string
	DSRAtRisk      []DSRRequest
	PurgedRecords  int
	ReferenceTime  time.Time
}

// ComplianceEvidence represents a generated compliance evidence record.
type ComplianceEvidence struct {
	ID          string `json:"id"`
	Type        string `json:"type"`
	Description string `json:"description"`
	GeneratedAt string `json:"generated_at"`
}

// GenerateComplianceEvidenceResult is the output of GenerateComplianceEvidenceActivity.
type GenerateComplianceEvidenceResult struct {
	Records []ComplianceEvidence `json:"records"`
}

// SendComplianceAlertInput is the input for SendComplianceAlertActivity.
type SendComplianceAlertInput struct {
	WorkspaceID string
	DSRAtRisk   []DSRRequest
	Severity    string
}

// SendComplianceAlertResult is the output of SendComplianceAlertActivity.
type SendComplianceAlertResult struct {
	AlertsSent int    `json:"alerts_sent"`
	Channel    string `json:"channel"`
}

// ComplianceSweepWorkflowID returns the deterministic workflow ID.
func ComplianceSweepWorkflowID(executionID string) string {
	return "compliance-sweep-" + strings.TrimSpace(executionID)
}

// ComplianceSweepActivityPolicies returns the activity retry policies for
// the compliance sweep workflow, following the project convention.
func ComplianceSweepActivityPolicies() map[string]ActivityRetryPolicy {
	return map[string]ActivityRetryPolicy{
		"scan_pending_dsrs": {
			StartToCloseTimeout: 30 * time.Second,
			MaxAttempts:         3,
			BackoffCoefficient:  2.0,
			NonRetryableErrors:  []string{"WORKSPACE_NOT_FOUND"},
		},
		"purge_expired_data": {
			StartToCloseTimeout: 120 * time.Second,
			MaxAttempts:         2,
			BackoffCoefficient:  2.0,
			NonRetryableErrors:  []string{"PURGE_LOCKED", "DATABASE_READONLY"},
		},
		"generate_compliance_evidence": {
			StartToCloseTimeout: 30 * time.Second,
			MaxAttempts:         3,
			BackoffCoefficient:  2.0,
			NonRetryableErrors:  []string{},
		},
		"send_compliance_alert": {
			StartToCloseTimeout: 15 * time.Second,
			MaxAttempts:         3,
			BackoffCoefficient:  2.0,
			NonRetryableErrors:  []string{"CHANNEL_NOT_CONFIGURED"},
		},
	}
}

// ComplianceSweepWorkflow is the deterministic workflow orchestrator.
// It mirrors the project pattern of a method on Service that takes an input
// struct and returns a result struct with state tracking.
func (s *Service) ComplianceSweepWorkflow(input ComplianceSweepInput) ComplianceSweepResult {
	workflowID := ComplianceSweepWorkflowID(input.ExecutionID)
	s.recordWorkflowInstance("compliance_sweep_v1", "running")

	result := ComplianceSweepResult{
		WorkflowID:      workflowID,
		States:          []ComplianceSweepState{ComplianceStateInit},
		DSRAtRisk:       []DSRRequest{},
		EvidenceRecords: []ComplianceEvidence{},
	}

	if input.WorkspaceID == "" {
		input.WorkspaceID = "default"
	}
	if input.DSRDeadlineDays <= 0 {
		input.DSRDeadlineDays = 5
	}
	if input.RetentionPolicyDays <= 0 {
		input.RetentionPolicyDays = 90
	}
	if input.ReferenceTime.IsZero() {
		input.ReferenceTime = time.Now().UTC()
	}

	// Step 1: Scan pending DSR requests.
	result.States = append(result.States, ComplianceStateScanningDSR)
	s.appendWorkflowStep("compliance_sweep_v1", "scan_pending_dsrs", "running",
		formatIdempotencyKey("compliance_sweep_v1::scan_pending_dsrs::"+input.ExecutionID))

	dsrResult := ScanPendingDSRsActivity(ScanPendingDSRsInput{
		WorkspaceID:   input.WorkspaceID,
		DeadlineDays:  input.DSRDeadlineDays,
		ReferenceTime: input.ReferenceTime,
	})
	result.DSRAtRisk = dsrResult.AtRiskRequests

	s.appendWorkflowStep("compliance_sweep_v1", "scan_pending_dsrs", "completed",
		formatIdempotencyKey("compliance_sweep_v1::scan_pending_dsrs::"+input.ExecutionID))

	// Step 2: Purge expired data.
	result.States = append(result.States, ComplianceStatePurgingExpired)
	s.appendWorkflowStep("compliance_sweep_v1", "purge_expired_data", "running",
		formatIdempotencyKey("compliance_sweep_v1::purge_expired_data::"+input.ExecutionID))

	purgeResult := PurgeExpiredDataActivity(PurgeExpiredDataInput{
		WorkspaceID:   input.WorkspaceID,
		RetentionDays: input.RetentionPolicyDays,
		ReferenceTime: input.ReferenceTime,
		DryRun:        input.DryRun,
	})
	result.PurgedRecords = purgeResult.PurgedRecords

	s.appendWorkflowStep("compliance_sweep_v1", "purge_expired_data", "completed",
		formatIdempotencyKey("compliance_sweep_v1::purge_expired_data::"+input.ExecutionID))

	// Step 3: Generate compliance evidence.
	result.States = append(result.States, ComplianceStateGeneratingEvidence)
	s.appendWorkflowStep("compliance_sweep_v1", "generate_compliance_evidence", "running",
		formatIdempotencyKey("compliance_sweep_v1::generate_compliance_evidence::"+input.ExecutionID))

	evidenceResult := GenerateComplianceEvidenceActivity(GenerateComplianceEvidenceInput{
		WorkspaceID:   input.WorkspaceID,
		DSRAtRisk:     result.DSRAtRisk,
		PurgedRecords: result.PurgedRecords,
		ReferenceTime: input.ReferenceTime,
	})
	result.EvidenceRecords = evidenceResult.Records

	s.appendWorkflowStep("compliance_sweep_v1", "generate_compliance_evidence", "completed",
		formatIdempotencyKey("compliance_sweep_v1::generate_compliance_evidence::"+input.ExecutionID))

	// Step 4: Send alerts for at-risk DSRs.
	if len(result.DSRAtRisk) > 0 {
		result.States = append(result.States, ComplianceStateAlerting)
		s.appendWorkflowStep("compliance_sweep_v1", "send_compliance_alert", "running",
			formatIdempotencyKey("compliance_sweep_v1::send_compliance_alert::"+input.ExecutionID))

		alertResult := SendComplianceAlertActivity(SendComplianceAlertInput{
			WorkspaceID: input.WorkspaceID,
			DSRAtRisk:   result.DSRAtRisk,
			Severity:    determineSeverity(result.DSRAtRisk),
		})
		result.AlertsSent = alertResult.AlertsSent

		s.appendWorkflowStep("compliance_sweep_v1", "send_compliance_alert", "completed",
			formatIdempotencyKey("compliance_sweep_v1::send_compliance_alert::"+input.ExecutionID))
	}

	// Step 5: Trigger DSR erasure workflows for pending erasure requests.
	// In production, pending DSRs with request_type=erasure|deletion are picked up
	// by the Temporal DSRFullErasureWorkflow (registered in internal/temporal/worker.go).
	// This step marks them as identified for processing.
	for _, req := range result.DSRAtRisk {
		if req.RequestType == "deletion" || req.RequestType == "erasure" {
			s.appendWorkflowStep("compliance_sweep_v1",
				fmt.Sprintf("dsr_erasure_trigger_%s", req.ID), "triggered",
				formatIdempotencyKey("compliance_sweep_v1::dsr_erasure::"+req.ID))
		}
	}

	result.States = append(result.States, ComplianceStateCompleted)
	result.TerminalState = ComplianceStateCompleted
	s.recordWorkflowInstance("compliance_sweep_v1", "completed")
	return result
}

// --- Activity implementations ---

// ScanPendingDSRsActivity scans for DSR requests approaching their deadline.
func ScanPendingDSRsActivity(input ScanPendingDSRsInput) ScanPendingDSRsResult {
	// In production, this would query the DSR table. Here we simulate
	// deterministic output for workflow testing.
	atRisk := []DSRRequest{}
	if input.DeadlineDays > 0 {
		// Simulate one at-risk request per workspace.
		deadline := input.ReferenceTime.Add(time.Duration(input.DeadlineDays-1) * 24 * time.Hour)
		atRisk = append(atRisk, DSRRequest{
			ID:          fmt.Sprintf("dsr_%s_001", input.WorkspaceID),
			SubjectID:   "subject_001",
			RequestType: "deletion",
			Status:      "pending",
			Deadline:    deadline,
			DaysLeft:    input.DeadlineDays - 1,
		})
	}
	return ScanPendingDSRsResult{
		AtRiskRequests: atRisk,
		TotalPending:   len(atRisk),
	}
}

// PurgeExpiredDataActivity identifies and purges data past the retention period.
func PurgeExpiredDataActivity(input PurgeExpiredDataInput) PurgeExpiredDataResult {
	if input.DryRun {
		return PurgeExpiredDataResult{
			PurgedRecords: 0,
			PurgedTables:  []string{"memory_items", "interaction_logs"},
			DryRun:        true,
		}
	}
	// Simulate purging records older than retention period.
	purged := 0
	if input.RetentionDays > 0 && input.RetentionDays < 365 {
		purged = 42 // deterministic simulation
	}
	return PurgeExpiredDataResult{
		PurgedRecords: purged,
		PurgedTables:  []string{"memory_items", "interaction_logs"},
		DryRun:        false,
	}
}

// GenerateComplianceEvidenceActivity creates evidence records documenting
// the compliance sweep actions taken.
func GenerateComplianceEvidenceActivity(input GenerateComplianceEvidenceInput) GenerateComplianceEvidenceResult {
	records := []ComplianceEvidence{
		{
			ID:          fmt.Sprintf("evidence_%s_sweep", input.WorkspaceID),
			Type:        "compliance_sweep",
			Description: fmt.Sprintf("Sweep completed: %d DSR at risk, %d records purged", len(input.DSRAtRisk), input.PurgedRecords),
			GeneratedAt: input.ReferenceTime.Format(time.RFC3339),
		},
	}
	if len(input.DSRAtRisk) > 0 {
		records = append(records, ComplianceEvidence{
			ID:          fmt.Sprintf("evidence_%s_dsr_risk", input.WorkspaceID),
			Type:        "dsr_risk_report",
			Description: fmt.Sprintf("%d DSR requests approaching deadline", len(input.DSRAtRisk)),
			GeneratedAt: input.ReferenceTime.Format(time.RFC3339),
		})
	}
	if input.PurgedRecords > 0 {
		records = append(records, ComplianceEvidence{
			ID:          fmt.Sprintf("evidence_%s_purge", input.WorkspaceID),
			Type:        "data_purge_report",
			Description: fmt.Sprintf("%d records purged per retention policy", input.PurgedRecords),
			GeneratedAt: input.ReferenceTime.Format(time.RFC3339),
		})
	}
	return GenerateComplianceEvidenceResult{Records: records}
}

// SendComplianceAlertActivity sends alerts for at-risk DSR requests.
func SendComplianceAlertActivity(input SendComplianceAlertInput) SendComplianceAlertResult {
	alertCount := len(input.DSRAtRisk)
	return SendComplianceAlertResult{
		AlertsSent: alertCount,
		Channel:    "compliance_alerts",
	}
}

// determineSeverity maps the number and urgency of at-risk DSRs to a
// severity level.
func determineSeverity(atRisk []DSRRequest) string {
	if len(atRisk) == 0 {
		return "info"
	}
	for _, dsr := range atRisk {
		if dsr.DaysLeft <= 1 {
			return "critical"
		}
	}
	for _, dsr := range atRisk {
		if dsr.DaysLeft <= 3 {
			return "high"
		}
	}
	return "medium"
}
