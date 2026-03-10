package temporal

import (
	"fmt"

	"github.com/brevio/brevio/internal/workflows"
	"go.temporal.io/sdk/client"
	"go.temporal.io/sdk/worker"
)

// NewWorker creates a Temporal worker with all registered workflows and activities
// using no-dependency (test/degraded) mode.
func NewWorker(c client.Client, taskQueue string) worker.Worker {
	return NewWorkerWithDeps(c, taskQueue, ActivityDeps{})
}

// NewWorkerWithDeps creates a Temporal worker with dependency-injected activities.
// When deps are provided, activities operate in production mode backed by pgx/outbox.
// When deps are zero-valued, activities operate in test/degraded mode.
func NewWorkerWithDeps(c client.Client, taskQueue string, deps ActivityDeps) worker.Worker {
	w := worker.New(c, taskQueue, worker.Options{
		MaxConcurrentActivityExecutionSize:     200,
		MaxConcurrentWorkflowTaskExecutionSize: 200,
	})

	// Construct activities with injected dependencies.
	var activities *Activities
	if deps.Pool != nil {
		activities = NewActivitiesWithProdDeps(deps)
	} else {
		activities = NewActivities()
	}

	// Register core workflows.
	w.RegisterWorkflow(MessageProcessingWorkflow)
	w.RegisterWorkflow(OutboxDispatchWorkflow)
	w.RegisterWorkflow(ToolHealthEvaluationWorkflow)
	w.RegisterWorkflow(OnboardingWorkflow)
	w.RegisterWorkflow(CostRollupWorkflow)
	w.RegisterWorkflow(KillSwitchWorkflow)
	w.RegisterWorkflow(VoiceSessionWorkflow)
	w.RegisterWorkflow(LearningConsolidationWorkflow)
	w.RegisterWorkflow(FederationSyncWorkflow)

	// Register P8 feature closure workflows.
	w.RegisterWorkflow(FederationNegotiationWorkflow)
	w.RegisterWorkflow(EdgeOfflineSyncWorkflow)
	w.RegisterWorkflow(BrowserAutomationWorkflow)
	w.RegisterWorkflow(FastPathPipelineWorkflow)
	w.RegisterWorkflow(ExperimentAssignmentWorkflow)
	w.RegisterWorkflow(OnboardingProvisioningWorkflow)
	w.RegisterWorkflow(BillingEnforcementWorkflow)
	w.RegisterWorkflow(LoadSheddingTierWorkflow)

	// Register V9.1 soft intelligence workflows.
	w.RegisterWorkflow(workflows.TrustScoringWorkflow)
	w.RegisterWorkflow(workflows.GoalProgressWorkflow)
	w.RegisterWorkflow(workflows.LearningConsolidationWorkflow)
	w.RegisterWorkflow(workflows.DailyIntrospectionWorkflow)
	w.RegisterWorkflow(workflows.DailyLogCaptureWorkflow)
	w.RegisterWorkflow(workflows.CrossRepoAnalysisWorkflow)
	w.RegisterWorkflow(workflows.MissionControlRefreshWorkflow)
	w.RegisterWorkflow(workflows.CapabilityExplorationWorkflow)

	// Register core activities (method-based).
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

	// Intelligence pipeline activities (P7).
	w.RegisterActivity(activities.RetrieveMemoryActivity)
	w.RegisterActivity(activities.SearchRAGActivity)
	w.RegisterActivity(activities.ExecuteReasoningLoopActivity)
	w.RegisterActivity(activities.EvaluateCouncilActivity)
	w.RegisterActivity(activities.EnqueueOutboxActivity)
	w.RegisterActivity(activities.AssessCognitiveStateActivity)

	// Voice activities (method-based).
	w.RegisterActivity(activities.InitVoiceSessionActivity)
	w.RegisterActivity(activities.ExtractVoiceTasksActivity)

	// Learning activities (method-based).
	w.RegisterActivity(activities.ClusterCorrectionsActivity)
	w.RegisterActivity(activities.DetectConflictsActivity)
	w.RegisterActivity(activities.ResolveConflictActivity)
	w.RegisterActivity(activities.ProposeRulesActivity)

	// Federation activities (method-based).
	w.RegisterActivity(activities.ExecuteFederationSyncActivity)
	w.RegisterActivity(activities.CheckFederationPolicyActivity)
	w.RegisterActivity(activities.ExecuteFederationNegotiateActivity)
	w.RegisterActivity(activities.CompensateFederationActivity)

	// Edge sync activities (P8).
	w.RegisterActivity(activities.FetchEdgeTasksActivity)
	w.RegisterActivity(activities.DetectEdgeConflictsActivity)
	w.RegisterActivity(activities.ResolveEdgeConflictsActivity)
	w.RegisterActivity(activities.ExecuteEdgeTasksActivity)

	// Browser automation activities (P8).
	w.RegisterActivity(activities.ValidateBrowserReceiptActivity)
	w.RegisterActivity(activities.StartBrowserSessionActivity)
	w.RegisterActivity(activities.ExecuteBrowserTaskActivity)
	w.RegisterActivity(activities.CloseBrowserSessionActivity)

	// Fast-path activities (P8).
	w.RegisterActivity(activities.FastPathMatchActivity)
	w.RegisterActivity(activities.RecordFastPathMetricActivity)

	// Experiment activities (P8).
	w.RegisterActivity(activities.CheckExistingAssignmentActivity)
	w.RegisterActivity(activities.DeterministicAssignActivity)
	w.RegisterActivity(activities.PersistAssignmentActivity)

	// Onboarding provisioning activities (P8).
	w.RegisterActivity(activities.InitOnboardingSessionActivity)
	w.RegisterActivity(activities.ExecuteProvisioningStageActivity)
	w.RegisterActivity(activities.FinalizeOnboardingActivity)

	// Billing enforcement activities (P8).
	w.RegisterActivity(activities.IngestBillingWebhookActivity)
	w.RegisterActivity(activities.UpdateBillingLedgerActivity)
	w.RegisterActivity(activities.EnforceBillingPolicyActivity)

	// Load shedding activities (P8).
	w.RegisterActivity(activities.EvaluateLoadSheddingTierActivity)
	w.RegisterActivity(activities.PropagateLoadSheddingTierActivity)

	// V9.1 soft intelligence activities (method-based).
	v91 := workflows.NewV91Activities()
	w.RegisterActivity(v91.CollectTrustMetricsActivity)
	w.RegisterActivity(v91.ComputeTrustScoreActivity)
	w.RegisterActivity(v91.ReviewGoalsActivity)
	w.RegisterActivity(v91.ConsolidateFeedbackActivity)
	w.RegisterActivity(v91.SummarizeDailyActivity)
	w.RegisterActivity(v91.AppendDailyLogActivity)
	w.RegisterActivity(v91.CollectDependencyGraphActivity)
	w.RegisterActivity(v91.DetectSharedPatternsActivity)
	w.RegisterActivity(v91.RefreshWidgetsActivity)
	w.RegisterActivity(v91.AnalyzeCapabilityGapsActivity)

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
