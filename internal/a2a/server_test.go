package a2a_test

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/brevio/brevio/internal/a2a"
)

const testToken = "test-m2m-token-abc123"

func setupTestServer() (*http.ServeMux, *a2a.TaskStore) {
	tokens := map[string]a2a.M2MToken{
		testToken: {
			AgentID:   "test-agent",
			Scopes:    []string{"a2a:tasks"},
			ExpiresAt: time.Now().Add(1 * time.Hour),
		},
	}
	validator := a2a.NewStaticM2MValidator(tokens)
	store := a2a.NewTaskStore(nil) // in-memory
	card := a2a.DefaultAgentCard("http://localhost:8080")
	server := a2a.NewServer(store, validator, card, nil)

	mux := http.NewServeMux()
	server.RegisterRoutes(mux)
	return mux, store
}

func TestA2AServer_AgentCardEndpoint_Returns200(t *testing.T) {
	mux, _ := setupTestServer()
	req := httptest.NewRequest("GET", "/.well-known/agent.json", nil)
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusOK, rr.Code)
	var card a2a.AgentCard
	require.NoError(t, json.Unmarshal(rr.Body.Bytes(), &card))
	assert.Equal(t, "Brevio AI Executive Agent", card.Name)
	assert.NotEmpty(t, card.Capabilities)
	assert.NotEmpty(t, card.AuthSchemes)
}

func TestA2AServer_CreateTask_ValidRequest_Returns202(t *testing.T) {
	mux, _ := setupTestServer()
	body, _ := json.Marshal(a2a.TaskCreateRequest{
		Capability: "schedule_meeting",
		Input: map[string]any{
			"workspace_id":     "ws-123",
			"title":            "Sync",
			"start_time":       "2026-03-17T10:00:00Z",
			"duration_minutes": 30,
		},
	})
	req := httptest.NewRequest("POST", "/a2a/tasks", bytes.NewReader(body))
	req.Header.Set("Authorization", "Bearer "+testToken)
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusAccepted, rr.Code)
	var task a2a.Task
	require.NoError(t, json.Unmarshal(rr.Body.Bytes(), &task))
	assert.NotEmpty(t, task.ID)
	assert.Equal(t, a2a.TaskStatusSubmitted, task.Status)
	assert.Equal(t, "schedule_meeting", task.Capability)
}

func TestA2AServer_CreateTask_MissingCapability_Returns400(t *testing.T) {
	mux, _ := setupTestServer()
	body, _ := json.Marshal(a2a.TaskCreateRequest{Input: map[string]any{"workspace_id": "ws-1"}})
	req := httptest.NewRequest("POST", "/a2a/tasks", bytes.NewReader(body))
	req.Header.Set("Authorization", "Bearer "+testToken)
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, req)
	assert.Equal(t, http.StatusBadRequest, rr.Code)
}

func TestA2AServer_CreateTask_UnsupportedCapability_Returns400(t *testing.T) {
	mux, _ := setupTestServer()
	body, _ := json.Marshal(a2a.TaskCreateRequest{
		Capability: "fly_to_moon",
		Input:      map[string]any{"workspace_id": "ws-1"},
	})
	req := httptest.NewRequest("POST", "/a2a/tasks", bytes.NewReader(body))
	req.Header.Set("Authorization", "Bearer "+testToken)
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, req)
	assert.Equal(t, http.StatusBadRequest, rr.Code)
}

func TestA2AServer_CreateTask_MissingToken_Returns401(t *testing.T) {
	mux, _ := setupTestServer()
	body, _ := json.Marshal(a2a.TaskCreateRequest{
		Capability: "schedule_meeting",
		Input:      map[string]any{"workspace_id": "ws-1"},
	})
	req := httptest.NewRequest("POST", "/a2a/tasks", bytes.NewReader(body))
	// No Authorization header
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, req)
	assert.Equal(t, http.StatusUnauthorized, rr.Code)
}

func TestA2AServer_GetTask_NotFound_Returns404(t *testing.T) {
	mux, _ := setupTestServer()
	req := httptest.NewRequest("GET", "/a2a/tasks/nonexistent-id", nil)
	req.Header.Set("Authorization", "Bearer "+testToken)
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, req)
	assert.Equal(t, http.StatusNotFound, rr.Code)
}

func TestA2AServer_GetTask_ExistingTask_Returns200(t *testing.T) {
	mux, store := setupTestServer()

	// Create a task directly in the store.
	task := a2a.Task{
		ID:                "task-abc",
		WorkspaceID:       "ws-1",
		RequestingAgentID: "test-agent",
		Capability:        "schedule_meeting",
		InputPayload:      map[string]any{"title": "Test"},
	}
	_, err := store.Create(context.Background(), task)
	require.NoError(t, err)

	req := httptest.NewRequest("GET", "/a2a/tasks/task-abc", nil)
	req.Header.Set("Authorization", "Bearer "+testToken)
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusOK, rr.Code)
	var got a2a.Task
	require.NoError(t, json.Unmarshal(rr.Body.Bytes(), &got))
	assert.Equal(t, "task-abc", got.ID)
	assert.Equal(t, a2a.TaskStatusSubmitted, got.Status)
}

func TestTaskStore_CreateAndGet_RoundTrip(t *testing.T) {
	store := a2a.NewTaskStore(nil) // in-memory

	task := a2a.Task{
		ID:                "rt-001",
		WorkspaceID:       "ws-rt",
		RequestingAgentID: "agent-rt",
		Capability:        "send_email",
		InputPayload:      map[string]any{"to": "alice@example.com", "subject": "Hi"},
	}
	created, err := store.Create(context.Background(), task)
	require.NoError(t, err)
	assert.Equal(t, a2a.TaskStatusSubmitted, created.Status)

	got, err := store.Get(context.Background(), "rt-001")
	require.NoError(t, err)
	assert.Equal(t, "rt-001", got.ID)
	assert.Equal(t, "send_email", got.Capability)
	assert.Equal(t, "agent-rt", got.RequestingAgentID)

	// Update to completed.
	updated, err := store.Update(context.Background(), "rt-001", a2a.TaskUpdateRequest{
		Status: a2a.TaskStatusCompleted,
		Output: map[string]any{"message_id": "msg-123"},
	})
	require.NoError(t, err)
	assert.Equal(t, a2a.TaskStatusCompleted, updated.Status)
	assert.NotNil(t, updated.CompletedAt)
}
