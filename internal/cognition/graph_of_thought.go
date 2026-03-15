package cognition

import (
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
)

// ThoughtNode represents a single node in a graph-of-thought.
type ThoughtNode struct {
	ID         string   `json:"id"`
	Content    string   `json:"content"`
	Type       string   `json:"type"`       // hypothesis, evidence, conclusion, question
	Confidence float64  `json:"confidence"` // 0.0 - 1.0
	Children   []string `json:"children"`
	Parents    []string `json:"parents"`
	Status     string   `json:"status"` // active, merged, pruned
}

// ThoughtGraph holds a directed acyclic graph of thought nodes.
type ThoughtGraph struct {
	ID        string                  `json:"id"`
	RootID    string                  `json:"root_id"`
	Nodes     map[string]*ThoughtNode `json:"nodes"`
	CreatedAt time.Time               `json:"created_at"`
}

// GraphEvaluation contains the evaluation results for a thought graph.
type GraphEvaluation struct {
	BestPath    []string `json:"best_path"`
	Confidence  float64  `json:"confidence"`
	BranchCount int      `json:"branch_count"`
	Depth       int      `json:"depth"`
	PrunedCount int      `json:"pruned_count"`
}

var validThoughtTypes = map[string]struct{}{
	"hypothesis": {},
	"evidence":   {},
	"conclusion": {},
	"question":   {},
}

// GoTEngine manages graph-of-thought reasoning.
type GoTEngine struct {
	mu     sync.Mutex
	graphs map[string]*ThoughtGraph
}

// NewGoTEngine creates a new GoTEngine.
func NewGoTEngine() *GoTEngine {
	return &GoTEngine{
		graphs: make(map[string]*ThoughtGraph),
	}
}

// CreateGraph creates a new thought graph with a root node.
func (e *GoTEngine) CreateGraph(rootContent string) (*ThoughtGraph, error) {
	if strings.TrimSpace(rootContent) == "" {
		return nil, fmt.Errorf("root content is required")
	}

	e.mu.Lock()
	defer e.mu.Unlock()

	graphID := uuid.Must(uuid.NewV7()).String()
	rootID := uuid.Must(uuid.NewV7()).String()

	root := &ThoughtNode{
		ID:         rootID,
		Content:    rootContent,
		Type:       "question",
		Confidence: 0.5,
		Children:   []string{},
		Parents:    []string{},
		Status:     "active",
	}

	graph := &ThoughtGraph{
		ID:        graphID,
		RootID:    rootID,
		Nodes:     map[string]*ThoughtNode{rootID: root},
		CreatedAt: time.Now().UTC(),
	}

	e.graphs[graphID] = graph
	return graph, nil
}

// Branch creates a divergent sub-thought from a parent node.
func (e *GoTEngine) Branch(graphID, parentID, content string, thoughtType string) (*ThoughtNode, error) {
	if strings.TrimSpace(content) == "" {
		return nil, fmt.Errorf("content is required")
	}
	if _, ok := validThoughtTypes[thoughtType]; !ok {
		return nil, fmt.Errorf("invalid thought type: %s", thoughtType)
	}

	e.mu.Lock()
	defer e.mu.Unlock()

	graph, ok := e.graphs[graphID]
	if !ok {
		return nil, fmt.Errorf("graph not found: %s", graphID)
	}

	parent, ok := graph.Nodes[parentID]
	if !ok {
		return nil, fmt.Errorf("parent node not found: %s", parentID)
	}

	if parent.Status == "pruned" {
		return nil, fmt.Errorf("cannot branch from pruned node")
	}

	nodeID := uuid.Must(uuid.NewV7()).String()
	node := &ThoughtNode{
		ID:         nodeID,
		Content:    content,
		Type:       thoughtType,
		Confidence: 0.5,
		Children:   []string{},
		Parents:    []string{parentID},
		Status:     "active",
	}

	graph.Nodes[nodeID] = node
	parent.Children = append(parent.Children, nodeID)

	return node, nil
}

// Merge synthesizes multiple branches into a single conclusion node.
func (e *GoTEngine) Merge(graphID string, nodeIDs []string, synthesis string) (*ThoughtNode, error) {
	if len(nodeIDs) < 2 {
		return nil, fmt.Errorf("at least 2 nodes required for merge")
	}
	if strings.TrimSpace(synthesis) == "" {
		return nil, fmt.Errorf("synthesis content is required")
	}

	e.mu.Lock()
	defer e.mu.Unlock()

	graph, ok := e.graphs[graphID]
	if !ok {
		return nil, fmt.Errorf("graph not found: %s", graphID)
	}

	var totalConf float64
	for _, nid := range nodeIDs {
		node, ok := graph.Nodes[nid]
		if !ok {
			return nil, fmt.Errorf("node not found: %s", nid)
		}
		if node.Status == "pruned" {
			return nil, fmt.Errorf("cannot merge pruned node: %s", nid)
		}
		totalConf += node.Confidence
	}

	mergedID := uuid.Must(uuid.NewV7()).String()
	merged := &ThoughtNode{
		ID:         mergedID,
		Content:    synthesis,
		Type:       "conclusion",
		Confidence: totalConf / float64(len(nodeIDs)),
		Children:   []string{},
		Parents:    append([]string(nil), nodeIDs...),
		Status:     "active",
	}

	graph.Nodes[mergedID] = merged

	for _, nid := range nodeIDs {
		node := graph.Nodes[nid]
		node.Children = append(node.Children, mergedID)
		node.Status = "merged"
	}

	return merged, nil
}

// Prune marks a node and its descendants as pruned.
func (e *GoTEngine) Prune(graphID, nodeID string) error {
	e.mu.Lock()
	defer e.mu.Unlock()

	graph, ok := e.graphs[graphID]
	if !ok {
		return fmt.Errorf("graph not found: %s", graphID)
	}

	node, ok := graph.Nodes[nodeID]
	if !ok {
		return fmt.Errorf("node not found: %s", nodeID)
	}

	if nodeID == graph.RootID {
		return fmt.Errorf("cannot prune root node")
	}

	e.pruneRecursive(graph, node)
	return nil
}

func (e *GoTEngine) pruneRecursive(graph *ThoughtGraph, node *ThoughtNode) {
	node.Status = "pruned"
	for _, childID := range node.Children {
		if child, ok := graph.Nodes[childID]; ok {
			e.pruneRecursive(graph, child)
		}
	}
}

// Evaluate scores the quality of a thought graph.
func (e *GoTEngine) Evaluate(graphID string) (*GraphEvaluation, error) {
	e.mu.Lock()
	defer e.mu.Unlock()

	graph, ok := e.graphs[graphID]
	if !ok {
		return nil, fmt.Errorf("graph not found: %s", graphID)
	}

	branchCount := 0
	prunedCount := 0
	for _, node := range graph.Nodes {
		if len(node.Children) > 1 {
			branchCount++
		}
		if node.Status == "pruned" {
			prunedCount++
		}
	}

	depth := e.computeDepth(graph, graph.RootID, 0)
	bestPath, bestConf := e.findBestPath(graph, graph.RootID)

	return &GraphEvaluation{
		BestPath:    bestPath,
		Confidence:  bestConf,
		BranchCount: branchCount,
		Depth:       depth,
		PrunedCount: prunedCount,
	}, nil
}

func (e *GoTEngine) computeDepth(graph *ThoughtGraph, nodeID string, current int) int {
	node, ok := graph.Nodes[nodeID]
	if !ok {
		return current
	}
	maxDepth := current
	for _, childID := range node.Children {
		d := e.computeDepth(graph, childID, current+1)
		if d > maxDepth {
			maxDepth = d
		}
	}
	return maxDepth
}

func (e *GoTEngine) findBestPath(graph *ThoughtGraph, nodeID string) ([]string, float64) {
	node, ok := graph.Nodes[nodeID]
	if !ok {
		return nil, 0
	}

	if len(node.Children) == 0 || node.Status == "pruned" {
		return []string{nodeID}, node.Confidence
	}

	var bestPath []string
	bestConf := -1.0

	for _, childID := range node.Children {
		child := graph.Nodes[childID]
		if child.Status == "pruned" {
			continue
		}
		path, conf := e.findBestPath(graph, childID)
		if conf > bestConf {
			bestConf = conf
			bestPath = path
		}
	}

	if bestPath == nil {
		return []string{nodeID}, node.Confidence
	}

	return append([]string{nodeID}, bestPath...), (node.Confidence + bestConf) / 2.0
}

// AddHypothesis creates a hypothesis node branching from the graph root.
// Convenience wrapper used by PlannerStep during multi-path planning.
func (e *GoTEngine) AddHypothesis(graphID, content string, confidence float64) (string, error) {
	e.mu.Lock()
	graph, ok := e.graphs[graphID]
	if !ok {
		e.mu.Unlock()
		return "", fmt.Errorf("got: graph %q not found", graphID)
	}
	rootID := graph.RootID
	e.mu.Unlock()

	node, err := e.Branch(graphID, rootID, content, "hypothesis")
	if err != nil {
		return "", err
	}

	e.mu.Lock()
	defer e.mu.Unlock()
	if g, ok := e.graphs[graphID]; ok {
		if n, ok := g.Nodes[node.ID]; ok {
			n.Confidence = confidence
		}
	}
	return node.ID, nil
}

// GetBestConclusion returns the conclusion node with the highest confidence.
func (e *GoTEngine) GetBestConclusion(graphID string) (*ThoughtNode, error) {
	e.mu.Lock()
	defer e.mu.Unlock()

	graph, ok := e.graphs[graphID]
	if !ok {
		return nil, fmt.Errorf("graph not found: %s", graphID)
	}

	var best *ThoughtNode
	for _, node := range graph.Nodes {
		if node.Type != "conclusion" || node.Status == "pruned" {
			continue
		}
		if best == nil || node.Confidence > best.Confidence {
			best = node
		}
	}

	if best == nil {
		return nil, fmt.Errorf("no conclusion nodes found")
	}
	return best, nil
}
