package eval

import (
	"context"
	"fmt"
	"testing"
	"time"
)

type mockMergeDecisionStore struct {
	samples []MergeDecisionSample
}

func (s *mockMergeDecisionStore) GetRecentAcceptedMerges(_ context.Context, _ string, limit int, _ time.Time) ([]MergeDecisionSample, error) {
	if limit < len(s.samples) {
		return s.samples[:limit], nil
	}
	return s.samples, nil
}

type mockPrecisionLLM struct {
	responses []string
	callIdx   int
	err       error
}

func (m *mockPrecisionLLM) Complete(_ context.Context, _, _ string) (string, error) {
	if m.err != nil {
		return "", m.err
	}
	idx := m.callIdx
	m.callIdx++
	if idx < len(m.responses) {
		return m.responses[idx], nil
	}
	return "CORRECT", nil
}

type nopPrecisionLogger struct{}

func (nopPrecisionLogger) Info(string, ...any) {}
func (nopPrecisionLogger) Warn(string, ...any) {}

func makeSamples(n int) []MergeDecisionSample {
	samples := make([]MergeDecisionSample, n)
	for i := range samples {
		samples[i] = MergeDecisionSample{
			NewBody:       fmt.Sprintf("new item %d", i),
			CandidateBody: fmt.Sprintf("candidate item %d", i),
			CosineScore:   0.95,
		}
	}
	return samples
}

func TestConsolidationPrecision_AllCorrect(t *testing.T) {
	store := &mockMergeDecisionStore{samples: makeSamples(10)}
	llm := &mockPrecisionLLM{responses: repeatStr("CORRECT", 10)}
	grader := NewLLMConsolidationPrecisionGrader(store, llm, nopPrecisionLogger{})

	precision, err := grader.MeasurePrecision(context.Background(), "ws-1")
	if err != nil {
		t.Fatal(err)
	}
	if precision != 1.0 {
		t.Fatalf("expected 1.0, got %v", precision)
	}
}

func TestConsolidationPrecision_HalfCorrect(t *testing.T) {
	store := &mockMergeDecisionStore{samples: makeSamples(10)}
	responses := make([]string, 10)
	for i := range responses {
		if i < 5 {
			responses[i] = "CORRECT"
		} else {
			responses[i] = "INCORRECT"
		}
	}
	llm := &mockPrecisionLLM{responses: responses}
	grader := NewLLMConsolidationPrecisionGrader(store, llm, nopPrecisionLogger{})

	precision, err := grader.MeasurePrecision(context.Background(), "ws-1")
	if err != nil {
		t.Fatal(err)
	}
	if precision != 0.5 {
		t.Fatalf("expected 0.5, got %v", precision)
	}
}

func TestConsolidationPrecision_LLMError_CountsAsCorrect(t *testing.T) {
	store := &mockMergeDecisionStore{samples: makeSamples(5)}
	llm := &mockPrecisionLLM{err: fmt.Errorf("LLM down")}
	grader := NewLLMConsolidationPrecisionGrader(store, llm, nopPrecisionLogger{})

	precision, err := grader.MeasurePrecision(context.Background(), "ws-1")
	if err != nil {
		t.Fatal(err)
	}
	if precision != 1.0 {
		t.Fatalf("expected 1.0 (errors count as correct), got %v", precision)
	}
}

func TestConsolidationPrecision_InsufficientData_Returns1(t *testing.T) {
	store := &mockMergeDecisionStore{samples: makeSamples(3)}
	llm := &mockPrecisionLLM{}
	grader := NewLLMConsolidationPrecisionGrader(store, llm, nopPrecisionLogger{})

	precision, err := grader.MeasurePrecision(context.Background(), "ws-1")
	if err != nil {
		t.Fatal(err)
	}
	if precision != 1.0 {
		t.Fatalf("expected 1.0 for insufficient data, got %v", precision)
	}
}

func TestConsolidationPrecision_EmptyStore_Returns1(t *testing.T) {
	store := &mockMergeDecisionStore{samples: nil}
	llm := &mockPrecisionLLM{}
	grader := NewLLMConsolidationPrecisionGrader(store, llm, nopPrecisionLogger{})

	precision, err := grader.MeasurePrecision(context.Background(), "ws-1")
	if err != nil {
		t.Fatal(err)
	}
	if precision != 1.0 {
		t.Fatalf("expected 1.0 for empty store, got %v", precision)
	}
}

func repeatStr(s string, n int) []string {
	result := make([]string, n)
	for i := range result {
		result[i] = s
	}
	return result
}
