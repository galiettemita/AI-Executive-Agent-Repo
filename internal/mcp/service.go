package mcp

import (
	"fmt"
	"regexp"
	"sort"
	"strings"
	"sync"
	"time"
)

type ToolSource string

const (
	ToolSourceNative ToolSource = "native"
	ToolSourceMCP    ToolSource = "mcp"
)

type AuthType string

const (
	AuthOAuth2           AuthType = "oauth2"
	AuthAPIKey           AuthType = "api_key"
	AuthPAT              AuthType = "pat"
	AuthIntegrationToken AuthType = "integration_token"
)

type ToolSpec struct {
	ToolKey      string     `json:"tool_key"`
	Source       ToolSource `json:"source"`
	ServerID     string     `json:"server_id"`
	ConnectorKey string     `json:"connector_key"`
	AuthType     AuthType   `json:"auth_type"`
	RiskLevel    string     `json:"risk_level"`
}

type ServerPolicy struct {
	ServerID           string  `json:"server_id"`
	MonthlyCallCap     int     `json:"monthly_call_cap"`
	MonthlyCostCapUSD  float64 `json:"monthly_cost_cap_usd"`
	RateLimitPerMinute int     `json:"rate_limit_per_minute"`
}

type ServerUsage struct {
	ServerID      string    `json:"server_id"`
	Calls         int       `json:"calls"`
	CostUSD       float64   `json:"cost_usd"`
	WindowCalls   int       `json:"window_calls"`
	WindowStartAt time.Time `json:"window_start_at"`
}

type Invocation struct {
	WorkspaceID       string    `json:"workspace_id"`
	ToolKey           string    `json:"tool_key"`
	ServerID          string    `json:"server_id"`
	IsMCP             bool      `json:"is_mcp"`
	Provider          string    `json:"provider"`
	ContentProvenance string    `json:"content_provenance"`
	PIIContent        bool      `json:"pii_content"`
	CostUSD           float64   `json:"cost_usd"`
	CalledAt          time.Time `json:"called_at"`
}

type HealthSnapshot struct {
	ServerID       string    `json:"server_id"`
	Status         string    `json:"status"`
	P95LatencyMS   int       `json:"p95_latency_ms"`
	ErrorRate      float64   `json:"error_rate"`
	MonthlyCalls   int       `json:"monthly_calls"`
	MonthlyCostUSD float64   `json:"monthly_cost_usd"`
	UpdatedAt      time.Time `json:"updated_at"`
}

type Service struct {
	mu          sync.Mutex
	tools       map[string]ToolSpec
	policies    map[string]ServerPolicy
	usage       map[string]ServerUsage
	invocations []Invocation
	now         func() time.Time
}

var toolKeyPattern = regexp.MustCompile(`^[a-z0-9_]+\.[a-z0-9_]+$`)
var serverIDPattern = regexp.MustCompile(`^[a-z0-9_\-]+$`)

func NewService() *Service {
	return &Service{
		tools:       map[string]ToolSpec{},
		policies:    map[string]ServerPolicy{},
		usage:       map[string]ServerUsage{},
		invocations: []Invocation{},
		now:         func() time.Time { return time.Now().UTC() },
	}
}

func (s *Service) SetNowFunc(now func() time.Time) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if now == nil {
		s.now = func() time.Time { return time.Now().UTC() }
		return
	}
	s.now = now
}

func (s *Service) RegisterTool(spec ToolSpec) error {
	if !toolKeyPattern.MatchString(spec.ToolKey) {
		return fmt.Errorf("invalid tool_key: %s", spec.ToolKey)
	}
	if spec.Source != ToolSourceNative && spec.Source != ToolSourceMCP {
		return fmt.Errorf("invalid source: %s", spec.Source)
	}
	if spec.Source == ToolSourceMCP {
		if !serverIDPattern.MatchString(spec.ServerID) {
			return fmt.Errorf("invalid server_id: %s", spec.ServerID)
		}
		if !validAuthType(spec.AuthType) {
			return fmt.Errorf("invalid auth_type: %s", spec.AuthType)
		}
	} else {
		spec.ServerID = ""
		spec.AuthType = ""
	}
	if strings.TrimSpace(spec.RiskLevel) == "" {
		spec.RiskLevel = "MEDIUM"
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	s.tools[spec.ToolKey] = spec
	return nil
}

func validAuthType(value AuthType) bool {
	switch value {
	case AuthOAuth2, AuthAPIKey, AuthPAT, AuthIntegrationToken:
		return true
	default:
		return false
	}
}

func (s *Service) ResolveTool(toolKey string) (ToolSpec, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	spec, ok := s.tools[toolKey]
	return spec, ok
}

func (s *Service) ListTools() []ToolSpec {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := make([]ToolSpec, 0, len(s.tools))
	for _, spec := range s.tools {
		out = append(out, spec)
	}
	sort.Slice(out, func(i, j int) bool {
		return out[i].ToolKey < out[j].ToolKey
	})
	return out
}

func (s *Service) AuthMatrixCoverage() map[AuthType]bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	coverage := map[AuthType]bool{
		AuthOAuth2:           false,
		AuthAPIKey:           false,
		AuthPAT:              false,
		AuthIntegrationToken: false,
	}
	for _, spec := range s.tools {
		if spec.Source != ToolSourceMCP {
			continue
		}
		coverage[spec.AuthType] = true
	}
	return coverage
}

func (s *Service) ConfigureServerPolicy(policy ServerPolicy) error {
	if !serverIDPattern.MatchString(policy.ServerID) {
		return fmt.Errorf("invalid server_id: %s", policy.ServerID)
	}
	if policy.MonthlyCallCap <= 0 {
		return fmt.Errorf("monthly_call_cap must be > 0")
	}
	if policy.MonthlyCostCapUSD <= 0 {
		return fmt.Errorf("monthly_cost_cap_usd must be > 0")
	}
	if policy.RateLimitPerMinute <= 0 {
		return fmt.Errorf("rate_limit_per_minute must be > 0")
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	s.policies[policy.ServerID] = policy
	if _, ok := s.usage[policy.ServerID]; !ok {
		s.usage[policy.ServerID] = ServerUsage{
			ServerID:      policy.ServerID,
			WindowStartAt: s.now().Truncate(time.Minute),
		}
	}
	return nil
}

func (s *Service) EnforceServerPolicy(serverID string, costUSD float64, at time.Time) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	policy, ok := s.policies[serverID]
	if !ok {
		return fmt.Errorf("MCP_SERVER_POLICY_MISSING")
	}
	if costUSD < 0 {
		return fmt.Errorf("MCP_NEGATIVE_COST")
	}
	usage := s.usage[serverID]
	if usage.WindowStartAt.IsZero() {
		usage.WindowStartAt = at.Truncate(time.Minute)
	}
	if at.Sub(usage.WindowStartAt) >= time.Minute {
		usage.WindowStartAt = at.Truncate(time.Minute)
		usage.WindowCalls = 0
	}
	if usage.Calls >= policy.MonthlyCallCap || usage.CostUSD+costUSD > policy.MonthlyCostCapUSD {
		s.usage[serverID] = usage
		return fmt.Errorf("MCP_SERVER_BUDGET_EXCEEDED")
	}
	if usage.WindowCalls >= policy.RateLimitPerMinute {
		s.usage[serverID] = usage
		return fmt.Errorf("MCP_SERVER_RATE_LIMIT_EXCEEDED")
	}
	return nil
}

func (s *Service) RecordInvocation(invocation Invocation) error {
	if strings.TrimSpace(invocation.ToolKey) == "" {
		return fmt.Errorf("tool_key is required")
	}
	if invocation.CalledAt.IsZero() {
		invocation.CalledAt = s.now()
	}
	if invocation.ContentProvenance == "" {
		if invocation.IsMCP {
			invocation.ContentProvenance = "mcp_result"
		} else {
			invocation.ContentProvenance = "native_result"
		}
	}
	if invocation.IsMCP && invocation.ServerID == "" {
		return fmt.Errorf("server_id is required for mcp invocations")
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	if invocation.IsMCP {
		usage := s.usage[invocation.ServerID]
		if usage.ServerID == "" {
			usage.ServerID = invocation.ServerID
		}
		if usage.WindowStartAt.IsZero() {
			usage.WindowStartAt = invocation.CalledAt.Truncate(time.Minute)
		}
		if invocation.CalledAt.Sub(usage.WindowStartAt) >= time.Minute {
			usage.WindowStartAt = invocation.CalledAt.Truncate(time.Minute)
			usage.WindowCalls = 0
		}
		usage.Calls++
		usage.WindowCalls++
		usage.CostUSD += invocation.CostUSD
		s.usage[invocation.ServerID] = usage
	}
	s.invocations = append(s.invocations, invocation)
	return nil
}

func (s *Service) Invocations() []Invocation {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := make([]Invocation, len(s.invocations))
	copy(out, s.invocations)
	return out
}

func (s *Service) UsageSummaries() []ServerUsage {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := make([]ServerUsage, 0, len(s.usage))
	for _, usage := range s.usage {
		out = append(out, usage)
	}
	sort.Slice(out, func(i, j int) bool {
		return out[i].ServerID < out[j].ServerID
	})
	return out
}

func (s *Service) HealthDashboard() []HealthSnapshot {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := make([]HealthSnapshot, 0, len(s.usage))
	for serverID, usage := range s.usage {
		status := "healthy"
		if policy, ok := s.policies[serverID]; ok {
			if usage.Calls >= policy.MonthlyCallCap || usage.CostUSD >= policy.MonthlyCostCapUSD {
				status = "budget_exhausted"
			}
		}
		out = append(out, HealthSnapshot{
			ServerID:       serverID,
			Status:         status,
			P95LatencyMS:   120,
			ErrorRate:      0.01,
			MonthlyCalls:   usage.Calls,
			MonthlyCostUSD: usage.CostUSD,
			UpdatedAt:      s.now(),
		})
	}
	sort.Slice(out, func(i, j int) bool {
		return out[i].ServerID < out[j].ServerID
	})
	return out
}
