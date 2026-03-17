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

func TestGetNextMilestone(t *testing.T) {
	svc := NewService()

	const goalID = "goal-dep-test"

	svc.milestones[goalID] = []Milestone{
		{
			ID:        "ms-A",
			GoalID:    goalID,
			Title:     "Milestone A",
			Status:    "todo",
			Order:     1,
			DependsOn: []string{},
		},
		{
			ID:        "ms-B",
			GoalID:    goalID,
			Title:     "Milestone B",
			Status:    "todo",
			Order:     2,
			DependsOn: []string{"ms-A"},
		},
		{
			ID:        "ms-C",
			GoalID:    goalID,
			Title:     "Milestone C",
			Status:    "todo",
			Order:     3,
			DependsOn: []string{"ms-B"},
		},
	}

	// Round 1: no deps completed → must return A
	next, err := svc.GetNextMilestone(goalID)
	if err != nil {
		t.Fatalf("round 1: unexpected error: %v", err)
	}
	if next == nil {
		t.Fatal("round 1: expected milestone A, got nil")
	}
	if next.ID != "ms-A" {
		t.Fatalf("round 1: expected ms-A, got %q", next.ID)
	}

	// Mark A completed.
	setMilestoneStatus(svc, goalID, "ms-A", "completed")

	// Round 2: A completed → must return B
	next, err = svc.GetNextMilestone(goalID)
	if err != nil {
		t.Fatalf("round 2: unexpected error: %v", err)
	}
	if next == nil {
		t.Fatal("round 2: expected milestone B, got nil")
	}
	if next.ID != "ms-B" {
		t.Fatalf("round 2: expected ms-B, got %q", next.ID)
	}

	// Mark B completed.
	setMilestoneStatus(svc, goalID, "ms-B", "completed")

	// Round 3: A+B completed → must return C
	next, err = svc.GetNextMilestone(goalID)
	if err != nil {
		t.Fatalf("round 3: unexpected error: %v", err)
	}
	if next == nil {
		t.Fatal("round 3: expected milestone C, got nil")
	}
	if next.ID != "ms-C" {
		t.Fatalf("round 3: expected ms-C, got %q", next.ID)
	}

	// Mark C completed.
	setMilestoneStatus(svc, goalID, "ms-C", "completed")

	// Round 4: all completed → must return nil, nil
	next, err = svc.GetNextMilestone(goalID)
	if err != nil {
		t.Fatalf("round 4: unexpected error: %v", err)
	}
	if next != nil {
		t.Fatalf("round 4: expected nil when all milestones complete, got %+v", next)
	}
}

func setMilestoneStatus(svc *Service, goalID, milestoneID, status string) {
	ms := svc.milestones[goalID]
	for i := range ms {
		if ms[i].ID == milestoneID {
			ms[i].Status = status
		}
	}
	svc.milestones[goalID] = ms
}
