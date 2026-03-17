package a2a_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/brevio/brevio/internal/a2a"
)

// mockExternalAgent sets up a mock A2A server that supports "book_travel" capability.
func mockExternalAgent(t *testing.T, taskStatus a2a.TaskStatus) *httptest.Server {
	var taskID string
	var callCount atomic.Int32

	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.URL.Path == "/.well-known/agent.json":
			json.NewEncoder(w).Encode(a2a.AgentCard{
				Name: "Mock Travel Agent",
				Capabilities: []a2a.Capability{
					{ID: "book_travel", Name: "Book Travel"},
				},
			})

		case r.URL.Path == "/a2a/tasks" && r.Method == "POST":
			var req a2a.TaskCreateRequest
			json.NewDecoder(r.Body).Decode(&req)
			taskID = "mock-task-001"
			w.WriteHeader(http.StatusAccepted)
			json.NewEncoder(w).Encode(a2a.Task{
				ID:         taskID,
				Capability: req.Capability,
				Status:     a2a.TaskStatusSubmitted,
			})

		case r.Method == "GET" && len(r.URL.Path) > len("/a2a/tasks/"):
			count := callCount.Add(1)
			status := a2a.TaskStatusWorking
			if count >= 2 || taskStatus == a2a.TaskStatusCompleted {
				status = taskStatus
			}
			json.NewEncoder(w).Encode(a2a.Task{
				ID:            taskID,
				Status:        status,
				OutputPayload: map[string]any{"booking_id": "BK-123"},
			})
		}
	}))
}

func TestA2AClient_Delegate_SuccessfulDelegation(t *testing.T) {
	server := mockExternalAgent(t, a2a.TaskStatusCompleted)
	defer server.Close()

	registry := a2a.NewExternalAgentRegistry(nil)
	_ = registry.Register(context.Background(), a2a.ExternalAgent{
		Name: "travel", BaseURL: server.URL, M2MToken: "tok",
		Capabilities: []string{"book_travel"}, IsActive: true,
	})

	client := a2a.NewA2AClient(registry, nil)
	result, err := client.Delegate(context.Background(), a2a.DelegateRequest{
		WorkspaceID: "ws-1", Capability: "book_travel",
		Input: map[string]any{"destination": "NYC"}, TimeoutSecs: 10,
	})
	require.NoError(t, err)
	assert.Equal(t, a2a.TaskStatusCompleted, result.Status)
	assert.Equal(t, "travel", result.AgentName)
	assert.NotEmpty(t, result.TaskID)
}

func TestA2AClient_Delegate_AgentNotFound_ReturnsError(t *testing.T) {
	registry := a2a.NewExternalAgentRegistry(nil) // empty registry
	client := a2a.NewA2AClient(registry, nil)
	_, err := client.Delegate(context.Background(), a2a.DelegateRequest{
		WorkspaceID: "ws-1", Capability: "unknown_cap",
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "no agent for capability")
}

func TestA2AClient_Delegate_UnsupportedCapability_ReturnsError(t *testing.T) {
	// Server only supports "book_travel", but we ask for "launch_rocket".
	server := mockExternalAgent(t, a2a.TaskStatusCompleted)
	defer server.Close()

	registry := a2a.NewExternalAgentRegistry(nil)
	_ = registry.Register(context.Background(), a2a.ExternalAgent{
		Name: "travel", BaseURL: server.URL, M2MToken: "tok",
		Capabilities: []string{"launch_rocket"}, IsActive: true,
	})

	client := a2a.NewA2AClient(registry, nil)
	_, err := client.Delegate(context.Background(), a2a.DelegateRequest{
		WorkspaceID: "ws-1", Capability: "launch_rocket",
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "does not support capability")
}

func TestA2AClient_Delegate_PollsUntilCompleted(t *testing.T) {
	var pollCount atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.URL.Path == "/.well-known/agent.json":
			json.NewEncoder(w).Encode(a2a.AgentCard{
				Capabilities: []a2a.Capability{{ID: "analyze"}},
			})
		case r.Method == "POST":
			w.WriteHeader(http.StatusAccepted)
			json.NewEncoder(w).Encode(a2a.Task{ID: "poll-task", Status: a2a.TaskStatusSubmitted})
		case r.Method == "GET":
			n := pollCount.Add(1)
			status := a2a.TaskStatusWorking
			if n >= 3 {
				status = a2a.TaskStatusCompleted
			}
			json.NewEncoder(w).Encode(a2a.Task{ID: "poll-task", Status: status, OutputPayload: map[string]any{"done": true}})
		}
	}))
	defer server.Close()

	registry := a2a.NewExternalAgentRegistry(nil)
	_ = registry.Register(context.Background(), a2a.ExternalAgent{
		Name: "analyzer", BaseURL: server.URL, M2MToken: "t",
		Capabilities: []string{"analyze"}, IsActive: true,
	})

	client := a2a.NewA2AClient(registry, nil)
	result, err := client.Delegate(context.Background(), a2a.DelegateRequest{
		WorkspaceID: "ws-1", Capability: "analyze", TimeoutSecs: 30,
	})
	require.NoError(t, err)
	assert.Equal(t, a2a.TaskStatusCompleted, result.Status)
	assert.GreaterOrEqual(t, int(pollCount.Load()), 3)
}

func TestA2AClient_Delegate_TimesOutAfterDeadline(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.URL.Path == "/.well-known/agent.json":
			json.NewEncoder(w).Encode(a2a.AgentCard{Capabilities: []a2a.Capability{{ID: "slow"}}})
		case r.Method == "POST":
			w.WriteHeader(http.StatusAccepted)
			json.NewEncoder(w).Encode(a2a.Task{ID: "slow-task", Status: a2a.TaskStatusSubmitted})
		case r.Method == "GET":
			// Always return working — never completes.
			json.NewEncoder(w).Encode(a2a.Task{ID: "slow-task", Status: a2a.TaskStatusWorking})
		}
	}))
	defer server.Close()

	registry := a2a.NewExternalAgentRegistry(nil)
	_ = registry.Register(context.Background(), a2a.ExternalAgent{
		Name: "slow-agent", BaseURL: server.URL, M2MToken: "t",
		Capabilities: []string{"slow"}, IsActive: true,
	})

	client := a2a.NewA2AClient(registry, nil)
	start := time.Now()
	result, err := client.Delegate(context.Background(), a2a.DelegateRequest{
		WorkspaceID: "ws-1", Capability: "slow", TimeoutSecs: 5,
	})
	elapsed := time.Since(start)
	require.NoError(t, err) // timeout returns a result, not an error
	assert.Equal(t, a2a.TaskStatusFailed, result.Status)
	assert.Contains(t, result.Error, "timeout")
	assert.Less(t, elapsed, 15*time.Second)
}

func TestExternalAgentRegistry_FindByCapability_ReturnsFirst(t *testing.T) {
	registry := a2a.NewExternalAgentRegistry(nil)
	_ = registry.Register(context.Background(), a2a.ExternalAgent{
		Name: "agent-a", BaseURL: "http://a", M2MToken: "t",
		Capabilities: []string{"cap1", "cap2"}, IsActive: true,
	})
	_ = registry.Register(context.Background(), a2a.ExternalAgent{
		Name: "agent-b", BaseURL: "http://b", M2MToken: "t",
		Capabilities: []string{"cap2", "cap3"}, IsActive: true,
	})

	agent, err := registry.FindByCapability(context.Background(), "cap2")
	require.NoError(t, err)
	assert.Equal(t, "agent-a", agent.Name, "should return first registered agent with capability")
}
