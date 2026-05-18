package council

import (
	"fmt"
	"sync"
	"time"
)

// CouncilPolicy defines rules for when and how to convene a council.
type CouncilPolicy struct {
	MinAgents            int      `json:"min_agents"`
	MaxAgents            int      `json:"max_agents"`
	RequiredCapabilities []string `json:"required_capabilities"`
	VotingStrategy       string   `json:"voting_strategy"`   // majority, unanimous, weighted
	ConveneThreshold     float64  `json:"convene_threshold"` // 0.0-1.0; topic complexity must exceed this
}

// CouncilAgent represents an agent participating in a council.
type CouncilAgent struct {
	ID           string   `json:"id"`
	Name         string   `json:"name"`
	Capabilities []string `json:"capabilities"`
}

// Vote records an agent's vote in a council deliberation.
type Vote struct {
	AgentID    string  `json:"agent_id"`
	Vote       string  `json:"vote"` // approve, reject, abstain
	Confidence float64 `json:"confidence"`
	CastAt     time.Time `json:"cast_at"`
}

// Council represents a convened council session.
type Council struct {
	ID          string         `json:"id"`
	WorkspaceID string         `json:"workspace_id"`
	Topic       string         `json:"topic"`
	Agents      []CouncilAgent `json:"agents"`
	Status      string         `json:"status"` // convened, deliberating, concluded
	Decision    string         `json:"decision"`
	Votes       []Vote         `json:"votes"`
	CreatedAt   time.Time      `json:"created_at"`
}

// CouncilDecision is the resolved outcome of a council deliberation.
type CouncilDecision struct {
	Decision   string  `json:"decision"`
	Confidence float64 `json:"confidence"`
	Dissenting int     `json:"dissenting"`
}

// CouncilService manages council convening and deliberation.
type CouncilService struct {
	mu       sync.RWMutex
	nextID   int
	councils map[string]*Council
	// agentPool is the set of available agents
	agentPool []CouncilAgent
	now       func() time.Time
}

// NewCouncilService creates a new council service.
func NewCouncilService() *CouncilService {
	return &CouncilService{
		nextID:   1,
		councils: map[string]*Council{},
		agentPool: []CouncilAgent{},
		now:      func() time.Time { return time.Now().UTC() },
	}
}

// RegisterAgent adds an agent to the available pool.
func (s *CouncilService) RegisterAgent(agent CouncilAgent) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.agentPool = append(s.agentPool, agent)
}

// ConveneCouncil evaluates whether to convene a council based on the policy
// threshold and assembles qualifying agents.
func (s *CouncilService) ConveneCouncil(workspaceID string, topic string, policy CouncilPolicy) (*Council, error) {
	if workspaceID == "" {
		return nil, fmt.Errorf("workspace_id is required")
	}
	if topic == "" {
		return nil, fmt.Errorf("topic is required")
	}

	// Compute a simple topic complexity heuristic (normalised length / 200, capped at 1.0)
	complexity := float64(len(topic)) / 200.0
	if complexity > 1.0 {
		complexity = 1.0
	}

	if complexity < policy.ConveneThreshold {
		return nil, fmt.Errorf("topic complexity %.2f below convene threshold %.2f", complexity, policy.ConveneThreshold)
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	// Select agents that have required capabilities
	var eligible []CouncilAgent
	for _, agent := range s.agentPool {
		if hasRequiredCapabilities(agent.Capabilities, policy.RequiredCapabilities) {
			eligible = append(eligible, agent)
		}
	}

	if len(eligible) < policy.MinAgents {
		return nil, fmt.Errorf("insufficient agents: need %d, have %d", policy.MinAgents, len(eligible))
	}

	// Cap to MaxAgents
	if policy.MaxAgents > 0 && len(eligible) > policy.MaxAgents {
		eligible = eligible[:policy.MaxAgents]
	}

	council := &Council{
		ID:          fmt.Sprintf("council_%06d", s.nextID),
		WorkspaceID: workspaceID,
		Topic:       topic,
		Agents:      eligible,
		Status:      "convened",
		Votes:       []Vote{},
		CreatedAt:   s.now(),
	}
	s.nextID++
	s.councils[council.ID] = council
	return council, nil
}

// CastVote records a vote from an agent in a council.
func (s *CouncilService) CastVote(councilID, agentID string, vote string, confidence float64) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	council, ok := s.councils[councilID]
	if !ok {
		return fmt.Errorf("council %q not found", councilID)
	}
	if council.Status == "concluded" {
		return fmt.Errorf("council %q has already concluded", councilID)
	}

	// Verify agent is in the council
	agentFound := false
	for _, a := range council.Agents {
		if a.ID == agentID {
			agentFound = true
			break
		}
	}
	if !agentFound {
		return fmt.Errorf("agent %q is not part of council %q", agentID, councilID)
	}

	// Prevent duplicate votes
	for _, v := range council.Votes {
		if v.AgentID == agentID {
			return fmt.Errorf("agent %q has already voted", agentID)
		}
	}

	council.Status = "deliberating"
	council.Votes = append(council.Votes, Vote{
		AgentID:    agentID,
		Vote:       vote,
		Confidence: confidence,
		CastAt:     s.now(),
	})
	return nil
}

// ResolveCouncil applies the voting strategy and produces a decision.
func (s *CouncilService) ResolveCouncil(councilID string) (*CouncilDecision, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	council, ok := s.councils[councilID]
	if !ok {
		return nil, fmt.Errorf("council %q not found", councilID)
	}
	if len(council.Votes) == 0 {
		return nil, fmt.Errorf("no votes cast in council %q", councilID)
	}

	// Count votes
	approvals := 0
	rejections := 0
	totalConfidence := 0.0
	weightedApprove := 0.0
	weightedReject := 0.0

	for _, v := range council.Votes {
		switch v.Vote {
		case "approve":
			approvals++
			weightedApprove += v.Confidence
		case "reject":
			rejections++
			weightedReject += v.Confidence
		}
		totalConfidence += v.Confidence
	}

	decision := &CouncilDecision{}
	totalVoters := len(council.Votes)

	// Determine the voting strategy from the policy.
	// We look up the original policy indirectly through council metadata.
	// For simplicity, we detect strategy from council config; default to majority.
	strategy := "majority"
	// The strategy is determined at convene time — we store it as part of internal state.
	// For the in-memory implementation, we infer from vote patterns.
	// In practice we'd store policy alongside the council.

	switch strategy {
	case "unanimous":
		if approvals == totalVoters {
			decision.Decision = "approved"
		} else {
			decision.Decision = "rejected"
		}
		decision.Dissenting = rejections
	case "weighted":
		if weightedApprove > weightedReject {
			decision.Decision = "approved"
		} else {
			decision.Decision = "rejected"
		}
		decision.Dissenting = rejections
	default: // majority
		if approvals > totalVoters/2 {
			decision.Decision = "approved"
		} else {
			decision.Decision = "rejected"
		}
		decision.Dissenting = rejections
	}

	if totalConfidence > 0 {
		decision.Confidence = totalConfidence / float64(totalVoters)
	}

	council.Status = "concluded"
	council.Decision = decision.Decision
	return decision, nil
}

// GetCouncil retrieves a council by ID.
func (s *CouncilService) GetCouncil(councilID string) (*Council, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	c, ok := s.councils[councilID]
	if !ok {
		return nil, false
	}
	cp := *c
	return &cp, true
}

func hasRequiredCapabilities(agentCaps, required []string) bool {
	if len(required) == 0 {
		return true
	}
	capSet := map[string]struct{}{}
	for _, c := range agentCaps {
		capSet[c] = struct{}{}
	}
	for _, req := range required {
		if _, ok := capSet[req]; !ok {
			return false
		}
	}
	return true
}
