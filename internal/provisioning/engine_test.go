package provisioning

import (
	"testing"
	"time"
)

func TestResolvePackage(t *testing.T) {
	t.Parallel()

	engine := NewProvisioningEngine()

	packages := []string{"package_a", "package_b", "package_c", "package_d", "package_e", "package_f"}
	for _, id := range packages {
		pkg, err := engine.ResolvePackage(id)
		if err != nil {
			t.Fatalf("resolve %s: %v", id, err)
		}
		if pkg.ID != id {
			t.Fatalf("expected id=%s got=%s", id, pkg.ID)
		}
		if len(pkg.RequiredConnectors) == 0 {
			t.Fatalf("package %s has no connectors", id)
		}
		if len(pkg.OAuthScopes) == 0 {
			t.Fatalf("package %s has no scopes", id)
		}
	}

	_, err := engine.ResolvePackage("nonexistent")
	if err == nil {
		t.Fatal("expected error for unknown package")
	}
}

func TestEvaluateEligibilityPlanTier(t *testing.T) {
	t.Parallel()

	engine := NewProvisioningEngine()

	// Package D requires enterprise plan.
	result, err := engine.EvaluateEligibility("ws_1", "package_d", 0.9, PlanPro)
	if err != nil {
		t.Fatalf("evaluate: %v", err)
	}
	if result.Eligible {
		t.Fatal("pro plan should not be eligible for package_d (requires enterprise)")
	}

	result, err = engine.EvaluateEligibility("ws_1", "package_d", 0.9, PlanEnterprise)
	if err != nil {
		t.Fatalf("evaluate: %v", err)
	}
	if !result.Eligible {
		t.Fatalf("enterprise plan should be eligible for package_d, reason: %s", result.Reason)
	}
}

func TestEvaluateEligibilityTrustScore(t *testing.T) {
	t.Parallel()

	engine := NewProvisioningEngine()

	// Package D requires trust >= 0.8.
	result, err := engine.EvaluateEligibility("ws_1", "package_d", 0.5, PlanEnterprise)
	if err != nil {
		t.Fatalf("evaluate: %v", err)
	}
	if result.Eligible {
		t.Fatal("low trust score should not be eligible for package_d")
	}

	result, err = engine.EvaluateEligibility("ws_1", "package_d", 0.85, PlanEnterprise)
	if err != nil {
		t.Fatalf("evaluate: %v", err)
	}
	if !result.Eligible {
		t.Fatalf("high trust score should be eligible: %s", result.Reason)
	}
}

func TestEvaluateEligibilityUnknownPlan(t *testing.T) {
	t.Parallel()

	engine := NewProvisioningEngine()
	result, err := engine.EvaluateEligibility("ws_1", "package_a", 0.5, "bogus_plan")
	if err != nil {
		t.Fatalf("evaluate: %v", err)
	}
	if result.Eligible {
		t.Fatal("unknown plan should not be eligible")
	}
}

func TestEvaluateEligibilityUnknownPackage(t *testing.T) {
	t.Parallel()

	engine := NewProvisioningEngine()
	_, err := engine.EvaluateEligibility("ws_1", "nonexistent", 0.5, PlanPro)
	if err == nil {
		t.Fatal("expected error for unknown package")
	}
}

func TestStartAndCompleteProvisioning(t *testing.T) {
	t.Parallel()

	engine := NewProvisioningEngine()
	now := time.Date(2026, 3, 3, 12, 0, 0, 0, time.UTC)
	engine.now = func() time.Time { return now }

	req, err := engine.StartProvisioning("ws_1", "package_a")
	if err != nil {
		t.Fatalf("start: %v", err)
	}
	if req.ID == "" {
		t.Fatal("expected request ID")
	}
	if req.Status != ProvisionStatusPending {
		t.Fatalf("expected status=pending got=%s", req.Status)
	}
	if req.WorkspaceID != "ws_1" {
		t.Fatalf("expected workspace=ws_1 got=%s", req.WorkspaceID)
	}

	now = now.Add(time.Minute)
	if err := engine.CompleteProvisioning(req.ID); err != nil {
		t.Fatalf("complete: %v", err)
	}

	requests := engine.ListRequests("ws_1")
	if len(requests) != 1 {
		t.Fatalf("expected 1 request, got=%d", len(requests))
	}
	if requests[0].Status != ProvisionStatusCompleted {
		t.Fatalf("expected status=completed got=%s", requests[0].Status)
	}
}

func TestStartProvisioningDuplicateBlocked(t *testing.T) {
	t.Parallel()

	engine := NewProvisioningEngine()

	_, err := engine.StartProvisioning("ws_1", "package_b")
	if err != nil {
		t.Fatalf("first start: %v", err)
	}

	_, err = engine.StartProvisioning("ws_1", "package_b")
	if err == nil {
		t.Fatal("expected error for duplicate provisioning")
	}
}

func TestStartProvisioningAfterCompletionAllowed(t *testing.T) {
	t.Parallel()

	engine := NewProvisioningEngine()

	req, _ := engine.StartProvisioning("ws_1", "package_c")
	engine.CompleteProvisioning(req.ID)

	// Should be able to start a new request after completion.
	req2, err := engine.StartProvisioning("ws_1", "package_c")
	if err != nil {
		t.Fatalf("re-provision after completion: %v", err)
	}
	if req2.ID == req.ID {
		t.Fatal("new request should have different ID")
	}
}

func TestFailProvisioning(t *testing.T) {
	t.Parallel()

	engine := NewProvisioningEngine()

	req, _ := engine.StartProvisioning("ws_1", "package_e")
	if err := engine.FailProvisioning(req.ID, "connector timeout"); err != nil {
		t.Fatalf("fail: %v", err)
	}

	requests := engine.ListRequests("ws_1")
	if len(requests) != 1 {
		t.Fatalf("expected 1 request, got=%d", len(requests))
	}
	if requests[0].Status != ProvisionStatusFailed {
		t.Fatalf("expected status=failed got=%s", requests[0].Status)
	}
	if requests[0].FailReason != "connector timeout" {
		t.Fatalf("expected reason='connector timeout' got=%s", requests[0].FailReason)
	}
}

func TestCompleteNonexistentRequestFails(t *testing.T) {
	t.Parallel()

	engine := NewProvisioningEngine()
	if err := engine.CompleteProvisioning("nonexistent"); err == nil {
		t.Fatal("expected error for nonexistent request")
	}
}

func TestFailNonexistentRequestFails(t *testing.T) {
	t.Parallel()

	engine := NewProvisioningEngine()
	if err := engine.FailProvisioning("nonexistent", "reason"); err == nil {
		t.Fatal("expected error for nonexistent request")
	}
}

func TestStartProvisioningUnknownPackageFails(t *testing.T) {
	t.Parallel()

	engine := NewProvisioningEngine()
	_, err := engine.StartProvisioning("ws_1", "nonexistent")
	if err == nil {
		t.Fatal("expected error for unknown package")
	}
}

func TestStartProvisioningEmptyWorkspaceFails(t *testing.T) {
	t.Parallel()

	engine := NewProvisioningEngine()
	_, err := engine.StartProvisioning("", "package_a")
	if err == nil {
		t.Fatal("expected error for empty workspace")
	}
}

func TestListRequestsEmpty(t *testing.T) {
	t.Parallel()

	engine := NewProvisioningEngine()
	requests := engine.ListRequests("ws_nonexistent")
	if len(requests) != 0 {
		t.Fatalf("expected 0 requests, got=%d", len(requests))
	}
}

func TestPackageRiskLevels(t *testing.T) {
	t.Parallel()

	engine := NewProvisioningEngine()

	// Package A should be low risk.
	pkgA, _ := engine.ResolvePackage("package_a")
	if pkgA.RiskLevel != RiskLow {
		t.Fatalf("expected package_a risk=low got=%s", pkgA.RiskLevel)
	}

	// Package D should be elevated risk.
	pkgD, _ := engine.ResolvePackage("package_d")
	if pkgD.RiskLevel != RiskElevated {
		t.Fatalf("expected package_d risk=elevated got=%s", pkgD.RiskLevel)
	}
}
