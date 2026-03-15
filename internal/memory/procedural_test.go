package memory

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/brevio/brevio/internal/rag"
)

func TestProceduralSearch_ActionVerbBoost(t *testing.T) {
	embedder := rag.NewMockEmbeddingProvider(1536)
	svc := NewProceduralMemoryService(embedder)

	procItem := Item{
		ID: uuid.Must(uuid.NewV7()), WorkspaceID: "ws-1",
		Body: "user always books meetings via Calendly", MemoryType: "procedural",
		CreatedAt: time.Now(),
	}
	// Embed the item
	if embs, err := embedder.Embed(context.Background(), []string{procItem.Body}); err == nil && len(embs) > 0 {
		procItem.Embedding = embs[0]
	}
	svc.AddProcedural(procItem)

	results, err := svc.SearchProcedural(context.Background(), "ws-1", "schedule a meeting", 5)
	if err != nil {
		t.Fatal(err)
	}
	if len(results) == 0 {
		t.Fatal("expected at least one procedural result")
	}
}

func TestProceduralSearch_NoActionVerb(t *testing.T) {
	mult := ActionVerbMultiplier("what happened last week")
	if mult != 1.0 {
		t.Fatalf("expected 1.0 for no action verb, got %v", mult)
	}
}

func TestActionVerbMultiplier_ScheduleReturns1_3(t *testing.T) {
	mult := ActionVerbMultiplier("schedule a call")
	if mult != 1.3 {
		t.Fatalf("expected 1.3, got %v", mult)
	}
}

func TestProceduralSearch_EmptyWorkspace(t *testing.T) {
	svc := NewProceduralMemoryService(nil)
	results, err := svc.SearchProcedural(context.Background(), "ws-empty", "schedule meeting", 5)
	if err != nil {
		t.Fatal(err)
	}
	if len(results) != 0 {
		t.Fatalf("expected 0 results for empty workspace, got %d", len(results))
	}
}
