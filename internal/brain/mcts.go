package brain

import (
	"context"
	"fmt"
	"math"
	"sort"
	"sync"
	"time"

	"github.com/brevio/brevio/internal/llm"
)

var mctsTempCycle = []float64{0.1, 0.4, 0.6, 0.3, 0.5}

// MCTSConfig configures Monte Carlo Tree Search planning.
type MCTSConfig struct {
	LLMClient         llm.Client
	CounterfactualSvc *CounterfactualService
	ORM               *OutcomeRewardModel
	MaxIterations     int
	MaxDepth          int
	ExplorationC      float64
	BranchingFactor   int
	TimeLimit         time.Duration
}

// MCTSNode is a node in the MCTS search tree.
type MCTSNode struct {
	mu         sync.Mutex
	Parent     *MCTSNode
	Children   []*MCTSNode
	Plan       *Plan
	Visits     int
	TotalValue float64
	Depth      int
	ExpandIdx  int
}

// UCB1 computes the Upper Confidence Bound 1 score.
func (n *MCTSNode) UCB1(c float64, parentVisits int) float64 {
	n.mu.Lock()
	defer n.mu.Unlock()
	if n.Visits == 0 {
		return math.Inf(1)
	}
	exploit := n.TotalValue / float64(n.Visits)
	explore := c * math.Sqrt(math.Log(float64(parentVisits))/float64(n.Visits))
	return exploit + explore
}

func (n *MCTSNode) avgValue() float64 {
	n.mu.Lock()
	defer n.mu.Unlock()
	if n.Visits == 0 {
		return 0
	}
	return n.TotalValue / float64(n.Visits)
}

// MCTSPlanner explores the plan space using MCTS.
type MCTSPlanner struct {
	cfg MCTSConfig
}

// NewMCTSPlanner creates an MCTS planner.
func NewMCTSPlanner(cfg MCTSConfig) *MCTSPlanner {
	if cfg.MaxIterations <= 0 {
		cfg.MaxIterations = 12
	}
	if cfg.MaxDepth <= 0 {
		cfg.MaxDepth = 8
	}
	if cfg.ExplorationC <= 0 {
		cfg.ExplorationC = math.Sqrt2
	}
	if cfg.BranchingFactor <= 0 {
		cfg.BranchingFactor = 3
	}
	if cfg.TimeLimit <= 0 {
		cfg.TimeLimit = 10 * time.Second
	}
	return &MCTSPlanner{cfg: cfg}
}

// Search runs MCTS and returns the best plan found.
func (m *MCTSPlanner) Search(ctx context.Context, input LLMPlannerInput) (*Plan, error) {
	if m.cfg.LLMClient == nil {
		return nil, fmt.Errorf("mcts: LLMClient required")
	}
	deadline := time.Now().Add(m.cfg.TimeLimit)
	root := &MCTSNode{Plan: &Plan{Steps: []PlanStep{}, RiskLevel: "low"}, Depth: 0}

	for i := 0; i < m.cfg.MaxIterations; i++ {
		if ctx.Err() != nil || time.Now().After(deadline) {
			break
		}
		leaf := m.selectLeaf(root)
		children := m.expand(ctx, leaf, input)
		for _, child := range children {
			value := m.simulate(child.Plan)
			m.backprop(child, value)
		}
		if len(children) == 0 {
			m.backprop(leaf, m.simulate(leaf.Plan))
		}
	}

	best := m.bestPlan(root, 0)
	if best == nil || len(best.Steps) == 0 {
		plan, _, err := callLLMPlanner(ctx, m.cfg.LLMClient, input)
		return plan, err
	}
	return best, nil
}

func (m *MCTSPlanner) selectLeaf(node *MCTSNode) *MCTSNode {
	for depth := 0; depth < m.cfg.MaxDepth*2; depth++ {
		node.mu.Lock()
		children := make([]*MCTSNode, len(node.Children))
		copy(children, node.Children)
		visits := node.Visits
		node.mu.Unlock()

		if len(children) == 0 {
			return node
		}
		for _, c := range children {
			c.mu.Lock()
			v := c.Visits
			c.mu.Unlock()
			if v == 0 {
				return c
			}
		}
		best := children[0]
		bestScore := children[0].UCB1(m.cfg.ExplorationC, visits)
		for _, c := range children[1:] {
			if s := c.UCB1(m.cfg.ExplorationC, visits); s > bestScore {
				bestScore = s
				best = c
			}
		}
		node = best
	}
	return node
}

func (m *MCTSPlanner) expand(ctx context.Context, node *MCTSNode, input LLMPlannerInput) []*MCTSNode {
	if node.Depth >= m.cfg.MaxDepth {
		return nil
	}
	children := make([]*MCTSNode, 0, m.cfg.BranchingFactor)
	for k := 0; k < m.cfg.BranchingFactor; k++ {
		expandInput := input
		expandInput.Temperature = mctsTempCycle[(node.ExpandIdx+k)%len(mctsTempCycle)]
		expandInput.UseThinking = false

		plan, _, err := callLLMPlanner(ctx, m.cfg.LLMClient, expandInput)
		if err != nil || plan == nil || len(plan.Steps) == 0 {
			continue
		}
		child := &MCTSNode{
			Parent:    node,
			Plan:      plan,
			Depth:     node.Depth + 1,
			ExpandIdx: (node.ExpandIdx + k + 1) % len(mctsTempCycle),
		}
		node.mu.Lock()
		node.Children = append(node.Children, child)
		node.mu.Unlock()
		children = append(children, child)
	}
	return children
}

func (m *MCTSPlanner) simulate(plan *Plan) float64 {
	if plan == nil || len(plan.Steps) == 0 {
		return 0
	}

	// ORM-backed scoring: evaluate the plan trajectory using the outcome reward model.
	if m.cfg.ORM != nil && m.cfg.ORM.llmClient != nil {
		ormScore, err := m.cfg.ORM.ScoreFinalOutcome(
			context.Background(),
			plan.WorkspaceID,
			"mcts_simulation",
			plan.Steps,
			nil, // no results yet (simulation)
			fmt.Sprintf("Plan with %d steps, risk=%s", len(plan.Steps), plan.RiskLevel),
		)
		if err == nil {
			return ormScore.OverallQuality / 5.0 // normalize to 0–1 for UCB1
		}
		// Fallback to heuristic on ORM failure.
	}

	if m.cfg.CounterfactualSvc != nil {
		analysis, err := m.cfg.CounterfactualSvc.ScoreAlternatives(
			Plan{Steps: []PlanStep{}, RiskLevel: "low"},
			[]Plan{*plan},
		)
		if err == nil && analysis != nil && len(analysis.AlternativeScores) > 0 {
			return analysis.AlternativeScores[0]
		}
	}
	return scorePlan(*plan)
}

func (m *MCTSPlanner) backprop(node *MCTSNode, value float64) {
	for node != nil {
		node.mu.Lock()
		node.Visits++
		node.TotalValue += value
		node.mu.Unlock()
		node = node.Parent
	}
}

func (m *MCTSPlanner) bestPlan(node *MCTSNode, depth int) *Plan {
	if node == nil || depth > m.cfg.MaxDepth {
		return nil
	}
	node.mu.Lock()
	children := make([]*MCTSNode, len(node.Children))
	copy(children, node.Children)
	node.mu.Unlock()

	if len(children) == 0 {
		return node.Plan
	}
	sort.Slice(children, func(i, j int) bool { return children[i].avgValue() > children[j].avgValue() })
	if result := m.bestPlan(children[0], depth+1); result != nil {
		return result
	}
	return node.Plan
}
