package brain

import (
	"context"
	"time"

	"github.com/google/uuid"
)

// WorldModelRepository defines persistence operations for world model facts.
// All read methods return only non-expired facts (expires_at > now()).
type WorldModelRepository interface {
	// AddFact upserts a fact. On conflict (workspace_id, subject, predicate),
	// the newer value wins (update value, source, confidence, learned_at, expires_at).
	AddFact(ctx context.Context, workspaceID uuid.UUID, subject, predicate, value, source string,
		confidence float64, expiresAt time.Time) (WorldFact, error)

	// GetFacts returns all non-expired facts for a workspace filtered by subject.
	GetFacts(ctx context.Context, workspaceID uuid.UUID, subject string) ([]WorldFact, error)

	// GetAllFacts returns all non-expired facts for a workspace.
	GetAllFacts(ctx context.Context, workspaceID uuid.UUID) ([]WorldFact, error)

	// ExpireFacts hard-deletes facts past their expires_at for a workspace.
	// Returns the count of deleted rows.
	ExpireFacts(ctx context.Context, workspaceID uuid.UUID) (int, error)
}
