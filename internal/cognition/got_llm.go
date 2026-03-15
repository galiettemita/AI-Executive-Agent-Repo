package cognition

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/brevio/brevio/internal/llm"
)

const gotHypothesisModel = "claude-haiku-4-5-20251001"
const gotSynthesisModel = "claude-sonnet-4-20250514"
const gotHypothesisMaxTokens = 1024
const gotSynthesisMaxTokens = 512
const gotDefaultPruneThreshold = 0.35
const gotDefaultMaxBranches = 3

// GoTLLMConfig configures the LLM-driven GoT engine.
type GoTLLMConfig struct {
	LLMClient      llm.Client
	MaxBranches    int
	PruneThreshold float64
	MaxDepth       int
	EnableMerge    bool
}

// GoTRunRequest is the input to one GoT session.
type GoTRunRequest struct {
	WorkspaceID string
	Question    string
	Context     string
	MaxSteps    int
}

// GoTRunResult is the output of a complete GoT session.
type GoTRunResult struct {
	GraphID     string  `json:"graph_id"`
	Conclusion  string  `json:"conclusion"`
	Confidence  float64 `json:"confidence"`
	NodeCount   int     `json:"node_count"`
	PrunedCount int     `json:"pruned_count"`
	Steps       int     `json:"steps"`
	LatencyMs   int64   `json:"latency_ms"`
}

// GoTLLMEngine wraps GoTEngine and drives it with LLM calls.
type GoTLLMEngine struct {
	inner *GoTEngine
	cfg   GoTLLMConfig
}

// NewGoTLLMEngine creates a GoTLLMEngine.
func NewGoTLLMEngine(cfg GoTLLMConfig) *GoTLLMEngine {
	if cfg.MaxBranches <= 0 {
		cfg.MaxBranches = gotDefaultMaxBranches
	}
	if cfg.PruneThreshold <= 0 {
		cfg.PruneThreshold = gotDefaultPruneThreshold
	}
	if cfg.MaxDepth <= 0 {
		cfg.MaxDepth = 4
	}
	return &GoTLLMEngine{inner: NewGoTEngine(), cfg: cfg}
}

// Run executes a full GoT reasoning session.
func (e *GoTLLMEngine) Run(ctx context.Context, req GoTRunRequest) (*GoTRunResult, error) {
	if strings.TrimSpace(req.Question) == "" {
		return nil, fmt.Errorf("got_llm: question required")
	}
	if req.MaxSteps <= 0 {
		req.MaxSteps = 3
	}
	start := time.Now()

	graph, err := e.inner.CreateGraph(req.Question)
	if err != nil {
		return nil, fmt.Errorf("got_llm: create graph: %w", err)
	}

	frontier := []string{graph.RootID}
	totalPruned := 0

	for step := 0; step < req.MaxSteps; step++ {
		if ctx.Err() != nil || len(frontier) == 0 {
			break
		}
		var newFrontier []string

		for _, parentID := range frontier {
			parent, ok := e.inner.GetNode(graph.ID, parentID)
			if !ok || parent.Status == "pruned" {
				continue
			}

			nodeType := "hypothesis"
			if step == req.MaxSteps-1 {
				nodeType = "conclusion"
			}

			hyps, err := e.generateHypotheses(ctx, req.Question, req.Context, parent.Content, nodeType)
			if err != nil {
				continue
			}

			for _, h := range hyps {
				node, err := e.inner.Branch(graph.ID, parentID, h.Content, nodeType)
				if err != nil {
					continue
				}
				_ = e.inner.SetNodeConfidence(graph.ID, node.ID, h.Score)
				if h.Score < e.cfg.PruneThreshold {
					_ = e.inner.Prune(graph.ID, node.ID)
					totalPruned++
					continue
				}
				newFrontier = append(newFrontier, node.ID)
			}
		}
		frontier = newFrontier
	}

	conclusion := ""
	confidence := 0.0
	if e.cfg.EnableMerge && len(frontier) >= 2 {
		top := e.TopNodes(graph.ID, frontier, 3)
		if len(top) >= 2 {
			synthesis, err := e.synthesise(ctx, req.Question, req.Context, graph.ID, top)
			if err == nil && synthesis != "" {
				merged, mergeErr := e.inner.Merge(graph.ID, top, synthesis)
				if mergeErr == nil {
					conclusion = merged.Content
					confidence = merged.Confidence
				}
			}
		}
	}

	if conclusion == "" {
		best, err := e.inner.GetBestConclusion(graph.ID)
		if err == nil && best != nil {
			conclusion, confidence = best.Content, best.Confidence
		} else if root, ok := e.inner.GetNode(graph.ID, graph.RootID); ok {
			conclusion, confidence = root.Content, 0.3
		}
	}

	eval, _ := e.inner.Evaluate(graph.ID)
	nodeCount := 1
	if eval != nil {
		nodeCount += eval.BranchCount
	}

	return &GoTRunResult{
		GraphID:     graph.ID,
		Conclusion:  conclusion,
		Confidence:  confidence,
		NodeCount:   nodeCount,
		PrunedCount: totalPruned,
		Steps:       req.MaxSteps,
		LatencyMs:   time.Since(start).Milliseconds(),
	}, nil
}

// Hypothesis is one branch candidate.
type Hypothesis struct {
	Content string  `json:"content"`
	Score   float64 `json:"score"`
}

const hypsSystemPrompt = `You are a rigorous analytical reasoner.
Generate %d DISTINCT alternative hypotheses for the question.
Each must be substantively different. Rate plausibility 0.0-1.0.
Respond ONLY with JSON array: [{"content":"...","score":0.8},...]
No preamble.`

func (e *GoTLLMEngine) generateHypotheses(ctx context.Context, question, ctxStr, parentContent, nodeType string) ([]Hypothesis, error) {
	system := fmt.Sprintf(hypsSystemPrompt, e.cfg.MaxBranches)
	var b strings.Builder
	b.WriteString(fmt.Sprintf("Question: %q\n", question))
	if ctxStr != "" {
		b.WriteString(fmt.Sprintf("Context: %s\n", ctxStr))
	}
	b.WriteString(fmt.Sprintf("Current reasoning: %q\nType: %s.", parentContent, nodeType))

	resp, _, err := e.cfg.LLMClient.Generate(ctx, llm.GenerateRequest{
		Model:       gotHypothesisModel,
		MaxTokens:   gotHypothesisMaxTokens,
		Temperature: 0.7,
		System:      system,
		Messages:    []llm.ChatMsg{{Role: "user", Content: b.String()}},
	})
	if err != nil {
		return nil, err
	}
	clean := strings.TrimSpace(resp.Content)
	clean = strings.TrimPrefix(strings.TrimPrefix(clean, "```json"), "```")
	clean = strings.TrimSuffix(clean, "```")
	clean = strings.TrimSpace(clean)

	var hyps []Hypothesis
	if err := json.Unmarshal([]byte(clean), &hyps); err != nil {
		return nil, fmt.Errorf("got_llm: parse hypotheses: %w", err)
	}
	valid := hyps[:0]
	for _, h := range hyps {
		if strings.TrimSpace(h.Content) == "" {
			continue
		}
		if h.Score < 0 {
			h.Score = 0
		}
		if h.Score > 1 {
			h.Score = 1
		}
		valid = append(valid, h)
	}
	return valid, nil
}

const synthSystemPrompt = `You are a synthesis expert.
Given a question and reasoning branches, produce ONE authoritative conclusion
integrating the strongest elements. 2-3 sentences max. Plain text only.`

func (e *GoTLLMEngine) synthesise(ctx context.Context, question, ctxStr, graphID string, nodeIDs []string) (string, error) {
	var b strings.Builder
	b.WriteString(fmt.Sprintf("Question: %q\n\nBranches:\n", question))
	for i, nid := range nodeIDs {
		n, ok := e.inner.GetNode(graphID, nid)
		if !ok {
			continue
		}
		b.WriteString(fmt.Sprintf("  %d (conf=%.2f): %s\n", i+1, n.Confidence, n.Content))
	}
	if ctxStr != "" {
		b.WriteString(fmt.Sprintf("\nContext: %s\n", ctxStr))
	}
	resp, _, err := e.cfg.LLMClient.Generate(ctx, llm.GenerateRequest{
		Model:       gotSynthesisModel,
		MaxTokens:   gotSynthesisMaxTokens,
		Temperature: 0.2,
		System:      synthSystemPrompt,
		Messages:    []llm.ChatMsg{{Role: "user", Content: b.String()}},
	})
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(resp.Content), nil
}

// TopNodes returns the top-N nodes by confidence from the given IDs.
func (e *GoTLLMEngine) TopNodes(graphID string, ids []string, n int) []string {
	type ns struct {
		id   string
		conf float64
	}
	scored := make([]ns, 0, len(ids))
	for _, id := range ids {
		node, ok := e.inner.GetNode(graphID, id)
		if !ok || node.Status == "pruned" {
			continue
		}
		scored = append(scored, ns{id, node.Confidence})
	}
	sort.Slice(scored, func(i, j int) bool { return scored[i].conf > scored[j].conf })
	result := make([]string, 0, n)
	for i := 0; i < n && i < len(scored); i++ {
		result = append(result, scored[i].id)
	}
	return result
}
