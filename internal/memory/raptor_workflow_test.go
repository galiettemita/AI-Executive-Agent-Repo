package memory_test

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"
	"go.temporal.io/sdk/testsuite"

	"github.com/brevio/brevio/internal/memory"
)

type RaptorWorkflowSuite struct {
	suite.Suite
	testsuite.WorkflowTestSuite
}

func TestRaptorWorkflowSuite(t *testing.T) {
	suite.Run(t, new(RaptorWorkflowSuite))
}

func (s *RaptorWorkflowSuite) TestRaptorConsolidationWorkflow_HappyPath() {
	env := s.NewTestWorkflowEnvironment()
	activities := &memory.RaptorConsolidationActivities{}
	env.OnActivity(activities.RaptorConsolidationActivity, mock.Anything, mock.Anything).
		Return(&memory.RaptorConsolidationResult{ClustersCreated: 3, ItemsProcessed: 50}, nil)

	env.ExecuteWorkflow(memory.RaptorConsolidationWorkflow,
		memory.RaptorConsolidationWorkflowInput{WorkspaceID: "ws-test"})

	s.True(env.IsWorkflowCompleted())
	s.NoError(env.GetWorkflowError())
}

func (s *RaptorWorkflowSuite) TestRaptorConsolidationWorkflow_ActivityError() {
	env := s.NewTestWorkflowEnvironment()
	activities := &memory.RaptorConsolidationActivities{}
	env.OnActivity(activities.RaptorConsolidationActivity, mock.Anything, mock.Anything).
		Return(nil, fmt.Errorf("clustering failed"))

	env.ExecuteWorkflow(memory.RaptorConsolidationWorkflow,
		memory.RaptorConsolidationWorkflowInput{WorkspaceID: "ws-test"})

	s.True(env.IsWorkflowCompleted())
	s.Error(env.GetWorkflowError())
}
