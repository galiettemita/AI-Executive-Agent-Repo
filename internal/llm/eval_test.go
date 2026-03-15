package llm

import (
	"context"
	"testing"
)

// mockClientForEval implements Client for eval testing with canned responses.
type mockClientForEval struct {
	classifyJSON string
	planJSON     string
}

func (m *mockClientForEval) Generate(_ context.Context, req GenerateRequest) (*GenerateResponse, *Usage, error) {
	// Classify calls use T0 tier (small MaxTokens); plan calls use T2/T3.
	content := m.planJSON
	if req.MaxTokens <= 512 {
		content = m.classifyJSON
	}
	return &GenerateResponse{Content: content, Model: "mock", ProviderID: "mock"}, &Usage{}, nil
}

func (m *mockClientForEval) Stream(_ context.Context, _ GenerateRequest, out chan<- StreamChunk) {
	defer close(out)
	out <- StreamChunk{Done: true}
}

func TestEvaluator_IntentAndToolMatch(t *testing.T) {
	t.Parallel()

	classifyJSON := `{"intent":"calendar_create","confidence":0.92,"skills":["google_calendar"],"requires_decomposition":false,"reasoning":"scheduling intent"}`
	planJSON := `{"intent":"calendar_create","confidence":0.92,"actions":[{"tool_key":"google_calendar.create_event","operation":"create","phase":"act"}],"tools":["google_calendar.create_event"],"risk_level":"elevated","reasoning":"create event","final_answer_requirements":"event created successfully"}`

	mock := &mockClientForEval{classifyJSON: classifyJSON, planJSON: planJSON}
	intel := NewIntelligenceService(IntelligenceConfig{
		Classifier:  mock,
		Planner:     mock,
		Synthesizer: mock,
	})
	eval := NewEvaluator(intel)

	cases := []EvalCase{
		{
			ID:               "tc_calendar_01",
			Input:            "Schedule a meeting with Alice tomorrow at 3pm",
			ExpectedIntent:   "calendar_create",
			ExpectedToolKeys: []string{"google_calendar.create_event"},
			MinConfidence:    0.80,
		},
	}

	results := eval.RunBatch(context.Background(), cases)
	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}
	r := results[0]
	if r.Err != nil {
		t.Fatalf("unexpected error: %v", r.Err)
	}
	if !r.IntentMatch {
		t.Errorf("intent: got %q, want %q", r.ClassifiedIntent, "calendar_create")
	}
	if !r.ToolKeyMatch {
		t.Errorf("tool keys: got %v, want to contain %v", r.PlannedToolKeys, cases[0].ExpectedToolKeys)
	}
	if !r.ConfidenceOK {
		t.Errorf("confidence should be >= 0.80")
	}
}

func TestEvaluator_IntentMismatch(t *testing.T) {
	t.Parallel()

	classifyJSON := `{"intent":"email_send","confidence":0.85,"skills":["gmail"],"requires_decomposition":false,"reasoning":"email intent"}`
	planJSON := `{"intent":"email_send","confidence":0.85,"actions":[{"tool_key":"gmail.send","operation":"send","phase":"act"}],"tools":["gmail.send"],"risk_level":"elevated","reasoning":"send email","final_answer_requirements":"email sent"}`

	mock := &mockClientForEval{classifyJSON: classifyJSON, planJSON: planJSON}
	intel := NewIntelligenceService(IntelligenceConfig{Classifier: mock, Planner: mock, Synthesizer: mock})
	eval := NewEvaluator(intel)

	cases := []EvalCase{{
		ID:             "tc_mismatch_01",
		Input:          "Book me a flight",
		ExpectedIntent: "travel_book",
		MinConfidence:  0.80,
	}}

	results := eval.RunBatch(context.Background(), cases)
	r := results[0]
	if r.IntentMatch {
		t.Error("expected IntentMatch=false for mismatched intent")
	}
	if r.Pass {
		t.Error("expected Pass=false when intent does not match")
	}
}

func TestEvalToolKeySubsetMatch(t *testing.T) {
	t.Parallel()
	if !evalToolKeySubsetMatch(nil, []string{"a", "b"}) {
		t.Error("nil expected should always match")
	}
	if !evalToolKeySubsetMatch([]string{}, []string{"a"}) {
		t.Error("empty expected should always match")
	}
	if !evalToolKeySubsetMatch([]string{"a"}, []string{"a", "b", "c"}) {
		t.Error("subset should match")
	}
	if evalToolKeySubsetMatch([]string{"d"}, []string{"a", "b", "c"}) {
		t.Error("non-subset should not match")
	}
}
