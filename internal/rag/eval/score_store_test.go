package eval

import (
	"context"
	"path/filepath"
	"testing"
	"time"
)

func TestScoreStore_SaveAndLoad(t *testing.T) {
	dir := t.TempDir()
	store := NewScoreStore(filepath.Join(dir, "baseline.json"))
	ctx := context.Background()

	scores := BaselineScores{
		RAGFaithfulness: 0.91,
		RAGRelevance:    0.87,
		RecordedAt:      time.Now().UTC(),
		EvalVersion:     "1.0",
	}
	if err := store.Save(ctx, scores); err != nil {
		t.Fatalf("Save: %v", err)
	}

	loaded, err := store.Load(ctx)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if loaded == nil {
		t.Fatal("expected non-nil scores")
	}
	if loaded.RAGFaithfulness != 0.91 {
		t.Fatalf("faithfulness: got %.3f, want 0.91", loaded.RAGFaithfulness)
	}
	if loaded.RAGRelevance != 0.87 {
		t.Fatalf("relevance: got %.3f, want 0.87", loaded.RAGRelevance)
	}
}

func TestScoreStore_Load_NotExist_ReturnsNilNil(t *testing.T) {
	store := NewScoreStore("/tmp/nonexistent_baseline_12345.json")
	loaded, err := store.Load(context.Background())
	if err != nil {
		t.Fatalf("expected nil error, got: %v", err)
	}
	if loaded != nil {
		t.Fatal("expected nil scores for missing file")
	}
}

func TestScoreStore_MetSingleVectorTarget_AboveThreshold(t *testing.T) {
	dir := t.TempDir()
	store := NewScoreStore(filepath.Join(dir, "baseline.json"))
	ctx := context.Background()

	_ = store.Save(ctx, BaselineScores{RAGFaithfulness: 0.91, RAGRelevance: 0.83})

	met, err := store.MetSingleVectorTarget(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if !met {
		t.Fatal("expected true when both thresholds exceeded")
	}
}

func TestScoreStore_MetSingleVectorTarget_BelowThreshold(t *testing.T) {
	dir := t.TempDir()
	store := NewScoreStore(filepath.Join(dir, "baseline.json"))
	ctx := context.Background()

	_ = store.Save(ctx, BaselineScores{RAGFaithfulness: 0.80, RAGRelevance: 0.83})

	met, err := store.MetSingleVectorTarget(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if met {
		t.Fatal("expected false when faithfulness below threshold")
	}
}

func TestScoreStore_MetSingleVectorTarget_NoBaseline_ReturnsFalse(t *testing.T) {
	store := NewScoreStore("/tmp/nonexistent_9999.json")
	met, err := store.MetSingleVectorTarget(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if met {
		t.Fatal("expected false when no baseline exists")
	}
}
