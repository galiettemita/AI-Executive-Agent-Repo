package canvas

import (
	"context"
	"encoding/json"
	"net/http"
	"sync"
	"time"

	"github.com/gorilla/websocket"
)

type Interaction struct {
	SessionID string `json:"session_id"`
	Type      string `json:"type"`
	Payload   string `json:"payload"`
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
	upgrader websocket.Upgrader
	injector Injector
	mu       sync.Mutex
	sessions map[*websocket.Conn]string
}

func NewService(injector Injector) *Service {
	return &Service{
		upgrader: websocket.Upgrader{CheckOrigin: func(r *http.Request) bool { return true }},
		injector: injector,
		sessions: map[*websocket.Conn]string{},
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

func (s *Service) HandleHealth(w http.ResponseWriter, _ *http.Request) {
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte("ok"))
}

func NewMux(service *Service) *http.ServeMux {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /v1/canvas/ws", service.HandleWebSocket)
	mux.HandleFunc("GET /healthz/ready", service.HandleHealth)
	mux.HandleFunc("GET /healthz/live", service.HandleHealth)
	return mux
}
