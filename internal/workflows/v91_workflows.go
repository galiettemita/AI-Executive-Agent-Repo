package workflows

import (
	"time"

	"go.temporal.io/sdk/temporal"
	"go.temporal.io/sdk/workflow"
)

// --- Input/Result types for V9.1 workflows ---

type TrustScoringWorkflowInput struct {
	WorkspaceID string `json:"workspace_id"`
	RunDate     string `json:"run_date"`
}

type TrustScoringWorkflowResult struct {
	WorkspaceID       string  `json:"workspace_id"`
	TrustScore        float64 `json:"trust_score"`
	PromotionEligible bool    `json:"promotion_eligible"`
	WorkspacesScored  int     `json:"workspaces_scored"`
}

type GoalProgressWorkflowInput struct {
	WorkspaceID   string `json:"workspace_id"`
	StalledAfterH int    `json:"stalled_after_hours"`
}

type GoalProgressWorkflowResult struct {
	WorkspaceID   string `json:"workspace_id"`
	GoalsReviewed int    `json:"goals_reviewed"`
	StalledGoals  int    `json:"stalled_goals"`
	Status        string `json:"status"`
}

type LearningConsolidationWorkflowInput struct {
	WorkspaceID     string `json:"workspace_id"`
	PendingFeedback int    `json:"pending_feedback"`
}

type LearningConsolidationWorkflowResult struct {
	WorkspaceID       string `json:"workspace_id"`
	ConsolidatedCount int    `json:"consolidated_count"`
	ConfirmedLessons  int    `json:"confirmed_lessons"`
	Status            string `json:"status"`
}

type DailyIntrospectionWorkflowInput struct {
	WorkspaceID string `json:"workspace_id"`
	CaptureDate string `json:"capture_date"`
}

type DailyIntrospectionWorkflowResult struct {
	WorkspaceID string `json:"workspace_id"`
	CaptureDate string `json:"capture_date"`
	Summary     string `json:"summary"`
	Status      string `json:"status"`
}

type DailyLogCaptureWorkflowInput struct {
	WorkspaceID       string `json:"workspace_id"`
	InteractiveTurnID string `json:"interactive_turn_id"`
	CaptureDate       string `json:"capture_date"`
}

type DailyLogCaptureWorkflowResult struct {
	WorkspaceID   string `json:"workspace_id"`
	EntriesLogged int    `json:"entries_logged"`
	Status        string `json:"status"`
}

type CrossRepoAnalysisWorkflowInput struct {
	WorkspaceID     string `json:"workspace_id"`
	RepositoryCount int    `json:"repository_count"`
}

type CrossRepoAnalysisWorkflowResult struct {
	WorkspaceID        string `json:"workspace_id"`
	SharedDependencies int    `json:"shared_dependencies"`
	SharedPatterns     int    `json:"shared_patterns"`
	Status             string `json:"status"`
}

type MissionControlRefreshWorkflowInput struct {
	WorkspaceID string `json:"workspace_id"`
	WidgetCount int    `json:"widget_count"`
}

type MissionControlRefreshWorkflowResult struct {
	WorkspaceID      string `json:"workspace_id"`
	WidgetsRefreshed int    `json:"widgets_refreshed"`
	Status           string `json:"status"`
}

type CapabilityExplorationWorkflowInput struct {
	WorkspaceID               string `json:"workspace_id"`
	CapabilityGapEventsLast7d int    `json:"capability_gap_events_last_7d"`
}

type CapabilityExplorationWorkflowResult struct {
	WorkspaceID          string `json:"workspace_id"`
	RecommendationsCount int    `json:"recommendations_count"`
	Status               string `json:"status"`
}

// --- Workflow implementations ---

// TrustScoringWorkflow performs nightly recalculation of trust scores for a workspace.
func TrustScoringWorkflow(ctx workflow.Context, input TrustScoringWorkflowInput) (*TrustScoringWorkflowResult, error) {
	logger := workflow.GetLogger(ctx)
	logger.Info("TrustScoringWorkflow started", "workspaceID", input.WorkspaceID)

	var a *V91Activities
	ao := workflow.ActivityOptions{
		StartToCloseTimeout: 60 * time.Second,
		RetryPolicy: &temporal.RetryPolicy{
			InitialInterval:    time.Second,
			BackoffCoefficient: 2.0,
			MaximumInterval:    60 * time.Second,
			MaximumAttempts:    3,
		},
	}
	ctx = workflow.WithActivityOptions(ctx, ao)

	var collectResult CollectTrustMetricsResult
	err := workflow.ExecuteActivity(ctx, a.CollectTrustMetricsActivity, CollectTrustMetricsInput{
		WorkspaceID: input.WorkspaceID,
	}).Get(ctx, &collectResult)
	if err != nil {
		return &TrustScoringWorkflowResult{
			WorkspaceID: input.WorkspaceID,
		}, nil
	}

	var scoreResult ComputeTrustScoreResult
	err = workflow.ExecuteActivity(ctx, a.ComputeTrustScoreActivity, ComputeTrustScoreInput{
		WorkspaceID:      input.WorkspaceID,
		SuccessCount30d:  collectResult.SuccessCount30d,
		FailureCount30d:  collectResult.FailureCount30d,
		OverrideCount30d: collectResult.OverrideCount30d,
	}).Get(ctx, &scoreResult)
	if err != nil {
		return nil, err
	}

	return &TrustScoringWorkflowResult{
		WorkspaceID:       input.WorkspaceID,
		TrustScore:        scoreResult.TrustScore,
		PromotionEligible: scoreResult.PromotionEligible,
		WorkspacesScored:  1,
	}, nil
}

// GoalProgressWorkflow performs weekly check of goal milestones and status updates.
func GoalProgressWorkflow(ctx workflow.Context, input GoalProgressWorkflowInput) (*GoalProgressWorkflowResult, error) {
	logger := workflow.GetLogger(ctx)
	logger.Info("GoalProgressWorkflow started", "workspaceID", input.WorkspaceID)

	var a *V91Activities
	ao := workflow.ActivityOptions{
		StartToCloseTimeout: 30 * time.Second,
		RetryPolicy: &temporal.RetryPolicy{
			InitialInterval:    time.Second,
			BackoffCoefficient: 2.0,
			MaximumInterval:    30 * time.Second,
			MaximumAttempts:    3,
		},
	}
	ctx = workflow.WithActivityOptions(ctx, ao)

	var reviewResult ReviewGoalsResult
	err := workflow.ExecuteActivity(ctx, a.ReviewGoalsActivity, ReviewGoalsInput{
		WorkspaceID:   input.WorkspaceID,
		StalledAfterH: input.StalledAfterH,
	}).Get(ctx, &reviewResult)
	if err != nil {
		return nil, err
	}

	status := "reviewed"
	if reviewResult.StalledGoals > 0 {
		status = "stalled_detected"
	}

	return &GoalProgressWorkflowResult{
		WorkspaceID:   input.WorkspaceID,
		GoalsReviewed: reviewResult.GoalsReviewed,
		StalledGoals:  reviewResult.StalledGoals,
		Status:        status,
	}, nil
}

// LearningConsolidationWorkflow consolidates feedback into confirmed lessons.
func LearningConsolidationWorkflow(ctx workflow.Context, input LearningConsolidationWorkflowInput) (*LearningConsolidationWorkflowResult, error) {
	logger := workflow.GetLogger(ctx)
	logger.Info("LearningConsolidationWorkflow started", "workspaceID", input.WorkspaceID)

	if input.PendingFeedback <= 0 {
		return &LearningConsolidationWorkflowResult{
			WorkspaceID: input.WorkspaceID,
			Status:      "skipped",
		}, nil
	}

	var a *V91Activities
	ao := workflow.ActivityOptions{
		StartToCloseTimeout: 120 * time.Second,
		RetryPolicy: &temporal.RetryPolicy{
			InitialInterval:    time.Second,
			BackoffCoefficient: 2.0,
			MaximumInterval:    60 * time.Second,
			MaximumAttempts:    3,
		},
	}
	ctx = workflow.WithActivityOptions(ctx, ao)

	var consolidateResult ConsolidateFeedbackResult
	err := workflow.ExecuteActivity(ctx, a.ConsolidateFeedbackActivity, ConsolidateFeedbackInput{
		WorkspaceID:     input.WorkspaceID,
		PendingFeedback: input.PendingFeedback,
	}).Get(ctx, &consolidateResult)
	if err != nil {
		return nil, err
	}

	return &LearningConsolidationWorkflowResult{
		WorkspaceID:       input.WorkspaceID,
		ConsolidatedCount: consolidateResult.ConsolidatedCount,
		ConfirmedLessons:  consolidateResult.ConfirmedLessons,
		Status:            "consolidated",
	}, nil
}

// DailyIntrospectionWorkflow triggers daily capture summarization and hindsight reflection.
func DailyIntrospectionWorkflow(ctx workflow.Context, input DailyIntrospectionWorkflowInput) (*DailyIntrospectionWorkflowResult, error) {
	logger := workflow.GetLogger(ctx)
	logger.Info("DailyIntrospectionWorkflow started", "workspaceID", input.WorkspaceID, "date", input.CaptureDate)

	var a *V91Activities
	ao := workflow.ActivityOptions{
		StartToCloseTimeout: 5 * time.Minute,
		RetryPolicy: &temporal.RetryPolicy{
			InitialInterval:    time.Second,
			BackoffCoefficient: 2.0,
			MaximumInterval:    60 * time.Second,
			MaximumAttempts:    3,
		},
	}
	ctx = workflow.WithActivityOptions(ctx, ao)

	// Step 1: Existing summarization
	var summarizeResult SummarizeDailyResult
	err := workflow.ExecuteActivity(ctx, a.SummarizeDailyActivity, SummarizeDailyInput{
		WorkspaceID: input.WorkspaceID,
		CaptureDate: input.CaptureDate,
	}).Get(ctx, &summarizeResult)
	if err != nil {
		// Non-fatal: continue even if summarization fails
		logger.Warn("SummarizeDailyActivity failed", "error", err)
	}

	// Step 2: Hindsight reflection — clusters intents, identifies failures, writes insights
	var reflectionResult interface{}
	reflectErr := workflow.ExecuteActivity(ctx, "ReflectionActivity",
		map[string]interface{}{
			"workspace_id": input.WorkspaceID,
			"date":         input.CaptureDate,
			"max_insights": 10,
		},
	).Get(ctx, &reflectionResult)
	if reflectErr != nil {
		logger.Warn("ReflectionActivity failed", "error", reflectErr)
	}

	return &DailyIntrospectionWorkflowResult{
		WorkspaceID: input.WorkspaceID,
		CaptureDate: input.CaptureDate,
		Summary:     summarizeResult.Summary,
		Status:      "completed",
	}, nil
}

// DailyLogCaptureWorkflow accumulates log entries throughout the day.
func DailyLogCaptureWorkflow(ctx workflow.Context, input DailyLogCaptureWorkflowInput) (*DailyLogCaptureWorkflowResult, error) {
	logger := workflow.GetLogger(ctx)
	logger.Info("DailyLogCaptureWorkflow started", "workspaceID", input.WorkspaceID)

	if input.InteractiveTurnID == "" {
		return &DailyLogCaptureWorkflowResult{
			WorkspaceID: input.WorkspaceID,
			Status:      "skipped",
		}, nil
	}

	var a *V91Activities
	ao := workflow.ActivityOptions{
		StartToCloseTimeout: 15 * time.Second,
		RetryPolicy: &temporal.RetryPolicy{
			InitialInterval:    time.Second,
			BackoffCoefficient: 2.0,
			MaximumInterval:    15 * time.Second,
			MaximumAttempts:    3,
		},
	}
	ctx = workflow.WithActivityOptions(ctx, ao)

	var appendResult AppendDailyLogResult
	err := workflow.ExecuteActivity(ctx, a.AppendDailyLogActivity, AppendDailyLogInput{
		WorkspaceID:       input.WorkspaceID,
		InteractiveTurnID: input.InteractiveTurnID,
		CaptureDate:       input.CaptureDate,
	}).Get(ctx, &appendResult)
	if err != nil {
		return nil, err
	}

	return &DailyLogCaptureWorkflowResult{
		WorkspaceID:   input.WorkspaceID,
		EntriesLogged: appendResult.EntriesLogged,
		Status:        "captured",
	}, nil
}

// CrossRepoAnalysisWorkflow analyzes cross-repository dependencies and patterns.
func CrossRepoAnalysisWorkflow(ctx workflow.Context, input CrossRepoAnalysisWorkflowInput) (*CrossRepoAnalysisWorkflowResult, error) {
	logger := workflow.GetLogger(ctx)
	logger.Info("CrossRepoAnalysisWorkflow started", "workspaceID", input.WorkspaceID)

	if input.RepositoryCount <= 1 {
		return &CrossRepoAnalysisWorkflowResult{
			WorkspaceID: input.WorkspaceID,
			Status:      "insufficient_repositories",
		}, nil
	}

	var a *V91Activities
	ao := workflow.ActivityOptions{
		StartToCloseTimeout: 120 * time.Second,
		RetryPolicy: &temporal.RetryPolicy{
			InitialInterval:    time.Second,
			BackoffCoefficient: 2.0,
			MaximumInterval:    60 * time.Second,
			MaximumAttempts:    3,
		},
	}
	ctx = workflow.WithActivityOptions(ctx, ao)

	var collectResult CollectDependencyGraphResult
	err := workflow.ExecuteActivity(ctx, a.CollectDependencyGraphActivity, CollectDependencyGraphInput{
		WorkspaceID: input.WorkspaceID,
	}).Get(ctx, &collectResult)
	if err != nil {
		return nil, err
	}

	var detectResult DetectSharedPatternsResult
	err = workflow.ExecuteActivity(ctx, a.DetectSharedPatternsActivity, DetectSharedPatternsInput{
		WorkspaceID: input.WorkspaceID,
	}).Get(ctx, &detectResult)
	if err != nil {
		return nil, err
	}

	return &CrossRepoAnalysisWorkflowResult{
		WorkspaceID:        input.WorkspaceID,
		SharedDependencies: collectResult.SharedDependencies,
		SharedPatterns:     detectResult.SharedPatterns,
		Status:             "completed",
	}, nil
}

// MissionControlRefreshWorkflow refreshes mission control widget data.
func MissionControlRefreshWorkflow(ctx workflow.Context, input MissionControlRefreshWorkflowInput) (*MissionControlRefreshWorkflowResult, error) {
	logger := workflow.GetLogger(ctx)
	logger.Info("MissionControlRefreshWorkflow started", "workspaceID", input.WorkspaceID)

	if input.WidgetCount <= 0 {
		return &MissionControlRefreshWorkflowResult{
			WorkspaceID: input.WorkspaceID,
			Status:      "empty",
		}, nil
	}

	var a *V91Activities
	ao := workflow.ActivityOptions{
		StartToCloseTimeout: 30 * time.Second,
		RetryPolicy: &temporal.RetryPolicy{
			InitialInterval:    time.Second,
			BackoffCoefficient: 2.0,
			MaximumInterval:    30 * time.Second,
			MaximumAttempts:    3,
		},
	}
	ctx = workflow.WithActivityOptions(ctx, ao)

	var refreshResult RefreshWidgetsResult
	err := workflow.ExecuteActivity(ctx, a.RefreshWidgetsActivity, RefreshWidgetsInput{
		WorkspaceID: input.WorkspaceID,
		WidgetCount: input.WidgetCount,
	}).Get(ctx, &refreshResult)
	if err != nil {
		return nil, err
	}

	return &MissionControlRefreshWorkflowResult{
		WorkspaceID:      input.WorkspaceID,
		WidgetsRefreshed: refreshResult.WidgetsRefreshed,
		Status:           "refreshed",
	}, nil
}

// CapabilityExplorationWorkflow performs periodic capability gap analysis.
func CapabilityExplorationWorkflow(ctx workflow.Context, input CapabilityExplorationWorkflowInput) (*CapabilityExplorationWorkflowResult, error) {
	logger := workflow.GetLogger(ctx)
	logger.Info("CapabilityExplorationWorkflow started", "workspaceID", input.WorkspaceID)

	var a *V91Activities
	ao := workflow.ActivityOptions{
		StartToCloseTimeout: 60 * time.Second,
		RetryPolicy: &temporal.RetryPolicy{
			InitialInterval:    time.Second,
			BackoffCoefficient: 2.0,
			MaximumInterval:    60 * time.Second,
			MaximumAttempts:    3,
		},
	}
	ctx = workflow.WithActivityOptions(ctx, ao)

	var analyzeResult AnalyzeCapabilityGapsResult
	err := workflow.ExecuteActivity(ctx, a.AnalyzeCapabilityGapsActivity, AnalyzeCapabilityGapsInput{
		WorkspaceID:               input.WorkspaceID,
		CapabilityGapEventsLast7d: input.CapabilityGapEventsLast7d,
	}).Get(ctx, &analyzeResult)
	if err != nil {
		return nil, err
	}

	if analyzeResult.RecommendationsCount == 0 {
		return &CapabilityExplorationWorkflowResult{
			WorkspaceID: input.WorkspaceID,
			Status:      "no_action",
		}, nil
	}

	return &CapabilityExplorationWorkflowResult{
		WorkspaceID:          input.WorkspaceID,
		RecommendationsCount: analyzeResult.RecommendationsCount,
		Status:               "recommendations_created",
	}, nil
}
