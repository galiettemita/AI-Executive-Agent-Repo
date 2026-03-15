package memory

import (
	"context"
	"testing"

	"github.com/google/uuid"

	"github.com/brevio/brevio/internal/rag"
)

type mockLinkRepo struct {
	links     []MemoryLink
	itemsByID map[string][]Item
}

func (r *mockLinkRepo) CreateLink(_ context.Context, link MemoryLink) error {
	r.links = append(r.links, link)
	return nil
}

func (r *mockLinkRepo) GetLinkedItemIDs(_ context.Context, _, itemID string) ([]string, error) {
	var ids []string
	for _, l := range r.links {
		if l.SourceID == itemID {
			ids = append(ids, l.TargetID)
		}
		if l.TargetID == itemID {
			ids = append(ids, l.SourceID)
		}
	}
	return ids, nil
}

func (r *mockLinkRepo) GetItemsByIDs(_ context.Context, _ string, ids []string) ([]Item, error) {
	var result []Item
	for _, id := range ids {
		for _, items := range r.itemsByID {
			for _, item := range items {
				if item.ID.String() == id {
					result = append(result, item)
				}
			}
		}
	}
	return result, nil
}

type mockSearcher struct{ results []Item }

func (m *mockSearcher) SearchByVector(_ context.Context, _ string, _ []float32, _ int) ([]Item, error) {
	return m.results, nil
}

type nopLinkLogger struct{}

func (nopLinkLogger) Info(string, ...any)  {}
func (nopLinkLogger) Warn(string, ...any)  {}
func (nopLinkLogger) Error(string, ...any) {}

func TestAutoLink_NoSelfLink(t *testing.T) {
	embedder := rag.NewMockEmbeddingProvider(1536)
	repo := &mockLinkRepo{}
	newItem := Item{ID: uuid.Must(uuid.NewV7()), WorkspaceID: "ws-1", Body: "user prefers morning meetings"}
	searcher := &mockSearcher{results: []Item{newItem}}
	svc := NewLinkService(repo, searcher, embedder, nopLinkLogger{})

	svc.AutoLink(context.Background(), newItem)

	for _, link := range repo.links {
		if link.SourceID == link.TargetID {
			t.Fatal("self-link was created — must never happen")
		}
	}
}

func TestAutoLink_CreatesLinkForSimilarItem(t *testing.T) {
	embedder := rag.NewMockEmbeddingProvider(1536)
	repo := &mockLinkRepo{}
	newItem := Item{ID: uuid.Must(uuid.NewV7()), WorkspaceID: "ws-1", Body: "hello world"}
	existing := Item{ID: uuid.Must(uuid.NewV7()), WorkspaceID: "ws-1", Body: "hello world"}
	searcher := &mockSearcher{results: []Item{existing}}
	svc := NewLinkService(repo, searcher, embedder, nopLinkLogger{})

	svc.AutoLink(context.Background(), newItem)

	if len(repo.links) == 0 {
		t.Fatal("expected at least one link to be created for identical text")
	}
}

func TestExpandWithLinks_AddsLinkedItems(t *testing.T) {
	embedder := rag.NewMockEmbeddingProvider(1536)
	primaryID := uuid.Must(uuid.NewV7())
	linkedID := uuid.Must(uuid.NewV7())
	primary := Item{ID: primaryID, WorkspaceID: "ws-1", RelevanceScore: 0.9}
	linked := Item{ID: linkedID, WorkspaceID: "ws-1", RelevanceScore: 0.8}
	repo := &mockLinkRepo{
		links:     []MemoryLink{{SourceID: primaryID.String(), TargetID: linkedID.String(), Strength: 0.9}},
		itemsByID: map[string][]Item{"ws-1": {linked}},
	}
	svc := NewLinkService(repo, &mockSearcher{}, embedder, nopLinkLogger{})

	expanded, err := svc.ExpandWithLinks(context.Background(), "ws-1", []Item{primary}, 5)
	if err != nil {
		t.Fatal(err)
	}
	if len(expanded) != 2 {
		t.Fatalf("expected 2 items, got %d", len(expanded))
	}

	linkedResult := expanded[1]
	if linkedResult.RelevanceScore >= 0.8 {
		t.Fatalf("linked item score should be <0.8 (70%% penalty applied), got %v", linkedResult.RelevanceScore)
	}
}

func TestExpandWithLinks_NoDuplicates(t *testing.T) {
	embedder := rag.NewMockEmbeddingProvider(1536)
	primaryID := uuid.Must(uuid.NewV7())
	primary := Item{ID: primaryID, WorkspaceID: "ws-1"}
	repo := &mockLinkRepo{
		links:     []MemoryLink{{SourceID: primaryID.String(), TargetID: primaryID.String()}},
		itemsByID: map[string][]Item{"ws-1": {primary}},
	}
	svc := NewLinkService(repo, &mockSearcher{}, embedder, nopLinkLogger{})

	expanded, _ := svc.ExpandWithLinks(context.Background(), "ws-1", []Item{primary}, 5)
	if len(expanded) != 1 {
		t.Fatalf("duplicate added: expected 1 item, got %d", len(expanded))
	}
}

func TestExpandWithLinks_MaxRespected(t *testing.T) {
	embedder := rag.NewMockEmbeddingProvider(1536)
	primaryID := uuid.Must(uuid.NewV7())
	primary := Item{ID: primaryID, WorkspaceID: "ws-1"}
	items := make([]Item, 10)
	links := make([]MemoryLink, 10)
	for i := range items {
		items[i] = Item{ID: uuid.Must(uuid.NewV7()), WorkspaceID: "ws-1"}
		links[i] = MemoryLink{SourceID: primaryID.String(), TargetID: items[i].ID.String()}
	}
	repo := &mockLinkRepo{links: links, itemsByID: map[string][]Item{"ws-1": items}}
	svc := NewLinkService(repo, &mockSearcher{}, embedder, nopLinkLogger{})

	expanded, _ := svc.ExpandWithLinks(context.Background(), "ws-1", []Item{primary}, 3)
	if len(expanded) > 4 {
		t.Fatalf("maxLinked=3 violated: got %d total items", len(expanded))
	}
}
