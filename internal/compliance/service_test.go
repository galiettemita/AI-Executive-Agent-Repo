package compliance

import "testing"

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
