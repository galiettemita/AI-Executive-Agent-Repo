package temporal

import (
	"fmt"

	"go.temporal.io/sdk/client"
	"go.temporal.io/sdk/worker"
)

// NewWorker creates and configures a Temporal worker with all registered workflows and activities.
func NewWorker(c client.Client, taskQueue string) worker.Worker {
	w := worker.New(c, taskQueue, worker.Options{
		MaxConcurrentActivityExecutionSize:     200,
		MaxConcurrentWorkflowTaskExecutionSize: 200,
	})

	// Register workflows
	w.RegisterWorkflow(MessageProcessingWorkflow)
	w.RegisterWorkflow(OutboxDispatchWorkflow)
	w.RegisterWorkflow(ToolHealthEvaluationWorkflow)
	w.RegisterWorkflow(OnboardingWorkflow)
	w.RegisterWorkflow(CostRollupWorkflow)
	w.RegisterWorkflow(KillSwitchWorkflow)
	w.RegisterWorkflow(VoiceSessionWorkflow)
	w.RegisterWorkflow(LearningConsolidationWorkflow)
	w.RegisterWorkflow(FederationSyncWorkflow)

	// Register activities
	activities := NewActivities()
	w.RegisterActivity(activities.ValidateEnvelopeActivity)
	w.RegisterActivity(activities.ClassifyIntentActivity)
	w.RegisterActivity(activities.GeneratePlanActivity)
	w.RegisterActivity(activities.AuthorizePlanActivity)
	w.RegisterActivity(activities.ExecuteToolActivity)
	w.RegisterActivity(activities.SynthesizeResponseActivity)
	w.RegisterActivity(activities.FetchPendingOutboxActivity)
	w.RegisterActivity(activities.DispatchOutboxEntryActivity)
	w.RegisterActivity(activities.EvaluateToolHealthActivity)
	w.RegisterActivity(activities.ExecuteOnboardingStageActivity)
	w.RegisterActivity(activities.AggregateCostsActivity)
	w.RegisterActivity(activities.ActivateKillSwitchActivity)

	// Voice activities
	w.RegisterActivity(InitVoiceSessionActivity)
	w.RegisterActivity(ExtractVoiceTasksActivity)

	// Learning activities
	w.RegisterActivity(ClusterCorrectionsActivity)
	w.RegisterActivity(DetectConflictsActivity)
	w.RegisterActivity(ResolveConflictActivity)
	w.RegisterActivity(ProposeRulesActivity)

	// Federation activities
	w.RegisterActivity(ExecuteFederationSyncActivity)

	return w
}

// StartWorker creates a client and starts the worker. Blocks until interrupted.
func StartWorker(taskQueue string) error {
	c, err := NewClient()
	if err != nil {
		return fmt.Errorf("failed to create temporal client: %w", err)
	}
	defer c.Close()

	w := NewWorker(c, taskQueue)
	return w.Run(worker.InterruptCh())
}
