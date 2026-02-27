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

func TestDecideProvisioningV1RuleOrder(t *testing.T) {
	t.Parallel()

	// 1) denied_server_ids
	if decision := DecideProvisioningV1(PolicyInput{
		ServerID:        "server_a",
		DeniedServerIDs: []string{"server_a"},
		AllowedServerIDs: []string{
			"server_a",
		},
		MaxAllowedRiskLevel: "CRITICAL",
		RiskLevel:           "LOW",
	}); decision != DecisionDeny {
		t.Fatalf("step 1 expected deny, got %s", decision)
	}

	// 2) allowed_server_ids
	if decision := DecideProvisioningV1(PolicyInput{
		ServerID:            "server_x",
		AllowedServerIDs:    []string{"server_a", "server_b"},
		MaxAllowedRiskLevel: "CRITICAL",
		RiskLevel:           "LOW",
	}); decision != DecisionDeny {
		t.Fatalf("step 2 expected deny, got %s", decision)
	}

	// 3) risk_level
	if decision := DecideProvisioningV1(PolicyInput{
		ServerID:            "server_a",
		AllowedServerIDs:    []string{"server_a"},
		MaxAllowedRiskLevel: "MEDIUM",
		RiskLevel:           "CRITICAL",
	}); decision != DecisionDeny {
		t.Fatalf("step 3 expected deny, got %s", decision)
	}

	// 4) budget exhausted
	if decision := DecideProvisioningV1(PolicyInput{
		ServerID:            "server_a",
		AllowedServerIDs:    []string{"server_a"},
		MaxAllowedRiskLevel: "CRITICAL",
		RiskLevel:           "LOW",
		BudgetExhausted:     true,
	}); decision != DecisionDeny {
		t.Fatalf("step 4 expected deny, got %s", decision)
	}

	// 5) operator review threshold
	if decision := DecideProvisioningV1(PolicyInput{
		ServerID:                       "server_a",
		AllowedServerIDs:               []string{"server_a"},
		MaxAllowedRiskLevel:            "CRITICAL",
		RiskLevel:                      "ELEVATED",
		RequireOperatorReviewAtOrAbove: "ELEVATED",
	}); decision != DecisionRequireOperatorReview {
		t.Fatalf("step 5 expected operator review, got %s", decision)
	}

	// 6) oauth owner approval
	if decision := DecideProvisioningV1(PolicyInput{
		ServerID:                   "server_a",
		AllowedServerIDs:           []string{"server_a"},
		MaxAllowedRiskLevel:        "CRITICAL",
		RiskLevel:                  "LOW",
		OAuthOwnerApprovalRequired: true,
	}); decision != DecisionRequireUserApproval {
		t.Fatalf("step 6 expected user approval, got %s", decision)
	}

	// 7) mcp deploy owner approval
	if decision := DecideProvisioningV1(PolicyInput{
		ServerID:                       "server_a",
		AllowedServerIDs:               []string{"server_a"},
		MaxAllowedRiskLevel:            "CRITICAL",
		RiskLevel:                      "LOW",
		MCPDeployOwnerApprovalRequired: true,
	}); decision != DecisionRequireUserApproval {
		t.Fatalf("step 7 expected user approval, got %s", decision)
	}

	// 8) allow
	if decision := DecideProvisioningV1(PolicyInput{
		ServerID:            "server_a",
		AllowedServerIDs:    []string{"server_a"},
		MaxAllowedRiskLevel: "CRITICAL",
		RiskLevel:           "LOW",
	}); decision != DecisionAllow {
		t.Fatalf("step 8 expected allow, got %s", decision)
	}
}

func TestProvisioningRBACHierarchy(t *testing.T) {
	t.Parallel()

	if !(RoleRank(RoleOwner) > RoleRank(RoleAdmin) &&
		RoleRank(RoleAdmin) > RoleRank(RoleDelegate) &&
		RoleRank(RoleDelegate) > RoleRank(RoleAuditor) &&
		RoleRank(RoleAuditor) > RoleRank(RoleOperator)) {
		t.Fatal("rbac hierarchy ordering mismatch")
	}

	if !CanApproveOAuthAndDeploy(RoleOwner) {
		t.Fatal("owner must be allowed to approve oauth/deploy")
	}
	if !CanApproveOAuthAndDeploy(RoleAdmin) {
		t.Fatal("admin must be allowed to approve oauth/deploy")
	}
	for _, role := range []Role{RoleDelegate, RoleAuditor, RoleOperator} {
		if CanApproveOAuthAndDeploy(role) {
			t.Fatalf("role %s must not approve oauth/deploy", role)
		}
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
