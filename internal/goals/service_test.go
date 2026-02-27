package goals

import "testing"

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
