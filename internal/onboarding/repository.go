package onboarding

import "context"

// Repository abstracts durable storage for onboarding sessions.
// Production uses PgRepository; tests use the in-memory OnboardingService.
type Repository interface {
	// StartSession creates a new onboarding session for the workspace.
	StartSession(ctx context.Context, workspaceID string) (*OnboardingSession, error)
	// AdvanceStage records answers and advances to the next stage.
	AdvanceStage(ctx context.Context, sessionID string, answers map[string]string) error
	// SkipStage skips the current stage.
	SkipStage(ctx context.Context, sessionID string) error
	// GetStatus retrieves the current session for a workspace.
	GetStatus(ctx context.Context, workspaceID string) (*OnboardingSession, error)
	// IsComplete checks whether the session is completed.
	IsComplete(ctx context.Context, sessionID string) (bool, error)
}
