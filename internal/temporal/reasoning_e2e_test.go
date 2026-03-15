package temporal_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/brevio/brevio/internal/brain"
	"github.com/brevio/brevio/internal/cognition"
	"github.com/brevio/brevio/internal/llm"
)

func mockAnthropicServer(content string) *httptest.Server {
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		resp := map[string]any{
			"id": "msg_test", "type": "message", "role": "assistant",
			"content":     []map[string]any{{"type": "text", "text": content}},
			"model":       "claude-haiku-4-5-20251001",
			"stop_reason": "end_turn",
			"usage":       map[string]int{"input_tokens": 100, "output_tokens": 50},
		}
		json.NewEncoder(w).Encode(resp)
	}))
}

func TestReasoningPipeline_LLMPlanner_E2E(t *testing.T) {
	planJSON := `{"steps":[{"tool_key":"email_read","parameters":{"workspace_id":"ws1","query":"test"},"phase":"gather"},{"tool_key":"email_send","parameters":{"workspace_id":"ws1","to":"bob@example.com","subject":"Re: test","body":"Reply"},"phase":"act"}],"risk_level":"elevated","reasoning":"read then send"}`
	srv := mockAnthropicServer(planJSON)
	defer srv.Close()

	client, err := llm.NewAnthropicClient(llm.AnthropicConfig{APIKey: "test-key", BaseURL: srv.URL})
	if err != nil {
		t.Fatal(err)
	}

	loop := brain.NewReasoningLoop(brain.ReasoningLoopConfig{
		QualityTarget:   0.8,
		MaxIterations:   1,
		LLMClient:       client,
		RegisteredTools: brain.DefaultToolRegistry(),
	})

	rc := &brain.ReasoningContext{
		MessageID:   "msg-e2e-1",
		WorkspaceID: "ws1",
		Intent:      "send email to Bob with reply to his message",
		Confidence:  0.85,
	}

	result, err := loop.RunLoop(context.Background(), rc, 1)
	if err != nil {
		t.Fatal(err)
	}
	if result == nil || result.FinalPlan == nil {
		t.Fatal("expected non-nil result with plan")
	}
	if len(result.FinalPlan.Steps) < 1 {
		t.Fatal("expected at least 1 step")
	}
}

func TestReasoningPipeline_StepBack_E2E(t *testing.T) {
	goalJSON := `{"abstract_goal":"Send a follow-up email to Bob","confidence":0.9}`
	srv := mockAnthropicServer(goalJSON)
	defer srv.Close()

	client, _ := llm.NewAnthropicClient(llm.AnthropicConfig{APIKey: "test", BaseURL: srv.URL})
	sbSvc := brain.NewStepBackService(client)

	result, err := sbSvc.Infer(context.Background(), brain.StepBackRequest{
		WorkspaceID:      "ws1",
		RawMessage:       "hey can u send bob that reply",
		IntentConfidence: 0.60,
	})
	if err != nil {
		t.Fatal(err)
	}
	if result.Skipped {
		t.Fatal("expected step-back to run for low confidence")
	}
	if result.AbstractGoal == "" {
		t.Fatal("expected non-empty abstract goal")
	}
}

func TestReasoningPipeline_SemanticCritic_E2E(t *testing.T) {
	judgeJSON := `{"intent_satisfied":true,"quality_score":0.88,"completeness":0.9,"accuracy":0.85,"should_retry":false}`
	srv := mockAnthropicServer(judgeJSON)
	defer srv.Close()

	client, _ := llm.NewAnthropicClient(llm.AnthropicConfig{APIKey: "test", BaseURL: srv.URL})
	critic := brain.NewSemanticCriticService(client, 0.75)

	score, err := critic.Evaluate(context.Background(), brain.SemanticCriticRequest{
		OriginalIntent: "send email to Bob",
		Steps:          []brain.PlanStep{{ToolKey: "email_send", Phase: "act"}},
		Results:        []brain.StepResult{{StepIndex: 0, ToolKey: "email_send", Success: true}},
		Duration:       500 * time.Millisecond,
	})
	if err != nil {
		t.Fatal(err)
	}
	if !score.Passed {
		t.Fatal("expected passed")
	}
}

func TestReasoningPipeline_GoT_E2E(t *testing.T) {
	hypsJSON := `[{"content":"Negotiate assertively","score":0.85},{"content":"Seek common ground","score":0.75}]`
	callCount := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		callCount++
		w.Header().Set("Content-Type", "application/json")
		content := hypsJSON
		if callCount > 1 {
			content = "The best approach combines assertive negotiation with finding common ground."
		}
		json.NewEncoder(w).Encode(map[string]any{
			"id": "msg", "type": "message", "role": "assistant",
			"content":     []map[string]any{{"type": "text", "text": content}},
			"model":       "claude-haiku-4-5-20251001",
			"stop_reason": "end_turn",
			"usage":       map[string]int{"input_tokens": 50, "output_tokens": 50},
		})
	}))
	defer srv.Close()

	client, _ := llm.NewAnthropicClient(llm.AnthropicConfig{APIKey: "test", BaseURL: srv.URL})
	engine := cognition.NewGoTLLMEngine(cognition.GoTLLMConfig{
		LLMClient: client, MaxBranches: 2, PruneThreshold: 0.3, EnableMerge: true,
	})

	result, err := engine.Run(context.Background(), cognition.GoTRunRequest{
		Question: "How should I approach this negotiation?", MaxSteps: 1,
	})
	if err != nil {
		t.Fatal(err)
	}
	if result.Conclusion == "" {
		t.Fatal("expected non-empty conclusion")
	}
}

func TestReasoningPipeline_PRM_E2E(t *testing.T) {
	prmJSON := `{"score":4,"is_plausible":true,"reasoning":"Output looks correct."}`
	srv := mockAnthropicServer(prmJSON)
	defer srv.Close()

	client, _ := llm.NewAnthropicClient(llm.AnthropicConfig{APIKey: "test", BaseURL: srv.URL})
	prm := brain.NewProcessRewardModel(brain.PRMConfig{LLMClient: client, MinStepScore: 2.0, Enabled: true})

	reward, shouldContinue, err := prm.ScoreStep(
		context.Background(), "send email",
		[]brain.PlanStep{{ToolKey: "email_read", Phase: "gather"}},
		[]brain.StepResult{{StepIndex: 0, ToolKey: "email_read", Success: true}},
		brain.StepResult{StepIndex: 0, ToolKey: "email_read", Success: true},
	)
	if err != nil {
		t.Fatal(err)
	}
	if !shouldContinue {
		t.Fatal("expected continue")
	}
	if reward.Score != 4 {
		t.Fatalf("expected score 4, got %v", reward.Score)
	}
}

func TestReasoningPipeline_HeuristicFallback_E2E(t *testing.T) {
	loop := brain.NewReasoningLoop(brain.ReasoningLoopConfig{
		QualityTarget: 0.8, MaxIterations: 1, LLMClient: nil,
	})
	rc := &brain.ReasoningContext{
		MessageID: "msg-fallback", WorkspaceID: "ws1",
		Intent: "send an email to the team about the meeting", Confidence: 0.9,
	}
	result, err := loop.RunLoop(context.Background(), rc, 1)
	if err != nil {
		t.Fatal(err)
	}
	if result == nil || len(result.FinalPlan.Steps) == 0 {
		t.Fatal("expected heuristic plan")
	}
}

func TestRunLoop_ReflectorStepCalled(t *testing.T) {
	loop := brain.NewReasoningLoop(brain.ReasoningLoopConfig{
		QualityTarget: 0.8, MaxIterations: 1,
	})
	rc := &brain.ReasoningContext{
		WorkspaceID: "ws1", Intent: "send email to alice",
	}
	result, err := loop.RunLoop(context.Background(), rc, 1)
	if err != nil {
		t.Fatal(err)
	}
	if result == nil {
		t.Fatal("expected result")
	}
	if result.Lessons == nil {
		t.Fatal("expected Lessons field (may be empty but not nil)")
	}
	if result.Iterations != 1 {
		t.Fatalf("expected 1 iteration, got %d", result.Iterations)
	}
}
