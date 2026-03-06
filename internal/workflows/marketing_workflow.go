package workflows

import "strings"

type MarketingCampaignState string

const (
	MarketingStateInit       MarketingCampaignState = "INIT"
	MarketingStateValidating MarketingCampaignState = "VALIDATING"
	MarketingStateEnriching  MarketingCampaignState = "ENRICHING"
	MarketingStateGenerating MarketingCampaignState = "GENERATING"
	MarketingStateScheduling MarketingCampaignState = "SCHEDULING"
	MarketingStateSending    MarketingCampaignState = "SENDING"
	MarketingStateTracking   MarketingCampaignState = "TRACKING"
	MarketingStateCompleted  MarketingCampaignState = "COMPLETED"
	MarketingStateFailed     MarketingCampaignState = "FAILED"
)

type MarketingWorkflowInput struct {
	CampaignID       string
	UserID           string
	CampaignType     string
	ContactCount     int
	ValidateError    bool
	EnrichmentError  bool
	GenerateError    bool
	ScheduleError    bool
	SendError        bool
	SendFailureCount int
	ABTestEnabled    bool
}

type MarketingWorkflowResult struct {
	WorkflowID    string
	States        []MarketingCampaignState
	TerminalState MarketingCampaignState
	Fallbacks     []string
}

func MarketingWorkflowID(campaignID string) string {
	return "marketing-" + strings.TrimSpace(campaignID)
}

func (s *Service) MarketingCampaignWorkflowV1(input MarketingWorkflowInput) MarketingWorkflowResult {
	workflowID := MarketingWorkflowID(input.CampaignID)
	result := MarketingWorkflowResult{
		WorkflowID: workflowID,
		States:     []MarketingCampaignState{MarketingStateInit},
		Fallbacks:  []string{},
	}

	result.States = append(result.States, MarketingStateValidating)
	if input.ValidateError || input.ContactCount == 0 {
		result.States = append(result.States, MarketingStateFailed)
		result.TerminalState = MarketingStateFailed
		return result
	}

	result.States = append(result.States, MarketingStateEnriching)
	if input.EnrichmentError {
		result.Fallbacks = append(result.Fallbacks, "skip_enrichment")
	}

	result.States = append(result.States, MarketingStateGenerating)
	if input.GenerateError {
		result.Fallbacks = append(result.Fallbacks, "template_fallback")
	}

	result.States = append(result.States, MarketingStateScheduling)
	if input.ScheduleError {
		result.Fallbacks = append(result.Fallbacks, "immediate_send")
	}

	result.States = append(result.States, MarketingStateSending)
	if input.SendError && input.SendFailureCount >= 3 {
		result.States = append(result.States, MarketingStateFailed)
		result.TerminalState = MarketingStateFailed
		return result
	}

	result.States = append(result.States, MarketingStateTracking)
	result.States = append(result.States, MarketingStateCompleted)
	result.TerminalState = MarketingStateCompleted
	return result
}
