package brain

import (
	"context"
	"errors"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/brevio/brevio/internal/cognition"
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

	assessment, err := rl.CriticStep(context.Background(), plan, result, "test request")
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

	assessment, err := rl.CriticStep(context.Background(), plan, result, "test request")
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
	assessment, err := rl.CriticStep(context.Background(), nil, nil, "")
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

// slowMockExecutor sleeps for a configured duration per tool key, recording
// start/end timestamps for concurrency verification.
type slowMockExecutor struct {
	mu        sync.Mutex
	delay     time.Duration
	errs      map[string]error
	starts    map[string]time.Time
	ends      map[string]time.Time
}

func newSlowMockExecutor(delay time.Duration) *slowMockExecutor {
	return &slowMockExecutor{
		delay:  delay,
		errs:   make(map[string]error),
		starts: make(map[string]time.Time),
		ends:   make(map[string]time.Time),
	}
}

func (m *slowMockExecutor) Execute(ctx context.Context, toolKey string, _ map[string]any) (map[string]any, error) {
	m.mu.Lock()
	m.starts[toolKey] = time.Now()
	m.mu.Unlock()

	if err, ok := m.errs[toolKey]; ok {
		m.mu.Lock()
		m.ends[toolKey] = time.Now()
		m.mu.Unlock()
		return nil, err
	}

	select {
	case <-time.After(m.delay):
	case <-ctx.Done():
		m.mu.Lock()
		m.ends[toolKey] = time.Now()
		m.mu.Unlock()
		return nil, ctx.Err()
	}

	m.mu.Lock()
	m.ends[toolKey] = time.Now()
	m.mu.Unlock()
	return map[string]any{"status": "ok"}, nil
}

func TestExecutorStep_ParallelIndependentSteps(t *testing.T) {
	t.Parallel()

	exec := newSlowMockExecutor(50 * time.Millisecond)
	rl := NewReasoningLoop(ReasoningLoopConfig{Executor: exec})

	plan := &Plan{
		Steps: []PlanStep{
			{ToolKey: "email_read", Parameters: map[string]any{}, Phase: "gather"},
			{ToolKey: "calendar_read", Parameters: map[string]any{}, Phase: "gather"},
			{ToolKey: "web_search", Parameters: map[string]any{}, Phase: "gather"},
		},
	}

	start := time.Now()
	result, err := rl.ExecutorStep(context.Background(), plan)
	elapsed := time.Since(start)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(result.Results) != 3 {
		t.Fatalf("expected 3 results, got %d", len(result.Results))
	}
	for i, r := range result.Results {
		if !r.Success {
			t.Errorf("step %d should have succeeded", i)
		}
	}
	// 3 independent 50ms steps running in parallel should complete well under 200ms.
	if elapsed >= 200*time.Millisecond {
		t.Errorf("expected parallel execution < 200ms, took %v", elapsed)
	}
}

func TestExecutorStep_DependentStepsExecuteInOrder(t *testing.T) {
	t.Parallel()

	exec := newSlowMockExecutor(20 * time.Millisecond)
	rl := NewReasoningLoop(ReasoningLoopConfig{Executor: exec})

	plan := &Plan{
		Steps: []PlanStep{
			{ToolKey: "step_0", Parameters: map[string]any{}, Phase: "gather"},
			{ToolKey: "step_1", Parameters: map[string]any{}, Phase: "act", DependsOn: []int{0}},
			{ToolKey: "step_2", Parameters: map[string]any{}, Phase: "verify", DependsOn: []int{1}},
		},
	}

	result, err := rl.ExecutorStep(context.Background(), plan)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	for i, r := range result.Results {
		if !r.Success {
			t.Errorf("step %d should have succeeded", i)
		}
	}

	exec.mu.Lock()
	defer exec.mu.Unlock()

	// step_1 must start after step_0 finishes.
	if exec.starts["step_1"].Before(exec.ends["step_0"]) {
		t.Error("step_1 started before step_0 finished")
	}
	// step_2 must start after step_1 finishes.
	if exec.starts["step_2"].Before(exec.ends["step_1"]) {
		t.Error("step_2 started before step_1 finished")
	}
}

func TestExecutorStep_FailedDependencySkipsDownstream(t *testing.T) {
	t.Parallel()

	exec := newSlowMockExecutor(10 * time.Millisecond)
	exec.errs["step_0"] = errors.New("step_0 failed")
	rl := NewReasoningLoop(ReasoningLoopConfig{Executor: exec})

	plan := &Plan{
		Steps: []PlanStep{
			{ToolKey: "step_0", Parameters: map[string]any{}, Phase: "gather"},
			{ToolKey: "step_1", Parameters: map[string]any{}, Phase: "act", DependsOn: []int{0}},
		},
	}

	result, err := rl.ExecutorStep(context.Background(), plan)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Results[0].Success {
		t.Error("step_0 should have failed")
	}
	if result.Results[1].Success {
		t.Error("step_1 should have been skipped")
	}
	if result.Results[1].Error != "skipped: dependency failed" {
		t.Errorf("expected 'skipped: dependency failed', got %q", result.Results[1].Error)
	}
	if !result.CompensationNeeded {
		t.Error("expected compensation needed")
	}
}

func TestExecutorStep_ContextCancelled(t *testing.T) {
	t.Parallel()

	exec := newSlowMockExecutor(500 * time.Millisecond)
	rl := NewReasoningLoop(ReasoningLoopConfig{Executor: exec})

	plan := &Plan{
		Steps: []PlanStep{
			{ToolKey: "slow_a", Parameters: map[string]any{}, Phase: "gather"},
			{ToolKey: "slow_b", Parameters: map[string]any{}, Phase: "gather"},
		},
	}

	ctx, cancel := context.WithCancel(context.Background())
	go func() {
		time.Sleep(50 * time.Millisecond)
		cancel()
	}()

	start := time.Now()
	result, err := rl.ExecutorStep(ctx, plan)
	elapsed := time.Since(start)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if elapsed >= 200*time.Millisecond {
		t.Errorf("expected cancellation within 200ms, took %v", elapsed)
	}
	for i, r := range result.Results {
		if r.Success {
			t.Errorf("step %d should have failed due to cancellation", i)
		}
	}
}

// ---------------------------------------------------------------------------
// Graph-of-Thought integration tests
// ---------------------------------------------------------------------------

func TestPlannerStep_GoT_LowConfidenceActivates(t *testing.T) {
	t.Parallel()

	gotEngine := cognition.NewGoTEngine()
	rl := NewReasoningLoop(ReasoningLoopConfig{
		GoTEngine:    gotEngine,
		GoTThreshold: 0.75,
	})
	rc := &ReasoningContext{
		MessageID:   "msg-got-1",
		WorkspaceID: "ws-1",
		Intent:      "send an email to John about the meeting",
		Confidence:  0.5, // below threshold — GoT should activate
	}

	// Without intelligence, GoT will fail (no GeneratePlan), but the
	// fallback heuristic planner should still produce a plan.
	plan, err := rl.PlannerStep(context.Background(), rc)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if plan == nil || len(plan.Steps) == 0 {
		t.Fatal("expected non-empty plan from fallback")
	}
}

func TestPlannerStep_GoT_HighConfidenceSkips(t *testing.T) {
	t.Parallel()

	gotEngine := cognition.NewGoTEngine()
	rl := NewReasoningLoop(ReasoningLoopConfig{
		GoTEngine:    gotEngine,
		GoTThreshold: 0.75,
	})
	rc := &ReasoningContext{
		MessageID:   "msg-got-2",
		WorkspaceID: "ws-1",
		Intent:      "check my calendar for tomorrow",
		Confidence:  0.9, // above threshold — GoT should NOT activate
	}

	plan, err := rl.PlannerStep(context.Background(), rc)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if plan == nil || len(plan.Steps) == 0 {
		t.Fatal("expected non-empty plan")
	}
}

func TestPlannerStep_GoT_FailsFallsBack(t *testing.T) {
	t.Parallel()

	// GoT engine is set, intelligence is nil — GoT path will fail,
	// should fall back to heuristic planner.
	gotEngine := cognition.NewGoTEngine()
	rl := NewReasoningLoop(ReasoningLoopConfig{
		GoTEngine:    gotEngine,
		GoTThreshold: 0.75,
	})
	rc := &ReasoningContext{
		MessageID:   "msg-got-3",
		WorkspaceID: "ws-1",
		Intent:      "send reply to the email about budget",
		Confidence:  0.4, // low confidence, GoT would try but intelligence is nil
	}

	plan, err := rl.PlannerStep(context.Background(), rc)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if plan == nil || len(plan.Steps) == 0 {
		t.Fatal("expected non-empty plan from fallback path")
	}
	// Should have email-related steps from heuristic fallback.
	found := false
	for _, s := range plan.Steps {
		if s.ToolKey == "email_read" || s.ToolKey == "email_send" {
			found = true
			break
		}
	}
	if !found {
		t.Error("expected email-related steps from heuristic fallback")
	}
}

func TestGenerateHypotheses(t *testing.T) {
	t.Parallel()

	hyps := generateHypotheses("Send an email", "", 3)
	if len(hyps) != 3 {
		t.Fatalf("expected 3 hypotheses, got %d", len(hyps))
	}
	if hyps[0] != "Send an email" {
		t.Errorf("first hypothesis should be the literal intent, got %q", hyps[0])
	}

	single := generateHypotheses("test", "", 1)
	if len(single) != 1 || single[0] != "test" {
		t.Errorf("single hypothesis should be the literal, got %v", single)
	}
}

// ---------------------------------------------------------------------------
// Dynamic iteration budget tests
// ---------------------------------------------------------------------------

func TestComputeIterationsFromSignals_Simple(t *testing.T) {
	t.Parallel()

	result := computeIterationsFromSignals(ComplexitySignals{
		IntentCount:            1,
		DomainCount:            1,
		HasDependencies:        false,
		HasTemporalConstraints: false,
	})
	if result != 1 {
		t.Errorf("expected 1 for simple signals, got %d", result)
	}
}

func TestComputeIterationsFromSignals_Complex(t *testing.T) {
	t.Parallel()

	result := computeIterationsFromSignals(ComplexitySignals{
		IntentCount:            3,
		DomainCount:            2,
		HasDependencies:        true,
		HasTemporalConstraints: true,
	})
	if result < 4 || result > 8 {
		t.Errorf("expected result in [4, 8] for complex signals, got %d", result)
	}
}

func TestRunLoop_DynamicBudget_UsesHigherValue(t *testing.T) {
	t.Parallel()

	exec := newMockExecutor()
	exec.results["email_read"] = map[string]any{"emails": []string{"hi"}}
	exec.results["email_send"] = map[string]any{"sent": true}
	exec.results["verify_output"] = map[string]any{"verified": true}

	decomp := NewDynamicDecompositionService()
	rl := NewReasoningLoop(ReasoningLoopConfig{
		Executor:      exec,
		MaxIterations: 1,
		Decomposition: decomp,
	})

	// Use a complex intent that yields dynamic > 1.
	rc := &ReasoningContext{
		MessageID:   "msg-dyn-1",
		WorkspaceID: "ws-1",
		Intent:      "send an email to John and schedule a meeting with Sarah and also search for the quarterly report document",
		Confidence:  0.9,
	}

	result, err := rl.RunLoop(context.Background(), rc, 1)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.DynamicBudget <= 1 {
		t.Errorf("expected dynamic budget > 1 for complex intent, got %d", result.DynamicBudget)
	}
}

func TestRunLoop_DynamicBudget_CapApplied(t *testing.T) {
	t.Parallel()

	exec := newMockExecutor()
	exec.results["email_read"] = map[string]any{"emails": []string{"hi"}}
	exec.results["email_send"] = map[string]any{"sent": true}
	exec.results["verify_output"] = map[string]any{"verified": true}

	decomp := NewDynamicDecompositionService()
	rl := NewReasoningLoop(ReasoningLoopConfig{
		Executor:         exec,
		MaxIterations:    20,
		MaxIterationsCap: 8,
		Decomposition:    decomp,
	})

	rc := &ReasoningContext{
		MessageID:   "msg-dyn-2",
		WorkspaceID: "ws-1",
		Intent:      "send an email to John about quarterly budget and schedule review meetings with ten people and compile financial documents",
		Confidence:  0.9,
	}

	result, err := rl.RunLoop(context.Background(), rc, 20)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.DynamicBudget > 8 {
		t.Errorf("expected dynamic budget capped at 8, got %d", result.DynamicBudget)
	}
}

// --- CompressConversation tests ---

type mockSummarizer struct {
	summary string
	called  bool
}

func (m *mockSummarizer) SummarizeText(_ context.Context, _ string, _ int) (string, error) {
	m.called = true
	return m.summary, nil
}

func TestCompressConversation_WithinBudget_NoCompression(t *testing.T) {
	t.Parallel()
	history := []Message{{Role: "user", Content: "hi"}}
	mock := &mockSummarizer{summary: "summary"}
	got, err := CompressConversation(context.Background(), history, 1_000_000, 6, mock)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if mock.called {
		t.Error("summarizer should not be called when within budget")
	}
	if len(got) != len(history) {
		t.Errorf("expected original history, got %d messages", len(got))
	}
}

func TestCompressConversation_ExceedsBudget_Compresses(t *testing.T) {
	t.Parallel()
	history := make([]Message, 10)
	for i := range history {
		history[i] = Message{Role: "user", Content: strings.Repeat("x", 100)}
	}
	mock := &mockSummarizer{summary: "Earlier: user asked about x."}
	compressed, _ := CompressConversation(context.Background(), history, 200, 3, mock)
	if !mock.called {
		t.Error("summarizer should have been called")
	}
	if len(compressed) == 0 {
		t.Fatal("expected non-empty compressed history")
	}
	if compressed[0].Metadata == nil {
		t.Error("expected metadata on summary message")
	}
	if _, ok := compressed[0].Metadata["compressed"]; !ok {
		t.Error("expected compressed=true metadata")
	}
}

func TestCompressConversation_NilSummarizer_ReturnsOriginal(t *testing.T) {
	t.Parallel()
	history := []Message{{Role: "user", Content: "hello"}}
	got, err := CompressConversation(context.Background(), history, 100, 3, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(got) != 1 {
		t.Errorf("expected original 1-message history, got %d", len(got))
	}
}

func TestCompressConversation_TooFewMessages_ReturnsOriginal(t *testing.T) {
	t.Parallel()
	history := []Message{
		{Role: "user", Content: "hi"},
		{Role: "assistant", Content: "hello"},
	}
	mock := &mockSummarizer{}
	got, _ := CompressConversation(context.Background(), history, 1, 6, mock)
	if mock.called {
		t.Error("summarizer should not be called with too few messages")
	}
	if len(got) != 2 {
		t.Errorf("expected 2 messages, got %d", len(got))
	}
}
