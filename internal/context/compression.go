package contextlayer

import (
	"fmt"
	"strings"
	"sync"
)

// Turn represents a single conversation turn.
type Turn struct {
	Role    string
	Content string
}

// CompressedTurn represents a compressed conversation segment.
type CompressedTurn struct {
	OriginalTurnCount int
	Summary           string
	EntityRefs        []string
}

// ConversationCompressor compresses conversation turns to fit within a token budget.
type ConversationCompressor struct {
	mu sync.Mutex
}

// NewConversationCompressor creates a new ConversationCompressor.
func NewConversationCompressor() *ConversationCompressor {
	return &ConversationCompressor{}
}

// Compress compresses turns to fit within maxTokens, preserving first and last turns.
func (cc *ConversationCompressor) Compress(turns []Turn, maxTokens int) []CompressedTurn {
	cc.mu.Lock()
	defer cc.mu.Unlock()

	if len(turns) == 0 {
		return nil
	}

	if maxTokens <= 0 {
		maxTokens = 2048
	}

	totalTokens := estimateTurnTokens(turns)
	if totalTokens <= maxTokens {
		// No compression needed: return each turn as-is.
		result := make([]CompressedTurn, 0, len(turns))
		for _, turn := range turns {
			result = append(result, CompressedTurn{
				OriginalTurnCount: 1,
				Summary:           fmt.Sprintf("[%s]: %s", turn.Role, turn.Content),
				EntityRefs:        extractEntityRefs(turn.Content),
			})
		}
		return result
	}

	// Keep first and last turns intact; compress middle turns.
	if len(turns) <= 2 {
		result := make([]CompressedTurn, 0, len(turns))
		for _, turn := range turns {
			result = append(result, CompressedTurn{
				OriginalTurnCount: 1,
				Summary:           fmt.Sprintf("[%s]: %s", turn.Role, turn.Content),
				EntityRefs:        extractEntityRefs(turn.Content),
			})
		}
		return result
	}

	result := make([]CompressedTurn, 0, 3)

	// First turn (preserved intact).
	result = append(result, CompressedTurn{
		OriginalTurnCount: 1,
		Summary:           fmt.Sprintf("[%s]: %s", turns[0].Role, turns[0].Content),
		EntityRefs:        extractEntityRefs(turns[0].Content),
	})

	// Compress middle turns.
	middleTurns := turns[1 : len(turns)-1]
	compressed := compressMiddleTurns(middleTurns)
	result = append(result, compressed)

	// Last turn (preserved intact).
	lastTurn := turns[len(turns)-1]
	result = append(result, CompressedTurn{
		OriginalTurnCount: 1,
		Summary:           fmt.Sprintf("[%s]: %s", lastTurn.Role, lastTurn.Content),
		EntityRefs:        extractEntityRefs(lastTurn.Content),
	})

	return result
}

func compressMiddleTurns(turns []Turn) CompressedTurn {
	allEntities := map[string]struct{}{}
	summaryParts := make([]string, 0, len(turns))

	for _, turn := range turns {
		// Summarize each turn to its first sentence or first 50 chars.
		summary := summarizeTurn(turn)
		summaryParts = append(summaryParts, summary)
		for _, entity := range extractEntityRefs(turn.Content) {
			allEntities[entity] = struct{}{}
		}
	}

	entityList := make([]string, 0, len(allEntities))
	for e := range allEntities {
		entityList = append(entityList, e)
	}

	return CompressedTurn{
		OriginalTurnCount: len(turns),
		Summary:           "[compressed] " + strings.Join(summaryParts, "; "),
		EntityRefs:        entityList,
	}
}

func summarizeTurn(turn Turn) string {
	content := strings.TrimSpace(turn.Content)
	// Take first sentence.
	for _, sep := range []string{". ", "! ", "? "} {
		if idx := strings.Index(content, sep); idx > 0 && idx < 80 {
			return content[:idx+1]
		}
	}
	if len(content) > 80 {
		return content[:80] + "..."
	}
	return content
}

// extractEntityRefs extracts capitalized words as entity references.
func extractEntityRefs(text string) []string {
	words := strings.Fields(text)
	seen := map[string]struct{}{}
	entities := []string{}
	for _, w := range words {
		clean := strings.Trim(w, ".,:;!?()[]{}\"'")
		if clean == "" {
			continue
		}
		if len(clean) >= 2 && clean[0] >= 'A' && clean[0] <= 'Z' {
			lower := strings.ToLower(clean)
			if _, ok := seen[lower]; !ok {
				seen[lower] = struct{}{}
				entities = append(entities, clean)
			}
		}
	}
	return entities
}

func estimateTurnTokens(turns []Turn) int {
	total := 0
	for _, turn := range turns {
		// Approximate: 1 token per 4 characters.
		total += len(turn.Content) / 4
	}
	return total
}
