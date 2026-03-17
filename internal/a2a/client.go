package a2a

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

// A2AClient delegates tasks to external A2A-compatible agents.
type A2AClient struct {
	httpClient *http.Client
	registry   *ExternalAgentRegistry
	taskStore  *TaskStore
}

// NewA2AClient creates an A2AClient.
func NewA2AClient(registry *ExternalAgentRegistry, taskStore *TaskStore) *A2AClient {
	return &A2AClient{
		httpClient: &http.Client{Timeout: 30 * time.Second},
		registry:   registry,
		taskStore:  taskStore,
	}
}

// DelegateRequest is the input for outbound A2A task delegation.
type DelegateRequest struct {
	WorkspaceID string         `json:"workspace_id"`
	Capability  string         `json:"capability"`
	Input       map[string]any `json:"input"`
	TimeoutSecs int            `json:"timeout_secs,omitempty"`
}

// DelegateResult is the output of an outbound A2A delegation.
type DelegateResult struct {
	TaskID    string         `json:"task_id"`
	AgentName string         `json:"agent_name"`
	Status    TaskStatus     `json:"status"`
	Output    map[string]any `json:"output,omitempty"`
	Error     string         `json:"error,omitempty"`
}

// Delegate finds an external agent, submits the task, and polls for completion.
func (c *A2AClient) Delegate(ctx context.Context, req DelegateRequest) (*DelegateResult, error) {
	agent, err := c.registry.FindByCapability(ctx, req.Capability)
	if err != nil {
		return nil, fmt.Errorf("a2a_client: no agent for capability %q: %w", req.Capability, err)
	}

	card, err := c.fetchAgentCard(ctx, agent.BaseURL, agent.M2MToken)
	if err != nil {
		return nil, fmt.Errorf("a2a_client: fetch agent card: %w", err)
	}
	if !cardSupportsCapability(card, req.Capability) {
		return nil, fmt.Errorf("a2a_client: agent %q does not support capability %q", agent.Name, req.Capability)
	}

	submitted, err := c.submitTask(ctx, agent.BaseURL, agent.M2MToken, TaskCreateRequest{
		Capability: req.Capability,
		Input:      req.Input,
	})
	if err != nil {
		return nil, fmt.Errorf("a2a_client: submit task: %w", err)
	}

	if c.taskStore != nil {
		outbound := Task{
			ID: "outbound-" + submitted.ID, WorkspaceID: req.WorkspaceID,
			RequestingAgentID: "brevio", Capability: req.Capability,
			InputPayload: req.Input, Status: TaskStatusSubmitted,
		}
		_, _ = c.taskStore.Create(ctx, outbound)
	}

	timeout := time.Duration(req.TimeoutSecs) * time.Second
	if timeout <= 0 || timeout > 5*time.Minute {
		timeout = 2 * time.Minute
	}
	deadline := time.Now().Add(timeout)

	for time.Now().Before(deadline) {
		task, pollErr := c.getTask(ctx, agent.BaseURL, agent.M2MToken, submitted.ID)
		if pollErr != nil {
			time.Sleep(2 * time.Second)
			continue
		}
		if isTerminalStatus(task.Status) {
			return &DelegateResult{
				TaskID: submitted.ID, AgentName: agent.Name,
				Status: task.Status, Output: task.OutputPayload, Error: task.ErrorMessage,
			}, nil
		}
		time.Sleep(2 * time.Second)
	}

	return &DelegateResult{
		TaskID: submitted.ID, AgentName: agent.Name,
		Status: TaskStatusFailed, Error: "delegation timeout exceeded",
	}, nil
}

func (c *A2AClient) fetchAgentCard(ctx context.Context, baseURL, token string) (*AgentCard, error) {
	req, _ := http.NewRequestWithContext(ctx, http.MethodGet, baseURL+"/.well-known/agent.json", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	var card AgentCard
	if err := json.NewDecoder(resp.Body).Decode(&card); err != nil {
		return nil, err
	}
	return &card, nil
}

func (c *A2AClient) submitTask(ctx context.Context, baseURL, token string, req TaskCreateRequest) (*Task, error) {
	body, _ := json.Marshal(req)
	httpReq, _ := http.NewRequestWithContext(ctx, http.MethodPost, baseURL+"/a2a/tasks", bytes.NewReader(body))
	httpReq.Header.Set("Authorization", "Bearer "+token)
	httpReq.Header.Set("Content-Type", "application/json")
	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusAccepted {
		b, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("submit_task: status %d: %s", resp.StatusCode, b)
	}
	var task Task
	if err := json.NewDecoder(resp.Body).Decode(&task); err != nil {
		return nil, err
	}
	return &task, nil
}

func (c *A2AClient) getTask(ctx context.Context, baseURL, token, taskID string) (*Task, error) {
	req, _ := http.NewRequestWithContext(ctx, http.MethodGet, baseURL+"/a2a/tasks/"+taskID, nil)
	req.Header.Set("Authorization", "Bearer "+token)
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	var task Task
	if err := json.NewDecoder(resp.Body).Decode(&task); err != nil {
		return nil, err
	}
	return &task, nil
}

func cardSupportsCapability(card *AgentCard, capability string) bool {
	for _, c := range card.Capabilities {
		if c.ID == capability {
			return true
		}
	}
	return false
}
