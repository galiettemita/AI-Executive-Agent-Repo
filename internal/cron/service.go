package cron

import (
	"encoding/json"
	"net/http"

	runtimeserver "github.com/brevio/brevio/internal/runtime"
)

type Config struct {
	DatabaseURL string
	RedisURL    string
}

type Service struct {
	config Config
	logger *runtimeserver.JSONLogger
}

func NewService(config Config, logger *runtimeserver.JSONLogger) *Service {
	return &Service{config: config, logger: logger}
}

func (s *Service) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("POST /api/v1/cron/jobs", s.handleCreateJob)
	mux.HandleFunc("GET /api/v1/cron/jobs", s.handleListJobs)
	mux.HandleFunc("GET /api/v1/cron/jobs/{id}", s.handleGetJob)
	mux.HandleFunc("PUT /api/v1/cron/jobs/{id}", s.handleUpdateJob)
	mux.HandleFunc("DELETE /api/v1/cron/jobs/{id}", s.handleDeleteJob)
	mux.HandleFunc("POST /api/v1/cron/jobs/{id}/pause", s.handlePauseJob)
	mux.HandleFunc("POST /api/v1/cron/jobs/{id}/resume", s.handleResumeJob)
	mux.HandleFunc("POST /api/v1/cron/jobs/{id}/trigger", s.handleTriggerJob)

	mux.HandleFunc("GET /api/v1/cron/executions", s.handleListExecutions)
	mux.HandleFunc("GET /api/v1/cron/executions/{id}", s.handleGetExecution)

	mux.HandleFunc("POST /api/v1/cron/webhooks", s.handleCreateWebhook)
	mux.HandleFunc("GET /api/v1/cron/webhooks", s.handleListWebhooks)
	mux.HandleFunc("DELETE /api/v1/cron/webhooks/{id}", s.handleDeleteWebhook)

	mux.HandleFunc("POST /api/v1/cron/notifications", s.handleCreateNotification)
	mux.HandleFunc("GET /api/v1/cron/notifications", s.handleListNotifications)

	mux.HandleFunc("GET /api/v1/cron/audit", s.handleGetAuditLog)
}

func (s *Service) handleCreateJob(w http.ResponseWriter, r *http.Request) {
	s.respondJSON(w, http.StatusCreated, map[string]any{"status": "created"})
}

func (s *Service) handleListJobs(w http.ResponseWriter, r *http.Request) {
	s.respondJSON(w, http.StatusOK, map[string]any{"jobs": []any{}})
}

func (s *Service) handleGetJob(w http.ResponseWriter, r *http.Request) {
	s.respondJSON(w, http.StatusOK, map[string]any{"id": r.PathValue("id")})
}

func (s *Service) handleUpdateJob(w http.ResponseWriter, r *http.Request) {
	s.respondJSON(w, http.StatusOK, map[string]any{"status": "updated"})
}

func (s *Service) handleDeleteJob(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusNoContent)
}

func (s *Service) handlePauseJob(w http.ResponseWriter, r *http.Request) {
	s.respondJSON(w, http.StatusOK, map[string]any{"status": "paused"})
}

func (s *Service) handleResumeJob(w http.ResponseWriter, r *http.Request) {
	s.respondJSON(w, http.StatusOK, map[string]any{"status": "active"})
}

func (s *Service) handleTriggerJob(w http.ResponseWriter, r *http.Request) {
	s.respondJSON(w, http.StatusAccepted, map[string]any{"status": "triggered"})
}

func (s *Service) handleListExecutions(w http.ResponseWriter, r *http.Request) {
	s.respondJSON(w, http.StatusOK, map[string]any{"executions": []any{}})
}

func (s *Service) handleGetExecution(w http.ResponseWriter, r *http.Request) {
	s.respondJSON(w, http.StatusOK, map[string]any{"id": r.PathValue("id")})
}

func (s *Service) handleCreateWebhook(w http.ResponseWriter, r *http.Request) {
	s.respondJSON(w, http.StatusCreated, map[string]any{"status": "created"})
}

func (s *Service) handleListWebhooks(w http.ResponseWriter, r *http.Request) {
	s.respondJSON(w, http.StatusOK, map[string]any{"webhooks": []any{}})
}

func (s *Service) handleDeleteWebhook(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusNoContent)
}

func (s *Service) handleCreateNotification(w http.ResponseWriter, r *http.Request) {
	s.respondJSON(w, http.StatusCreated, map[string]any{"status": "created"})
}

func (s *Service) handleListNotifications(w http.ResponseWriter, r *http.Request) {
	s.respondJSON(w, http.StatusOK, map[string]any{"notifications": []any{}})
}

func (s *Service) handleGetAuditLog(w http.ResponseWriter, r *http.Request) {
	s.respondJSON(w, http.StatusOK, map[string]any{"audit_entries": []any{}})
}

func (s *Service) respondJSON(w http.ResponseWriter, status int, data any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(data)
}
