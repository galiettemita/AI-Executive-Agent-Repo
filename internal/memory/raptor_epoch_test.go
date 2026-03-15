package memory

import (
	"context"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
)

type mockEpochRepo struct {
	l1Summaries []Item
	existing    *Item
}

func (r *mockEpochRepo) GetLevel1SummariesSince(_ context.Context, _ string, _ time.Time) ([]Item, error) {
	return r.l1Summaries, nil
}

func (r *mockEpochRepo) GetEpochSummaryForPeriod(_ context.Context, _ string, _, _ time.Time) (*Item, error) {
	return r.existing, nil
}

type mockEpochLLM struct {
	response string
	err      error
}

func (m *mockEpochLLM) Complete(_ context.Context, system, _ string) (string, error) {
	// Verify prompt contains period info
	if m.err != nil {
		return "", m.err
	}
	return m.response, nil
}

func makeL1Summaries(n int) []Item {
	items := make([]Item, n)
	for i := range items {
		items[i] = Item{
			ID:          uuid.Must(uuid.NewV7()),
			Body:        fmt.Sprintf("Cluster summary %d about project work", i),
			UserID:      "u1",
			WorkspaceID: "ws-1",
			MemoryType:  "semantic",
			CreatedAt:   time.Now().Add(-time.Duration(i) * 24 * time.Hour),
		}
	}
	return items
}

func TestEpochSummary_GeneratesFromL1s(t *testing.T) {
	repo := &mockEpochRepo{l1Summaries: makeL1Summaries(5)}
	llm := &mockEpochLLM{response: "This period focused on project alpha. Key outcomes: deadline met."}
	memorySvc := NewService()
	consolidator := NewRAPTORConsolidator(nil, nil, nil, memorySvc, nopRaptorLogger{})

	err := consolidator.GenerateEpochSummaries(
		context.Background(), "ws-1", DefaultEpochConfig(), repo, llm)
	if err != nil {
		t.Fatal(err)
	}

	// Verify summary was written to memory service
	found := false
	for _, item := range memorySvc.items {
		if strings.Contains(item.Body, "project alpha") {
			found = true
			break
		}
	}
	if !found {
		t.Fatal("expected epoch summary to be stored in memory service")
	}
}

func TestEpochSummary_BelowMinimum_Skipped(t *testing.T) {
	repo := &mockEpochRepo{l1Summaries: makeL1Summaries(2)}
	llm := &mockEpochLLM{response: "should not be called"}
	memorySvc := NewService()
	consolidator := NewRAPTORConsolidator(nil, nil, nil, memorySvc, nopRaptorLogger{})

	err := consolidator.GenerateEpochSummaries(
		context.Background(), "ws-1", DefaultEpochConfig(), repo, llm)
	if err != nil {
		t.Fatal(err)
	}
	if len(memorySvc.items) != 0 {
		t.Fatal("expected no epoch summary for < 3 L1 summaries")
	}
}

func TestEpochSummary_IdempotentSamePeriod(t *testing.T) {
	existingItem := &Item{ID: uuid.Must(uuid.NewV7()), Body: "existing epoch"}
	repo := &mockEpochRepo{
		l1Summaries: makeL1Summaries(5),
		existing:    existingItem,
	}
	llm := &mockEpochLLM{response: "should not be called"}
	memorySvc := NewService()
	consolidator := NewRAPTORConsolidator(nil, nil, nil, memorySvc, nopRaptorLogger{})

	err := consolidator.GenerateEpochSummaries(
		context.Background(), "ws-1", DefaultEpochConfig(), repo, llm)
	if err != nil {
		t.Fatal(err)
	}
	if len(memorySvc.items) != 0 {
		t.Fatal("expected no new summary when epoch already exists")
	}
}

func TestEpochSummary_LLMError_ReturnsError(t *testing.T) {
	repo := &mockEpochRepo{l1Summaries: makeL1Summaries(5)}
	llm := &mockEpochLLM{err: fmt.Errorf("LLM unavailable")}
	memorySvc := NewService()
	consolidator := NewRAPTORConsolidator(nil, nil, nil, memorySvc, nopRaptorLogger{})

	err := consolidator.GenerateEpochSummaries(
		context.Background(), "ws-1", DefaultEpochConfig(), repo, llm)
	if err == nil {
		t.Fatal("expected error when LLM fails")
	}
}

func TestEpochSummarizer_PromptContainsPeriod(t *testing.T) {
	// Verify the system prompt includes period dates
	capturedSystem := ""
	llm := &capturingLLM{captured: &capturedSystem, response: "Summary for the period."}
	repo := &mockEpochRepo{l1Summaries: makeL1Summaries(5)}
	memorySvc := NewService()
	consolidator := NewRAPTORConsolidator(nil, nil, nil, memorySvc, nopRaptorLogger{})

	_ = consolidator.GenerateEpochSummaries(
		context.Background(), "ws-1", DefaultEpochConfig(), repo, llm)
	if !strings.Contains(capturedSystem, "30 days") {
		t.Fatalf("expected period in system prompt, got: %s", capturedSystem[:min2(100, len(capturedSystem))])
	}
}

func TestEpochSummarizer_PromptContainsCount(t *testing.T) {
	capturedSystem := ""
	llm := &capturingLLM{captured: &capturedSystem, response: "Summary."}
	repo := &mockEpochRepo{l1Summaries: makeL1Summaries(5)}
	memorySvc := NewService()
	consolidator := NewRAPTORConsolidator(nil, nil, nil, memorySvc, nopRaptorLogger{})

	_ = consolidator.GenerateEpochSummaries(
		context.Background(), "ws-1", DefaultEpochConfig(), repo, llm)
	if !strings.Contains(capturedSystem, "5 cluster") {
		t.Fatalf("expected cluster count in prompt, got: %s", capturedSystem[:min2(100, len(capturedSystem))])
	}
}

type capturingLLM struct {
	captured *string
	response string
}

func (m *capturingLLM) Complete(_ context.Context, system, _ string) (string, error) {
	*m.captured = system
	return m.response, nil
}

func min2(a, b int) int {
	if a < b {
		return a
	}
	return b
}
