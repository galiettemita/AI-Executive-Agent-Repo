package cognitive

import (
	"context"
	"fmt"
	"sort"
	"sync"

)

// ThoughtStatus represents the lifecycle state of a thought node.
type ThoughtStatus string

const (
	ThoughtActive ThoughtStatus = "active"
	ThoughtMerged ThoughtStatus = "merged"
	ThoughtPruned ThoughtStatus = "pruned"
	ThoughtFinal  ThoughtStatus = "final"
)

// ThoughtNode is a single node in a Graph-of-Thought structure.
type ThoughtNode struct {
	ID       string
	Content  string
	ParentID string
	Children []string
	Score    float64
	Status   ThoughtStatus
	Depth    int
}

// ThoughtGraph holds an entire graph-of-thought exploration.
type ThoughtGraph struct {
	ID           string
	RootID       string
	Nodes        map[string]*ThoughtNode
	MaxDepth     int
	BranchFactor int
	mu           sync.RWMutex
}

// ThoughtResult is the outcome of a full GoT evaluation.
type ThoughtResult struct {
	FinalThought  string
	NodesExplored int
	Depth         int
	BestScore     float64
}

// GraphOfThought implements graph-of-thought reasoning.
type GraphOfThought struct{}

// NewGraphOfThought creates a new GraphOfThought service.
func NewGraphOfThought() *GraphOfThought {
	return &GraphOfThought{}
}

// NewGraph creates a new ThoughtGraph with a root node.
func (g *GraphOfThought) NewGraph(rootThought string, maxDepth, branchFactor int) *ThoughtGraph {
	rootID := newID()
	root := &ThoughtNode{
		ID:       rootID,
		Content:  rootThought,
		Children: []string{},
		Score:    0.0,
		Status:   ThoughtActive,
		Depth:    0,
	}

	tg := &ThoughtGraph{
		ID:           newID(),
		RootID:       rootID,
		Nodes:        map[string]*ThoughtNode{rootID: root},
		MaxDepth:     maxDepth,
		BranchFactor: branchFactor,
	}
	return tg
}

// Branch creates child thought nodes from a parent node.
func (g *GraphOfThought) Branch(graph *ThoughtGraph, parentID string, thoughts []string) ([]*ThoughtNode, error) {
	graph.mu.Lock()
	defer graph.mu.Unlock()

	parent, ok := graph.Nodes[parentID]
	if !ok {
		return nil, fmt.Errorf("parent node %s not found", parentID)
	}
	if parent.Status == ThoughtPruned {
		return nil, fmt.Errorf("cannot branch from pruned node %s", parentID)
	}
	if parent.Depth >= graph.MaxDepth {
		return nil, fmt.Errorf("max depth %d reached", graph.MaxDepth)
	}
	if len(thoughts) > graph.BranchFactor {
		thoughts = thoughts[:graph.BranchFactor]
	}

	var nodes []*ThoughtNode
	for _, t := range thoughts {
		node := &ThoughtNode{
			ID:       newID(),
			Content:  t,
			ParentID: parentID,
			Children: []string{},
			Score:    0.0,
			Status:   ThoughtActive,
			Depth:    parent.Depth + 1,
		}
		graph.Nodes[node.ID] = node
		parent.Children = append(parent.Children, node.ID)
		nodes = append(nodes, node)
	}
	return nodes, nil
}

// Merge combines multiple thought nodes into a synthesized node.
func (g *GraphOfThought) Merge(graph *ThoughtGraph, nodeIDs []string, synthesis string) (*ThoughtNode, error) {
	graph.mu.Lock()
	defer graph.mu.Unlock()

	if len(nodeIDs) < 2 {
		return nil, fmt.Errorf("merge requires at least 2 nodes")
	}

	maxDepth := 0
	var parentID string
	totalScore := 0.0

	for _, id := range nodeIDs {
		node, ok := graph.Nodes[id]
		if !ok {
			return nil, fmt.Errorf("node %s not found", id)
		}
		if node.Depth > maxDepth {
			maxDepth = node.Depth
		}
		totalScore += node.Score
		parentID = node.ParentID
		node.Status = ThoughtMerged
	}

	merged := &ThoughtNode{
		ID:       newID(),
		Content:  synthesis,
		ParentID: parentID,
		Children: []string{},
		Score:    totalScore / float64(len(nodeIDs)),
		Status:   ThoughtActive,
		Depth:    maxDepth,
	}
	graph.Nodes[merged.ID] = merged

	return merged, nil
}

// Prune marks a node and all its descendants as pruned.
func (g *GraphOfThought) Prune(graph *ThoughtGraph, nodeID string) {
	graph.mu.Lock()
	defer graph.mu.Unlock()

	g.pruneRecursive(graph, nodeID)
}

func (g *GraphOfThought) pruneRecursive(graph *ThoughtGraph, nodeID string) {
	node, ok := graph.Nodes[nodeID]
	if !ok {
		return
	}
	node.Status = ThoughtPruned
	for _, childID := range node.Children {
		g.pruneRecursive(graph, childID)
	}
}

// ScoreNode assigns a score to a thought node.
func (g *GraphOfThought) ScoreNode(graph *ThoughtGraph, nodeID string, score float64) error {
	graph.mu.Lock()
	defer graph.mu.Unlock()

	node, ok := graph.Nodes[nodeID]
	if !ok {
		return fmt.Errorf("node %s not found", nodeID)
	}
	node.Score = score
	return nil
}

// BestPath finds the highest-scoring path from root to a leaf node.
func (g *GraphOfThought) BestPath(graph *ThoughtGraph) []*ThoughtNode {
	graph.mu.RLock()
	defer graph.mu.RUnlock()

	root, ok := graph.Nodes[graph.RootID]
	if !ok {
		return nil
	}

	bestPath, _ := g.bestPathDFS(graph, root)
	return bestPath
}

func (g *GraphOfThought) bestPathDFS(graph *ThoughtGraph, node *ThoughtNode) ([]*ThoughtNode, float64) {
	if node.Status == ThoughtPruned {
		return nil, -1
	}

	activeChildren := make([]*ThoughtNode, 0)
	for _, cid := range node.Children {
		child, ok := graph.Nodes[cid]
		if ok && child.Status != ThoughtPruned {
			activeChildren = append(activeChildren, child)
		}
	}

	// Leaf node.
	if len(activeChildren) == 0 {
		return []*ThoughtNode{node}, node.Score
	}

	var bestChildPath []*ThoughtNode
	bestChildScore := -1.0

	for _, child := range activeChildren {
		path, score := g.bestPathDFS(graph, child)
		if path != nil && score > bestChildScore {
			bestChildScore = score
			bestChildPath = path
		}
	}

	if bestChildPath == nil {
		return []*ThoughtNode{node}, node.Score
	}

	fullPath := append([]*ThoughtNode{node}, bestChildPath...)
	return fullPath, node.Score + bestChildScore
}

// Evaluate runs a full Graph-of-Thought evaluation cycle.
func (g *GraphOfThought) Evaluate(ctx context.Context, graph *ThoughtGraph) (*ThoughtResult, error) {
	if graph == nil {
		return nil, fmt.Errorf("graph must not be nil")
	}

	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	default:
	}

	// Score all unscored active nodes based on content length heuristic.
	graph.mu.Lock()
	for _, node := range graph.Nodes {
		if node.Status == ThoughtActive && node.Score == 0.0 && len(node.Content) > 0 {
			// Simple scoring: longer, more detailed thoughts score higher.
			node.Score = float64(len(node.Content)) / 100.0
			if node.Score > 1.0 {
				node.Score = 1.0
			}
		}
	}
	graph.mu.Unlock()

	// Prune low-scoring leaf nodes.
	graph.mu.Lock()
	for _, node := range graph.Nodes {
		if node.Status == ThoughtActive && len(node.Children) == 0 && node.Score < 0.2 && node.ID != graph.RootID {
			node.Status = ThoughtPruned
		}
	}
	graph.mu.Unlock()

	// Find the best path.
	bestPath := g.BestPath(graph)

	explored := 0
	maxDepth := 0
	bestScore := 0.0

	graph.mu.RLock()
	for _, node := range graph.Nodes {
		if node.Status != ThoughtPruned {
			explored++
		}
		if node.Depth > maxDepth && node.Status != ThoughtPruned {
			maxDepth = node.Depth
		}
		if node.Score > bestScore {
			bestScore = node.Score
		}
	}
	graph.mu.RUnlock()

	finalThought := ""
	if len(bestPath) > 0 {
		finalNode := bestPath[len(bestPath)-1]
		finalNode.Status = ThoughtFinal
		finalThought = finalNode.Content
	}

	return &ThoughtResult{
		FinalThought:  finalThought,
		NodesExplored: explored,
		Depth:         maxDepth,
		BestScore:     bestScore,
	}, nil
}

// activeLeaves returns all active leaf nodes sorted by score descending.
func (g *GraphOfThought) activeLeaves(graph *ThoughtGraph) []*ThoughtNode {
	graph.mu.RLock()
	defer graph.mu.RUnlock()

	var leaves []*ThoughtNode
	for _, node := range graph.Nodes {
		if node.Status != ThoughtActive {
			continue
		}
		hasActiveChild := false
		for _, cid := range node.Children {
			if c, ok := graph.Nodes[cid]; ok && c.Status == ThoughtActive {
				hasActiveChild = true
				break
			}
		}
		if !hasActiveChild {
			leaves = append(leaves, node)
		}
	}
	sort.Slice(leaves, func(i, j int) bool {
		return leaves[i].Score > leaves[j].Score
	})
	return leaves
}
