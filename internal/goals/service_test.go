package goals

import (
	"testing"
	"time"
)

func TestGoalsLifecycle(t *testing.T) {
	s := NewService()
	goal := s.UpsertGoal(Goal{
		WorkspaceID: "ws_1",
		Title:       "Ship strict closure",
		Status:      "active",
		Priority:    "high",
	})
	if goal.ID == "" {
		t.Fatalf("expected goal id")
	}

	s.AddMilestone(goal.ID, Milestone{Title: "Implement endpoints", Status: "pending"})
	s.AddProgress(goal.ID, ProgressLog{Summary: "Implemented core handlers"})
	if len(s.ListMilestones(goal.ID)) != 1 {
		t.Fatalf("expected one milestone")
	}
	if len(s.ListProgress(goal.ID)) != 1 {
		t.Fatalf("expected one progress log")
	}

	s.UpsertMissionControlConfig("ws_1", MissionControlConfig{RefreshCadenceMinutes: 15})
	s.SetMissionControlWidgets("ws_1", []MissionControlWidget{
		{WidgetKey: "goals_overview", Enabled: true, Position: 2},
		{WidgetKey: "trust_scores", Enabled: true, Position: 1},
	})
	snapshot := s.MissionControlSnapshot("ws_1")
	if snapshot["goals_count"] == nil {
		t.Fatalf("expected mission control snapshot")
	}
}

func TestCreateGoalDailyRateLimit(t *testing.T) {
	t.Parallel()

	s := NewService()
	now := time.Date(2026, 2, 28, 10, 0, 0, 0, time.UTC)
	for i := 0; i < 20; i++ {
		if _, err := s.CreateGoal(Goal{WorkspaceID: "ws_rate", Title: "goal", Status: "active", Priority: "medium"}, now); err != nil {
			t.Fatalf("create goal %d: %v", i+1, err)
		}
	}
	if _, err := s.CreateGoal(Goal{WorkspaceID: "ws_rate", Title: "goal overflow", Status: "active", Priority: "medium"}, now); err == nil {
		t.Fatal("expected daily goal creation rate limit to trigger")
	}
}

func TestGoalReviewMarksStalled(t *testing.T) {
	t.Parallel()

	s := NewService()
	goal := s.UpsertGoal(Goal{WorkspaceID: "ws_stalled", Title: "Goal", Status: "active", Priority: "medium"})
	old := time.Now().UTC().Add(-10 * 24 * time.Hour)
	s.mu.Lock()
	stored := s.goals[goal.ID]
	stored.CreatedAt = old
	stored.UpdatedAt = old
	s.goals[goal.ID] = stored
	s.mu.Unlock()

	stalled := s.ReviewGoals("ws_stalled", time.Now().UTC(), 7*24*time.Hour)
	if len(stalled) != 1 {
		t.Fatalf("expected one stalled goal, got %d", len(stalled))
	}
	if stalled[0].Status != "stalled" {
		t.Fatalf("expected stalled status, got %s", stalled[0].Status)
	}
}
