package learning

import (
	"testing"
	"time"
)

func TestDetectRedundantConflicts(t *testing.T) {
	d := NewConflictDetector()
	lessons := []Lesson{
		{ID: "l1", WorkspaceID: "ws1", Title: "Always greet users", Status: "confirmed", CreatedAt: time.Now()},
		{ID: "l2", WorkspaceID: "ws1", Title: "Always greet users", Status: "confirmed", CreatedAt: time.Now()},
	}
	conflicts := d.DetectConflicts(lessons)
	if len(conflicts) != 1 {
		t.Fatalf("expected 1 conflict, got %d", len(conflicts))
	}
	if conflicts[0].ConflictType != "redundant" {
		t.Fatalf("expected redundant, got %s", conflicts[0].ConflictType)
	}
}

func TestDetectSupersededConflicts(t *testing.T) {
	d := NewConflictDetector()
	now := time.Now()
	// Titles must share a meaningful word but NOT be similar enough to trigger "redundant"
	lessons := []Lesson{
		{ID: "l1", WorkspaceID: "ws1", Title: "Use formal tone in emails", Status: "proposed", CreatedAt: now.Add(-1 * time.Hour)},
		{ID: "l2", WorkspaceID: "ws1", Title: "Apply formal standards across channels", Status: "confirmed", CreatedAt: now},
	}
	conflicts := d.DetectConflicts(lessons)
	if len(conflicts) != 1 {
		t.Fatalf("expected 1 conflict, got %d", len(conflicts))
	}
	if conflicts[0].ConflictType != "superseded" {
		t.Fatalf("expected superseded, got %s", conflicts[0].ConflictType)
	}
}

func TestDetectContradictoryConflicts(t *testing.T) {
	d := NewConflictDetector()
	// Both must be "confirmed", same workspace, share a meaningful word but NOT similar enough for "redundant"
	lessons := []Lesson{
		{ID: "l1", WorkspaceID: "ws1", Title: "always use formal language in responses", Status: "confirmed", CreatedAt: time.Now()},
		{ID: "l2", WorkspaceID: "ws1", Title: "prefer casual language when chatting", Status: "confirmed", CreatedAt: time.Now()},
	}
	conflicts := d.DetectConflicts(lessons)
	if len(conflicts) != 1 {
		t.Fatalf("expected 1 conflict, got %d", len(conflicts))
	}
	if conflicts[0].ConflictType != "contradictory" {
		t.Fatalf("expected contradictory, got %s", conflicts[0].ConflictType)
	}
}

func TestNoConflicts(t *testing.T) {
	d := NewConflictDetector()
	lessons := []Lesson{
		{ID: "l1", WorkspaceID: "ws1", Title: "Greet users", Status: "confirmed", CreatedAt: time.Now()},
		{ID: "l2", WorkspaceID: "ws1", Title: "Log all errors", Status: "confirmed", CreatedAt: time.Now()},
	}
	conflicts := d.DetectConflicts(lessons)
	if len(conflicts) != 0 {
		t.Fatalf("expected 0 conflicts, got %d", len(conflicts))
	}
}

func TestResolveConflict(t *testing.T) {
	d := NewConflictDetector()
	lessons := []Lesson{
		{ID: "l1", WorkspaceID: "ws1", Title: "Same title", Status: "confirmed", CreatedAt: time.Now()},
		{ID: "l2", WorkspaceID: "ws1", Title: "Same title", Status: "confirmed", CreatedAt: time.Now()},
	}
	conflicts := d.DetectConflicts(lessons)
	if len(conflicts) == 0 {
		t.Fatal("expected at least 1 conflict")
	}

	err := d.ResolveConflict(conflicts[0].ID, "keep_a")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	resolved, ok := d.GetConflict(conflicts[0].ID)
	if !ok {
		t.Fatal("expected conflict to be found")
	}
	if resolved.Resolution != "keep_a" {
		t.Fatalf("unexpected resolution: %s", resolved.Resolution)
	}
}

func TestResolveConflictNotFound(t *testing.T) {
	d := NewConflictDetector()
	err := d.ResolveConflict("nonexistent", "resolve")
	if err == nil {
		t.Fatal("expected error for not found")
	}
}

func TestResolveConflictEmptyResolution(t *testing.T) {
	d := NewConflictDetector()
	lessons := []Lesson{
		{ID: "l1", WorkspaceID: "ws1", Title: "dup", Status: "confirmed", CreatedAt: time.Now()},
		{ID: "l2", WorkspaceID: "ws1", Title: "dup", Status: "confirmed", CreatedAt: time.Now()},
	}
	conflicts := d.DetectConflicts(lessons)
	err := d.ResolveConflict(conflicts[0].ID, "")
	if err == nil {
		t.Fatal("expected error for empty resolution")
	}
}

func TestDetectConflictsFiltersByWorkspace(t *testing.T) {
	d := NewConflictDetector()
	// Different workspaces with related (but not identical) titles should not produce
	// contradictory or superseded conflicts. Note: identical titles still trigger
	// redundancy detection regardless of workspace.
	lessons := []Lesson{
		{ID: "l1", WorkspaceID: "ws1", Title: "Handle errors gracefully in production", Status: "confirmed", CreatedAt: time.Now()},
		{ID: "l2", WorkspaceID: "ws2", Title: "Retry errors automatically in staging", Status: "confirmed", CreatedAt: time.Now()},
	}
	conflicts := d.DetectConflicts(lessons)
	if len(conflicts) != 0 {
		t.Fatalf("expected 0 (different workspaces), got %d", len(conflicts))
	}
}
