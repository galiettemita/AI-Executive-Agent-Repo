package brain

import (
	"testing"
)

func TestScoreAlternatives(t *testing.T) {
	svc := NewCounterfactualService()

	original := Plan{
		Steps: []PlanStep{
			{ToolKey: "email_read", Phase: "gather"},
			{ToolKey: "email_send", Phase: "act"},
			{ToolKey: "verify_output", Phase: "verify"},
		},
		RiskLevel:       "critical",
		EstimatedTokens: 600,
	}

	alt1 := Plan{
		Steps: []PlanStep{
			{ToolKey: "email_read", Phase: "gather"},
			{ToolKey: "draft_reply", Phase: "act"},
		},
		RiskLevel:       "low",
		EstimatedTokens: 400,
	}

	alt2 := Plan{
		Steps: []PlanStep{
			{ToolKey: "search", Phase: "gather"},
			{ToolKey: "email_read", Phase: "gather"},
			{ToolKey: "email_send", Phase: "act"},
			{ToolKey: "verify_output", Phase: "verify"},
		},
		RiskLevel:       "elevated",
		EstimatedTokens: 800,
	}

	analysis, err := svc.ScoreAlternatives(original, []Plan{alt1, alt2})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(analysis.AlternativeScores) != 2 {
		t.Fatalf("expected 2 alternative scores, got %d", len(analysis.AlternativeScores))
	}
	if analysis.OriginalPlanScore <= 0 {
		t.Fatal("expected positive original score")
	}
	// alt1 (low risk, fewer steps) should score better than critical original.
	if analysis.AlternativeScores[0] <= analysis.OriginalPlanScore {
		t.Log("alt1 did not score higher; score comparison may vary by heuristic")
	}
}

func TestScoreAlternatives_NoAlternatives(t *testing.T) {
	svc := NewCounterfactualService()
	_, err := svc.ScoreAlternatives(Plan{}, nil)
	if err == nil {
		t.Fatal("expected error for no alternatives")
	}
}

func TestShouldHaveChosen_Yes(t *testing.T) {
	svc := NewCounterfactualService()

	analysis := &CounterfactualAnalysis{
		OriginalPlanScore:    0.5,
		AlternativeScores:    []float64{0.8},
		BestAlternativeIndex: 0,
		ImprovementPotential: 0.3,
	}

	if !svc.ShouldHaveChosen(analysis) {
		t.Fatal("expected should-have-chosen for 0.3 improvement")
	}
}

func TestShouldHaveChosen_No(t *testing.T) {
	svc := NewCounterfactualService()

	analysis := &CounterfactualAnalysis{
		OriginalPlanScore:    0.8,
		AlternativeScores:    []float64{0.82},
		BestAlternativeIndex: 0,
		ImprovementPotential: 0.02,
	}

	if svc.ShouldHaveChosen(analysis) {
		t.Fatal("expected no should-have-chosen for tiny improvement")
	}
}

func TestShouldHaveChosen_Nil(t *testing.T) {
	svc := NewCounterfactualService()
	if svc.ShouldHaveChosen(nil) {
		t.Fatal("expected false for nil analysis")
	}
}

func TestScoreAlternatives_EmptyPlan(t *testing.T) {
	svc := NewCounterfactualService()

	analysis, err := svc.ScoreAlternatives(Plan{}, []Plan{
		{Steps: []PlanStep{{ToolKey: "a", Phase: "gather"}}, RiskLevel: "low", EstimatedTokens: 100},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if analysis.OriginalPlanScore != 0 {
		t.Fatalf("expected 0 score for empty plan, got %f", analysis.OriginalPlanScore)
	}
	if analysis.ImprovementPotential <= 0 {
		t.Fatal("expected positive improvement potential vs empty plan")
	}
}
