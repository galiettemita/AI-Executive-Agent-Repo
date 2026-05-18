package workflows

import "strings"

type CronExecutionState string

const (
	CronStateInit      CronExecutionState = "INIT"
	CronStateLoading   CronExecutionState = "LOADING"
	CronStateExecuting CronExecutionState = "EXECUTING"
	CronStateWebhook   CronExecutionState = "WEBHOOK"
	CronStateNotifying CronExecutionState = "NOTIFYING"
	CronStateCompleted CronExecutionState = "COMPLETED"
	CronStateFailed    CronExecutionState = "FAILED"
	CronStateSkipped   CronExecutionState = "SKIPPED"
	CronStateTimedOut  CronExecutionState = "TIMED_OUT"
)

type CronWorkflowInput struct {
	ExecutionID     string
	JobID           string
	UserID          string
	ActionType      string
	LoadError       bool
	ExecuteError    bool
	RetryCount      int
	MaxRetries      int
	TimeoutExceeded bool
	WebhookEnabled  bool
	WebhookError    bool
	NotifyOnFailure bool
	JobExpired      bool
}

type CronWorkflowResult struct {
	WorkflowID    string
	States        []CronExecutionState
	TerminalState CronExecutionState
	Fallbacks     []string
	RetryCount    int
}

func CronWorkflowID(executionID string) string {
	return "cron-" + strings.TrimSpace(executionID)
}

func (s *Service) CronExecutionWorkflowV1(input CronWorkflowInput) CronWorkflowResult {
	workflowID := CronWorkflowID(input.ExecutionID)
	result := CronWorkflowResult{
		WorkflowID: workflowID,
		States:     []CronExecutionState{CronStateInit},
		Fallbacks:  []string{},
		RetryCount: input.RetryCount,
	}

	if input.JobExpired {
		result.States = append(result.States, CronStateSkipped)
		result.TerminalState = CronStateSkipped
		return result
	}

	if input.TimeoutExceeded {
		result.States = append(result.States, CronStateTimedOut)
		result.TerminalState = CronStateTimedOut
		return result
	}

	result.States = append(result.States, CronStateLoading)
	if input.LoadError {
		result.States = append(result.States, CronStateFailed)
		result.TerminalState = CronStateFailed
		return result
	}

	result.States = append(result.States, CronStateExecuting)
	if input.ExecuteError {
		if input.RetryCount < input.MaxRetries {
			result.Fallbacks = append(result.Fallbacks, "retry")
			result.RetryCount++
		} else {
			if input.NotifyOnFailure {
				result.States = append(result.States, CronStateNotifying)
			}
			result.States = append(result.States, CronStateFailed)
			result.TerminalState = CronStateFailed
			return result
		}
	}

	if input.WebhookEnabled {
		result.States = append(result.States, CronStateWebhook)
		if input.WebhookError {
			result.Fallbacks = append(result.Fallbacks, "webhook_retry")
		}
	}

	result.States = append(result.States, CronStateNotifying)
	result.States = append(result.States, CronStateCompleted)
	result.TerminalState = CronStateCompleted
	return result
}
