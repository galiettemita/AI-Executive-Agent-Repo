package temporal

import (
	"fmt"

	caipkg "github.com/brevio/brevio/internal/compliance/cai"
	consentpkg "github.com/brevio/brevio/internal/compliance/consent"
	hipaapkg "github.com/brevio/brevio/internal/compliance/hipaa"
	soc2pkg "github.com/brevio/brevio/internal/compliance/soc2"
	evaluationpkg "github.com/brevio/brevio/internal/evaluation"
	learningpkg "github.com/brevio/brevio/internal/learning"
	dpobatchpkg "github.com/brevio/brevio/internal/learning/dpo"
	federatedpkg "github.com/brevio/brevio/internal/learning/federated"
	memorypkg "github.com/brevio/brevio/internal/memory"
	redteampkg "github.com/brevio/brevio/internal/security/redteam"
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

	// Register V10.1 cost/revenue workflows.
	w.RegisterWorkflow(SubscriptionReconciliationWorkflow)

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
	w.RegisterActivity(activities.VerifyExecutionActivity)
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

	// Cognitive routing and quality gate activities.
	w.RegisterActivity(activities.DualProcessRoutingActivity)
	w.RegisterActivity(activities.ClarificationCheckActivity)
	w.RegisterActivity(activities.ResponseDriftCheckActivity)
	w.RegisterActivity(activities.AnalyseSentimentActivity)

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

	// V10.1 cost/revenue intelligence activities.
	w.RegisterActivity(activities.EnqueueLLMCostActivity)
	w.RegisterActivity(activities.EnqueueConnectorCostActivity)
	w.RegisterActivity(activities.IngestSubscriptionEventActivity)
	w.RegisterActivity(activities.ReconcileMRRActivity)
	w.RegisterActivity(activities.WriteLedgerFromOutboxActivity)

	// V10.2 intelligence gap closure workflows.
	w.RegisterWorkflow(IntelligenceProcessingWorkflow)
	w.RegisterWorkflow(AutonomyDemotionWorkflow)

	// V10.2 intelligence activities.
	w.RegisterActivity(activities.ApplyEQStrategyActivity)
	w.RegisterActivity(activities.EvaluateAutonomyDemotionActivity)
	w.RegisterActivity(activities.PersistCriticReflectorActivity)
	w.RegisterActivity(activities.RecordCalibrationOutcomeActivity)
	w.RegisterActivity(activities.ClassifyMultiIntentActivity)
	w.RegisterActivity(activities.AssessUncertaintyActivity)
	w.RegisterActivity(activities.EvaluateInterruptionsActivity)
	w.RegisterActivity(activities.PersistReasoningStepActivity)
	w.RegisterActivity(activities.V102ReasoningLoopActivity)

	// V10.2 P8 memory/context/RAG/latency workflows.
	w.RegisterWorkflow(MemoryContextMaintenanceWorkflow)

	// V10.3 cognitive intelligence workflows.
	w.RegisterWorkflow(NightlyConsolidationWorkflow)
	w.RegisterWorkflow(WeeklyDriftDetectionWorkflow)
	w.RegisterWorkflow(HeuristicUpdateWorkflow)
	w.RegisterWorkflow(BeliefMaintenanceWorkflow)
	w.RegisterWorkflow(CognitiveSignalProcessingWorkflow)

	// V10.3 cognitive intelligence activities.
	w.RegisterActivity(activities.UpdateHeuristicActivity)
	w.RegisterActivity(activities.RecalculateMetacognitiveActivity)
	w.RegisterActivity(activities.UpdateBeliefActivity)
	w.RegisterActivity(activities.DecayBeliefsActivity)
	w.RegisterActivity(activities.RunConsolidationActivity)
	w.RegisterActivity(activities.DetectDriftActivity)
	w.RegisterActivity(activities.RecordImplicitSignalActivity)
	w.RegisterActivity(activities.PersistClarificationActivity)
	w.RegisterActivity(activities.EvaluateCognitiveSignalsActivity)

	// V10.4 outbound call workflows.
	w.RegisterWorkflow(OutboundCallWorkflow)
	w.RegisterWorkflow(CallWebhookProcessingWorkflow)

	// V10.4 outbound call activities.
	w.RegisterActivity(activities.RequestCallApprovalActivity)
	w.RegisterActivity(activities.VerifyPhoneActivity)
	w.RegisterActivity(activities.MakeCallActivity)
	w.RegisterActivity(activities.ProcessCallWebhookActivity)
	w.RegisterActivity(activities.PersistTranscriptSegmentActivity)
	w.RegisterActivity(activities.CheckProviderHealthActivity)

	// V10.2 P8 memory/context/RAG/latency activities.
	w.RegisterActivity(activities.ApplyMemoryDecayActivity)
	w.RegisterActivity(activities.DetectLessonConflictActivity)
	w.RegisterActivity(activities.ResolveLessonConflictActivity)
	w.RegisterActivity(activities.EmbedAndChunkActivity)
	w.RegisterActivity(activities.RankWithFreshnessActivity)
	w.RegisterActivity(activities.PersistCompressionActivity)
	w.RegisterActivity(activities.EnforceContextBudgetActivity)
	w.RegisterActivity(activities.EvaluateLatencyBudgetActivity)
	w.RegisterActivity(activities.WarmFastPathCacheActivity)

	// Memory decay and RAPTOR consolidation cron workflows.
	w.RegisterWorkflow(memorypkg.DecaySweepWorkflow)
	w.RegisterWorkflow(memorypkg.RaptorConsolidationWorkflow)
	decayActivities := &memorypkg.DecaySweepActivities{DecaySvc: memorypkg.NewMemoryDecayService()}
	w.RegisterActivity(decayActivities.DecaySweepActivity)
	raptorActivities := &memorypkg.RaptorConsolidationActivities{MemoryRepo: deps.MemoryRepo}
	w.RegisterActivity(raptorActivities.RaptorConsolidationActivity)

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

	// PAHF preference learning activities.
	w.RegisterActivity(activities.PreferenceUpdateActivity)
	w.RegisterActivity(activities.PreferenceRetrievalActivity)

	// Hindsight reflection activity.
	w.RegisterActivity(activities.ReflectionActivity)

	// Production eval sampling and hallucination detection.
	w.RegisterWorkflow(ProductionEvalSamplerWorkflow)
	w.RegisterActivity(activities.ProductionEvalSampleActivity)
	w.RegisterActivity(activities.SynthesisVerifyActivity)

	// Vision pre-processing.
	w.RegisterActivity(activities.VisionPreProcessActivity)

	// Knowledge graph extraction (Phase 5).
	w.RegisterActivity(activities.KGExtractActivity)

	// SubAgent orchestrator workflow + activities.
	w.RegisterWorkflow(SubAgentOrchestratorWorkflow)
	w.RegisterActivity(activities.CheckSubAgentAutonomyActivity)
	w.RegisterActivity(activities.DecomposeSubTasksActivity)

	// Plan simulator.
	w.RegisterActivity(activities.SimulatePlanActivity)

	// GAIA benchmark.
	w.RegisterWorkflow(GAIARunnerWorkflow)
	w.RegisterActivity(activities.InitBenchmarkRunActivity)
	w.RegisterActivity(activities.RunBenchmarkTaskActivity)
	w.RegisterActivity(activities.RunAllBenchmarkTasksActivity)
	w.RegisterActivity(activities.FinalizeBenchmarkRunActivity)

	// DPO pipeline.
	w.RegisterWorkflow(DPORoundWorkflow)
	w.RegisterActivity(activities.FeedbackIngestionActivity)
	w.RegisterActivity(activities.DPODatasetReadyActivity)
	w.RegisterActivity(activities.StartDPORoundActivity)
	w.RegisterActivity(activities.PollDPOJobActivity)
	w.RegisterActivity(activities.CheckpointDeployActivity)
	w.RegisterActivity(activities.QualityDeltaMonitorActivity)

	// A2A task execution.
	w.RegisterWorkflow(A2ATaskExecutionWorkflow)
	w.RegisterActivity(activities.DelegateA2ATaskActivity)
	w.RegisterActivity(activities.ListExternalAgentsActivity)

	// Proactive monitoring.
	w.RegisterWorkflow(ProactiveMonitorWorkflow)
	w.RegisterActivity(activities.DetectProactiveSignalsActivity)
	w.RegisterActivity(activities.BuildAndDispatchProactiveOfferActivity)

	// World model expiry sweep.
	w.RegisterWorkflow(WorldModelExpiryCronWorkflow)
	w.RegisterActivity(activities.WorldModelExpirySweepActivity)
	w.RegisterActivity(activities.WorldModelExpirySweepWorkspaceListActivity)

	// EQ A/B promotion check.
	w.RegisterActivity(activities.EQPromotionCheckActivity)

	// DSR GDPR erasure cascade.
	w.RegisterWorkflow(DSRFullErasureWorkflow)
	w.RegisterActivity(activities.DSRDeleteEpisodicMemoryActivity)
	w.RegisterActivity(activities.DSRDeleteKGTriplesActivity)
	w.RegisterActivity(activities.DSRDeleteVectorChunksActivity)
	w.RegisterActivity(activities.DSRRedactExecutionLogsActivity)
	w.RegisterActivity(activities.DSRNullifyPIIActivity)
	w.RegisterActivity(activities.DSRRevokeConsentActivity)
	w.RegisterActivity(activities.DSRConfirmationActivity)
	w.RegisterActivity(activities.DSRSLAMonitorActivity)

	// EU AI Act compliance (Art. 9, 10, 73).
	w.RegisterWorkflow(EUAIActComplianceWorkflow)
	w.RegisterActivity(activities.EUAIActAggregateRisksActivity)
	w.RegisterActivity(activities.EUAIActCheckIncidentThresholdsActivity)
	w.RegisterActivity(activities.EUAIActGenerateConformityEvidenceActivity)

	// MCP tool discovery.
	w.RegisterWorkflow(MCPToolDiscoveryWorkflow)
	w.RegisterActivity(activities.MCPToolDiscoveryActivity)

	// A2A agent heartbeat.
	w.RegisterWorkflow(AgentHeartbeatWorkflow)
	w.RegisterActivity(activities.AgentHeartbeatActivity)

	// Red-team adversarial pipeline (P3-01).
	w.RegisterWorkflow(redteampkg.RedTeamWorkflow)
	if deps.RedTeamRunner != nil {
		rtActivities := &redteampkg.Activities{Runner: deps.RedTeamRunner}
		w.RegisterActivity(rtActivities.RunGCGAttacksActivity)
		w.RegisterActivity(rtActivities.RunAutoDanActivity)
		w.RegisterActivity(rtActivities.RunHarmBenchActivity)
		w.RegisterActivity(rtActivities.PersistReportActivity)
		w.RegisterActivity(rtActivities.CheckAutoHardeningActivity)
	}

	// Membership inference audit (P3-03).
	w.RegisterWorkflow(learningpkg.MembershipInferenceWorkflow)
	w.RegisterActivity(learningpkg.RunMembershipInferenceActivity)

	// Consent revocation erasure (P3-04).
	w.RegisterWorkflow(consentpkg.ConsentRevocationWorkflow)
	w.RegisterActivity(consentpkg.IdentifyDataForErasureActivity)
	w.RegisterActivity(consentpkg.ErasePreferenceDataActivity)
	w.RegisterActivity(consentpkg.EraseAnalyticsDataActivity)
	w.RegisterActivity(consentpkg.EraseMarketingDataActivity)
	w.RegisterActivity(consentpkg.AuditRevocationCompleteActivity)

	// HIPAA breach workflow (P3-06).
	w.RegisterWorkflow(hipaapkg.HIPAABreachWorkflow)
	w.RegisterWorkflow(hipaapkg.HIPAALogRetentionWorkflow)
	w.RegisterActivity(hipaapkg.CreateBreachRecordActivity)
	w.RegisterActivity(hipaapkg.NotifyComplianceTeamActivity)
	w.RegisterActivity(hipaapkg.ContainBreachActivity)
	w.RegisterActivity(hipaapkg.EscalateBreachActivity)
	w.RegisterActivity(hipaapkg.CleanupOldHIPAALogsActivity)

	// Compliance evidence collection (P3-05).
	w.RegisterWorkflow(soc2pkg.ComplianceEvidenceWorkflow)
	w.RegisterActivity(soc2pkg.CollectCC61Activity)
	w.RegisterActivity(soc2pkg.CollectCC66Activity)
	w.RegisterActivity(soc2pkg.CollectCC72Activity)
	w.RegisterActivity(soc2pkg.CollectCC92Activity)
	w.RegisterActivity(soc2pkg.CollectPI14Activity)
	w.RegisterActivity(soc2pkg.CollectISO27001Activity)

	// Continual learning: forgetting detector (P3-11).
	w.RegisterWorkflow(learningpkg.ForgettingDetectorWorkflow)
	w.RegisterActivity(learningpkg.UpdateBaselinesActivity)
	w.RegisterActivity(learningpkg.DetectForgettingActivity)
	w.RegisterActivity(learningpkg.RunAnchorPromotionActivity)

	// Shadow evaluation & MT-Bench (P3-12).
	w.RegisterWorkflow(evaluationpkg.ShadowEvalWorkflow)
	w.RegisterActivity(evaluationpkg.ScoreChampionActivity)
	w.RegisterActivity(evaluationpkg.ScoreChallengerActivity)
	w.RegisterActivity(evaluationpkg.LLMJudgeEvalActivity)
	w.RegisterActivity(evaluationpkg.RecordShadowResultActivity)
	w.RegisterWorkflow(evaluationpkg.MTBenchRunnerWorkflow)
	w.RegisterActivity(evaluationpkg.RunMTBenchActivity)

	// DPO batch processing (P3-10).
	w.RegisterWorkflow(dpobatchpkg.DPOBatchWorkflow)
	w.RegisterActivity(dpobatchpkg.ProcessQueuedPairsActivity)

	// Constitutional AI principle discovery (P3-13).
	w.RegisterWorkflow(caipkg.ConstitutionalPrincipleDiscoveryWorkflow)
	w.RegisterActivity(caipkg.RunDiscoveryActivity)
	w.RegisterActivity(caipkg.NotifyAdminActivity)

	// Federated fine-tuning (P3-14).
	w.RegisterWorkflow(federatedpkg.FederatedFineTuningWorkflow)
	w.RegisterActivity(federatedpkg.RunFederatedRoundActivity)

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
