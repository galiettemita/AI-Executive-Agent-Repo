package memory

import (
	"math"
	"testing"
	"time"

	"github.com/google/uuid"
)

func TestApplyDecay(t *testing.T) {
	svc := NewMemoryDecayService()
	past := svc.now().Add(-48 * time.Hour)
	svc.AddItem(MemoryItem{
		WorkspaceID:    "ws1",
		Content:        "old fact",
		RelevanceScore: 1.0,
		Category:       "semantic",
		LastAccessedAt: past,
	})

	count, err := svc.ApplyDecay("ws1", DecayConfig{HalfLifeDays: 1, MinRetention: 0.01})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if count != 1 {
		t.Fatalf("expected 1 decayed, got %d", count)
	}
}

func TestApplyDecayExempt(t *testing.T) {
	svc := NewMemoryDecayService()
	past := svc.now().Add(-48 * time.Hour)
	svc.AddItem(MemoryItem{
		WorkspaceID:    "ws1",
		Content:        "important rule",
		RelevanceScore: 1.0,
		Category:       "rule",
		LastAccessedAt: past,
	})

	count, err := svc.ApplyDecay("ws1", DecayConfig{HalfLifeDays: 1, MinRetention: 0.01, ExemptCategories: []string{"rule"}})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if count != 0 {
		t.Fatalf("expected 0 decayed (exempt), got %d", count)
	}
}

func TestApplyDecayInvalidConfig(t *testing.T) {
	svc := NewMemoryDecayService()
	_, err := svc.ApplyDecay("ws1", DecayConfig{HalfLifeDays: 0})
	if err == nil {
		t.Fatal("expected error for zero half-life")
	}
	_, err = svc.ApplyDecay("", DecayConfig{HalfLifeDays: 1})
	if err == nil {
		t.Fatal("expected error for empty workspace")
	}
}

func TestRefreshMemory(t *testing.T) {
	svc := NewMemoryDecayService()
	item := svc.AddItem(MemoryItem{
		WorkspaceID:    "ws1",
		Content:        "refreshable",
		RelevanceScore: 0.5,
		Category:       "semantic",
	})

	err := svc.RefreshMemory(item.ID.String())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// After refresh, score should be reset to 1.0
	for _, mi := range svc.GetDecayedMemories("ws1", 2.0) {
		if mi.ID == item.ID && mi.RelevanceScore != 1.0 {
			t.Fatalf("expected score 1.0 after refresh, got %f", mi.RelevanceScore)
		}
	}
}

func TestRefreshMemoryNotFound(t *testing.T) {
	svc := NewMemoryDecayService()
	err := svc.RefreshMemory(uuid.Must(uuid.NewV7()).String())
	if err == nil {
		t.Fatal("expected error for not found")
	}
}

func TestGetDecayedMemories(t *testing.T) {
	svc := NewMemoryDecayService()
	svc.AddItem(MemoryItem{WorkspaceID: "ws1", Content: "low", RelevanceScore: 0.1, Category: "semantic"})
	svc.AddItem(MemoryItem{WorkspaceID: "ws1", Content: "high", RelevanceScore: 0.9, Category: "semantic"})

	decayed := svc.GetDecayedMemories("ws1", 0.5)
	if len(decayed) != 1 {
		t.Fatalf("expected 1 decayed item, got %d", len(decayed))
	}
}

func TestPurgeDecayed(t *testing.T) {
	svc := NewMemoryDecayService()
	svc.AddItem(MemoryItem{WorkspaceID: "ws1", Content: "low", RelevanceScore: 0.05, Category: "semantic"})
	svc.AddItem(MemoryItem{WorkspaceID: "ws1", Content: "ok", RelevanceScore: 0.8, Category: "semantic"})

	purged, err := svc.PurgeDecayed("ws1", 0.1)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if purged != 1 {
		t.Fatalf("expected 1 purged, got %d", purged)
	}

	remaining := svc.GetDecayedMemories("ws1", 2.0) // threshold above all
	if len(remaining) != 1 {
		t.Fatalf("expected 1 remaining, got %d", len(remaining))
	}
}

func TestPurgeDecayedMissingWorkspace(t *testing.T) {
	svc := NewMemoryDecayService()
	_, err := svc.PurgeDecayed("", 0.1)
	if err == nil {
		t.Fatal("expected error for empty workspace")
	}
}

// --- AdjustedHalfLife tests ---

func TestAdjustedHalfLife_ZeroRetrievals(t *testing.T) {
	hl := AdjustedHalfLife(30.0, 0)
	if math.Abs(hl-30.0) > 0.001 {
		t.Fatalf("zero retrievals: got %v, want 30.0", hl)
	}
}

func TestAdjustedHalfLife_OneRetrieval(t *testing.T) {
	hl := AdjustedHalfLife(30.0, 1)
	// 30 * (1 + ln(2)) ≈ 30 * 1.693 ≈ 50.79
	if math.Abs(hl-50.79) > 0.2 {
		t.Fatalf("1 retrieval: got %v, want ~50.79", hl)
	}
}

func TestAdjustedHalfLife_FiftyRetrievals(t *testing.T) {
	hl := AdjustedHalfLife(30.0, 50)
	if hl < 100.0 {
		t.Fatalf("50 retrievals: got %v, want > 100.0", hl)
	}
}

func TestAdjustedHalfLife_NegativeBase_SafeDefault(t *testing.T) {
	hl := AdjustedHalfLife(-5.0, 0)
	if hl <= 0 {
		t.Fatalf("negative base: got %v, expected safe positive default", hl)
	}
}

func TestDefaultBaseHalfLife_Rule(t *testing.T) {
	got := TestExportedDefaultHalfLife("rule")
	if got != 730.0 {
		t.Fatalf("rule: got %v, want 730.0", got)
	}
}

func TestDefaultBaseHalfLife_Heartbeat(t *testing.T) {
	got := TestExportedDefaultHalfLife("heartbeat")
	if math.Abs(got-0.1667) > 0.001 {
		t.Fatalf("heartbeat: got %v, want ~0.1667", got)
	}
}

func TestDecay_HighRetrievalSlowsDecay(t *testing.T) {
	itemFresh := &MemoryItem{RelevanceScore: 1.0, BaseHalfLifeDays: 30.0, RetrievalCount: 0, Confidence: 1.0}
	itemFrequent := &MemoryItem{RelevanceScore: 1.0, BaseHalfLifeDays: 30.0, RetrievalCount: 20, Confidence: 1.0}

	svc := NewTestDecayService()
	svc.TestApplyDecay(itemFresh, 60.0)
	svc.TestApplyDecay(itemFrequent, 60.0)

	if itemFrequent.RelevanceScore <= itemFresh.RelevanceScore {
		t.Fatalf("frequent item score (%v) should > fresh item score (%v) after 60 days",
			itemFrequent.RelevanceScore, itemFresh.RelevanceScore)
	}
}

func TestDecay_LowConfidenceDecaysFaster(t *testing.T) {
	itemLow := &MemoryItem{RelevanceScore: 1.0, BaseHalfLifeDays: 30.0, RetrievalCount: 0, Confidence: 0.3}
	itemHigh := &MemoryItem{RelevanceScore: 1.0, BaseHalfLifeDays: 30.0, RetrievalCount: 0, Confidence: 1.0}

	svc := NewTestDecayService()
	svc.TestApplyDecay(itemLow, 30.0)
	svc.TestApplyDecay(itemHigh, 30.0)

	if itemLow.RelevanceScore >= itemHigh.RelevanceScore {
		t.Fatalf("low confidence (%v) should decay faster than high confidence (%v)",
			itemLow.RelevanceScore, itemHigh.RelevanceScore)
	}
}
