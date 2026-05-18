package brain

import (
	"testing"
	"time"
)

func TestCritique_HighQuality(t *testing.T) {
	svc := NewCriticReflectorService()

	output, err := svc.Critique(ExecutionTrace{
		WorkspaceID: "ws1",
		Intent:      "read email",
		PlanSteps:   3,
		Succeeded:   3,
		Failed:      0,
		ToolsUsed:   []string{"email_read"},
		Duration:    2 * time.Second,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !output.Passed {
		t.Fatalf("expected pass, got score %f", output.OverallScore)
	}
	if output.OverallScore < 0.7 {
		t.Fatalf("expected high score, got %f", output.OverallScore)
	}
	if len(output.FailureModes) != 0 {
		t.Fatalf("expected no failure modes, got %v", output.FailureModes)
	}
}

func TestCritique_LowQuality(t *testing.T) {
	svc := NewCriticReflectorService()

	output, err := svc.Critique(ExecutionTrace{
		WorkspaceID: "ws1",
		Intent:      "complex multi-step",
		PlanSteps:   10,
		Succeeded:   2,
		Failed:      8,
		ToolsUsed:   []string{"a", "b", "c"},
		Duration:    45 * time.Second,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if output.Passed {
		t.Fatal("expected fail for low quality execution")
	}
	if len(output.FailureModes) == 0 {
		t.Fatal("expected failure modes")
	}
	if output.ImprovementDirective == "" {
		t.Fatal("expected improvement directive")
	}
}

func TestCritique_EmptyWorkspace(t *testing.T) {
	svc := NewCriticReflectorService()
	_, err := svc.Critique(ExecutionTrace{})
	if err == nil {
		t.Fatal("expected error for empty workspace")
	}
}

func TestReflect(t *testing.T) {
	svc := NewCriticReflectorService()

	criticOutput := &CriticOutput{
		OverallScore: 0.4,
		FailureModes: []string{"low_completeness", "low_reliability"},
	}
	trace := ExecutionTrace{
		WorkspaceID: "ws1",
		PlanSteps:   5,
		Succeeded:   1,
		Failed:      4,
	}

	reflectorOutput, err := svc.Reflect(criticOutput, trace)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(reflectorOutput.LessonCandidates) == 0 {
		t.Fatal("expected lesson candidates")
	}
	if !reflectorOutput.PatternDetected {
		t.Fatal("expected pattern detection for multiple failures")
	}
}

func TestReflect_EscalateToFeedback(t *testing.T) {
	svc := NewCriticReflectorService()

	criticOutput := &CriticOutput{
		OverallScore: 0.2,
		FailureModes: []string{"low_completeness"},
	}
	trace := ExecutionTrace{WorkspaceID: "ws1", Failed: 0}

	reflectorOutput, err := svc.Reflect(criticOutput, trace)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !reflectorOutput.EscalateToFeedback {
		t.Fatal("expected escalation for very low score")
	}
}

func TestGetCritiqueHistory(t *testing.T) {
	svc := NewCriticReflectorService()

	_, _ = svc.Critique(ExecutionTrace{WorkspaceID: "ws1", PlanSteps: 1, Succeeded: 1, Duration: time.Second})
	_, _ = svc.Critique(ExecutionTrace{WorkspaceID: "ws2", PlanSteps: 2, Succeeded: 1, Failed: 1, Duration: time.Second})

	history := svc.GetCritiqueHistory()
	if len(history) != 2 {
		t.Fatalf("expected 2 history entries, got %d", len(history))
	}
}
