package memory

import (
	"context"
	"fmt"
	"math"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
)

// ProactiveSurfacingThreshold is the minimum composite score to surface a memory.
const ProactiveSurfacingThreshold = 0.45

// SurfacingCandidate represents a memory that may be proactively surfaced.
type SurfacingCandidate struct {
	MemoryID       uuid.UUID
	Content        string
	RelevanceScore float64
	Reason         string
}

// ProactiveSurfacingEmbedder is the minimal embedding interface for proactive surfacing.
type ProactiveSurfacingEmbedder interface {
	Embed(ctx context.Context, texts []string) ([][]float32, error)
}

// SurfacingMemory is an in-memory store entry for proactive surfacing.
type SurfacingMemory struct {
	ID          uuid.UUID
	WorkspaceID string
	Content     string
	Keywords    []string
	Embedding   []float32 // nil if not yet embedded
	Confidence  float64   // 0 treated as 1.0
	CreatedAt   time.Time
}

// ProactiveSurfacingService surfaces relevant memories based on current context.
// Supports hybrid scoring: embedding similarity (65%) + keyword overlap (20%) + temporal (15%).
type ProactiveSurfacingService struct {
	mu       sync.Mutex
	memories map[string][]SurfacingMemory // keyed by workspace_id
	embedder ProactiveSurfacingEmbedder   // nil = keyword-only mode
}

// NewProactiveSurfacingService creates a new ProactiveSurfacingService.
// Pass nil embedder for keyword-only mode (backwards compatible).
func NewProactiveSurfacingService(embedder ProactiveSurfacingEmbedder) *ProactiveSurfacingService {
	return &ProactiveSurfacingService{
		memories: map[string][]SurfacingMemory{},
		embedder: embedder,
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
		Confidence:  1.0,
		CreatedAt:   time.Now().UTC(),
	}

	// Embed at write time — best-effort.
	if ps.embedder != nil {
		if embs, err := ps.embedder.Embed(context.Background(), []string{content}); err == nil && len(embs) > 0 {
			mem.Embedding = embs[0]
		}
	}

	ps.mu.Lock()
	defer ps.mu.Unlock()
	ps.memories[workspaceID] = append(ps.memories[workspaceID], mem)
	return mem, nil
}

// FindRelevantMemories returns the most relevant memories for the current context.
// Uses hybrid scoring: embedding similarity (65%) + keyword overlap (20%) + temporal (15%).
// Falls back to keyword + recency when embedder is nil or embedding fails.
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

	// Attempt to embed the context signal for semantic scoring.
	var contextVec []float32
	if ps.embedder != nil {
		if embs, err := ps.embedder.Embed(context.Background(), []string{currentContext}); err == nil && len(embs) > 0 {
			contextVec = embs[0]
		}
	}

	type scored struct {
		candidate SurfacingCandidate
		score     float64
	}

	candidates := make([]scored, 0, len(memories))
	for _, mem := range memories {
		keywordScore := RankByRelevance(contextKeywords, mem.Keywords)
		temporalScore := computeRecencyScore(now, mem.CreatedAt)

		var composite float64
		if contextVec != nil && mem.Embedding != nil {
			denseScore := float64(cosineSim32(contextVec, mem.Embedding))
			composite = 0.65*denseScore + 0.20*keywordScore + 0.15*temporalScore
		} else {
			// Keyword-only fallback (original formula).
			composite = 0.7*keywordScore + 0.3*temporalScore
		}

		// Confidence dampening.
		confidence := mem.Confidence
		if confidence <= 0 {
			confidence = 1.0
		}
		composite *= confidence

		// Temporal penalty for very old memories (exponential).
		daysSince := now.Sub(mem.CreatedAt).Hours() / 24
		if daysSince > 30 {
			composite *= math.Exp(-0.03 * (daysSince - 30))
		}

		if composite < ProactiveSurfacingThreshold {
			continue
		}

		reason := "semantic_match"
		if contextVec == nil || mem.Embedding == nil {
			reason = "keyword_match"
		}
		if keywordScore > 0 && temporalScore > 0 {
			reason += "+recency"
		}

		candidates = append(candidates, scored{
			candidate: SurfacingCandidate{
				MemoryID:       mem.ID,
				Content:        mem.Content,
				RelevanceScore: composite,
				Reason:         reason,
			},
			score: composite,
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
