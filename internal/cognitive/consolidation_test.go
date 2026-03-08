package cognitive

import (
	"testing"
	"time"
)

func TestRunConsolidation(t *testing.T) {
	t.Parallel()

	cs := NewConsolidationService()
	episodes := []Episode{
		{ID: "e1", Content: "deploy service to production cluster", Timestamp: time.Now(), Tags: []string{"deploy", "production"}, Importance: 0.8},
		{ID: "e2", Content: "deploy service to staging cluster", Timestamp: time.Now(), Tags: []string{"deploy", "staging"}, Importance: 0.6},
		{ID: "e3", Content: "monitor production metrics dashboard", Timestamp: time.Now(), Tags: []string{"production", "monitoring"}, Importance: 0.7},
	}

	run, err := cs.RunConsolidation("ws1", episodes)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if run.ID == "" {
		t.Fatal("expected non-empty run ID")
	}
	if run.EpisodicCount != 3 {
		t.Fatalf("expected 3 episodic, got %d", run.EpisodicCount)
	}
	if run.PatternsFound == 0 {
		t.Fatal("expected patterns to be found")
	}
	if run.CompletedAt.Before(run.StartedAt) {
		t.Fatal("expected completed_at >= started_at")
	}
}

func TestExtractPatternsFromTags(t *testing.T) {
	t.Parallel()

	cs := NewConsolidationService()
	episodes := []Episode{
		{ID: "e1", Content: "first episode content here", Tags: []string{"api", "backend"}},
		{ID: "e2", Content: "second episode content here", Tags: []string{"api", "frontend"}},
		{ID: "e3", Content: "third episode unrelated", Tags: []string{"database"}},
	}

	patterns := cs.ExtractPatterns(episodes)
	// "api" tag appears in 2 episodes, so it should be a pattern.
	found := false
	for _, p := range patterns {
		if p.Pattern == "tag:api" {
			found = true
			if p.Frequency != 2 {
				t.Fatalf("expected frequency 2, got %d", p.Frequency)
			}
		}
	}
	if !found {
		t.Fatal("expected tag:api pattern")
	}
}

func TestExtractPatternsFromContent(t *testing.T) {
	t.Parallel()

	cs := NewConsolidationService()
	episodes := []Episode{
		{ID: "e1", Content: "kubernetes deployment failed retry"},
		{ID: "e2", Content: "kubernetes service unreachable error"},
		{ID: "e3", Content: "redis cache eviction policy"},
	}

	patterns := cs.ExtractPatterns(episodes)
	// "kubernetes" appears in e1 and e2.
	foundKubernetes := false
	for _, p := range patterns {
		if p.Pattern == "word:kubernetes" {
			foundKubernetes = true
			if p.Frequency != 2 {
				t.Fatalf("expected frequency 2 for kubernetes, got %d", p.Frequency)
			}
		}
	}
	if !foundKubernetes {
		t.Fatal("expected word:kubernetes pattern")
	}
}

func TestExtractPatternsEmpty(t *testing.T) {
	t.Parallel()

	cs := NewConsolidationService()
	patterns := cs.ExtractPatterns(nil)
	if patterns != nil {
		t.Fatalf("expected nil patterns for empty input, got %d", len(patterns))
	}
}

func TestConsolidationRunsAreStored(t *testing.T) {
	t.Parallel()

	cs := NewConsolidationService()
	episodes := []Episode{
		{ID: "e1", Content: "data point alpha", Tags: []string{"tag1"}},
		{ID: "e2", Content: "data point alpha", Tags: []string{"tag1"}},
	}

	cs.RunConsolidation("ws1", episodes)
	cs.RunConsolidation("ws1", episodes)

	// Verify runs are stored (access internal state).
	cs.mu.RLock()
	runs := cs.runs["ws1"]
	cs.mu.RUnlock()
	if len(runs) != 2 {
		t.Fatalf("expected 2 runs stored, got %d", len(runs))
	}
}
