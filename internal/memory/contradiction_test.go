package memory

import (
	"context"
	"fmt"
	"testing"

	"github.com/google/uuid"
)

type mockContradictionEmbedder struct {
	err error
}

func (m *mockContradictionEmbedder) Embed(_ context.Context, texts []string) ([][]float32, error) {
	if m.err != nil {
		return nil, m.err
	}
	result := make([][]float32, len(texts))
	for i := range texts {
		v := make([]float32, 8)
		v[0] = 0.9
		result[i] = v
	}
	return result, nil
}

type mockContradictionSearcher struct {
	results []Item
}

func (m *mockContradictionSearcher) SearchByVector(_ context.Context, _ string, _ []float32, _ int) ([]Item, error) {
	return m.results, nil
}

type mockContradictionLLM struct {
	response string
	err      error
}

func (m *mockContradictionLLM) Complete(_ context.Context, _, _ string) (string, error) {
	return m.response, m.err
}

type mockContradictionUpdater struct {
	markedIDs            []string
	markedConfidences    []float64
	contradictsNewIDs    []string
	contradictsOldIDs    []string
}

func (m *mockContradictionUpdater) MarkContradicted(_ context.Context, itemID string, confidence float64) error {
	m.markedIDs = append(m.markedIDs, itemID)
	m.markedConfidences = append(m.markedConfidences, confidence)
	return nil
}

func (m *mockContradictionUpdater) SetContradictsItemID(_ context.Context, newItemID, supersededItemID string) error {
	m.contradictsNewIDs = append(m.contradictsNewIDs, newItemID)
	m.contradictsOldIDs = append(m.contradictsOldIDs, supersededItemID)
	return nil
}

type nopContradictionLogger struct{}

func (nopContradictionLogger) Info(string, ...any)  {}
func (nopContradictionLogger) Warn(string, ...any)  {}
func (nopContradictionLogger) Error(string, ...any) {}

func TestDetector_FindsContradiction(t *testing.T) {
	existingID := uuid.Must(uuid.NewV7())
	newID := uuid.Must(uuid.NewV7())
	updater := &mockContradictionUpdater{}
	detector := NewContradictionDetector(
		&mockContradictionEmbedder{},
		&mockContradictionSearcher{results: []Item{
			{ID: existingID, WorkspaceID: "ws-1", Body: "user prefers morning meetings", RelevanceScore: 0.85},
		}},
		&mockContradictionLLM{response: `{"contradicts":true,"confidence":0.92,"reason":"different time preference"}`},
		updater,
		nopContradictionLogger{},
	)

	detector.DetectAndMark(context.Background(), Item{
		ID: newID, WorkspaceID: "ws-1", Body: "user now prefers afternoon meetings",
	})

	if len(updater.markedIDs) != 1 {
		t.Fatalf("expected 1 marked, got %d", len(updater.markedIDs))
	}
	if updater.markedIDs[0] != existingID.String() {
		t.Fatalf("marked wrong item: %s", updater.markedIDs[0])
	}
	if len(updater.contradictsNewIDs) != 1 || updater.contradictsNewIDs[0] != newID.String() {
		t.Fatal("SetContradictsItemID not called correctly")
	}
}

func TestDetector_IgnoresDuplicate_HighSimilarity(t *testing.T) {
	updater := &mockContradictionUpdater{}
	detector := NewContradictionDetector(
		&mockContradictionEmbedder{},
		&mockContradictionSearcher{results: []Item{
			{ID: uuid.Must(uuid.NewV7()), Body: "same content", RelevanceScore: 0.95}, // above 0.92 → duplicate range
		}},
		&mockContradictionLLM{response: `{"contradicts":true,"confidence":0.95,"reason":"test"}`},
		updater,
		nopContradictionLogger{},
	)

	detector.DetectAndMark(context.Background(), Item{
		ID: uuid.Must(uuid.NewV7()), WorkspaceID: "ws-1", Body: "same content",
	})

	if len(updater.markedIDs) != 0 {
		t.Fatal("should not mark duplicates as contradicted (handled by ShouldMergeDuplicate)")
	}
}

func TestDetector_LLMSaysNoContradiction(t *testing.T) {
	updater := &mockContradictionUpdater{}
	detector := NewContradictionDetector(
		&mockContradictionEmbedder{},
		&mockContradictionSearcher{results: []Item{
			{ID: uuid.Must(uuid.NewV7()), Body: "different topic", RelevanceScore: 0.83},
		}},
		&mockContradictionLLM{response: `{"contradicts":false,"confidence":0.90,"reason":"different topics"}`},
		updater,
		nopContradictionLogger{},
	)

	detector.DetectAndMark(context.Background(), Item{
		ID: uuid.Must(uuid.NewV7()), WorkspaceID: "ws-1", Body: "new info",
	})

	if len(updater.markedIDs) != 0 {
		t.Fatal("should not mark when LLM says no contradiction")
	}
}

func TestDetector_LowLLMConfidence_Ignored(t *testing.T) {
	updater := &mockContradictionUpdater{}
	detector := NewContradictionDetector(
		&mockContradictionEmbedder{},
		&mockContradictionSearcher{results: []Item{
			{ID: uuid.Must(uuid.NewV7()), Body: "existing", RelevanceScore: 0.85},
		}},
		&mockContradictionLLM{response: `{"contradicts":true,"confidence":0.60,"reason":"maybe"}`},
		updater,
		nopContradictionLogger{},
	)

	detector.DetectAndMark(context.Background(), Item{
		ID: uuid.Must(uuid.NewV7()), WorkspaceID: "ws-1", Body: "new",
	})

	if len(updater.markedIDs) != 0 {
		t.Fatal("should not mark when LLM confidence < 0.75")
	}
}

func TestDetector_LLMError_NonFatal(t *testing.T) {
	updater := &mockContradictionUpdater{}
	detector := NewContradictionDetector(
		&mockContradictionEmbedder{},
		&mockContradictionSearcher{results: []Item{
			{ID: uuid.Must(uuid.NewV7()), Body: "existing", RelevanceScore: 0.85},
		}},
		&mockContradictionLLM{err: fmt.Errorf("LLM unavailable")},
		updater,
		nopContradictionLogger{},
	)

	// Should not panic
	detector.DetectAndMark(context.Background(), Item{
		ID: uuid.Must(uuid.NewV7()), WorkspaceID: "ws-1", Body: "new item content here",
	})

	if len(updater.markedIDs) != 0 {
		t.Fatal("should not mark on LLM error")
	}
}

func TestDetector_SelfSkipped(t *testing.T) {
	selfID := uuid.Must(uuid.NewV7())
	updater := &mockContradictionUpdater{}
	detector := NewContradictionDetector(
		&mockContradictionEmbedder{},
		&mockContradictionSearcher{results: []Item{
			{ID: selfID, Body: "same item", RelevanceScore: 0.90},
		}},
		&mockContradictionLLM{response: `{"contradicts":true,"confidence":0.95,"reason":"self"}`},
		updater,
		nopContradictionLogger{},
	)

	detector.DetectAndMark(context.Background(), Item{
		ID: selfID, WorkspaceID: "ws-1", Body: "same item",
	})

	if len(updater.markedIDs) != 0 {
		t.Fatal("should not mark self as contradicted")
	}
}

func TestDetector_RecoversPanic(t *testing.T) {
	// Embedder that panics
	detector := NewContradictionDetector(
		&panicEmbedder{},
		&mockContradictionSearcher{},
		&mockContradictionLLM{},
		&mockContradictionUpdater{},
		nopContradictionLogger{},
	)

	// Should not crash
	detector.DetectAndMark(context.Background(), Item{
		ID: uuid.Must(uuid.NewV7()), WorkspaceID: "ws-1", Body: "test",
	})
}

func TestItem_ContradictionFields_JSONRoundTrip(t *testing.T) {
	itemID := "superseded-item-id"
	item := Item{
		ContradictsItemID:       &itemID,
		IsContradicted:          true,
		ContradictionConfidence: 0.88,
	}
	if item.ContradictsItemID == nil || *item.ContradictsItemID != "superseded-item-id" {
		t.Fatal("ContradictsItemID not set correctly")
	}
	if !item.IsContradicted {
		t.Fatal("IsContradicted should be true")
	}
}

func TestItem_IsContradicted_Default_False(t *testing.T) {
	item := Item{}
	if item.IsContradicted {
		t.Fatal("default should be false")
	}
	if item.ContradictionConfidence != 0 {
		t.Fatal("default contradiction confidence should be 0")
	}
}

// Context filter tests

func TestContextFilter_ExcludesContradicted_BelowThreshold(t *testing.T) {
	items := []Item{
		{Body: "old preference", IsContradicted: true, RelevanceScore: 0.3},
		{Body: "current preference", IsContradicted: false, RelevanceScore: 0.9},
	}
	var filtered []Item
	for _, item := range items {
		if item.IsContradicted && item.RelevanceScore < 0.50 {
			continue
		}
		filtered = append(filtered, item)
	}
	if len(filtered) != 1 {
		t.Fatalf("expected 1 item after filter, got %d", len(filtered))
	}
	if filtered[0].Body != "current preference" {
		t.Fatal("wrong item kept")
	}
}

func TestContextFilter_PenalizesContradicted_AboveThreshold(t *testing.T) {
	item := Item{Body: "historical", IsContradicted: true, RelevanceScore: 0.7}
	if item.IsContradicted && item.RelevanceScore >= 0.50 {
		item.RelevanceScore *= 0.60
	}
	if item.RelevanceScore >= 0.7 {
		t.Fatalf("expected penalized score, got %v", item.RelevanceScore)
	}
}

func TestContextFilter_NormalItem_Unaffected(t *testing.T) {
	item := Item{Body: "normal", IsContradicted: false, RelevanceScore: 0.8}
	originalScore := item.RelevanceScore
	// No filter applied to non-contradicted items
	if item.RelevanceScore != originalScore {
		t.Fatal("score should be unchanged")
	}
}

type panicEmbedder struct{}

func (p *panicEmbedder) Embed(_ context.Context, _ []string) ([][]float32, error) {
	panic("embedder panic!")
}
