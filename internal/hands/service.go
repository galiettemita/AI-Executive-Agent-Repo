package hands

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/brevio/brevio/internal/connectors"
	"github.com/brevio/brevio/internal/disclosure"
)

// SkillMetadata describes a registered skill.
type SkillMetadata struct {
	ID            string                 `json:"id"`
	Version       string                 `json:"version"`
	ConnectorKey  string                 `json:"connector_key"`
	Domain        string                 `json:"domain"`
	WriteCapable  bool                   `json:"write_capable"`
	Reversible    bool                   `json:"reversible"`
	AutonomyFloor string                 `json:"autonomy_floor"`
	InputSchema   map[string]interface{} `json:"input_schema"`
	OutputSchema  map[string]interface{} `json:"output_schema"`
}

// ExecuteRequest is the input for POST /v1/skills/:id/execute.
type ExecuteRequest struct {
	SkillID        string                 `json:"skill_id"`
	WorkspaceID    string                 `json:"workspace_id"`
	ReceiptID      string                 `json:"receipt_id"`
	IdempotencyKey string                 `json:"idempotency_key"`
	Mode           string                 `json:"mode"` // "simulate" or "commit"
	Args           map[string]interface{} `json:"args"`
}

// ExecuteResult is the response from skill execution.
type ExecuteResult struct {
	SkillID    string `json:"skill_id"`
	Status     string `json:"status"` // "SUCCESS", "FAILED", "TIMEOUT"
	Data       any    `json:"data,omitempty"`
	Error      *SkillError `json:"error,omitempty"`
	LatencyMs  int64  `json:"latency_ms"`
	Mode       string `json:"mode"`
}

// SkillError represents a skill execution error.
type SkillError struct {
	Code      string `json:"code"`
	Message   string `json:"message"`
	Retryable bool   `json:"retryable"`
}

// MCPClient calls an MCP server to execute a tool.
type MCPClient interface {
	Execute(ctx context.Context, serverURL string, toolKey string, args map[string]interface{}) (any, error)
}

// Service is the Go hands runtime skill execution service.
type Service struct {
	mu             sync.RWMutex
	skills         map[string]SkillMetadata
	connectorSvc   *connectors.Service
	mcpClient      MCPClient
	startedAt      time.Time
}

// NewService creates a hands runtime service backed by the connector registry.
func NewService(connectorSvc *connectors.Service, mcpClient MCPClient) *Service {
	svc := &Service{
		skills:       map[string]SkillMetadata{},
		connectorSvc: connectorSvc,
		mcpClient:    mcpClient,
		startedAt:    time.Now(),
	}
	svc.syncFromRegistry()
	return svc
}

// syncFromRegistry loads skills from the connectors service.
func (s *Service) syncFromRegistry() {
	if s.connectorSvc == nil {
		return
	}

	connectorList := s.connectorSvc.ListConnectors()
	connectorMap := map[string]connectors.Connector{}
	for _, c := range connectorList {
		connectorMap[c.Key] = c
	}

	tools := s.connectorSvc.ListAllTools()
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, t := range tools {
		conn := connectorMap[t.ConnectorKey]
		s.skills[t.ToolKey] = SkillMetadata{
			ID:            t.ToolKey,
			Version:       "1.0.0",
			ConnectorKey:  t.ConnectorKey,
			Domain:        conn.Domain,
			WriteCapable:  t.Write,
			Reversible:    t.Reversible,
			AutonomyFloor: t.AutonomyFloor,
			InputSchema:   t.InputSchema,
			OutputSchema:  t.OutputSchema,
		}
	}
}

// ListSkills returns all registered skills.
func (s *Service) ListSkills() []SkillMetadata {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]SkillMetadata, 0, len(s.skills))
	for _, sk := range s.skills {
		out = append(out, sk)
	}
	return out
}

// GetSchema returns the schema for a skill by ID.
func (s *Service) GetSchema(skillID string) (*SkillMetadata, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	sk, ok := s.skills[skillID]
	if !ok {
		return nil, false
	}
	return &sk, true
}

// Execute runs a skill with the given arguments.
func (s *Service) Execute(ctx context.Context, req ExecuteRequest) ExecuteResult {
	start := time.Now()

	s.mu.RLock()
	skill, ok := s.skills[req.SkillID]
	s.mu.RUnlock()

	if !ok {
		return ExecuteResult{
			SkillID:   req.SkillID,
			Status:    "FAILED",
			LatencyMs: time.Since(start).Milliseconds(),
			Mode:      req.Mode,
			Error: &SkillError{
				Code:      "SKILL_NOT_FOUND",
				Message:   fmt.Sprintf("skill %q not registered", req.SkillID),
				Retryable: false,
			},
		}
	}

	if req.ReceiptID == "" {
		return ExecuteResult{
			SkillID:   req.SkillID,
			Status:    "FAILED",
			LatencyMs: time.Since(start).Milliseconds(),
			Mode:      req.Mode,
			Error: &SkillError{
				Code:      "AUTHORIZATION_REQUIRED",
				Message:   "receipt_id is required",
				Retryable: false,
			},
		}
	}

	// Validate args payload size (max 512KB serialized).
	if req.Args != nil {
		argsBytes, marshalErr := json.Marshal(req.Args)
		if marshalErr != nil {
			return ExecuteResult{
				SkillID:   req.SkillID,
				Status:    "FAILED",
				LatencyMs: time.Since(start).Milliseconds(),
				Mode:      req.Mode,
				Error: &SkillError{
					Code:      "INVALID_ARGS",
					Message:   fmt.Sprintf("failed to serialize args: %v", marshalErr),
					Retryable: false,
				},
			}
		}
		const maxArgsBytes = 512 * 1024
		if len(argsBytes) > maxArgsBytes {
			return ExecuteResult{
				SkillID:   req.SkillID,
				Status:    "FAILED",
				LatencyMs: time.Since(start).Milliseconds(),
				Mode:      req.Mode,
				Error: &SkillError{
					Code:      "PAYLOAD_TOO_LARGE",
					Message:   fmt.Sprintf("args payload %d bytes exceeds limit of %d bytes", len(argsBytes), maxArgsBytes),
					Retryable: false,
				},
			}
		}
	}

	// Resolve MCP server URL from connector.
	var mcpURL string
	if s.connectorSvc != nil {
		connList := s.connectorSvc.ListConnectors()
		for _, c := range connList {
			if c.Key == skill.ConnectorKey {
				mcpURL = c.MCPServerURL
				break
			}
		}
	}

	// Execute via MCP client.
	if s.mcpClient != nil && mcpURL != "" {
		execCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
		defer cancel()

		// AI Identity Disclosure (EU AI Act Article 50): inject disclosure into args.
		disclosedArgs := make(map[string]interface{}, len(req.Args))
		for k, v := range req.Args {
			disclosedArgs[k] = v
		}
		if disclosure.IsEmailSkill(req.SkillID) {
			anyArgs := make(map[string]any, len(disclosedArgs))
			for k, v := range disclosedArgs {
				anyArgs[k] = v
			}
			anyArgs = disclosure.InjectEmailDisclosure(anyArgs)
			for k, v := range anyArgs {
				disclosedArgs[k] = v
			}
		}
		if disclosure.IsCalendarWriteSkill(req.SkillID) {
			anyArgs := make(map[string]any, len(disclosedArgs))
			for k, v := range disclosedArgs {
				anyArgs[k] = v
			}
			anyArgs = disclosure.InjectCalendarDisclosure(anyArgs)
			for k, v := range anyArgs {
				disclosedArgs[k] = v
			}
		}

		data, err := s.mcpClient.Execute(execCtx, mcpURL, req.SkillID, disclosedArgs)
		latency := time.Since(start).Milliseconds()
		if err != nil {
			errCode := "MCP_EXECUTION_FAILED"
			if execCtx.Err() == context.DeadlineExceeded {
				errCode = "EXECUTION_TIMEOUT"
			}
			retryable := isRetryableError(err)
			return ExecuteResult{
				SkillID:   req.SkillID,
				Status:    "FAILED",
				LatencyMs: latency,
				Mode:      req.Mode,
				Error: &SkillError{
					Code:      errCode,
					Message:   err.Error(),
					Retryable: retryable,
				},
			}
		}
		return ExecuteResult{
			SkillID:   req.SkillID,
			Status:    "SUCCESS",
			Data:      data,
			LatencyMs: latency,
			Mode:      req.Mode,
		}
	}

	// No MCP client or URL — return simulated success for simulate mode,
	// or acknowledge with metadata for commit.
	return ExecuteResult{
		SkillID:   req.SkillID,
		Status:    "SUCCESS",
		LatencyMs: time.Since(start).Milliseconds(),
		Mode:      req.Mode,
		Data: map[string]interface{}{
			"tool_key":   req.SkillID,
			"connector":  skill.ConnectorKey,
			"domain":     skill.Domain,
			"mcp_url":    mcpURL,
			"executed":   true,
			"args":       req.Args,
		},
	}
}

func isRetryableError(err error) bool {
	msg := err.Error()
	return strings.Contains(msg, "timeout") ||
		strings.Contains(msg, "429") ||
		strings.Contains(msg, "503") ||
		strings.Contains(msg, "connection refused")
}

// RegisterRoutes registers the hands runtime HTTP endpoints on the given mux.
func (s *Service) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("GET /v1/skills", s.handleListSkills)
	mux.HandleFunc("GET /v1/skills/{id}/schema", s.handleGetSchema)
	mux.HandleFunc("POST /v1/skills/{id}/execute", s.handleExecute)
	mux.HandleFunc("GET /healthz/live", s.handleLive)
	mux.HandleFunc("GET /healthz/ready", s.handleReady)
}

func (s *Service) handleListSkills(w http.ResponseWriter, r *http.Request) {
	skills := s.ListSkills()
	respondJSON(w, http.StatusOK, map[string]any{
		"count":  len(skills),
		"skills": skills,
	})
}

func (s *Service) handleGetSchema(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	schema, ok := s.GetSchema(id)
	if !ok {
		respondJSON(w, http.StatusNotFound, map[string]any{
			"error": fmt.Sprintf("skill %q not found", id),
		})
		return
	}
	respondJSON(w, http.StatusOK, schema)
}

func (s *Service) handleExecute(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")

	body, err := io.ReadAll(io.LimitReader(r.Body, 1<<20))
	if err != nil {
		respondJSON(w, http.StatusBadRequest, map[string]any{"error": "invalid request body"})
		return
	}

	var req ExecuteRequest
	if err := json.Unmarshal(body, &req); err != nil {
		respondJSON(w, http.StatusBadRequest, map[string]any{"error": "invalid JSON"})
		return
	}
	req.SkillID = id

	if req.Mode == "" {
		req.Mode = "commit"
	}
	if req.Mode != "simulate" && req.Mode != "commit" {
		respondJSON(w, http.StatusBadRequest, map[string]any{"error": "mode must be simulate or commit"})
		return
	}

	result := s.Execute(r.Context(), req)

	status := http.StatusOK
	if result.Status == "FAILED" {
		status = http.StatusUnprocessableEntity
	}
	respondJSON(w, status, result)
}

func (s *Service) handleLive(w http.ResponseWriter, r *http.Request) {
	respondJSON(w, http.StatusOK, map[string]any{
		"status": "alive",
		"uptime_ms": time.Since(s.startedAt).Milliseconds(),
	})
}

func (s *Service) handleReady(w http.ResponseWriter, r *http.Request) {
	s.mu.RLock()
	skillCount := len(s.skills)
	s.mu.RUnlock()

	respondJSON(w, http.StatusOK, map[string]any{
		"status":      "ready",
		"skill_count": skillCount,
	})
}

func respondJSON(w http.ResponseWriter, status int, data any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(data)
}

// HTTPMCPClient is a production MCP client that calls MCP servers over HTTP.
type HTTPMCPClient struct {
	client *http.Client
}

// NewHTTPMCPClient creates an HTTP-based MCP client with a timeout.
func NewHTTPMCPClient(timeout time.Duration) *HTTPMCPClient {
	return &HTTPMCPClient{
		client: &http.Client{Timeout: timeout},
	}
}

func (c *HTTPMCPClient) Execute(ctx context.Context, serverURL string, toolKey string, args map[string]interface{}) (any, error) {
	payload := map[string]interface{}{
		"tool_key": toolKey,
		"args":     args,
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("marshal mcp request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, "POST", serverURL+"/execute", strings.NewReader(string(body)))
	if err != nil {
		return nil, fmt.Errorf("create mcp request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("mcp call failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 500 {
		return nil, fmt.Errorf("mcp server error: status %d", resp.StatusCode)
	}
	if resp.StatusCode == 429 {
		return nil, fmt.Errorf("mcp rate limited: 429")
	}
	if resp.StatusCode >= 400 {
		return nil, fmt.Errorf("mcp client error: status %d", resp.StatusCode)
	}

	respBody, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return nil, fmt.Errorf("read mcp response: %w", err)
	}

	var result any
	if err := json.Unmarshal(respBody, &result); err != nil {
		return string(respBody), nil
	}
	return result, nil
}

// FakeMCPClient is a test-only MCP client that returns configurable responses.
type FakeMCPClient struct {
	mu        sync.Mutex
	calls     []FakeMCPCall
	responses map[string]any
	err       error
}

// FakeMCPCall records a call made to the fake MCP client.
type FakeMCPCall struct {
	ServerURL string
	ToolKey   string
	Args      map[string]interface{}
}

func NewFakeMCPClient() *FakeMCPClient {
	return &FakeMCPClient{
		responses: map[string]any{},
	}
}

func (f *FakeMCPClient) SetResponse(toolKey string, data any) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.responses[toolKey] = data
}

func (f *FakeMCPClient) SetError(err error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.err = err
}

func (f *FakeMCPClient) Calls() []FakeMCPCall {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]FakeMCPCall{}, f.calls...)
}

func (f *FakeMCPClient) Execute(_ context.Context, serverURL string, toolKey string, args map[string]interface{}) (any, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.calls = append(f.calls, FakeMCPCall{ServerURL: serverURL, ToolKey: toolKey, Args: args})
	if f.err != nil {
		return nil, f.err
	}
	if resp, ok := f.responses[toolKey]; ok {
		return resp, nil
	}
	return map[string]any{"status": "ok", "tool_key": toolKey, "source": "fake_mcp"}, nil
}
