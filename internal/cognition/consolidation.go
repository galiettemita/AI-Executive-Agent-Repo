package cognition

import (
	"fmt"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
)

// Episode represents an episodic memory entry.
type Episode struct {
	ID              string    `json:"id"`
	Content         string    `json:"content"`
	Timestamp       time.Time `json:"timestamp"`
	Tags            []string  `json:"tags"`
	ImportanceScore float64   `json:"importance_score"`
}

// SemanticPattern represents a recurring pattern found across episodes.
type SemanticPattern struct {
	Pattern        string   `json:"pattern"`
	Frequency      int      `json:"frequency"`
	Confidence     float64  `json:"confidence"`
	SourceEpisodes []string `json:"source_episodes"`
}

// ConsolidationRun tracks the results of a consolidation operation.
type ConsolidationRun struct {
	ID                 string    `json:"id"`
	WorkspaceID        string    `json:"workspace_id"`
	StartedAt          time.Time `json:"started_at"`
	CompletedAt        time.Time `json:"completed_at"`
	EpisodicProcessed  int       `json:"episodic_processed"`
	SemanticsExtracted int       `json:"semantics_extracted"`
	PatternsFound      int       `json:"patterns_found"`
}

// ConsolidationService manages overnight memory consolidation.
type ConsolidationService struct {
	mu              sync.Mutex
	runs            map[string]*ConsolidationRun
	semanticMemory  []SemanticPattern
}

// NewConsolidationService creates a new ConsolidationService.
func NewConsolidationService() *ConsolidationService {
	return &ConsolidationService{
		runs:           make(map[string]*ConsolidationRun),
		semanticMemory: []SemanticPattern{},
	}
}

// RunConsolidation processes episodes and extracts patterns.
func (s *ConsolidationService) RunConsolidation(workspaceID string, episodes []Episode) (*ConsolidationRun, error) {
	if strings.TrimSpace(workspaceID) == "" {
		return nil, fmt.Errorf("workspace_id is required")
	}
	if len(episodes) == 0 {
		return nil, fmt.Errorf("at least one episode is required")
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	startedAt := time.Now().UTC()
	patterns := extractPatternsInternal(episodes)

	runID := uuid.Must(uuid.NewV7()).String()
	run := &ConsolidationRun{
		ID:                 runID,
		WorkspaceID:        workspaceID,
		StartedAt:          startedAt,
		CompletedAt:        time.Now().UTC(),
		EpisodicProcessed:  len(episodes),
		SemanticsExtracted: countHighConfidence(patterns),
		PatternsFound:      len(patterns),
	}

	s.runs[runID] = run
	return run, nil
}

func countHighConfidence(patterns []SemanticPattern) int {
	count := 0
	for _, p := range patterns {
		if p.Confidence >= 0.5 {
			count++
		}
	}
	return count
}

// ExtractPatterns finds recurring themes/patterns across episodes.
func (s *ConsolidationService) ExtractPatterns(episodes []Episode) []SemanticPattern {
	return extractPatternsInternal(episodes)
}

func extractPatternsInternal(episodes []Episode) []SemanticPattern {
	// Count word/tag co-occurrences across episodes
	tagFreq := make(map[string][]string) // tag -> episode IDs
	wordFreq := make(map[string][]string)

	for _, ep := range episodes {
		for _, tag := range ep.Tags {
			tagLower := strings.ToLower(tag)
			tagFreq[tagLower] = append(tagFreq[tagLower], ep.ID)
		}
		words := strings.Fields(strings.ToLower(ep.Content))
		seen := make(map[string]struct{})
		for _, w := range words {
			if len(w) < 4 { // skip short words
				continue
			}
			if _, ok := seen[w]; ok {
				continue
			}
			seen[w] = struct{}{}
			wordFreq[w] = append(wordFreq[w], ep.ID)
		}
	}

	var patterns []SemanticPattern

	// Tags that appear in multiple episodes are patterns
	for tag, epIDs := range tagFreq {
		if len(epIDs) >= 2 {
			confidence := float64(len(epIDs)) / float64(len(episodes))
			if confidence > 1.0 {
				confidence = 1.0
			}
			patterns = append(patterns, SemanticPattern{
				Pattern:        "tag:" + tag,
				Frequency:      len(epIDs),
				Confidence:     confidence,
				SourceEpisodes: epIDs,
			})
		}
	}

	// Words that appear in multiple episodes
	for word, epIDs := range wordFreq {
		if len(epIDs) >= 3 {
			confidence := float64(len(epIDs)) / float64(len(episodes))
			if confidence > 1.0 {
				confidence = 1.0
			}
			patterns = append(patterns, SemanticPattern{
				Pattern:        "word:" + word,
				Frequency:      len(epIDs),
				Confidence:     confidence,
				SourceEpisodes: epIDs,
			})
		}
	}

	sort.Slice(patterns, func(i, j int) bool {
		return patterns[i].Confidence > patterns[j].Confidence
	})

	return patterns
}

// PromoteToSemantic converts an episodic pattern to long-term semantic memory.
func (s *ConsolidationService) PromoteToSemantic(pattern SemanticPattern) error {
	if strings.TrimSpace(pattern.Pattern) == "" {
		return fmt.Errorf("pattern is required")
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	// Check for duplicates
	for _, existing := range s.semanticMemory {
		if existing.Pattern == pattern.Pattern {
			return fmt.Errorf("pattern already promoted: %s", pattern.Pattern)
		}
	}

	s.semanticMemory = append(s.semanticMemory, pattern)
	return nil
}
