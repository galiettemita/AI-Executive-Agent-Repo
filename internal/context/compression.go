package contextlayer

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"sync"
	"time"
	"unicode"
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

// extractEntityRefs extracts probable named entities from text.
// Requires 2+ consecutive capitalized words (multi-word entities),
// OR single ALL-CAPS tokens of 2+ chars (acronyms: CEO, Q3, API).
func extractEntityRefs(text string) []string {
	words := strings.Fields(text)
	var entities []string
	var multiWordBuffer []string

	flush := func() {
		if len(multiWordBuffer) >= 2 {
			entities = append(entities, strings.Join(multiWordBuffer, " "))
		} else if len(multiWordBuffer) == 1 {
			w := multiWordBuffer[0]
			if w == strings.ToUpper(w) && len(w) >= 2 {
				entities = append(entities, w)
			}
		}
		multiWordBuffer = multiWordBuffer[:0]
	}

	for i, raw := range words {
		word := strings.TrimRight(raw, ".,!?;:()")
		if len(word) == 0 {
			flush()
			continue
		}

		isFirstOfSentence := i == 0 || (i > 0 && strings.HasSuffix(
			strings.TrimRight(words[i-1], " "), "."))
		isCapitalized := len(word) > 0 && unicode.IsUpper(rune(word[0]))
		isAllCaps := word == strings.ToUpper(word) && len(word) >= 2

		if (isCapitalized && !isFirstOfSentence) || isAllCaps {
			multiWordBuffer = append(multiWordBuffer, word)
		} else {
			flush()
		}
	}
	flush()

	seen := make(map[string]bool)
	deduped := make([]string, 0, len(entities))
	for _, e := range entities {
		if !seen[e] {
			seen[e] = true
			deduped = append(deduped, e)
		}
	}
	return deduped
}

func estimateTurnTokens(turns []Turn) int {
	total := 0
	for _, turn := range turns {
		total += len(turn.Content) / 4
	}
	return total
}

// TOKEN_BUDGET_TRIGGER is the estimated token count that triggers LLM compression.
const TOKEN_BUDGET_TRIGGER = 2000

// ShouldCompress returns true when middle turns exceed the token budget.
func ShouldCompress(turns []Turn) bool {
	if len(turns) == 0 {
		return false
	}
	return estimateTurnTokens(turns) > TOKEN_BUDGET_TRIGGER
}

// CompressedSegment is the structured output from LLM-based compression.
type CompressedSegment struct {
	Summary           string    `json:"summary"`
	KeyDecisions      []string  `json:"key_decisions"`
	ActionItems       []string  `json:"action_items"`
	Entities          []string  `json:"entities"`
	OpenQuestions     []string  `json:"open_questions"`
	OriginalTurnCount int       `json:"original_turn_count"`
	OriginalTokenEst  int       `json:"original_token_est"`
	CompressedAt      time.Time `json:"compressed_at"`
}

// FormatForContext renders the CompressedSegment as a compact string for context injection.
func (c *CompressedSegment) FormatForContext() string {
	var b strings.Builder
	b.WriteString(fmt.Sprintf("[Summary of %d earlier turns]\n", c.OriginalTurnCount))
	b.WriteString(c.Summary)
	if len(c.KeyDecisions) > 0 {
		b.WriteString("\nDecisions: " + strings.Join(c.KeyDecisions, " | "))
	}
	if len(c.ActionItems) > 0 {
		b.WriteString("\nActions: " + strings.Join(c.ActionItems, " | "))
	}
	if len(c.OpenQuestions) > 0 {
		b.WriteString("\nOpen: " + strings.Join(c.OpenQuestions, " | "))
	}
	return b.String()
}

// CompressionArtifactRecord maps to the compression_artifacts DB table.
type CompressionArtifactRecord struct {
	WorkspaceID        string            `db:"workspace_id"`
	ConversationID     string            `db:"conversation_id"`
	TurnRangeStart     int               `db:"turn_range_start"`
	TurnRangeEnd       int               `db:"turn_range_end"`
	OriginalTokenEst   int               `db:"original_token_count"`
	CompressedTokenEst int               `db:"compressed_token_count"`
	TurnCount          int               `db:"turn_count"`
	CompressedContent  CompressedSegment `db:"compressed_content"`
	CreatedAt          time.Time         `db:"created_at"`
}

// CompressionArtifactStore persists compression artifacts.
type CompressionArtifactStore interface {
	Store(ctx context.Context, record CompressionArtifactRecord) error
}

// CompressionLLMClient is the minimal interface for compression LLM calls.
type CompressionLLMClient interface {
	Complete(ctx context.Context, system, user string) (string, error)
}

// CompressionLogger is the logging interface for compression.
type CompressionLogger interface {
	Info(msg string, args ...any)
	Warn(msg string, args ...any)
	Error(msg string, args ...any)
}

// LLMCompressor replaces heuristic truncation with Claude Haiku summarization.
type LLMCompressor struct {
	llm      CompressionLLMClient
	store    CompressionArtifactStore
	fallback *ConversationCompressor
	logger   CompressionLogger
}

func NewLLMCompressor(
	llm CompressionLLMClient,
	store CompressionArtifactStore,
	fallback *ConversationCompressor,
	logger CompressionLogger,
) (*LLMCompressor, error) {
	if fallback == nil {
		return nil, fmt.Errorf("context.NewLLMCompressor: ConversationCompressor must not be nil")
	}
	return &LLMCompressor{llm: llm, store: store, fallback: fallback, logger: logger}, nil
}

const compressionSystemPrompt = `You are a memory compressor for an AI executive assistant.
Compress the provided conversation segment into a structured JSON summary.

STRICT PRESERVATION RULES:
- Every explicit decision or agreement
- Every action item, commitment, or task
- Every named entity with its role: person names, company names, project names, dates, amounts
- All unresolved questions

EXCLUSION RULES:
- Greetings, pleasantries, filler
- Repeated or redundant information
- Vague or non-actionable statements

OUTPUT FORMAT — respond with ONLY valid JSON:
{
  "summary": "2-4 sentence prose summary in past tense",
  "key_decisions": ["decision 1"],
  "action_items": ["action 1"],
  "entities": ["Alice Chen (CEO)", "Project Falcon"],
  "open_questions": ["question 1"]
}

ALL arrays must be present. Use [] for empty categories.`

// CompressTurns compresses middle turns using Claude Haiku.
func (c *LLMCompressor) CompressTurns(
	ctx context.Context,
	workspaceID, conversationID string,
	turns []Turn,
	turnStartIdx, turnEndIdx int,
) (string, error) {
	if len(turns) == 0 {
		return "", nil
	}

	estimatedTokens := estimateTurnTokens(turns)
	if estimatedTokens < TOKEN_BUDGET_TRIGGER {
		return joinTurns(turns), nil
	}

	var transcript strings.Builder
	for i, t := range turns {
		transcript.WriteString(fmt.Sprintf("[%s, turn %d]: %s\n\n",
			t.Role, turnStartIdx+i+1, t.Content))
	}

	userPrompt := fmt.Sprintf(
		"Compress this conversation segment (%d turns, ~%d tokens):\n\n%s",
		len(turns), estimatedTokens, transcript.String(),
	)

	raw, err := c.llm.Complete(ctx, compressionSystemPrompt, userPrompt)
	if err != nil {
		c.logger.Warn("llm_compress: LLM call failed; using heuristic fallback",
			"conversation_id", conversationID, "turns", len(turns), "error", err)
		return heuristicCompressMiddle(turns), nil
	}

	segment, err := parseCompressedSegment(raw, len(turns), estimatedTokens)
	if err != nil {
		c.logger.Warn("llm_compress: parse failed; using heuristic fallback",
			"error", err)
		return heuristicCompressMiddle(turns), nil
	}

	// Persist artifact (best-effort)
	if c.store != nil {
		go func() {
			storeCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			record := CompressionArtifactRecord{
				WorkspaceID:        workspaceID,
				ConversationID:     conversationID,
				TurnRangeStart:     turnStartIdx,
				TurnRangeEnd:       turnEndIdx,
				OriginalTokenEst:   estimatedTokens,
				CompressedTokenEst: len(segment.FormatForContext()) / 4,
				TurnCount:          len(turns),
				CompressedContent:  *segment,
				CreatedAt:          time.Now().UTC(),
			}
			if err := c.store.Store(storeCtx, record); err != nil {
				c.logger.Error("llm_compress: artifact persistence failed",
					"conversation_id", conversationID, "error", err)
			}
		}()
	}

	return segment.FormatForContext(), nil
}

func parseCompressedSegment(raw string, turnCount, tokenEst int) (*CompressedSegment, error) {
	raw = strings.TrimSpace(raw)
	raw = strings.TrimPrefix(raw, "```json")
	raw = strings.TrimPrefix(raw, "```")
	raw = strings.TrimSuffix(raw, "```")
	raw = strings.TrimSpace(raw)

	var seg CompressedSegment
	if err := json.Unmarshal([]byte(raw), &seg); err != nil {
		return nil, fmt.Errorf("json unmarshal: %w", err)
	}
	if strings.TrimSpace(seg.Summary) == "" {
		return nil, fmt.Errorf("summary is empty")
	}
	if seg.KeyDecisions == nil {
		seg.KeyDecisions = []string{}
	}
	if seg.ActionItems == nil {
		seg.ActionItems = []string{}
	}
	if seg.Entities == nil {
		seg.Entities = []string{}
	}
	if seg.OpenQuestions == nil {
		seg.OpenQuestions = []string{}
	}

	seg.OriginalTurnCount = turnCount
	seg.OriginalTokenEst = tokenEst
	seg.CompressedAt = time.Now().UTC()
	return &seg, nil
}

func heuristicCompressMiddle(turns []Turn) string {
	ct := compressMiddleTurns(turns)
	return ct.Summary
}

func joinTurns(turns []Turn) string {
	parts := make([]string, len(turns))
	for i, t := range turns {
		parts[i] = fmt.Sprintf("[%s]: %s", t.Role, t.Content)
	}
	return strings.Join(parts, "\n")
}

// DynamicMemoryTokenBudget adjusts memory slot token budget based on retrieval quality.
func DynamicMemoryTokenBudget(avgScore float64, baseBudget int) int {
	var multiplier float64
	switch {
	case avgScore >= 0.80:
		multiplier = 1.30
	case avgScore >= 0.65:
		multiplier = 1.00
	case avgScore >= 0.45:
		multiplier = 0.75
	default:
		multiplier = 0.50
	}

	budget := int(float64(baseBudget) * multiplier)
	if budget < 500 {
		budget = 500
	}
	if budget > baseBudget*3/2 {
		budget = baseBudget * 3 / 2
	}
	return budget
}
