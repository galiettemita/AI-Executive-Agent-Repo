package rag

import "testing"

func TestRAGServiceLifecycle(t *testing.T) {
	t.Parallel()

	s := NewService(NewMockEmbeddingProvider(1536))
	cfg := s.SetRerankerConfig("ws_1", 0.7, 0.3)
	if cfg.DenseWeight <= cfg.BM25Weight {
		t.Fatalf("expected dense weight normalization, got %+v", cfg)
	}

	collection := s.UpsertCollection(Collection{
		WorkspaceID:    "ws_1",
		Name:           "policies",
		Description:    "workspace policy docs",
		EmbeddingModel: "text-embedding-3-small",
		ChunkSize:      96,
		BM25Enabled:    true,
	})
	if collection.ID == "" {
		t.Fatalf("expected collection id")
	}
	if collection.CollectionID == "" || collection.CollectionID != collection.ID {
		t.Fatalf("expected collection_id mirror: %+v", collection)
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
	if results.RetrievalID != "turn_1" {
		t.Fatalf("unexpected retrieval id mirror: %#v", results)
	}
	if results.QueryRewrite == "" {
		t.Fatalf("expected query rewrite")
	}
	if len(results.Results) == 0 {
		t.Fatalf("expected search results")
	}
	if results.Results[0].Source == "" {
		t.Fatalf("expected result source provenance: %#v", results.Results[0])
	}

	stored, ok := s.GetRetrieval("turn_1")
	if !ok {
		t.Fatalf("expected retrieval log for turn_1")
	}
	if len(stored.Results) == 0 {
		t.Fatalf("expected stored retrieval results")
	}
	retrievalScores := s.ListRetrievalEvalScores("ws_1")
	if len(retrievalScores) == 0 {
		t.Fatalf("expected retrieval eval scores")
	}
	if retrievalScores[0].RetrievalID != "turn_1" {
		t.Fatalf("unexpected retrieval eval payload: %+v", retrievalScores[0])
	}

	scores := s.ListEvalScores("ws_1")
	if len(scores) != 1 {
		t.Fatalf("expected 1 eval score, got %d", len(scores))
	}
	if scores[0].Faithfulness < 0.75 || scores[0].Relevance < 0.70 {
		t.Fatalf("unexpected eval scores: %#v", scores[0])
	}
}
