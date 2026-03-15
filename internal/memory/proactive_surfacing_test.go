package memory

import (
	"testing"
	"time"
)

func TestProactiveSurfacingAddAndFind(t *testing.T) {
	t.Parallel()

	ps := NewProactiveSurfacingService(nil)
	_, err := ps.AddMemory("ws1", "project deadline is next Friday")
	if err != nil {
		t.Fatalf("add memory: %v", err)
	}
	_, err = ps.AddMemory("ws1", "budget review scheduled for Monday")
	if err != nil {
		t.Fatalf("add memory: %v", err)
	}

	candidates := ps.FindRelevantMemories("ws1", "project deadline", 5)
	if len(candidates) == 0 {
		t.Fatal("expected at least one candidate")
	}
	if candidates[0].RelevanceScore <= 0 {
		t.Fatal("expected positive relevance score")
	}
}

func TestProactiveSurfacingWorkspaceIsolation(t *testing.T) {
	t.Parallel()

	ps := NewProactiveSurfacingService(nil)
	_, _ = ps.AddMemory("ws1", "alpha memory content")
	_, _ = ps.AddMemory("ws2", "beta memory content")

	candidates := ps.FindRelevantMemories("ws1", "alpha", 10)
	for _, c := range candidates {
		if c.Content == "beta memory content" {
			t.Fatal("should not return memories from other workspaces")
		}
	}
}

func TestProactiveSurfacingEmptyResults(t *testing.T) {
	t.Parallel()

	ps := NewProactiveSurfacingService(nil)
	candidates := ps.FindRelevantMemories("ws_empty", "anything", 5)
	if len(candidates) != 0 {
		t.Fatalf("expected 0 candidates for empty workspace, got %d", len(candidates))
	}
}

func TestProactiveSurfacingLimit(t *testing.T) {
	t.Parallel()

	ps := NewProactiveSurfacingService(nil)
	for i := 0; i < 10; i++ {
		_, _ = ps.AddMemory("ws1", "memory about project planning tasks")
	}

	candidates := ps.FindRelevantMemories("ws1", "project planning", 3)
	if len(candidates) > 3 {
		t.Fatalf("expected at most 3 candidates, got %d", len(candidates))
	}
}

func TestComputeRecencyScore(t *testing.T) {
	t.Parallel()

	now := time.Now().UTC()

	score := computeRecencyScore(now, now)
	if score != 1.0 {
		t.Fatalf("expected 1.0 for current time, got %f", score)
	}

	old := now.Add(-8 * 24 * time.Hour)
	score = computeRecencyScore(now, old)
	if score != 0 {
		t.Fatalf("expected 0 for 8-day-old memory, got %f", score)
	}
}
