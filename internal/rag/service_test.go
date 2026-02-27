package rag

import "testing"

func TestRAGServiceLifecycle(t *testing.T) {
	s := NewService()

	collection := s.UpsertCollection(Collection{
		WorkspaceID: "ws_1",
		Name:        "policies",
		Description: "workspace policy docs",
	})
	if collection.ID == "" {
		t.Fatalf("expected collection id")
	}

	if _, ingested, ok := s.Ingest(collection.ID, []string{
		"policy alpha requires confirmation for elevated actions",
		"policy beta requires recipient verification for financial transfers",
	}); !ok || ingested == 0 {
		t.Fatalf("expected successful ingestion, got ok=%v ingested=%d", ok, ingested)
	}

	results := s.Search("ws_1", "turn_1", "financial recipient verification", []string{collection.ID}, 2)
	if results.TurnID != "turn_1" {
		t.Fatalf("unexpected retrieval turn id: %#v", results)
	}
	if len(results.Results) == 0 {
		t.Fatalf("expected search results")
	}

	stored, ok := s.GetRetrieval("turn_1")
	if !ok {
		t.Fatalf("expected retrieval log for turn_1")
	}
	if len(stored.Results) == 0 {
		t.Fatalf("expected stored retrieval results")
	}

	scores := s.ListEvalScores("ws_1")
	if len(scores) != 1 {
		t.Fatalf("expected 1 eval score, got %d", len(scores))
	}
	if scores[0].Faithfulness < 0.75 || scores[0].Relevance < 0.70 {
		t.Fatalf("unexpected eval scores: %#v", scores[0])
	}
}
