package brain

import (
	"context"
	"testing"

	"github.com/brevio/brevio/internal/llm"
)

// mockORMClient is a test double that returns a fixed response.
type mockORMClient struct {
	response string
}

func (m *mockORMClient) Generate(_ context.Context, _ llm.GenerateRequest) (*llm.GenerateResponse, *llm.Usage, error) {
	return &llm.GenerateResponse{Content: m.response}, &llm.Usage{}, nil
}

func (m *mockORMClient) Stream(_ context.Context, _ llm.GenerateRequest, out chan<- llm.StreamChunk) {
	close(out)
}

// mockCriticRepo is a no-op critic trace repository for tests.
type mockCriticRepo struct{}

func (m *mockCriticRepo) Save(_ context.Context, _ CriticOutput) error                                     { return nil }
func (m *mockCriticRepo) StoreORMResult(_ context.Context, _, _ string, _ *OutcomeScore) error { return nil }

func TestScoreFinalOutcome_PassingScore(t *testing.T) {
	t.Parallel()
	mock := &mockORMClient{response: `{"overall_quality":4.0,"intent_satisfied":true,"completeness":0.9,"accuracy":0.85,"side_effects":[],"improvement_hints":""}`}
	orm := NewOutcomeRewardModel(mock, &mockCriticRepo{})
	score, err := orm.ScoreFinalOutcome(context.Background(), "ws1", "test intent", nil, nil, "test response")
	if err != nil {
		t.Fatal(err)
	}
	if score.OverallQuality != 4.0 {
		t.Errorf("expected 4.0 got %f", score.OverallQuality)
	}
	if !orm.Passes(score) {
		t.Error("expected pass")
	}
	if score.LatencyMs < 0 {
		t.Error("expected non-negative latency")
	}
}

func TestScoreFinalOutcome_FailingScore(t *testing.T) {
	t.Parallel()
	mock := &mockORMClient{response: `{"overall_quality":2.0,"intent_satisfied":false,"completeness":0.4,"accuracy":0.3,"side_effects":["partial write"],"improvement_hints":"retry with more context"}`}
	orm := NewOutcomeRewardModel(mock, &mockCriticRepo{})
	score, err := orm.ScoreFinalOutcome(context.Background(), "ws1", "test intent", nil, nil, "bad response")
	if err != nil {
		t.Fatal(err)
	}
	if orm.Passes(score) {
		t.Error("expected fail")
	}
	if score.ImprovementHints != "retry with more context" {
		t.Errorf("unexpected improvement hints: %s", score.ImprovementHints)
	}
}

func TestScoreFinalOutcome_MalformedJSON(t *testing.T) {
	t.Parallel()
	mock := &mockORMClient{response: `not json`}
	orm := NewOutcomeRewardModel(mock, &mockCriticRepo{})
	_, err := orm.ScoreFinalOutcome(context.Background(), "ws1", "intent", nil, nil, "resp")
	if err == nil {
		t.Fatal("expected error on malformed JSON")
	}
}

func TestScoreFinalOutcome_OutOfRangeQuality(t *testing.T) {
	t.Parallel()
	mock := &mockORMClient{response: `{"overall_quality":6.0,"intent_satisfied":true,"completeness":1.0,"accuracy":1.0,"side_effects":[],"improvement_hints":""}`}
	orm := NewOutcomeRewardModel(mock, &mockCriticRepo{})
	_, err := orm.ScoreFinalOutcome(context.Background(), "ws1", "intent", nil, nil, "resp")
	if err == nil {
		t.Fatal("expected error on out-of-range quality")
	}
}

func TestPassesThreshold(t *testing.T) {
	t.Parallel()
	orm := NewOutcomeRewardModel(nil, nil)
	cases := []struct {
		q    float64
		want bool
	}{
		{2.4, false},
		{2.5, true},
		{3.0, true},
		{5.0, true},
		{1.0, false},
	}
	for _, c := range cases {
		got := orm.Passes(&OutcomeScore{OverallQuality: c.q})
		if got != c.want {
			t.Errorf("q=%f: want %v got %v", c.q, c.want, got)
		}
	}
}

func TestPassesNilScore(t *testing.T) {
	t.Parallel()
	orm := NewOutcomeRewardModel(nil, nil)
	if orm.Passes(nil) {
		t.Error("nil score should not pass")
	}
}

func TestScoreFinalOutcome_NilClient(t *testing.T) {
	t.Parallel()
	orm := NewOutcomeRewardModel(nil, nil)
	_, err := orm.ScoreFinalOutcome(context.Background(), "ws1", "intent", nil, nil, "resp")
	if err == nil {
		t.Fatal("expected error with nil client")
	}
}
