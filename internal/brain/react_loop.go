package brain

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/brevio/brevio/internal/llm"
)

const reactModel = "claude-sonnet-4-20250514"
const reactMaxTokens = 4096
const reactMaxTurns = 12

// ReactTurn records one think→act→observe cycle.
type ReactTurn struct {
	TurnIndex   int            `json:"turn_index"`
	Thought     string         `json:"thought,omitempty"`
	ToolName    string         `json:"tool_name,omitempty"`
	ToolCallID  string         `json:"tool_call_id,omitempty"`
	ToolInput   map[string]any `json:"tool_input,omitempty"`
	Observation string         `json:"observation,omitempty"`
	IsFinal     bool           `json:"is_final"`
	FinalAnswer string         `json:"final_answer,omitempty"`
	LatencyMs   int64          `json:"latency_ms"`
}

// ReactResult is the complete output of a ReAct session.
type ReactResult struct {
	FinalAnswer    string      `json:"final_answer"`
	Turns          []ReactTurn `json:"turns"`
	TotalLatencyMs int64       `json:"total_latency_ms"`
	TotalTokensIn  int         `json:"total_tokens_in"`
	TotalTokensOut int         `json:"total_tokens_out"`
}

// ReactLoopConfig configures the ReAct loop.
type ReactLoopConfig struct {
	LLMClient      llm.Client
	Executor       ToolExecutor
	Tools          []llm.ToolDefinition
	MaxTurns       int
	UseThinking    bool
	ThinkingBudget int
	StepTimeout    time.Duration
}

// ReactLoop implements the ReAct pattern: reason→act→observe, repeated until done.
type ReactLoop struct {
	cfg ReactLoopConfig
}

// NewReActLoop creates a ReAct loop.
func NewReActLoop(cfg ReactLoopConfig) *ReactLoop {
	if cfg.MaxTurns <= 0 {
		cfg.MaxTurns = reactMaxTurns
	}
	if cfg.StepTimeout <= 0 {
		cfg.StepTimeout = 30 * time.Second
	}
	return &ReactLoop{cfg: cfg}
}

const reactSystemPrompt = `You are Brevio, an executive AI assistant. Use the available tools
to accomplish the user's request step by step.

For each turn: call ONE tool with precise parameters.
When the task is fully complete, call the "final_answer" tool.
Be concise. Never call a tool unnecessarily.`

// Run executes the ReAct loop and returns the final result.
func (r *ReactLoop) Run(ctx context.Context, rc *ReasoningContext) (*ReactResult, error) {
	if rc == nil {
		return nil, fmt.Errorf("react_loop: nil context")
	}
	if r.cfg.LLMClient == nil {
		return nil, fmt.Errorf("react_loop: no LLM client")
	}

	tools := append(append([]llm.ToolDefinition{}, r.cfg.Tools...), llm.ToolDefinition{
		Name:        "final_answer",
		Description: "Signal task completion. Provide the response to the user.",
		InputSchema: map[string]any{
			"type":     "object",
			"required": []string{"answer"},
			"properties": map[string]any{
				"answer": map[string]any{"type": "string"},
			},
			"additionalProperties": false,
		},
	})

	baseMessages := make([]llm.ChatMsg, 0, len(rc.ConversationHistory)+1)
	for _, m := range rc.ConversationHistory {
		baseMessages = append(baseMessages, llm.ChatMsg{Role: m.Role, Content: m.Content})
	}
	userContent := rc.Intent
	if len(rc.RetrievedFacts) > 0 {
		userContent += formatRetrievedFacts(rc.RetrievedFacts)
	}
	baseMessages = append(baseMessages, llm.ChatMsg{Role: "user", Content: userContent})

	result := &ReactResult{}
	start := time.Now()

	var priorToolCalls []llm.AssistantToolUse
	var pendingResults []llm.ToolResult
	currentMessages := append([]llm.ChatMsg{}, baseMessages...)

	for turn := 0; turn < r.cfg.MaxTurns; turn++ {
		if err := ctx.Err(); err != nil {
			return result, fmt.Errorf("react_loop: cancelled at turn %d: %w", turn, err)
		}

		req := llm.GenerateRequest{
			Model:                   reactModel,
			MaxTokens:               reactMaxTokens,
			Temperature:             0.1,
			System:                  r.buildSystem(rc),
			Messages:                currentMessages,
			Tools:                   tools,
			ToolChoice:              llm.ToolChoiceAuto,
			PriorAssistantToolCalls: priorToolCalls,
			ToolResults:             pendingResults,
		}
		if r.cfg.UseThinking && turn == 0 {
			budget := r.cfg.ThinkingBudget
			if budget <= 0 {
				budget = 4096
			}
			req.Thinking = &llm.ThinkingConfig{Enabled: true, BudgetTokens: budget}
		}

		turnStart := time.Now()
		resp, usage, err := r.cfg.LLMClient.Generate(ctx, req)
		turnLatency := time.Since(turnStart).Milliseconds()
		if err != nil {
			return result, fmt.Errorf("react_loop: turn %d LLM: %w", turn, err)
		}
		if usage != nil {
			result.TotalTokensIn += usage.InputTokens
			result.TotalTokensOut += usage.OutputTokens
		}

		t := ReactTurn{TurnIndex: turn, Thought: resp.ThinkingContent, LatencyMs: turnLatency}

		// Check for final_answer tool call.
		for _, tc := range resp.ToolCalls {
			if tc.Name == "final_answer" {
				if ans, ok := tc.Input["answer"].(string); ok {
					t.IsFinal = true
					t.FinalAnswer = ans
					result.Turns = append(result.Turns, t)
					result.FinalAnswer = ans
					result.TotalLatencyMs = time.Since(start).Milliseconds()
					return result, nil
				}
			}
		}

		// Text response without tool call = final answer.
		if len(resp.ToolCalls) == 0 && resp.Content != "" {
			t.IsFinal = true
			t.FinalAnswer = resp.Content
			result.Turns = append(result.Turns, t)
			result.FinalAnswer = resp.Content
			result.TotalLatencyMs = time.Since(start).Milliseconds()
			return result, nil
		}

		// Process tool call — ReAct uses one tool per turn.
		priorToolCalls = nil
		pendingResults = nil
		if len(resp.ToolCalls) > 0 {
			tc := resp.ToolCalls[0]
			t.ToolName = tc.Name
			t.ToolCallID = tc.ID
			t.ToolInput = tc.Input

			obs, _ := r.executeTool(ctx, tc)
			t.Observation = obs

			priorToolCalls = []llm.AssistantToolUse{{
				ID:    tc.ID,
				Name:  tc.Name,
				Input: tc.Input,
				Text:  resp.Content,
			}}
			pendingResults = []llm.ToolResult{{
				ToolCallID: tc.ID,
				Content:    obs,
			}}
		}

		result.Turns = append(result.Turns, t)
	}

	result.TotalLatencyMs = time.Since(start).Milliseconds()
	return result, fmt.Errorf("react_loop: exceeded max turns (%d)", r.cfg.MaxTurns)
}

func (r *ReactLoop) executeTool(ctx context.Context, tc llm.ToolCall) (string, error) {
	if r.cfg.Executor == nil {
		return fmt.Sprintf(`{"status":"planned","tool":%q}`, tc.Name), nil
	}
	stepCtx, cancel := context.WithTimeout(ctx, r.cfg.StepTimeout)
	defer cancel()
	out, err := r.cfg.Executor.Execute(stepCtx, tc.Name, tc.Input)
	if err != nil {
		return fmt.Sprintf(`{"error":%q}`, err.Error()), err
	}
	enc, _ := json.Marshal(out)
	return string(enc), nil
}

func (r *ReactLoop) buildSystem(rc *ReasoningContext) string {
	if rc == nil || rc.WorkspaceID == "" {
		return reactSystemPrompt
	}
	return reactSystemPrompt + "\n\nWorkspace ID: " + rc.WorkspaceID
}
