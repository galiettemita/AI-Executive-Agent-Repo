package database

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/google/uuid"
)

// TestRLSFailClosedWorkspaceIDRequired proves the fail-closed contract:
// any database operation without workspace_id in context MUST fail.
func TestRLSFailClosedWorkspaceIDRequired(t *testing.T) {
	t.Parallel()

	t.Run("empty_context_returns_ErrWorkspaceUnset", func(t *testing.T) {
		t.Parallel()
		_, err := WorkspaceIDFromContext(context.Background())
		if !errors.Is(err, ErrWorkspaceUnset) {
			t.Fatalf("empty context must return ErrWorkspaceUnset, got: %v", err)
		}
	})

	t.Run("nil_uuid_returns_ErrWorkspaceUnset", func(t *testing.T) {
		t.Parallel()
		ctx := WithWorkspaceID(context.Background(), uuid.Nil)
		_, err := WorkspaceIDFromContext(ctx)
		if !errors.Is(err, ErrWorkspaceUnset) {
			t.Fatalf("nil UUID must return ErrWorkspaceUnset, got: %v", err)
		}
	})

	t.Run("valid_uuid_succeeds", func(t *testing.T) {
		t.Parallel()
		expected := uuid.MustParse("018f3f6a-9a0f-7cc6-8f2f-1f0f2d2f2d2f")
		ctx := WithWorkspaceID(context.Background(), expected)
		got, err := WorkspaceIDFromContext(ctx)
		if err != nil {
			t.Fatalf("valid UUID must succeed, got error: %v", err)
		}
		if got != expected {
			t.Fatalf("workspace_id mismatch: got=%s want=%s", got, expected)
		}
	})

	t.Run("pool_exec_rejects_without_workspace", func(t *testing.T) {
		t.Parallel()
		pool := &Pool{}
		_, err := pool.Exec(context.Background(), "SELECT 1")
		if !errors.Is(err, ErrWorkspaceUnset) {
			t.Fatalf("Pool.Exec without workspace must fail with ErrWorkspaceUnset, got: %v", err)
		}
	})

	t.Run("session_setter_receives_correct_SET_command", func(t *testing.T) {
		t.Parallel()
		setter := &fakeWorkspaceSessionSetter{}
		workspaceID := uuid.MustParse("018f3f6a-9a0f-7cc6-8f2f-1f0f2d2f2d2f")

		if err := setWorkspaceIDOnSession(context.Background(), setter, workspaceID); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if setter.sql != "SET app.workspace_id = $1" {
			t.Fatalf("expected SET app.workspace_id SQL, got: %s", setter.sql)
		}
		if len(setter.args) != 1 || setter.args[0] != workspaceID.String() {
			t.Fatalf("expected workspace_id as argument, got: %v", setter.args)
		}
	})

	t.Run("session_setter_error_propagated", func(t *testing.T) {
		t.Parallel()
		dbErr := errors.New("connection refused")
		setter := &fakeWorkspaceSessionSetter{execErr: dbErr}
		workspaceID := uuid.MustParse("018f3f6a-9a0f-7cc6-8f2f-1f0f2d2f2d2f")

		err := setWorkspaceIDOnSession(context.Background(), setter, workspaceID)
		if !errors.Is(err, dbErr) {
			t.Fatalf("expected propagated error, got: %v", err)
		}
	})
}

// TestRLSFailClosedVerifyPostgresMigrationPattern validates that the
// migration verification script contains the critical RLS assertions.
func TestRLSFailClosedVerifyPostgresMigrationPattern(t *testing.T) {
	t.Parallel()

	verifyScript := readFileForTest(t, "scripts/database/verify_postgres_migrations.sh")

	required := []string{
		"relrowsecurity",
		"workspace_id",
		"app.workspace_id",
		"cross-workspace isolation",
	}
	for _, token := range required {
		if !containsCI(verifyScript, token) {
			t.Fatalf("verify_postgres_migrations.sh missing RLS verification token: %q", token)
		}
	}
}

func readFileForTest(t *testing.T, relPath string) string {
	t.Helper()
	_, currentFile, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("unable to resolve caller")
	}
	root := filepath.Clean(filepath.Join(filepath.Dir(currentFile), "..", ".."))
	fullPath := filepath.Join(root, relPath)
	body, err := os.ReadFile(fullPath)
	if err != nil {
		t.Fatalf("read %s: %v", relPath, err)
	}
	return string(body)
}

func containsCI(haystack, needle string) bool {
	return strings.Contains(strings.ToLower(haystack), strings.ToLower(needle))
}
