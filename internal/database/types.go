package database

import (
	"context"
	"errors"

	"github.com/google/uuid"
)

type Config struct {
	DSN string
}

var ErrWorkspaceUnset = errors.New("workspace_id is required in context")

type workspaceContextKey struct{}

func WithWorkspaceID(ctx context.Context, workspaceID uuid.UUID) context.Context {
	return context.WithValue(ctx, workspaceContextKey{}, workspaceID)
}

func WorkspaceIDFromContext(ctx context.Context) (uuid.UUID, error) {
	workspaceID, ok := ctx.Value(workspaceContextKey{}).(uuid.UUID)
	if !ok || workspaceID == uuid.Nil {
		return uuid.Nil, ErrWorkspaceUnset
	}
	return workspaceID, nil
}

func ValidateConfig(cfg Config) error {
	if cfg.DSN == "" {
		return errors.New("dsn must not be empty")
	}
	return nil
}
