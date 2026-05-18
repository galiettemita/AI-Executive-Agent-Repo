package workflows

import "strings"

type AgentOrchestrationState string

const (
	AgentStateInit       AgentOrchestrationState = "INIT"
	AgentStatePlanning   AgentOrchestrationState = "PLANNING"
	AgentStateDelegating AgentOrchestrationState = "DELEGATING"
	AgentStateExecuting  AgentOrchestrationState = "EXECUTING"
	AgentStateEvaluating AgentOrchestrationState = "EVALUATING"
	AgentStateMerging    AgentOrchestrationState = "MERGING"
	AgentStateCompleted  AgentOrchestrationState = "COMPLETED"
	AgentStateFailed     AgentOrchestrationState = "FAILED"
	AgentStateTimedOut   AgentOrchestrationState = "TIMED_OUT"
)

type AgentWorkflowInput struct {
	ExecutionID      string
	AgentID          string
	UserID           string
	IsSubExecution   bool
	PlanError        bool
	DelegationError  bool
	ExecuteError     bool
	EvaluateError    bool
	QualityScore     float64
	QualityThreshold float64
	IterationCount   int
	MaxIterations    int
	TimeoutExceeded  bool
	WorkerCount      int
}

type AgentWorkflowResult struct {
	WorkflowID    string
	States        []AgentOrchestrationState
	TerminalState AgentOrchestrationState
	Fallbacks     []string
	Iterations    int
}

func AgentWorkflowID(executionID string) string {
	return "agent-" + strings.TrimSpace(executionID)
}

func (s *Service) AgentOrchestrationWorkflowV1(input AgentWorkflowInput) AgentWorkflowResult {
	workflowID := AgentWorkflowID(input.ExecutionID)
	result := AgentWorkflowResult{
		WorkflowID: workflowID,
		States:     []AgentOrchestrationState{AgentStateInit},
		Fallbacks:  []string{},
		Iterations: input.IterationCount,
	}

	if input.TimeoutExceeded {
		result.States = append(result.States, AgentStateTimedOut)
		result.TerminalState = AgentStateTimedOut
		return result
	}

	result.States = append(result.States, AgentStatePlanning)
	if input.PlanError {
		result.Fallbacks = append(result.Fallbacks, "single_step_plan")
	}

	if input.WorkerCount > 1 {
		result.States = append(result.States, AgentStateDelegating)
		if input.DelegationError {
			result.Fallbacks = append(result.Fallbacks, "sequential_execution")
		}
	}

	result.States = append(result.States, AgentStateExecuting)
	if input.ExecuteError {
		result.States = append(result.States, AgentStateFailed)
		result.TerminalState = AgentStateFailed
		return result
	}

	result.States = append(result.States, AgentStateEvaluating)
	if input.QualityThreshold > 0 && input.QualityScore < input.QualityThreshold {
		if input.IterationCount < input.MaxIterations {
			result.Fallbacks = append(result.Fallbacks, "retry_with_feedback")
			result.Iterations++
		} else {
			result.Fallbacks = append(result.Fallbacks, "accept_best_effort")
		}
	}

	result.States = append(result.States, AgentStateMerging)
	result.States = append(result.States, AgentStateCompleted)
	result.TerminalState = AgentStateCompleted
	return result
}
