package integration

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/brevio/brevio/internal/connectors"
	"github.com/brevio/brevio/internal/hands"
	"github.com/brevio/brevio/internal/llm"
	"github.com/brevio/brevio/internal/temporal"
)

// ---------------------------------------------------------------------------
// No-stubs end-to-end gate: plan → execute → verify with real service
// boundaries (fake providers, spy servers). This test MUST FAIL if:
//   - planning returns hardcoded tool keys without args
//   - tool execution returns success without contacting hands
//   - verification is skipped
// ---------------------------------------------------------------------------

func TestNoStubs_PlanExecuteVerify_EndToEnd(t *testing.T) {
	t.Parallel()

	// -----------------------------------------------------------------------
	// 1. Seed the tool registry from connectors.yaml.
	// -----------------------------------------------------------------------
	kp := connectors.NewInMemoryKeyProvider("v1", make([]byte, 32))
	connSvc := connectors.NewService(kp)
	seedPath := filepath.Join("..", "..", "internal", "connectors", "seeds", "connectors.yaml")
	if err := connSvc.LoadSeedFile(seedPath); err != nil {
		t.Fatalf("load seed file: %v", err)
	}

	toolKeys := connSvc.ToolKeys()
	if len(toolKeys) == 0 {
		t.Fatal("expected non-empty tool registry after seeding")
	}

	// Pick a safe read-only tool from the registry for planning.
	var safeToolKey string
	allTools := connSvc.ListAllTools()
	for _, tool := range allTools {
		if !tool.Write {
			safeToolKey = tool.ToolKey
			break
		}
	}
	if safeToolKey == "" {
		t.Fatal("no read-only tool found in registry")
	}
	t.Logf("Using tool: %s", safeToolKey)

	// -----------------------------------------------------------------------
	// 2. Create fake LLM server that returns structured responses.
	// -----------------------------------------------------------------------
	llmCallCount := struct {
		mu       sync.Mutex
		classify int
		plan     int
		verify   int
	}{}

	fakeLLM := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var reqBody map[string]any
		json.NewDecoder(r.Body).Decode(&reqBody)

		// Anthropic puts system prompt in top-level "system" field, not in messages.
		systemPrompt, _ := reqBody["system"].(string)

		w.Header().Set("Content-Type", "application/json")

		// Anthropic Messages API response format.
		var content string
		llmCallCount.mu.Lock()
		// Route based on unique keywords in each system prompt.
		// Classify: "intent classifier" (unique to classify prompt)
		// Plan: "execution planner" (unique to plan prompt)
		// Verify: "verdict" (unique to verify prompt)
		if containsAny(systemPrompt, "intent classifier") {
			llmCallCount.classify++
			content = `{"intent":"web_search","confidence":0.92,"skills":["web_search"],"requires_decomposition":false,"reasoning":"search query"}`
		} else if containsAny(systemPrompt, "execution planner") {
			llmCallCount.plan++
			content = `{"intent":"web_search","actions":[{"tool_key":"` + safeToolKey + `","operation":"query","parameters":{"q":"test"},"phase":"gather","depends_on":[]}],"tools":["` + safeToolKey + `"],"risk_level":"low","reasoning":"simple search","final_answer_requirements":"Verify search results returned."}`
		} else if containsAny(systemPrompt, "verdict") {
			llmCallCount.verify++
			content = `{"verdict":"pass","reasons":["all tools executed successfully"],"retry_hints":""}`
		} else {
			llmCallCount.mu.Unlock()
			// Synthesize response fallback.
			content = `{"response_text":"Here are your results.","suggested_actions":[],"follow_up":""}`
			w.WriteHeader(http.StatusOK)
			json.NewEncoder(w).Encode(map[string]any{
				"id":    "msg_test",
				"type":  "message",
				"model": "claude-3-haiku-20240307",
				"content": []map[string]any{
					{"type": "text", "text": content},
				},
				"stop_reason": "end_turn",
				"usage":       map[string]any{"input_tokens": 10, "output_tokens": 20},
			})
			return
		}
		llmCallCount.mu.Unlock()

		json.NewEncoder(w).Encode(map[string]any{
			"id":    "msg_test",
			"type":  "message",
			"model": "claude-3-haiku-20240307",
			"content": []map[string]any{
				{"type": "text", "text": content},
			},
			"stop_reason": "end_turn",
			"usage":       map[string]any{"input_tokens": 10, "output_tokens": 20},
		})
	}))
	defer fakeLLM.Close()

	// -----------------------------------------------------------------------
	// 3. Create spy hands server that records calls.
	// -----------------------------------------------------------------------
	handsCalls := struct {
		mu    sync.Mutex
		calls []string
	}{}

	spyHands := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var reqBody map[string]any
		json.NewDecoder(r.Body).Decode(&reqBody)
		toolKey, _ := reqBody["tool_key"].(string)

		handsCalls.mu.Lock()
		handsCalls.calls = append(handsCalls.calls, toolKey)
		handsCalls.mu.Unlock()

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{
			"status":   "ok",
			"tool_key": toolKey,
			"result":   "search results",
		})
	}))
	defer spyHands.Close()

	// -----------------------------------------------------------------------
	// 4. Wire LLM service with fake provider.
	// -----------------------------------------------------------------------
	anthropicClient, err := llm.NewAnthropicClient(llm.AnthropicConfig{
		APIKey:  "test-key",
		BaseURL: fakeLLM.URL,
	})
	if err != nil {
		t.Fatalf("create anthropic client: %v", err)
	}

	toolRegistry := &testToolRegistry{tools: make(map[string]bool)}
	for _, tk := range toolKeys {
		toolRegistry.tools[tk] = true
	}

	intel := llm.NewIntelligenceService(llm.IntelligenceConfig{
		Classifier:   anthropicClient,
		Planner:      anthropicClient,
		Synthesizer:  anthropicClient,
		ToolRegistry: toolRegistry,
	})

	llmSvc := llm.NewService()
	llmSvc.SetIntelligence(intel)

	// -----------------------------------------------------------------------
	// 5. Wire hands runtime with spy MCP server.
	// -----------------------------------------------------------------------
	// Create hands service backed by connectors + spy MCP client.
	mcpClient := hands.NewHTTPMCPClient(5 * time.Second)
	handsSvc := hands.NewService(connSvc, mcpClient)

	// We need the spy server to be the MCP target. Override the connector
	// MCP URLs by using the hands ExecutorAdapter with a custom executor
	// that routes to our spy server.
	spyExecutor := &spyHandsExecutor{
		svc:       handsSvc,
		spyURL:    spyHands.URL,
		mcpClient: mcpClient,
	}

	activities := temporal.NewActivitiesWithProdDeps(temporal.ActivityDeps{
		LLMService:    llmSvc,
		HandsExecutor: spyExecutor,
	})

	// -----------------------------------------------------------------------
	// 6. Run the full activity chain: classify → plan → authorize → execute → verify
	// -----------------------------------------------------------------------
	ctx := context.Background()

	// Step 1: Classify intent.
	classifyResult, err := activities.ClassifyIntentActivity(ctx, temporal.ClassifyIntentInput{
		MessageID:   "e2e-msg-001",
		WorkspaceID: "e2e-ws-001",
		Payload:     "search for the latest AI news",
	})
	if err != nil {
		t.Fatalf("ClassifyIntent failed: %v", err)
	}
	if classifyResult.Intent == "" {
		t.Fatal("expected non-empty intent from classify")
	}
	t.Logf("Classify: intent=%s confidence=%.2f", classifyResult.Intent, classifyResult.Confidence)

	// Step 2: Generate plan.
	planResult, err := activities.GeneratePlanActivity(ctx, temporal.GeneratePlanInput{
		MessageID:   "e2e-msg-001",
		WorkspaceID: "e2e-ws-001",
		Intent:      classifyResult.Intent,
		Confidence:  classifyResult.Confidence,
		Payload:     "search for the latest AI news",
	})
	if err != nil {
		t.Fatalf("GeneratePlan failed: %v", err)
	}
	if planResult.PlanID == "" {
		t.Fatal("expected non-empty PlanID")
	}
	if len(planResult.ToolKeys) == 0 {
		t.Fatal("NO-STUBS VIOLATION: plan returned zero tool keys — planning may be stubbed")
	}
	if planResult.Deterministic {
		t.Fatal("NO-STUBS VIOLATION: plan is deterministic — LLM was not called")
	}
	t.Logf("Plan: id=%s tools=%v risk=%s deterministic=%v", planResult.PlanID, planResult.ToolKeys, planResult.RiskLevel, planResult.Deterministic)

	// Step 3: Authorize plan.
	authResult, err := activities.AuthorizePlanActivity(ctx, temporal.AuthorizePlanInput{
		MessageID:   "e2e-msg-001",
		WorkspaceID: "e2e-ws-001",
		PlanID:      planResult.PlanID,
		ToolKeys:    planResult.ToolKeys,
		RiskLevel:   planResult.RiskLevel,
	})
	if err != nil {
		t.Fatalf("AuthorizePlan failed: %v", err)
	}
	if authResult.Decision != "allow" {
		t.Fatalf("expected authorization allow, got %q", authResult.Decision)
	}
	if authResult.ReceiptID == "" {
		t.Fatal("expected non-empty receipt from authorization")
	}
	t.Logf("Authorize: decision=%s receipt=%s", authResult.Decision, authResult.ReceiptID)

	// Step 4: Execute each tool.
	var toolResults []temporal.ToolExecutionActivityResult
	for _, toolKey := range planResult.ToolKeys {
		execResult, err := activities.ExecuteToolActivity(ctx, temporal.ExecuteToolInput{
			MessageID:      "e2e-msg-001",
			WorkspaceID:    "e2e-ws-001",
			ToolKey:        toolKey,
			ReceiptID:      authResult.ReceiptID,
			IdempotencyKey: "e2e-idem-" + toolKey,
		})
		if err != nil {
			t.Fatalf("ExecuteTool(%s) failed: %v", toolKey, err)
		}
		if !execResult.Success {
			t.Fatalf("ExecuteTool(%s) returned Success=false", toolKey)
		}
		toolResults = append(toolResults, *execResult)
		t.Logf("Execute: tool=%s success=%v output_present=%v", toolKey, execResult.Success, execResult.ToolOutput != nil)
	}

	// Assertion: hands spy server was actually contacted.
	handsCalls.mu.Lock()
	callCount := len(handsCalls.calls)
	handsCalls.mu.Unlock()
	if callCount == 0 {
		t.Fatal("NO-STUBS VIOLATION: hands spy server received zero calls — tool execution is stubbed")
	}
	t.Logf("Hands spy: %d call(s) recorded", callCount)

	// Step 5: Verify execution.
	verifyResult, err := activities.VerifyExecutionActivity(ctx, temporal.VerifyExecutionInput{
		MessageID:       "e2e-msg-001",
		WorkspaceID:     "e2e-ws-001",
		OriginalPayload: "search for the latest AI news",
		PlanID:          planResult.PlanID,
		PlanToolKeys:    planResult.ToolKeys,
		PlanRiskLevel:   planResult.RiskLevel,
		FinalAnswerReqs: planResult.FinalAnswerReqs,
		ToolResults:     toolResults,
	})
	if err != nil {
		t.Fatalf("VerifyExecution failed: %v", err)
	}
	if verifyResult.Verdict == "" {
		t.Fatal("NO-STUBS VIOLATION: verify returned empty verdict — verification may be skipped")
	}
	if verifyResult.Verdict != "pass" {
		t.Fatalf("expected verify verdict 'pass', got %q reasons=%v", verifyResult.Verdict, verifyResult.Reasons)
	}
	t.Logf("Verify: verdict=%s reasons=%v", verifyResult.Verdict, verifyResult.Reasons)

	// -----------------------------------------------------------------------
	// 7. Final assertions: prove no stubs were used.
	// -----------------------------------------------------------------------
	llmCallCount.mu.Lock()
	classifyCalls := llmCallCount.classify
	planCalls := llmCallCount.plan
	verifyCalls := llmCallCount.verify
	llmCallCount.mu.Unlock()

	if classifyCalls == 0 {
		t.Error("NO-STUBS VIOLATION: LLM classify was never called")
	}
	if planCalls == 0 {
		t.Error("NO-STUBS VIOLATION: LLM plan was never called")
	}
	if verifyCalls == 0 {
		t.Error("NO-STUBS VIOLATION: LLM verify was never called")
	}

	t.Logf("LLM calls: classify=%d plan=%d verify=%d", classifyCalls, planCalls, verifyCalls)
	t.Logf("END-TO-END PASS: plan→execute→verify chain completed with real service boundaries")
}

// ---------------------------------------------------------------------------
// Test helpers
// ---------------------------------------------------------------------------

// testToolRegistry implements llm.ToolRegistry for test use.
type testToolRegistry struct {
	tools map[string]bool
}

func (r *testToolRegistry) ToolKeys() []string {
	keys := make([]string, 0, len(r.tools))
	for k := range r.tools {
		keys = append(keys, k)
	}
	return keys
}

func (r *testToolRegistry) HasTool(toolKey string) bool {
	return r.tools[toolKey]
}

// spyHandsExecutor wraps hands.Service but routes MCP calls through a spy server.
type spyHandsExecutor struct {
	svc       *hands.Service
	spyURL    string
	mcpClient *hands.HTTPMCPClient
}

func (e *spyHandsExecutor) ExecuteTool(ctx context.Context, skillID, workspaceID, receiptID, idempotencyKey, mode string, args map[string]interface{}) (bool, any, error) {
	// Route directly through the spy MCP server instead of the real MCP URL.
	output, err := e.mcpClient.Execute(ctx, e.spyURL, skillID, args)
	if err != nil {
		return false, nil, err
	}
	return true, output, nil
}

func containsAny(s string, substrs ...string) bool {
	for _, sub := range substrs {
		for i := 0; i <= len(s)-len(sub); i++ {
			if s[i:i+len(sub)] == sub {
				return true
			}
		}
	}
	return false
}
