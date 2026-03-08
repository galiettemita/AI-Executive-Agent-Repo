package cognition

import (
	"testing"
)

func TestStoreCase(t *testing.T) {
	e := NewCaseReasoningEngine()
	c, err := e.StoreCase("ws1", "server crash on deploy", "rollback and restart", "resolved", 0.9, map[string]string{"env": "prod"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if c.ID == "" {
		t.Fatal("expected non-empty ID")
	}
	if c.ReuseCount != 0 {
		t.Fatalf("expected reuse count 0, got %d", c.ReuseCount)
	}
}

func TestStoreCaseValidation(t *testing.T) {
	e := NewCaseReasoningEngine()
	_, err := e.StoreCase("", "problem", "solution", "ok", 0.5, nil)
	if err == nil {
		t.Fatal("expected error for empty workspace")
	}
	_, err = e.StoreCase("ws1", "", "solution", "ok", 0.5, nil)
	if err == nil {
		t.Fatal("expected error for empty problem")
	}
	_, err = e.StoreCase("ws1", "problem", "", "ok", 0.5, nil)
	if err == nil {
		t.Fatal("expected error for empty solution")
	}
}

func TestRetrieveSimilar(t *testing.T) {
	e := NewCaseReasoningEngine()
	_, _ = e.StoreCase("ws1", "server crash during deployment", "rollback", "ok", 0.9, nil)
	_, _ = e.StoreCase("ws1", "database timeout on query", "optimize index", "ok", 0.8, nil)
	_, _ = e.StoreCase("ws1", "server error crash logs", "check logs", "ok", 0.7, nil)

	results := e.RetrieveSimilar("ws1", "server crash", 2)
	if len(results) < 1 {
		t.Fatal("expected at least 1 similar case")
	}
	if len(results) > 2 {
		t.Fatalf("expected at most 2 results, got %d", len(results))
	}
}

func TestRetrieveSimilarNoMatch(t *testing.T) {
	e := NewCaseReasoningEngine()
	_, _ = e.StoreCase("ws1", "specific issue", "fix", "ok", 0.5, nil)

	results := e.RetrieveSimilar("ws1", "completely unrelated xyz abc", 5)
	if len(results) != 0 {
		t.Fatalf("expected 0 results for unrelated query, got %d", len(results))
	}
}

func TestAdaptSolution(t *testing.T) {
	e := NewCaseReasoningEngine()
	c, _ := e.StoreCase("ws1", "memory leak in service A", "increase heap, add gc tuning", "resolved", 0.8, nil)

	adapted, err := e.AdaptSolution(c.ID, "memory leak in service B")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if adapted == "" {
		t.Fatal("expected non-empty adapted solution")
	}
}

func TestAdaptSolutionNotFound(t *testing.T) {
	e := NewCaseReasoningEngine()
	_, err := e.AdaptSolution("nonexistent", "problem")
	if err == nil {
		t.Fatal("expected error for nonexistent case")
	}
}

func TestRecordReuse(t *testing.T) {
	e := NewCaseReasoningEngine()
	c, _ := e.StoreCase("ws1", "problem", "solution", "ok", 0.5, nil)

	err := e.RecordReuse(c.ID)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	e.mu.Lock()
	stored := e.cases[c.ID]
	e.mu.Unlock()

	if stored.ReuseCount != 1 {
		t.Fatalf("expected reuse count 1, got %d", stored.ReuseCount)
	}
}

func TestRecordReuseNotFound(t *testing.T) {
	e := NewCaseReasoningEngine()
	err := e.RecordReuse("nonexistent")
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestPruneLowReuse(t *testing.T) {
	e := NewCaseReasoningEngine()
	c1, _ := e.StoreCase("ws1", "low reuse", "sol", "ok", 0.5, nil)
	c2, _ := e.StoreCase("ws1", "high reuse", "sol", "ok", 0.5, nil)
	_ = e.RecordReuse(c2.ID)
	_ = e.RecordReuse(c2.ID)
	_ = e.RecordReuse(c2.ID)

	pruned := e.PruneLowReuse(2)
	if pruned != 1 {
		t.Fatalf("expected 1 pruned, got %d", pruned)
	}
	_ = c1
}

func TestJaccardSimilarity(t *testing.T) {
	a := wordSet("the quick brown fox")
	b := wordSet("the quick red fox")
	sim := jaccardSimilarity(a, b)
	if sim <= 0 || sim >= 1 {
		t.Fatalf("expected similarity between 0 and 1, got %f", sim)
	}
}

func TestNilFeatures(t *testing.T) {
	e := NewCaseReasoningEngine()
	c, err := e.StoreCase("ws1", "problem", "solution", "ok", 0.5, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if c.Features == nil {
		t.Fatal("expected features to be initialized even if nil was passed")
	}
}
