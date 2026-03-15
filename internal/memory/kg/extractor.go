package kg

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
)

// ExtractorLLMClient is the minimal LLM interface for extraction.
type ExtractorLLMClient interface {
	Complete(ctx context.Context, system, user string) (string, error)
}

// LLMExtractor extracts knowledge graph triples from conversation text.
type LLMExtractor struct {
	llm    ExtractorLLMClient
	logger Logger
}

func NewLLMExtractor(llm ExtractorLLMClient, logger Logger) *LLMExtractor {
	return &LLMExtractor{llm: llm, logger: logger}
}

const extractionSystemPrompt = `You are a knowledge graph extraction engine for an AI executive assistant.

Extract factual entity relationships from conversation text as structured triples.

EXTRACTION RULES:
1. Only extract facts with confidence > 0.70
2. Subject and object must be specific named entities (person, company, project, role, date, amount, product)
3. Predicate must be a normalized lowercase verb phrase with underscores
4. DO NOT extract opinions, hypotheticals, vague statements, or questions
5. DO NOT create self-referential triples (subject == object)

OUTPUT FORMAT — respond with ONLY a valid JSON array:
[{"subject":"Alice Chen","predicate":"reports_to","object":"Bob Smith","subject_type":"person","object_type":"person","confidence":0.95}]

If NO high-confidence triples can be extracted, return exactly: []`

// Extract processes a single conversation turn and returns validated triples.
func (e *LLMExtractor) Extract(ctx context.Context, req ExtractionRequest) ([]ExtractedTriple, error) {
	if len(strings.TrimSpace(req.Content)) < 15 {
		return nil, nil
	}

	userPrompt := fmt.Sprintf("[%s]:\n%s", req.Role, req.Content)

	raw, err := e.llm.Complete(ctx, extractionSystemPrompt, userPrompt)
	if err != nil {
		e.logger.Warn("kg_extract: LLM call failed (non-fatal)",
			"turn_id", req.TurnID, "error", err)
		return nil, nil
	}

	raw = strings.TrimSpace(raw)
	if raw == "" || raw == "[]" {
		return nil, nil
	}
	raw = strings.TrimPrefix(strings.TrimPrefix(raw, "```json"), "```")
	raw = strings.TrimSuffix(raw, "```")
	raw = strings.TrimSpace(raw)

	var triples []ExtractedTriple
	if err := json.Unmarshal([]byte(raw), &triples); err != nil {
		e.logger.Warn("kg_extract: JSON parse failed (non-fatal)",
			"turn_id", req.TurnID, "error", err)
		return nil, nil
	}

	valid := make([]ExtractedTriple, 0, len(triples))
	for _, t := range triples {
		if strings.TrimSpace(t.Subject) == "" ||
			strings.TrimSpace(t.Predicate) == "" ||
			strings.TrimSpace(t.Object) == "" {
			continue
		}
		if strings.EqualFold(strings.TrimSpace(t.Subject), strings.TrimSpace(t.Object)) {
			continue
		}
		if t.Confidence < 0.70 {
			continue
		}
		t.Predicate = strings.ToLower(strings.ReplaceAll(
			strings.TrimSpace(t.Predicate), " ", "_"))
		valid = append(valid, t)
	}

	return valid, nil
}
