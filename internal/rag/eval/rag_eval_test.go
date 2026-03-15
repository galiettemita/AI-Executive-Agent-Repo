package eval

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/brevio/brevio/internal/rag"
	"github.com/brevio/brevio/internal/rag/eval/golden"
)

type mockEvalLLM struct {
	response string
	err      error
}

func (m *mockEvalLLM) Complete(_ context.Context, _, _ string) (string, error) {
	return m.response, m.err
}

type mockEvalEmbedder struct{}

func (m *mockEvalEmbedder) Embed(_ context.Context, texts []string) ([][]float32, error) {
	result := make([][]float32, len(texts))
	for i := range texts {
		v := make([]float32, 8)
		v[0] = 0.9 // all vectors point roughly the same direction → high cosine sim
		v[1] = float32(i) * 0.01
		result[i] = v
	}
	return result, nil
}

// Faithfulness tests

func TestFaithfulnessGrader_HighScore(t *testing.T) {
	llm := &mockEvalLLM{response: "0.9"}
	grader := NewFaithfulnessGrader(llm)
	chunks := []RAGChunk{{Content: "Alice reports to Bob"}}
	score, err := grader.Grade(context.Background(), "Who does Alice report to?", "Alice reports to Bob.", chunks)
	if err != nil {
		t.Fatal(err)
	}
	if score != 0.9 {
		t.Fatalf("expected 0.9, got %v", score)
	}
}

func TestFaithfulnessGrader_LLMError(t *testing.T) {
	llm := &mockEvalLLM{err: fmt.Errorf("LLM error")}
	grader := NewFaithfulnessGrader(llm)
	_, err := grader.Grade(context.Background(), "query", "answer", []RAGChunk{{Content: "text"}})
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestFaithfulnessGrader_BadOutput(t *testing.T) {
	llm := &mockEvalLLM{response: "definitely relevant"}
	grader := NewFaithfulnessGrader(llm)
	_, err := grader.Grade(context.Background(), "query", "answer", []RAGChunk{{Content: "text"}})
	if err == nil {
		t.Fatal("expected parse error")
	}
}

func TestFaithfulnessGrader_ClampedTo01(t *testing.T) {
	llm := &mockEvalLLM{response: "1.5"}
	grader := NewFaithfulnessGrader(llm)
	score, err := grader.Grade(context.Background(), "q", "a", []RAGChunk{{Content: "t"}})
	if err != nil {
		t.Fatal(err)
	}
	if score > 1.0 {
		t.Fatalf("expected clamped to 1.0, got %v", score)
	}
}

func TestFaithfulnessGrader_EmptyChunks(t *testing.T) {
	llm := &mockEvalLLM{response: "0.5"}
	grader := NewFaithfulnessGrader(llm)
	score, err := grader.Grade(context.Background(), "q", "a", nil)
	if err != nil {
		t.Fatal(err)
	}
	if score != 0 {
		t.Fatalf("expected 0 for empty chunks, got %v", score)
	}
}

// Relevance tests

func TestRelevanceGrader_HighSimilarity(t *testing.T) {
	embedder := &mockEvalEmbedder{}
	grader := NewRelevanceGrader(embedder)
	chunks := []RAGChunk{{Content: "relevant content"}}
	score, err := grader.Grade(context.Background(), "relevant query", chunks)
	if err != nil {
		t.Fatal(err)
	}
	if score < 0.5 {
		t.Fatalf("expected high similarity, got %v", score)
	}
}

func TestRelevanceGrader_EmptyChunks(t *testing.T) {
	embedder := &mockEvalEmbedder{}
	grader := NewRelevanceGrader(embedder)
	score, err := grader.Grade(context.Background(), "query", nil)
	if err != nil {
		t.Fatal(err)
	}
	if score != 0 {
		t.Fatalf("expected 0, got %v", score)
	}
}

// Threshold tests

func TestThresholdCheck_GTPass(t *testing.T) {
	tc := ThresholdCheck{Actual: 0.90, Threshold: 0.85, Operator: "gt"}
	if !tc.Check() {
		t.Fatal("expected pass")
	}
}

func TestThresholdCheck_GTFail(t *testing.T) {
	tc := ThresholdCheck{Actual: 0.80, Threshold: 0.85, Operator: "gt"}
	if tc.Check() {
		t.Fatal("expected fail")
	}
}

func TestThresholdCheck_LTPass(t *testing.T) {
	tc := ThresholdCheck{Actual: 0.03, Threshold: 0.05, Operator: "lt"}
	if !tc.Check() {
		t.Fatal("expected pass")
	}
}

func TestAllChecks_AllPass(t *testing.T) {
	checks := AllChecks(0.90, 0.85, 0.95, 0.02, 0.70, 0.98)
	allPassed := true
	for _, c := range checks {
		c.Check()
		if !c.Passed {
			allPassed = false
			t.Errorf("check %s failed: actual=%v threshold=%v", c.Name, c.Actual, c.Threshold)
		}
	}
	if !allPassed {
		t.Fatal("expected all checks to pass")
	}
}

// Golden dataset tests

func TestGoldenLoader_ValidFile(t *testing.T) {
	ds, err := golden.Load("golden/dataset.json")
	if err != nil {
		t.Fatal(err)
	}
	if len(ds.Queries) < 1 {
		t.Fatalf("expected at least 1 query, got %d", len(ds.Queries))
	}
}

func TestGoldenLoader_MissingFile(t *testing.T) {
	_, err := golden.Load("nonexistent.json")
	if err == nil {
		t.Fatal("expected error for missing file")
	}
}

func TestGoldenLoader_EmptyQueries(t *testing.T) {
	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "empty.json")
	data, _ := json.Marshal(golden.GoldenDataset{Queries: nil})
	os.WriteFile(path, data, 0644)
	_, err := golden.Load(path)
	if err == nil {
		t.Fatal("expected error for empty queries")
	}
}

func TestDatasetFilter_ByCategory(t *testing.T) {
	ds := &golden.GoldenDataset{
		Queries: []golden.GoldenQuery{
			{ID: "1", Category: "entity_lookup"},
			{ID: "2", Category: "temporal"},
			{ID: "3", Category: "entity_lookup"},
		},
	}
	filtered := ds.Filter("entity_lookup", "")
	if len(filtered) != 2 {
		t.Fatalf("expected 2 entity_lookup queries, got %d", len(filtered))
	}
}

// Unused import guard
var _ = rag.NewMockEmbeddingProvider
