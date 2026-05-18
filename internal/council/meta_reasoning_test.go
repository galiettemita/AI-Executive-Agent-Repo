package council

import (
	"testing"
)

func TestSetConvenePolicy(t *testing.T) {
	svc := NewCouncilMetaReasoningService()

	err := svc.SetConvenePolicy("ws1", CouncilConvenePolicy{
		MinComplexity:          0.7,
		MinStakeholders:        3,
		RequiresDomainExpertise: true,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Invalid complexity.
	err = svc.SetConvenePolicy("ws1", CouncilConvenePolicy{MinComplexity: 1.5})
	if err == nil {
		t.Fatal("expected error for invalid complexity")
	}

	// Empty workspace.
	err = svc.SetConvenePolicy("", CouncilConvenePolicy{MinComplexity: 0.5})
	if err == nil {
		t.Fatal("expected error for empty workspace")
	}
}

func TestGetConvenePolicy(t *testing.T) {
	svc := NewCouncilMetaReasoningService()

	// Default policy.
	policy := svc.GetConvenePolicy("ws1")
	if policy.MinComplexity != 0.6 {
		t.Fatalf("expected default complexity 0.6, got %f", policy.MinComplexity)
	}

	// Custom policy.
	_ = svc.SetConvenePolicy("ws1", CouncilConvenePolicy{MinComplexity: 0.9, MinStakeholders: 5})
	policy = svc.GetConvenePolicy("ws1")
	if policy.MinComplexity != 0.9 {
		t.Fatalf("expected 0.9, got %f", policy.MinComplexity)
	}
}

func TestShouldConveneCouncil_ComplexityMet(t *testing.T) {
	svc := NewCouncilMetaReasoningService()

	decision := svc.ShouldConveneCouncil("complex request", 0.8, "finance", 1)
	if !decision.ShouldConvene {
		t.Fatal("expected convene for high complexity")
	}
}

func TestShouldConveneCouncil_StakeholdersMet(t *testing.T) {
	svc := NewCouncilMetaReasoningService()

	decision := svc.ShouldConveneCouncil("simple request", 0.3, "", 3)
	if !decision.ShouldConvene {
		t.Fatal("expected convene for multiple stakeholders")
	}
}

func TestShouldConveneCouncil_NotMet(t *testing.T) {
	svc := NewCouncilMetaReasoningService()

	decision := svc.ShouldConveneCouncil("simple request", 0.2, "", 0)
	if decision.ShouldConvene {
		t.Fatal("expected no convene for low complexity and few stakeholders")
	}
	if decision.Reason != "criteria not met" {
		t.Fatalf("expected 'criteria not met', got %q", decision.Reason)
	}
}

func TestShouldConveneCouncilForWorkspace(t *testing.T) {
	svc := NewCouncilMetaReasoningService()

	_ = svc.SetConvenePolicy("ws1", CouncilConvenePolicy{
		MinComplexity:   0.3,
		MinStakeholders: 1,
	})

	decision := svc.ShouldConveneCouncilForWorkspace("ws1", "request", 0.4, "", 0)
	if !decision.ShouldConvene {
		t.Fatal("expected convene with workspace-specific low threshold")
	}

	// Different workspace uses default.
	decision = svc.ShouldConveneCouncilForWorkspace("ws2", "request", 0.4, "", 0)
	if decision.ShouldConvene {
		t.Fatal("expected no convene with default threshold for ws2")
	}
}
