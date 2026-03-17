// citations.go implements CitationExtractor, which maps response claims to the
// source document chunks that support them (plan 06 — Citation Attribution).

package rag

import (
	"context"
	"encoding/json"
	"fmt"
)

// Citation represents a single claim-to-source mapping.
type Citation struct {
	ClaimText    string  `json:"claim_text"`
	ChunkID      string  `json:"chunk_id"`
	Collection   string  `json:"collection"`
	SourceURL    string  `json:"source_url"`
	Confidence   float64 `json:"confidence"`
	ChunkSnippet string  `json:"chunk_snippet"`
}

// CitationLLM is the LLM interface for citation extraction.
type CitationLLM interface {
	Complete(ctx context.Context, system, user string) (string, error)
}

// CitationExtractor maps response claims to source chunks.
type CitationExtractor struct {
	llm    CitationLLM
	logger Logger
}

// NewCitationExtractor creates a new citation extractor.
func NewCitationExtractor(llm CitationLLM, logger Logger) *CitationExtractor {
	return &CitationExtractor{llm: llm, logger: logger}
}

// Extract identifies which claims in the response are supported by which chunks.
func (e *CitationExtractor) Extract(ctx context.Context, response string, chunks []RetrievalResult) ([]Citation, error) {
	if len(chunks) == 0 {
		return nil, nil
	}

	chunkByID := make(map[string]RetrievalResult, len(chunks))
	for _, ch := range chunks {
		chunkByID[ch.ChunkID] = ch
	}

	// Build chunk text for prompt.
	var chunkText string
	for _, ch := range chunks {
		chunkText += fmt.Sprintf("ChunkID: %s\nSnippet: %s\n---\n", ch.ChunkID, limitStr(ch.Snippet, 300))
	}

	systemPrompt := "You are a citation extractor. Map response claims to chunk IDs. " +
		"Output a JSON array of objects: [{\"claim_text\": string, \"chunk_id\": string, \"confidence\": float}]. " +
		"Maximum 5 citations. Skip conversational phrases, greetings, and meta-statements. " +
		"Output ONLY the raw JSON array — no markdown, no code fences, no preamble."

	userMsg := fmt.Sprintf("Assistant Response:\n%s\n\nRetrieved Chunks:\n%s", response, chunkText)

	llmOutput, err := e.llm.Complete(ctx, systemPrompt, userMsg)
	if err != nil {
		if e.logger != nil {
			e.logger.Error("citation extraction LLM call failed", "error", err)
		}
		return nil, nil
	}

	var rawCitations []struct {
		ClaimText  string  `json:"claim_text"`
		ChunkID    string  `json:"chunk_id"`
		Confidence float64 `json:"confidence"`
	}
	if err := json.Unmarshal([]byte(llmOutput), &rawCitations); err != nil {
		if e.logger != nil {
			e.logger.Error("citation JSON parse failed", "error", err, "raw", llmOutput)
		}
		return nil, nil
	}

	// Hallucination guard: skip chunk IDs not in the provided chunks.
	var citations []Citation
	for _, raw := range rawCitations {
		chunkData, exists := chunkByID[raw.ChunkID]
		if !exists {
			if e.logger != nil {
				e.logger.Warn("citation extractor skipping hallucinated chunk ID", "chunk_id", raw.ChunkID)
			}
			continue
		}
		citations = append(citations, Citation{
			ClaimText:    raw.ClaimText,
			ChunkID:      raw.ChunkID,
			Collection:   chunkData.Collection,
			SourceURL:    chunkData.Source,
			Confidence:   raw.Confidence,
			ChunkSnippet: limitStr(chunkData.Snippet, 200),
		})
	}

	return citations, nil
}

// limitStr truncates a string to max characters.
func limitStr(s string, max int) string {
	if len(s) <= max {
		return s
	}
	return s[:max]
}
