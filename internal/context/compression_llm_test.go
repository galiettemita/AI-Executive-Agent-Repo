package contextlayer

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

type mockCompressionLLM struct {
	response string
	err      error
}

func (m *mockCompressionLLM) Complete(_ context.Context, _, _ string) (string, error) {
	return m.response, m.err
}

type mockCompressionStore struct {
	mu      sync.Mutex
	records []CompressionArtifactRecord
}

func (s *mockCompressionStore) Store(_ context.Context, record CompressionArtifactRecord) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.records = append(s.records, record)
	return nil
}

type nopCompressionLogger struct{}

func (nopCompressionLogger) Info(string, ...any)  {}
func (nopCompressionLogger) Warn(string, ...any)  {}
func (nopCompressionLogger) Error(string, ...any) {}

func longTurns(n int) []Turn {
	turns := make([]Turn, n)
	for i := range turns {
		turns[i] = Turn{Role: "user", Content: strings.Repeat("word ", 400)} // ~2000 chars each
	}
	return turns
}

func TestLLMCompressor_StructuredOutput(t *testing.T) {
	llm := &mockCompressionLLM{response: `{
		"summary": "User discussed project deadlines with Alice.",
		"key_decisions": ["Deadline moved to Friday"],
		"action_items": ["Send updated timeline"],
		"entities": ["Alice Chen", "Project Alpha"],
		"open_questions": ["Budget approval pending"]
	}`}
	store := &mockCompressionStore{}
	fallback := NewConversationCompressor()
	compressor, cErr := NewLLMCompressor(llm, store, fallback, nopCompressionLogger{})
	require.NoError(t, cErr)

	result, err := compressor.CompressTurns(context.Background(), "ws-1", "conv-1",
		longTurns(5), 1, 5)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(result, "project deadlines") {
		t.Fatalf("result missing summary: %q", result)
	}
	if !strings.Contains(result, "Deadline moved") {
		t.Fatalf("result missing decisions: %q", result)
	}
	if !strings.Contains(result, "Send updated") {
		t.Fatalf("result missing actions: %q", result)
	}
}

func TestLLMCompressor_NullArraysNormalized(t *testing.T) {
	llm := &mockCompressionLLM{response: `{
		"summary": "Discussion about budget.",
		"key_decisions": null,
		"action_items": null,
		"entities": null,
		"open_questions": null
	}`}
	fallback := NewConversationCompressor()
	compressor, cErr := NewLLMCompressor(llm, nil, fallback, nopCompressionLogger{})
	require.NoError(t, cErr)

	result, err := compressor.CompressTurns(context.Background(), "ws-1", "conv-1",
		longTurns(5), 1, 5)
	if err != nil {
		t.Fatal(err)
	}
	if result == "" {
		t.Fatal("expected non-empty result")
	}
}

func TestLLMCompressor_MalformedJSON_FallsBack(t *testing.T) {
	llm := &mockCompressionLLM{response: "Sorry, I can't help with that."}
	fallback := NewConversationCompressor()
	compressor, cErr := NewLLMCompressor(llm, nil, fallback, nopCompressionLogger{})
	require.NoError(t, cErr)

	result, err := compressor.CompressTurns(context.Background(), "ws-1", "conv-1",
		longTurns(5), 1, 5)
	if err != nil {
		t.Fatal(err)
	}
	if result == "" {
		t.Fatal("expected heuristic fallback to produce output")
	}
}

func TestLLMCompressor_LLMError_FallsBack(t *testing.T) {
	llm := &mockCompressionLLM{err: fmt.Errorf("LLM unavailable")}
	fallback := NewConversationCompressor()
	compressor, cErr := NewLLMCompressor(llm, nil, fallback, nopCompressionLogger{})
	require.NoError(t, cErr)

	result, err := compressor.CompressTurns(context.Background(), "ws-1", "conv-1",
		longTurns(5), 1, 5)
	if err != nil {
		t.Fatal(err)
	}
	if result == "" {
		t.Fatal("expected heuristic fallback output")
	}
}

func TestLLMCompressor_BelowTokenBudget_NoLLMCall(t *testing.T) {
	called := false
	llm := &mockCompressionLLM{response: `{"summary":"test"}`, err: nil}
	_ = llm
	trackingLLM := &trackingCompressionLLM{called: &called}
	fallback := NewConversationCompressor()
	compressor, cErr := NewLLMCompressor(trackingLLM, nil, fallback, nopCompressionLogger{})
	require.NoError(t, cErr)

	shortTurns := []Turn{
		{Role: "user", Content: "hello"},
		{Role: "assistant", Content: "hi there"},
	}
	_, err := compressor.CompressTurns(context.Background(), "ws-1", "conv-1",
		shortTurns, 0, 1)
	if err != nil {
		t.Fatal(err)
	}
	if called {
		t.Error("LLM should NOT be called for turns below token budget")
	}
}

func TestLLMCompressor_PersistsArtifact(t *testing.T) {
	llm := &mockCompressionLLM{response: `{
		"summary": "Test summary.",
		"key_decisions": [],
		"action_items": [],
		"entities": [],
		"open_questions": []
	}`}
	store := &mockCompressionStore{}
	fallback := NewConversationCompressor()
	compressor, cErr := NewLLMCompressor(llm, store, fallback, nopCompressionLogger{})
	require.NoError(t, cErr)

	_, _ = compressor.CompressTurns(context.Background(), "ws-1", "conv-1",
		longTurns(5), 1, 5)

	// Give async goroutine time to complete
	for i := 0; i < 50; i++ {
		time.Sleep(10 * time.Millisecond)
		store.mu.Lock()
		n := len(store.records)
		store.mu.Unlock()
		if n > 0 {
			break
		}
	}

	store.mu.Lock()
	defer store.mu.Unlock()
	if len(store.records) == 0 {
		t.Error("expected artifact to be stored")
	} else {
		if store.records[0].WorkspaceID != "ws-1" {
			t.Errorf("workspace_id = %q, want ws-1", store.records[0].WorkspaceID)
		}
	}
}

func TestHeuristicCompressor_StillWorks(t *testing.T) {
	cc := NewConversationCompressor()
	turns := []Turn{
		{Role: "user", Content: "first turn"},
		{Role: "assistant", Content: "middle turn with some content"},
		{Role: "user", Content: "last turn"},
	}
	result := cc.Compress(turns, 10) // force compression
	if len(result) == 0 {
		t.Fatal("expected non-empty result from heuristic compressor")
	}
}

// Entity extraction tests

func TestExtractEntityRefs_MultiWordEntity(t *testing.T) {
	entities := extractEntityRefs("I met Alice Chen from Acme Corp today.")
	found := map[string]bool{}
	for _, e := range entities {
		found[e] = true
	}
	if !found["Alice Chen"] {
		t.Errorf("missing 'Alice Chen' in %v", entities)
	}
	if !found["Acme Corp"] {
		t.Errorf("missing 'Acme Corp' in %v", entities)
	}
}

func TestExtractEntityRefs_SentenceInitialFiltered(t *testing.T) {
	entities := extractEntityRefs("The meeting was productive.")
	for _, e := range entities {
		if e == "The" {
			t.Error("sentence-initial 'The' should not be an entity")
		}
	}
}

func TestExtractEntityRefs_AcronymKept(t *testing.T) {
	entities := extractEntityRefs("The CEO called about Q3 revenue.")
	found := map[string]bool{}
	for _, e := range entities {
		found[e] = true
	}
	if !found["CEO"] {
		t.Errorf("missing 'CEO' in %v", entities)
	}
	if !found["Q3"] {
		t.Errorf("missing 'Q3' in %v", entities)
	}
}

func TestExtractEntityRefs_NoDuplicates(t *testing.T) {
	entities := extractEntityRefs("saw CEO and then CEO again.")
	count := 0
	for _, e := range entities {
		if e == "CEO" {
			count++
		}
	}
	if count > 1 {
		t.Errorf("CEO appears %d times, expected 1", count)
	}
}

// Context assembly tests

func TestShouldCompress_TokenBased(t *testing.T) {
	// 3 turns × 5000 chars = ~3750 tokens > 2000
	turns := make([]Turn, 3)
	for i := range turns {
		turns[i] = Turn{Content: strings.Repeat("x", 5000)}
	}
	if !ShouldCompress(turns) {
		t.Error("expected compression trigger for large turns")
	}
}

func TestShouldCompress_ShortTurns_NotTriggered(t *testing.T) {
	// 40 turns × 50 chars = ~500 tokens < 2000
	turns := make([]Turn, 40)
	for i := range turns {
		turns[i] = Turn{Content: strings.Repeat("x", 50)}
	}
	if ShouldCompress(turns) {
		t.Error("40 short turns should NOT trigger compression (token-based, not turn-count)")
	}
}

func TestDynamicMemoryBudget_HighQuality(t *testing.T) {
	budget := DynamicMemoryTokenBudget(0.85, 3000)
	expected := int(3000 * 1.30)
	if budget != expected {
		t.Fatalf("high quality: got %d, want %d", budget, expected)
	}
}

func TestDynamicMemoryBudget_LowQuality(t *testing.T) {
	budget := DynamicMemoryTokenBudget(0.30, 3000)
	expected := int(3000 * 0.50)
	if budget != expected {
		t.Fatalf("low quality: got %d, want %d", budget, expected)
	}
}

func TestDynamicMemoryBudget_ClampedToFloor(t *testing.T) {
	budget := DynamicMemoryTokenBudget(0.30, 600)
	if budget < 500 {
		t.Fatalf("budget %d should be clamped to floor 500", budget)
	}
}

// helpers

type trackingCompressionLLM struct {
	called *bool
}

func (m *trackingCompressionLLM) Complete(_ context.Context, _, _ string) (string, error) {
	*m.called = true
	return `{"summary":"test","key_decisions":[],"action_items":[],"entities":[],"open_questions":[]}`, nil
}
