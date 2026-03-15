package adaptive

import (
	"context"
	"fmt"
	"testing"
	"time"
)

type mockClassifierLLM struct {
	response string
	err      error
	called   bool
}

func (m *mockClassifierLLM) Complete(_ context.Context, _, _ string) (string, error) {
	m.called = true
	return m.response, m.err
}

type nopLogger struct{}

func (nopLogger) Debug(string, ...any) {}
func (nopLogger) Warn(string, ...any)  {}
func (nopLogger) Error(string, ...any) {}

// NO_RETRIEVAL tests

func TestClassifier_OK_NoRetrieval(t *testing.T) {
	c := NewRetrievalClassifier(nil, nopLogger{})
	r := c.Classify(context.Background(), "ok")
	if r.Tier != TierNoRetrieval {
		t.Fatalf("expected NO_RETRIEVAL, got %s", r.Tier)
	}
	if r.Confidence < 0.90 {
		t.Fatalf("expected confidence >= 0.90, got %v", r.Confidence)
	}
}

func TestClassifier_GotIt_NoRetrieval(t *testing.T) {
	c := NewRetrievalClassifier(nil, nopLogger{})
	r := c.Classify(context.Background(), "Got it!")
	if r.Tier != TierNoRetrieval {
		t.Fatalf("expected NO_RETRIEVAL, got %s", r.Tier)
	}
}

func TestClassifier_Thanks_NoRetrieval(t *testing.T) {
	c := NewRetrievalClassifier(nil, nopLogger{})
	r := c.Classify(context.Background(), "Thanks so much!")
	if r.Tier != TierNoRetrieval {
		t.Fatalf("expected NO_RETRIEVAL for short non-question, got %s", r.Tier)
	}
}

func TestClassifier_WillDo_NoRetrieval(t *testing.T) {
	c := NewRetrievalClassifier(nil, nopLogger{})
	r := c.Classify(context.Background(), "Will do.")
	if r.Tier != TierNoRetrieval {
		t.Fatalf("expected NO_RETRIEVAL, got %s", r.Tier)
	}
}

func TestClassifier_And_Short_NoRetrieval(t *testing.T) {
	c := NewRetrievalClassifier(nil, nopLogger{})
	r := c.Classify(context.Background(), "And that's it")
	if r.Tier != TierNoRetrieval {
		t.Fatalf("expected NO_RETRIEVAL, got %s", r.Tier)
	}
}

func TestClassifier_Empty_NoRetrieval(t *testing.T) {
	c := NewRetrievalClassifier(nil, nopLogger{})
	r := c.Classify(context.Background(), "")
	if r.Tier != TierNoRetrieval {
		t.Fatalf("expected NO_RETRIEVAL for empty, got %s", r.Tier)
	}
}

// SINGLE_HOP tests

func TestClassifier_WhatDidAliceSay_SingleHop(t *testing.T) {
	c := NewRetrievalClassifier(nil, nopLogger{})
	r := c.Classify(context.Background(), "What did Alice say about the deadline?")
	if r.Tier != TierSingleHop {
		t.Fatalf("expected SINGLE_HOP, got %s", r.Tier)
	}
}

func TestClassifier_FindBudget_SingleHop(t *testing.T) {
	c := NewRetrievalClassifier(nil, nopLogger{})
	r := c.Classify(context.Background(), "Find the Q3 budget numbers")
	if r.Tier != TierSingleHop {
		t.Fatalf("expected SINGLE_HOP, got %s", r.Tier)
	}
}

func TestClassifier_ScheduleCall_SingleHop(t *testing.T) {
	c := NewRetrievalClassifier(nil, nopLogger{})
	r := c.Classify(context.Background(), "Schedule a call with Bob")
	if r.Tier != TierSingleHop {
		t.Fatalf("expected SINGLE_HOP, got %s", r.Tier)
	}
}

// MULTI_HOP tests

func TestClassifier_SummarizeAll_MultiHop(t *testing.T) {
	c := NewRetrievalClassifier(nil, nopLogger{})
	r := c.Classify(context.Background(), "Summarize everything about Project Falcon this month")
	if r.Tier != TierMultiHop {
		t.Fatalf("expected MULTI_HOP, got %s", r.Tier)
	}
}

func TestClassifier_AllMeetings_MultiHop(t *testing.T) {
	c := NewRetrievalClassifier(nil, nopLogger{})
	r := c.Classify(context.Background(), "List all my meetings this month")
	if r.Tier != TierMultiHop {
		t.Fatalf("expected MULTI_HOP, got %s", r.Tier)
	}
}

func TestClassifier_CrossEntity_MultiHop(t *testing.T) {
	c := NewRetrievalClassifier(nil, nopLogger{})
	r := c.Classify(context.Background(), "Who are all people connected to the board meeting?")
	if r.Tier != TierMultiHop {
		t.Fatalf("expected MULTI_HOP, got %s", r.Tier)
	}
}

// LLM fallback tests

func TestClassifier_AmbiguousTriggersLLM(t *testing.T) {
	llm := &mockClassifierLLM{response: "SINGLE_HOP"}
	c := NewRetrievalClassifier(llm, nopLogger{})
	// A medium-length non-question non-task query is ambiguous
	r := c.Classify(context.Background(), "something about the project status and next steps maybe")
	if !llm.called {
		t.Fatal("expected LLM to be called for ambiguous query")
	}
	_ = r
}

func TestClassifier_LLMError_FallsToSingleHop(t *testing.T) {
	llm := &mockClassifierLLM{err: fmt.Errorf("LLM unavailable")}
	c := NewRetrievalClassifier(llm, nopLogger{})
	r := c.Classify(context.Background(), "something ambiguous about the project status maybe")
	if r.Tier != TierSingleHop {
		t.Fatalf("expected SINGLE_HOP fallback, got %s", r.Tier)
	}
}

func TestClassifier_LLMBadOutput_FallsBack(t *testing.T) {
	llm := &mockClassifierLLM{response: "UNKNOWN_TIER"}
	c := NewRetrievalClassifier(llm, nopLogger{})
	r := c.Classify(context.Background(), "something ambiguous about the project status maybe")
	if r.Tier != TierSingleHop {
		t.Fatalf("expected SINGLE_HOP fallback for bad LLM output, got %s", r.Tier)
	}
}

func TestClassifier_LLMNoRetrieval(t *testing.T) {
	llm := &mockClassifierLLM{response: "NO_RETRIEVAL"}
	c := NewRetrievalClassifier(llm, nopLogger{})
	r := c.Classify(context.Background(), "something ambiguous about the project status maybe")
	if r.Tier != TierNoRetrieval {
		t.Fatalf("expected NO_RETRIEVAL from LLM, got %s", r.Tier)
	}
	if r.Method != "llm" {
		t.Fatalf("expected method=llm, got %s", r.Method)
	}
}

// Gate tests

func TestGate_SkipRetrieval_NoRet(t *testing.T) {
	c := NewRetrievalClassifier(nil, nopLogger{})
	g := NewGate(c, nil, nopLogger{})
	if !g.ShouldSkipRetrieval(context.Background(), "ok") {
		t.Fatal("expected skip retrieval for 'ok'")
	}
}

func TestGate_NoSkip_SingleHop(t *testing.T) {
	c := NewRetrievalClassifier(nil, nopLogger{})
	g := NewGate(c, nil, nopLogger{})
	if g.ShouldSkipRetrieval(context.Background(), "What did Alice say about the deadline?") {
		t.Fatal("should NOT skip retrieval for targeted question")
	}
}

func TestGate_IsMultiHop_Aggregation(t *testing.T) {
	c := NewRetrievalClassifier(nil, nopLogger{})
	g := NewGate(c, nil, nopLogger{})
	if !g.IsMultiHop(context.Background(), "List all meetings this month") {
		t.Fatal("expected multi-hop for aggregation query")
	}
}

func TestGate_MetricsRecorded(t *testing.T) {
	c := NewRetrievalClassifier(nil, nopLogger{})
	m := &trackingMetrics{}
	g := NewGate(c, m, nopLogger{})
	g.Route(context.Background(), "ok")
	if m.classifiedTier != "no_retrieval" {
		t.Fatalf("expected tier=no_retrieval in metrics, got %q", m.classifiedTier)
	}
	if !m.skipped {
		t.Fatal("expected retrieval_skipped metric to be recorded")
	}
}

// Tier String tests

func TestTierString(t *testing.T) {
	if TierNoRetrieval.String() != "no_retrieval" {
		t.Fatal("wrong string for TierNoRetrieval")
	}
	if TierSingleHop.String() != "single_hop" {
		t.Fatal("wrong string for TierSingleHop")
	}
	if TierMultiHop.String() != "multi_hop" {
		t.Fatal("wrong string for TierMultiHop")
	}
}

// helpers

type trackingMetrics struct {
	classifiedTier string
	skipped        bool
}

func (m *trackingMetrics) IncQueryClassified(tier, _ string)                    { m.classifiedTier = tier }
func (m *trackingMetrics) ObserveClassificationLatency(_ string, _ time.Duration) {}
func (m *trackingMetrics) IncRetrievalSkipped()                                   { m.skipped = true }
