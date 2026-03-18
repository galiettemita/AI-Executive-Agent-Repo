package caching

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// stubSemanticLookup is a test double that records Lookup calls.
type stubSemanticLookup struct {
	lookupCalls int
	response    string
	hasHit      bool
}

func (s *stubSemanticLookup) Lookup(_ context.Context, _, _, _ string) (string, bool) {
	s.lookupCalls++
	if s.hasHit {
		return s.response, true
	}
	return "", false
}

func (s *stubSemanticLookup) Put(_ context.Context, _, _, _, _, _ string) {}

func TestLLMCacheBridge_SemanticHitPopulatesL1(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	workspaceID := "ws-test-001"
	query := "What time is my next meeting?"
	intent := ""

	expectedResponse := "Your next meeting is at 3pm."

	stub := &stubSemanticLookup{response: expectedResponse, hasHit: true}
	layers := NewService()

	bridge, err := NewLLMCacheBridge(stub, layers)
	require.NoError(t, err)

	// First call: L1/L2/L3 miss, semantic hit → populates L1/L2/L3.
	response1, ok := bridge.Lookup(ctx, workspaceID, query, intent)
	require.True(t, ok)
	assert.Equal(t, expectedResponse, response1)
	assert.Equal(t, 1, stub.lookupCalls, "semantic Lookup should be called exactly once")

	// Second call: L1 hit → semantic Lookup must NOT be called again.
	response2, ok := bridge.Lookup(ctx, workspaceID, query, intent)
	require.True(t, ok)
	assert.Equal(t, expectedResponse, response2)
	assert.Equal(t, 1, stub.lookupCalls, "semantic Lookup must NOT be called on L1 hit")
}

func TestLLMCacheBridge_TotalMiss(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	stub := &stubSemanticLookup{hasHit: false}
	layers := NewService()

	bridge, err := NewLLMCacheBridge(stub, layers)
	require.NoError(t, err)

	_, ok := bridge.Lookup(ctx, "ws-miss", "some query", "")
	assert.False(t, ok)
	assert.Equal(t, 1, stub.lookupCalls)
}

func TestLLMCacheBridge_NilDependenciesReturnError(t *testing.T) {
	t.Parallel()

	_, err := NewLLMCacheBridge(nil, nil)
	assert.Error(t, err)

	_, err = NewLLMCacheBridge(&stubSemanticLookup{}, nil)
	assert.Error(t, err)

	_, err = NewLLMCacheBridge(nil, NewService())
	assert.Error(t, err)
}

func TestLLMCacheBridge_PutThenLookup(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	workspaceID := "ws-put-test"
	query := "What is the weather?"
	response := "It is sunny and 72F."

	stub := &stubSemanticLookup{hasHit: false}
	layers := NewService()

	bridge, err := NewLLMCacheBridge(stub, layers)
	require.NoError(t, err)

	// Put an entry.
	bridge.Put(ctx, workspaceID, query, "", response, "test-model")

	// Lookup should hit L1 without touching semantic.
	got, ok := bridge.Lookup(ctx, workspaceID, query, "")
	require.True(t, ok)
	assert.Equal(t, response, got)
	assert.Equal(t, 0, stub.lookupCalls, "semantic should not be called when L1 has the entry")
}
