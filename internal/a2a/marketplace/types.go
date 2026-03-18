package marketplace

import (
	"context"
	"time"
)

// AgentRegistration is the runtime record stored for each registered agent.
type AgentRegistration struct {
	AgentID       string       `json:"agent_id"`
	BaseURL       string       `json:"base_url"`
	Capabilities  []Capability `json:"capabilities"`
	AuthSchemes   []string     `json:"auth_schemes"`
	Version       string       `json:"version"`
	LastHeartbeat time.Time    `json:"last_heartbeat"`
	TrustScore    float64      `json:"trust_score"` // 0.0–1.0
	Status        string       `json:"status"`       // "active", "inactive", "suspended"
	RegisteredAt  time.Time    `json:"registered_at"`
}

// Capability describes a single capability an agent can perform.
type Capability struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Description string `json:"description"`
}

// AgentRegistryRepository is the storage interface for registered agents.
type AgentRegistryRepository interface {
	Register(ctx context.Context, reg AgentRegistration) error
	FindByCapability(ctx context.Context, capabilityID string) ([]AgentRegistration, error)
	UpdateHeartbeat(ctx context.Context, agentID string, at time.Time) error
	MarkInactive(ctx context.Context, agentID string) error
	UpdateTrustScore(ctx context.Context, agentID string, score float64) error
	GetAll(ctx context.Context) ([]AgentRegistration, error)
	GetByID(ctx context.Context, agentID string) (*AgentRegistration, error)
}

// AgentOutcomeRepository stores success/failure outcomes for trust scoring.
type AgentOutcomeRepository interface {
	RecordOutcome(ctx context.Context, agentID string, success bool, responseTimeMs int64) error
	GetStats(ctx context.Context, agentID string, days int) (*AgentStats, error)
}

// AgentStats holds aggregated statistics for trust scoring.
type AgentStats struct {
	SuccessRate       float64
	ResponseTimeP99Ms int64
	ErrorRate         float64
	TotalCalls        int
}
