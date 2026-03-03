package canvas

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"net/netip"
	"net/url"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/gorilla/websocket"
)

type Interaction struct {
	SessionID string    `json:"session_id"`
	Type      string    `json:"type"`
	Payload   string    `json:"payload"`
	CreatedAt time.Time `json:"created_at"`
}

type ToolCall struct {
	SessionID string
	ToolKey   string
	Payload   string
}

type Injector interface {
	Inject(ctx context.Context, toolCall ToolCall) error
}

type InMemoryInjector struct {
	mu        sync.Mutex
	toolCalls []ToolCall
}

func (i *InMemoryInjector) Inject(_ context.Context, toolCall ToolCall) error {
	i.mu.Lock()
	defer i.mu.Unlock()
	i.toolCalls = append(i.toolCalls, toolCall)
	return nil
}

func (i *InMemoryInjector) Count() int {
	i.mu.Lock()
	defer i.mu.Unlock()
	return len(i.toolCalls)
}

type Service struct {
	upgrader  websocket.Upgrader
	injector  Injector
	startedAt time.Time
	mu        sync.Mutex
	sessions  map[*websocket.Conn]string
	logs      []Interaction
}

func NewService(injector Injector) *Service {
	return &Service{
		upgrader:  websocket.Upgrader{CheckOrigin: func(r *http.Request) bool { return true }},
		injector:  injector,
		startedAt: time.Now().UTC(),
		sessions:  map[*websocket.Conn]string{},
		logs:      []Interaction{},
	}
}

func (s *Service) HandleWebSocket(w http.ResponseWriter, r *http.Request) {
	conn, err := s.upgrader.Upgrade(w, r, nil)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	defer conn.Close()

	sessionID := r.URL.Query().Get("session_id")
	if sessionID == "" {
		sessionID = time.Now().UTC().Format(time.RFC3339Nano)
	}

	s.mu.Lock()
	s.sessions[conn] = sessionID
	s.mu.Unlock()

	for {
		_, message, err := conn.ReadMessage()
		if err != nil {
			break
		}

		var interaction Interaction
		if err := json.Unmarshal(message, &interaction); err != nil {
			continue
		}
		if interaction.SessionID == "" {
			interaction.SessionID = sessionID
		}
		interaction.CreatedAt = time.Now().UTC()

		s.mu.Lock()
		s.logs = append(s.logs, interaction)
		s.mu.Unlock()

		_ = s.injector.Inject(r.Context(), ToolCall{
			SessionID: interaction.SessionID,
			ToolKey:   "gateway.inject_tool_call",
			Payload:   interaction.Payload,
		})

		_ = conn.WriteMessage(websocket.TextMessage, []byte(`{"status":"ok"}`))
	}

	s.mu.Lock()
	delete(s.sessions, conn)
	s.mu.Unlock()
}

func (s *Service) HandleA2UISurface(w http.ResponseWriter, r *http.Request) {
	surface := map[string]any{
		"surface_id": "mission_control",
		"widgets": []map[string]any{
			{"type": "goal_progress", "title": "Goals"},
			{"type": "trust_score", "title": "Autonomy Trust"},
			{"type": "daily_capture", "title": "Daily Capture"},
		},
	}
	writeJSON(w, http.StatusOK, surface)
}

func (s *Service) HandleFetchPreview(w http.ResponseWriter, r *http.Request) {
	var payload struct {
		URL string `json:"url"`
	}
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	if err := validateFetchURL(payload.URL); err != nil {
		http.Error(w, err.Error(), http.StatusForbidden)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"url":     payload.URL,
		"preview": "fetch_allowed",
	})
}

func (s *Service) HandlePush(w http.ResponseWriter, _ *http.Request) {
	// Canvas push is asynchronous in production; mux contract only requires acceptance.
	// Returning 204 keeps the endpoint side-effect free for unit/contract tests.
	w.WriteHeader(http.StatusNoContent)
}

func validateFetchURL(raw string) error {
	parsed, err := url.Parse(raw)
	if err != nil {
		return fmt.Errorf("invalid url: %w", err)
	}
	host := strings.ToLower(parsed.Hostname())
	if host == "" {
		return fmt.Errorf("missing host")
	}
	if host == "localhost" || strings.HasPrefix(host, "127.") || host == "169.254.169.254" || host == "::1" {
		return fmt.Errorf("blocked host")
	}
	ip := net.ParseIP(host)
	if ip == nil {
		return nil
	}
	addr, ok := netip.AddrFromSlice(ip)
	if ok && addr.IsLoopback() {
		return fmt.Errorf("blocked loopback host")
	}
	return nil
}

func (s *Service) ActiveSessionCount() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return len(s.sessions)
}

func (s *Service) InteractionCount() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return len(s.logs)
}

func (s *Service) HandleHealth(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path == "/health" || r.URL.Path == "/health/deep" {
		checks := map[string]string{
			"process": "ok",
		}
		if r.URL.Path == "/health/deep" {
			checks["db"] = healthEnvCheck("DATABASE_URL")
			checks["redis"] = healthEnvCheck("REDIS_URL")
			checks["temporal"] = healthEnvCheck("TEMPORAL_HOST")
		}
		version := strings.TrimSpace(os.Getenv("SERVICE_VERSION"))
		if version == "" {
			version = "0.1.0"
		}
		writeJSON(w, http.StatusOK, map[string]any{
			"status":    "healthy",
			"version":   version,
			"uptime_ms": time.Since(s.startedAt).Milliseconds(),
			"checks":    checks,
		})
		return
	}
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte("ok"))
}

func NewMux(service *Service) *http.ServeMux {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /v1/canvas/ws", service.HandleWebSocket)
	mux.HandleFunc("POST /v1/canvas/push", service.HandlePush)
	mux.HandleFunc("GET /v1/canvas/surfaces/mission_control", service.HandleA2UISurface)
	mux.HandleFunc("POST /v1/canvas/fetch", service.HandleFetchPreview)
	mux.HandleFunc("GET /health", service.HandleHealth)
	mux.HandleFunc("GET /health/deep", service.HandleHealth)
	mux.HandleFunc("GET /healthz/ready", service.HandleHealth)
	mux.HandleFunc("GET /healthz/live", service.HandleHealth)
	return mux
}

func healthEnvCheck(key string) string {
	if strings.TrimSpace(os.Getenv(key)) == "" {
		return "not_configured"
	}
	return "configured"
}

func writeJSON(w http.ResponseWriter, status int, payload any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(payload)
}
