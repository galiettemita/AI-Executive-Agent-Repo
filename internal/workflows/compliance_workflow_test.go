package workflows

import (
	"slices"
	"testing"
	"time"
)

func TestComplianceSweepWorkflowHappyPath(t *testing.T) {
	t.Parallel()
	svc := NewService()
	result := svc.ComplianceSweepWorkflow(ComplianceSweepInput{
		ExecutionID:         "sweep-001",
		WorkspaceID:         "ws1",
		DSRDeadlineDays:     5,
		RetentionPolicyDays: 90,
		ReferenceTime:       time.Date(2026, 3, 1, 0, 0, 0, 0, time.UTC),
	})

	if result.WorkflowID != "compliance-sweep-sweep-001" {
		t.Fatalf("unexpected workflow ID: %s", result.WorkflowID)
	}
	if result.TerminalState != ComplianceStateCompleted {
		t.Fatalf("expected COMPLETED, got %s", result.TerminalState)
	}
	if !slices.Contains(result.States, ComplianceStateScanningDSR) {
		t.Fatalf("expected SCANNING_DSR state: %v", result.States)
	}
	if !slices.Contains(result.States, ComplianceStatePurgingExpired) {
		t.Fatalf("expected PURGING_EXPIRED state: %v", result.States)
	}
	if !slices.Contains(result.States, ComplianceStateGeneratingEvidence) {
		t.Fatalf("expected GENERATING_EVIDENCE state: %v", result.States)
	}
}

func TestComplianceSweepWorkflowAtRiskDSR(t *testing.T) {
	t.Parallel()
	svc := NewService()
	result := svc.ComplianceSweepWorkflow(ComplianceSweepInput{
		ExecutionID:     "sweep-002",
		WorkspaceID:     "ws1",
		DSRDeadlineDays: 5,
		ReferenceTime:   time.Date(2026, 3, 1, 0, 0, 0, 0, time.UTC),
	})

	if len(result.DSRAtRisk) == 0 {
		t.Fatal("expected at-risk DSR requests")
	}
	if !slices.Contains(result.States, ComplianceStateAlerting) {
		t.Fatalf("expected ALERTING state when DSRs are at risk: %v", result.States)
	}
	if result.AlertsSent == 0 {
		t.Fatal("expected alerts to be sent for at-risk DSRs")
	}
}

func TestComplianceSweepWorkflowDryRun(t *testing.T) {
	t.Parallel()
	svc := NewService()
	result := svc.ComplianceSweepWorkflow(ComplianceSweepInput{
		ExecutionID:         "sweep-003",
		WorkspaceID:         "ws1",
		DSRDeadlineDays:     5,
		RetentionPolicyDays: 90,
		DryRun:              true,
		ReferenceTime:       time.Date(2026, 3, 1, 0, 0, 0, 0, time.UTC),
	})

	if result.PurgedRecords != 0 {
		t.Fatalf("expected 0 purged records in dry run, got %d", result.PurgedRecords)
	}
	if result.TerminalState != ComplianceStateCompleted {
		t.Fatalf("expected COMPLETED even in dry run, got %s", result.TerminalState)
	}
}

func TestComplianceSweepWorkflowDefaultValues(t *testing.T) {
	t.Parallel()
	svc := NewService()
	result := svc.ComplianceSweepWorkflow(ComplianceSweepInput{
		ExecutionID: "sweep-004",
	})

	if result.TerminalState != ComplianceStateCompleted {
		t.Fatalf("expected COMPLETED with defaults, got %s", result.TerminalState)
	}
	if result.WorkflowID != "compliance-sweep-sweep-004" {
		t.Fatalf("unexpected workflow ID: %s", result.WorkflowID)
	}
}

func TestComplianceSweepWorkflowEvidenceGeneration(t *testing.T) {
	t.Parallel()
	svc := NewService()
	result := svc.ComplianceSweepWorkflow(ComplianceSweepInput{
		ExecutionID:         "sweep-005",
		WorkspaceID:         "ws1",
		DSRDeadlineDays:     5,
		RetentionPolicyDays: 90,
		ReferenceTime:       time.Date(2026, 3, 1, 0, 0, 0, 0, time.UTC),
	})

	if len(result.EvidenceRecords) == 0 {
		t.Fatal("expected evidence records")
	}
	foundSweep := false
	for _, ev := range result.EvidenceRecords {
		if ev.Type == "compliance_sweep" {
			foundSweep = true
		}
	}
	if !foundSweep {
		t.Fatal("expected compliance_sweep evidence type")
	}
}

func TestComplianceSweepWorkflowID(t *testing.T) {
	t.Parallel()
	id := ComplianceSweepWorkflowID("  sweep-padded  ")
	if id != "compliance-sweep-sweep-padded" {
		t.Fatalf("expected trimmed ID, got %s", id)
	}
}

func TestComplianceSweepActivityPolicies(t *testing.T) {
	t.Parallel()
	policies := ComplianceSweepActivityPolicies()
	expected := []string{"scan_pending_dsrs", "purge_expired_data", "generate_compliance_evidence", "send_compliance_alert"}
	for _, key := range expected {
		if _, ok := policies[key]; !ok {
			t.Fatalf("missing activity policy: %s", key)
		}
	}
}

func TestScanPendingDSRsActivity(t *testing.T) {
	t.Parallel()
	result := ScanPendingDSRsActivity(ScanPendingDSRsInput{
		WorkspaceID:   "ws1",
		DeadlineDays:  5,
		ReferenceTime: time.Date(2026, 3, 1, 0, 0, 0, 0, time.UTC),
	})

	if len(result.AtRiskRequests) == 0 {
		t.Fatal("expected at-risk requests")
	}
	if result.AtRiskRequests[0].DaysLeft != 4 {
		t.Fatalf("expected 4 days left, got %d", result.AtRiskRequests[0].DaysLeft)
	}
}

func TestPurgeExpiredDataActivityDryRun(t *testing.T) {
	t.Parallel()
	result := PurgeExpiredDataActivity(PurgeExpiredDataInput{
		WorkspaceID:   "ws1",
		RetentionDays: 90,
		DryRun:        true,
	})

	if result.PurgedRecords != 0 {
		t.Fatalf("expected 0 purged in dry run, got %d", result.PurgedRecords)
	}
	if !result.DryRun {
		t.Fatal("expected dry_run flag to be true")
	}
}

func TestPurgeExpiredDataActivityLive(t *testing.T) {
	t.Parallel()
	result := PurgeExpiredDataActivity(PurgeExpiredDataInput{
		WorkspaceID:   "ws1",
		RetentionDays: 90,
		DryRun:        false,
	})

	if result.PurgedRecords != 42 {
		t.Fatalf("expected 42 purged records, got %d", result.PurgedRecords)
	}
}

func TestGenerateComplianceEvidenceActivity(t *testing.T) {
	t.Parallel()
	result := GenerateComplianceEvidenceActivity(GenerateComplianceEvidenceInput{
		WorkspaceID:   "ws1",
		DSRAtRisk:     []DSRRequest{{ID: "dsr_1", DaysLeft: 3}},
		PurgedRecords: 10,
		ReferenceTime: time.Date(2026, 3, 1, 0, 0, 0, 0, time.UTC),
	})

	if len(result.Records) < 3 {
		t.Fatalf("expected at least 3 evidence records, got %d", len(result.Records))
	}
}

func TestSendComplianceAlertActivity(t *testing.T) {
	t.Parallel()
	result := SendComplianceAlertActivity(SendComplianceAlertInput{
		WorkspaceID: "ws1",
		DSRAtRisk:   []DSRRequest{{ID: "dsr_1"}, {ID: "dsr_2"}},
		Severity:    "high",
	})

	if result.AlertsSent != 2 {
		t.Fatalf("expected 2 alerts sent, got %d", result.AlertsSent)
	}
	if result.Channel != "compliance_alerts" {
		t.Fatalf("unexpected channel: %s", result.Channel)
	}
}

func TestDetermineSeverity(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name     string
		atRisk   []DSRRequest
		expected string
	}{
		{"empty", []DSRRequest{}, "info"},
		{"critical", []DSRRequest{{DaysLeft: 1}}, "critical"},
		{"high", []DSRRequest{{DaysLeft: 2}}, "high"},
		{"medium", []DSRRequest{{DaysLeft: 4}}, "medium"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got := determineSeverity(tc.atRisk)
			if got != tc.expected {
				t.Fatalf("expected %s, got %s", tc.expected, got)
			}
		})
	}
}

func TestComplianceSweepWorkflowStepsTracked(t *testing.T) {
	t.Parallel()
	svc := NewService()
	_ = svc.ComplianceSweepWorkflow(ComplianceSweepInput{
		ExecutionID: "sweep-006",
		WorkspaceID: "ws1",
	})

	instance, ok := svc.WorkflowInstance("compliance_sweep_v1")
	if !ok {
		t.Fatal("expected workflow instance to be recorded")
	}
	if instance.Status != "completed" {
		t.Fatalf("expected completed status, got %s", instance.Status)
	}

	steps := svc.WorkflowSteps("compliance_sweep_v1")
	if len(steps) == 0 {
		t.Fatal("expected workflow steps to be recorded")
	}
}
