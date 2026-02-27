package database

import (
	"context"
	"testing"

	"github.com/google/uuid"
)

func TestWorkspaceIDFromContextRequiresWorkspace(t *testing.T) {
	t.Parallel()

	_, err := WorkspaceIDFromContext(context.Background())
	if err == nil {
		t.Fatal("expected workspace context error")
	}
}

func TestWorkspaceIDRoundTrip(t *testing.T) {
	t.Parallel()

	workspaceID := uuid.MustParse("018f3f6a-9a0f-7cc6-8f2f-1f0f2d2f2d2f")
	ctx := WithWorkspaceID(context.Background(), workspaceID)
	got, err := WorkspaceIDFromContext(ctx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != workspaceID {
		t.Fatalf("workspace mismatch: got %s want %s", got, workspaceID)
	}
}
