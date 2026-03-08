package memory

import (
	"testing"
	"time"
)

func TestStoreAndRetrieveEpisodes(t *testing.T) {
	t.Parallel()

	er := NewEpisodicRetriever()
	now := time.Now().UTC()

	ep1, err := er.StoreEpisode("ws1", "User discussed project deadlines", now.Add(-48*time.Hour))
	if err != nil {
		t.Fatalf("store episode 1: %v", err)
	}
	if ep1.WorkspaceID != "ws1" {
		t.Fatalf("expected workspace ws1, got %s", ep1.WorkspaceID)
	}

	_, err = er.StoreEpisode("ws1", "User asked about deployment schedule", now.Add(-24*time.Hour))
	if err != nil {
		t.Fatalf("store episode 2: %v", err)
	}

	_, err = er.StoreEpisode("ws1", "User reviewed budget numbers", now)
	if err != nil {
		t.Fatalf("store episode 3: %v", err)
	}

	results := er.RetrieveRelevant("ws1", "project schedule deadlines", 2)
	if len(results) != 2 {
		t.Fatalf("expected 2 results, got %d", len(results))
	}
}

func TestRetrieveRelevantWorkspaceIsolation(t *testing.T) {
	t.Parallel()

	er := NewEpisodicRetriever()
	now := time.Now().UTC()

	_, _ = er.StoreEpisode("ws1", "alpha episode", now)
	_, _ = er.StoreEpisode("ws2", "beta episode", now)

	results := er.RetrieveRelevant("ws1", "alpha", 10)
	if len(results) != 1 {
		t.Fatalf("expected 1 result for ws1, got %d", len(results))
	}
	if results[0].Summary != "alpha episode" {
		t.Fatalf("expected alpha episode, got %s", results[0].Summary)
	}
}

func TestRankByRelevanceScoring(t *testing.T) {
	t.Parallel()

	score := RankByRelevance([]string{"project", "deadline"}, []string{"project", "deadline", "review"})
	if score != 1.0 {
		t.Fatalf("expected 1.0, got %f", score)
	}

	score = RankByRelevance([]string{"project", "budget"}, []string{"project", "deadline"})
	if score != 0.5 {
		t.Fatalf("expected 0.5, got %f", score)
	}

	score = RankByRelevance([]string{}, []string{"project"})
	if score != 0 {
		t.Fatalf("expected 0 for empty query, got %f", score)
	}
}

func TestInjectIntoContextFormatting(t *testing.T) {
	t.Parallel()

	date := time.Date(2026, 1, 15, 0, 0, 0, 0, time.UTC)
	episodes := []Episode{
		{Summary: "User discussed project plan", Date: date},
		{Summary: "User reviewed budget", Date: date.Add(24 * time.Hour)},
	}

	ctx := InjectIntoContext(episodes)
	if ctx == "" {
		t.Fatal("expected non-empty context string")
	}
	if !contains(ctx, "[Episodic Memory Context]") {
		t.Fatal("expected header in context")
	}
	if !contains(ctx, "2026-01-15") {
		t.Fatal("expected date in context")
	}

	empty := InjectIntoContext(nil)
	if empty != "" {
		t.Fatal("expected empty string for nil episodes")
	}
}

func TestStoreEpisodeValidation(t *testing.T) {
	t.Parallel()

	er := NewEpisodicRetriever()

	_, err := er.StoreEpisode("", "summary", time.Now())
	if err == nil {
		t.Fatal("expected error for empty workspace_id")
	}

	_, err = er.StoreEpisode("ws1", "", time.Now())
	if err == nil {
		t.Fatal("expected error for empty summary")
	}
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > 0 && containsSubstring(s, substr))
}

func containsSubstring(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
