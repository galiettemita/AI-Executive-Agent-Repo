package brain

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/brevio/brevio/internal/llm"
)

const plannerModel = "claude-sonnet-4-20250514"
const plannerMaxTokens = 2048
const maxORMRetries = 2

// LLMPlannerInput carries all context for a single planner invocation.
type LLMPlannerInput struct {
	WorkspaceID    string
	Intent         string
	StepBackGoal   string
	RetrievedFacts []RetrievedFact
	Tools          []llm.ToolDefinition
	ContextBudget  int
	UseThinking    bool
	ThinkingBudget int
	Temperature    float64 // 0 = use default (0.1)
}

// LLMPlannerOutput is the JSON structure the planner returns.
type LLMPlannerOutput struct {
	Steps     []PlanStep `json:"steps"`
	RiskLevel string     `json:"risk_level"`
	Reasoning string     `json:"reasoning"`
}

const llmPlannerSystemTemplate = `You are Brevio's task planner. Decompose the user request
into a minimal, executable plan using ONLY the tools listed below.

Rules:
1. Select ONLY tools from the provided list. Never invent tool names.
2. Use the MINIMUM number of steps needed.
3. Express step dependencies in depends_on (list of preceding step indices).
4. Phases: "gather"=read/query, "act"=write/send/create, "verify"=confirm success.
5. Each act step must be preceded by a gather step that provides required data.
6. Maximum 10 steps.
7. Respond ONLY with JSON — no preamble, no explanation.

## Available tools
%s`

// callLLMPlanner invokes the LLM to produce a structured plan.
func callLLMPlanner(ctx context.Context, client llm.Client, input LLMPlannerInput) (*Plan, string, error) {
	if client == nil {
		return nil, "", fmt.Errorf("llm_planner: no client")
	}

	temp := input.Temperature
	if temp <= 0 {
		temp = 0.1
	}

	req := llm.GenerateRequest{
		Model:       plannerModel,
		MaxTokens:   plannerMaxTokens,
		Temperature: temp,
		System:      buildPlannerSystem(input.Tools),
		Messages:    []llm.ChatMsg{{Role: "user", Content: buildPlannerUserMsg(input)}},
		JSONSchema:  plannerOutputSchema(),
	}
	if input.UseThinking {
		budget := input.ThinkingBudget
		if budget <= 0 {
			budget = plannerMaxTokens / 2
		}
		req.Thinking = &llm.ThinkingConfig{Enabled: true, BudgetTokens: budget}
		req.JSONSchema = nil // thinking + JSONSchema incompatible
	}

	resp, _, err := client.Generate(ctx, req)
	if err != nil {
		return nil, "", fmt.Errorf("llm_planner: generate: %w", err)
	}

	var out LLMPlannerOutput
	clean := strings.TrimSpace(resp.Content)
	if err := json.Unmarshal([]byte(clean), &out); err != nil {
		return nil, resp.ThinkingContent, fmt.Errorf("llm_planner: parse (%.150s): %w", clean, err)
	}
	if len(out.Steps) == 0 {
		return nil, resp.ThinkingContent, fmt.Errorf("llm_planner: empty plan")
	}
	if len(out.Steps) > 10 {
		out.Steps = out.Steps[:10]
	}
	switch out.RiskLevel {
	case "low", "elevated", "critical":
	default:
		out.RiskLevel = "low"
	}
	return &Plan{
		Steps:           out.Steps,
		RiskLevel:       out.RiskLevel,
		EstimatedTokens: estimateTokens(out.Steps, input.ContextBudget),
	}, resp.ThinkingContent, nil
}

func buildPlannerSystem(tools []llm.ToolDefinition) string {
	var toolList strings.Builder
	for _, t := range tools {
		toolList.WriteString(fmt.Sprintf("- %s: %s\n", t.Name, t.Description))
	}
	if toolList.Len() == 0 {
		toolList.WriteString("(no tools registered)\n")
	}
	return fmt.Sprintf(llmPlannerSystemTemplate, toolList.String())
}

func buildPlannerUserMsg(input LLMPlannerInput) string {
	var b strings.Builder
	b.WriteString(fmt.Sprintf("Workspace: %s\n", input.WorkspaceID))
	b.WriteString(fmt.Sprintf("User request: %q\n", input.Intent))
	if input.StepBackGoal != "" {
		b.WriteString(fmt.Sprintf("Underlying goal: %q\n", input.StepBackGoal))
	}
	if len(input.RetrievedFacts) > 0 {
		b.WriteString("\nContext from memory:\n")
		for i, f := range input.RetrievedFacts {
			if i >= 6 {
				break
			}
			b.WriteString(fmt.Sprintf("  - [%s] %s\n", f.Source, f.Snippet))
		}
	}
	return b.String()
}

func plannerOutputSchema() map[string]any {
	return map[string]any{
		"type":     "object",
		"required": []string{"steps", "risk_level"},
		"properties": map[string]any{
			"steps": map[string]any{
				"type":     "array",
				"maxItems": 10,
				"items": map[string]any{
					"type":     "object",
					"required": []string{"tool_key", "parameters", "phase"},
					"properties": map[string]any{
						"tool_key":   map[string]any{"type": "string"},
						"parameters": map[string]any{"type": "object"},
						"depends_on": map[string]any{"type": "array", "items": map[string]any{"type": "integer"}},
						"phase":      map[string]any{"type": "string", "enum": []string{"gather", "act", "verify"}},
					},
					"additionalProperties": false,
				},
			},
			"risk_level": map[string]any{"type": "string", "enum": []string{"low", "elevated", "critical"}},
			"reasoning":  map[string]any{"type": "string"},
		},
		"additionalProperties": false,
	}
}
