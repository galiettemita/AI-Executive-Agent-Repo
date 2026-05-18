package learning

import (
	"testing"
)

func TestAddCorrection(t *testing.T) {
	svc := NewLessonConsolidationService()

	err := svc.AddCorrection("ws1", "always format dates as YYYY-MM-DD")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Empty workspace.
	err = svc.AddCorrection("", "something")
	if err == nil {
		t.Fatal("expected error for empty workspace")
	}

	// Empty correction.
	err = svc.AddCorrection("ws1", "  ")
	if err == nil {
		t.Fatal("expected error for empty correction")
	}
}

func TestClusterCorrections(t *testing.T) {
	svc := NewLessonConsolidationService()

	// Add corrections that should cluster on "format".
	_ = svc.AddCorrection("ws1", "format dates correctly")
	_ = svc.AddCorrection("ws1", "format names in title case")
	_ = svc.AddCorrection("ws1", "format addresses with zip code")

	// Add a singleton that won't cluster.
	_ = svc.AddCorrection("ws1", "remember to include signature")

	clusters, err := svc.ClusterCorrections("ws1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(clusters) == 0 {
		t.Fatal("expected at least one cluster")
	}

	found := false
	for _, c := range clusters {
		if c.Topic == "format" && c.Count >= 3 {
			found = true
		}
	}
	if !found {
		t.Fatal("expected a cluster with topic 'format' and count >= 3")
	}
}

func TestClusterCorrections_EmptyWorkspace(t *testing.T) {
	svc := NewLessonConsolidationService()
	_, err := svc.ClusterCorrections("")
	if err == nil {
		t.Fatal("expected error for empty workspace")
	}
}

func TestProposeRulesAndConfirm(t *testing.T) {
	svc := NewLessonConsolidationService()

	_ = svc.AddCorrection("ws1", "email should have greeting")
	_ = svc.AddCorrection("ws1", "email should have sign-off")

	clusters, _ := svc.ClusterCorrections("ws1")
	if len(clusters) == 0 {
		t.Fatal("expected clusters")
	}

	proposal, err := svc.ProposeRules(clusters[0])
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if proposal.Status != "pending" {
		t.Fatalf("expected pending status, got %s", proposal.Status)
	}

	pending := svc.GetPendingProposals()
	if len(pending) != 1 {
		t.Fatalf("expected 1 pending proposal, got %d", len(pending))
	}

	err = svc.ConfirmProposal(clusters[0].ID)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	pending = svc.GetPendingProposals()
	if len(pending) != 0 {
		t.Fatalf("expected 0 pending after confirm, got %d", len(pending))
	}
}

func TestRejectProposal(t *testing.T) {
	svc := NewLessonConsolidationService()

	_ = svc.AddCorrection("ws1", "reply quickly to urgent messages")
	_ = svc.AddCorrection("ws1", "reply with acknowledgement")

	clusters, _ := svc.ClusterCorrections("ws1")
	if len(clusters) == 0 {
		t.Fatal("expected clusters")
	}

	_, _ = svc.ProposeRules(clusters[0])

	err := svc.RejectProposal(clusters[0].ID)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	pending := svc.GetPendingProposals()
	if len(pending) != 0 {
		t.Fatalf("expected 0 pending after reject, got %d", len(pending))
	}

	// Reject nonexistent.
	err = svc.RejectProposal("nonexistent")
	if err == nil {
		t.Fatal("expected error for nonexistent proposal")
	}
}
