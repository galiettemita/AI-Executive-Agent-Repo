package compliance

import (
	"strings"
	"testing"
)

func TestComplianceLifecycle(t *testing.T) {
	s := NewService()

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

	request := s.CreateDSR(DSRRequest{
		WorkspaceID: "ws_1",
		UserID:      "user_1",
		RequestType: "deletion",
	})
	if request.ID == "" {
		t.Fatalf("expected dsr id")
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
