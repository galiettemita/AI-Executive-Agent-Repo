package self_modification

import "testing"

func TestSelfModificationPolicyLifecycle(t *testing.T) {
	t.Parallel()

	s := NewService()
	_, err := s.UpsertPolicyStrict("ws_1", Policy{
		Enabled:         true,
		RequireApproval: true,
		MaxAllowedRisk:  "critical",
	})
	if err != nil {
		t.Fatalf("upsert policy: %v", err)
	}
	policy, ok := s.GetPolicy("ws_1")
	if !ok {
		t.Fatalf("expected policy lookup success")
	}
	if !policy.RequireApproval {
		t.Fatalf("expected require approval policy")
	}
}

func TestSelfModificationInvalidRiskRejected(t *testing.T) {
	t.Parallel()

	s := NewService()
	if _, err := s.UpsertPolicyStrict("ws_1", Policy{
		Enabled:         true,
		RequireApproval: true,
		MaxAllowedRisk:  "unknown",
	}); err == nil {
		t.Fatal("expected invalid risk error")
	}
}

func TestSelfModificationDecisionPaths(t *testing.T) {
	t.Parallel()

	s := NewService()
	if _, err := s.UpsertPolicyStrict("ws_allow", Policy{
		Enabled:         true,
		RequireApproval: false,
		MaxAllowedRisk:  "critical",
	}); err != nil {
		t.Fatalf("upsert allow policy: %v", err)
	}
	allow := s.EvaluateAction("ws_allow", ActionRequest{
		ActionKey:     "apply_patch",
		RequestedRisk: "elevated",
	})
	if allow.Decision != "allow" || allow.Reason != "ALLOW_WITH_AUDIT" || allow.AuditEvent != "BREVIO.self_modification.executed.v1" {
		t.Fatalf("unexpected allow decision: %+v", allow)
	}

	if _, err := s.UpsertPolicyStrict("ws_review", Policy{
		Enabled:         true,
		RequireApproval: true,
		MaxAllowedRisk:  "critical",
	}); err != nil {
		t.Fatalf("upsert review policy: %v", err)
	}
	review := s.EvaluateAction("ws_review", ActionRequest{
		ActionKey:     "refactor_router",
		RequestedRisk: "low",
	})
	if review.Decision != "require_approval" || review.Reason != "REQUIRE_APPROVAL" {
		t.Fatalf("unexpected review decision: %+v", review)
	}

	if _, err := s.UpsertPolicyStrict("ws_deny", Policy{
		Enabled:         false,
		RequireApproval: true,
		MaxAllowedRisk:  "low",
	}); err != nil {
		t.Fatalf("upsert deny policy: %v", err)
	}
	deny := s.EvaluateAction("ws_deny", ActionRequest{
		ActionKey:     "rewrite_core",
		RequestedRisk: "critical",
	})
	if deny.Decision != "deny" || deny.Reason != "SELF_MODIFICATION_DENIED" || deny.AuditEvent != "BREVIO.self_modification.denied.v1" {
		t.Fatalf("unexpected deny decision: %+v", deny)
	}
}
