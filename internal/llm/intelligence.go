package llm

import (
	"context"
	"encoding/json"
	"fmt"
	"regexp"
	"sort"
	"strings"
	"sync"
)

// validToolKeyPattern matches tool keys with 1–4 dot-separated lowercase segments.
// Each segment: starts with a letter, followed by letters/digits/underscores.
// Valid:   "web_search", "google_gmail.read_email", "some.tool.v2", "a.b.c.d"
// Invalid: "", "Gmail", "my tool", "a..b", ".start", "a.b.c.d.e"
var validToolKeyPattern = regexp.MustCompile(`^[a-z][a-z0-9_]*(\.[a-z][a-z0-9_]*){0,3}$`)

// IntentClassification is the structured output of intent classification.
type IntentClassification struct {
	Intent                string   `json:"intent"`
	Confidence            float64  `json:"confidence"`
	Skills                []string `json:"skills"`
	RequiresDecomposition bool     `json:"requires_decomposition"`
	Reasoning             string   `json:"reasoning"`
}

// PlanAction is a single action in a generated plan.
type PlanAction struct {
	ToolKey    string         `json:"tool_key"`
	Operation  string         `json:"operation"`
	Parameters map[string]any `json:"parameters,omitempty"`
	Phase      string         `json:"phase"`
	DependsOn  []int          `json:"depends_on,omitempty"` // indices of prior steps this depends on
}

// GeneratedPlan is the structured output of plan generation.
type GeneratedPlan struct {
	Intent                   string       `json:"intent"`
	Confidence               float64      `json:"confidence"`
	Actions                  []PlanAction `json:"actions"`
	Tools                    []string     `json:"tools"`
	RiskLevel                string       `json:"risk_level"`
	Reasoning                string       `json:"reasoning"`
	FinalAnswerRequirements  string       `json:"final_answer_requirements,omitempty"` // success criteria for the verify step
}

// VerifyInput carries the context for the verify/critic activity.
type VerifyInput struct {
	OriginalRequest string                    `json:"original_request"`
	Plan            *GeneratedPlan            `json:"plan"`
	ToolOutputs     []ToolOutputForVerify     `json:"tool_outputs"`
	MemoryContext   string                    `json:"memory_context,omitempty"`
	RAGContext      string                    `json:"rag_context,omitempty"`
	RetryHints      string                    `json:"retry_hints,omitempty"` // populated on re-verify after retry
}

// ToolOutputForVerify is a summary of a single tool execution result.
type ToolOutputForVerify struct {
	ToolKey     string `json:"tool_key"`
	Success     bool   `json:"success"`
	PayloadHash string `json:"payload_hash,omitempty"`
	Phase       string `json:"phase"`
}

// VerifyResult is the structured output of the verify/critic step.
type VerifyResult struct {
	Verdict    string   `json:"verdict"`     // "pass" or "fail"
	Reasons    []string `json:"reasons"`     // human-readable explanation
	RetryHints string   `json:"retry_hints"` // guidance for the next plan attempt if verdict is fail
}

// SynthesizedResponse is the structured output of response synthesis.
type SynthesizedResponse struct {
	ResponseText     string   `json:"response_text"`
	SuggestedActions []string `json:"suggested_actions,omitempty"`
	FollowUp         bool     `json:"follow_up_scheduled"`
}

// ToolRegistry provides the set of known tool_keys that the planner may reference.
// MoAConfig configures Mixture-of-Agents parallel planning.
type MoAConfig struct {
	Enabled         bool    // false = disabled (default, safe)
	ProposalCount   int     // parallel planning calls (default 3, min 2, max 5)
	TemperatureBase float64 // temperature for proposal[0] (default 0.2)
	TemperatureStep float64 // increment per additional proposal (default 0.15)
}

// MoAArbitrationResult is the output of the arbitrator LLM.
type MoAArbitrationResult struct {
	SelectedPlan  GeneratedPlan `json:"selected_plan"`
	Rationale     string        `json:"rationale"`
	RejectedCount int           `json:"rejected_count"`
}

// StepEvalInput carries context for evaluating a single plan step.
type StepEvalInput struct {
	OriginalRequest string         `json:"original_request"`
	StepIndex       int            `json:"step_index"`
	ToolKey         string         `json:"tool_key"`
	Phase           string         `json:"phase"`
	Output          map[string]any `json:"output,omitempty"`
	StepError       string         `json:"step_error,omitempty"`
	PlanContext     string         `json:"plan_context"`
}

// StepEvalResult is the per-step verdict from the PRM evaluator.
type StepEvalResult struct {
	Verdict     string `json:"verdict"`    // "pass" | "fail" | "uncertain"
	Explanation string `json:"explanation"`
	RetryHint   string `json:"retry_hint"`
}

type ToolRegistry interface {
	ToolKeys() []string
	HasTool(toolKey string) bool
}

// IntelligenceService orchestrates LLM calls for the intelligence pipeline.
// It uses the Client interface for provider-agnostic inference and enforces
// structured output validation.
// ConfidenceRoutingConfig controls adaptive tier selection by classification confidence.
type ConfidenceRoutingConfig struct {
	LowConfidenceThreshold        float64 // below this → T3 (default 0.60)
	HighConfidenceSimpleThreshold float64 // above this + !decomposition → T1 (default 0.90)
	EnableDowngrade               bool    // allow T1 for high-confidence simple requests
	EnableUpgrade                 bool    // allow T3 for low-confidence requests
}

// DefaultConfidenceRoutingConfig returns safe, conservative defaults.
func DefaultConfidenceRoutingConfig() ConfidenceRoutingConfig {
	return ConfidenceRoutingConfig{
		LowConfidenceThreshold:        0.60,
		HighConfidenceSimpleThreshold: 0.90,
		EnableDowngrade:               true,
		EnableUpgrade:                 true,
	}
}

// ResolveTierFromConfidence selects the LLM tier based on confidence and complexity.
//
//	confidence < LowConfidenceThreshold              → "T3" (most capable)
//	confidence > HighConfidenceSimpleThreshold AND
//	!requiresDecomposition                           → "T1" (fast/cheap)
//	otherwise                                        → "T2" (balanced default)
func ResolveTierFromConfidence(
	confidence float64,
	requiresDecomposition bool,
	cfg ConfidenceRoutingConfig,
) string {
	if cfg.EnableUpgrade && confidence < cfg.LowConfidenceThreshold {
		return "T3"
	}
	if cfg.EnableDowngrade &&
		confidence > cfg.HighConfidenceSimpleThreshold &&
		!requiresDecomposition {
		return "T1"
	}
	return "T2"
}

type IntelligenceService struct {
	classifier        Client // T0/T1 tier for fast classification
	planner           Client // T2/T3 tier for planning
	synthesizer       Client // T2/T3 tier for response generation
	toolRegistry      ToolRegistry
	confidenceRouting ConfidenceRoutingConfig
	routingEnabled    bool
}

// IntelligenceConfig holds the clients for each pipeline stage.
type IntelligenceConfig struct {
	Classifier               Client
	Planner                  Client
	Synthesizer              Client
	ToolRegistry             ToolRegistry
	ConfidenceRouting        ConfidenceRoutingConfig
	ConfidenceRoutingEnabled bool
}

// NewIntelligenceService creates an IntelligenceService with injected clients.
func NewIntelligenceService(cfg IntelligenceConfig) *IntelligenceService {
	return &IntelligenceService{
		classifier:        cfg.Classifier,
		planner:           cfg.Planner,
		synthesizer:       cfg.Synthesizer,
		toolRegistry:      cfg.ToolRegistry,
		confidenceRouting: cfg.ConfidenceRouting,
		routingEnabled:    cfg.ConfidenceRoutingEnabled,
	}
}

// ClassifyIntent calls the LLM to classify user intent.
func (s *IntelligenceService) ClassifyIntent(ctx context.Context, payload string, workspaceID string) (*IntentClassification, *Usage, error) {
	if s.classifier == nil {
		return nil, nil, fmt.Errorf("intelligence: classifier client not configured")
	}

	tier := ResolveTierModel("T0")
	req := GenerateRequest{
		Model:       tier.PrimaryModel,
		MaxTokens:   tier.MaxOutputTokens,
		Temperature: 0.1,
		Messages: []ChatMsg{
			{
				Role: "system",
				Content: `You are Brevio's intent classifier. Given a user message, output a JSON object with these fields:
- intent: string (the classified intent, e.g. "email_query", "calendar_management", "general_query", "task_creation", "information_retrieval", "document_management", "web_search")
- confidence: number between 0.0 and 1.0
- skills: array of skill IDs that would be needed (e.g. ["email.read", "calendar.read"])
- requires_decomposition: boolean (true if the request needs multiple steps)
- reasoning: brief explanation of classification logic

Respond with ONLY the JSON object.`,
			},
			{
				Role:    "user",
				Content: payload,
			},
		},
		JSONSchema: intentClassificationSchema(),
	}

	resp, usage, err := s.classifier.Generate(ctx, req)
	if err != nil {
		return nil, nil, fmt.Errorf("intelligence: classify intent: %w", err)
	}

	var result IntentClassification
	content := extractJSON(resp.Content)
	if err := json.Unmarshal([]byte(content), &result); err != nil {
		return nil, usage, fmt.Errorf("intelligence: parse classification: %w (raw: %s)", err, truncate(resp.Content, 200))
	}

	if result.Confidence < 0 {
		result.Confidence = 0
	}
	if result.Confidence > 1 {
		result.Confidence = 1
	}
	if result.Intent == "" {
		result.Intent = "general_query"
		result.Confidence = 0.5
	}

	return &result, usage, nil
}

// GeneratePlan calls the LLM to produce a structured execution plan.
func (s *IntelligenceService) GeneratePlan(ctx context.Context, intent string, confidence float64, payload string, memoryContext string, ragContext string) (*GeneratedPlan, *Usage, error) {
	plannerTier := "T2"
	if s.routingEnabled {
		plannerTier = ResolveTierFromConfidence(confidence, false, s.confidenceRouting)
	}
	return s.generatePlanWithTemp(ctx, intent, confidence, payload, memoryContext, ragContext, 0.2, plannerTier)
}

// GeneratePlanWithRouting generates a plan using full confidence + decomposition routing.
// Preferred variant when ClassifyIntent output (requiresDecomposition) is available.
func (s *IntelligenceService) GeneratePlanWithRouting(
	ctx context.Context,
	intent string,
	confidence float64,
	requiresDecomposition bool,
	payload string,
	memoryContext string,
	ragContext string,
) (*GeneratedPlan, *Usage, error) {
	plannerTier := "T2"
	if s.routingEnabled {
		plannerTier = ResolveTierFromConfidence(confidence, requiresDecomposition, s.confidenceRouting)
	}
	return s.generatePlanWithTemp(ctx, intent, confidence, payload, memoryContext, ragContext, 0.2, plannerTier)
}

// generatePlanWithTemp generates a plan with an explicit temperature and tier.
// Used by MoA to produce diverse proposals and by routing to select the right model.
func (s *IntelligenceService) generatePlanWithTemp(
	ctx context.Context,
	intent string,
	confidence float64,
	payload string,
	memoryContext string,
	ragContext string,
	temperature float64,
	tierName string,
) (*GeneratedPlan, *Usage, error) {
	if s.planner == nil {
		return nil, nil, fmt.Errorf("intelligence: planner client not configured")
	}

	tier := ResolveTierModel(tierName)

	var contextSection strings.Builder
	if memoryContext != "" {
		contextSection.WriteString("\n## Relevant Memory\n")
		contextSection.WriteString(memoryContext)
	}
	if ragContext != "" {
		contextSection.WriteString("\n## Retrieved Knowledge\n")
		contextSection.WriteString(ragContext)
	}

	// Build available tools section from registry.
	var availableToolsSection string
	if s.toolRegistry != nil {
		keys := s.toolRegistry.ToolKeys()
		if len(keys) > 0 {
			availableToolsSection = "\n\nAvailable tools (you MUST only use tool_keys from this list):\n" + strings.Join(keys, ", ")
		}
	}

	req := GenerateRequest{
		Model:       tier.PrimaryModel,
		MaxTokens:   tier.MaxOutputTokens,
		// Temperature is overridden to 1.0 by AnthropicClient when Thinking is set.
		// The explicit value here applies only to OpenAI fallback (which silently ignores Thinking).
		Temperature: temperature,
		Thinking: &ThinkingConfig{
			Enabled:      true,
			BudgetTokens: 8000,
		},
		// JSONSchema is intentionally absent — incompatible with thinking.
		// Response parsed via extractJSON() on the text content block.
		Messages: []ChatMsg{
			{
				Role: "system",
				Content: `You are Brevio's execution planner. Given a classified intent and user message, generate a structured execution plan as a JSON object with these fields:

- intent: string (the intent being planned for)
- confidence: number 0.0-1.0 (your confidence in this plan)
- actions: array of action objects, each with:
  - tool_key: string in snake_case format (e.g. "google_gmail.read_email", "google_calendar.create_event", "brave_search.search")
  - operation: string describing what to do
  - parameters: object with relevant parameters
  - phase: one of "gather", "act", "verify"
  - depends_on: array of integer indices of prior steps this depends on (optional)
- tools: array of unique tool_key strings used in the plan
- risk_level: one of "low", "elevated", "critical"
- reasoning: brief explanation of the plan
- final_answer_requirements: string describing what constitutes a successful outcome for verification

Rules:
- tool_key MUST match a tool from the available tools list — do NOT invent tool_keys
- tool_key must be lowercase alphanumeric with dots/underscores only
- Maximum 8 actions per plan
- Actions must be in deterministic order: gather → act → verify
- Include a verify step if any "act" phase steps exist
- risk_level "critical" for financial, sending messages, or data deletion operations
- risk_level "elevated" for calendar writes, document edits, CRM updates
- final_answer_requirements must describe success criteria the verifier can check

Respond with ONLY the JSON object.` + availableToolsSection,
			},
			{
				Role: "user",
				Content: fmt.Sprintf("Intent: %s (confidence: %.2f)\nUser message: %s%s",
					intent, confidence, payload, contextSection.String()),
			},
		},
	}

	resp, usage, err := s.planner.Generate(ctx, req)
	if err != nil {
		return nil, nil, fmt.Errorf("intelligence: generate plan: %w", err)
	}

	var plan GeneratedPlan
	content := extractJSON(resp.Content)
	if err := json.Unmarshal([]byte(content), &plan); err != nil {
		return nil, usage, fmt.Errorf("intelligence: parse plan: %w (raw: %s)", err, truncate(resp.Content, 200))
	}

	if err := validatePlan(&plan); err != nil {
		return nil, usage, fmt.Errorf("intelligence: plan validation: %w", err)
	}

	// Validate tool_keys against registry if available.
	if err := validatePlanToolRegistryKeys(&plan, s.toolRegistry); err != nil {
		return nil, usage, fmt.Errorf("intelligence: plan tool registry validation: %w", err)
	}

	canonicalizePlan(&plan)

	return &plan, usage, nil
}

// ArbitratePlans passes multiple candidate plans to an arbitrator LLM and
// returns the best synthesised plan.
func (s *IntelligenceService) ArbitratePlans(
	ctx context.Context,
	originalIntent string,
	proposals []GeneratedPlan,
) (*MoAArbitrationResult, *Usage, error) {
	if len(proposals) == 0 {
		return nil, nil, fmt.Errorf("llm: ArbitratePlans: no proposals provided")
	}
	if len(proposals) == 1 {
		return &MoAArbitrationResult{
			SelectedPlan: proposals[0],
			Rationale:    "only one proposal",
		}, &Usage{}, nil
	}
	if s.synthesizer == nil {
		return nil, nil, fmt.Errorf("llm: ArbitratePlans requires synthesizer client")
	}

	tier := ResolveTierModel("T2")
	systemPrompt := `You are Brevio's plan arbitrator. You receive multiple candidate plans
for the same user intent, each produced independently. Your task:
1. Evaluate each plan for correctness, completeness, minimal steps, and risk.
2. Select the BEST plan (or synthesise the best elements into one).
3. Return ONLY a JSON object with fields:
   "selected_plan": the complete chosen plan (same schema as input plans),
   "rationale": one sentence explaining your choice,
   "rejected_count": how many proposals you rejected.
Return ONLY valid JSON. No markdown.`

	proposalsJSON, err := json.Marshal(map[string]any{
		"original_intent": originalIntent,
		"proposals":       proposals,
	})
	if err != nil {
		return nil, nil, fmt.Errorf("llm: ArbitratePlans marshal: %w", err)
	}

	req := GenerateRequest{
		Model:       tier.PrimaryModel,
		MaxTokens:   2048,
		Temperature: 0.0,
		Messages: []ChatMsg{
			{Role: "system", Content: systemPrompt},
			{Role: "user", Content: string(proposalsJSON)},
		},
	}

	resp, usage, err := s.synthesizer.Generate(ctx, req)
	if err != nil {
		return nil, usage, fmt.Errorf("llm: ArbitratePlans LLM: %w", err)
	}

	clean := extractJSON(resp.Content)

	var result MoAArbitrationResult
	if err := json.Unmarshal([]byte(clean), &result); err != nil {
		return nil, usage, fmt.Errorf("llm: ArbitratePlans parse: %w", err)
	}
	return &result, usage, nil
}

// GeneratePlanMoA runs ProposalCount parallel planning calls and arbitrates the best.
// Falls back to single-call planning if disabled or not enough proposals succeed.
func (s *IntelligenceService) GeneratePlanMoA(
	ctx context.Context,
	intent string,
	confidence float64,
	payload string,
	memoryContext string,
	ragContext string,
	cfg MoAConfig,
) (*GeneratedPlan, *Usage, error) {
	if !cfg.Enabled {
		return s.GeneratePlan(ctx, intent, confidence, payload, memoryContext, ragContext)
	}
	if cfg.ProposalCount < 2 {
		cfg.ProposalCount = 2
	}
	if cfg.ProposalCount > 5 {
		cfg.ProposalCount = 5
	}
	if cfg.TemperatureBase <= 0 {
		cfg.TemperatureBase = 0.2
	}
	if cfg.TemperatureStep <= 0 {
		cfg.TemperatureStep = 0.15
	}

	type planResult struct {
		plan  *GeneratedPlan
		usage *Usage
		err   error
	}
	results := make(chan planResult, cfg.ProposalCount)

	var wg sync.WaitGroup
	for i := 0; i < cfg.ProposalCount; i++ {
		wg.Add(1)
		temp := cfg.TemperatureBase + float64(i)*cfg.TemperatureStep
		go func(t float64) {
			defer wg.Done()
			p, u, e := s.generatePlanWithTemp(ctx, intent, confidence, payload, memoryContext, ragContext, t, "T2")
			results <- planResult{p, u, e}
		}(temp)
	}
	wg.Wait()
	close(results)

	var proposals []GeneratedPlan
	var totalUsage Usage
	for r := range results {
		if r.err == nil && r.plan != nil {
			proposals = append(proposals, *r.plan)
		}
		if r.usage != nil {
			totalUsage.InputTokens += r.usage.InputTokens
			totalUsage.OutputTokens += r.usage.OutputTokens
		}
	}

	if len(proposals) < 2 {
		if len(proposals) == 1 {
			return &proposals[0], &totalUsage, nil
		}
		return s.GeneratePlan(ctx, intent, confidence, payload, memoryContext, ragContext)
	}

	arbResult, arbUsage, err := s.ArbitratePlans(ctx, intent, proposals)
	if arbUsage != nil {
		totalUsage.InputTokens += arbUsage.InputTokens
		totalUsage.OutputTokens += arbUsage.OutputTokens
	}
	if err != nil {
		best := proposals[0]
		for _, p := range proposals[1:] {
			if p.Confidence > best.Confidence {
				best = p
			}
		}
		return &best, &totalUsage, nil
	}
	return &arbResult.SelectedPlan, &totalUsage, nil
}

// SynthesizeResponse calls the LLM to generate a user-facing response.
func (s *IntelligenceService) SynthesizeResponse(ctx context.Context, payload string, toolResults string) (*SynthesizedResponse, *Usage, error) {
	if s.synthesizer == nil {
		return nil, nil, fmt.Errorf("intelligence: synthesizer client not configured")
	}

	tier := ResolveTierModel("T2")
	req := GenerateRequest{
		Model:       tier.PrimaryModel,
		MaxTokens:   tier.MaxOutputTokens,
		Temperature: 0.3,
		Messages: []ChatMsg{
			{Role: "system", Content: synthesisSystemPrompt()},
			{
				Role: "user",
				Content: fmt.Sprintf("Original request: %s\n\nTool execution results:\n%s", payload, toolResults),
			},
		},
		JSONSchema: synthesizedResponseSchema(),
	}

	resp, usage, err := s.synthesizer.Generate(ctx, req)
	if err != nil {
		return nil, nil, fmt.Errorf("intelligence: synthesize response: %w", err)
	}

	var result SynthesizedResponse
	content := extractJSON(resp.Content)
	if err := json.Unmarshal([]byte(content), &result); err != nil {
		return nil, usage, fmt.Errorf("intelligence: parse synthesis: %w (raw: %s)", err, truncate(resp.Content, 200))
	}

	return &result, usage, nil
}

// StreamSynthesizeResponse streams the synthesis response to the caller.
// Does NOT use JSONSchema — tool_use forcing is incompatible with streaming.
func (s *IntelligenceService) StreamSynthesizeResponse(
	ctx context.Context,
	payload string,
	toolResults string,
	out chan<- StreamChunk,
) {
	if s.synthesizer == nil {
		out <- StreamChunk{Error: fmt.Errorf("intelligence: synthesizer not configured")}
		close(out)
		return
	}
	tier := ResolveTierModel("T2")
	req := GenerateRequest{
		Model:       tier.PrimaryModel,
		MaxTokens:   tier.MaxOutputTokens,
		Temperature: 0.3,
		Messages: []ChatMsg{
			{Role: "system", Content: synthesisSystemPrompt()},
			{
				Role: "user",
				Content: fmt.Sprintf(
					"Tool execution results:\n%s\n\nOriginal request: %s",
					toolResults, payload,
				),
			},
		},
	}
	s.synthesizer.Stream(ctx, req, out)
}

// VerifyExecution calls the LLM to evaluate whether tool outputs satisfy the plan.
func (s *IntelligenceService) VerifyExecution(ctx context.Context, input VerifyInput) (*VerifyResult, *Usage, error) {
	if s.synthesizer == nil {
		return nil, nil, fmt.Errorf("intelligence: synthesizer client not configured (used for verify)")
	}

	tier := ResolveTierModel("T2")

	planJSON, _ := json.Marshal(input.Plan)
	outputsJSON, _ := json.Marshal(input.ToolOutputs)

	var retrySection string
	if input.RetryHints != "" {
		retrySection = fmt.Sprintf("\n\nPrevious attempt was rejected. Verifier hints:\n%s", input.RetryHints)
	}

	req := GenerateRequest{
		Model:       tier.PrimaryModel,
		MaxTokens:   tier.MaxOutputTokens,
		Temperature: 0.1,
		Thinking: &ThinkingConfig{
			Enabled:      true,
			BudgetTokens: 4000,
		},
		Messages: []ChatMsg{
			{
				Role: "system",
				Content: `You are Brevio's execution verifier (critic). Given a user request, the execution plan, and tool outputs, determine whether the execution was successful.

Output a JSON object with:
- verdict: "pass" or "fail"
- reasons: array of strings explaining the verdict
- retry_hints: string with guidance for re-planning if verdict is "fail" (empty string if pass)

Rules:
- "pass" if all critical tools executed successfully and the results address the user's request
- "fail" if any critical tool failed, results are incomplete, or the plan did not address the request
- retry_hints should be specific and actionable for the planner

Respond with ONLY the JSON object.`,
			},
			{
				Role: "user",
				Content: fmt.Sprintf("User request: %s\n\nPlan:\n%s\n\nTool outputs:\n%s\n\nFinal answer requirements: %s%s",
					input.OriginalRequest, string(planJSON), string(outputsJSON),
					input.Plan.FinalAnswerRequirements, retrySection),
			},
		},
	}

	resp, usage, err := s.synthesizer.Generate(ctx, req)
	if err != nil {
		return nil, nil, fmt.Errorf("intelligence: verify execution: %w", err)
	}

	var result VerifyResult
	content := extractJSON(resp.Content)
	if err := json.Unmarshal([]byte(content), &result); err != nil {
		return nil, usage, fmt.Errorf("intelligence: parse verify result: %w (raw: %s)", err, truncate(resp.Content, 200))
	}

	if result.Verdict != "pass" && result.Verdict != "fail" {
		return nil, usage, fmt.Errorf("intelligence: invalid verify verdict: %q (must be pass or fail)", result.Verdict)
	}

	return &result, usage, nil
}

// VerifySteps evaluates all step results in a single batched LLM call.
// Uses T1 (fast/cheap) model. Returns one StepEvalResult per input, same order.
func (s *IntelligenceService) VerifySteps(ctx context.Context, inputs []StepEvalInput) ([]StepEvalResult, *Usage, error) {
	if s.synthesizer == nil {
		return nil, nil, fmt.Errorf("llm: VerifySteps requires synthesizer client")
	}
	if len(inputs) == 0 {
		return []StepEvalResult{}, &Usage{}, nil
	}

	tier := ResolveTierModel("T1")
	systemPrompt := `You are Brevio's step evaluator. For each step provided, output a JSON array
where each element corresponds to the input step (same order) and contains:
- "verdict": "pass", "fail", or "uncertain"
- "explanation": one sentence describing the outcome
- "retry_hint": if verdict is "fail", a specific actionable fix; otherwise ""
Return ONLY the JSON array. No markdown, no preamble.`

	inputJSON, err := json.Marshal(inputs)
	if err != nil {
		return nil, nil, fmt.Errorf("llm: VerifySteps marshal: %w", err)
	}

	req := GenerateRequest{
		Model:       tier.PrimaryModel,
		MaxTokens:   1024,
		Temperature: 0.0,
		Messages: []ChatMsg{
			{Role: "system", Content: systemPrompt},
			{Role: "user", Content: string(inputJSON)},
		},
	}

	resp, usage, err := s.synthesizer.Generate(ctx, req)
	if err != nil {
		return nil, usage, fmt.Errorf("llm: VerifySteps LLM call: %w", err)
	}

	clean := extractJSON(resp.Content)

	var results []StepEvalResult
	if err := json.Unmarshal([]byte(clean), &results); err != nil {
		return nil, usage, fmt.Errorf("llm: VerifySteps parse: %w", err)
	}
	if len(results) != len(inputs) {
		return nil, usage, fmt.Errorf("llm: VerifySteps expected %d results, got %d", len(inputs), len(results))
	}
	return results, usage, nil
}

// synthesisSystemPrompt returns the system prompt used for response synthesis.
// Shared by SynthesizeResponse and StreamSynthesizeResponse to ensure consistency.
func synthesisSystemPrompt() string {
	return `You are Brevio, an executive AI assistant. Generate a natural, concise response incorporating skill execution results. Output a JSON object with:
- response_text: string (the user-facing response, max 4096 chars)
- suggested_actions: array of string suggestions for follow-up
- follow_up_scheduled: boolean

Respond with ONLY the JSON object.`
}

// SummarizeText implements brain.Summarizer.
// Uses T0 (Haiku) — fast and cheap. Called only when context budget is exceeded.
func (s *IntelligenceService) SummarizeText(
	ctx context.Context,
	conversationText string,
	maxOutputTokens int,
) (string, error) {
	if s.classifier == nil {
		return "", fmt.Errorf("intelligence: classifier not configured for summarization")
	}
	if maxOutputTokens <= 0 {
		maxOutputTokens = 256
	}

	req := GenerateRequest{
		Model:       ResolveTierModel("T0").PrimaryModel,
		MaxTokens:   maxOutputTokens,
		Temperature: 0.0,
		// No Thinking (Haiku doesn't support it), no JSONSchema (plain text response).
		Messages: []ChatMsg{
			{
				Role: "system",
				Content: "Summarize the following conversation in 2–3 sentences. " +
					"Preserve key decisions, facts, and action items. Be concise.",
			},
			{Role: "user", Content: conversationText},
		},
	}

	resp, _, err := s.classifier.Generate(ctx, req)
	if err != nil {
		return "", fmt.Errorf("summarize conversation: %w", err)
	}
	return strings.TrimSpace(resp.Content), nil
}

// ValidateStrictPlanJSON parses and validates a plan JSON string against the
// canonical schema. Returns a non-retryable error if the plan is structurally invalid.
func ValidateStrictPlanJSON(raw string) (*GeneratedPlan, error) {
	content := extractJSON(raw)
	var plan GeneratedPlan
	if err := json.Unmarshal([]byte(content), &plan); err != nil {
		return nil, fmt.Errorf("strict plan parse failed (non-retryable): %w", err)
	}
	if err := validatePlan(&plan); err != nil {
		return nil, fmt.Errorf("strict plan validation failed (non-retryable): %w", err)
	}
	canonicalizePlan(&plan)
	return &plan, nil
}

// ValidateToolKey returns nil if the key format is valid, or a descriptive error.
// Exported so other packages (executor, hands) can validate without the full service.
func ValidateToolKey(toolKey string) error {
	if strings.TrimSpace(toolKey) == "" {
		return fmt.Errorf("tool_key is required (got empty string)")
	}
	if !validToolKeyPattern.MatchString(toolKey) {
		return fmt.Errorf(
			"invalid tool_key format %q: must be 1–4 dot-separated lowercase segments "+
				"(e.g. \"web_search\", \"google_gmail.read_email\", \"some.tool.v2\")",
			toolKey,
		)
	}
	return nil
}

// validatePlanToolRegistryKeys checks every action's tool_key exists in the registry.
// Returns a descriptive error listing available keys when any are missing.
func validatePlanToolRegistryKeys(plan *GeneratedPlan, registry ToolRegistry) error {
	if registry == nil || plan == nil {
		return nil
	}
	available := registry.ToolKeys()
	if len(available) == 0 {
		return nil
	}
	availableSet := make(map[string]struct{}, len(available))
	for _, k := range available {
		availableSet[k] = struct{}{}
	}
	var missing []string
	for _, action := range plan.Actions {
		if _, ok := availableSet[action.ToolKey]; !ok {
			missing = append(missing, action.ToolKey)
		}
	}
	if len(missing) == 0 {
		return nil
	}
	return fmt.Errorf(
		"plan references unknown tool keys %v; available tools: %v",
		missing, available,
	)
}

// validatePlan checks structural validity of a generated plan.
func validatePlan(plan *GeneratedPlan) error {
	if plan.Intent == "" {
		return fmt.Errorf("plan intent is required")
	}
	if len(plan.Actions) == 0 {
		return fmt.Errorf("plan must have at least one action")
	}
	if len(plan.Actions) > 8 {
		return fmt.Errorf("plan exceeds max 8 actions: got %d", len(plan.Actions))
	}

	validRisk := map[string]bool{"low": true, "elevated": true, "critical": true}
	if !validRisk[plan.RiskLevel] {
		plan.RiskLevel = "low"
	}

	for i, action := range plan.Actions {
		if err := ValidateToolKey(action.ToolKey); err != nil {
			return fmt.Errorf("action[%d]: %w", i, err)
		}
		validPhase := map[string]bool{"gather": true, "act": true, "verify": true}
		if !validPhase[action.Phase] {
			return fmt.Errorf("action[%d] invalid phase: %q", i, action.Phase)
		}
	}

	if plan.Confidence < 0 || plan.Confidence > 1 {
		return fmt.Errorf("plan confidence must be between 0 and 1")
	}

	return nil
}

// canonicalizePlan enforces deterministic ordering and populates the tools list.
func canonicalizePlan(plan *GeneratedPlan) {
	phaseOrder := map[string]int{"gather": 0, "act": 1, "verify": 2}
	sort.SliceStable(plan.Actions, func(i, j int) bool {
		pi := phaseOrder[plan.Actions[i].Phase]
		pj := phaseOrder[plan.Actions[j].Phase]
		if pi != pj {
			return pi < pj
		}
		return plan.Actions[i].ToolKey < plan.Actions[j].ToolKey
	})

	toolSet := make(map[string]bool, len(plan.Actions))
	for _, a := range plan.Actions {
		toolSet[a.ToolKey] = true
	}
	plan.Tools = make([]string, 0, len(toolSet))
	for t := range toolSet {
		plan.Tools = append(plan.Tools, t)
	}
	sort.Strings(plan.Tools)
}

// extractJSON attempts to extract a JSON object from LLM output that may
// contain markdown code fences or surrounding text.
func extractJSON(raw string) string {
	s := strings.TrimSpace(raw)

	// Try markdown code block extraction.
	if idx := strings.Index(s, "```json"); idx != -1 {
		start := idx + len("```json")
		end := strings.Index(s[start:], "```")
		if end != -1 {
			return strings.TrimSpace(s[start : start+end])
		}
	}
	if idx := strings.Index(s, "```"); idx != -1 {
		start := idx + len("```")
		end := strings.Index(s[start:], "```")
		if end != -1 {
			candidate := strings.TrimSpace(s[start : start+end])
			if len(candidate) > 0 && (candidate[0] == '{' || candidate[0] == '[') {
				return candidate
			}
		}
	}

	// Find first { and last } for bare JSON.
	firstBrace := strings.Index(s, "{")
	lastBrace := strings.LastIndex(s, "}")
	if firstBrace >= 0 && lastBrace > firstBrace {
		return s[firstBrace : lastBrace+1]
	}

	return s
}

func truncate(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "..."
}

// JSON Schemas for structured output enforcement.

func intentClassificationSchema() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"intent":                 map[string]any{"type": "string"},
			"confidence":             map[string]any{"type": "number"},
			"skills":                 map[string]any{"type": "array", "items": map[string]any{"type": "string"}},
			"requires_decomposition": map[string]any{"type": "boolean"},
			"reasoning":              map[string]any{"type": "string"},
		},
		"required":             []string{"intent", "confidence", "skills", "requires_decomposition", "reasoning"},
		"additionalProperties": false,
	}
}

func generatedPlanSchema() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"intent":     map[string]any{"type": "string"},
			"confidence": map[string]any{"type": "number"},
			"actions": map[string]any{
				"type":     "array",
				"maxItems": 8,
				"items": map[string]any{
					"type": "object",
					"properties": map[string]any{
						"tool_key":   map[string]any{"type": "string"},
						"operation":  map[string]any{"type": "string"},
						"parameters": map[string]any{"type": "object"},
						"phase":      map[string]any{"type": "string", "enum": []string{"gather", "act", "verify"}},
						"depends_on": map[string]any{"type": "array", "items": map[string]any{"type": "integer"}},
					},
					"required": []string{"tool_key", "operation", "phase"},
				},
			},
			"tools":                    map[string]any{"type": "array", "items": map[string]any{"type": "string"}},
			"risk_level":               map[string]any{"type": "string", "enum": []string{"low", "elevated", "critical"}},
			"reasoning":                map[string]any{"type": "string"},
			"final_answer_requirements": map[string]any{"type": "string"},
		},
		"required":             []string{"intent", "confidence", "actions", "tools", "risk_level", "reasoning"},
		"additionalProperties": false,
	}
}

func verifyResultSchema() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"verdict":     map[string]any{"type": "string", "enum": []string{"pass", "fail"}},
			"reasons":     map[string]any{"type": "array", "items": map[string]any{"type": "string"}},
			"retry_hints": map[string]any{"type": "string"},
		},
		"required":             []string{"verdict", "reasons", "retry_hints"},
		"additionalProperties": false,
	}
}

func synthesizedResponseSchema() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"response_text":      map[string]any{"type": "string"},
			"suggested_actions":  map[string]any{"type": "array", "items": map[string]any{"type": "string"}},
			"follow_up_scheduled": map[string]any{"type": "boolean"},
		},
		"required":             []string{"response_text", "suggested_actions", "follow_up_scheduled"},
		"additionalProperties": false,
	}
}
