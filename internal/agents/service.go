package agents

import (
	"encoding/json"
	"net/http"

	runtimeserver "github.com/brevio/brevio/internal/runtime"
)

type Config struct {
	DatabaseURL  string
	RedisURL     string
	TemporalHost string
}

type Service struct {
	config Config
	logger *runtimeserver.JSONLogger
}

func NewService(config Config, logger *runtimeserver.JSONLogger) *Service {
	return &Service{config: config, logger: logger}
}

func (s *Service) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("POST /api/v1/agents/definitions", s.handleCreateDefinition)
	mux.HandleFunc("GET /api/v1/agents/definitions", s.handleListDefinitions)
	mux.HandleFunc("GET /api/v1/agents/definitions/{id}", s.handleGetDefinition)
	mux.HandleFunc("PUT /api/v1/agents/definitions/{id}", s.handleUpdateDefinition)
	mux.HandleFunc("DELETE /api/v1/agents/definitions/{id}", s.handleDeleteDefinition)

	mux.HandleFunc("POST /api/v1/agents/execute", s.handleExecute)
	mux.HandleFunc("GET /api/v1/agents/executions", s.handleListExecutions)
	mux.HandleFunc("GET /api/v1/agents/executions/{id}", s.handleGetExecution)
	mux.HandleFunc("POST /api/v1/agents/executions/{id}/cancel", s.handleCancelExecution)
	mux.HandleFunc("GET /api/v1/agents/executions/{id}/messages", s.handleGetMessages)

	mux.HandleFunc("GET /api/v1/agents/tools", s.handleListTools)
	mux.HandleFunc("POST /api/v1/agents/tools", s.handleRegisterTool)

	mux.HandleFunc("POST /api/v1/agents/delegation-rules", s.handleCreateDelegationRule)
	mux.HandleFunc("GET /api/v1/agents/delegation-rules", s.handleListDelegationRules)
}

func (s *Service) handleCreateDefinition(w http.ResponseWriter, r *http.Request) {
	s.respondJSON(w, http.StatusCreated, map[string]any{"status": "created"})
}

func (s *Service) handleListDefinitions(w http.ResponseWriter, r *http.Request) {
	s.respondJSON(w, http.StatusOK, map[string]any{"definitions": []any{}})
}

func (s *Service) handleGetDefinition(w http.ResponseWriter, r *http.Request) {
	s.respondJSON(w, http.StatusOK, map[string]any{"id": r.PathValue("id")})
}

func (s *Service) handleUpdateDefinition(w http.ResponseWriter, r *http.Request) {
	s.respondJSON(w, http.StatusOK, map[string]any{"status": "updated"})
}

func (s *Service) handleDeleteDefinition(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusNoContent)
}

func (s *Service) handleExecute(w http.ResponseWriter, r *http.Request) {
	s.respondJSON(w, http.StatusAccepted, map[string]any{"status": "executing"})
}

func (s *Service) handleListExecutions(w http.ResponseWriter, r *http.Request) {
	s.respondJSON(w, http.StatusOK, map[string]any{"executions": []any{}})
}

func (s *Service) handleGetExecution(w http.ResponseWriter, r *http.Request) {
	s.respondJSON(w, http.StatusOK, map[string]any{"id": r.PathValue("id")})
}

func (s *Service) handleCancelExecution(w http.ResponseWriter, r *http.Request) {
	s.respondJSON(w, http.StatusOK, map[string]any{"status": "cancelled"})
}

func (s *Service) handleGetMessages(w http.ResponseWriter, r *http.Request) {
	s.respondJSON(w, http.StatusOK, map[string]any{"messages": []any{}})
}

func (s *Service) handleListTools(w http.ResponseWriter, r *http.Request) {
	s.respondJSON(w, http.StatusOK, map[string]any{"tools": []any{}})
}

func (s *Service) handleRegisterTool(w http.ResponseWriter, r *http.Request) {
	s.respondJSON(w, http.StatusCreated, map[string]any{"status": "registered"})
}

func (s *Service) handleCreateDelegationRule(w http.ResponseWriter, r *http.Request) {
	s.respondJSON(w, http.StatusCreated, map[string]any{"status": "created"})
}

func (s *Service) handleListDelegationRules(w http.ResponseWriter, r *http.Request) {
	s.respondJSON(w, http.StatusOK, map[string]any{"rules": []any{}})
}

func (s *Service) respondJSON(w http.ResponseWriter, status int, data any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(data)
}
