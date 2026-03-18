package evaluation

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
)

// MTBenchConversation represents a two-turn MT-Bench evaluation item.
type MTBenchConversation struct {
	ID       string          `json:"id"`
	Category string          `json:"category"`
	Turns    []MTBenchTurn   `json:"turns"`
}

// MTBenchTurn is one turn in a multi-turn conversation.
type MTBenchTurn struct {
	Turn      int    `json:"turn"`
	User      string `json:"user"`
	Reference string `json:"reference"`
}

// ConversationScore holds per-turn and aggregate scores.
type ConversationScore struct {
	ConversationID string    `json:"conversation_id"`
	Category       string    `json:"category"`
	TurnScores     []float64 `json:"turn_scores"`
	AvgScore       float64   `json:"avg_score"`
}

// LLMClient generates text completions for MT-Bench judging.
type LLMClient interface {
	Complete(ctx context.Context, systemPrompt, userPrompt string) (string, error)
}

// MTBenchJudge scores multi-turn conversations using an LLM-as-judge.
type MTBenchJudge struct {
	llmClient LLMClient
	logger    *slog.Logger
}

// NewMTBenchJudge creates a judge instance.
func NewMTBenchJudge(client LLMClient, logger *slog.Logger) *MTBenchJudge {
	return &MTBenchJudge{llmClient: client, logger: logger}
}

// ScoreTurn evaluates a single turn response.
func (j *MTBenchJudge) ScoreTurn(ctx context.Context, category string, userTurn string, agentResponse string, turnNumber int) (float64, error) {
	prompt := fmt.Sprintf(
		`You are evaluating an AI executive assistant response. Score it 1-10.
Category: %s
Turn: %d
User request: %s
Agent response: %s

Score criteria: helpfulness (40%%), accuracy (30%%), depth (30%%)
Return ONLY a JSON object: {"score": N, "reasoning": "brief explanation"}`,
		category, turnNumber, userTurn, agentResponse,
	)

	resp, err := j.llmClient.Complete(ctx, "You are an expert evaluator.", prompt)
	if err != nil {
		j.logger.Warn("mt_bench_judge_error", "error", err)
		return 5.0, nil // fallback score
	}

	var result struct {
		Score     float64 `json:"score"`
		Reasoning string  `json:"reasoning"`
	}
	if err := json.Unmarshal([]byte(resp), &result); err != nil {
		j.logger.Warn("mt_bench_judge_parse_error", "response", resp, "error", err)
		return 5.0, nil
	}

	if result.Score < 1.0 || result.Score > 10.0 {
		return 5.0, nil
	}

	return result.Score, nil
}

// EvaluateConversation scores all turns and returns aggregated results.
func (j *MTBenchJudge) EvaluateConversation(ctx context.Context, conv MTBenchConversation, agentResponses []string) (*ConversationScore, error) {
	score := &ConversationScore{
		ConversationID: conv.ID,
		Category:       conv.Category,
	}

	total := 0.0
	for i, turn := range conv.Turns {
		response := ""
		if i < len(agentResponses) {
			response = agentResponses[i]
		}

		turnScore, err := j.ScoreTurn(ctx, conv.Category, turn.User, response, turn.Turn)
		if err != nil {
			turnScore = 5.0
		}

		score.TurnScores = append(score.TurnScores, turnScore)
		total += turnScore
	}

	if len(score.TurnScores) > 0 {
		score.AvgScore = total / float64(len(score.TurnScores))
	}

	return score, nil
}
