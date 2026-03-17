package a2a

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/google/uuid"
)

const brevioA2AVersion = "1.0.0"

// TaskDispatcher submits a task to the Temporal workflow engine for processing.
type TaskDispatcher interface {
	DispatchA2ATask(ctx context.Context, task Task) error
}

// Server is the A2A HTTP server.
type Server struct {
	taskStore  *TaskStore
	validator  M2MTokenValidator
	agentCard  AgentCard
	dispatcher TaskDispatcher
}

// NewServer creates an A2A HTTP server.
func NewServer(taskStore *TaskStore, validator M2MTokenValidator, agentCard AgentCard, dispatcher TaskDispatcher) *Server {
	return &Server{taskStore: taskStore, validator: validator, agentCard: agentCard, dispatcher: dispatcher}
}

// RegisterRoutes mounts A2A endpoints on the given ServeMux.
func (s *Server) RegisterRoutes(mux *http.ServeMux) {
	auth := M2MAuthMiddleware(s.validator)
	mux.Handle("GET /.well-known/agent.json", http.HandlerFunc(s.handleAgentCard))
	mux.Handle("POST /a2a/tasks", auth(http.HandlerFunc(s.handleCreateTask)))
	mux.Handle("GET /a2a/tasks/", auth(http.HandlerFunc(s.handleGetTask)))
}

func (s *Server) handleAgentCard(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(s.agentCard)
}

func (s *Server) handleCreateTask(w http.ResponseWriter, r *http.Request) {
	agentID, ok := AgentIDFromContext(r.Context())
	if !ok {
		http.Error(w, `{"error":"agent identity required"}`, http.StatusUnauthorized)
		return
	}

	var req TaskCreateRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, `{"error":"invalid request body"}`, http.StatusBadRequest)
		return
	}
	if req.Capability == "" {
		http.Error(w, `{"error":"capability required"}`, http.StatusBadRequest)
		return
	}
	if !s.isCapabilitySupported(req.Capability) {
		http.Error(w, fmt.Sprintf(`{"error":"unsupported capability: %s"}`, req.Capability), http.StatusBadRequest)
		return
	}

	workspaceID, _ := req.Input["workspace_id"].(string)
	if workspaceID == "" {
		http.Error(w, `{"error":"workspace_id required in input"}`, http.StatusBadRequest)
		return
	}

	task := Task{
		ID:                uuid.New().String(),
		WorkspaceID:       workspaceID,
		RequestingAgentID: agentID,
		Capability:        req.Capability,
		InputPayload:      req.Input,
		Status:            TaskStatusSubmitted,
	}

	created, err := s.taskStore.Create(r.Context(), task)
	if err != nil {
		http.Error(w, `{"error":"failed to create task"}`, http.StatusInternalServerError)
		return
	}

	if s.dispatcher != nil {
		_ = s.dispatcher.DispatchA2ATask(r.Context(), *created)
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusAccepted)
	_ = json.NewEncoder(w).Encode(created)
}

func (s *Server) handleGetTask(w http.ResponseWriter, r *http.Request) {
	parts := strings.Split(strings.TrimPrefix(r.URL.Path, "/a2a/tasks/"), "/")
	taskID := parts[0]
	if taskID == "" {
		http.Error(w, `{"error":"task id required"}`, http.StatusBadRequest)
		return
	}

	if len(parts) > 1 && parts[1] == "stream" {
		StreamTaskStatus(r.Context(), w, taskID, s.taskStore, 10*time.Minute)
		return
	}

	task, err := s.taskStore.Get(r.Context(), taskID)
	if err != nil {
		http.Error(w, `{"error":"task not found"}`, http.StatusNotFound)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(task)
}

func (s *Server) isCapabilitySupported(capability string) bool {
	for _, c := range s.agentCard.Capabilities {
		if c.ID == capability {
			return true
		}
	}
	return false
}

// DefaultAgentCard returns Brevio's A2A AgentCard with supported capabilities.
func DefaultAgentCard(baseURL string) AgentCard {
	return AgentCard{
		Name:        "Brevio AI Executive Agent",
		Description: "AI executive assistant for WhatsApp and iMessage — calendar, email, research, scheduling",
		Version:     brevioA2AVersion,
		URL:         baseURL,
		Capabilities: []Capability{
			{
				ID:          "schedule_meeting",
				Name:        "Schedule Meeting",
				Description: "Schedule a calendar event on behalf of a Brevio workspace user",
				InputSchema: map[string]any{
					"type":     "object",
					"required": []string{"workspace_id", "title", "start_time", "duration_minutes"},
				},
				OutputSchema: map[string]any{
					"type": "object",
					"properties": map[string]any{
						"event_id": map[string]any{"type": "string"},
						"status":   map[string]any{"type": "string"},
					},
				},
			},
			{
				ID:          "send_email",
				Name:        "Send Email",
				Description: "Draft and send an email on behalf of a Brevio workspace user",
				InputSchema: map[string]any{
					"type":     "object",
					"required": []string{"workspace_id", "to", "subject", "body"},
				},
				OutputSchema: map[string]any{
					"type": "object",
					"properties": map[string]any{
						"message_id": map[string]any{"type": "string"},
					},
				},
			},
		},
		AuthSchemes: []AuthScheme{
			{
				Type:     "oauth2_m2m",
				TokenURL: baseURL + "/oauth/token",
				Scopes:   []string{"a2a:tasks"},
			},
		},
	}
}
