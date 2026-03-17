package subagent

// OrchestratorInput drives the SubAgentOrchestratorWorkflow.
type OrchestratorInput struct {
	MessageID   string   `json:"message_id"`
	WorkspaceID string   `json:"workspace_id"`
	RawPayload  string   `json:"raw_payload"`
	Intent      string   `json:"intent"`
	ToolKeys    []string `json:"tool_keys"`
	ReceiptID   string   `json:"receipt_id"`
	Tier        string   `json:"tier,omitempty"`
}

// OrchestratorResult aggregates results from all parallel child workflows.
type OrchestratorResult struct {
	SubTasksLaunched int             `json:"sub_tasks_launched"`
	SubTasksComplete int             `json:"sub_tasks_complete"`
	SubTasksFailed   int             `json:"sub_tasks_failed"`
	MergedContext    string          `json:"merged_context"`
	Results          []SubTaskResult `json:"results"`
	TerminalState    string          `json:"terminal_state"`
}

// SubTaskResult captures the outcome of one child workflow execution.
type SubTaskResult struct {
	SubTaskID       string `json:"sub_task_id"`
	Domain          string `json:"domain"`
	TerminalState   string `json:"terminal_state"`
	ResponsePayload string `json:"response_payload,omitempty"`
	Error           string `json:"error,omitempty"`
}

// CheckAutonomyInput is the activity input for the A3+ gate check.
type CheckAutonomyInput struct {
	WorkspaceID  string `json:"workspace_id"`
	RequiredTier string `json:"required_tier"`
}

// CheckAutonomyResult is the gate verdict.
type CheckAutonomyResult struct {
	CurrentTier string `json:"current_tier"`
	Permitted   bool   `json:"permitted"`
	Reason      string `json:"reason"`
}

// DecomposeInput is the Temporal activity input for task decomposition.
type DecomposeInput struct {
	Intent   string   `json:"intent"`
	ToolKeys []string `json:"tool_keys"`
}
