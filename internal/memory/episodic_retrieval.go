package memory

import (
	"fmt"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
)

// Episode represents a stored episodic memory entry.
type Episode struct {
	ID          uuid.UUID
	WorkspaceID string
	Summary     string
	Date        time.Time
	Keywords    []string
}

// EpisodicRetriever provides workspace-scoped episode storage and retrieval.
type EpisodicRetriever struct {
	mu       sync.Mutex
	episodes map[string][]Episode // keyed by workspace_id
}

// NewEpisodicRetriever creates a new EpisodicRetriever.
func NewEpisodicRetriever() *EpisodicRetriever {
	return &EpisodicRetriever{
		episodes: map[string][]Episode{},
	}
}

// StoreEpisode stores an episode for a workspace.
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

	er.mu.Lock()
	defer er.mu.Unlock()
	er.episodes[workspaceID] = append(er.episodes[workspaceID], episode)
	return episode, nil
}

// RetrieveRelevant returns the most relevant episodes for a query, limited by count.
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

	type scored struct {
		episode Episode
		score   float64
	}

	queryKeywords := extractKeywords(query)
	scored_episodes := make([]scored, 0, len(episodes))
	for _, ep := range episodes {
		score := RankByRelevance(queryKeywords, ep.Keywords)
		scored_episodes = append(scored_episodes, scored{episode: ep, score: score})
	}

	sort.Slice(scored_episodes, func(i, j int) bool {
		if scored_episodes[i].score == scored_episodes[j].score {
			return scored_episodes[i].episode.Date.After(scored_episodes[j].episode.Date)
		}
		return scored_episodes[i].score > scored_episodes[j].score
	})

	if len(scored_episodes) > limit {
		scored_episodes = scored_episodes[:limit]
	}

	result := make([]Episode, 0, len(scored_episodes))
	for _, se := range scored_episodes {
		result = append(result, se.episode)
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
