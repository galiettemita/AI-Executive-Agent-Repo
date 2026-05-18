package browser

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
	mux.HandleFunc("POST /api/v1/browser/sessions", s.handleCreateSession)
	mux.HandleFunc("GET /api/v1/browser/sessions/{id}", s.handleGetSession)
	mux.HandleFunc("DELETE /api/v1/browser/sessions/{id}", s.handleDeleteSession)

	mux.HandleFunc("POST /api/v1/browser/sessions/{sessionId}/tasks", s.handleCreateTask)
	mux.HandleFunc("GET /api/v1/browser/sessions/{sessionId}/tasks", s.handleListTasks)
	mux.HandleFunc("GET /api/v1/browser/sessions/{sessionId}/tasks/{taskId}", s.handleGetTask)

	mux.HandleFunc("POST /api/v1/browser/scrape", s.handleScrape)
	mux.HandleFunc("POST /api/v1/browser/screenshot", s.handleScreenshot)
	mux.HandleFunc("POST /api/v1/browser/form-fill", s.handleFormFill)

	mux.HandleFunc("GET /api/v1/browser/fingerprints", s.handleListFingerprints)
	mux.HandleFunc("POST /api/v1/browser/fingerprints", s.handleCreateFingerprint)
	mux.HandleFunc("DELETE /api/v1/browser/fingerprints/{id}", s.handleDeleteFingerprint)

	mux.HandleFunc("GET /api/v1/browser/proxies", s.handleListProxies)
	mux.HandleFunc("POST /api/v1/browser/proxies", s.handleCreateProxy)
	mux.HandleFunc("DELETE /api/v1/browser/proxies/{id}", s.handleDeleteProxy)

	mux.HandleFunc("POST /api/v1/browser/captcha/solve", s.handleSolveCaptcha)
	mux.HandleFunc("GET /api/v1/browser/cookies/{domain}", s.handleGetCookies)
	mux.HandleFunc("POST /api/v1/browser/cookies", s.handleStoreCookies)
}

func (s *Service) handleCreateSession(w http.ResponseWriter, r *http.Request) {
	s.respondJSON(w, http.StatusCreated, map[string]any{"status": "created", "message": "browser session created"})
}

func (s *Service) handleGetSession(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	s.respondJSON(w, http.StatusOK, map[string]any{"id": id, "status": "pending"})
}

func (s *Service) handleDeleteSession(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusNoContent)
}

func (s *Service) handleCreateTask(w http.ResponseWriter, r *http.Request) {
	s.respondJSON(w, http.StatusCreated, map[string]any{"status": "created"})
}

func (s *Service) handleListTasks(w http.ResponseWriter, r *http.Request) {
	s.respondJSON(w, http.StatusOK, map[string]any{"tasks": []any{}})
}

func (s *Service) handleGetTask(w http.ResponseWriter, r *http.Request) {
	taskId := r.PathValue("taskId")
	s.respondJSON(w, http.StatusOK, map[string]any{"id": taskId, "status": "pending"})
}

func (s *Service) handleScrape(w http.ResponseWriter, r *http.Request) {
	s.respondJSON(w, http.StatusAccepted, map[string]any{"status": "accepted"})
}

func (s *Service) handleScreenshot(w http.ResponseWriter, r *http.Request) {
	s.respondJSON(w, http.StatusAccepted, map[string]any{"status": "accepted"})
}

func (s *Service) handleFormFill(w http.ResponseWriter, r *http.Request) {
	s.respondJSON(w, http.StatusAccepted, map[string]any{"status": "accepted"})
}

func (s *Service) handleListFingerprints(w http.ResponseWriter, r *http.Request) {
	s.respondJSON(w, http.StatusOK, map[string]any{"fingerprints": []any{}})
}

func (s *Service) handleCreateFingerprint(w http.ResponseWriter, r *http.Request) {
	s.respondJSON(w, http.StatusCreated, map[string]any{"status": "created"})
}

func (s *Service) handleDeleteFingerprint(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusNoContent)
}

func (s *Service) handleListProxies(w http.ResponseWriter, r *http.Request) {
	s.respondJSON(w, http.StatusOK, map[string]any{"proxies": []any{}})
}

func (s *Service) handleCreateProxy(w http.ResponseWriter, r *http.Request) {
	s.respondJSON(w, http.StatusCreated, map[string]any{"status": "created"})
}

func (s *Service) handleDeleteProxy(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusNoContent)
}

func (s *Service) handleSolveCaptcha(w http.ResponseWriter, r *http.Request) {
	s.respondJSON(w, http.StatusAccepted, map[string]any{"status": "accepted"})
}

func (s *Service) handleGetCookies(w http.ResponseWriter, r *http.Request) {
	s.respondJSON(w, http.StatusOK, map[string]any{"cookies": []any{}})
}

func (s *Service) handleStoreCookies(w http.ResponseWriter, r *http.Request) {
	s.respondJSON(w, http.StatusCreated, map[string]any{"status": "stored"})
}

func (s *Service) respondJSON(w http.ResponseWriter, status int, data any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(data)
}
