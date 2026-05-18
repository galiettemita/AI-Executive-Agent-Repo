package cognitive

import (
	"sort"
	"strings"
	"sync"
	"time"

)

// Episode represents a unit of episodic memory.
type Episode struct {
	ID         string
	Content    string
	Timestamp  time.Time
	Tags       []string
	Importance float64
}

// SemanticPattern is a pattern extracted from episodic memories.
type SemanticPattern struct {
	Pattern    string
	Frequency  int
	Confidence float64
	Episodes   []string // episode IDs
}

// ConsolidationRun records the result of a memory consolidation cycle.
type ConsolidationRun struct {
	ID                string
	WorkspaceID       string
	EpisodicCount     int
	SemanticExtracted int
	PatternsFound     int
	StartedAt         time.Time
	CompletedAt       time.Time
}

// ConsolidationService extracts semantic patterns from episodic memories.
type ConsolidationService struct {
	mu   sync.RWMutex
	runs map[string][]ConsolidationRun // workspaceID -> runs
}

// NewConsolidationService creates a new ConsolidationService.
func NewConsolidationService() *ConsolidationService {
	return &ConsolidationService{
		runs: make(map[string][]ConsolidationRun),
	}
}

// RunConsolidation processes episodes and extracts semantic patterns.
func (cs *ConsolidationService) RunConsolidation(workspaceID string, episodes []Episode) (*ConsolidationRun, error) {
	startedAt := time.Now()

	patterns := cs.ExtractPatterns(episodes)

	run := ConsolidationRun{
		ID:                newID(),
		WorkspaceID:       workspaceID,
		EpisodicCount:     len(episodes),
		SemanticExtracted: len(patterns),
		PatternsFound:     len(patterns),
		StartedAt:         startedAt,
		CompletedAt:       time.Now(),
	}

	cs.mu.Lock()
	cs.runs[workspaceID] = append(cs.runs[workspaceID], run)
	cs.mu.Unlock()

	return &run, nil
}

// ExtractPatterns finds recurring semantic patterns across episodes.
func (cs *ConsolidationService) ExtractPatterns(episodes []Episode) []SemanticPattern {
	if len(episodes) == 0 {
		return nil
	}

	// Extract tag co-occurrences as patterns.
	tagEpisodes := make(map[string][]string) // tag -> episode IDs
	for _, ep := range episodes {
		for _, tag := range ep.Tags {
			tagLower := strings.ToLower(tag)
			tagEpisodes[tagLower] = append(tagEpisodes[tagLower], ep.ID)
		}
	}

	// Extract word frequency patterns from content.
	wordEpisodes := make(map[string][]string) // word -> episode IDs
	for _, ep := range episodes {
		seen := make(map[string]bool)
		words := strings.Fields(strings.ToLower(ep.Content))
		for _, w := range words {
			w = strings.Trim(w, ".,;:!?\"'()[]{}#")
			if len(w) <= 2 || isStopWord(w) {
				continue
			}
			if !seen[w] {
				wordEpisodes[w] = append(wordEpisodes[w], ep.ID)
				seen[w] = true
			}
		}
	}

	var patterns []SemanticPattern

	// Patterns from tags appearing in multiple episodes.
	for tag, epIDs := range tagEpisodes {
		if len(epIDs) >= 2 {
			confidence := float64(len(epIDs)) / float64(len(episodes))
			patterns = append(patterns, SemanticPattern{
				Pattern:    "tag:" + tag,
				Frequency:  len(epIDs),
				Confidence: confidence,
				Episodes:   epIDs,
			})
		}
	}

	// Patterns from words appearing across multiple episodes.
	for word, epIDs := range wordEpisodes {
		if len(epIDs) >= 2 {
			confidence := float64(len(epIDs)) / float64(len(episodes))
			patterns = append(patterns, SemanticPattern{
				Pattern:    "word:" + word,
				Frequency:  len(epIDs),
				Confidence: confidence,
				Episodes:   epIDs,
			})
		}
	}

	// Sort by frequency descending.
	sort.Slice(patterns, func(i, j int) bool {
		return patterns[i].Frequency > patterns[j].Frequency
	})

	return patterns
}

// isStopWord returns true for common English stop words.
func isStopWord(w string) bool {
	stops := map[string]bool{
		"the": true, "and": true, "for": true, "are": true, "but": true,
		"not": true, "you": true, "all": true, "can": true, "has": true,
		"was": true, "one": true, "our": true, "out": true, "this": true,
		"that": true, "with": true, "from": true, "have": true, "been": true,
		"will": true, "they": true, "which": true, "their": true, "there": true,
		"would": true, "each": true, "about": true, "than": true, "into": true,
	}
	return stops[w]
}
