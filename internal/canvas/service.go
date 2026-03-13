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

	runtimeserver "github.com/brevio/brevio/internal/runtime"
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

// Priority levels for A2UI messages.
const (
	PriorityCritical = "critical"
	PriorityHigh     = "high"
	PriorityNormal   = "normal"
	PriorityLow      = "low"
)

// Supported A2UI message types.
const (
	MsgTypeMissionControlUpdate = "mission_control_update"
	MsgTypeApprovalRequest      = "approval_request"
	MsgTypeNotification         = "notification"
	MsgTypeProgressUpdate       = "progress_update"
	MsgTypeToolResult           = "tool_result"
)

// A2UIMessage represents an agent-to-UI message pushed over WebSocket.
type A2UIMessage struct {
	Type        string         `json:"type"`
	WorkspaceID string         `json:"workspace_id"`
	Payload     map[string]any `json:"payload"`
	Timestamp   time.Time      `json:"timestamp"`
	Priority    string         `json:"priority"`
}

// Widget represents a mission control dashboard widget.
type Widget struct {
	ID        string         `json:"id"`
	Type      string         `json:"type"` // metric, chart, list, status
	Title     string         `json:"title"`
	Data      map[string]any `json:"data"`
	UpdatedAt time.Time      `json:"updated_at"`
}

// ApprovalRequest describes an action requiring operator approval.
type ApprovalRequest struct {
	ID          string    `json:"id"`
	Action      string    `json:"action"`
	RiskLevel   string    `json:"risk_level"`
	Description string    `json:"description"`
	ExpiresAt   time.Time `json:"expires_at"`
}

// ApprovalResponse is the operator's decision on an approval request.
type ApprovalResponse struct {
	Approved    bool   `json:"approved"`
	Reason      string `json:"reason"`
	RespondedBy string `json:"responded_by"`
}

// connEntry tracks a WebSocket connection with its workspace and session.
type connEntry struct {
	sessionID   string
	workspaceID string
}

// surfaceRegistration tracks a registered UI surface.
type surfaceRegistration struct {
	WorkspaceID string `json:"workspace_id"`
	SurfaceType string `json:"surface_type"`
}

type Service struct {
	upgrader  websocket.Upgrader
	injector  Injector
	startedAt time.Time
	wsToken   string // CANVAS_WS_TOKEN — when set, WebSocket upgrades require a matching bearer/query token
	mu        sync.Mutex
	sessions  map[*websocket.Conn]string
	conns     map[*websocket.Conn]connEntry
	logs      []Interaction

	surfaces    map[string][]surfaceRegistration // workspaceID -> surfaces
	approvalsMu sync.Mutex
	approvals   map[string]chan ApprovalResponse // approvalID -> response channel
	messageHandlers map[string]func(sessionID string, payload map[string]any)
}

// NewService creates a canvas service with origin-restricted WebSocket upgrader.
// Allowed origins are loaded from CANVAS_ALLOWED_ORIGINS env var (comma-separated).
// When no allowlist is configured (local/test), all origins are accepted.
func NewService(injector Injector) *Service {
	allowedOrigins := parseAllowedOrigins(os.Getenv("CANVAS_ALLOWED_ORIGINS"))

	wsToken := strings.TrimSpace(os.Getenv("CANVAS_WS_TOKEN"))

	s := &Service{
		upgrader: websocket.Upgrader{CheckOrigin: func(r *http.Request) bool {
			// REPAIR: enforce origin allowlist for production security.
			if len(allowedOrigins) == 0 {
				return true // no allowlist configured — local/test only
			}
			origin := r.Header.Get("Origin")
			if origin == "" {
				return false
			}
			for _, allowed := range allowedOrigins {
				if strings.EqualFold(origin, allowed) {
					return true
				}
			}
			return false
		}},
		injector:        injector,
		wsToken:         wsToken,
		startedAt:       time.Now().UTC(),
		sessions:        map[*websocket.Conn]string{},
		conns:           map[*websocket.Conn]connEntry{},
		logs:            []Interaction{},
		surfaces:        map[string][]surfaceRegistration{},
		approvals:       map[string]chan ApprovalResponse{},
		messageHandlers: map[string]func(sessionID string, payload map[string]any){},
	}
	s.registerDefaultHandlers()
	return s
}

// parseAllowedOrigins splits a comma-separated list of allowed origins.
func parseAllowedOrigins(raw string) []string {
	if raw == "" {
		return nil
	}
	parts := strings.Split(raw, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		trimmed := strings.TrimSpace(p)
		if trimmed != "" {
			out = append(out, trimmed)
		}
	}
	return out
}

func (s *Service) registerDefaultHandlers() {
	s.messageHandlers[MsgTypeMissionControlUpdate] = func(sessionID string, payload map[string]any) {}
	s.messageHandlers[MsgTypeApprovalRequest] = func(sessionID string, payload map[string]any) {
		approvalID, _ := payload["id"].(string)
		if approvalID == "" {
			return
		}
		approved, _ := payload["approved"].(bool)
		reason, _ := payload["reason"].(string)
		respondedBy, _ := payload["responded_by"].(string)
		s.approvalsMu.Lock()
		ch, ok := s.approvals[approvalID]
		s.approvalsMu.Unlock()
		if ok {
			select {
			case ch <- ApprovalResponse{Approved: approved, Reason: reason, RespondedBy: respondedBy}:
			default:
			}
		}
	}
	s.messageHandlers[MsgTypeNotification] = func(sessionID string, payload map[string]any) {}
	s.messageHandlers[MsgTypeProgressUpdate] = func(sessionID string, payload map[string]any) {}
	s.messageHandlers[MsgTypeToolResult] = func(sessionID string, payload map[string]any) {}
}

// validateWSAuth checks the WebSocket upgrade request for a valid auth token.
// It returns true if the request is authorized. When CANVAS_WS_TOKEN is unset
// (empty), all requests are allowed for backward compatibility in local/test
// environments. When set, the token must appear as a Bearer token in the
// Authorization header or as the "token" query parameter.
func (s *Service) validateWSAuth(r *http.Request) bool {
	if s.wsToken == "" {
		return true // no token configured — allow all (local/test)
	}

	// Check Authorization header: "Bearer <token>"
	if auth := r.Header.Get("Authorization"); auth != "" {
		const prefix = "Bearer "
		if strings.HasPrefix(auth, prefix) {
			if strings.TrimSpace(auth[len(prefix):]) == s.wsToken {
				return true
			}
		}
	}

	// Check "token" query parameter
	if r.URL.Query().Get("token") == s.wsToken {
		return true
	}

	return false
}

func (s *Service) HandleWebSocket(w http.ResponseWriter, r *http.Request) {
	if !s.validateWSAuth(r) {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

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

	workspaceID := r.URL.Query().Get("workspace_id")
	if workspaceID == "" {
		workspaceID = r.Header.Get("X-Workspace-ID")
	}
	if workspaceID == "" {
		workspaceID = "default"
	}

	s.mu.Lock()
	s.sessions[conn] = sessionID
	s.conns[conn] = connEntry{sessionID: sessionID, workspaceID: workspaceID}
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
	delete(s.conns, conn)
	s.mu.Unlock()
}

// BroadcastToWorkspace sends an A2UIMessage to all WebSocket connections for a workspace.
func (s *Service) BroadcastToWorkspace(workspaceID string, msg A2UIMessage) {
	if msg.Timestamp.IsZero() {
		msg.Timestamp = time.Now().UTC()
	}
	if msg.Priority == "" {
		msg.Priority = PriorityNormal
	}
	msg.WorkspaceID = workspaceID

	data, err := json.Marshal(msg)
	if err != nil {
		return
	}

	s.mu.Lock()
	targets := make([]*websocket.Conn, 0)
	for conn, entry := range s.conns {
		if entry.workspaceID == workspaceID {
			targets = append(targets, conn)
		}
	}
	s.mu.Unlock()

	for _, conn := range targets {
		_ = conn.WriteMessage(websocket.TextMessage, data)
	}
}

// HandleIncoming parses a raw incoming WebSocket message and routes it to the
// appropriate message handler based on the message type.
func (s *Service) HandleIncoming(sessionID string, raw []byte) {
	var envelope struct {
		Type    string         `json:"type"`
		Payload map[string]any `json:"payload"`
	}
	if err := json.Unmarshal(raw, &envelope); err != nil {
		return
	}
	if envelope.Type == "" {
		return
	}
	if envelope.Payload == nil {
		envelope.Payload = map[string]any{}
	}

	s.mu.Lock()
	handler, ok := s.messageHandlers[envelope.Type]
	s.mu.Unlock()
	if ok {
		handler(sessionID, envelope.Payload)
	}
}

// RegisterSurface registers a UI surface type for a workspace.
func (s *Service) RegisterSurface(workspaceID, surfaceType string) {
	if workspaceID == "" {
		workspaceID = "default"
	}
	s.mu.Lock()
	defer s.mu.Unlock()

	// Prevent duplicates.
	for _, reg := range s.surfaces[workspaceID] {
		if reg.SurfaceType == surfaceType {
			return
		}
	}
	s.surfaces[workspaceID] = append(s.surfaces[workspaceID], surfaceRegistration{
		WorkspaceID: workspaceID,
		SurfaceType: surfaceType,
	})
}

// RegisteredSurfaces returns the surface types registered for a workspace.
func (s *Service) RegisteredSurfaces(workspaceID string) []string {
	s.mu.Lock()
	defer s.mu.Unlock()
	regs := s.surfaces[workspaceID]
	out := make([]string, len(regs))
	for i, r := range regs {
		out[i] = r.SurfaceType
	}
	return out
}

// PushMissionControlUpdate pushes dashboard widget data to all workspace connections.
func (s *Service) PushMissionControlUpdate(workspaceID string, widgets []Widget) {
	widgetMaps := make([]map[string]any, len(widgets))
	for i, w := range widgets {
		widgetMaps[i] = map[string]any{
			"id":         w.ID,
			"type":       w.Type,
			"title":      w.Title,
			"data":       w.Data,
			"updated_at": w.UpdatedAt,
		}
	}
	s.BroadcastToWorkspace(workspaceID, A2UIMessage{
		Type:     MsgTypeMissionControlUpdate,
		Payload:  map[string]any{"widgets": widgetMaps},
		Priority: PriorityNormal,
	})
}

// RequestApproval sends an approval request to the workspace UI and returns a
// channel that will receive the operator's response. The caller is responsible
// for reading the response or handling a timeout.
func (s *Service) RequestApproval(workspaceID string, req ApprovalRequest) chan ApprovalResponse {
	ch := make(chan ApprovalResponse, 1)

	s.approvalsMu.Lock()
	s.approvals[req.ID] = ch
	s.approvalsMu.Unlock()

	s.BroadcastToWorkspace(workspaceID, A2UIMessage{
		Type: MsgTypeApprovalRequest,
		Payload: map[string]any{
			"id":          req.ID,
			"action":      req.Action,
			"risk_level":  req.RiskLevel,
			"description": req.Description,
			"expires_at":  req.ExpiresAt,
		},
		Priority: PriorityHigh,
	})

	return ch
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
			for key, status := range runtimeserver.DeepDependencyChecks(os.Getenv) {
				checks[key] = status
			}
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

func writeJSON(w http.ResponseWriter, status int, payload any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(payload)
}
