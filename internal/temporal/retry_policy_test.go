package temporal

import (
	"testing"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"
	"go.temporal.io/sdk/testsuite"
)

type RetryPolicySuite struct {
	suite.Suite
	testsuite.WorkflowTestSuite
}

func TestRetryPolicySuite(t *testing.T) {
	suite.Run(t, new(RetryPolicySuite))
}

// Test 1: OutboxDispatch — fetch returns entries, dispatch succeeds.
func (s *RetryPolicySuite) TestOutboxDispatch_FetchAndDispatch_Succeeds() {
	env := s.NewTestWorkflowEnvironment()
	var a *Activities

	env.OnActivity(a.FetchPendingOutboxActivity, mock.Anything, mock.Anything).Return(
		&OutboxFetchResult{
			Entries: []OutboxEntry{
				{ID: "e1", Target: "https://example.com/hook", Payload: `{"ok":true}`},
			},
		}, nil,
	)
	env.OnActivity(a.DispatchOutboxEntryActivity, mock.Anything, mock.Anything).Return(
		&OutboxEntryDispatchResult{Success: true, DLQ: false}, nil,
	)

	env.ExecuteWorkflow(OutboxDispatchWorkflow, OutboxDispatchInput{
		BatchSize: 10,
	})

	s.True(env.IsWorkflowCompleted())
	s.NoError(env.GetWorkflowError())

	var result OutboxDispatchResult
	s.NoError(env.GetWorkflowResult(&result))
	s.Equal(1, result.TotalFetched)
	s.Equal(1, result.TotalDispatched)
	s.Equal(0, result.TotalDLQ)
}

// Test 2: OutboxDispatch — dispatch failure increments DLQ count.
func (s *RetryPolicySuite) TestOutboxDispatch_DispatchFail_DLQCounted() {
	env := s.NewTestWorkflowEnvironment()
	var a *Activities

	env.OnActivity(a.FetchPendingOutboxActivity, mock.Anything, mock.Anything).Return(
		&OutboxFetchResult{
			Entries: []OutboxEntry{
				{ID: "e-fail", Target: "https://bad.endpoint/hook", Payload: `{}`},
			},
		}, nil,
	)
	env.OnActivity(a.DispatchOutboxEntryActivity, mock.Anything, mock.Anything).Return(
		&OutboxEntryDispatchResult{Success: false, DLQ: true}, nil,
	)

	env.ExecuteWorkflow(OutboxDispatchWorkflow, OutboxDispatchInput{
		BatchSize: 10,
	})

	s.True(env.IsWorkflowCompleted())
	var result OutboxDispatchResult
	s.NoError(env.GetWorkflowResult(&result))
	s.Equal(1, result.TotalDLQ)
}

// Test 3: OutboxDispatch — empty batch completes with zero counts.
func (s *RetryPolicySuite) TestOutboxDispatch_EmptyBatch_CompletesClean() {
	env := s.NewTestWorkflowEnvironment()
	var a *Activities

	env.OnActivity(a.FetchPendingOutboxActivity, mock.Anything, mock.Anything).Return(
		&OutboxFetchResult{Entries: []OutboxEntry{}}, nil,
	)

	env.ExecuteWorkflow(OutboxDispatchWorkflow, OutboxDispatchInput{
		BatchSize: 10,
	})

	s.True(env.IsWorkflowCompleted())
	var result OutboxDispatchResult
	s.NoError(env.GetWorkflowResult(&result))
	s.Equal(0, result.TotalFetched)
	s.Equal(0, result.TotalDispatched)
}
