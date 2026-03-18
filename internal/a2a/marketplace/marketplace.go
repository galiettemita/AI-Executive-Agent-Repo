package marketplace

import (
	"context"
	"errors"
	"fmt"
	"log"
	"sort"
	"time"
)

var (
	// ErrAgentNotFound is returned when a requested agent does not exist.
	ErrAgentNotFound = errors.New("marketplace: agent not found")
	// ErrInvalidAgentCard is returned when registration data is incomplete.
	ErrInvalidAgentCard = errors.New("marketplace: invalid agent card")
	// ErrUnauthorized is returned when M2M token validation fails.
	ErrUnauthorized = errors.New("marketplace: unauthorized")
)

// M2MAuthValidator validates machine-to-machine tokens on agent registration.
type M2MAuthValidator interface {
	Validate(ctx context.Context, token string) error
}

// AgentMarketplace manages the lifecycle of external agent registrations.
type AgentMarketplace struct {
	repo         AgentRegistryRepository
	outcomes     AgentOutcomeRepository
	scorer       *AgentTrustScorer
	m2mValidator M2MAuthValidator
}

// NewAgentMarketplace creates a new marketplace.
func NewAgentMarketplace(
	repo AgentRegistryRepository,
	outcomes AgentOutcomeRepository,
	scorer *AgentTrustScorer,
	validator M2MAuthValidator,
) *AgentMarketplace {
	return &AgentMarketplace{
		repo:         repo,
		outcomes:     outcomes,
		scorer:       scorer,
		m2mValidator: validator,
	}
}

// Register validates the M2M token and registers an agent.
func (m *AgentMarketplace) Register(ctx context.Context, agentID, baseURL, version string,
	capabilities []Capability, authSchemes []string, m2mToken string) error {

	if m.m2mValidator != nil {
		if err := m.m2mValidator.Validate(ctx, m2mToken); err != nil {
			return ErrUnauthorized
		}
	}
	if agentID == "" || baseURL == "" {
		return ErrInvalidAgentCard
	}
	if len(capabilities) == 0 {
		return ErrInvalidAgentCard
	}

	reg := AgentRegistration{
		AgentID:       agentID,
		BaseURL:       baseURL,
		Capabilities:  capabilities,
		AuthSchemes:   authSchemes,
		Version:       version,
		LastHeartbeat: time.Now(),
		TrustScore:    0.5,
		Status:        "active",
		RegisteredAt:  time.Now(),
	}
	return m.repo.Register(ctx, reg)
}

// Discover returns active agents matching the given capability, sorted by trust score.
func (m *AgentMarketplace) Discover(ctx context.Context, capabilityID string) ([]AgentRegistration, error) {
	agents, err := m.repo.FindByCapability(ctx, capabilityID)
	if err != nil {
		return nil, err
	}

	// Filter active only.
	active := make([]AgentRegistration, 0, len(agents))
	for _, a := range agents {
		if a.Status == "active" {
			active = append(active, a)
		}
	}

	// Sort by TrustScore DESC, then LastHeartbeat DESC.
	sort.Slice(active, func(i, j int) bool {
		if active[i].TrustScore != active[j].TrustScore {
			return active[i].TrustScore > active[j].TrustScore
		}
		return active[i].LastHeartbeat.After(active[j].LastHeartbeat)
	})

	return active, nil
}

// RecordOutcome records a delegation result and recomputes trust.
func (m *AgentMarketplace) RecordOutcome(ctx context.Context, agentID string, success bool, responseTimeMs int64) error {
	if err := m.outcomes.RecordOutcome(ctx, agentID, success, responseTimeMs); err != nil {
		return fmt.Errorf("record outcome: %w", err)
	}
	score, _ := m.scorer.Compute(ctx, agentID, 0.8)
	return m.repo.UpdateTrustScore(ctx, agentID, score)
}

// RecordHeartbeat updates the last heartbeat timestamp for an agent.
func (m *AgentMarketplace) RecordHeartbeat(ctx context.Context, agentID string, at time.Time) error {
	return m.repo.UpdateHeartbeat(ctx, agentID, at)
}

// RefreshTrustScores recomputes trust scores for all registered agents.
func (m *AgentMarketplace) RefreshTrustScores(ctx context.Context) error {
	agents, err := m.repo.GetAll(ctx)
	if err != nil {
		return err
	}
	for _, agent := range agents {
		score, err := m.scorer.Compute(ctx, agent.AgentID, 0.8)
		if err != nil {
			log.Printf("[marketplace] trust score compute failed for %s: %v", agent.AgentID, err)
			continue
		}
		if err := m.repo.UpdateTrustScore(ctx, agent.AgentID, score); err != nil {
			log.Printf("[marketplace] trust score update failed for %s: %v", agent.AgentID, err)
		}
	}
	return nil
}

// MarkInactiveStaleAgents marks agents with stale heartbeats as inactive.
func (m *AgentMarketplace) MarkInactiveStaleAgents(ctx context.Context, cutoff time.Duration) error {
	if cutoff == 0 {
		cutoff = 15 * time.Minute
	}
	agents, err := m.repo.GetAll(ctx)
	if err != nil {
		return err
	}
	deadline := time.Now().Add(-cutoff)
	for _, agent := range agents {
		if agent.Status == "active" && agent.LastHeartbeat.Before(deadline) {
			if err := m.repo.MarkInactive(ctx, agent.AgentID); err != nil {
				log.Printf("[marketplace] mark inactive failed for %s: %v", agent.AgentID, err)
			}
		}
	}
	return nil
}

// GetAllAgents returns all registered agents.
func (m *AgentMarketplace) GetAllAgents(ctx context.Context) ([]AgentRegistration, error) {
	return m.repo.GetAll(ctx)
}
