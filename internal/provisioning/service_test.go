package provisioning

import "testing"

func TestCapabilityResolutionDeterministicAcross20Calls(t *testing.T) {
	t.Parallel()

	svc := NewService()
	first := svc.ResolveCapabilities("Schedule a calendar email follow-up", true)
	for i := 0; i < 19; i++ {
		next := svc.ResolveCapabilities("Schedule a calendar email follow-up", true)
		if next.ResponseHash != first.ResponseHash {
			t.Fatalf("hash mismatch at run %d", i+2)
		}
	}
}

func TestPolicyGateDenyAndApprovalBranches(t *testing.T) {
	t.Parallel()

	denyDecision := DecideProvisioningV1(PolicyInput{
		ServerID:            "server_a",
		RiskLevel:           "CRITICAL",
		MaxAllowedRiskLevel: "MEDIUM",
		AllowedServerIDs:    []string{"server_a"},
		DeniedServerIDs:     nil,
		BudgetExhausted:     false,
	})
	if denyDecision != DecisionDeny {
		t.Fatalf("expected deny, got %s", denyDecision)
	}

	reviewDecision := DecideProvisioningV1(PolicyInput{
		ServerID:                       "server_a",
		RiskLevel:                      "ELEVATED",
		MaxAllowedRiskLevel:            "CRITICAL",
		RequireOperatorReviewAtOrAbove: "ELEVATED",
	})
	if reviewDecision != DecisionRequireOperatorReview {
		t.Fatalf("expected operator review, got %s", reviewDecision)
	}
}

func TestArtifactVerificationRejectsTamperedDigest(t *testing.T) {
	t.Parallel()

	manifest := ArtifactManifest{ImageDigest: "sha256:abc", DigestSHA256: "deadbeef"}
	if err := VerifyArtifact(manifest, []byte("artifact-content")); err == nil {
		t.Fatal("expected digest mismatch error")
	}
}

func TestDriftWatchdogQuarantine(t *testing.T) {
	t.Parallel()

	if status := DriftWatchdog(true); status != "quarantine" {
		t.Fatalf("expected quarantine, got %s", status)
	}
}
