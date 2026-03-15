package llm

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

// ---------------------------------------------------------------------------
// Strict plan schema validation tests
// ---------------------------------------------------------------------------

func TestValidateStrictPlanJSON_Valid(t *testing.T) {
	t.Parallel()

	raw := `{
		"intent": "email_query",
		"confidence": 0.9,
		"actions": [
			{"tool_key": "email.read", "operation": "fetch inbox", "phase": "gather"},
			{"tool_key": "email.send", "operation": "reply", "phase": "act", "depends_on": [0]}
		],
		"tools": ["email.read", "email.send"],
		"risk_level": "elevated",
		"reasoning": "User wants to read and reply to email",
		"final_answer_requirements": "Email was sent successfully"
	}`

	plan, err := ValidateStrictPlanJSON(raw)
	if err != nil {
		t.Fatalf("expected valid plan, got error: %v", err)
	}
	if plan.Intent != "email_query" {
		t.Errorf("expected intent 'email_query', got %q", plan.Intent)
	}
	if len(plan.Actions) != 2 {
		t.Errorf("expected 2 actions, got %d", len(plan.Actions))
	}
	// canonicalize should sort actions by phase then tool_key.
	if plan.Actions[0].Phase != "gather" {
		t.Errorf("expected first action phase 'gather', got %q", plan.Actions[0].Phase)
	}
	if plan.Actions[1].Phase != "act" {
		t.Errorf("expected second action phase 'act', got %q", plan.Actions[1].Phase)
	}
	// Tools list should be sorted and deduplicated.
	if len(plan.Tools) != 2 || plan.Tools[0] != "email.read" || plan.Tools[1] != "email.send" {
		t.Errorf("unexpected tools: %v", plan.Tools)
	}
}

func TestValidateStrictPlanJSON_MarkdownFenced(t *testing.T) {
	t.Parallel()

	raw := "```json\n" + `{
		"intent": "search",
		"confidence": 0.8,
		"actions": [{"tool_key": "web.search", "operation": "search", "phase": "gather"}],
		"tools": ["web.search"],
		"risk_level": "low",
		"reasoning": "simple search"
	}` + "\n```"

	plan, err := ValidateStrictPlanJSON(raw)
	if err != nil {
		t.Fatalf("expected valid plan from markdown-fenced JSON, got: %v", err)
	}
	if plan.Intent != "search" {
		t.Errorf("expected intent 'search', got %q", plan.Intent)
	}
}

func TestValidateStrictPlanJSON_EmptyIntent(t *testing.T) {
	t.Parallel()

	raw := `{
		"intent": "",
		"confidence": 0.9,
		"actions": [{"tool_key": "echo", "operation": "test", "phase": "gather"}],
		"tools": ["echo"],
		"risk_level": "low",
		"reasoning": "test"
	}`

	_, err := ValidateStrictPlanJSON(raw)
	if err == nil {
		t.Fatal("expected error for empty intent")
	}
}

func TestValidateStrictPlanJSON_NoActions(t *testing.T) {
	t.Parallel()

	raw := `{
		"intent": "test",
		"confidence": 0.5,
		"actions": [],
		"tools": [],
		"risk_level": "low",
		"reasoning": "nothing"
	}`

	_, err := ValidateStrictPlanJSON(raw)
	if err == nil {
		t.Fatal("expected error for empty actions")
	}
}

func TestValidateStrictPlanJSON_TooManyActions(t *testing.T) {
	t.Parallel()

	actions := make([]map[string]any, 9)
	for i := range actions {
		actions[i] = map[string]any{
			"tool_key": "echo", "operation": "test", "phase": "gather",
		}
	}
	blob, _ := json.Marshal(map[string]any{
		"intent": "test", "confidence": 0.5, "actions": actions,
		"tools": []string{"echo"}, "risk_level": "low", "reasoning": "too many",
	})

	_, err := ValidateStrictPlanJSON(string(blob))
	if err == nil {
		t.Fatal("expected error for >8 actions")
	}
}

func TestValidateStrictPlanJSON_InvalidToolKeyFormat(t *testing.T) {
	t.Parallel()

	raw := `{
		"intent": "test",
		"confidence": 0.5,
		"actions": [{"tool_key": "INVALID-KEY!", "operation": "test", "phase": "gather"}],
		"tools": ["INVALID-KEY!"],
		"risk_level": "low",
		"reasoning": "bad key"
	}`

	_, err := ValidateStrictPlanJSON(raw)
	if err == nil {
		t.Fatal("expected error for invalid tool_key format")
	}
}

func TestValidateStrictPlanJSON_InvalidPhase(t *testing.T) {
	t.Parallel()

	raw := `{
		"intent": "test",
		"confidence": 0.5,
		"actions": [{"tool_key": "echo", "operation": "test", "phase": "unknown"}],
		"tools": ["echo"],
		"risk_level": "low",
		"reasoning": "bad phase"
	}`

	_, err := ValidateStrictPlanJSON(raw)
	if err == nil {
		t.Fatal("expected error for invalid phase")
	}
}

func TestValidateStrictPlanJSON_GarbageInput(t *testing.T) {
	t.Parallel()

	_, err := ValidateStrictPlanJSON("this is not json at all")
	if err == nil {
		t.Fatal("expected error for garbage input")
	}
}

func TestValidateStrictPlanJSON_DependsOnPreserved(t *testing.T) {
	t.Parallel()

	raw := `{
		"intent": "multi_step",
		"confidence": 0.85,
		"actions": [
			{"tool_key": "email.read", "operation": "fetch", "phase": "gather"},
			{"tool_key": "email.send", "operation": "reply", "phase": "act", "depends_on": [0]},
			{"tool_key": "email.verify", "operation": "check", "phase": "verify", "depends_on": [0, 1]}
		],
		"tools": ["email.read", "email.send", "email.verify"],
		"risk_level": "elevated",
		"reasoning": "multi-step email flow"
	}`

	plan, err := ValidateStrictPlanJSON(raw)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// After canonicalization, verify phase should be last.
	lastAction := plan.Actions[len(plan.Actions)-1]
	if lastAction.Phase != "verify" {
		t.Errorf("expected last action to be verify phase, got %q", lastAction.Phase)
	}
	if len(lastAction.DependsOn) != 2 {
		t.Errorf("expected depends_on with 2 items, got %v", lastAction.DependsOn)
	}
}

// ---------------------------------------------------------------------------
// IntelligenceService with httptest — end-to-end classification
// ---------------------------------------------------------------------------

func TestIntelligenceService_ClassifyIntent_WithHTTPTest(t *testing.T) {
	t.Parallel()

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		resp := anthropicResponse{
			ID:   "msg_test",
			Model: ModelAnthropicHaiku,
			Content: []anthropicContentBlock{{
				Type: "text",
				Text: `{"intent":"calendar_management","confidence":0.91,"skills":["calendar.read","calendar.write"],"requires_decomposition":false,"reasoning":"User wants to schedule a meeting"}`,
			}},
			StopReason: "end_turn",
			Usage:      anthropicUsage{InputTokens: 100, OutputTokens: 50},
		}
		json.NewEncoder(w).Encode(resp)
	}))
	defer ts.Close()

	client, _ := NewAnthropicClient(AnthropicConfig{
		APIKey:  "test-key",
		BaseURL: ts.URL,
		Timeout: 5 * time.Second,
	})

	intel := NewIntelligenceService(IntelligenceConfig{
		Classifier: client,
	})

	result, usage, err := intel.ClassifyIntent(context.Background(), "schedule a meeting with John tomorrow", "ws-001")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Intent != "calendar_management" {
		t.Errorf("expected intent 'calendar_management', got %q", result.Intent)
	}
	if result.Confidence < 0.9 {
		t.Errorf("expected confidence >= 0.9, got %f", result.Confidence)
	}
	if len(result.Skills) != 2 {
		t.Errorf("expected 2 skills, got %d", len(result.Skills))
	}
	if usage == nil || usage.InputTokens == 0 {
		t.Error("expected non-zero usage")
	}
}

// ---------------------------------------------------------------------------
// IntelligenceService — plan generation with httptest
// ---------------------------------------------------------------------------

func TestIntelligenceService_GeneratePlan_WithHTTPTest(t *testing.T) {
	t.Parallel()

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		resp := anthropicResponse{
			ID:    "msg_plan",
			Model: "claude-sonnet-4-20250514",
			Content: []anthropicContentBlock{{
				Type: "text",
				Text: `{
					"intent": "calendar_management",
					"confidence": 0.9,
					"actions": [
						{"tool_key": "calendar.read", "operation": "check availability", "phase": "gather"},
						{"tool_key": "calendar.write", "operation": "create event", "phase": "act", "depends_on": [0]}
					],
					"tools": ["calendar.read", "calendar.write"],
					"risk_level": "elevated",
					"reasoning": "Need to check availability then create event",
					"final_answer_requirements": "Event created at the correct time"
				}`,
			}},
			StopReason: "end_turn",
			Usage:      anthropicUsage{InputTokens: 200, OutputTokens: 100},
		}
		json.NewEncoder(w).Encode(resp)
	}))
	defer ts.Close()

	client, _ := NewAnthropicClient(AnthropicConfig{
		APIKey:  "test-key",
		BaseURL: ts.URL,
		Timeout: 5 * time.Second,
	})

	intel := NewIntelligenceService(IntelligenceConfig{
		Planner: client,
	})

	plan, usage, err := intel.GeneratePlan(context.Background(), "calendar_management", 0.9, "schedule meeting with John", "", "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(plan.Actions) < 1 {
		t.Fatal("expected at least 1 action in plan")
	}
	if len(plan.Tools) < 1 {
		t.Fatal("expected at least 1 tool in plan")
	}
	if plan.FinalAnswerRequirements == "" {
		t.Error("expected non-empty final_answer_requirements")
	}
	if usage == nil || usage.InputTokens == 0 {
		t.Error("expected non-zero usage")
	}
}

// ---------------------------------------------------------------------------
// IntelligenceService — verify execution with httptest
// ---------------------------------------------------------------------------

func TestIntelligenceService_VerifyExecution_Pass(t *testing.T) {
	t.Parallel()

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		resp := anthropicResponse{
			ID:    "msg_verify",
			Model: "claude-sonnet-4-20250514",
			Content: []anthropicContentBlock{{
				Type: "text",
				Text: `{"verdict":"pass","reasons":["all tools executed successfully","results match requirements"],"retry_hints":""}`,
			}},
			Usage: anthropicUsage{InputTokens: 150, OutputTokens: 30},
		}
		json.NewEncoder(w).Encode(resp)
	}))
	defer ts.Close()

	client, _ := NewAnthropicClient(AnthropicConfig{
		APIKey:  "test-key",
		BaseURL: ts.URL,
		Timeout: 5 * time.Second,
	})

	intel := NewIntelligenceService(IntelligenceConfig{
		Synthesizer: client,
	})

	result, _, err := intel.VerifyExecution(context.Background(), VerifyInput{
		OriginalRequest: "schedule meeting",
		Plan: &GeneratedPlan{
			Intent:    "calendar_management",
			Tools:     []string{"calendar.write"},
			RiskLevel: "elevated",
			FinalAnswerRequirements: "Event created",
		},
		ToolOutputs: []ToolOutputForVerify{
			{ToolKey: "calendar.write", Success: true, Phase: "commit"},
		},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Verdict != "pass" {
		t.Errorf("expected pass verdict, got %q", result.Verdict)
	}
}

func TestIntelligenceService_VerifyExecution_Fail(t *testing.T) {
	t.Parallel()

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		resp := anthropicResponse{
			ID:    "msg_verify",
			Model: "claude-sonnet-4-20250514",
			Content: []anthropicContentBlock{{
				Type: "text",
				Text: `{"verdict":"fail","reasons":["calendar.write returned error"],"retry_hints":"Try alternative time slot"}`,
			}},
			Usage: anthropicUsage{InputTokens: 150, OutputTokens: 30},
		}
		json.NewEncoder(w).Encode(resp)
	}))
	defer ts.Close()

	client, _ := NewAnthropicClient(AnthropicConfig{
		APIKey:  "test-key",
		BaseURL: ts.URL,
		Timeout: 5 * time.Second,
	})

	intel := NewIntelligenceService(IntelligenceConfig{
		Synthesizer: client,
	})

	result, _, err := intel.VerifyExecution(context.Background(), VerifyInput{
		OriginalRequest: "schedule meeting",
		Plan: &GeneratedPlan{
			Intent:    "calendar_management",
			Tools:     []string{"calendar.write"},
			RiskLevel: "elevated",
			FinalAnswerRequirements: "Event created",
		},
		ToolOutputs: []ToolOutputForVerify{
			{ToolKey: "calendar.write", Success: false, Phase: "commit"},
		},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Verdict != "fail" {
		t.Errorf("expected fail verdict, got %q", result.Verdict)
	}
	if result.RetryHints == "" {
		t.Error("expected non-empty retry_hints on fail")
	}
}

// ---------------------------------------------------------------------------
// extractJSON tests
// ---------------------------------------------------------------------------

func TestExtractJSON(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name     string
		input    string
		expected string
	}{
		{"bare json", `{"key":"value"}`, `{"key":"value"}`},
		{"with surrounding text", `Sure! Here's the result: {"key":"value"} Hope that helps!`, `{"key":"value"}`},
		{"markdown fenced", "```json\n{\"key\":\"value\"}\n```", `{"key":"value"}`},
		{"markdown generic fence", "```\n{\"key\":\"value\"}\n```", `{"key":"value"}`},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := extractJSON(tc.input)
			if got != tc.expected {
				t.Errorf("extractJSON(%q) = %q, want %q", tc.input, got, tc.expected)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// ToolRegistry integration — planner refuses unknown tool_keys
// ---------------------------------------------------------------------------

type fakeToolRegistry struct {
	tools map[string]bool
}

func (r *fakeToolRegistry) ToolKeys() []string {
	keys := make([]string, 0, len(r.tools))
	for k := range r.tools {
		keys = append(keys, k)
	}
	return keys
}

func (r *fakeToolRegistry) HasTool(toolKey string) bool {
	return r.tools[toolKey]
}

func TestIntelligenceService_GeneratePlan_RefusesUnknownTools(t *testing.T) {
	t.Parallel()

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		resp := anthropicResponse{
			ID:    "msg_plan_unknown",
			Model: "claude-sonnet-4-20250514",
			Content: []anthropicContentBlock{{
				Type: "text",
				Text: `{
					"intent": "email_query",
					"confidence": 0.9,
					"actions": [
						{"tool_key": "google_gmail.read_email", "operation": "read", "phase": "gather"},
						{"tool_key": "unknown_tool.do_stuff", "operation": "act", "phase": "act"}
					],
					"tools": ["google_gmail.read_email", "unknown_tool.do_stuff"],
					"risk_level": "low",
					"reasoning": "test",
					"final_answer_requirements": "test"
				}`,
			}},
			StopReason: "end_turn",
			Usage:      anthropicUsage{InputTokens: 100, OutputTokens: 50},
		}
		json.NewEncoder(w).Encode(resp)
	}))
	defer ts.Close()

	client, _ := NewAnthropicClient(AnthropicConfig{
		APIKey:  "test-key",
		BaseURL: ts.URL,
		Timeout: 5 * time.Second,
	})

	registry := &fakeToolRegistry{tools: map[string]bool{
		"google_gmail.read_email": true,
		"google_gmail.send_email": true,
	}}

	intel := NewIntelligenceService(IntelligenceConfig{
		Planner:      client,
		ToolRegistry: registry,
	})

	_, _, err := intel.GeneratePlan(context.Background(), "email_query", 0.9, "read my emails", "", "")
	if err == nil {
		t.Fatal("expected error for unknown tool_key, got nil")
	}
	if !contains(err.Error(), "unknown tool keys") {
		t.Errorf("expected error about unknown tool keys, got: %v", err)
	}
	if !contains(err.Error(), "unknown_tool.do_stuff") {
		t.Errorf("expected error to mention the unknown tool, got: %v", err)
	}
	if !contains(err.Error(), "google_gmail.read_email") {
		t.Errorf("expected error to list available tools, got: %v", err)
	}
}

func TestIntelligenceService_GeneratePlan_AcceptsKnownTools(t *testing.T) {
	t.Parallel()

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		resp := anthropicResponse{
			ID:    "msg_plan_known",
			Model: "claude-sonnet-4-20250514",
			Content: []anthropicContentBlock{{
				Type: "text",
				Text: `{
					"intent": "email_query",
					"confidence": 0.9,
					"actions": [
						{"tool_key": "google_gmail.read_email", "operation": "read", "phase": "gather"}
					],
					"tools": ["google_gmail.read_email"],
					"risk_level": "low",
					"reasoning": "test",
					"final_answer_requirements": "test"
				}`,
			}},
			StopReason: "end_turn",
			Usage:      anthropicUsage{InputTokens: 100, OutputTokens: 50},
		}
		json.NewEncoder(w).Encode(resp)
	}))
	defer ts.Close()

	client, _ := NewAnthropicClient(AnthropicConfig{
		APIKey:  "test-key",
		BaseURL: ts.URL,
		Timeout: 5 * time.Second,
	})

	registry := &fakeToolRegistry{tools: map[string]bool{
		"google_gmail.read_email": true,
		"google_gmail.send_email": true,
	}}

	intel := NewIntelligenceService(IntelligenceConfig{
		Planner:      client,
		ToolRegistry: registry,
	})

	plan, _, err := intel.GeneratePlan(context.Background(), "email_query", 0.9, "read my emails", "", "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if plan.Intent != "email_query" {
		t.Errorf("expected intent email_query, got %q", plan.Intent)
	}
}

func TestIntelligenceService_GeneratePlan_NoRegistrySkipsValidation(t *testing.T) {
	t.Parallel()

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		resp := anthropicResponse{
			ID:    "msg_plan_noreg",
			Model: "claude-sonnet-4-20250514",
			Content: []anthropicContentBlock{{
				Type: "text",
				Text: `{
					"intent": "general",
					"confidence": 0.8,
					"actions": [
						{"tool_key": "any_tool.whatever", "operation": "do", "phase": "gather"}
					],
					"tools": ["any_tool.whatever"],
					"risk_level": "low",
					"reasoning": "test",
					"final_answer_requirements": "test"
				}`,
			}},
			StopReason: "end_turn",
			Usage:      anthropicUsage{InputTokens: 100, OutputTokens: 50},
		}
		json.NewEncoder(w).Encode(resp)
	}))
	defer ts.Close()

	client, _ := NewAnthropicClient(AnthropicConfig{
		APIKey:  "test-key",
		BaseURL: ts.URL,
		Timeout: 5 * time.Second,
	})

	// No registry — should pass without validation.
	intel := NewIntelligenceService(IntelligenceConfig{
		Planner: client,
	})

	plan, _, err := intel.GeneratePlan(context.Background(), "general", 0.8, "do something", "", "")
	if err != nil {
		t.Fatalf("unexpected error without registry: %v", err)
	}
	if plan == nil {
		t.Fatal("expected non-nil plan")
	}
}

// ---------------------------------------------------------------------------
// ValidateToolKey tests
// ---------------------------------------------------------------------------

func TestValidateToolKey_ValidKeys(t *testing.T) {
	t.Parallel()

	valid := []string{
		"web_search",
		"google_gmail.read_email",
		"some.tool.v2",
		"a.b.c.d",
		"namespace_one.tool_two.version_three",
	}
	for _, key := range valid {
		if err := ValidateToolKey(key); err != nil {
			t.Errorf("expected %q to be valid, got error: %v", key, err)
		}
	}
}

func TestValidateToolKey_InvalidKeys(t *testing.T) {
	t.Parallel()

	invalid := []string{
		"",
		"Gmail",
		"my tool",
		"a..b",
		".start",
		"a.b.c.d.e",
	}
	for _, key := range invalid {
		if err := ValidateToolKey(key); err == nil {
			t.Errorf("expected %q to be invalid, got nil error", key)
		}
	}
}

func TestValidateToolKey_ThreeSegment_PreviouslyRejected(t *testing.T) {
	t.Parallel()

	// "some.tool.v2" was rejected by the old single-dot regex.
	if err := ValidateToolKey("some.tool.v2"); err != nil {
		t.Errorf("three-segment key should now be valid, got error: %v", err)
	}
	if err := ValidateToolKey("google_gmail.read_email.v2"); err != nil {
		t.Errorf("three-segment key should now be valid, got error: %v", err)
	}
}

func TestValidatePlanToolRegistryKeys_MissingKey(t *testing.T) {
	t.Parallel()

	plan := &GeneratedPlan{
		Actions: []PlanAction{
			{ToolKey: "web_search", Phase: "gather"},
			{ToolKey: "unknown_tool", Phase: "act"},
		},
	}
	registry := &fakeToolRegistry{tools: map[string]bool{
		"web_search":    true,
		"calendar_read": true,
	}}

	err := validatePlanToolRegistryKeys(plan, registry)
	if err == nil {
		t.Fatal("expected error for unknown tool key")
	}
	if !contains(err.Error(), "unknown_tool") {
		t.Errorf("expected error to mention unknown_tool, got: %v", err)
	}
	if !contains(err.Error(), "web_search") {
		t.Errorf("expected error to list available tools, got: %v", err)
	}
}

func TestValidatePlanToolRegistryKeys_NilRegistry(t *testing.T) {
	t.Parallel()

	plan := &GeneratedPlan{
		Actions: []PlanAction{
			{ToolKey: "anything", Phase: "gather"},
		},
	}
	if err := validatePlanToolRegistryKeys(plan, nil); err != nil {
		t.Errorf("nil registry should skip validation, got: %v", err)
	}
}

// ---------------------------------------------------------------------------
// MoA tests
// ---------------------------------------------------------------------------

func TestGeneratePlanMoA_Disabled_DelegatesToGeneratePlan(t *testing.T) {
	t.Parallel()

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		resp := anthropicResponse{
			ID:    "msg_moa_disabled",
			Model: "claude-sonnet-4-20250514",
			Content: []anthropicContentBlock{{
				Type: "text",
				Text: `{
					"intent": "email_query",
					"confidence": 0.9,
					"actions": [{"tool_key": "email_read", "operation": "read", "phase": "gather"}],
					"tools": ["email_read"],
					"risk_level": "low",
					"reasoning": "simple read",
					"final_answer_requirements": "email read"
				}`,
			}},
			StopReason: "end_turn",
			Usage:      anthropicUsage{InputTokens: 100, OutputTokens: 50},
		}
		json.NewEncoder(w).Encode(resp)
	}))
	defer ts.Close()

	client, _ := NewAnthropicClient(AnthropicConfig{
		APIKey:  "test-key",
		BaseURL: ts.URL,
		Timeout: 5 * time.Second,
	})

	intel := NewIntelligenceService(IntelligenceConfig{Planner: client})

	plan, _, err := intel.GeneratePlanMoA(
		context.Background(), "email_query", 0.9, "read my emails", "", "",
		MoAConfig{Enabled: false},
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if plan == nil {
		t.Fatal("expected non-nil plan")
	}
	if plan.Intent != "email_query" {
		t.Errorf("expected intent=email_query, got %s", plan.Intent)
	}
}

func TestArbitratePlans_SingleProposal_ReturnsDirectly(t *testing.T) {
	t.Parallel()

	intel := NewIntelligenceService(IntelligenceConfig{})

	proposal := GeneratedPlan{
		Intent:     "test",
		Confidence: 0.9,
		Actions:    []PlanAction{{ToolKey: "web_search", Phase: "gather", Operation: "search"}},
		Tools:      []string{"web_search"},
		RiskLevel:  "low",
		Reasoning:  "single plan",
	}

	result, _, err := intel.ArbitratePlans(context.Background(), "test", []GeneratedPlan{proposal})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Rationale != "only one proposal" {
		t.Errorf("expected 'only one proposal', got %q", result.Rationale)
	}
	if result.SelectedPlan.Intent != "test" {
		t.Errorf("expected plan to be returned unchanged")
	}
}

func TestArbitratePlans_NoProposals_ReturnsError(t *testing.T) {
	t.Parallel()

	intel := NewIntelligenceService(IntelligenceConfig{})

	_, _, err := intel.ArbitratePlans(context.Background(), "test", []GeneratedPlan{})
	if err == nil {
		t.Fatal("expected error for empty proposals")
	}
}

func TestGeneratePlanMoA_AllProposalsFail_FallsBack(t *testing.T) {
	t.Parallel()

	var callCount int64
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		n := atomic.AddInt64(&callCount, 1)
		// First N calls fail (proposals), then the fallback GeneratePlan call succeeds.
		if n <= 3 {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		resp := anthropicResponse{
			ID:    "msg_moa_fallback",
			Model: "claude-sonnet-4-20250514",
			Content: []anthropicContentBlock{{
				Type: "text",
				Text: `{
					"intent": "fallback",
					"confidence": 0.8,
					"actions": [{"tool_key": "web_search", "operation": "search", "phase": "gather"}],
					"tools": ["web_search"],
					"risk_level": "low",
					"reasoning": "fallback plan",
					"final_answer_requirements": "search done"
				}`,
			}},
			StopReason: "end_turn",
			Usage:      anthropicUsage{InputTokens: 50, OutputTokens: 25},
		}
		json.NewEncoder(w).Encode(resp)
	}))
	defer ts.Close()

	client, _ := NewAnthropicClient(AnthropicConfig{
		APIKey:  "test-key",
		BaseURL: ts.URL,
		Timeout: 5 * time.Second,
	})

	intel := NewIntelligenceService(IntelligenceConfig{Planner: client})

	plan, _, err := intel.GeneratePlanMoA(
		context.Background(), "test", 0.5, "test", "", "",
		MoAConfig{Enabled: true, ProposalCount: 3},
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if plan == nil {
		t.Fatal("expected non-nil plan from fallback")
	}
}

// ---------------------------------------------------------------------------
// Confidence routing tests
// ---------------------------------------------------------------------------

func TestResolveTierFromConfidence_LowConfidence_T3(t *testing.T) {
	t.Parallel()
	cfg := DefaultConfidenceRoutingConfig()
	result := ResolveTierFromConfidence(0.4, false, cfg)
	if result != "T3" {
		t.Errorf("expected T3 for low confidence, got %s", result)
	}
}

func TestResolveTierFromConfidence_HighConfidenceSimple_T1(t *testing.T) {
	t.Parallel()
	cfg := DefaultConfidenceRoutingConfig()
	result := ResolveTierFromConfidence(0.95, false, cfg)
	if result != "T1" {
		t.Errorf("expected T1 for high confidence simple, got %s", result)
	}
}

func TestResolveTierFromConfidence_HighConfidenceComplex_T2(t *testing.T) {
	t.Parallel()
	cfg := DefaultConfidenceRoutingConfig()
	result := ResolveTierFromConfidence(0.95, true, cfg)
	if result != "T2" {
		t.Errorf("expected T2 for high confidence complex (decomposition=true), got %s", result)
	}
}

func TestResolveTierFromConfidence_BothDisabled_AlwaysT2(t *testing.T) {
	t.Parallel()
	cfg := ConfidenceRoutingConfig{
		LowConfidenceThreshold:        0.60,
		HighConfidenceSimpleThreshold: 0.90,
		EnableDowngrade:               false,
		EnableUpgrade:                 false,
	}
	for _, conf := range []float64{0.1, 0.4, 0.6, 0.8, 0.95, 1.0} {
		result := ResolveTierFromConfidence(conf, false, cfg)
		if result != "T2" {
			t.Errorf("expected T2 with both disabled at confidence=%.2f, got %s", conf, result)
		}
	}
}

func TestResolveTierFromConfidence_Boundaries(t *testing.T) {
	t.Parallel()
	cfg := DefaultConfidenceRoutingConfig()

	// confidence=0.60 exactly → T2 (threshold is exclusive: < 0.60 triggers T3)
	result := ResolveTierFromConfidence(0.60, false, cfg)
	if result != "T2" {
		t.Errorf("expected T2 at boundary 0.60, got %s", result)
	}

	// confidence=0.90 exactly → T2 (threshold is exclusive: > 0.90 triggers T1)
	result = ResolveTierFromConfidence(0.90, false, cfg)
	if result != "T2" {
		t.Errorf("expected T2 at boundary 0.90, got %s", result)
	}
}

func TestGeneratePlan_RoutingEnabled_LowConfidence_UsesT3Model(t *testing.T) {
	t.Parallel()

	var mu sync.Mutex
	var capturedModel string
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body map[string]any
		json.NewDecoder(r.Body).Decode(&body)
		if m, ok := body["model"].(string); ok {
			mu.Lock()
			capturedModel = m
			mu.Unlock()
		}
		resp := anthropicResponse{
			ID:    "msg_routing",
			Model: "claude-sonnet-4-20250514",
			Content: []anthropicContentBlock{{
				Type: "text",
				Text: `{
					"intent": "complex_query",
					"confidence": 0.4,
					"actions": [{"tool_key": "web_search", "operation": "search", "phase": "gather"}],
					"tools": ["web_search"],
					"risk_level": "low",
					"reasoning": "test",
					"final_answer_requirements": "done"
				}`,
			}},
			StopReason: "end_turn",
			Usage:      anthropicUsage{InputTokens: 100, OutputTokens: 50},
		}
		json.NewEncoder(w).Encode(resp)
	}))
	defer ts.Close()

	client, _ := NewAnthropicClient(AnthropicConfig{
		APIKey:  "test-key",
		BaseURL: ts.URL,
		Timeout: 5 * time.Second,
	})

	intel := NewIntelligenceService(IntelligenceConfig{
		Planner:                  client,
		ConfidenceRoutingEnabled: true,
		ConfidenceRouting:        DefaultConfidenceRoutingConfig(),
	})

	_, _, err := intel.GeneratePlan(context.Background(), "complex_query", 0.4, "test", "", "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// T3 model should be used for low confidence.
	t3Tier := ResolveTierModel("T3")
	mu.Lock()
	model := capturedModel
	mu.Unlock()
	if model != t3Tier.PrimaryModel {
		t.Errorf("expected T3 model %q, got %q", t3Tier.PrimaryModel, model)
	}
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > 0 && containsHelper(s, substr))
}

func containsHelper(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
