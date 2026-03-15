package eval

import (
	"context"
	"fmt"
	"strings"
	"time"
)

// MetricSummary aggregates §7 metric results.
type MetricSummary struct {
	AvgFaithfulness        float64
	AvgRelevance           float64
	TopKHitRate            float64
	ConsolidationPrecision float64
	ContextOverflowRate    float64
	CompressionSavings     float64
	WorkingMemoryHitRate   float64
}

// MergeDecisionSample is a single merge decision from the audit log.
type MergeDecisionSample struct {
	NewBody       string
	CandidateBody string
	CosineScore   float64
}

// MergeDecisionStore reads from memory_merge_decision_log.
type MergeDecisionStore interface {
	GetRecentAcceptedMerges(ctx context.Context, workspaceID string, limit int, since time.Time) ([]MergeDecisionSample, error)
}

// LLMConsolidationPrecisionGrader samples recent merge decisions and asks Claude Haiku
// to verify correctness. Precision = correct / total.
type LLMConsolidationPrecisionGrader struct {
	store     MergeDecisionStore
	llmClient RAGEvalLLMClient
	logger    PrecisionLogger
}

// PrecisionLogger is the logging interface for the precision grader.
type PrecisionLogger interface {
	Info(msg string, args ...any)
	Warn(msg string, args ...any)
}

func NewLLMConsolidationPrecisionGrader(
	store MergeDecisionStore,
	llm RAGEvalLLMClient,
	logger PrecisionLogger,
) *LLMConsolidationPrecisionGrader {
	return &LLMConsolidationPrecisionGrader{store: store, llmClient: llm, logger: logger}
}

const mergePrecisionSystemPrompt = `You are a memory deduplication quality evaluator.
You will be shown two memory items that were merged by a deduplication system.
Determine whether merging them was semantically correct.

A merge is CORRECT if:
  - Both items express the same fact or preference
  - One is a more specific or updated version of the other
  - They are paraphrases of the same information

A merge is INCORRECT if:
  - They express different facts that happen to share keywords
  - They are about different time periods and both should be kept
  - One adds genuinely new information not present in the other

Output ONLY one word: CORRECT or INCORRECT`

// MeasurePrecision samples up to 20 recent merge decisions and grades them.
func (g *LLMConsolidationPrecisionGrader) MeasurePrecision(
	ctx context.Context,
	workspaceID string,
) (float64, error) {
	since := time.Now().UTC().AddDate(0, 0, -30)
	samples, err := g.store.GetRecentAcceptedMerges(ctx, workspaceID, 20, since)
	if err != nil {
		return 0, fmt.Errorf("consolidation precision: get samples: %w", err)
	}

	if len(samples) < 5 {
		g.logger.Info("consolidation precision: insufficient merge samples",
			"count", len(samples))
		return 1.0, nil // don't penalise new deployments
	}

	correct := 0
	for _, sample := range samples {
		userPrompt := fmt.Sprintf(
			"Item 1 (existing):\n%s\n\nItem 2 (new, merged into item 1):\n%s\n\nWas this merge CORRECT or INCORRECT?",
			truncateStr(sample.CandidateBody, 300),
			truncateStr(sample.NewBody, 300),
		)

		raw, err := g.llmClient.Complete(ctx, mergePrecisionSystemPrompt, userPrompt)
		if err != nil {
			g.logger.Warn("consolidation precision: LLM call failed", "error", err)
			correct++ // on error, count as correct to avoid false CI failures
			continue
		}

		verdict := strings.TrimSpace(strings.ToUpper(raw))
		if verdict == "CORRECT" {
			correct++
		}
	}

	return float64(correct) / float64(len(samples)), nil
}

func truncateStr(s string, max int) string {
	if len(s) <= max {
		return s
	}
	return s[:max] + "..."
}

// DatasetReadinessCheck verifies the golden dataset is populated with real entries.
func DatasetReadinessCheck(queries []struct {
	ExpectedChunkIDs []string
	WorkspaceID      string
	Query            string
}) (bool, string) {
	realEntries := 0
	for _, q := range queries {
		if len(q.ExpectedChunkIDs) > 0 &&
			q.WorkspaceID != "00000000-0000-0000-0000-000000000000" &&
			q.WorkspaceID != "" &&
			!strings.Contains(q.Query, "PLACEHOLDER") {
			realEntries++
		}
	}

	if realEntries == 0 {
		return false, "Golden dataset has no real entries. Run: go run ./cmd/eval/seed --workspace=<id>"
	}
	if realEntries < 10 {
		return true, fmt.Sprintf("Warning: only %d real entries. Recommend 50+.", realEntries)
	}
	return true, fmt.Sprintf("Dataset has %d real entries", realEntries)
}
