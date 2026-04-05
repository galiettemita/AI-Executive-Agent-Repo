package temporal_reasoning

import "testing"

func TestTemporalReasoningLifecycle(t *testing.T) {
	t.Parallel()

	s := NewService()

	cfg := s.UpsertConfig("ws_1", Config{
		DefaultTimezone:           "America/New_York",
		MaxHorizonDays:            180,
		ConflictPriorityThreshold: 70,
		TravelSpeedKPH:            60,
	})
	if cfg.WorkspaceID != "ws_1" {
		t.Fatalf("expected workspace assignment, got %#v", cfg)
	}

	created, err := s.UpsertConstraint("ws_1", Constraint{
		Subject:  "focus_block",
		StartsAt: "2026-02-27T10:00:00Z",
		EndsAt:   "2026-02-27T11:00:00Z",
		Priority: 95,
	})
	if err != nil {
		t.Fatalf("unexpected constraint validation error: %v", err)
	}
	if created.ID == "" {
		t.Fatalf("expected generated id")
	}

	constraints := s.ListConstraints("ws_1")
	if len(constraints) != 1 {
		t.Fatalf("expected 1 constraint, got %d", len(constraints))
	}

	conflicts, err := s.DetectConflicts("ws_1", "2026-02-27T10:30:00Z", "2026-02-27T10:45:00Z")
	if err != nil {
		t.Fatalf("detect conflicts: %v", err)
	}
	if len(conflicts) != 1 {
		t.Fatalf("expected 1 conflict, got %d", len(conflicts))
	}
	if conflicts[0].Reason != "TEMPORAL_CONSTRAINT_VIOLATION" {
		t.Fatalf("unexpected conflict reason: %#v", conflicts[0])
	}
	if conflicts[0].Title != "focus_block" {
		t.Fatalf("expected conflict title propagation, got %#v", conflicts[0])
	}
	report, err := s.BuildConflictReport("ws_1", "2026-02-27T10:30:00Z", "2026-02-27T10:45:00Z")
	if err != nil {
		t.Fatalf("build conflict report: %v", err)
	}
	if !report.HasConflict || len(report.Conflicts) != 1 {
		t.Fatalf("expected scheduling conflict report payload, got %#v", report)
	}
	if report.ResolutionHint == "" {
		t.Fatalf("expected resolution hint in conflict report")
	}

	resolution, err := s.ResolveExpression("ws_1", "tomorrow morning", "2026-02-27", "")
	if err != nil {
		t.Fatalf("resolve tomorrow morning: %v", err)
	}
	if resolution.ResolvedDate != "2026-02-28" {
		t.Fatalf("unexpected resolved date: %#v", resolution)
	}
	if resolution.Timezone != "America/New_York" {
		t.Fatalf("expected timezone from config, got %#v", resolution)
	}
	nextWeekday, err := s.ResolveExpression("ws_1", "next monday", "2026-02-27", "")
	if err != nil {
		t.Fatalf("resolve next monday: %v", err)
	}
	if nextWeekday.ResolvedDate != "2026-03-02" {
		t.Fatalf("expected next monday resolution, got %#v", nextWeekday)
	}

	minutes := s.EstimateTravelMinutes("ws_1", "HQ", "Airport", 30)
	if minutes != 30 {
		t.Fatalf("expected 30 minutes at 60kph for 30km, got %d", minutes)
	}
	if _, ok := s.LookupTravelMinutes("ws_1", "HQ", "Airport", 30); !ok {
		t.Fatalf("expected cached travel estimate")
	}

	if !s.DeleteConstraint("ws_1", created.ID) {
		t.Fatalf("expected constraint deletion success")
	}
}

func TestTemporalReasoningRejectsInvalidConstraintAndWindow(t *testing.T) {
	t.Parallel()

	s := NewService()

	if _, err := s.UpsertConstraint("ws_1", Constraint{
		Subject:  "",
		StartsAt: "2026-02-27T11:00:00Z",
		EndsAt:   "2026-02-27T10:00:00Z",
	}); err == nil {
		t.Fatal("expected invalid constraint rejection")
	}

	if _, err := s.BuildConflictReport("ws_1", "bad", "2026-02-27T10:00:00Z"); err == nil {
		t.Fatal("expected invalid proposed window rejection")
	}
}
