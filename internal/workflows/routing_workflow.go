package workflows

import "strings"

type RoutingDecisionState string

const (
	RoutingStateInit        RoutingDecisionState = "INIT"
	RoutingStateClassifying RoutingDecisionState = "CLASSIFYING"
	RoutingStateSelecting   RoutingDecisionState = "SELECTING"
	RoutingStateValidating  RoutingDecisionState = "VALIDATING"
	RoutingStateCompleted   RoutingDecisionState = "COMPLETED"
	RoutingStateFallback    RoutingDecisionState = "FALLBACK"
	RoutingStateFailed      RoutingDecisionState = "FAILED"
)

type RoutingWorkflowInput struct {
	RequestID       string
	UserID          string
	SkillID         string
	ClassifyError   bool
	ComplexityScore float64
	SelectError     bool
	SelectedModel   string
	ModelHealthy    bool
	BudgetExceeded  bool
	FallbackModel   string
}

type RoutingWorkflowResult struct {
	WorkflowID    string
	States        []RoutingDecisionState
	TerminalState RoutingDecisionState
	SelectedModel string
	Fallbacks     []string
}

func RoutingWorkflowID(requestID string) string {
	return "routing-" + strings.TrimSpace(requestID)
}

func (s *Service) RoutingDecisionWorkflowV1(input RoutingWorkflowInput) RoutingWorkflowResult {
	workflowID := RoutingWorkflowID(input.RequestID)
	result := RoutingWorkflowResult{
		WorkflowID: workflowID,
		States:     []RoutingDecisionState{RoutingStateInit},
		Fallbacks:  []string{},
	}

	result.States = append(result.States, RoutingStateClassifying)
	if input.ClassifyError {
		result.Fallbacks = append(result.Fallbacks, "default_complexity")
	}

	result.States = append(result.States, RoutingStateSelecting)
	if input.SelectError {
		if input.FallbackModel != "" {
			result.Fallbacks = append(result.Fallbacks, "fallback_model")
			result.SelectedModel = input.FallbackModel
		} else {
			result.States = append(result.States, RoutingStateFailed)
			result.TerminalState = RoutingStateFailed
			return result
		}
	} else {
		result.SelectedModel = input.SelectedModel
	}

	result.States = append(result.States, RoutingStateValidating)
	if input.BudgetExceeded {
		result.Fallbacks = append(result.Fallbacks, "budget_downgrade")
	}
	if !input.ModelHealthy {
		if input.FallbackModel != "" {
			result.States = append(result.States, RoutingStateFallback)
			result.SelectedModel = input.FallbackModel
			result.Fallbacks = append(result.Fallbacks, "health_fallback")
		} else {
			result.States = append(result.States, RoutingStateFailed)
			result.TerminalState = RoutingStateFailed
			return result
		}
	}

	result.States = append(result.States, RoutingStateCompleted)
	result.TerminalState = RoutingStateCompleted
	return result
}
