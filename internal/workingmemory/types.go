package workingmemory

import "time"

// Item is the in-flight state for a single agentic task.
// One item per (WorkspaceID, TaskID). Overwritten on each state update.
// Redis key: "wm:{WorkspaceID}:{TaskID}"
type Item struct {
	TaskID      string `json:"task_id"`
	WorkspaceID string `json:"workspace_id"`
	UserID      string `json:"user_id"`

	// ScratchPad holds arbitrary JSON-serializable task state.
	ScratchPad map[string]any `json:"scratch_pad"`

	// WorkflowID is the Temporal workflow ID if this task runs in a workflow.
	WorkflowID    string `json:"workflow_id,omitempty"`
	WorkflowRunID string `json:"workflow_run_id,omitempty"`

	// Stage is the current step name within the workflow.
	Stage string `json:"stage,omitempty"`

	// PendingToolCalls holds dispatched-but-unresolved tool calls.
	PendingToolCalls map[string]PendingToolCall `json:"pending_tool_calls,omitempty"`

	// ContextSummary is a short text injected into the context assembly slot.
	ContextSummary string `json:"context_summary,omitempty"`

	TTL       time.Duration `json:"ttl"`
	CreatedAt time.Time     `json:"created_at"`
	UpdatedAt time.Time     `json:"updated_at"`
}

// PendingToolCall records a dispatched tool call awaiting a response.
type PendingToolCall struct {
	ToolName   string         `json:"tool_name"`
	ToolCallID string         `json:"tool_call_id"`
	Input      map[string]any `json:"input"`
	IssuedAt   time.Time      `json:"issued_at"`
}

const (
	DefaultTTL  = 4 * time.Hour
	WorkflowTTL = 24 * time.Hour
	KeyPrefix   = "wm"
)
