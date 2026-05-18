package router

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
	mux.HandleFunc("POST /api/v1/routing/select", s.handleSelectModel)
	mux.HandleFunc("POST /api/v1/routing/classify", s.handleClassify)

	mux.HandleFunc("GET /api/v1/routing/models", s.handleListModels)
	mux.HandleFunc("POST /api/v1/routing/models", s.handleRegisterModel)
	mux.HandleFunc("PUT /api/v1/routing/models/{id}", s.handleUpdateModel)

	mux.HandleFunc("GET /api/v1/routing/rules", s.handleListRules)
	mux.HandleFunc("POST /api/v1/routing/rules", s.handleCreateRule)
	mux.HandleFunc("PUT /api/v1/routing/rules/{id}", s.handleUpdateRule)
	mux.HandleFunc("DELETE /api/v1/routing/rules/{id}", s.handleDeleteRule)

	mux.HandleFunc("GET /api/v1/routing/decisions", s.handleListDecisions)
	mux.HandleFunc("GET /api/v1/routing/decisions/stats", s.handleDecisionStats)

	mux.HandleFunc("GET /api/v1/routing/preferences", s.handleGetPreferences)
	mux.HandleFunc("PUT /api/v1/routing/preferences", s.handleUpdatePreferences)

	mux.HandleFunc("GET /api/v1/routing/health", s.handleProviderHealth)
	mux.HandleFunc("GET /api/v1/routing/costs", s.handleCostSummary)
}

func (s *Service) handleSelectModel(w http.ResponseWriter, r *http.Request) {
	s.respondJSON(w, http.StatusOK, map[string]any{
		"selected_model": "claude-sonnet-4-20250514",
		"provider":       "anthropic",
		"reason":         "default routing",
	})
}

func (s *Service) handleClassify(w http.ResponseWriter, r *http.Request) {
	s.respondJSON(w, http.StatusOK, map[string]any{
		"complexity": "medium",
		"score":      0.5,
	})
}

func (s *Service) handleListModels(w http.ResponseWriter, r *http.Request) {
	s.respondJSON(w, http.StatusOK, map[string]any{"models": []any{}})
}

func (s *Service) handleRegisterModel(w http.ResponseWriter, r *http.Request) {
	s.respondJSON(w, http.StatusCreated, map[string]any{"status": "registered"})
}

func (s *Service) handleUpdateModel(w http.ResponseWriter, r *http.Request) {
	s.respondJSON(w, http.StatusOK, map[string]any{"status": "updated"})
}

func (s *Service) handleListRules(w http.ResponseWriter, r *http.Request) {
	s.respondJSON(w, http.StatusOK, map[string]any{"rules": []any{}})
}

func (s *Service) handleCreateRule(w http.ResponseWriter, r *http.Request) {
	s.respondJSON(w, http.StatusCreated, map[string]any{"status": "created"})
}

func (s *Service) handleUpdateRule(w http.ResponseWriter, r *http.Request) {
	s.respondJSON(w, http.StatusOK, map[string]any{"status": "updated"})
}

func (s *Service) handleDeleteRule(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusNoContent)
}

func (s *Service) handleListDecisions(w http.ResponseWriter, r *http.Request) {
	s.respondJSON(w, http.StatusOK, map[string]any{"decisions": []any{}})
}

func (s *Service) handleDecisionStats(w http.ResponseWriter, r *http.Request) {
	s.respondJSON(w, http.StatusOK, map[string]any{"stats": map[string]any{}})
}

func (s *Service) handleGetPreferences(w http.ResponseWriter, r *http.Request) {
	s.respondJSON(w, http.StatusOK, map[string]any{"preferences": map[string]any{}})
}

func (s *Service) handleUpdatePreferences(w http.ResponseWriter, r *http.Request) {
	s.respondJSON(w, http.StatusOK, map[string]any{"status": "updated"})
}

func (s *Service) handleProviderHealth(w http.ResponseWriter, r *http.Request) {
	s.respondJSON(w, http.StatusOK, map[string]any{"providers": []any{}})
}

func (s *Service) handleCostSummary(w http.ResponseWriter, r *http.Request) {
	s.respondJSON(w, http.StatusOK, map[string]any{"costs": map[string]any{}})
}

func (s *Service) respondJSON(w http.ResponseWriter, status int, data any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(data)
}
