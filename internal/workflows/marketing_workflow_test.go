package workflows

import (
	"slices"
	"testing"
)

func TestMarketingWorkflowHappyPath(t *testing.T) {
	t.Parallel()
	svc := NewService()
	result := svc.MarketingCampaignWorkflowV1(MarketingWorkflowInput{
		CampaignID:   "campaign-001",
		UserID:       "user-001",
		CampaignType: "email",
		ContactCount: 100,
	})

	if result.WorkflowID != "marketing-campaign-001" {
		t.Fatalf("unexpected workflow id: %s", result.WorkflowID)
	}
	if result.TerminalState != MarketingStateCompleted {
		t.Fatalf("expected COMPLETED, got %s", result.TerminalState)
	}
	wantStates := []MarketingCampaignState{
		MarketingStateInit, MarketingStateValidating, MarketingStateEnriching,
		MarketingStateGenerating, MarketingStateScheduling, MarketingStateSending,
		MarketingStateTracking, MarketingStateCompleted,
	}
	if !slices.Equal(result.States, wantStates) {
		t.Fatalf("unexpected states: %v", result.States)
	}
}

func TestMarketingWorkflowNoContacts(t *testing.T) {
	t.Parallel()
	svc := NewService()
	result := svc.MarketingCampaignWorkflowV1(MarketingWorkflowInput{
		CampaignID:   "campaign-002",
		ContactCount: 0,
	})

	if result.TerminalState != MarketingStateFailed {
		t.Fatalf("expected FAILED with no contacts, got %s", result.TerminalState)
	}
}

func TestMarketingWorkflowSendFailure(t *testing.T) {
	t.Parallel()
	svc := NewService()
	result := svc.MarketingCampaignWorkflowV1(MarketingWorkflowInput{
		CampaignID:       "campaign-003",
		ContactCount:     50,
		SendError:        true,
		SendFailureCount: 3,
	})

	if result.TerminalState != MarketingStateFailed {
		t.Fatalf("expected FAILED after 3 send failures, got %s", result.TerminalState)
	}
}

func TestMarketingWorkflowValidateError(t *testing.T) {
	t.Parallel()
	svc := NewService()
	result := svc.MarketingCampaignWorkflowV1(MarketingWorkflowInput{
		CampaignID:    "campaign-004",
		ContactCount:  10,
		ValidateError: true,
	})

	if result.TerminalState != MarketingStateFailed {
		t.Fatalf("expected FAILED on validation error, got %s", result.TerminalState)
	}
}

func TestMarketingWorkflowEnrichmentFallback(t *testing.T) {
	t.Parallel()
	svc := NewService()
	result := svc.MarketingCampaignWorkflowV1(MarketingWorkflowInput{
		CampaignID:      "campaign-005",
		ContactCount:    50,
		EnrichmentError: true,
	})

	if result.TerminalState != MarketingStateCompleted {
		t.Fatalf("expected COMPLETED with enrichment fallback, got %s", result.TerminalState)
	}
	if !slices.Contains(result.Fallbacks, "skip_enrichment") {
		t.Fatalf("expected skip_enrichment fallback: %v", result.Fallbacks)
	}
}

func TestMarketingWorkflowScheduleFallback(t *testing.T) {
	t.Parallel()
	svc := NewService()
	result := svc.MarketingCampaignWorkflowV1(MarketingWorkflowInput{
		CampaignID:    "campaign-006",
		ContactCount:  50,
		ScheduleError: true,
	})

	if result.TerminalState != MarketingStateCompleted {
		t.Fatalf("expected COMPLETED with schedule fallback, got %s", result.TerminalState)
	}
	if !slices.Contains(result.Fallbacks, "immediate_send") {
		t.Fatalf("expected immediate_send fallback: %v", result.Fallbacks)
	}
}

func TestMarketingWorkflowSendErrorBelowThreshold(t *testing.T) {
	t.Parallel()
	svc := NewService()
	result := svc.MarketingCampaignWorkflowV1(MarketingWorkflowInput{
		CampaignID:       "campaign-007",
		ContactCount:     50,
		SendError:        true,
		SendFailureCount: 2,
	})

	if result.TerminalState != MarketingStateCompleted {
		t.Fatalf("expected COMPLETED with send errors below threshold, got %s", result.TerminalState)
	}
}

func TestMarketingWorkflowIDTrimming(t *testing.T) {
	t.Parallel()
	id := MarketingWorkflowID("  camp-padded  ")
	if id != "marketing-camp-padded" {
		t.Fatalf("expected trimmed ID, got %s", id)
	}
}
