package worker

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newTestRedisStore(t *testing.T) (*RedisSessionStore, *miniredis.Miniredis) {
	t.Helper()
	mr, err := miniredis.Run()
	require.NoError(t, err)
	t.Cleanup(mr.Close)
	client := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	t.Cleanup(func() { client.Close() })
	store, err := NewRedisSessionStore(client, 5*time.Minute)
	require.NoError(t, err)
	return store, mr
}

// runStoreTests runs the shared test suite against any SessionStore implementation.
func runStoreTests(t *testing.T, store SessionStore) {
	ctx := context.Background()

	t.Run("CreateSession_Success", func(t *testing.T) {
		sess, err := store.Create(ctx, "ws1", "user1")
		require.NoError(t, err)
		assert.NotEmpty(t, sess.ID)
		assert.Equal(t, "active", sess.Status)
		assert.Equal(t, "ws1", sess.WorkspaceID)
		assert.Equal(t, "user1", sess.UserID)

		got, err := store.Get(ctx, sess.ID)
		require.NoError(t, err)
		assert.Equal(t, sess.ID, got.ID)
		assert.Equal(t, "active", got.Status)
	})

	t.Run("CreateSession_EmptyWorkspaceID", func(t *testing.T) {
		_, err := store.Create(ctx, "", "user1")
		require.Error(t, err)
	})

	t.Run("CreateSession_EmptyUserID", func(t *testing.T) {
		_, err := store.Create(ctx, "ws1", "")
		require.Error(t, err)
	})

	t.Run("GetSession_NotFound", func(t *testing.T) {
		_, err := store.Get(ctx, "nonexistent-id")
		require.Error(t, err)
		assert.True(t, errors.Is(err, ErrSessionNotFound))
	})

	t.Run("EndSession_Success", func(t *testing.T) {
		sess, _ := store.Create(ctx, "ws-end", "user1")
		ended, err := store.End(ctx, sess.ID)
		require.NoError(t, err)
		assert.Equal(t, "ended", ended.Status)
		assert.True(t, ended.DurationMs >= 0)
		assert.True(t, ended.EndedAt.After(ended.StartedAt) || ended.EndedAt.Equal(ended.StartedAt))
	})

	t.Run("EndSession_NotFound", func(t *testing.T) {
		_, err := store.End(ctx, "nonexistent-id")
		require.Error(t, err)
		assert.True(t, errors.Is(err, ErrSessionNotFound))
	})

	t.Run("EndSession_AlreadyEnded", func(t *testing.T) {
		sess, _ := store.Create(ctx, "ws-double-end", "user1")
		_, err := store.End(ctx, sess.ID)
		require.NoError(t, err)
		_, err = store.End(ctx, sess.ID)
		require.Error(t, err)
		assert.True(t, errors.Is(err, ErrSessionNotActive))
	})

	t.Run("AddTurn_Success", func(t *testing.T) {
		sess, _ := store.Create(ctx, "ws-turn", "user1")
		err := store.AddTurn(ctx, sess.ID, TranscriptTurn{Speaker: "user", Text: "Hello"})
		require.NoError(t, err)
		got, _ := store.Get(ctx, sess.ID)
		assert.Len(t, got.TranscriptTurns, 1)
		assert.Equal(t, "user", got.TranscriptTurns[0].Speaker)
	})

	t.Run("AddTurn_NotFound", func(t *testing.T) {
		err := store.AddTurn(ctx, "nonexistent-id", TranscriptTurn{Speaker: "user", Text: "hi"})
		require.Error(t, err)
		assert.True(t, errors.Is(err, ErrSessionNotFound))
	})

	t.Run("AddTurn_EndedSession", func(t *testing.T) {
		sess, _ := store.Create(ctx, "ws-turn-ended", "user1")
		store.End(ctx, sess.ID)
		err := store.AddTurn(ctx, sess.ID, TranscriptTurn{Speaker: "user", Text: "hi"})
		require.Error(t, err)
		assert.True(t, errors.Is(err, ErrSessionNotActive))
	})

	t.Run("ListSessions_ReturnsMatchingWorkspace", func(t *testing.T) {
		store.Create(ctx, "ws-list", "user1")
		store.Create(ctx, "ws-list", "user2")
		store.Create(ctx, "ws-other", "user3")

		sessions, err := store.List(ctx, "ws-list")
		require.NoError(t, err)
		assert.Len(t, sessions, 2)
	})

	t.Run("ListSessions_EmptyForUnknownWorkspace", func(t *testing.T) {
		sessions, err := store.List(ctx, "ws-unknown-xyz")
		require.NoError(t, err)
		assert.Empty(t, sessions)
	})

	t.Run("DeleteSession_Success", func(t *testing.T) {
		sess, _ := store.Create(ctx, "ws-del", "user1")
		err := store.Delete(ctx, sess.ID)
		require.NoError(t, err)
		_, err = store.Get(ctx, sess.ID)
		assert.True(t, errors.Is(err, ErrSessionNotFound))
	})

	t.Run("DeleteSession_NotFound", func(t *testing.T) {
		err := store.Delete(ctx, "nonexistent-id")
		require.Error(t, err)
		assert.True(t, errors.Is(err, ErrSessionNotFound))
	})
}

func TestRedisSessionStore(t *testing.T) {
	store, _ := newTestRedisStore(t)
	runStoreTests(t, store)
}

func TestInMemorySessionStore(t *testing.T) {
	store := NewInMemorySessionStore()
	runStoreTests(t, store)
}

// Redis-only tests

func TestRedisSessionStore_TTLExpiry(t *testing.T) {
	store, mr := newTestRedisStore(t)
	ctx := context.Background()

	sess, err := store.Create(ctx, "ws-ttl", "user1")
	require.NoError(t, err)

	// Fast-forward past TTL.
	mr.FastForward(6 * time.Minute)

	_, err = store.Get(ctx, sess.ID)
	require.Error(t, err)
	assert.True(t, errors.Is(err, ErrSessionNotFound))
}

func TestRedisSessionStore_WorkspaceIndexCleansExpiredIDs(t *testing.T) {
	store, mr := newTestRedisStore(t)
	ctx := context.Background()

	sess, err := store.Create(ctx, "ws-cleanup", "user1")
	require.NoError(t, err)

	// Verify it's listed.
	sessions, err := store.List(ctx, "ws-cleanup")
	require.NoError(t, err)
	assert.Len(t, sessions, 1)

	// Expire the session key directly (simulates TTL expiry).
	mr.FastForward(6 * time.Minute)

	// List should silently clean up the stale ID.
	sessions, err = store.List(ctx, "ws-cleanup")
	require.NoError(t, err)
	assert.Empty(t, sessions)

	// The workspace set should no longer contain the expired ID.
	members, err := redis.NewClient(&redis.Options{Addr: mr.Addr()}).SMembers(ctx, "brevio:voice:ws:ws-cleanup").Result()
	require.NoError(t, err)
	assert.NotContains(t, members, sess.ID)
}
