package memory

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/brevio/brevio/internal/rag"
)

type mockConsolidationLLM struct{ response string }

func (m *mockConsolidationLLM) Complete(_ context.Context, _, _ string) (string, error) {
	return m.response, nil
}

type mockConsolidationRepo struct {
	episodes        []Item
	consolidatedIDs []uuid.UUID
}

func (r *mockConsolidationRepo) GetUnconsolidatedEpisodes(_ context.Context, _ string, limit int) ([]Item, error) {
	if limit < len(r.episodes) {
		return r.episodes[:limit], nil
	}
	return r.episodes, nil
}

func (r *mockConsolidationRepo) MarkConsolidated(_ context.Context, ids []uuid.UUID, _ uuid.UUID) error {
	r.consolidatedIDs = append(r.consolidatedIDs, ids...)
	return nil
}

func (r *mockConsolidationRepo) InsertConsolidationSummary(_ context.Context, _ uuid.UUID, _ uuid.UUID, _ int, _, _ time.Time) error {
	return nil
}

type nopRaptorLogger struct{}

func (nopRaptorLogger) Info(string, ...any)  {}
func (nopRaptorLogger) Error(string, ...any) {}

func TestRAPTOR_MinClusterSizeNotMet(t *testing.T) {
	repo := &mockConsolidationRepo{
		episodes: []Item{
			{ID: uuid.Must(uuid.NewV7()), Body: "user likes morning meetings", UserID: "u1", WorkspaceID: "ws-1", MemoryType: "episodic", CreatedAt: time.Now()},
			{ID: uuid.Must(uuid.NewV7()), Body: "user prefers coffee in the morning", UserID: "u1", WorkspaceID: "ws-1", MemoryType: "episodic", CreatedAt: time.Now()},
		},
	}
	embedder := rag.NewMockEmbeddingProvider(1536)
	llm := &mockConsolidationLLM{response: "Summary"}
	memorySvc := NewService()
	consolidator := NewRAPTORConsolidator(embedder, llm, repo, memorySvc, nopRaptorLogger{})

	err := consolidator.ConsolidateWorkspace(context.Background(), "ws-1")
	if err != nil {
		t.Fatal(err)
	}

	if len(repo.consolidatedIDs) != 0 {
		t.Fatalf("expected 0 consolidated items, got %d", len(repo.consolidatedIDs))
	}
}

func TestRAPTOR_SourcesPreservedNotDeleted(t *testing.T) {
	episodes := make([]Item, 5)
	for i := range episodes {
		episodes[i] = Item{
			ID:          uuid.Must(uuid.NewV7()),
			Body:        strings.Repeat("user had a meeting about project alpha ", 3),
			UserID:      "u1",
			WorkspaceID: "ws-1",
			MemoryType:  "episodic",
			CreatedAt:   time.Now().Add(time.Duration(i) * time.Hour),
		}
	}
	repo := &mockConsolidationRepo{episodes: episodes}
	embedder := rag.NewMockEmbeddingProvider(1536)
	llm := &mockConsolidationLLM{response: "Between 2024-01-01 and 2024-01-05, user had 5 meetings about project alpha"}
	memorySvc := NewService()
	consolidator := NewRAPTORConsolidator(embedder, llm, repo, memorySvc, nopRaptorLogger{})

	err := consolidator.ConsolidateWorkspace(context.Background(), "ws-1")
	if err != nil {
		t.Fatal(err)
	}

	// Episodes should be marked consolidated (not deleted)
	if len(repo.consolidatedIDs) == 0 {
		t.Fatal("expected episodes to be marked consolidated")
	}
}

func TestRAPTOR_SummaryContainsLLMOutput(t *testing.T) {
	episodes := make([]Item, 4)
	for i := range episodes {
		episodes[i] = Item{
			ID:          uuid.Must(uuid.NewV7()),
			Body:        "test episode content about alpha project",
			UserID:      "u1",
			WorkspaceID: "ws-1",
			MemoryType:  "episodic",
			CreatedAt:   time.Now().Add(time.Duration(i) * time.Hour),
		}
	}
	expectedSummary := "Between 2024-01-01 and 2024-01-04, multiple test episodes occurred about alpha project"
	repo := &mockConsolidationRepo{episodes: episodes}
	embedder := rag.NewMockEmbeddingProvider(1536)
	llm := &mockConsolidationLLM{response: expectedSummary}
	memorySvc := NewService()
	consolidator := NewRAPTORConsolidator(embedder, llm, repo, memorySvc, nopRaptorLogger{})

	_ = consolidator.ConsolidateWorkspace(context.Background(), "ws-1")

	// The summary item should have been written to the memory service
	// Check that the service has at least one item with the summary text
	found := false
	for _, item := range memorySvc.items {
		if strings.Contains(item.Body, "multiple test episodes") {
			found = true
			break
		}
	}
	if !found {
		t.Fatal("LLM summary not preserved in memory service items")
	}
}
