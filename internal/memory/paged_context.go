package memory

import (
	"context"
	"fmt"
	"log"
	"strings"

	"github.com/brevio/brevio/internal/rag"
)

const (
	pageOutThreshold = 0.85 // page out when context is 85% full
	archivalTopK     = 5    // retrieve top-5 archival memories per turn
)

// ConversationTurn represents a single message exchange in a session.
type ConversationTurn struct {
	Index         int
	Role          string // "user" | "assistant"
	Content       string
	TokenEstimate int
}

// ArchivalMemory is a consolidated summary of evicted conversation turns.
type ArchivalMemory struct {
	ID             string
	SessionID      string
	WorkspaceID    string
	Content        string
	Embedding      []float32
	PageGeneration int
	SourceTurns    []int
}

// ArchivalMemoryRepository persists and retrieves archived context.
type ArchivalMemoryRepository interface {
	StoreMemory(ctx context.Context, mem ArchivalMemory) error
	SearchByEmbedding(ctx context.Context, sessionID, workspaceID string, embedding []float32, topK int) ([]ArchivalMemory, error)
}

// SimpleTokenCounter estimates token count as len/4 (rough byte-to-token ratio).
type SimpleTokenCounter struct{}

// Count returns an approximate token count.
func (s *SimpleTokenCounter) Count(text string) int {
	return len(text) / 4
}

// PagedContextManager manages infinite context via main + archival storage.
type PagedContextManager struct {
	sessionID      string
	workspaceID    string
	modelWindow    int // max tokens for the model
	mainContext    []ConversationTurn
	pageGeneration int
	archivalRepo   ArchivalMemoryRepository
	embedder       rag.EmbeddingProvider
	tokenCounter   *SimpleTokenCounter
}

// NewPagedContextManager creates a paged context manager.
func NewPagedContextManager(
	sessionID, workspaceID string,
	modelWindow int,
	archivalRepo ArchivalMemoryRepository,
	embedder rag.EmbeddingProvider,
) *PagedContextManager {
	if modelWindow <= 0 {
		modelWindow = 128000
	}
	return &PagedContextManager{
		sessionID:    sessionID,
		workspaceID:  workspaceID,
		modelWindow:  modelWindow,
		archivalRepo: archivalRepo,
		embedder:     embedder,
		tokenCounter: &SimpleTokenCounter{},
	}
}

// AddTurn adds a conversation turn and triggers page-out if needed.
func (m *PagedContextManager) AddTurn(ctx context.Context, turn ConversationTurn) error {
	turn.TokenEstimate = m.tokenCounter.Count(turn.Content)
	m.mainContext = append(m.mainContext, turn)

	totalTokens := 0
	for _, t := range m.mainContext {
		totalTokens += t.TokenEstimate
	}

	if float64(totalTokens)/float64(m.modelWindow) > pageOutThreshold {
		return m.pageOut(ctx)
	}
	return nil
}

// pageOut evicts the oldest half of main context to archival storage.
func (m *PagedContextManager) pageOut(ctx context.Context) error {
	evictCount := len(m.mainContext) / 2
	if evictCount == 0 {
		return nil
	}

	toEvict := m.mainContext[:evictCount]

	// Build consolidated text from evicted turns.
	var sb strings.Builder
	sourceTurns := make([]int, evictCount)
	for i, t := range toEvict {
		sb.WriteString(fmt.Sprintf("%s: %s\n", t.Role, t.Content))
		sourceTurns[i] = t.Index
	}

	// Summarize using a simple concatenation (RAPTOR consolidation can be plugged in).
	consolidatedText := sb.String()
	if len(consolidatedText) > 2000 {
		consolidatedText = consolidatedText[:2000] + "..."
	}

	// Embed the consolidated text.
	var embedding []float32
	if m.embedder != nil {
		embeddings, err := m.embedder.Embed(ctx, []string{consolidatedText})
		if err == nil && len(embeddings) > 0 {
			embedding = embeddings[0]
		}
	}
	if embedding == nil {
		embedding = make([]float32, 1536)
	}

	// Store in archival.
	if m.archivalRepo != nil {
		err := m.archivalRepo.StoreMemory(ctx, ArchivalMemory{
			SessionID:      m.sessionID,
			WorkspaceID:    m.workspaceID,
			Content:        consolidatedText,
			Embedding:      embedding,
			PageGeneration: m.pageGeneration,
			SourceTurns:    sourceTurns,
		})
		if err != nil {
			return fmt.Errorf("archival store: %w", err)
		}
	}

	// Remove evicted turns.
	m.mainContext = m.mainContext[evictCount:]
	m.pageGeneration++
	log.Printf("[paged_context] paged_out turns_evicted=%d generation=%d", evictCount, m.pageGeneration)
	return nil
}

// PageIn retrieves relevant archival memories for the current intent.
func (m *PagedContextManager) PageIn(ctx context.Context, currentIntent string) ([]ArchivalMemory, error) {
	if m.archivalRepo == nil || m.embedder == nil {
		return nil, nil
	}
	embeddings, err := m.embedder.Embed(ctx, []string{currentIntent})
	if err != nil || len(embeddings) == 0 {
		return nil, err
	}
	return m.archivalRepo.SearchByEmbedding(ctx, m.sessionID, m.workspaceID, embeddings[0], archivalTopK)
}

// BuildContextWithArchival returns main context and relevant archival memories.
func (m *PagedContextManager) BuildContextWithArchival(ctx context.Context, currentIntent string) ([]ConversationTurn, []ArchivalMemory, error) {
	archival, err := m.PageIn(ctx, currentIntent)
	if err != nil {
		return m.mainContext, nil, err
	}
	return m.mainContext, archival, nil
}

// MainContextFusion formats archival memories for injection into a system prompt.
func (m *PagedContextManager) MainContextFusion(archivalMemories []ArchivalMemory) string {
	if len(archivalMemories) == 0 {
		return ""
	}
	var sb strings.Builder
	sb.WriteString("[Recalled from previous context]:\n")
	for _, mem := range archivalMemories {
		content := mem.Content
		if len(content) > 500 {
			content = content[:500] + "..."
		}
		sb.WriteString(fmt.Sprintf("- %s\n", content))
	}
	return sb.String()
}

// MainContextSize returns the total estimated token count of the main context.
func (m *PagedContextManager) MainContextSize() int {
	total := 0
	for _, t := range m.mainContext {
		total += t.TokenEstimate
	}
	return total
}

// PageGeneration returns the current page generation counter.
func (m *PagedContextManager) PageGeneration() int {
	return m.pageGeneration
}
