package brain

import (
	"context"
	"errors"
	"testing"
)

// mockExecutor implements ToolExecutor for testing.
type mockExecutor struct {
	results map[string]map[string]any
	errs    map[string]error
}

func (m *mockExecutor) Execute(_ context.Context, toolKey string, _ map[string]any) (map[string]any, error) {
	if err, ok := m.errs[toolKey]; ok {
		return nil, err
	}
	if result, ok := m.results[toolKey]; ok {
		return result, nil
	}
	return map[string]any{"status": "ok"}, nil
}

func newMockExecutor() *mockExecutor {
	return &mockExecutor{
		results: make(map[string]map[string]any),
		errs:    make(map[string]error),
	}
}

func TestPlannerStep_EmailIntent(t *testing.T) {
	t.Parallel()

	rl := NewReasoningLoop(ReasoningLoopConfig{})
	rc := &ReasoningContext{
		MessageID:   "msg-1",
		WorkspaceID: "ws-1",
		Intent:      "send an email to John about the meeting",
		Confidence:  0.9,
	}

	plan, err := rl.PlannerStep(context.Background(), rc)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(plan.Steps) == 0 {
		t.Fatal("expected non-empty plan")
	}

	// Should have email_read (gather), email_send (act), verify_output (verify)
	toolKeys := make(map[string]bool)
	for _, step := range plan.Steps {
		toolKeys[step.ToolKey] = true
	}
	if !toolKeys["email_read"] {
		t.Error("expected email_read step")
	}
	if !toolKeys["email_send"] {
		t.Error("expected email_send step")
	}
	if !toolKeys["verify_output"] {
		t.Error("expected verify_output step")
	}
	if plan.RiskLevel != "critical" {
		t.Errorf("got risk_level=%q want=%q (email_send is high risk)", plan.RiskLevel, "critical")
	}
}

func TestPlannerStep_CalendarIntent(t *testing.T) {
	t.Parallel()

	rl := NewReasoningLoop(ReasoningLoopConfig{})
	rc := &ReasoningContext{
		MessageID:   "msg-2",
		WorkspaceID: "ws-1",
		Intent:      "check my calendar for tomorrow",
		Confidence:  0.85,
	}

	plan, err := rl.PlannerStep(context.Background(), rc)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(plan.Steps) != 1 {
		t.Fatalf("expected 1 step (read-only), got %d", len(plan.Steps))
	}
	if plan.Steps[0].ToolKey != "calendar_read" {
		t.Errorf("got tool_key=%q want=%q", plan.Steps[0].ToolKey, "calendar_read")
	}
	if plan.RiskLevel != "low" {
		t.Errorf("got risk_level=%q want=%q", plan.RiskLevel, "low")
	}
}

func TestPlannerStep_EmptyIntent(t *testing.T) {
	t.Parallel()

	rl := NewReasoningLoop(ReasoningLoopConfig{})
	rc := &ReasoningContext{Intent: ""}

	_, err := rl.PlannerStep(context.Background(), rc)
	if err == nil {
		t.Fatal("expected error for empty intent")
	}
}

func TestPlannerStep_NilContext(t *testing.T) {
	t.Parallel()

	rl := NewReasoningLoop(ReasoningLoopConfig{})
	_, err := rl.PlannerStep(context.Background(), nil)
	if !errors.Is(err, ErrReasoningNoContext) {
		t.Errorf("expected ErrReasoningNoContext, got %v", err)
	}
}

func TestExecutorStep_AllSuccess(t *testing.T) {
	t.Parallel()

	exec := newMockExecutor()
	exec.results["calendar_read"] = map[string]any{"events": []string{"standup"}}

	rl := NewReasoningLoop(ReasoningLoopConfig{Executor: exec})
	plan := &Plan{
		Steps: []PlanStep{
			{ToolKey: "calendar_read", Parameters: map[string]any{}, Phase: "gather"},
		},
	}

	result, err := rl.ExecutorStep(context.Background(), plan)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(result.Results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(result.Results))
	}
	if !result.Results[0].Success {
		t.Error("expected step to succeed")
	}
	if result.CompensationNeeded {
		t.Error("expected no compensation needed")
	}
}

func TestExecutorStep_DependencyFailure(t *testing.T) {
	t.Parallel()

	exec := newMockExecutor()
	exec.errs["email_read"] = errors.New("connection refused")

	rl := NewReasoningLoop(ReasoningLoopConfig{Executor: exec})
	plan := &Plan{
		Steps: []PlanStep{
			{ToolKey: "email_read", Parameters: map[string]any{}, Phase: "gather"},
			{ToolKey: "email_send", Parameters: map[string]any{}, Phase: "act", DependsOn: []int{0}},
		},
	}

	result, err := rl.ExecutorStep(context.Background(), plan)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Results[0].Success {
		t.Error("expected step 0 to fail")
	}
	if result.Results[1].Success {
		t.Error("expected step 1 to fail due to dependency")
	}
	if !result.CompensationNeeded {
		t.Error("expected compensation needed")
	}
}

func TestExecutorStep_NilPlan(t *testing.T) {
	t.Parallel()

	rl := NewReasoningLoop(ReasoningLoopConfig{})
	_, err := rl.ExecutorStep(context.Background(), nil)
	if !errors.Is(err, ErrExecutorNoPlan) {
		t.Errorf("expected ErrExecutorNoPlan, got %v", err)
	}
}

func TestExecutorStep_NilExecutor(t *testing.T) {
	t.Parallel()

	rl := NewReasoningLoop(ReasoningLoopConfig{})
	plan := &Plan{
		Steps: []PlanStep{
			{ToolKey: "web_search", Parameters: map[string]any{}, Phase: "gather"},
		},
	}

	result, err := rl.ExecutorStep(context.Background(), plan)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result.Results[0].Success {
		t.Error("expected synthetic success with nil executor")
	}
}

func TestCriticStep_AllSuccess(t *testing.T) {
	t.Parallel()

	rl := NewReasoningLoop(ReasoningLoopConfig{})
	plan := &Plan{
		Steps: []PlanStep{
			{ToolKey: "calendar_read", Phase: "gather"},
			{ToolKey: "calendar_write", Phase: "act"},
		},
	}
	result := &ExecutionResult{
		Results: []StepResult{
			{StepIndex: 0, ToolKey: "calendar_read", Success: true},
			{StepIndex: 1, ToolKey: "calendar_write", Success: true},
		},
	}

	assessment, err := rl.CriticStep(context.Background(), plan, result)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if assessment.QualityScore != 1.0 {
		t.Errorf("got score=%.2f want=1.0", assessment.QualityScore)
	}
	if assessment.ShouldRetry {
		t.Error("should not retry when all steps succeeded")
	}
}

func TestCriticStep_PartialFailure(t *testing.T) {
	t.Parallel()

	rl := NewReasoningLoop(ReasoningLoopConfig{})
	plan := &Plan{
		Steps: []PlanStep{
			{ToolKey: "email_read", Phase: "gather"},
			{ToolKey: "email_send", Phase: "act"},
		},
	}
	result := &ExecutionResult{
		Results: []StepResult{
			{StepIndex: 0, ToolKey: "email_read", Success: true},
			{StepIndex: 1, ToolKey: "email_send", Success: false, Error: "timeout"},
		},
		CompensationNeeded: true,
	}

	assessment, err := rl.CriticStep(context.Background(), plan, result)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if assessment.QualityScore >= 0.8 {
		t.Errorf("score %.2f should be below 0.8 for partial failure", assessment.QualityScore)
	}
	if !assessment.ShouldRetry {
		t.Error("expected should_retry=true for partial failure")
	}
	if len(assessment.Issues) == 0 {
		t.Error("expected issues for partial failure")
	}
}

func TestCriticStep_NilInputs(t *testing.T) {
	t.Parallel()

	rl := NewReasoningLoop(ReasoningLoopConfig{})
	assessment, err := rl.CriticStep(context.Background(), nil, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if assessment.QualityScore != 0 {
		t.Errorf("expected score=0 for nil inputs, got %.2f", assessment.QualityScore)
	}
}

func TestReflectorStep_HighQuality(t *testing.T) {
	t.Parallel()

	rl := NewReasoningLoop(ReasoningLoopConfig{})
	assessment := &CriticAssessment{
		QualityScore: 0.95,
	}

	reflection, err := rl.ReflectorStep(context.Background(), assessment)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if reflection.ConfidenceAdjustment <= 0 {
		t.Errorf("expected positive confidence adjustment for high quality, got %.2f", reflection.ConfidenceAdjustment)
	}
	if len(reflection.Lessons) == 0 {
		t.Error("expected at least one lesson")
	}
}

func TestReflectorStep_LowQuality(t *testing.T) {
	t.Parallel()

	rl := NewReasoningLoop(ReasoningLoopConfig{})
	assessment := &CriticAssessment{
		QualityScore: 0.3,
		Issues:       []string{"step 1 failed: timeout"},
		RetryHints:   []string{"retry step 1"},
	}

	reflection, err := rl.ReflectorStep(context.Background(), assessment)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if reflection.ConfidenceAdjustment >= 0 {
		t.Errorf("expected negative confidence adjustment for low quality, got %.2f", reflection.ConfidenceAdjustment)
	}
	if len(reflection.SuggestedImprovements) == 0 {
		t.Error("expected suggested improvements")
	}
}

func TestReflectorStep_Nil(t *testing.T) {
	t.Parallel()

	rl := NewReasoningLoop(ReasoningLoopConfig{})
	reflection, err := rl.ReflectorStep(context.Background(), nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(reflection.Lessons) != 0 {
		t.Error("expected no lessons for nil assessment")
	}
}

func TestRunLoop_SuccessFirstIteration(t *testing.T) {
	t.Parallel()

	exec := newMockExecutor()
	exec.results["email_read"] = map[string]any{"emails": []string{"hi"}}
	exec.results["email_send"] = map[string]any{"sent": true}
	exec.results["verify_output"] = map[string]any{"verified": true}

	rl := NewReasoningLoop(ReasoningLoopConfig{Executor: exec})
	rc := &ReasoningContext{
		MessageID:   "msg-1",
		WorkspaceID: "ws-1",
		Intent:      "send an email to the team",
		Confidence:  0.9,
	}

	result, err := rl.RunLoop(context.Background(), rc, 3)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Iterations != 1 {
		t.Errorf("got iterations=%d want=1 (should succeed on first try)", result.Iterations)
	}
	if result.CriticScore < 0.8 {
		t.Errorf("got critic_score=%.2f want>=0.8", result.CriticScore)
	}
	if result.FinalPlan == nil {
		t.Error("expected non-nil final plan")
	}
	if result.FinalResult == nil {
		t.Error("expected non-nil final result")
	}
}

func TestRunLoop_NilContext(t *testing.T) {
	t.Parallel()

	rl := NewReasoningLoop(ReasoningLoopConfig{})
	_, err := rl.RunLoop(context.Background(), nil, 3)
	if !errors.Is(err, ErrReasoningNoContext) {
		t.Errorf("expected ErrReasoningNoContext, got %v", err)
	}
}

func TestRunLoop_CancelledContext(t *testing.T) {
	t.Parallel()

	rl := NewReasoningLoop(ReasoningLoopConfig{})
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // cancel immediately

	rc := &ReasoningContext{
		Intent:      "search for documents",
		WorkspaceID: "ws-1",
	}

	_, err := rl.RunLoop(ctx, rc, 3)
	if err == nil {
		t.Fatal("expected error for cancelled context")
	}
}

func TestRunLoop_MaxIterations(t *testing.T) {
	t.Parallel()

	exec := newMockExecutor()
	exec.errs["email_send"] = errors.New("always fails")
	exec.results["email_read"] = map[string]any{"emails": []string{}}
	// verify_output depends on email_send which fails, so it will also fail

	rl := NewReasoningLoop(ReasoningLoopConfig{Executor: exec, MaxIterations: 2})
	rc := &ReasoningContext{
		MessageID:   "msg-1",
		WorkspaceID: "ws-1",
		Intent:      "send reply to the email",
		Confidence:  0.9,
	}

	result, err := rl.RunLoop(context.Background(), rc, 2)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Iterations != 2 {
		t.Errorf("got iterations=%d want=2 (should exhaust max)", result.Iterations)
	}
	if result.CriticScore >= 0.8 {
		t.Errorf("got critic_score=%.2f, expected <0.8 with failures", result.CriticScore)
	}
}

func TestRunLoop_ReadOnlySuccess(t *testing.T) {
	t.Parallel()

	exec := newMockExecutor()
	exec.results["calendar_read"] = map[string]any{"events": []string{"standup", "lunch"}}

	rl := NewReasoningLoop(ReasoningLoopConfig{Executor: exec})
	rc := &ReasoningContext{
		MessageID:   "msg-3",
		WorkspaceID: "ws-2",
		Intent:      "what meetings do I have today on my calendar",
		Confidence:  0.95,
	}

	result, err := rl.RunLoop(context.Background(), rc, 3)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Iterations != 1 {
		t.Errorf("got iterations=%d want=1", result.Iterations)
	}
	if result.CriticScore != 1.0 {
		t.Errorf("got critic_score=%.2f want=1.0", result.CriticScore)
	}
}

func TestNewReasoningLoop_Defaults(t *testing.T) {
	t.Parallel()

	rl := NewReasoningLoop(ReasoningLoopConfig{})
	if rl.qualityTarget != 0.8 {
		t.Errorf("got qualityTarget=%.2f want=0.8", rl.qualityTarget)
	}
	if rl.maxIterations != 3 {
		t.Errorf("got maxIterations=%d want=3", rl.maxIterations)
	}
}

func TestAssessRiskLevel(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		steps []PlanStep
		want  string
	}{
		{
			name:  "read only is low",
			steps: []PlanStep{{ToolKey: "calendar_read"}},
			want:  "low",
		},
		{
			name:  "calendar write is elevated",
			steps: []PlanStep{{ToolKey: "calendar_write"}},
			want:  "elevated",
		},
		{
			name:  "email send is critical",
			steps: []PlanStep{{ToolKey: "email_send"}},
			want:  "critical",
		},
		{
			name:  "mixed takes highest",
			steps: []PlanStep{{ToolKey: "calendar_read"}, {ToolKey: "email_send"}},
			want:  "critical",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := assessRiskLevel(tt.steps)
			if got != tt.want {
				t.Errorf("assessRiskLevel()=%q want=%q", got, tt.want)
			}
		})
	}
}

func TestDeduplicate(t *testing.T) {
	t.Parallel()

	got := deduplicate([]string{"a", "b", "a", "c", "b"})
	want := []string{"a", "b", "c"}
	if len(got) != len(want) {
		t.Fatalf("got %v want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("got[%d]=%q want=%q", i, got[i], want[i])
		}
	}
}

func TestExecutionOrder(t *testing.T) {
	t.Parallel()

	steps := []PlanStep{
		{ToolKey: "verify", Phase: "verify", DependsOn: []int{1}},
		{ToolKey: "send", Phase: "act"},
		{ToolKey: "read", Phase: "gather"},
	}

	order := executionOrder(steps)
	// gather (idx 2) should come first, then act (idx 1), then verify (idx 0)
	if order[0] != 2 {
		t.Errorf("expected gather step first, got index %d", order[0])
	}
	if order[1] != 1 {
		t.Errorf("expected act step second, got index %d", order[1])
	}
	if order[2] != 0 {
		t.Errorf("expected verify step third, got index %d", order[2])
	}
}
