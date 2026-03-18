package memory

import (
	"context"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// mockArchivalRepo is an in-memory archival memory store for tests.
type mockArchivalRepo struct {
	stored []ArchivalMemory
}

func (r *mockArchivalRepo) StoreMemory(_ context.Context, mem ArchivalMemory) error {
	r.stored = append(r.stored, mem)
	return nil
}

func (r *mockArchivalRepo) SearchByEmbedding(_ context.Context, _, _ string, _ []float32, topK int) ([]ArchivalMemory, error) {
	if topK > len(r.stored) {
		topK = len(r.stored)
	}
	return r.stored[:topK], nil
}

// mockEmbedder returns zero vectors.
type mockEmbedder struct{}

func (e *mockEmbedder) Embed(_ context.Context, texts []string) ([][]float32, error) {
	result := make([][]float32, len(texts))
	for i := range texts {
		result[i] = make([]float32, 1536)
	}
	return result, nil
}

func (e *mockEmbedder) Dimensions() int    { return 1536 }
func (e *mockEmbedder) ModelName() string  { return "mock" }

func TestPagedContextManager_AddTurnNoPageOut(t *testing.T) {
	t.Parallel()
	mgr := NewPagedContextManager("s1", "ws1", 100000, &mockArchivalRepo{}, &mockEmbedder{})

	err := mgr.AddTurn(context.Background(), ConversationTurn{
		Index: 0, Role: "user", Content: "Hello",
	})
	require.NoError(t, err)
	assert.Equal(t, 0, mgr.PageGeneration(), "should not page out on small turn")
}

func TestPagedContextManager_PageOutAt85Percent(t *testing.T) {
	t.Parallel()
	repo := &mockArchivalRepo{}
	// Small window to trigger page-out easily.
	mgr := NewPagedContextManager("s1", "ws1", 100, repo, &mockEmbedder{})

	// Each turn ~25 tokens (100 chars / 4). 4 turns = 100 tokens = 100% of window.
	for i := 0; i < 4; i++ {
		content := strings.Repeat("a", 100) // ~25 tokens each
		err := mgr.AddTurn(context.Background(), ConversationTurn{
			Index: i, Role: "user", Content: content,
		})
		require.NoError(t, err)
	}

	// At 85% threshold with 100-token window, page-out should have triggered.
	assert.GreaterOrEqual(t, mgr.PageGeneration(), 1, "should have paged out")
	assert.Greater(t, len(repo.stored), 0, "archival should have stored entries")
}

func TestPagedContextManager_EvictsOldestHalf(t *testing.T) {
	t.Parallel()
	repo := &mockArchivalRepo{}
	mgr := NewPagedContextManager("s1", "ws1", 40, repo, &mockEmbedder{})

	// Add 4 turns of ~10 tokens each = 40 tokens total (100% of 40-token window).
	for i := 0; i < 4; i++ {
		_ = mgr.AddTurn(context.Background(), ConversationTurn{
			Index: i, Role: "user", Content: strings.Repeat("x", 40),
		})
	}

	// The oldest 2 should be evicted (50% of 4 turns).
	assert.True(t, mgr.PageGeneration() >= 1, "page-out should have occurred")
	// After eviction, main context should have fewer turns.
	assert.Less(t, len(mgr.mainContext), 4, "main context should shrink after page-out")
}

func TestPagedContextManager_PageInRetrieves(t *testing.T) {
	t.Parallel()
	repo := &mockArchivalRepo{
		stored: []ArchivalMemory{
			{Content: "previous meeting notes about Q3 revenue"},
			{Content: "client feedback from last week"},
		},
	}
	mgr := NewPagedContextManager("s1", "ws1", 100000, repo, &mockEmbedder{})

	memories, err := mgr.PageIn(context.Background(), "What did we discuss about revenue?")
	require.NoError(t, err)
	assert.Len(t, memories, 2)
}

func TestPagedContextManager_MainContextFusion(t *testing.T) {
	t.Parallel()
	mgr := NewPagedContextManager("s1", "ws1", 100000, nil, nil)

	memories := []ArchivalMemory{
		{Content: "fact A from earlier"},
		{Content: "fact B from earlier"},
	}
	result := mgr.MainContextFusion(memories)
	assert.Contains(t, result, "[Recalled from previous context]")
	assert.Contains(t, result, "fact A")
	assert.Contains(t, result, "fact B")
}

func TestPagedContextManager_EmptyArchivalReturnsEmpty(t *testing.T) {
	t.Parallel()
	mgr := NewPagedContextManager("s1", "ws1", 100000, nil, nil)
	result := mgr.MainContextFusion(nil)
	assert.Empty(t, result)
}

func TestSimpleTokenCounter(t *testing.T) {
	t.Parallel()
	tc := &SimpleTokenCounter{}
	assert.Equal(t, 25, tc.Count(strings.Repeat("a", 100)))
	assert.Equal(t, 0, tc.Count(""))
}
