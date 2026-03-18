package workingmemory_test

import (
	"context"
	"strings"
	"sync"
	"testing"
	"time"

	wm "github.com/brevio/brevio/internal/workingmemory"
	"github.com/stretchr/testify/require"
)

// --- in-process mock Redis ---

type mockRedis struct {
	mu   sync.Mutex
	data map[string][]byte
}

func newMockRedis() *mockRedis { return &mockRedis{data: make(map[string][]byte)} }

func (m *mockRedis) Set(_ context.Context, key string, value []byte, _ time.Duration) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.data[key] = value
	return nil
}

func (m *mockRedis) Get(_ context.Context, key string) ([]byte, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	v, ok := m.data[key]
	if !ok {
		return nil, nil
	}
	return v, nil
}

func (m *mockRedis) Del(_ context.Context, keys ...string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	for _, k := range keys {
		delete(m.data, k)
	}
	return nil
}

func (m *mockRedis) Scan(_ context.Context, pattern string) ([]string, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	prefix := strings.TrimSuffix(pattern, "*")
	var out []string
	for k := range m.data {
		if strings.HasPrefix(k, prefix) {
			out = append(out, k)
		}
	}
	return out, nil
}

func (m *mockRedis) Expire(_ context.Context, _ string, _ time.Duration) error { return nil }

// --- nop logger ---

type nopLogger struct{}

func (nopLogger) Info(string, ...any)  {}
func (nopLogger) Warn(string, ...any)  {}
func (nopLogger) Error(string, ...any) {}

// --- helper ---

func newTestService(t *testing.T) (*wm.Service, *mockRedis) {
	t.Helper()
	redis := newMockRedis()
	repo, err := wm.NewRepository(redis)
	require.NoError(t, err)
	svc := wm.NewService(repo, nopLogger{})
	return svc, redis
}

func TestGetOrCreate_CreatesNewItem(t *testing.T) {
	svc, _ := newTestService(t)
	ctx := context.Background()

	item, err := svc.GetOrCreate(ctx, "ws-1", "task-1", "user-1")
	if err != nil {
		t.Fatal(err)
	}
	if item == nil {
		t.Fatal("expected non-nil item")
	}
	if item.TaskID != "task-1" {
		t.Fatalf("TaskID = %q, want task-1", item.TaskID)
	}
	if item.ScratchPad == nil {
		t.Fatal("ScratchPad must not be nil")
	}
}

func TestGetOrCreate_ReturnsExistingItem(t *testing.T) {
	svc, _ := newTestService(t)
	ctx := context.Background()

	first, _ := svc.GetOrCreate(ctx, "ws-1", "task-2", "user-1")
	_ = svc.MergeScratchPad(ctx, "ws-1", "task-2", map[string]any{"key": "value"})

	second, err := svc.GetOrCreate(ctx, "ws-1", "task-2", "user-1")
	if err != nil {
		t.Fatal(err)
	}
	if second.CreatedAt != first.CreatedAt {
		t.Fatal("second call must return existing item")
	}
	if second.ScratchPad["key"] != "value" {
		t.Fatal("scratch pad not preserved")
	}
}

func TestMergeScratchPad_MergesKeys(t *testing.T) {
	svc, _ := newTestService(t)
	ctx := context.Background()

	_, _ = svc.GetOrCreate(ctx, "ws-1", "task-3", "user-1")
	_ = svc.MergeScratchPad(ctx, "ws-1", "task-3", map[string]any{"a": "1", "b": "2"})
	_ = svc.MergeScratchPad(ctx, "ws-1", "task-3", map[string]any{"b": "updated", "c": "3"})

	item, _ := svc.GetOrCreate(ctx, "ws-1", "task-3", "user-1")
	if item.ScratchPad["a"] != "1" {
		t.Fatal("key 'a' not preserved")
	}
	if item.ScratchPad["b"] != "updated" {
		t.Fatal("key 'b' not updated")
	}
	if item.ScratchPad["c"] != "3" {
		t.Fatal("key 'c' not added")
	}
}

func TestComplete_EvictsItem(t *testing.T) {
	svc, _ := newTestService(t)
	ctx := context.Background()

	_, _ = svc.GetOrCreate(ctx, "ws-1", "task-4", "user-1")
	svc.Complete(ctx, "ws-1", "task-4")

	item, _ := svc.GetOrCreate(ctx, "ws-1", "task-4", "user-1")
	if item == nil {
		t.Fatal("expected fresh item after eviction")
	}
}

func TestBindWorkflow_ExtendsTTL(t *testing.T) {
	svc, _ := newTestService(t)
	ctx := context.Background()

	_, _ = svc.GetOrCreate(ctx, "ws-1", "task-5", "user-1")
	err := svc.BindWorkflow(ctx, "ws-1", "task-5", "wf-abc", "run-123")
	if err != nil {
		t.Fatal(err)
	}

	item, _ := svc.GetOrCreate(ctx, "ws-1", "task-5", "user-1")
	if item.WorkflowID != "wf-abc" {
		t.Fatalf("WorkflowID = %q, want wf-abc", item.WorkflowID)
	}
	if item.TTL != wm.WorkflowTTL {
		t.Fatalf("TTL = %v, want %v", item.TTL, wm.WorkflowTTL)
	}
}

func TestBuildContextSnippet_Empty(t *testing.T) {
	svc, _ := newTestService(t)
	ctx := context.Background()

	_, _ = svc.GetOrCreate(ctx, "ws-1", "task-6", "user-1")
	snippet, err := svc.BuildContextSnippet(ctx, "ws-1", "task-6")
	if err != nil {
		t.Fatal(err)
	}
	if snippet != "" {
		t.Fatalf("expected empty snippet for item with no state, got: %q", snippet)
	}
}

func TestBuildContextSnippet_WithStage(t *testing.T) {
	svc, _ := newTestService(t)
	ctx := context.Background()

	_, _ = svc.GetOrCreate(ctx, "ws-1", "task-7", "user-1")
	_ = svc.SetStage(ctx, "ws-1", "task-7", "awaiting_approval")

	snippet, err := svc.BuildContextSnippet(ctx, "ws-1", "task-7")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(snippet, "awaiting_approval") {
		t.Fatalf("snippet missing stage: %q", snippet)
	}
}

func TestBuildContextSnippet_MissingTask(t *testing.T) {
	svc, _ := newTestService(t)
	ctx := context.Background()

	snippet, err := svc.BuildContextSnippet(ctx, "ws-1", "nonexistent-task")
	if err != nil {
		t.Fatal(err)
	}
	if snippet != "" {
		t.Fatalf("expected empty for missing task, got %q", snippet)
	}
}

func TestAddPendingToolCall_AppearsInSnippet(t *testing.T) {
	svc, _ := newTestService(t)
	ctx := context.Background()

	_, _ = svc.GetOrCreate(ctx, "ws-1", "task-8", "user-1")
	_ = svc.AddPendingToolCall(ctx, "ws-1", "task-8", wm.PendingToolCall{
		ToolName: "send_email", ToolCallID: "tc-1",
	})

	snippet, _ := svc.BuildContextSnippet(ctx, "ws-1", "task-8")
	if !strings.Contains(snippet, "send_email") {
		t.Fatalf("snippet missing tool name: %q", snippet)
	}
}

func TestResolvePendingToolCall_RemovesFromSnippet(t *testing.T) {
	svc, _ := newTestService(t)
	ctx := context.Background()

	_, _ = svc.GetOrCreate(ctx, "ws-1", "task-9", "user-1")
	_ = svc.AddPendingToolCall(ctx, "ws-1", "task-9", wm.PendingToolCall{
		ToolName: "send_email", ToolCallID: "tc-1",
	})
	_ = svc.ResolvePendingToolCall(ctx, "ws-1", "task-9", "tc-1")

	item, _ := svc.GetOrCreate(ctx, "ws-1", "task-9", "user-1")
	if len(item.PendingToolCalls) != 0 {
		t.Fatalf("expected empty PendingToolCalls after resolve, got %d", len(item.PendingToolCalls))
	}
}
