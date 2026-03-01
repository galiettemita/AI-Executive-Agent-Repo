package eval

import (
	"context"
	"testing"
)

func TestToolSelectionAndParameterGraders(t *testing.T) {
	t.Parallel()

	toolGrader := ToolSelectionGrader{}
	toolScore, err := toolGrader.Grade(context.Background(), EvalEntry{
		ID:            "eval-001",
		ExpectedTools: []string{"google_calendar.create_event"},
	}, EvalResult{
		ActualTools: []string{"google_calendar.create_event"},
	})
	if err != nil {
		t.Fatalf("tool grader error: %v", err)
	}
	if !toolScore.Pass || toolScore.Score != 1 {
		t.Fatalf("unexpected tool grading result: %+v", toolScore)
	}

	paramGrader := ParameterGrader{}
	paramScore, err := paramGrader.Grade(context.Background(), EvalEntry{
		ID:             "eval-002",
		ExpectedParams: map[string]string{"time": "2pm"},
	}, EvalResult{
		ActualParams: map[string]string{"time": "2pm"},
	})
	if err != nil {
		t.Fatalf("parameter grader error: %v", err)
	}
	if !paramScore.Pass || paramScore.Score != 1 {
		t.Fatalf("unexpected parameter grading result: %+v", paramScore)
	}
}
