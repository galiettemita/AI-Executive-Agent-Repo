package memory

import (
	"context"
	"math"
	"sort"
	"strings"
	"sync"
	"time"
)

// actionVerbs are high-signal words that indicate procedural memory is relevant.
var actionVerbs = map[string]float64{
	"schedule": 1.3, "book": 1.3, "send": 1.2, "create": 1.2,
	"draft": 1.2, "set": 1.1, "find": 1.1, "update": 1.1,
	"cancel": 1.2, "reschedule": 1.3, "forward": 1.1, "reply": 1.1,
	"remind": 1.2, "add": 1.1, "remove": 1.1, "move": 1.1,
}

// ProceduralEmbedder is the minimal embedding interface for procedural retrieval.
type ProceduralEmbedder interface {
	Embed(ctx context.Context, texts []string) ([][]float32, error)
}

// ProceduralMemoryService retrieves procedural memories with action-verb boosting.
type ProceduralMemoryService struct {
	embedder ProceduralEmbedder
	mu       sync.Mutex
	items    map[string][]Item // keyed by workspace_id
}

func NewProceduralMemoryService(embedder ProceduralEmbedder) *ProceduralMemoryService {
	return &ProceduralMemoryService{
		embedder: embedder,
		items:    map[string][]Item{},
	}
}

// AddProcedural stores a procedural memory item.
func (s *ProceduralMemoryService) AddProcedural(item Item) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.items[item.WorkspaceID] = append(s.items[item.WorkspaceID], item)
}

// SearchProcedural finds procedural memories relevant to the query.
func (s *ProceduralMemoryService) SearchProcedural(
	ctx context.Context,
	workspaceID string,
	query string,
	limit int,
) ([]Item, error) {
	if limit <= 0 {
		limit = 5
	}

	s.mu.Lock()
	items := append([]Item(nil), s.items[workspaceID]...)
	s.mu.Unlock()

	if len(items) == 0 {
		return nil, nil
	}

	// Embed query for semantic scoring.
	var queryVec []float32
	if s.embedder != nil {
		if embs, err := s.embedder.Embed(ctx, []string{query}); err == nil && len(embs) > 0 {
			queryVec = embs[0]
		}
	}

	actionMultiplier := ActionVerbMultiplier(query)

	type scored struct {
		item  Item
		score float64
	}
	results := make([]scored, 0, len(items))
	for _, item := range items {
		var base float64
		if queryVec != nil && item.Embedding != nil {
			base = float64(cosineSim32(queryVec, item.Embedding))
		} else {
			// Keyword fallback
			queryKW := extractKeywords(query)
			itemKW := extractKeywords(item.Body)
			base = RankByRelevance(queryKW, itemKW)
		}

		base *= actionMultiplier

		// Recency: procedural memories older than 90 days get mild penalty.
		daysSince := time.Since(item.CreatedAt).Hours() / 24
		if daysSince > 90 {
			base *= math.Exp(-0.005 * (daysSince - 90))
		}

		results = append(results, scored{item: item, score: base})
	}

	sort.Slice(results, func(i, j int) bool {
		return results[i].score > results[j].score
	})

	out := make([]Item, 0, limit)
	for i := 0; i < limit && i < len(results); i++ {
		it := results[i].item
		it.RelevanceScore = results[i].score
		out = append(out, it)
	}
	return out, nil
}

// ActionVerbMultiplier returns a score multiplier based on detected action verbs.
func ActionVerbMultiplier(query string) float64 {
	words := strings.Fields(strings.ToLower(query))
	maxBoost := 1.0
	for _, word := range words {
		if boost, ok := actionVerbs[word]; ok {
			if boost > maxBoost {
				maxBoost = boost
			}
		}
	}
	return maxBoost
}
