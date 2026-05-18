package workflows

import (
	"context"
	"testing"
)

func TestCollectTrustMetricsActivity(t *testing.T) {
	t.Parallel()
	a := NewV91Activities()
	result, err := a.CollectTrustMetricsActivity(context.Background(), CollectTrustMetricsInput{
		WorkspaceID: "ws-1",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.SuccessCount30d <= 0 {
		t.Fatal("expected positive success count")
	}
}

func TestCollectTrustMetricsActivityMissingWorkspace(t *testing.T) {
	t.Parallel()
	a := NewV91Activities()
	_, err := a.CollectTrustMetricsActivity(context.Background(), CollectTrustMetricsInput{})
	if err == nil {
		t.Fatal("expected error for missing workspace_id")
	}
}

func TestComputeTrustScoreActivity(t *testing.T) {
	t.Parallel()
	a := NewV91Activities()
	result, err := a.ComputeTrustScoreActivity(context.Background(), ComputeTrustScoreInput{
		WorkspaceID:      "ws-1",
		SuccessCount30d:  25,
		FailureCount30d:  2,
		OverrideCount30d: 1,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// score = (25 - 4 - 3) / 28 = 18/28 = 0.6429
	if result.TrustScore < 0.5 || result.TrustScore > 1.0 {
		t.Fatalf("unexpected trust score: %f", result.TrustScore)
	}
}

func TestComputeTrustScorePromotionEligible(t *testing.T) {
	t.Parallel()
	a := NewV91Activities()
	result, err := a.ComputeTrustScoreActivity(context.Background(), ComputeTrustScoreInput{
		WorkspaceID:      "ws-1",
		SuccessCount30d:  30,
		FailureCount30d:  0,
		OverrideCount30d: 0,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.TrustScore != 1.0 {
		t.Fatalf("expected trust score 1.0, got %f", result.TrustScore)
	}
	if !result.PromotionEligible {
		t.Fatal("expected promotion eligible")
	}
}

func TestReviewGoalsActivity(t *testing.T) {
	t.Parallel()
	a := NewV91Activities()
	result, err := a.ReviewGoalsActivity(context.Background(), ReviewGoalsInput{
		WorkspaceID:   "ws-1",
		StalledAfterH: 168,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.GoalsReviewed <= 0 {
		t.Fatal("expected positive goals reviewed")
	}
}

func TestReviewGoalsActivityMissingWorkspace(t *testing.T) {
	t.Parallel()
	a := NewV91Activities()
	_, err := a.ReviewGoalsActivity(context.Background(), ReviewGoalsInput{})
	if err == nil {
		t.Fatal("expected error for missing workspace_id")
	}
}

func TestConsolidateFeedbackActivity(t *testing.T) {
	t.Parallel()
	a := NewV91Activities()
	result, err := a.ConsolidateFeedbackActivity(context.Background(), ConsolidateFeedbackInput{
		WorkspaceID:     "ws-1",
		PendingFeedback: 10,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.ConsolidatedCount != 10 {
		t.Fatalf("expected 10 consolidated, got %d", result.ConsolidatedCount)
	}
	if result.ConfirmedLessons != 10 {
		t.Fatalf("expected 10 confirmed lessons, got %d", result.ConfirmedLessons)
	}
}

func TestConsolidateFeedbackActivityCap(t *testing.T) {
	t.Parallel()
	a := NewV91Activities()
	result, err := a.ConsolidateFeedbackActivity(context.Background(), ConsolidateFeedbackInput{
		WorkspaceID:     "ws-1",
		PendingFeedback: 50,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.ConfirmedLessons != 20 {
		t.Fatalf("expected capped at 20, got %d", result.ConfirmedLessons)
	}
}

func TestSummarizeDailyActivity(t *testing.T) {
	t.Parallel()
	a := NewV91Activities()
	result, err := a.SummarizeDailyActivity(context.Background(), SummarizeDailyInput{
		WorkspaceID: "ws-1",
		CaptureDate: "2026-03-08",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Summary == "" {
		t.Fatal("expected non-empty summary")
	}
}

func TestAppendDailyLogActivity(t *testing.T) {
	t.Parallel()
	a := NewV91Activities()
	result, err := a.AppendDailyLogActivity(context.Background(), AppendDailyLogInput{
		WorkspaceID:       "ws-1",
		InteractiveTurnID: "turn-001",
		CaptureDate:       "2026-03-08",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.EntriesLogged != 1 {
		t.Fatalf("expected 1 entry logged, got %d", result.EntriesLogged)
	}
}

func TestAppendDailyLogActivityMissingTurnID(t *testing.T) {
	t.Parallel()
	a := NewV91Activities()
	_, err := a.AppendDailyLogActivity(context.Background(), AppendDailyLogInput{
		WorkspaceID: "ws-1",
	})
	if err == nil {
		t.Fatal("expected error for missing interactive_turn_id")
	}
}

func TestCollectDependencyGraphActivity(t *testing.T) {
	t.Parallel()
	a := NewV91Activities()
	result, err := a.CollectDependencyGraphActivity(context.Background(), CollectDependencyGraphInput{
		WorkspaceID: "ws-1",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.SharedDependencies < 0 {
		t.Fatal("expected non-negative shared dependencies")
	}
}

func TestDetectSharedPatternsActivity(t *testing.T) {
	t.Parallel()
	a := NewV91Activities()
	result, err := a.DetectSharedPatternsActivity(context.Background(), DetectSharedPatternsInput{
		WorkspaceID: "ws-1",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.SharedPatterns < 0 {
		t.Fatal("expected non-negative shared patterns")
	}
}

func TestRefreshWidgetsActivity(t *testing.T) {
	t.Parallel()
	a := NewV91Activities()
	result, err := a.RefreshWidgetsActivity(context.Background(), RefreshWidgetsInput{
		WorkspaceID: "ws-1",
		WidgetCount: 5,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.WidgetsRefreshed != 5 {
		t.Fatalf("expected 5 widgets refreshed, got %d", result.WidgetsRefreshed)
	}
}

func TestAnalyzeCapabilityGapsActivity(t *testing.T) {
	t.Parallel()
	a := NewV91Activities()

	// Below threshold: no recommendations.
	result, err := a.AnalyzeCapabilityGapsActivity(context.Background(), AnalyzeCapabilityGapsInput{
		WorkspaceID:               "ws-1",
		CapabilityGapEventsLast7d: 2,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.RecommendationsCount != 0 {
		t.Fatalf("expected 0 recommendations for < 3 gaps, got %d", result.RecommendationsCount)
	}

	// At threshold: recommendations created.
	result, err = a.AnalyzeCapabilityGapsActivity(context.Background(), AnalyzeCapabilityGapsInput{
		WorkspaceID:               "ws-1",
		CapabilityGapEventsLast7d: 6,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.RecommendationsCount <= 0 {
		t.Fatalf("expected positive recommendations for 6 gaps, got %d", result.RecommendationsCount)
	}
}

func TestStandaloneTrustActivities(t *testing.T) {
	t.Parallel()
	// Verify standalone functions delegate correctly.
	result, err := CollectTrustMetricsActivity(context.Background(), CollectTrustMetricsInput{
		WorkspaceID: "ws-1",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.SuccessCount30d <= 0 {
		t.Fatal("expected positive success count from standalone function")
	}
}

func TestStandaloneComputeTrustScore(t *testing.T) {
	t.Parallel()
	result, err := ComputeTrustScoreActivity(context.Background(), ComputeTrustScoreInput{
		WorkspaceID:      "ws-1",
		SuccessCount30d:  20,
		FailureCount30d:  0,
		OverrideCount30d: 0,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.TrustScore != 1.0 {
		t.Fatalf("expected 1.0, got %f", result.TrustScore)
	}
}

func TestStandaloneReviewGoals(t *testing.T) {
	t.Parallel()
	result, err := ReviewGoalsActivity(context.Background(), ReviewGoalsInput{
		WorkspaceID:   "ws-1",
		StalledAfterH: 168,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.GoalsReviewed <= 0 {
		t.Fatal("expected positive goals reviewed from standalone")
	}
}

// TestV91WorkflowInputDefaults verifies that workflow input types have
// proper zero-value semantics.
func TestV91WorkflowInputDefaults(t *testing.T) {
	t.Parallel()

	// TrustScoringWorkflowInput
	trustInput := TrustScoringWorkflowInput{}
	if trustInput.WorkspaceID != "" {
		t.Fatal("expected empty workspace_id")
	}

	// GoalProgressWorkflowInput
	goalInput := GoalProgressWorkflowInput{}
	if goalInput.StalledAfterH != 0 {
		t.Fatal("expected zero stalled_after_hours")
	}

	// LearningConsolidationWorkflowInput
	learnInput := LearningConsolidationWorkflowInput{}
	if learnInput.PendingFeedback != 0 {
		t.Fatal("expected zero pending_feedback")
	}

	// DailyIntrospectionWorkflowInput
	introInput := DailyIntrospectionWorkflowInput{}
	if introInput.CaptureDate != "" {
		t.Fatal("expected empty capture_date")
	}

	// CrossRepoAnalysisWorkflowInput
	crossInput := CrossRepoAnalysisWorkflowInput{}
	if crossInput.RepositoryCount != 0 {
		t.Fatal("expected zero repository_count")
	}

	// MissionControlRefreshWorkflowInput
	mcInput := MissionControlRefreshWorkflowInput{}
	if mcInput.WidgetCount != 0 {
		t.Fatal("expected zero widget_count")
	}

	// CapabilityExplorationWorkflowInput
	capInput := CapabilityExplorationWorkflowInput{}
	if capInput.CapabilityGapEventsLast7d != 0 {
		t.Fatal("expected zero capability_gap_events")
	}
}
