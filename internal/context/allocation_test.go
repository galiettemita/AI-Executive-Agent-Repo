package contextlayer

import "testing"

func TestAllocateFullStrategy(t *testing.T) {
	t.Parallel()

	ca := NewContextAllocator()
	sources := map[string][]string{
		"history": {"turn1", "turn2", "turn3"},
	}
	strategies := []AllocationStrategy{
		{SourceName: "history", Strategy: "full", MaxTokens: 500},
	}

	result := ca.Allocate(sources, strategies)
	if len(result["history"]) != 3 {
		t.Fatalf("expected 3 items, got %d", len(result["history"]))
	}
}

func TestAllocateRecencyStrategy(t *testing.T) {
	t.Parallel()

	ca := NewContextAllocator()
	sources := map[string][]string{
		"history": {"old1", "old2", "old3", "recent1", "recent2"},
	}
	strategies := []AllocationStrategy{
		{SourceName: "history", Strategy: "recency", MaxTokens: 200},
	}

	result := ca.Allocate(sources, strategies)
	items := result["history"]
	if len(items) != 2 {
		t.Fatalf("expected 2 items for recency with MaxTokens=200, got %d", len(items))
	}
	if items[0] != "recent1" || items[1] != "recent2" {
		t.Fatalf("expected newest items, got %v", items)
	}
}

func TestAllocateRelevanceStrategy(t *testing.T) {
	t.Parallel()

	ca := NewContextAllocator()
	sources := map[string][]string{
		"docs": {"the cat sat on the mat", "project deadline is tomorrow", "budget review notes"},
	}
	strategies := []AllocationStrategy{
		{
			SourceName: "docs",
			Strategy:   "relevance",
			MaxTokens:  200,
			Config:     map[string]string{"query": "project deadline"},
		},
	}

	result := ca.Allocate(sources, strategies)
	items := result["docs"]
	if len(items) == 0 {
		t.Fatal("expected at least one result")
	}
	if items[0] != "project deadline is tomorrow" {
		t.Fatalf("expected most relevant item first, got %s", items[0])
	}
}

func TestAllocateDiversityMMRStrategy(t *testing.T) {
	t.Parallel()

	ca := NewContextAllocator()
	sources := map[string][]string{
		"docs": {
			"project deadline is tomorrow",
			"project deadline is next week",
			"budget review for Q3",
			"team offsite planning notes",
		},
	}
	strategies := []AllocationStrategy{
		{SourceName: "docs", Strategy: "diversity_mmr", MaxTokens: 200},
	}

	result := ca.Allocate(sources, strategies)
	items := result["docs"]
	if len(items) != 2 {
		t.Fatalf("expected 2 items, got %d", len(items))
	}
	// First should be the first item; second should be diverse
	if items[0] != "project deadline is tomorrow" {
		t.Fatalf("expected first item preserved, got %s", items[0])
	}
}

func TestAllocateDefaultsForMissingStrategy(t *testing.T) {
	t.Parallel()

	ca := NewContextAllocator()
	sources := map[string][]string{
		"unknown_source": {"item1", "item2"},
	}

	result := ca.Allocate(sources, nil)
	if len(result["unknown_source"]) != 2 {
		t.Fatalf("expected 2 items with default strategy, got %d", len(result["unknown_source"]))
	}
}
