package memory

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
)

// ContradictionDetector identifies when a new memory write supersedes an existing one.
type ContradictionDetector struct {
	embedder   ContradictionEmbedder
	searchRepo ContradictionSearcher
	llm        ContradictionLLMClient
	updateRepo ContradictionUpdater
	logger     ContradictionLogger
}

// ContradictionEmbedder provides embeddings for the detector.
type ContradictionEmbedder interface {
	Embed(ctx context.Context, texts []string) ([][]float32, error)
}

// ContradictionSearcher finds similar existing items.
type ContradictionSearcher interface {
	SearchByVector(ctx context.Context, workspaceID string, vec []float32, limit int) ([]Item, error)
}

// ContradictionUpdater marks items as contradicted.
type ContradictionUpdater interface {
	MarkContradicted(ctx context.Context, itemID string, confidence float64) error
	SetContradictsItemID(ctx context.Context, newItemID, supersededItemID string) error
}

// ContradictionLLMClient is the LLM interface for contradiction judgment.
type ContradictionLLMClient interface {
	Complete(ctx context.Context, system, user string) (string, error)
}

// ContradictionLogger is the logging interface for the detector.
type ContradictionLogger interface {
	Info(msg string, args ...any)
	Warn(msg string, args ...any)
	Error(msg string, args ...any)
}

const contradictionSystemPrompt = `You are a contradiction detector for an AI memory system.

You will be shown two memory items about the same user.
Determine whether the NEW item CONTRADICTS the EXISTING item.

A CONTRADICTION means the new item expresses a DIFFERENT or UPDATED value for the SAME attribute.
NOT a contradiction: items that ADD new info, CONFIRM the old one, or are about different topics.

Output JSON only:
{"contradicts": true|false, "confidence": 0.0-1.0, "reason": "one sentence"}`

// ContradictionResult is the output of one LLM judgment call.
type ContradictionResult struct {
	Contradicts bool    `json:"contradicts"`
	Confidence  float64 `json:"confidence"`
	Reason      string  `json:"reason"`
}

func NewContradictionDetector(
	embedder ContradictionEmbedder,
	searchRepo ContradictionSearcher,
	llm ContradictionLLMClient,
	updateRepo ContradictionUpdater,
	logger ContradictionLogger,
) *ContradictionDetector {
	return &ContradictionDetector{
		embedder:   embedder,
		searchRepo: searchRepo,
		llm:        llm,
		updateRepo: updateRepo,
		logger:     logger,
	}
}

const (
	contradictionSimilarityThreshold = 0.80
	contradictionLLMThreshold        = 0.75
	contradictionCandidateLimit      = 5
)

// DetectAndMark runs contradiction detection for a newly written item.
// Designed to be called in a goroutine — non-blocking, recovers panics.
func (d *ContradictionDetector) DetectAndMark(ctx context.Context, newItem Item) {
	defer func() {
		if r := recover(); r != nil {
			d.logger.Error("contradiction_detector: panic recovered",
				"item_id", newItem.ID, "panic", fmt.Sprintf("%v", r))
		}
	}()

	if newItem.Body == "" || newItem.WorkspaceID == "" {
		return
	}

	embeddings, err := d.embedder.Embed(ctx, []string{newItem.Body})
	if err != nil || len(embeddings) == 0 {
		d.logger.Warn("contradiction_detector: embed failed",
			"item_id", newItem.ID, "error", err)
		return
	}

	candidates, err := d.searchRepo.SearchByVector(
		ctx, newItem.WorkspaceID, embeddings[0], contradictionCandidateLimit)
	if err != nil {
		d.logger.Warn("contradiction_detector: search failed", "error", err)
		return
	}

	newItemID := newItem.ID.String()
	for _, candidate := range candidates {
		candidateID := candidate.ID.String()
		if candidateID == newItemID {
			continue
		}
		if candidate.IsContradicted {
			continue
		}

		sim := candidate.RelevanceScore
		if sim < contradictionSimilarityThreshold || sim >= 0.92 {
			continue
		}

		result, err := d.judgeContradiction(ctx, candidate.Body, newItem.Body)
		if err != nil {
			d.logger.Warn("contradiction_detector: LLM judge failed",
				"candidate_id", candidateID, "error", err)
			continue
		}

		if result.Contradicts && result.Confidence >= contradictionLLMThreshold {
			d.logger.Info("contradiction_detector: contradiction found",
				"new_item_id", newItemID,
				"superseded_id", candidateID,
				"confidence", result.Confidence,
				"reason", result.Reason)

			if err := d.updateRepo.MarkContradicted(ctx, candidateID, result.Confidence); err != nil {
				d.logger.Error("contradiction_detector: mark contradicted failed",
					"candidate_id", candidateID, "error", err)
				continue
			}
			if err := d.updateRepo.SetContradictsItemID(ctx, newItemID, candidateID); err != nil {
				d.logger.Error("contradiction_detector: set contradicts_item_id failed",
					"new_item_id", newItemID, "error", err)
			}
		}
	}
}

func (d *ContradictionDetector) judgeContradiction(
	ctx context.Context,
	existingBody, newBody string,
) (*ContradictionResult, error) {
	userPrompt := fmt.Sprintf(
		"EXISTING item:\n%s\n\nNEW item:\n%s",
		truncContradiction(existingBody, 300),
		truncContradiction(newBody, 300),
	)

	raw, err := d.llm.Complete(ctx, contradictionSystemPrompt, userPrompt)
	if err != nil {
		return nil, fmt.Errorf("LLM call: %w", err)
	}

	raw = strings.TrimSpace(raw)
	raw = strings.TrimPrefix(strings.TrimPrefix(raw, "```json"), "```")
	raw = strings.TrimSuffix(raw, "```")
	raw = strings.TrimSpace(raw)

	var result ContradictionResult
	if err := json.Unmarshal([]byte(raw), &result); err != nil {
		return nil, fmt.Errorf("parse: %w", err)
	}
	return &result, nil
}

func truncContradiction(s string, max int) string {
	if len(s) <= max {
		return s
	}
	return s[:max] + "..."
}
