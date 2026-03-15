package memory

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
)

// EpisodicEmbedder is the minimal embedding interface for episodic retrieval.
type EpisodicEmbedder interface {
	Embed(ctx context.Context, texts []string) ([][]float32, error)
}

// Episode represents a stored episodic memory entry.
type Episode struct {
	ID              uuid.UUID
	WorkspaceID     string
	Summary         string
	Date            time.Time
	Keywords        []string
	Embedding       []float32 // nil if not yet embedded
	SimilarityScore float64   // set during retrieval, not persisted
}

// EpisodicRetriever provides workspace-scoped episode storage and retrieval.
// Supports hybrid scoring: embedding similarity (70%) + keyword overlap (30%).
type EpisodicRetriever struct {
	mu       sync.Mutex
	episodes map[string][]Episode // keyed by workspace_id
	embedder EpisodicEmbedder     // nil = keyword-only mode
}

// NewEpisodicRetriever creates a new EpisodicRetriever.
// Pass nil embedder for keyword-only mode (backwards compatible).
func NewEpisodicRetriever(embedder EpisodicEmbedder) *EpisodicRetriever {
	return &EpisodicRetriever{
		episodes: map[string][]Episode{},
		embedder: embedder,
	}
}

// StoreEpisode stores an episode for a workspace. Embeds at write time if embedder is available.
func (er *EpisodicRetriever) StoreEpisode(workspaceID, summary string, date time.Time) (Episode, error) {
	if strings.TrimSpace(workspaceID) == "" {
		return Episode{}, fmt.Errorf("workspace_id is required")
	}
	if strings.TrimSpace(summary) == "" {
		return Episode{}, fmt.Errorf("summary is required")
	}

	episode := Episode{
		ID:          uuid.Must(uuid.NewV7()),
		WorkspaceID: workspaceID,
		Summary:     strings.TrimSpace(summary),
		Date:        date,
		Keywords:    extractKeywords(summary),
	}

	// Embed at write time — best-effort (keyword fallback on failure).
	if er.embedder != nil {
		if embs, err := er.embedder.Embed(context.Background(), []string{summary}); err == nil && len(embs) > 0 {
			episode.Embedding = embs[0]
		}
	}

	er.mu.Lock()
	defer er.mu.Unlock()
	er.episodes[workspaceID] = append(er.episodes[workspaceID], episode)
	return episode, nil
}

// RetrieveRelevant returns the most relevant episodes for a query using hybrid scoring:
// 70% embedding cosine similarity + 30% keyword overlap. Falls back to keyword-only
// if embedder is nil or embedding fails.
func (er *EpisodicRetriever) RetrieveRelevant(workspaceID, query string, limit int) []Episode {
	er.mu.Lock()
	defer er.mu.Unlock()

	if limit <= 0 {
		limit = 5
	}

	episodes := er.episodes[workspaceID]
	if len(episodes) == 0 {
		return nil
	}

	queryKeywords := extractKeywords(query)

	// Attempt to embed the query for semantic scoring.
	var queryVec []float32
	if er.embedder != nil {
		if embs, err := er.embedder.Embed(context.Background(), []string{query}); err == nil && len(embs) > 0 {
			queryVec = embs[0]
		}
	}

	type scored struct {
		episode Episode
		score   float64
	}

	scoredEpisodes := make([]scored, 0, len(episodes))
	for _, ep := range episodes {
		keywordScore := RankByRelevance(queryKeywords, ep.Keywords)

		var hybrid float64
		if queryVec != nil && ep.Embedding != nil {
			denseScore := float64(cosineSim32(queryVec, ep.Embedding))
			hybrid = 0.70*denseScore + 0.30*keywordScore
		} else {
			hybrid = keywordScore // keyword-only fallback
		}

		// Temporal recency boost: episodes from the last 7 days get +5% boost.
		daysSince := time.Since(ep.Date).Hours() / 24
		if daysSince < 7 {
			hybrid *= 1.05
		}

		scoredEpisodes = append(scoredEpisodes, scored{episode: ep, score: hybrid})
	}

	sort.Slice(scoredEpisodes, func(i, j int) bool {
		if scoredEpisodes[i].score == scoredEpisodes[j].score {
			return scoredEpisodes[i].episode.Date.After(scoredEpisodes[j].episode.Date)
		}
		return scoredEpisodes[i].score > scoredEpisodes[j].score
	})

	if len(scoredEpisodes) > limit {
		scoredEpisodes = scoredEpisodes[:limit]
	}

	result := make([]Episode, 0, len(scoredEpisodes))
	for _, se := range scoredEpisodes {
		ep := se.episode
		ep.SimilarityScore = se.score
		result = append(result, ep)
	}
	return result
}

// RankByRelevance computes keyword overlap score between query keywords and episode keywords.
func RankByRelevance(queryKeywords, episodeKeywords []string) float64 {
	if len(queryKeywords) == 0 || len(episodeKeywords) == 0 {
		return 0
	}

	episodeSet := map[string]struct{}{}
	for _, kw := range episodeKeywords {
		episodeSet[kw] = struct{}{}
	}

	matches := 0
	for _, kw := range queryKeywords {
		if _, ok := episodeSet[kw]; ok {
			matches++
		}
	}

	return float64(matches) / float64(len(queryKeywords))
}

// InjectIntoContext formats episodes into a context string.
func InjectIntoContext(episodes []Episode) string {
	if len(episodes) == 0 {
		return ""
	}

	var sb strings.Builder
	sb.WriteString("[Episodic Memory Context]\n")
	for i, ep := range episodes {
		sb.WriteString(fmt.Sprintf("- Episode %d (%s): %s\n", i+1, ep.Date.Format("2006-01-02"), ep.Summary))
	}
	return sb.String()
}

// extractKeywords splits text into lowercase keyword tokens.
func extractKeywords(text string) []string {
	fields := strings.Fields(strings.ToLower(strings.TrimSpace(text)))
	out := make([]string, 0, len(fields))
	for _, f := range fields {
		clean := strings.Trim(f, ".,:;!?()[]{}\"'")
		if clean == "" {
			continue
		}
		out = append(out, clean)
	}
	return out
}
