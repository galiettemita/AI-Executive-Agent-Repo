package workflows

import (
	"context"
	"fmt"
	"math"
	"time"
)

// --- Activity input/output types ---

type CollectTrustMetricsInput struct {
	WorkspaceID string `json:"workspace_id"`
}

type CollectTrustMetricsResult struct {
	SuccessCount30d    int `json:"success_count_30d"`
	FailureCount30d    int `json:"failure_count_30d"`
	OverrideCount30d   int `json:"override_count_30d"`
	Trailing14dFailure int `json:"trailing_14d_failure"`
}

type ComputeTrustScoreInput struct {
	WorkspaceID      string `json:"workspace_id"`
	SuccessCount30d  int    `json:"success_count_30d"`
	FailureCount30d  int    `json:"failure_count_30d"`
	OverrideCount30d int    `json:"override_count_30d"`
}

type ComputeTrustScoreResult struct {
	TrustScore        float64 `json:"trust_score"`
	PromotionEligible bool    `json:"promotion_eligible"`
}

type ReviewGoalsInput struct {
	WorkspaceID   string `json:"workspace_id"`
	StalledAfterH int    `json:"stalled_after_hours"`
}

type ReviewGoalsResult struct {
	GoalsReviewed int `json:"goals_reviewed"`
	StalledGoals  int `json:"stalled_goals"`
}

type ConsolidateFeedbackInput struct {
	WorkspaceID     string `json:"workspace_id"`
	PendingFeedback int    `json:"pending_feedback"`
}

type ConsolidateFeedbackResult struct {
	ConsolidatedCount int `json:"consolidated_count"`
	ConfirmedLessons  int `json:"confirmed_lessons"`
}

type SummarizeDailyInput struct {
	WorkspaceID string `json:"workspace_id"`
	CaptureDate string `json:"capture_date"`
}

type SummarizeDailyResult struct {
	Summary string `json:"summary"`
}

type AppendDailyLogInput struct {
	WorkspaceID       string `json:"workspace_id"`
	InteractiveTurnID string `json:"interactive_turn_id"`
	CaptureDate       string `json:"capture_date"`
}

type AppendDailyLogResult struct {
	EntriesLogged int `json:"entries_logged"`
}

type CollectDependencyGraphInput struct {
	WorkspaceID string `json:"workspace_id"`
}

type CollectDependencyGraphResult struct {
	SharedDependencies int `json:"shared_dependencies"`
}

type DetectSharedPatternsInput struct {
	WorkspaceID string `json:"workspace_id"`
}

type DetectSharedPatternsResult struct {
	SharedPatterns int `json:"shared_patterns"`
}

type RefreshWidgetsInput struct {
	WorkspaceID string `json:"workspace_id"`
	WidgetCount int    `json:"widget_count"`
}

type RefreshWidgetsResult struct {
	WidgetsRefreshed int `json:"widgets_refreshed"`
}

type AnalyzeCapabilityGapsInput struct {
	WorkspaceID               string `json:"workspace_id"`
	CapabilityGapEventsLast7d int    `json:"capability_gap_events_last_7d"`
}

type AnalyzeCapabilityGapsResult struct {
	RecommendationsCount int `json:"recommendations_count"`
}

// --- V91Activities holds all V9.1 activity implementations ---

type V91Activities struct{}

func NewV91Activities() *V91Activities {
	return &V91Activities{}
}

func (a *V91Activities) CollectTrustMetricsActivity(_ context.Context, input CollectTrustMetricsInput) (*CollectTrustMetricsResult, error) {
	if input.WorkspaceID == "" {
		return nil, fmt.Errorf("workspace_id is required")
	}
	// In production, this queries the trust_events table for 30-day window.
	return &CollectTrustMetricsResult{
		SuccessCount30d:    25,
		FailureCount30d:    2,
		OverrideCount30d:   1,
		Trailing14dFailure: 0,
	}, nil
}

func (a *V91Activities) ComputeTrustScoreActivity(_ context.Context, input ComputeTrustScoreInput) (*ComputeTrustScoreResult, error) {
	denominator := maxInt(input.SuccessCount30d+input.FailureCount30d+input.OverrideCount30d, 1)
	score := float64(input.SuccessCount30d-2*input.FailureCount30d-3*input.OverrideCount30d) / float64(denominator)
	score = math.Round(score*10000) / 10000

	eligible := score >= 0.85 && input.SuccessCount30d >= 20

	return &ComputeTrustScoreResult{
		TrustScore:        score,
		PromotionEligible: eligible,
	}, nil
}

func (a *V91Activities) ReviewGoalsActivity(_ context.Context, input ReviewGoalsInput) (*ReviewGoalsResult, error) {
	if input.WorkspaceID == "" {
		return nil, fmt.Errorf("workspace_id is required")
	}
	// In production, this queries goals and checks for stalled status.
	return &ReviewGoalsResult{
		GoalsReviewed: 5,
		StalledGoals:  0,
	}, nil
}

func (a *V91Activities) ConsolidateFeedbackActivity(_ context.Context, input ConsolidateFeedbackInput) (*ConsolidateFeedbackResult, error) {
	if input.WorkspaceID == "" {
		return nil, fmt.Errorf("workspace_id is required")
	}
	// In production, this processes pending feedback entries and creates
	// confirmed lessons in the learning store.
	confirmed := input.PendingFeedback
	if confirmed > 20 {
		confirmed = 20
	}
	return &ConsolidateFeedbackResult{
		ConsolidatedCount: input.PendingFeedback,
		ConfirmedLessons:  confirmed,
	}, nil
}

func (a *V91Activities) SummarizeDailyActivity(_ context.Context, input SummarizeDailyInput) (*SummarizeDailyResult, error) {
	if input.WorkspaceID == "" {
		return nil, fmt.Errorf("workspace_id is required")
	}
	date := input.CaptureDate
	if date == "" {
		date = time.Now().UTC().Format("2006-01-02")
	}
	return &SummarizeDailyResult{
		Summary: fmt.Sprintf("Daily introspection completed for %s", date),
	}, nil
}

func (a *V91Activities) AppendDailyLogActivity(_ context.Context, input AppendDailyLogInput) (*AppendDailyLogResult, error) {
	if input.WorkspaceID == "" {
		return nil, fmt.Errorf("workspace_id is required")
	}
	if input.InteractiveTurnID == "" {
		return nil, fmt.Errorf("interactive_turn_id is required")
	}
	return &AppendDailyLogResult{
		EntriesLogged: 1,
	}, nil
}

func (a *V91Activities) CollectDependencyGraphActivity(_ context.Context, input CollectDependencyGraphInput) (*CollectDependencyGraphResult, error) {
	if input.WorkspaceID == "" {
		return nil, fmt.Errorf("workspace_id is required")
	}
	// In production, this collects dependency data from all ingested repositories.
	return &CollectDependencyGraphResult{
		SharedDependencies: 3,
	}, nil
}

func (a *V91Activities) DetectSharedPatternsActivity(_ context.Context, input DetectSharedPatternsInput) (*DetectSharedPatternsResult, error) {
	if input.WorkspaceID == "" {
		return nil, fmt.Errorf("workspace_id is required")
	}
	return &DetectSharedPatternsResult{
		SharedPatterns: 2,
	}, nil
}

func (a *V91Activities) RefreshWidgetsActivity(_ context.Context, input RefreshWidgetsInput) (*RefreshWidgetsResult, error) {
	if input.WorkspaceID == "" {
		return nil, fmt.Errorf("workspace_id is required")
	}
	return &RefreshWidgetsResult{
		WidgetsRefreshed: input.WidgetCount,
	}, nil
}

func (a *V91Activities) AnalyzeCapabilityGapsActivity(_ context.Context, input AnalyzeCapabilityGapsInput) (*AnalyzeCapabilityGapsResult, error) {
	if input.WorkspaceID == "" {
		return nil, fmt.Errorf("workspace_id is required")
	}
	count := 0
	if input.CapabilityGapEventsLast7d >= 3 {
		count = input.CapabilityGapEventsLast7d / 3
		if count == 0 {
			count = 1
		}
	}
	return &AnalyzeCapabilityGapsResult{
		RecommendationsCount: count,
	}, nil
}

// Standalone activity functions that delegate to the V91Activities struct methods.
// These are used as activity references in workflow registrations.

func CollectTrustMetricsActivity(ctx context.Context, input CollectTrustMetricsInput) (*CollectTrustMetricsResult, error) {
	return NewV91Activities().CollectTrustMetricsActivity(ctx, input)
}

func ComputeTrustScoreActivity(ctx context.Context, input ComputeTrustScoreInput) (*ComputeTrustScoreResult, error) {
	return NewV91Activities().ComputeTrustScoreActivity(ctx, input)
}

func ReviewGoalsActivity(ctx context.Context, input ReviewGoalsInput) (*ReviewGoalsResult, error) {
	return NewV91Activities().ReviewGoalsActivity(ctx, input)
}

func ConsolidateFeedbackActivity(ctx context.Context, input ConsolidateFeedbackInput) (*ConsolidateFeedbackResult, error) {
	return NewV91Activities().ConsolidateFeedbackActivity(ctx, input)
}

func SummarizeDailyActivity(ctx context.Context, input SummarizeDailyInput) (*SummarizeDailyResult, error) {
	return NewV91Activities().SummarizeDailyActivity(ctx, input)
}

func AppendDailyLogActivity(ctx context.Context, input AppendDailyLogInput) (*AppendDailyLogResult, error) {
	return NewV91Activities().AppendDailyLogActivity(ctx, input)
}

func CollectDependencyGraphActivity(ctx context.Context, input CollectDependencyGraphInput) (*CollectDependencyGraphResult, error) {
	return NewV91Activities().CollectDependencyGraphActivity(ctx, input)
}

func DetectSharedPatternsActivity(ctx context.Context, input DetectSharedPatternsInput) (*DetectSharedPatternsResult, error) {
	return NewV91Activities().DetectSharedPatternsActivity(ctx, input)
}

func RefreshWidgetsActivity(ctx context.Context, input RefreshWidgetsInput) (*RefreshWidgetsResult, error) {
	return NewV91Activities().RefreshWidgetsActivity(ctx, input)
}

func AnalyzeCapabilityGapsActivity(ctx context.Context, input AnalyzeCapabilityGapsInput) (*AnalyzeCapabilityGapsResult, error) {
	return NewV91Activities().AnalyzeCapabilityGapsActivity(ctx, input)
}
