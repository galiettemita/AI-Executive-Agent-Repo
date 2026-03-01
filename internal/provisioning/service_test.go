package provisioning

import (
	"crypto/ed25519"
	"crypto/rand"
	"encoding/base64"
	"testing"
)

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

	manifest := ArtifactManifest{ImageDigest: "sha256:abc", DigestSHA256: "deadbeef", SBOMS3URI: "s3://sboms/example.spdx.json", VulnerabilityPassed: true}
	if err := VerifyArtifact(manifest, []byte("artifact-content")); err == nil {
		t.Fatal("expected digest mismatch error")
	}
}

func TestArtifactVerificationWithSignatureAndSBOM(t *testing.T) {
	t.Parallel()

	artifact := []byte("artifact-content")
	pub, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatalf("generate keypair: %v", err)
	}
	signature := ed25519.Sign(priv, artifact)

	manifest := ArtifactManifest{
		ImageDigest:         "sha256:abc",
		DigestSHA256:        hash(string(artifact)),
		SignaturePublicKey:  base64.StdEncoding.EncodeToString(pub),
		Signature:           base64.StdEncoding.EncodeToString(signature),
		SBOMS3URI:           "s3://sboms/example.spdx.json",
		VulnerabilityPassed: true,
	}
	if err := VerifyArtifact(manifest, artifact); err != nil {
		t.Fatalf("expected valid artifact verification, got %v", err)
	}
}

func TestRankServersDeterministicAndExplanationReplay(t *testing.T) {
	t.Parallel()

	svc := NewService()
	svc.RegisterRankerVersion(1, map[string]float64{
		"risk_penalty":      0.7,
		"reliability_score": 1.2,
		"cost_efficiency":   0.4,
	})
	if err := svc.SetActiveRankerVersion(1); err != nil {
		t.Fatalf("set active ranker: %v", err)
	}

	metrics := map[string]CandidateMetrics{
		"server_a": {RiskPenalty: 0.1, ReliabilityScore: 0.9, CostEfficiency: 0.5},
		"server_b": {RiskPenalty: 0.3, ReliabilityScore: 0.8, CostEfficiency: 0.8},
	}
	firstRanked, firstExplanation, err := svc.RankServers(metrics)
	if err != nil {
		t.Fatalf("rank servers first call: %v", err)
	}
	secondRanked, secondExplanation, err := svc.RankServers(metrics)
	if err != nil {
		t.Fatalf("rank servers second call: %v", err)
	}
	if len(firstRanked) != len(secondRanked) {
		t.Fatalf("ranked result size mismatch: %d vs %d", len(firstRanked), len(secondRanked))
	}
	for i := range firstRanked {
		if firstRanked[i] != secondRanked[i] {
			t.Fatalf("ranked output mismatch at index %d: %+v vs %+v", i, firstRanked[i], secondRanked[i])
		}
	}
	if firstExplanation != secondExplanation {
		t.Fatalf("expected explanation replay identity, got %s vs %s", firstExplanation, secondExplanation)
	}
}

func TestDriftWatchdogQuarantine(t *testing.T) {
	t.Parallel()

	if status := DriftWatchdog(true); status != "quarantine" {
		t.Fatalf("expected quarantine, got %s", status)
	}
}

func TestRankFormulaV1WeightsAndTiebreaker(t *testing.T) {
	t.Parallel()

	weights := DefaultRankerWeightsV1()
	if weights["capability_match"] != 0.30 || weights["reliability"] != 0.25 || weights["workspace_preference"] != 0.05 {
		t.Fatalf("unexpected default weights: %+v", weights)
	}

	ranked := RankServersByFormulaV1([]RankingInputV1{
		{
			ServerID:                  "server_b",
			CapabilityMatchScore:      0.9,
			ReliabilityScore:          0.9,
			EstimatedMonthlyCost:      10,
			BudgetRemaining:           100,
			P95LatencyMS:              500,
			ArtifactVerificationState: "verified",
			InAllowedServerIDs:        true,
		},
		{
			ServerID:                  "server_a",
			CapabilityMatchScore:      0.9,
			ReliabilityScore:          0.9,
			EstimatedMonthlyCost:      10,
			BudgetRemaining:           100,
			P95LatencyMS:              500,
			ArtifactVerificationState: "verified",
			InAllowedServerIDs:        true,
		},
	}, weights)

	if len(ranked) != 2 {
		t.Fatalf("unexpected ranked length: %d", len(ranked))
	}
	// Scores are equal within tie threshold; lexical server id must win.
	if ranked[0].ServerID != "server_a" {
		t.Fatalf("expected lexical tiebreak on equal scores, got %+v", ranked)
	}
}

func TestRankFactorsV1Mappings(t *testing.T) {
	t.Parallel()

	factors := RankFactorsV1(RankingInputV1{
		ServerID:                  "server_x",
		CapabilityMatchScore:      1.2,
		ReliabilityScore:          0.8,
		EstimatedMonthlyCost:      20,
		BudgetRemaining:           100,
		P95LatencyMS:              1000,
		ArtifactVerificationState: "unverified",
		PreviouslyDeclined:        true,
	})
	if factors.CapabilityMatch != 1.0 {
		t.Fatalf("expected capability score to clamp at 1.0, got %f", factors.CapabilityMatch)
	}
	if factors.Security != 0.5 {
		t.Fatalf("unexpected security score: %f", factors.Security)
	}
	if factors.WorkspacePreference != 0.0 {
		t.Fatalf("expected declined server preference to be 0.0, got %f", factors.WorkspacePreference)
	}
}
