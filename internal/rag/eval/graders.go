package eval

import "context"

type EvalEntry struct {
	ID             string
	ExpectedTools  []string
	ExpectedParams map[string]string
}

type EvalResult struct {
	ActualTools  []string
	ActualParams map[string]string
}

type GradeResult struct {
	Score       float64
	Pass        bool
	Details     map[string]float64
	Explanation string
}

type EvalGrader interface {
	Grade(ctx context.Context, input EvalEntry, actual EvalResult) (GradeResult, error)
}

type ToolSelectionGrader struct{}

func (ToolSelectionGrader) Grade(_ context.Context, input EvalEntry, actual EvalResult) (GradeResult, error) {
	expectedSet := map[string]struct{}{}
	actualSet := map[string]struct{}{}
	for _, key := range input.ExpectedTools {
		expectedSet[key] = struct{}{}
	}
	for _, key := range actual.ActualTools {
		actualSet[key] = struct{}{}
	}

	if len(expectedSet) == 0 && len(actualSet) == 0 {
		return GradeResult{Score: 1, Pass: true, Details: map[string]float64{"tool_selection": 1}, Explanation: "no tool selection expected"}, nil
	}

	matches := 0
	for key := range expectedSet {
		if _, ok := actualSet[key]; ok {
			matches++
		}
	}
	score := float64(matches) / float64(maxInt(len(expectedSet), 1))
	return GradeResult{
		Score:       score,
		Pass:        score >= 1.0,
		Details:     map[string]float64{"tool_selection": score},
		Explanation: "tool selection match score",
	}, nil
}

type ParameterGrader struct{}

func (ParameterGrader) Grade(_ context.Context, input EvalEntry, actual EvalResult) (GradeResult, error) {
	if len(input.ExpectedParams) == 0 {
		return GradeResult{Score: 1, Pass: true, Details: map[string]float64{"parameters": 1}, Explanation: "no parameter constraints"}, nil
	}
	matches := 0
	for key, expected := range input.ExpectedParams {
		if actual.ActualParams[key] == expected {
			matches++
		}
	}
	score := float64(matches) / float64(maxInt(len(input.ExpectedParams), 1))
	return GradeResult{
		Score:       score,
		Pass:        score >= 0.8,
		Details:     map[string]float64{"parameters": score},
		Explanation: "parameter match score",
	}, nil
}

func maxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}
