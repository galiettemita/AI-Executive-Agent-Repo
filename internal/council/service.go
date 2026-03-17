package council

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/brevio/brevio/internal/llm"
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
	AgentID       string    `json:"agent_id"`
	Vote          string    `json:"vote"` // approve, reject, abstain
	Confidence    float64   `json:"confidence"`
	Justification string    `json:"justification,omitempty"`
	CastAt        time.Time `json:"cast_at"`
}

// Council represents a convened council session.
type Council struct {
	ID             string         `json:"id"`
	WorkspaceID    string         `json:"workspace_id"`
	Topic          string         `json:"topic"`
	Agents         []CouncilAgent `json:"agents"`
	Status         string         `json:"status"` // convened, deliberating, concluded
	Decision       string         `json:"decision"`
	Votes          []Vote         `json:"votes"`
	VotingStrategy string         `json:"voting_strategy"` // majority | unanimous | weighted
	CreatedAt      time.Time      `json:"created_at"`
}

// CouncilDecision is the resolved outcome of a council deliberation.
type CouncilDecision struct {
	Decision   string  `json:"decision"`
	Confidence float64 `json:"confidence"`
	Dissenting int     `json:"dissenting"`
}

// CouncilService manages council convening and deliberation.
type CouncilService struct {
	mu        sync.RWMutex
	nextID    int
	councils  map[string]*Council
	agentPool []CouncilAgent
	now       func() time.Time
	llmClient llm.Client // may be nil; agents abstain if nil
}

// NewCouncilService creates a new council service.
func NewCouncilService() *CouncilService {
	return NewCouncilServiceWithLLM(nil)
}

// NewCouncilServiceWithLLM creates a council service with LLM-backed deliberation.
func NewCouncilServiceWithLLM(client llm.Client) *CouncilService {
	return &CouncilService{
		nextID:    1,
		councils:  map[string]*Council{},
		agentPool: []CouncilAgent{},
		now:       func() time.Time { return time.Now().UTC() },
		llmClient: client,
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
	council.VotingStrategy = policy.VotingStrategy
	if council.VotingStrategy == "" {
		council.VotingStrategy = "majority"
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

	strategy := council.VotingStrategy
	if strategy == "" {
		strategy = "majority"
	}

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

// Agent ID constants for specialist agents.
const (
	AgentIDFinancialRisk = "agent_financial_risk"
	AgentIDLegal         = "agent_legal"
	AgentIDUserAdvocate  = "agent_user_advocate"
	AgentIDTechnical     = "agent_technical"
	AgentIDEthics        = "agent_ethics"
)

var agentSystemPrompts = map[string]string{
	AgentIDFinancialRisk: `You are a financial risk specialist. Assess financial risk.
Respond ONLY with JSON: {"vote":"approve|reject|abstain","confidence":<0-1>,"justification":"<one sentence>"}`,
	AgentIDLegal: `You are a legal compliance specialist. Review for legal/regulatory risk.
Respond ONLY with JSON: {"vote":"approve|reject|abstain","confidence":<0-1>,"justification":"<one sentence>"}`,
	AgentIDUserAdvocate: `You are a user advocate protecting the user's best interests.
Respond ONLY with JSON: {"vote":"approve|reject|abstain","confidence":<0-1>,"justification":"<one sentence>"}`,
	AgentIDTechnical: `You are a technical reliability specialist. Assess execution risk.
Respond ONLY with JSON: {"vote":"approve|reject|abstain","confidence":<0-1>,"justification":"<one sentence>"}`,
	AgentIDEthics: `You are an AI ethics specialist. Review for ethical issues.
Respond ONLY with JSON: {"vote":"approve|reject|abstain","confidence":<0-1>,"justification":"<one sentence>"}`,
}

// DefaultCouncilAgents returns the standard specialist agents.
func DefaultCouncilAgents() []CouncilAgent {
	return []CouncilAgent{
		{ID: AgentIDFinancialRisk, Name: "Financial Risk", Capabilities: []string{"financial", "risk"}},
		{ID: AgentIDLegal, Name: "Legal Compliance", Capabilities: []string{"legal", "compliance"}},
		{ID: AgentIDUserAdvocate, Name: "User Advocate", Capabilities: []string{"user_welfare"}},
		{ID: AgentIDTechnical, Name: "Technical Review", Capabilities: []string{"technical", "reliability"}},
	}
}

// DeliberateAndVote has each council agent deliberate via LLM before voting.
func (s *CouncilService) DeliberateAndVote(ctx context.Context, councilID, actionDescription string) error {
	s.mu.RLock()
	council, ok := s.councils[councilID]
	if !ok {
		s.mu.RUnlock()
		return fmt.Errorf("council %q not found", councilID)
	}
	agents := make([]CouncilAgent, len(council.Agents))
	copy(agents, council.Agents)
	topic := council.Topic
	s.mu.RUnlock()

	if s.llmClient == nil {
		for _, ag := range agents {
			_ = s.CastVote(councilID, ag.ID, "abstain", 0.5)
		}
		return nil
	}

	// voteResult carries a single agent's deliberation outcome across the fan-out channel.
	type voteResult struct {
		agentID       string
		vote          string
		confidence    float64
		justification string
	}

	results := make(chan voteResult, len(agents))

	for _, ag := range agents {
		ag := ag // capture: prevents all goroutines closing over same pointer
		go func() {
			vote, conf, just, err := s.agentDeliberate(
				ctx,
				ag,
				topic,
				actionDescription,
			)
			if err != nil {
				vote, conf, just = "abstain", 0.5, "deliberation error"
			}
			results <- voteResult{
				agentID:       ag.ID,
				vote:          vote,
				confidence:    conf,
				justification: just,
			}
		}()
	}

	for range agents {
		r := <-results
		s.mu.Lock()
		if c, ok := s.councils[councilID]; ok {
			alreadyVoted := false
			for _, v := range c.Votes {
				if v.AgentID == r.agentID {
					alreadyVoted = true
					break
				}
			}
			if !alreadyVoted {
				c.Status = "deliberating"
				c.Votes = append(c.Votes, Vote{
					AgentID:       r.agentID,
					Vote:          r.vote,
					Confidence:    r.confidence,
					Justification: r.justification,
					CastAt:        s.now(),
				})
			}
		}
		s.mu.Unlock()
	}
	return nil
}

func (s *CouncilService) agentDeliberate(ctx context.Context, agent CouncilAgent, topic, action string) (string, float64, string, error) {
	systemPrompt, ok := agentSystemPrompts[agent.ID]
	if !ok {
		systemPrompt = `You are a council member. Review the proposed action.
Respond ONLY with JSON: {"vote":"approve|reject|abstain","confidence":<0-1>,"justification":"<one sentence>"}`
	}
	userMsg := fmt.Sprintf("Topic: %q\n\nProposed action:\n%s", topic, action)
	resp, _, apiErr := s.llmClient.Generate(ctx, llm.GenerateRequest{
		Model:       "claude-haiku-4-5-20251001",
		MaxTokens:   256,
		Temperature: 0.1,
		System:      systemPrompt,
		Messages:    []llm.ChatMsg{{Role: "user", Content: userMsg}},
	})
	if apiErr != nil {
		return "abstain", 0.5, "", apiErr
	}
	var result struct {
		Vote          string  `json:"vote"`
		Confidence    float64 `json:"confidence"`
		Justification string  `json:"justification"`
	}
	if jsonErr := json.Unmarshal([]byte(strings.TrimSpace(resp.Content)), &result); jsonErr != nil {
		return "abstain", 0.5, "", jsonErr
	}
	switch result.Vote {
	case "approve", "reject", "abstain":
	default:
		result.Vote = "abstain"
	}
	if result.Confidence < 0 || result.Confidence > 1 {
		result.Confidence = 0.5
	}
	return result.Vote, result.Confidence, result.Justification, nil
}

// ConveneAndDecide is a convenience method that convenes, deliberates, and resolves.
func (s *CouncilService) ConveneAndDecide(ctx context.Context, workspaceID, topic, actionDesc string) (bool, error) {
	for _, ag := range DefaultCouncilAgents() {
		s.RegisterAgent(ag)
	}
	policy := CouncilPolicy{
		MinAgents: 2, MaxAgents: 4, VotingStrategy: "majority",
		ConveneThreshold: 0.0,
	}
	council, err := s.ConveneCouncil(workspaceID, topic, policy)
	if err != nil {
		return false, fmt.Errorf("council: convene: %w", err)
	}
	if err := s.DeliberateAndVote(ctx, council.ID, actionDesc); err != nil {
		return false, fmt.Errorf("council: deliberate: %w", err)
	}
	decision, err := s.ResolveCouncil(council.ID)
	if err != nil {
		return false, fmt.Errorf("council: resolve: %w", err)
	}
	return decision.Decision == "approved", nil
}
