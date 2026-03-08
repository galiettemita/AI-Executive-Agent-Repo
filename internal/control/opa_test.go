package control

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

func TestOPAEvaluator_EvaluatePolicy_AllGatesPass(t *testing.T) {
	t.Parallel()

	evaluator := NewOPAEvaluator(nil)

	decision, err := evaluator.EvaluatePolicy(context.Background(), PolicyInput{
		AutonomyLevel:          "A3",
		ToolRiskLevel:          "LOW",
		IsWrite:                false,
		RateLimited:            false,
		BudgetExhausted:        false,
		FirewallAllowed:        true,
		SemanticVerifierPassed: true,
		BlockedTool:            false,
		WorkspacePlan:          "pro",
		Domain:                 "calendar",
		ToolKey:                "calendar_read",
		UserRole:               "admin",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !decision.Allowed {
		t.Errorf("expected allowed=true, got reason=%s", decision.Reason)
	}
	if decision.Reason != "all_gates_passed" {
		t.Errorf("got reason=%q want=%q", decision.Reason, "all_gates_passed")
	}
}

func TestOPAEvaluator_ContentFirewallBlock(t *testing.T) {
	t.Parallel()

	evaluator := NewOPAEvaluator(nil)

	decision, err := evaluator.EvaluatePolicy(context.Background(), PolicyInput{
		FirewallAllowed:        false,
		SemanticVerifierPassed: true,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if decision.Allowed {
		t.Fatal("expected denied for firewall block")
	}
	if decision.Reason != "content_firewall_blocked" {
		t.Errorf("got reason=%q want=%q", decision.Reason, "content_firewall_blocked")
	}
	if !decision.ReceiptRequired {
		t.Error("expected receipt_required=true for firewall block")
	}
}

func TestOPAEvaluator_SemanticVerifierFailed(t *testing.T) {
	t.Parallel()

	evaluator := NewOPAEvaluator(nil)

	decision, err := evaluator.EvaluatePolicy(context.Background(), PolicyInput{
		FirewallAllowed:        true,
		SemanticVerifierPassed: false,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if decision.Allowed {
		t.Fatal("expected denied for semantic verifier failure")
	}
	if decision.Reason != "semantic_verifier_failed" {
		t.Errorf("got reason=%q want=%q", decision.Reason, "semantic_verifier_failed")
	}
}

func TestOPAEvaluator_RateLimitGate(t *testing.T) {
	t.Parallel()

	evaluator := NewOPAEvaluator(nil)

	decision, err := evaluator.EvaluatePolicy(context.Background(), PolicyInput{
		FirewallAllowed:        true,
		SemanticVerifierPassed: true,
		RateLimited:            true,
		ToolKey:                "email_send",
		Domain:                 "email",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if decision.Allowed {
		t.Fatal("expected denied for rate limit")
	}
	if decision.Reason != "rate_limit_exceeded" {
		t.Errorf("got reason=%q want=%q", decision.Reason, "rate_limit_exceeded")
	}
	if decision.Constraints["tool_key"] != "email_send" {
		t.Errorf("expected tool_key constraint")
	}
}

func TestOPAEvaluator_BudgetGate(t *testing.T) {
	t.Parallel()

	evaluator := NewOPAEvaluator(nil)

	decision, err := evaluator.EvaluatePolicy(context.Background(), PolicyInput{
		FirewallAllowed:        true,
		SemanticVerifierPassed: true,
		BudgetExhausted:        true,
		WorkspacePlan:          "free",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if decision.Allowed {
		t.Fatal("expected denied for budget exhaustion")
	}
	if decision.Reason != "budget_exhausted" {
		t.Errorf("got reason=%q want=%q", decision.Reason, "budget_exhausted")
	}
}

func TestOPAEvaluator_ToolWriteGate_BlockedTool(t *testing.T) {
	t.Parallel()

	evaluator := NewOPAEvaluator(nil)

	decision, err := evaluator.EvaluatePolicy(context.Background(), PolicyInput{
		FirewallAllowed:        true,
		SemanticVerifierPassed: true,
		IsWrite:                true,
		BlockedTool:            true,
		ToolKey:                "dangerous_tool",
		AutonomyLevel:          "A4",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if decision.Allowed {
		t.Fatal("expected denied for blocked tool")
	}
	if decision.Reason != "tool_blocked" {
		t.Errorf("got reason=%q want=%q", decision.Reason, "tool_blocked")
	}
}

func TestOPAEvaluator_ToolWriteGate_FinancialRestriction(t *testing.T) {
	t.Parallel()

	evaluator := NewOPAEvaluator(nil)

	decision, err := evaluator.EvaluatePolicy(context.Background(), PolicyInput{
		FirewallAllowed:        true,
		SemanticVerifierPassed: true,
		IsWrite:                true,
		Domain:                 "financial",
		UserRole:               "member",
		AutonomyLevel:          "A3",
		ToolRiskLevel:          "LOW",
		WorkspacePlan:          "pro",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if decision.Allowed {
		t.Fatal("expected denied for member financial write")
	}
	if !decision.RequiresApproval {
		t.Error("expected requires_approval=true")
	}
	if decision.Reason != "financial_write_restricted_role" {
		t.Errorf("got reason=%q want=%q", decision.Reason, "financial_write_restricted_role")
	}
}

func TestOPAEvaluator_ToolWriteGate_FinancialAllowedForAdmin(t *testing.T) {
	t.Parallel()

	evaluator := NewOPAEvaluator(nil)

	decision, err := evaluator.EvaluatePolicy(context.Background(), PolicyInput{
		FirewallAllowed:        true,
		SemanticVerifierPassed: true,
		IsWrite:                true,
		Domain:                 "financial",
		UserRole:               "admin",
		AutonomyLevel:          "A3",
		ToolRiskLevel:          "LOW",
		WorkspacePlan:          "pro",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !decision.Allowed {
		t.Errorf("expected allowed=true for admin financial write, got reason=%s", decision.Reason)
	}
}

func TestOPAEvaluator_FreePlanWriteRestriction(t *testing.T) {
	t.Parallel()

	evaluator := NewOPAEvaluator(nil)

	decision, err := evaluator.EvaluatePolicy(context.Background(), PolicyInput{
		FirewallAllowed:        true,
		SemanticVerifierPassed: true,
		IsWrite:                true,
		Domain:                 "crm",
		UserRole:               "owner",
		AutonomyLevel:          "A3",
		ToolRiskLevel:          "LOW",
		WorkspacePlan:          "free",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if decision.Allowed {
		t.Fatal("expected denied for free plan CRM write")
	}
	if decision.Reason != "free_plan_write_restricted" {
		t.Errorf("got reason=%q want=%q", decision.Reason, "free_plan_write_restricted")
	}
}

func TestOPAEvaluator_AutonomyGates(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name             string
		autonomy         string
		risk             string
		wantAllowed      bool
		wantApproval     bool
		wantReasonPrefix string
	}{
		{
			name: "A0 write denied", autonomy: "A0", risk: "LOW",
			wantAllowed: false, wantReasonPrefix: "autonomy_a0",
		},
		{
			name: "A1 requires approval", autonomy: "A1", risk: "LOW",
			wantAllowed: false, wantApproval: true, wantReasonPrefix: "autonomy_a1",
		},
		{
			name: "A2 low risk allowed", autonomy: "A2", risk: "LOW",
			wantAllowed: true, wantReasonPrefix: "autonomy_a2",
		},
		{
			name: "A2 critical risk requires approval", autonomy: "A2", risk: "CRITICAL",
			wantAllowed: false, wantApproval: true, wantReasonPrefix: "autonomy_a2",
		},
		{
			name: "A2 elevated risk requires approval", autonomy: "A2", risk: "ELEVATED",
			wantAllowed: false, wantApproval: true, wantReasonPrefix: "autonomy_a2",
		},
		{
			name: "A3 low risk auto commit", autonomy: "A3", risk: "LOW",
			wantAllowed: true, wantReasonPrefix: "autonomy_a3",
		},
		{
			name: "A3 critical requires approval", autonomy: "A3", risk: "CRITICAL",
			wantAllowed: false, wantApproval: true, wantReasonPrefix: "autonomy_a3",
		},
		{
			name: "A4 full auto", autonomy: "A4", risk: "CRITICAL",
			wantAllowed: true, wantReasonPrefix: "autonomy_a4",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			evaluator := NewOPAEvaluator(nil)
			decision, err := evaluator.EvaluatePolicy(context.Background(), PolicyInput{
				FirewallAllowed:        true,
				SemanticVerifierPassed: true,
				IsWrite:                true,
				AutonomyLevel:          tt.autonomy,
				ToolRiskLevel:          tt.risk,
				WorkspacePlan:          "pro",
				Domain:                 "calendar",
				UserRole:               "owner",
			})
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if decision.Allowed != tt.wantAllowed {
				t.Errorf("allowed=%v want=%v (reason=%s)", decision.Allowed, tt.wantAllowed, decision.Reason)
			}
			if decision.RequiresApproval != tt.wantApproval {
				t.Errorf("requires_approval=%v want=%v", decision.RequiresApproval, tt.wantApproval)
			}
			if !containsSubstring(decision.Reason, tt.wantReasonPrefix) {
				t.Errorf("reason=%q does not contain prefix=%q", decision.Reason, tt.wantReasonPrefix)
			}
		})
	}
}

func TestOPAEvaluator_EvaluateGateWithOPA(t *testing.T) {
	t.Parallel()

	svc := NewService("test-secret")
	evaluator := NewOPAEvaluator(svc)

	output := evaluator.EvaluateGateWithOPA(context.Background(), PolicyInput{
		FirewallAllowed:        true,
		SemanticVerifierPassed: true,
		IsWrite:                false,
		AutonomyLevel:          "A2",
		WorkspacePlan:          "pro",
		Domain:                 "calendar",
		UserRole:               "admin",
	})
	if output.Decision != "allow" {
		t.Errorf("got decision=%q want=%q reason=%q", output.Decision, "allow", output.ReasonCode)
	}
}

func TestOPAEvaluator_EvaluateGateWithOPA_Deny(t *testing.T) {
	t.Parallel()

	evaluator := NewOPAEvaluator(nil)

	output := evaluator.EvaluateGateWithOPA(context.Background(), PolicyInput{
		FirewallAllowed: false,
	})
	if output.Decision != "deny" {
		t.Errorf("got decision=%q want=%q", output.Decision, "deny")
	}
}

func TestOPAEvaluator_EvaluateGateWithOPA_RequireApproval(t *testing.T) {
	t.Parallel()

	evaluator := NewOPAEvaluator(nil)

	output := evaluator.EvaluateGateWithOPA(context.Background(), PolicyInput{
		FirewallAllowed:        true,
		SemanticVerifierPassed: true,
		IsWrite:                true,
		AutonomyLevel:          "A1",
		WorkspacePlan:          "pro",
		Domain:                 "calendar",
		UserRole:               "owner",
	})
	if output.Decision != "require_approval" {
		t.Errorf("got decision=%q want=%q reason=%q", output.Decision, "require_approval", output.ReasonCode)
	}
}

func TestOPAEvaluator_LoadPolicies(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()

	// Write a sample .rego file
	regoContent := `package brevio.control.test_policy

default allow = false

allow {
    input.autonomy_level == "A4"
}
`
	if err := os.WriteFile(filepath.Join(dir, "test_policy.rego"), []byte(regoContent), 0o644); err != nil {
		t.Fatal(err)
	}

	// Write a non-rego file that should be ignored
	if err := os.WriteFile(filepath.Join(dir, "notes.txt"), []byte("ignore me"), 0o644); err != nil {
		t.Fatal(err)
	}

	evaluator := NewOPAEvaluator(nil)
	if err := evaluator.LoadPolicies(dir); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if evaluator.PolicyCount() != 1 {
		t.Errorf("got %d policies want 1", evaluator.PolicyCount())
	}
}

func TestOPAEvaluator_LoadPolicies_MissingDir(t *testing.T) {
	t.Parallel()

	evaluator := NewOPAEvaluator(nil)
	err := evaluator.LoadPolicies("/nonexistent/path")
	if err == nil {
		t.Fatal("expected error for missing directory")
	}
}

func TestOPAEvaluator_FallbackGate(t *testing.T) {
	t.Parallel()

	svc := NewService("test-secret")
	evaluator := NewOPAEvaluator(svc)

	// Test the fallback path directly
	output := evaluator.fallbackGate(PolicyInput{
		AutonomyLevel:          "A3",
		ToolRiskLevel:          "LOW",
		IsWrite:                true,
		FirewallAllowed:        true,
		SemanticVerifierPassed: true,
	})
	if output.Decision != "allow" {
		t.Errorf("got decision=%q want=%q reason=%q", output.Decision, "allow", output.ReasonCode)
	}
}

func TestOPAEvaluator_FallbackGate_NilService(t *testing.T) {
	t.Parallel()

	evaluator := NewOPAEvaluator(nil)
	output := evaluator.fallbackGate(PolicyInput{})
	if output.Decision != "deny" {
		t.Errorf("got decision=%q want=%q", output.Decision, "deny")
	}
	if output.ReasonCode != "NO_SERVICE_FALLBACK" {
		t.Errorf("got reason=%q want=%q", output.ReasonCode, "NO_SERVICE_FALLBACK")
	}
}

func TestExtractPackageName(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		source string
		want   string
	}{
		{
			name:   "standard package",
			source: "package brevio.control.budget\n\ndefault allow = false",
			want:   "brevio.control.budget",
		},
		{
			name:   "with comments",
			source: "# comment\npackage brevio.test\n\nallow = true",
			want:   "brevio.test",
		},
		{
			name:   "no package",
			source: "allow = true",
			want:   "",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := extractPackageName(tt.source)
			if got != tt.want {
				t.Errorf("extractPackageName()=%q want=%q", got, tt.want)
			}
		})
	}
}

func containsSubstring(s, sub string) bool {
	return len(sub) == 0 || len(s) >= len(sub) && containsCheck(s, sub)
}

func containsCheck(s, sub string) bool {
	for i := 0; i <= len(s)-len(sub); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}
