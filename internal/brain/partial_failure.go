package brain

import (
	"fmt"
	"sync"
)

// TaskResult captures the outcome of a single task in a multi-task plan.
type TaskResult struct {
	TaskIndex int            `json:"task_index"`
	TaskName  string         `json:"task_name"`
	Success   bool           `json:"success"`
	Output    map[string]any `json:"output,omitempty"`
	Error     string         `json:"error,omitempty"`
}

// PartialResult aggregates the results of a multi-task execution.
type PartialResult struct {
	TotalTasks  int          `json:"total_tasks"`
	Succeeded   int          `json:"succeeded"`
	Failed      int          `json:"failed"`
	Results     []TaskResult `json:"results"`
	FailedTasks []TaskResult `json:"failed_tasks"`
}

// Decision represents the policy decision for handling a partial failure.
type Decision string

const (
	DecisionDeliverPartial Decision = "deliver_partial"
	DecisionRetryFailed    Decision = "retry_failed"
	DecisionAskUser        Decision = "ask_user"
	DecisionFailAll        Decision = "fail_all"
)

// PartialFailurePolicy evaluates partial results and determines next action.
type PartialFailurePolicy struct {
	mu                   sync.Mutex
	minSuccessRatio      float64 // min ratio of succeeded/total to deliver partial
	maxRetryableFailures int     // max failed tasks that can be retried
}

// NewPartialFailurePolicy creates a policy with default thresholds.
func NewPartialFailurePolicy() *PartialFailurePolicy {
	return &PartialFailurePolicy{
		minSuccessRatio:      0.5,
		maxRetryableFailures: 3,
	}
}

// NewPartialFailurePolicyWithConfig creates a policy with custom thresholds.
func NewPartialFailurePolicyWithConfig(minSuccessRatio float64, maxRetryableFailures int) *PartialFailurePolicy {
	if minSuccessRatio <= 0 || minSuccessRatio > 1 {
		minSuccessRatio = 0.5
	}
	if maxRetryableFailures <= 0 {
		maxRetryableFailures = 3
	}
	return &PartialFailurePolicy{
		minSuccessRatio:      minSuccessRatio,
		maxRetryableFailures: maxRetryableFailures,
	}
}

// BuildPartialResult constructs a PartialResult from a list of TaskResults.
func BuildPartialResult(results []TaskResult) PartialResult {
	pr := PartialResult{
		TotalTasks: len(results),
		Results:    results,
	}
	for _, r := range results {
		if r.Success {
			pr.Succeeded++
		} else {
			pr.Failed++
			pr.FailedTasks = append(pr.FailedTasks, r)
		}
	}
	return pr
}

// Evaluate examines a partial result and returns the appropriate decision.
func (p *PartialFailurePolicy) Evaluate(pr PartialResult) (Decision, error) {
	if pr.TotalTasks == 0 {
		return DecisionFailAll, fmt.Errorf("no tasks to evaluate")
	}

	p.mu.Lock()
	defer p.mu.Unlock()

	// All succeeded.
	if pr.Failed == 0 {
		return DecisionDeliverPartial, nil
	}

	// All failed.
	if pr.Succeeded == 0 {
		return DecisionFailAll, nil
	}

	successRatio := float64(pr.Succeeded) / float64(pr.TotalTasks)

	// If enough succeeded, deliver partial results.
	if successRatio >= p.minSuccessRatio {
		return DecisionDeliverPartial, nil
	}

	// If failures are retryable (few enough), suggest retry.
	if pr.Failed <= p.maxRetryableFailures {
		return DecisionRetryFailed, nil
	}

	// Otherwise, ask the user.
	return DecisionAskUser, nil
}
