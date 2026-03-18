package learning

import (
	"context"
	"fmt"
	"log/slog"
	"strings"

	"github.com/google/uuid"
	learndpo "github.com/brevio/brevio/internal/learning/dpo"
	"github.com/brevio/brevio/internal/learning/ppo"
)

// LocalRLHFPipeline generates preference pairs using only local Ollama
// inference — no external API calls. Suitable for privacy-mode workspaces.
type LocalRLHFPipeline struct {
	ollamaClient ppo.LLMCompleter
	dpoQueue     ppo.DPOQueueWriter
	logger       *slog.Logger
}

// NewLocalRLHFPipeline creates a local RLHF pipeline.
func NewLocalRLHFPipeline(ollama ppo.LLMCompleter, dpoQueue ppo.DPOQueueWriter, logger *slog.Logger) *LocalRLHFPipeline {
	return &LocalRLHFPipeline{
		ollamaClient: ollama,
		dpoQueue:     dpoQueue,
		logger:       logger,
	}
}

// GeneratePreferencePairs creates DPO training pairs from conversations
// using local Ollama inference only.
func (p *LocalRLHFPipeline) GeneratePreferencePairs(
	ctx context.Context,
	workspaceID uuid.UUID,
	sampleConversations []string,
) ([]learndpo.PreferencePair, error) {
	var pairs []learndpo.PreferencePair

	for _, conv := range sampleConversations {
		userTurn, agentTurn := parseConversationTurns(conv)
		if userTurn == "" || agentTurn == "" {
			continue
		}

		// Generate adversarial variant via Ollama.
		adversarial, err := p.ollamaClient.Complete(ctx,
			"Rewrite this response to be subtly unhelpful, overlong, or off-topic:",
			agentTurn,
		)
		if err != nil {
			p.logger.Warn("local_rlhf_adversarial_failed", "error", err)
			continue
		}

		pair := learndpo.PreferencePair{
			ID:               uuid.New(),
			WorkspaceID:      workspaceID,
			PromptText:       userTurn,
			ChosenResponse:   agentTurn,
			RejectedResponse: adversarial,
		}
		pairs = append(pairs, pair)

		if p.dpoQueue != nil {
			if qErr := p.dpoQueue.EnqueuePair(ctx, pair); qErr != nil {
				p.logger.Error("local_rlhf_queue_error", "error", qErr)
			}
		}
	}

	p.logger.Info("local_rlhf_pairs_generated",
		"workspace_id", workspaceID,
		"input_conversations", len(sampleConversations),
		"pairs_generated", len(pairs),
	)

	return pairs, nil
}

// parseConversationTurns extracts user and agent turns from a conversation string.
// Format expected: "User: <message>\nAssistant: <response>"
func parseConversationTurns(conv string) (string, string) {
	lines := strings.Split(conv, "\n")
	var userTurn, agentTurn string

	for _, line := range lines {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "User:") || strings.HasPrefix(line, "user:") {
			userTurn = strings.TrimSpace(strings.TrimPrefix(strings.TrimPrefix(line, "User:"), "user:"))
		} else if strings.HasPrefix(line, "Assistant:") || strings.HasPrefix(line, "assistant:") {
			agentTurn = strings.TrimSpace(strings.TrimPrefix(strings.TrimPrefix(line, "Assistant:"), "assistant:"))
		}
	}

	// Fallback: if no prefix markers, split at midpoint.
	if userTurn == "" && agentTurn == "" && len(lines) >= 2 {
		userTurn = lines[0]
		agentTurn = lines[len(lines)-1]
	}

	return userTurn, agentTurn
}

// IsLocalRLHFEnabled checks if local RLHF is configured for a workspace.
func IsLocalRLHFEnabled(privacyMode string, localRLHFEnabled bool) bool {
	return privacyMode == "strict" && localRLHFEnabled
}

// ErrLocalRLHFOnly is returned when a privacy-mode workspace cannot use external providers.
var ErrLocalRLHFOnly = fmt.Errorf("workspace requires local RLHF only (privacy_mode=strict)")
