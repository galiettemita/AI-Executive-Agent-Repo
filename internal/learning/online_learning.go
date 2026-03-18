package learning

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/google/uuid"
	learndpo "github.com/brevio/brevio/internal/learning/dpo"
	"github.com/brevio/brevio/internal/learning/ppo"
)

// ORMScoreReader reads ORM scores for interactions.
type ORMScoreReader interface {
	GetScore(ctx context.Context, requestID uuid.UUID) (float64, error)
}

// CaseLibraryUpdater records failure cases for future learning.
type CaseLibraryUpdater interface {
	UpdateFailureCase(ctx context.Context, workspaceID, requestID uuid.UUID, badResponse, idealResponse string) error
}

// OnlineLearningInput is the input for online learning processing.
type OnlineLearningInput struct {
	WorkspaceID   uuid.UUID `json:"workspace_id"`
	RequestID     uuid.UUID `json:"request_id"`
	UserMessage   string    `json:"user_message"`
	AgentResponse string    `json:"agent_response"`
	ORMScore      float64   `json:"orm_score"`
}

// OnlineLearningService processes low-ORM interactions to generate
// preference pairs for DPO training.
type OnlineLearningService struct {
	dpoQueue    ppo.DPOQueueWriter
	llm         ppo.LLMCompleter
	caseLibrary CaseLibraryUpdater
	logger      *slog.Logger
}

// NewOnlineLearningService creates an online learning service.
func NewOnlineLearningService(
	dpoQueue ppo.DPOQueueWriter,
	llm ppo.LLMCompleter,
	caseLibrary CaseLibraryUpdater,
	logger *slog.Logger,
) *OnlineLearningService {
	return &OnlineLearningService{
		dpoQueue:    dpoQueue,
		llm:         llm,
		caseLibrary: caseLibrary,
		logger:      logger,
	}
}

// ProcessInteraction generates a preference pair for interactions with ORM < 3.0.
func (s *OnlineLearningService) ProcessInteraction(ctx context.Context, input OnlineLearningInput) error {
	if input.ORMScore >= 3.0 {
		return nil
	}

	// Generate ideal response.
	systemPrompt := "Given this user request, generate the ideal executive assistant response:"
	idealResponse, err := s.llm.Complete(ctx, systemPrompt, input.UserMessage)
	if err != nil {
		return fmt.Errorf("generate ideal response: %w", err)
	}

	// Create preference pair.
	pair := learndpo.PreferencePair{
		ID:               uuid.New(),
		WorkspaceID:      input.WorkspaceID,
		PromptText:       input.UserMessage,
		ChosenResponse:   idealResponse,
		RejectedResponse: input.AgentResponse,
	}

	if err := s.dpoQueue.EnqueuePair(ctx, pair); err != nil {
		return fmt.Errorf("enqueue pair: %w", err)
	}

	// Update case library.
	if s.caseLibrary != nil {
		if clErr := s.caseLibrary.UpdateFailureCase(ctx, input.WorkspaceID, input.RequestID, input.AgentResponse, idealResponse); clErr != nil {
			s.logger.Error("update_failure_case_error", "error", clErr)
		}
	}

	s.logger.Info("online_learning_pair_queued",
		"workspace_id", input.WorkspaceID,
		"request_id", input.RequestID,
		"orm_score", input.ORMScore,
	)

	return nil
}
