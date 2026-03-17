package a2a

import "time"

// AgentCard describes Brevio's capabilities to external A2A peers.
type AgentCard struct {
	Name         string       `json:"name"`
	Description  string       `json:"description"`
	Version      string       `json:"version"`
	URL          string       `json:"url"`
	Capabilities []Capability `json:"capabilities"`
	AuthSchemes  []AuthScheme `json:"auth_schemes"`
	Contact      string       `json:"contact,omitempty"`
}

// Capability describes one capability Brevio exposes to external agents.
type Capability struct {
	ID           string         `json:"id"`
	Name         string         `json:"name"`
	Description  string         `json:"description"`
	InputSchema  map[string]any `json:"input_schema"`
	OutputSchema map[string]any `json:"output_schema"`
}

// AuthScheme describes a supported authentication method.
type AuthScheme struct {
	Type     string   `json:"type"`
	TokenURL string   `json:"token_url"`
	Scopes   []string `json:"scopes"`
}

// TaskStatus represents the lifecycle state of an A2A task.
type TaskStatus string

const (
	TaskStatusSubmitted TaskStatus = "submitted"
	TaskStatusWorking   TaskStatus = "working"
	TaskStatusCompleted TaskStatus = "completed"
	TaskStatusFailed    TaskStatus = "failed"
	TaskStatusCancelled TaskStatus = "cancelled"
)

// Task is an A2A task record.
type Task struct {
	ID                string         `json:"id"`
	WorkspaceID       string         `json:"workspace_id"`
	RequestingAgentID string         `json:"requesting_agent_id"`
	Capability        string         `json:"capability"`
	InputPayload      map[string]any `json:"input"`
	Status            TaskStatus     `json:"status"`
	OutputPayload     map[string]any `json:"output,omitempty"`
	ErrorMessage      string         `json:"error,omitempty"`
	CreatedAt         time.Time      `json:"created_at"`
	UpdatedAt         time.Time      `json:"updated_at"`
	CompletedAt       *time.Time     `json:"completed_at,omitempty"`
}

// TaskCreateRequest is the body of POST /a2a/tasks.
type TaskCreateRequest struct {
	Capability  string         `json:"capability"`
	Input       map[string]any `json:"input"`
	CallbackURL string         `json:"callback_url,omitempty"`
}

// TaskUpdateRequest is used internally to transition task state.
type TaskUpdateRequest struct {
	Status TaskStatus     `json:"status"`
	Output map[string]any `json:"output,omitempty"`
	Error  string         `json:"error,omitempty"`
}

// SSEEvent is a Server-Sent Event for task status streaming.
type SSEEvent struct {
	Event string `json:"event"`
	Data  Task   `json:"data"`
}
