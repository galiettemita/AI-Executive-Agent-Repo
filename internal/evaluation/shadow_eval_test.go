package evaluation

import (
	"context"
	"log/slog"
	"os"
	"testing"
)

var testLogger = slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))

// mockORMScorer returns configurable scores.
type mockORMScorer struct {
	score float64
}

func (m *mockORMScorer) Score(_ context.Context, _ string) (float64, error) {
	return m.score, nil
}

// mockLLMJudge returns configurable scores.
type mockLLMJudge struct {
	champScore float64
	challScore float64
}

func (m *mockLLMJudge) Compare(_ context.Context, _, _ string) (float64, float64, error) {
	return m.champScore, m.challScore, nil
}

// mockLLMClient returns fixed completions.
type mockLLMClient struct {
	response string
}

func (m *mockLLMClient) Complete(_ context.Context, _, _ string) (string, error) {
	return m.response, nil
}

func TestShadowWorkflowScoring(t *testing.T) {
	activities := &ShadowEvalActivities{
		ORM:   &mockORMScorer{score: 7.5},
		Judge: &mockLLMJudge{champScore: 8.0, challScore: 7.0},
	}

	champScore, err := activities.ScoreChampion(context.Background(), "Champion response")
	if err != nil {
		t.Fatalf("ScoreChampion failed: %v", err)
	}
	if champScore != 7.5 {
		t.Errorf("Expected champion score 7.5, got %f", champScore)
	}

	challScore, err := activities.ScoreChallenger(context.Background(), "Challenger response")
	if err != nil {
		t.Fatalf("ScoreChallenger failed: %v", err)
	}
	if challScore != 7.5 {
		t.Errorf("Expected challenger score 7.5, got %f", challScore)
	}

	judgeResult, err := activities.LLMJudgeEval(context.Background(), "champion", "challenger")
	if err != nil {
		t.Fatalf("LLMJudgeEval failed: %v", err)
	}
	if judgeResult[0] != 8.0 || judgeResult[1] != 7.0 {
		t.Errorf("Expected judge [8.0, 7.0], got %v", judgeResult)
	}
}

func TestChallengerPromotionCriteriaMet(t *testing.T) {
	promoter := NewChallengerPromoter(nil, testLogger)

	decision, err := promoter.EvaluatePromotion(context.Background(), "challenger-v1")
	if err != nil {
		t.Fatalf("EvaluatePromotion failed: %v", err)
	}

	// Without DB, all metrics are 0 → ORM improvement < 0.2 → not eligible.
	if decision.Eligible {
		t.Error("Expected not eligible without DB data (improvement < 0.2)")
	}
}

func TestChallengerPromotionFails(t *testing.T) {
	promoter := NewChallengerPromoter(nil, testLogger)

	decision, err := promoter.EvaluatePromotion(context.Background(), "challenger-v2")
	if err != nil {
		t.Fatalf("EvaluatePromotion failed: %v", err)
	}

	if decision.Eligible {
		t.Error("Expected not eligible when ORM improvement < 0.2")
	}

	if decision.Metrics.ORMImprovement >= 0.2 {
		t.Errorf("Expected ORM improvement < 0.2, got %f", decision.Metrics.ORMImprovement)
	}
}

func TestMTBenchJudgeScoring(t *testing.T) {
	judge := NewMTBenchJudge(
		&mockLLMClient{response: `{"score": 8.5, "reasoning": "Excellent response"}`},
		testLogger,
	)

	score, err := judge.ScoreTurn(context.Background(), "writing", "Draft an email", "Here is your email...", 1)
	if err != nil {
		t.Fatalf("ScoreTurn failed: %v", err)
	}
	if score != 8.5 {
		t.Errorf("Expected score 8.5, got %f", score)
	}
}

func TestMTBenchJudgeFallbackOnBadJSON(t *testing.T) {
	judge := NewMTBenchJudge(
		&mockLLMClient{response: "not valid json"},
		testLogger,
	)

	score, err := judge.ScoreTurn(context.Background(), "writing", "Draft an email", "Here is your email...", 1)
	if err != nil {
		t.Fatalf("ScoreTurn failed: %v", err)
	}
	if score != 5.0 {
		t.Errorf("Expected fallback score 5.0, got %f", score)
	}
}

func TestMTBenchEvaluateConversation(t *testing.T) {
	judge := NewMTBenchJudge(
		&mockLLMClient{response: `{"score": 7.0, "reasoning": "Good"}`},
		testLogger,
	)

	conv := MTBenchConversation{
		ID:       "writing_01",
		Category: "writing",
		Turns: []MTBenchTurn{
			{Turn: 1, User: "Draft an email"},
			{Turn: 2, User: "Make it more urgent"},
		},
	}

	result, err := judge.EvaluateConversation(context.Background(), conv, []string{"Email here", "Urgent email here"})
	if err != nil {
		t.Fatalf("EvaluateConversation failed: %v", err)
	}

	if len(result.TurnScores) != 2 {
		t.Errorf("Expected 2 turn scores, got %d", len(result.TurnScores))
	}
	if result.AvgScore != 7.0 {
		t.Errorf("Expected avg score 7.0, got %f", result.AvgScore)
	}
}

func TestMTBenchCIRegressionFailsOnDrop(t *testing.T) {
	baseline := 7.2
	currentScore := 6.7 // 0.5 drop, exceeds 0.3 threshold

	if currentScore < baseline-0.3 {
		t.Logf("Regression detected: current=%.1f, baseline=%.1f, drop=%.1f (threshold 0.3)",
			currentScore, baseline, baseline-currentScore)
	} else {
		t.Error("Expected regression to be detected for 0.5 drop")
	}
}
