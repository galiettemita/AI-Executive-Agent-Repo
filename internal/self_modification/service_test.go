package self_modification

import "testing"

func TestSelfModificationPolicyLifecycle(t *testing.T) {
	s := NewService()
	s.UpsertPolicy("ws_1", Policy{
		Enabled:         true,
		RequireApproval: true,
		MaxAllowedRisk:  "critical",
	})
	policy, ok := s.GetPolicy("ws_1")
	if !ok {
		t.Fatalf("expected policy lookup success")
	}
	if !policy.RequireApproval {
		t.Fatalf("expected require approval policy")
	}
}
