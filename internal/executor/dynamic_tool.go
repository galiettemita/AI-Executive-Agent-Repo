// Package executor implements tool execution, including dynamic tool creation
// via the CREATOR pattern (Qian et al. 2023).
package executor

import (
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
	"strings"
	"time"

	"github.com/brevio/brevio/internal/llm"
)

// DynamicTool is a runtime-generated Python tool produced by the CREATOR pattern.
// Tools expire 2 hours after creation; Execute rejects expired tools.
type DynamicTool struct {
	ID          string
	Name        string
	Description string
	Code        string
	CreatedFor  string
	ExpiresAt   time.Time
}

// DynamicToolResult holds the output of one dynamic tool execution.
type DynamicToolResult struct {
	Output    map[string]any
	ExitCode  int
	Error     string
	LatencyMs int64
}

// DynamicToolCreator implements the CREATOR pattern (Qian et al. 2023).
type DynamicToolCreator struct {
	llmClient llm.Client
}

// NewDynamicToolCreator returns a DynamicToolCreator backed by the given client.
func NewDynamicToolCreator(c llm.Client) *DynamicToolCreator {
	return &DynamicToolCreator{llmClient: c}
}

const createToolSystemPrompt = `Create a Python function. OUTPUT JSON: {"name":"snake_case","description":"...","code":"def execute(input_data: dict) -> dict:\n ..."}. Only stdlib+requests+json+math+re+datetime. No os,sys,subprocess,socket,open,eval,exec. Max 40 lines. Handle errors with try/except.`

var blockedPatterns = []string{
	"import os",
	"import sys",
	"import subprocess",
	"import socket",
	"__import__",
	"exec(",
	"eval(",
	"open(",
	"file(",
	"input(",
	"os.system",
	"os.popen",
	"subprocess.",
	"shutil.",
	"glob.",
	"pathlib.",
	"tempfile.",
	"pickle.",
	"marshal.",
}

// validateDynamicCode returns an error if code contains any blocked pattern.
// Check is case-insensitive per Plan §6 Step 3.
func validateDynamicCode(code string) error {
	lowerCode := strings.ToLower(code)
	for _, pattern := range blockedPatterns {
		if strings.Contains(lowerCode, strings.ToLower(pattern)) {
			return fmt.Errorf("dynamic_tool: blocked pattern %q in generated code", pattern)
		}
	}
	return nil
}

type llmToolResponse struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	Code        string `json:"code"`
}

// CreateTool calls the LLM to generate a purpose-built Python tool for
// the given task description. Plan §6 Step 2.
func (c *DynamicToolCreator) CreateTool(ctx context.Context, taskDescription string) (*DynamicTool, error) {
	if strings.TrimSpace(taskDescription) == "" {
		return nil, fmt.Errorf("dynamic_tool: taskDescription must not be empty")
	}

	resp, _, err := c.llmClient.Generate(ctx, llm.GenerateRequest{
		Model:     "claude-haiku-4-5-20251001",
		MaxTokens: 1024,
		Messages: []llm.ChatMsg{
			{Role: "system", Content: createToolSystemPrompt},
			{Role: "user", Content: taskDescription},
		},
	})
	if err != nil {
		return nil, fmt.Errorf("dynamic_tool: LLM call failed: %w", err)
	}

	raw := resp.Content
	cleaned := strings.TrimSpace(raw)
	if strings.HasPrefix(cleaned, "```") {
		if idx := strings.Index(cleaned[3:], "```"); idx >= 0 {
			start := strings.Index(cleaned, "\n") + 1
			cleaned = strings.TrimSpace(cleaned[start : start+idx])
		}
	}

	var toolResp llmToolResponse
	if err := json.Unmarshal([]byte(cleaned), &toolResp); err != nil {
		return nil, fmt.Errorf("dynamic_tool: LLM returned non-JSON: %w (raw: %.200s)", err, raw)
	}
	if toolResp.Name == "" {
		return nil, fmt.Errorf("dynamic_tool: LLM response missing 'name' field")
	}
	if toolResp.Code == "" {
		return nil, fmt.Errorf("dynamic_tool: LLM response missing 'code' field")
	}

	if err := validateDynamicCode(toolResp.Code); err != nil {
		return nil, fmt.Errorf("dynamic_tool: code failed security validation: %w", err)
	}

	return &DynamicTool{
		ID:          fmt.Sprintf("dyn_%d", time.Now().UnixNano()),
		Name:        toolResp.Name,
		Description: toolResp.Description,
		Code:        toolResp.Code,
		CreatedFor:  taskDescription,
		ExpiresAt:   time.Now().Add(2 * time.Hour),
	}, nil
}

// Execute runs a DynamicTool as a Python subprocess with a 15s timeout.
// Plan §6 Step 4.
func (c *DynamicToolCreator) Execute(ctx context.Context, tool *DynamicTool, input map[string]any) (*DynamicToolResult, error) {
	if tool == nil {
		return nil, fmt.Errorf("dynamic_tool: tool must not be nil")
	}

	if tool.ExpiresAt.Before(time.Now()) {
		return nil, fmt.Errorf("dynamic_tool: tool %q expired at %s",
			tool.Name, tool.ExpiresAt.Format(time.RFC3339))
	}

	jsonInput, err := json.Marshal(input)
	if err != nil {
		return nil, fmt.Errorf("dynamic_tool: failed to marshal input: %w", err)
	}

	script := tool.Code + `

import sys as _sys
import json as _json

if __name__ == "__main__":
    _input = _json.loads(_sys.argv[1])
    _result = execute(_input)
    print(_json.dumps(_result))
`

	execCtx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()

	start := time.Now()

	cmd := exec.CommandContext(execCtx, "python3", "-c", script, string(jsonInput))

	var stdout, stderr strings.Builder
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	runErr := cmd.Run()
	latencyMs := time.Since(start).Milliseconds()

	exitCode := 0
	if cmd.ProcessState != nil {
		exitCode = cmd.ProcessState.ExitCode()
	}

	result := &DynamicToolResult{
		ExitCode:  exitCode,
		LatencyMs: latencyMs,
	}

	if runErr != nil {
		result.Error = fmt.Sprintf("subprocess error: %v; stderr: %s", runErr, stderr.String())
		return result, fmt.Errorf("dynamic_tool: subprocess failed: %w", runErr)
	}

	outStr := strings.TrimSpace(stdout.String())
	if outStr == "" {
		result.Error = fmt.Sprintf("subprocess produced no output; stderr: %s", stderr.String())
		return result, fmt.Errorf("dynamic_tool: subprocess produced empty stdout")
	}

	var output map[string]any
	if err := json.Unmarshal([]byte(outStr), &output); err != nil {
		result.Error = fmt.Sprintf("stdout is not valid JSON: %s", outStr)
		return result, fmt.Errorf("dynamic_tool: stdout JSON unmarshal failed: %w", err)
	}

	result.Output = output
	return result, nil
}
