package rag

import (
	"context"
	"fmt"
	"strings"
	"time"
)

// DocumentMeta holds source information used to build context headers for embedding.
type DocumentMeta struct {
	Title       string
	SourceType  string
	Date        time.Time
	Section     string
	AuthorName  string
	WorkspaceID string
}

// ChunkEnricher prepends document context to chunk text before embedding.
type ChunkEnricher interface {
	Enrich(ctx context.Context, meta DocumentMeta, chunkText string) (string, error)
}

// MetadataChunkEnricher uses document metadata to build a structured header. Zero LLM calls.
type MetadataChunkEnricher struct{}

func NewMetadataChunkEnricher() *MetadataChunkEnricher { return &MetadataChunkEnricher{} }

func (e *MetadataChunkEnricher) Enrich(_ context.Context, meta DocumentMeta, text string) (string, error) {
	parts := make([]string, 0, 5)
	if meta.Title != "" {
		parts = append(parts, "Source: "+meta.Title)
	}
	if meta.SourceType != "" {
		parts = append(parts, "Type: "+meta.SourceType)
	}
	if !meta.Date.IsZero() {
		parts = append(parts, "Date: "+meta.Date.Format("2006-01-02"))
	}
	if meta.Section != "" {
		parts = append(parts, "Section: "+meta.Section)
	}
	if meta.AuthorName != "" {
		parts = append(parts, "Author: "+meta.AuthorName)
	}

	if len(parts) == 0 {
		return text, nil
	}
	return "[" + strings.Join(parts, " | ") + "]\n" + text, nil
}

// EnricherLLMClient is the minimal LLM interface needed by LLMChunkEnricher.
type EnricherLLMClient interface {
	Complete(ctx context.Context, systemPrompt, userPrompt string) (string, error)
}

// LLMChunkEnricher generates a one-sentence description via Claude Haiku.
// Falls back to MetadataChunkEnricher on LLM error.
type LLMChunkEnricher struct {
	llm      EnricherLLMClient
	fallback *MetadataChunkEnricher
}

func NewLLMChunkEnricher(llm EnricherLLMClient) *LLMChunkEnricher {
	return &LLMChunkEnricher{llm: llm, fallback: NewMetadataChunkEnricher()}
}

const llmEnricherSystemPrompt = `You are a retrieval optimization assistant.
Write exactly ONE sentence (max 20 words) describing what the following text chunk
is about within the context of its source document.
Output ONLY the sentence. No punctuation at end. No preamble.`

func (e *LLMChunkEnricher) Enrich(ctx context.Context, meta DocumentMeta, text string) (string, error) {
	preview := text
	if len(preview) > 400 {
		preview = preview[:400] + "…"
	}

	userPrompt := fmt.Sprintf("Document: %s | Date: %s | Section: %s\n\nChunk:\n%s",
		meta.Title, meta.Date.Format("2006-01-02"), meta.Section, preview)

	desc, err := e.llm.Complete(ctx, llmEnricherSystemPrompt, userPrompt)
	if err != nil || strings.TrimSpace(desc) == "" {
		return e.fallback.Enrich(ctx, meta, text)
	}

	metaEnriched, _ := e.fallback.Enrich(ctx, meta, text)
	return "[Context: " + strings.TrimSpace(desc) + "]\n" + metaEnriched, nil
}

// PassthroughChunkEnricher returns text unchanged.
type PassthroughChunkEnricher struct{}

func NewPassthroughChunkEnricher() *PassthroughChunkEnricher { return &PassthroughChunkEnricher{} }

func (e *PassthroughChunkEnricher) Enrich(_ context.Context, _ DocumentMeta, text string) (string, error) {
	return text, nil
}
