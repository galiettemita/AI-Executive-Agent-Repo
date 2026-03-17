// Package reflection implements the hindsight reflection system.
// After each day of agent activity, it clusters intents, identifies failures,
// extracts preference signals, and writes structured insight records to memory.
package reflection

import "time"

// DayLog is a snapshot of one day's agent activity for a workspace.
type DayLog struct {
	WorkspaceID  string        `json:"workspace_id"`
	Date         string        `json:"date"`
	IntentEvents []IntentEvent `json:"intent_events"`
	ToolEvents   []ToolEvent   `json:"tool_events"`
}

// IntentEvent is one user intent processed during the day.
type IntentEvent struct {
	Intent       string   `json:"intent"`
	Confidence   float64  `json:"confidence"`
	Outcome      string   `json:"outcome"` // "success" | "failure" | "clarified" | "denied"
	ToolKeysUsed []string `json:"tool_keys_used"`
}

// ToolEvent is one tool execution during the day.
type ToolEvent struct {
	ToolKey   string `json:"tool_key"`
	Success   bool   `json:"success"`
	ErrorCode string `json:"error_code,omitempty"`
}

// IntentCluster groups similar intents into a theme.
type IntentCluster struct {
	Theme       string   `json:"theme"`
	Count       int      `json:"count"`
	Intents     []string `json:"intents"`
	SuccessRate float64  `json:"success_rate"`
}

// ToolFailurePattern describes a repeated tool failure pattern.
type ToolFailurePattern struct {
	ToolKey    string   `json:"tool_key"`
	FailCount  int      `json:"fail_count"`
	TotalCount int      `json:"total_count"`
	FailRate   float64  `json:"fail_rate"`
	ErrorCodes []string `json:"error_codes"`
	RootCause  string   `json:"root_cause"`
}

// DailyInsight is a structured record written to long-term memory.
type DailyInsight struct {
	WorkspaceID string         `json:"workspace_id"`
	Date        string         `json:"date"`
	InsightType string         `json:"insight_type"` // "intent_pattern" | "tool_failure"
	Body        string         `json:"body"`
	Strength    float64        `json:"strength"`
	Metadata    map[string]any `json:"metadata,omitempty"`
	CreatedAt   time.Time      `json:"created_at"`
}

// ReflectionInput is the activity input.
type ReflectionInput struct {
	WorkspaceID string `json:"workspace_id"`
	Date        string `json:"date"`
	MaxInsights int    `json:"max_insights,omitempty"`
}

// ReflectionResult is the activity output.
type ReflectionResult struct {
	WorkspaceID     string               `json:"workspace_id"`
	Date            string               `json:"date"`
	InsightsWritten int                  `json:"insights_written"`
	IntentClusters  []IntentCluster      `json:"intent_clusters"`
	FailurePatterns []ToolFailurePattern `json:"failure_patterns"`
	TopInsights     []DailyInsight       `json:"top_insights"`
}
