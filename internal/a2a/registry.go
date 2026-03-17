package a2a

import (
	"context"
	"fmt"
	"sync"

	"github.com/jackc/pgx/v5/pgxpool"
)

// ExternalAgent is a known external A2A agent that Brevio can delegate to.
type ExternalAgent struct {
	ID           string   `json:"id"`
	Name         string   `json:"name"`
	Description  string   `json:"description"`
	BaseURL      string   `json:"base_url"`
	M2MToken     string   `json:"-"` // never serialized
	Capabilities []string `json:"capabilities"`
	IsActive     bool     `json:"is_active"`
}

// ExternalAgentRegistry stores and retrieves trusted external A2A agents.
type ExternalAgentRegistry struct {
	pool *pgxpool.Pool
	mu   sync.Mutex
	mem  []ExternalAgent // in-memory fallback when pool is nil
}

func NewExternalAgentRegistry(pool *pgxpool.Pool) *ExternalAgentRegistry {
	return &ExternalAgentRegistry{pool: pool}
}

// FindByCapability returns the first active external agent that supports a capability.
func (r *ExternalAgentRegistry) FindByCapability(ctx context.Context, capability string) (*ExternalAgent, error) {
	if r.pool == nil {
		r.mu.Lock()
		defer r.mu.Unlock()
		for _, a := range r.mem {
			if !a.IsActive {
				continue
			}
			for _, c := range a.Capabilities {
				if c == capability {
					copy := a
					return &copy, nil
				}
			}
		}
		return nil, fmt.Errorf("external_agent_registry: capability %q not found", capability)
	}

	var agent ExternalAgent
	err := r.pool.QueryRow(ctx, `
		SELECT id, name, description, base_url, m2m_token, capabilities
		FROM external_agents
		WHERE is_active = true AND $1 = ANY(capabilities)
		ORDER BY name LIMIT 1
	`, capability).Scan(&agent.ID, &agent.Name, &agent.Description, &agent.BaseURL, &agent.M2MToken, &agent.Capabilities)
	if err != nil {
		return nil, fmt.Errorf("external_agent_registry: capability %q not found", capability)
	}
	agent.IsActive = true
	return &agent, nil
}

// List returns all active external agents.
func (r *ExternalAgentRegistry) List(ctx context.Context) ([]ExternalAgent, error) {
	if r.pool == nil {
		r.mu.Lock()
		defer r.mu.Unlock()
		var active []ExternalAgent
		for _, a := range r.mem {
			if a.IsActive {
				active = append(active, a)
			}
		}
		return active, nil
	}

	rows, err := r.pool.Query(ctx, `
		SELECT id, name, description, base_url, capabilities
		FROM external_agents WHERE is_active = true ORDER BY name
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var agents []ExternalAgent
	for rows.Next() {
		var a ExternalAgent
		a.IsActive = true
		if err := rows.Scan(&a.ID, &a.Name, &a.Description, &a.BaseURL, &a.Capabilities); err != nil {
			continue
		}
		agents = append(agents, a)
	}
	return agents, rows.Err()
}

// Register adds or updates an external agent record.
func (r *ExternalAgentRegistry) Register(ctx context.Context, agent ExternalAgent) error {
	if r.pool == nil {
		r.mu.Lock()
		defer r.mu.Unlock()
		for i, a := range r.mem {
			if a.Name == agent.Name {
				r.mem[i] = agent
				return nil
			}
		}
		r.mem = append(r.mem, agent)
		return nil
	}

	_, err := r.pool.Exec(ctx, `
		INSERT INTO external_agents (name, description, base_url, m2m_token, capabilities)
		VALUES ($1,$2,$3,$4,$5)
		ON CONFLICT (name) DO UPDATE
			SET description=EXCLUDED.description, base_url=EXCLUDED.base_url,
			    m2m_token=EXCLUDED.m2m_token, capabilities=EXCLUDED.capabilities, updated_at=NOW()
	`, agent.Name, agent.Description, agent.BaseURL, agent.M2MToken, agent.Capabilities)
	return err
}
