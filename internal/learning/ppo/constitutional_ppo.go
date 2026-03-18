package ppo

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/google/uuid"
	learndpo "github.com/brevio/brevio/internal/learning/dpo"
)

// CAICritiquer evaluates a response against Constitutional AI principles.
type CAICritiquer interface {
	Evaluate(ctx context.Context, response string) ([]CAIViolation, error)
}

// CAIViolation describes a violation of a Constitutional AI principle.
type CAIViolation struct {
	Principle   string  `json:"principle"` // C1-C8
	Description string  `json:"description"`
	Severity    float64 `json:"severity"`
}

// LLMCompleter generates corrected responses.
type LLMCompleter interface {
	Complete(ctx context.Context, systemPrompt, userPrompt string) (string, error)
}

// DPOQueueWriter enqueues preference pairs for batch DPO processing.
type DPOQueueWriter interface {
	EnqueuePair(ctx context.Context, pair learndpo.PreferencePair) error
}

// PPORoundResult captures the output of one PPO evaluation cycle.
type PPORoundResult struct {
	WorkspaceID       uuid.UUID         `json:"workspace_id"`
	RequestID         uuid.UUID         `json:"request_id"`
	OriginalResponse  string            `json:"original_response"`
	CorrectedResponse string            `json:"corrected_response"`
	Violations        []CAIViolation    `json:"violations"`
	RewardSignal      float64           `json:"reward_signal"`
	PairQueuedAt      time.Time         `json:"pair_queued_at,omitempty"`
}

// ConstitutionalPPOLoop evaluates LLM responses against CAI principles
// and queues corrected preference pairs for DPO training.
type ConstitutionalPPOLoop struct {
	caiCritiquer CAICritiquer
	llm          LLMCompleter
	dpoQueue     DPOQueueWriter
	logger       *slog.Logger
}

// NewConstitutionalPPOLoop creates a PPO loop instance.
func NewConstitutionalPPOLoop(
	cai CAICritiquer,
	llm LLMCompleter,
	dpoQueue DPOQueueWriter,
	logger *slog.Logger,
) *ConstitutionalPPOLoop {
	return &ConstitutionalPPOLoop{
		caiCritiquer: cai,
		llm:          llm,
		dpoQueue:     dpoQueue,
		logger:       logger,
	}
}

// EvaluateAndCorrect runs the CAI critique loop on a response.
// If violations are found, generates a corrected response and queues
// a preference pair for DPO training.
func (p *ConstitutionalPPOLoop) EvaluateAndCorrect(
	ctx context.Context,
	workspaceID, requestID uuid.UUID,
	response string,
) (*PPORoundResult, error) {
	result := &PPORoundResult{
		WorkspaceID:      workspaceID,
		RequestID:        requestID,
		OriginalResponse: response,
	}

	// Step 1: Evaluate against CAI principles.
	violations, err := p.caiCritiquer.Evaluate(ctx, response)
	if err != nil {
		return nil, fmt.Errorf("cai evaluate: %w", err)
	}
	result.Violations = violations

	// Step 2: Compute reward signal.
	result.RewardSignal = computeReward(violations)

	// Step 3: If violations found, generate corrected response and queue pair.
	if result.RewardSignal < 0.0 && len(violations) > 0 {
		violationNames := make([]string, len(violations))
		for i, v := range violations {
			violationNames[i] = v.Principle
		}

		systemPrompt := fmt.Sprintf(
			"The following response violates principle(s) %v. "+
				"Rewrite it to comply with all Constitutional AI principles while preserving the helpful intent:",
			violationNames,
		)

		corrected, llmErr := p.llm.Complete(ctx, systemPrompt, response)
		if llmErr != nil {
			p.logger.Error("ppo_correction_failed", "error", llmErr)
			return result, nil
		}
		result.CorrectedResponse = corrected

		// Queue preference pair.
		pair := learndpo.PreferencePair{
			ID:               uuid.New(),
			WorkspaceID:      workspaceID,
			PromptText:       response,
			ChosenResponse:   corrected,
			RejectedResponse: response,
		}

		if queueErr := p.dpoQueue.EnqueuePair(ctx, pair); queueErr != nil {
			p.logger.Error("ppo_queue_pair_error", "error", queueErr)
		} else {
			result.PairQueuedAt = time.Now()
			p.logger.Info("ppo_pair_queued",
				"workspace_id", workspaceID,
				"request_id", requestID,
				"violations", len(violations),
				"reward", result.RewardSignal,
			)
		}
	}

	return result, nil
}

func computeReward(violations []CAIViolation) float64 {
	if len(violations) == 0 {
		return 1.0
	}

	for _, v := range violations {
		if v.Principle == "C1" || v.Principle == "C2" {
			return -1.0
		}
	}

	reward := -0.5 * float64(len(violations))
	if reward < -1.0 {
		reward = -1.0
	}
	return reward
}
