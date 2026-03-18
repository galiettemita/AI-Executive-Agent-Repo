package marketplace

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// memRegistry is an in-memory implementation of AgentRegistryRepository for tests.
type memRegistry struct {
	agents map[string]AgentRegistration
}

func newMemRegistry() *memRegistry {
	return &memRegistry{agents: map[string]AgentRegistration{}}
}

func (r *memRegistry) Register(_ context.Context, reg AgentRegistration) error {
	r.agents[reg.AgentID] = reg
	return nil
}

func (r *memRegistry) FindByCapability(_ context.Context, capID string) ([]AgentRegistration, error) {
	var out []AgentRegistration
	for _, a := range r.agents {
		for _, c := range a.Capabilities {
			if c.ID == capID {
				out = append(out, a)
				break
			}
		}
	}
	return out, nil
}

func (r *memRegistry) UpdateHeartbeat(_ context.Context, id string, at time.Time) error {
	if a, ok := r.agents[id]; ok {
		a.LastHeartbeat = at
		r.agents[id] = a
	}
	return nil
}

func (r *memRegistry) MarkInactive(_ context.Context, id string) error {
	if a, ok := r.agents[id]; ok {
		a.Status = "inactive"
		r.agents[id] = a
	}
	return nil
}

func (r *memRegistry) UpdateTrustScore(_ context.Context, id string, score float64) error {
	if a, ok := r.agents[id]; ok {
		a.TrustScore = score
		r.agents[id] = a
	}
	return nil
}

func (r *memRegistry) GetAll(_ context.Context) ([]AgentRegistration, error) {
	out := make([]AgentRegistration, 0, len(r.agents))
	for _, a := range r.agents {
		out = append(out, a)
	}
	return out, nil
}

func (r *memRegistry) GetByID(_ context.Context, id string) (*AgentRegistration, error) {
	a, ok := r.agents[id]
	if !ok {
		return nil, ErrAgentNotFound
	}
	return &a, nil
}

// memOutcomes is an in-memory outcome repo.
type memOutcomes struct {
	outcomes map[string][]bool
}

func newMemOutcomes() *memOutcomes {
	return &memOutcomes{outcomes: map[string][]bool{}}
}

func (o *memOutcomes) RecordOutcome(_ context.Context, agentID string, success bool, _ int64) error {
	o.outcomes[agentID] = append(o.outcomes[agentID], success)
	return nil
}

func (o *memOutcomes) GetStats(_ context.Context, agentID string, _ int) (*AgentStats, error) {
	outs := o.outcomes[agentID]
	if len(outs) == 0 {
		return &AgentStats{}, nil
	}
	successes := 0
	for _, s := range outs {
		if s {
			successes++
		}
	}
	return &AgentStats{
		SuccessRate:       float64(successes) / float64(len(outs)),
		ResponseTimeP99Ms: 500,
		ErrorRate:         1.0 - float64(successes)/float64(len(outs)),
		TotalCalls:        len(outs),
	}, nil
}

// noopValidator accepts all tokens.
type noopValidator struct{}

func (v *noopValidator) Validate(_ context.Context, _ string) error { return nil }

// failValidator rejects all tokens.
type failValidator struct{}

func (v *failValidator) Validate(_ context.Context, _ string) error {
	return errors.New("invalid token")
}

func TestMarketplace_RegisterAndDiscover(t *testing.T) {
	t.Parallel()
	repo := newMemRegistry()
	outcomes := newMemOutcomes()
	scorer := NewAgentTrustScorer(outcomes)
	mp := NewAgentMarketplace(repo, outcomes, scorer, &noopValidator{})

	err := mp.Register(context.Background(), "agent-1", "https://agent1.example.com", "1.0",
		[]Capability{{ID: "calendar", Name: "Calendar", Description: "Calendar management"}},
		[]string{"bearer"}, "valid-token")
	require.NoError(t, err)

	agents, err := mp.Discover(context.Background(), "calendar")
	require.NoError(t, err)
	require.Len(t, agents, 1)
	assert.Equal(t, "agent-1", agents[0].AgentID)
	assert.Equal(t, "active", agents[0].Status)
}

func TestMarketplace_RegisterRejectsInvalidCard(t *testing.T) {
	t.Parallel()
	mp := NewAgentMarketplace(newMemRegistry(), newMemOutcomes(), nil, &noopValidator{})

	err := mp.Register(context.Background(), "", "https://x.com", "1.0", nil, nil, "tok")
	assert.ErrorIs(t, err, ErrInvalidAgentCard)

	err = mp.Register(context.Background(), "a", "https://x.com", "1.0", nil, nil, "tok")
	assert.ErrorIs(t, err, ErrInvalidAgentCard)
}

func TestMarketplace_RegisterRejectsInvalidToken(t *testing.T) {
	t.Parallel()
	mp := NewAgentMarketplace(newMemRegistry(), newMemOutcomes(), nil, &failValidator{})

	err := mp.Register(context.Background(), "a", "https://x.com", "1.0",
		[]Capability{{ID: "x"}}, nil, "bad-token")
	assert.ErrorIs(t, err, ErrUnauthorized)
}

func TestMarketplace_DiscoverSortsByTrustScore(t *testing.T) {
	t.Parallel()
	repo := newMemRegistry()
	mp := NewAgentMarketplace(repo, newMemOutcomes(), nil, &noopValidator{})

	_ = mp.Register(context.Background(), "low", "https://low.example.com", "1.0",
		[]Capability{{ID: "search"}}, nil, "tok")
	_ = mp.Register(context.Background(), "high", "https://high.example.com", "1.0",
		[]Capability{{ID: "search"}}, nil, "tok")

	// Manually set trust scores.
	_ = repo.UpdateTrustScore(context.Background(), "low", 0.3)
	_ = repo.UpdateTrustScore(context.Background(), "high", 0.9)

	agents, _ := mp.Discover(context.Background(), "search")
	require.Len(t, agents, 2)
	assert.Equal(t, "high", agents[0].AgentID)
	assert.Equal(t, "low", agents[1].AgentID)
}

func TestMarketplace_MarkInactiveStaleAgents(t *testing.T) {
	t.Parallel()
	repo := newMemRegistry()
	mp := NewAgentMarketplace(repo, newMemOutcomes(), nil, &noopValidator{})

	_ = mp.Register(context.Background(), "stale", "https://stale.example.com", "1.0",
		[]Capability{{ID: "x"}}, nil, "tok")
	// Backdate heartbeat.
	_ = repo.UpdateHeartbeat(context.Background(), "stale", time.Now().Add(-20*time.Minute))

	_ = mp.MarkInactiveStaleAgents(context.Background(), 15*time.Minute)

	agent, _ := repo.GetByID(context.Background(), "stale")
	assert.Equal(t, "inactive", agent.Status)
}

func TestTrustScorer_Compute(t *testing.T) {
	t.Parallel()
	outcomes := newMemOutcomes()
	scorer := NewAgentTrustScorer(outcomes)

	// No data = neutral 0.5.
	score, _ := scorer.Compute(context.Background(), "new-agent", 0.8)
	assert.Equal(t, 0.5, score)

	// Record some outcomes.
	for i := 0; i < 10; i++ {
		_ = outcomes.RecordOutcome(context.Background(), "tested", true, 200)
	}
	score, _ = scorer.Compute(context.Background(), "tested", 0.8)
	assert.True(t, score > 0.7, "expected high trust score for 100% success rate, got %f", score)
}
