package cognition

import (
	"testing"
	"time"
)

func TestRunConsolidation(t *testing.T) {
	s := NewConsolidationService()
	episodes := []Episode{
		{ID: "e1", Content: "deployed service alpha", Tags: []string{"deploy"}, ImportanceScore: 0.8, Timestamp: time.Now()},
		{ID: "e2", Content: "deployed service beta", Tags: []string{"deploy"}, ImportanceScore: 0.7, Timestamp: time.Now()},
	}

	run, err := s.RunConsolidation("ws1", episodes)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if run.EpisodicProcessed != 2 {
		t.Fatalf("expected 2 processed, got %d", run.EpisodicProcessed)
	}
	if run.PatternsFound < 1 {
		t.Fatalf("expected at least 1 pattern, got %d", run.PatternsFound)
	}
}

func TestRunConsolidationEmpty(t *testing.T) {
	s := NewConsolidationService()
	_, err := s.RunConsolidation("ws1", nil)
	if err == nil {
		t.Fatal("expected error for empty episodes")
	}
}

func TestRunConsolidationEmptyWorkspace(t *testing.T) {
	s := NewConsolidationService()
	_, err := s.RunConsolidation("", []Episode{{ID: "e1", Content: "test"}})
	if err == nil {
		t.Fatal("expected error for empty workspace")
	}
}

func TestExtractPatternsTags(t *testing.T) {
	s := NewConsolidationService()
	episodes := []Episode{
		{ID: "e1", Content: "first incident", Tags: []string{"incident", "prod"}},
		{ID: "e2", Content: "second incident", Tags: []string{"incident"}},
		{ID: "e3", Content: "third report", Tags: []string{"report", "prod"}},
	}

	patterns := s.ExtractPatterns(episodes)
	found := false
	for _, p := range patterns {
		if p.Pattern == "tag:incident" && p.Frequency == 2 {
			found = true
		}
	}
	if !found {
		t.Fatal("expected to find 'tag:incident' pattern with frequency 2")
	}
}

func TestExtractPatternsWords(t *testing.T) {
	s := NewConsolidationService()
	episodes := []Episode{
		{ID: "e1", Content: "the server crashed today"},
		{ID: "e2", Content: "server was slow yesterday"},
		{ID: "e3", Content: "server needs reboot now"},
	}

	patterns := s.ExtractPatterns(episodes)
	found := false
	for _, p := range patterns {
		if p.Pattern == "word:server" {
			found = true
		}
	}
	if !found {
		t.Fatal("expected to find 'word:server' pattern")
	}
}

func TestPromoteToSemantic(t *testing.T) {
	s := NewConsolidationService()
	pattern := SemanticPattern{Pattern: "tag:deploy", Frequency: 5, Confidence: 0.8, SourceEpisodes: []string{"e1"}}
	err := s.PromoteToSemantic(pattern)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestPromoteToSemanticDuplicate(t *testing.T) {
	s := NewConsolidationService()
	pattern := SemanticPattern{Pattern: "tag:deploy", Frequency: 5, Confidence: 0.8}
	_ = s.PromoteToSemantic(pattern)

	err := s.PromoteToSemantic(pattern)
	if err == nil {
		t.Fatal("expected error for duplicate promotion")
	}
}

func TestPromoteToSemanticEmpty(t *testing.T) {
	s := NewConsolidationService()
	err := s.PromoteToSemantic(SemanticPattern{Pattern: ""})
	if err == nil {
		t.Fatal("expected error for empty pattern")
	}
}
