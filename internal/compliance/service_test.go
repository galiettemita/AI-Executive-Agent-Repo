package compliance

import (
	"strings"
	"testing"
	"time"
)

func TestComplianceLifecycle(t *testing.T) {
	t.Parallel()

	s := NewService()
	base := time.Date(2026, 2, 28, 12, 0, 0, 0, time.UTC)
	s.now = func() time.Time { return base }

	framework := s.UpsertFramework(Framework{
		WorkspaceID: "ws_1",
		Key:         "soc2",
		Status:      "active",
		VersionInt:  1,
	})
	if framework.ID == "" {
		t.Fatalf("expected framework id")
	}

	s.AddEvidence(Evidence{
		WorkspaceID: "ws_1",
		FrameworkID: framework.ID,
		ArtifactURI: "s3://breviosboms/evidence.json",
		SHA256:      "sha256:abc123",
	})
	evidence := s.ListEvidence("ws_1")
	if len(evidence) != 1 {
		t.Fatalf("expected 1 evidence item, got %d", len(evidence))
	}
	if evidence[0].CollectedAt == "" {
		t.Fatalf("expected evidence collected timestamp")
	}

	request := s.CreateDSR(DSRRequest{
		WorkspaceID: "ws_1",
		UserID:      "user_1",
		RequestType: "deletion",
	})
	if request.ID == "" {
		t.Fatalf("expected dsr id")
	}
	if request.RequestID == "" || request.DeadlineAt == "" {
		t.Fatalf("expected schema-aligned request fields: %#v", request)
	}

	updated, ok := s.UpdateDSR(request.ID, DSRRequest{Status: "in_progress"})
	if !ok {
		t.Fatalf("expected dsr update success")
	}
	if updated.Status != "in_progress" {
		t.Fatalf("unexpected dsr status: %#v", updated)
	}

	requests := s.ListDSR("ws_1")
	if len(requests) != 1 {
		t.Fatalf("expected 1 dsr request, got %d", len(requests))
	}
}

func TestAddEvidenceComputesHashWhenMissing(t *testing.T) {
	t.Parallel()

	svc := NewService()
	evidence := svc.AddEvidence(Evidence{
		WorkspaceID: "ws_hash_missing",
		FrameworkID: "framework_1",
		EventType:   "BREVIO.compliance.evidence_collected.v1",
		ArtifactURI: "s3://breviosboms/evidence.json",
	})
	if !strings.HasPrefix(evidence.SHA256, "sha256:") {
		t.Fatalf("expected sha256 prefix, got %s", evidence.SHA256)
	}
	digest := strings.TrimPrefix(evidence.SHA256, "sha256:")
	if len(digest) != 64 {
		t.Fatalf("expected 64-hex digest, got %s", evidence.SHA256)
	}
}

func TestAddEvidenceNormalizesProvidedDigest(t *testing.T) {
	t.Parallel()

	svc := NewService()
	inputDigest := "A237C8B072402B9E53D6329E6A14F1F0B9ABCA81FA0D9A74C8947E7EA7607195"
	evidence := svc.AddEvidence(Evidence{
		WorkspaceID: "ws_hash_normalize",
		FrameworkID: "framework_2",
		EventType:   "BREVIO.compliance.evidence_collected.v1",
		ArtifactURI: "s3://breviosboms/evidence.json",
		SHA256:      inputDigest,
	})
	if evidence.SHA256 != "sha256:"+strings.ToLower(inputDigest) {
		t.Fatalf("unexpected normalized hash: %s", evidence.SHA256)
	}
}

func TestDSRSLAAtRiskDetection(t *testing.T) {
	t.Parallel()

	s := NewService()
	base := time.Date(2026, 2, 28, 12, 0, 0, 0, time.UTC)
	s.now = func() time.Time { return base }

	request := s.CreateDSR(DSRRequest{
		WorkspaceID: "ws_sla",
		UserID:      "user_sla",
		RequestType: "access",
	})

	// Move time near deadline to trigger SLA warning window.
	s.now = func() time.Time { return base.Add(26 * 24 * time.Hour) }
	atRisk := s.ListDSRAtRisk("ws_sla")
	if len(atRisk) != 1 {
		t.Fatalf("expected one SLA-at-risk request, got %d", len(atRisk))
	}
	if atRisk[0].ID != request.ID || !atRisk[0].SLAAtRisk {
		t.Fatalf("unexpected SLA-at-risk payload: %#v", atRisk[0])
	}

	if _, ok := s.UpdateDSR(request.ID, DSRRequest{Status: "completed"}); !ok {
		t.Fatal("expected dsr completion update")
	}
	if len(s.ListDSRAtRisk("ws_sla")) != 0 {
		t.Fatal("expected no SLA-at-risk requests after completion")
	}
}

func TestDeletionRequestProducesIrreversibleDeletionReport(t *testing.T) {
	t.Parallel()

	s := NewService()
	base := time.Date(2026, 2, 28, 12, 0, 0, 0, time.UTC)
	s.now = func() time.Time { return base }

	request := s.CreateDSR(DSRRequest{
		WorkspaceID: "ws_delete",
		UserID:      "user_delete",
		RequestType: "deletion",
		Status:      "received",
	})
	if request.RequestID == "" {
		t.Fatalf("expected request_id for deletion dsr: %#v", request)
	}

	updated, ok := s.UpdateDSR(request.ID, DSRRequest{Status: "completed", RequestType: "deletion"})
	if !ok {
		t.Fatal("expected deletion dsr update to succeed")
	}
	if updated.Status != "completed" {
		t.Fatalf("expected completed deletion status, got: %#v", updated)
	}

	report, ok := s.GetDeletionReport(request.RequestID)
	if !ok {
		t.Fatalf("expected deletion report for request %s", request.RequestID)
	}
	if !report.Irreversible || !report.DatabasePurged || !report.CachePurged {
		t.Fatalf("expected irreversible DB/cache purge report, got: %#v", report)
	}
	if !report.ConnectorRevoked || !report.MCPOAuthRevoked {
		t.Fatalf("expected connector and mcp oauth revocations, got: %#v", report)
	}
	if report.BackupRotationDays > 30 {
		t.Fatalf("expected backup rotation <= 30 days, got: %#v", report)
	}
	if len(s.ListDeletionReports("ws_delete")) != 1 {
		t.Fatalf("expected one workspace deletion report")
	}
}

func TestDefaultRetentionPoliciesAndClassMapping(t *testing.T) {
	t.Parallel()

	policies := DefaultRetentionPolicies()
	if len(policies) != 6 {
		t.Fatalf("unexpected retention policy count: %d", len(policies))
	}
	if got := policies["RP-005"]; got.RetentionPeriod != 30*24*time.Hour || got.ExpiryAction != "hard_delete" {
		t.Fatalf("unexpected RP-005 policy: %+v", got)
	}
	if got := DefaultRetentionPolicyForDataClass("FINANCIAL"); got != "RP-002" {
		t.Fatalf("unexpected FINANCIAL retention policy: %s", got)
	}
	if got := DefaultRetentionPolicyForDataClass("HEALTH"); got != "RP-004" {
		t.Fatalf("unexpected HEALTH retention policy: %s", got)
	}
	if got := DefaultRetentionPolicyForDataClass("PUBLIC"); got != "RP-001" {
		t.Fatalf("unexpected PUBLIC retention policy: %s", got)
	}
}
