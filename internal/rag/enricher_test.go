package rag_test

import (
	"context"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/brevio/brevio/internal/rag"
)

type mockEnricherLLM struct {
	response string
	err      error
}

func (m *mockEnricherLLM) Complete(_ context.Context, _, _ string) (string, error) {
	return m.response, m.err
}

func TestMetadataEnricher_AllFields(t *testing.T) {
	e := rag.NewMetadataChunkEnricher()
	meta := rag.DocumentMeta{
		Title: "Q3-Board-Deck", SourceType: "document",
		Date:    time.Date(2024, 10, 15, 0, 0, 0, 0, time.UTC),
		Section: "Revenue", AuthorName: "Alice Chen",
	}
	result, err := e.Enrich(context.Background(), meta, "Revenue was $4.2M")
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"Q3-Board-Deck", "document", "2024-10-15", "Revenue", "Alice Chen"} {
		if !strings.Contains(result, want) {
			t.Errorf("missing %q in result: %q", want, result)
		}
	}
	if !strings.Contains(result, "Revenue was $4.2M") {
		t.Error("original text missing from result")
	}
}

func TestMetadataEnricher_NoMetadata_ReturnsOriginal(t *testing.T) {
	e := rag.NewMetadataChunkEnricher()
	original := "Some chunk text with no metadata"
	result, err := e.Enrich(context.Background(), rag.DocumentMeta{}, original)
	if err != nil {
		t.Fatal(err)
	}
	if result != original {
		t.Fatalf("expected original text unchanged, got: %q", result)
	}
}

func TestLLMEnricher_PrependsSentence(t *testing.T) {
	llm := &mockEnricherLLM{response: "This chunk covers Q3 revenue figures"}
	e := rag.NewLLMChunkEnricher(llm)
	result, err := e.Enrich(context.Background(), rag.DocumentMeta{Title: "Q3-Deck"}, "Revenue was $4.2M")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(result, "Q3 revenue figures") {
		t.Errorf("LLM description not in result: %q", result)
	}
}

func TestLLMEnricher_LLMError_FallsBackToMetadata(t *testing.T) {
	llm := &mockEnricherLLM{err: fmt.Errorf("LLM unavailable")}
	e := rag.NewLLMChunkEnricher(llm)
	meta := rag.DocumentMeta{Title: "Q3-Deck", SourceType: "document"}
	result, err := e.Enrich(context.Background(), meta, "Revenue was $4.2M")
	if err != nil {
		t.Fatalf("fallback should not error: %v", err)
	}
	if !strings.Contains(result, "Q3-Deck") {
		t.Errorf("metadata fallback not used: %q", result)
	}
}

func TestPassthroughEnricher_Unchanged(t *testing.T) {
	e := rag.NewPassthroughChunkEnricher()
	text := "hello world"
	result, _ := e.Enrich(context.Background(), rag.DocumentMeta{}, text)
	if result != text {
		t.Fatalf("passthrough changed text: %q", result)
	}
}
