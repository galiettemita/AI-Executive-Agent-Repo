package database

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgconn"
)

type fakeWorkspaceSessionSetter struct {
	sql      string
	args     []any
	execErr  error
	execCall int
}

func (f *fakeWorkspaceSessionSetter) Exec(_ context.Context, sql string, arguments ...any) (pgconn.CommandTag, error) {
	f.execCall++
	f.sql = sql
	f.args = append([]any(nil), arguments...)
	if f.execErr != nil {
		return pgconn.CommandTag{}, f.execErr
	}
	return pgconn.NewCommandTag("SET"), nil
}

func TestPoolExecRejectsWithoutWorkspaceID(t *testing.T) {
	t.Parallel()

	pool := &Pool{}
	_, err := pool.Exec(context.Background(), "SELECT 1")
	if !errors.Is(err, ErrWorkspaceUnset) {
		t.Fatalf("expected ErrWorkspaceUnset, got %v", err)
	}
}

func TestSetWorkspaceIDOnSession(t *testing.T) {
	t.Parallel()

	setter := &fakeWorkspaceSessionSetter{}
	workspaceID := uuid.MustParse("018f3f6a-9a0f-7cc6-8f2f-1f0f2d2f2d2f")

	if err := setWorkspaceIDOnSession(context.Background(), setter, workspaceID); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if setter.execCall != 1 {
		t.Fatalf("unexpected exec count: got=%d want=1", setter.execCall)
	}
	if setter.sql != "SET app.workspace_id = $1" {
		t.Fatalf("unexpected sql: %s", setter.sql)
	}
	if len(setter.args) != 1 {
		t.Fatalf("unexpected arg count: %d", len(setter.args))
	}
	if got, ok := setter.args[0].(string); !ok || got != workspaceID.String() {
		t.Fatalf("unexpected workspace arg: %#v", setter.args[0])
	}
}

func TestSetWorkspaceIDOnSessionPropagatesError(t *testing.T) {
	t.Parallel()

	expectedErr := errors.New("set workspace failed")
	setter := &fakeWorkspaceSessionSetter{execErr: expectedErr}
	workspaceID := uuid.MustParse("018f3f6a-9a0f-7cc6-8f2f-1f0f2d2f2d2f")

	err := setWorkspaceIDOnSession(context.Background(), setter, workspaceID)
	if !errors.Is(err, expectedErr) {
		t.Fatalf("expected %v, got %v", expectedErr, err)
	}
}
