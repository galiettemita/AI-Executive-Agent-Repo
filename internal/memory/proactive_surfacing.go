package memory

import (
	"fmt"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
)

// SurfacingCandidate represents a memory that may be proactively surfaced.
type SurfacingCandidate struct {
	MemoryID       uuid.UUID
	Content        string
	RelevanceScore float64
	Reason         string
}

// SurfacingMemory is an in-memory store entry for proactive surfacing.
type SurfacingMemory struct {
	ID          uuid.UUID
	WorkspaceID string
	Content     string
	Keywords    []string
	CreatedAt   time.Time
}

// ProactiveSurfacingService surfaces relevant memories based on current context.
type ProactiveSurfacingService struct {
	mu       sync.Mutex
	memories map[string][]SurfacingMemory // keyed by workspace_id
}

// NewProactiveSurfacingService creates a new ProactiveSurfacingService.
func NewProactiveSurfacingService() *ProactiveSurfacingService {
	return &ProactiveSurfacingService{
		memories: map[string][]SurfacingMemory{},
	}
}

// AddMemory stores a memory for proactive surfacing.
func (ps *ProactiveSurfacingService) AddMemory(workspaceID, content string) (SurfacingMemory, error) {
	if strings.TrimSpace(workspaceID) == "" {
		return SurfacingMemory{}, fmt.Errorf("workspace_id is required")
	}
	if strings.TrimSpace(content) == "" {
		return SurfacingMemory{}, fmt.Errorf("content is required")
	}

	mem := SurfacingMemory{
		ID:          uuid.Must(uuid.NewV7()),
		WorkspaceID: workspaceID,
		Content:     strings.TrimSpace(content),
		Keywords:    extractKeywords(content),
		CreatedAt:   time.Now().UTC(),
	}

	ps.mu.Lock()
	defer ps.mu.Unlock()
	ps.memories[workspaceID] = append(ps.memories[workspaceID], mem)
	return mem, nil
}

// FindRelevantMemories returns the most relevant memories for the current context.
func (ps *ProactiveSurfacingService) FindRelevantMemories(workspaceID, currentContext string, limit int) []SurfacingCandidate {
	ps.mu.Lock()
	defer ps.mu.Unlock()

	if limit <= 0 {
		limit = 5
	}

	memories := ps.memories[workspaceID]
	if len(memories) == 0 {
		return nil
	}

	contextKeywords := extractKeywords(currentContext)
	now := time.Now().UTC()

	type scored struct {
		candidate SurfacingCandidate
		score     float64
	}

	candidates := make([]scored, 0, len(memories))
	for _, mem := range memories {
		keywordScore := RankByRelevance(contextKeywords, mem.Keywords)
		recencyScore := computeRecencyScore(now, mem.CreatedAt)
		combinedScore := 0.7*keywordScore + 0.3*recencyScore

		if combinedScore <= 0 {
			continue
		}

		reason := "keyword_match"
		if keywordScore == 0 && recencyScore > 0 {
			reason = "recency"
		} else if keywordScore > 0 && recencyScore > 0 {
			reason = "keyword_match+recency"
		}

		candidates = append(candidates, scored{
			candidate: SurfacingCandidate{
				MemoryID:       mem.ID,
				Content:        mem.Content,
				RelevanceScore: combinedScore,
				Reason:         reason,
			},
			score: combinedScore,
		})
	}

	sort.Slice(candidates, func(i, j int) bool {
		return candidates[i].score > candidates[j].score
	})

	if len(candidates) > limit {
		candidates = candidates[:limit]
	}

	result := make([]SurfacingCandidate, 0, len(candidates))
	for _, c := range candidates {
		result = append(result, c.candidate)
	}
	return result
}

// computeRecencyScore returns a score between 0 and 1 based on how recent the memory is.
// Memories within the last hour score 1.0; decay linearly over 7 days.
func computeRecencyScore(now, created time.Time) float64 {
	age := now.Sub(created)
	if age <= 0 {
		return 1.0
	}
	maxAge := 7 * 24 * time.Hour
	if age >= maxAge {
		return 0
	}
	return 1.0 - (float64(age) / float64(maxAge))
}
