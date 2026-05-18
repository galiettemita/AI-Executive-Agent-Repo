package workflows

import (
	"slices"
	"testing"
)

func TestMemoryStoreWorkflowHappyPath(t *testing.T) {
	t.Parallel()
	svc := NewService()
	result := svc.MemoryStoreWorkflowV1(MemoryStoreWorkflowInput{
		DocumentID:    "doc-001",
		UserID:        "user-001",
		ContentLength: 2048,
	})

	if result.WorkflowID != "mem-store-doc-001" {
		t.Fatalf("unexpected workflow id: %s", result.WorkflowID)
	}
	if result.TerminalState != MemoryStateCompleted {
		t.Fatalf("expected COMPLETED, got %s", result.TerminalState)
	}
	if result.ChunkCount < 1 {
		t.Fatalf("expected at least 1 chunk, got %d", result.ChunkCount)
	}
	wantStates := []MemoryOperationState{
		MemoryStateInit, MemoryStateChunking, MemoryStateEmbedding,
		MemoryStateIndexing, MemoryStateCompleted,
	}
	if !slices.Equal(result.States, wantStates) {
		t.Fatalf("unexpected states: %v", result.States)
	}
}

func TestMemoryStoreEmbedError(t *testing.T) {
	t.Parallel()
	svc := NewService()
	result := svc.MemoryStoreWorkflowV1(MemoryStoreWorkflowInput{
		DocumentID: "doc-002",
		EmbedError: true,
	})

	if result.TerminalState != MemoryStateFailed {
		t.Fatalf("expected FAILED on embed error, got %s", result.TerminalState)
	}
}

func TestMemoryRecallWorkflowHappyPath(t *testing.T) {
	t.Parallel()
	svc := NewService()
	result := svc.MemoryRecallWorkflowV1(MemoryRecallWorkflowInput{
		QueryID: "query-001",
		UserID:  "user-001",
	})

	if result.WorkflowID != "mem-recall-query-001" {
		t.Fatalf("unexpected workflow id: %s", result.WorkflowID)
	}
	if result.TerminalState != MemoryStateCompleted {
		t.Fatalf("expected COMPLETED, got %s", result.TerminalState)
	}
}

func TestMemoryRecallSearchFallback(t *testing.T) {
	t.Parallel()
	svc := NewService()
	result := svc.MemoryRecallWorkflowV1(MemoryRecallWorkflowInput{
		QueryID:     "query-002",
		SearchError: true,
	})

	if result.TerminalState != MemoryStateCompleted {
		t.Fatalf("expected COMPLETED with fallback, got %s", result.TerminalState)
	}
	if !slices.Contains(result.Fallbacks, "keyword_search") {
		t.Fatalf("expected keyword_search fallback: %v", result.Fallbacks)
	}
}

func TestMemoryStoreChunkCalculation(t *testing.T) {
	t.Parallel()
	svc := NewService()
	result := svc.MemoryStoreWorkflowV1(MemoryStoreWorkflowInput{
		DocumentID:    "doc-003",
		ContentLength: 4096,
	})

	expectedChunks := 4096 / 512
	if result.ChunkCount != expectedChunks {
		t.Fatalf("expected %d chunks, got %d", expectedChunks, result.ChunkCount)
	}
}

func TestMemoryStoreMinimumOneChunk(t *testing.T) {
	t.Parallel()
	svc := NewService()
	result := svc.MemoryStoreWorkflowV1(MemoryStoreWorkflowInput{
		DocumentID:    "doc-004",
		ContentLength: 0,
	})

	if result.ChunkCount != 1 {
		t.Fatalf("expected minimum 1 chunk for empty content, got %d", result.ChunkCount)
	}
}

func TestMemoryStoreChunkErrorFallback(t *testing.T) {
	t.Parallel()
	svc := NewService()
	result := svc.MemoryStoreWorkflowV1(MemoryStoreWorkflowInput{
		DocumentID: "doc-005",
		ChunkError: true,
	})

	if result.TerminalState != MemoryStateCompleted {
		t.Fatalf("expected COMPLETED with chunk fallback, got %s", result.TerminalState)
	}
	if !slices.Contains(result.Fallbacks, "single_chunk") {
		t.Fatalf("expected single_chunk fallback: %v", result.Fallbacks)
	}
}

func TestMemoryStoreIndexErrorFallback(t *testing.T) {
	t.Parallel()
	svc := NewService()
	result := svc.MemoryStoreWorkflowV1(MemoryStoreWorkflowInput{
		DocumentID: "doc-006",
		IndexError: true,
	})

	if result.TerminalState != MemoryStateCompleted {
		t.Fatalf("expected COMPLETED with index retry, got %s", result.TerminalState)
	}
	if !slices.Contains(result.Fallbacks, "retry_index") {
		t.Fatalf("expected retry_index fallback: %v", result.Fallbacks)
	}
}

func TestMemoryRecallRankErrorFallback(t *testing.T) {
	t.Parallel()
	svc := NewService()
	result := svc.MemoryRecallWorkflowV1(MemoryRecallWorkflowInput{
		QueryID:   "query-003",
		RankError: true,
	})

	if result.TerminalState != MemoryStateCompleted {
		t.Fatalf("expected COMPLETED with rank fallback, got %s", result.TerminalState)
	}
	if !slices.Contains(result.Fallbacks, "recency_rank") {
		t.Fatalf("expected recency_rank fallback: %v", result.Fallbacks)
	}
}

func TestMemoryWorkflowIDTrimming(t *testing.T) {
	t.Parallel()
	storeID := MemoryStoreWorkflowID("  doc-padded  ")
	if storeID != "mem-store-doc-padded" {
		t.Fatalf("expected trimmed store ID, got %s", storeID)
	}
	recallID := MemoryRecallWorkflowID("  query-padded  ")
	if recallID != "mem-recall-query-padded" {
		t.Fatalf("expected trimmed recall ID, got %s", recallID)
	}
}
