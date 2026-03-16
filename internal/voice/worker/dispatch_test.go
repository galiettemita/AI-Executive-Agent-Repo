package worker

import (
	"context"
	"fmt"
	"log/slog"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func noopProcessor(_ context.Context, _ VoiceSession) error { return nil }

func TestNewWorkerDispatch_DefaultsMaxWorkers(t *testing.T) {
	wd, err := NewWorkerDispatch(0, noopProcessor, nil)
	require.NoError(t, err)
	assert.Equal(t, 4, wd.MaxWorkers())
}

func TestNewWorkerDispatch_NilProcessor(t *testing.T) {
	_, err := NewWorkerDispatch(2, nil, nil)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "processor must not be nil")
}

func TestDispatch_EmptySessionID(t *testing.T) {
	wd, _ := NewWorkerDispatch(2, noopProcessor, nil)
	err := wd.Dispatch(context.Background(), VoiceSession{})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "session ID must not be empty")
}

func TestDispatch_CancelledContext(t *testing.T) {
	// Fill all 2 slots so Dispatch must wait.
	block := make(chan struct{})
	wd, _ := NewWorkerDispatch(2, func(_ context.Context, _ VoiceSession) error {
		<-block
		return nil
	}, nil)
	sess := func(id string) VoiceSession { return VoiceSession{ID: id} }

	ctx := context.Background()
	require.NoError(t, wd.Dispatch(ctx, sess("s1")))
	require.NoError(t, wd.Dispatch(ctx, sess("s2")))

	cancelCtx, cancel := context.WithCancel(ctx)
	cancel() // already cancelled
	err := wd.Dispatch(cancelCtx, sess("s3"))
	require.Error(t, err)
	assert.Contains(t, err.Error(), "cancelled")

	close(block)
}

func TestDispatch_ProcessorIsCalled(t *testing.T) {
	var called atomic.Bool
	wd, _ := NewWorkerDispatch(2, func(_ context.Context, _ VoiceSession) error {
		called.Store(true)
		return nil
	}, nil)
	require.NoError(t, wd.Dispatch(context.Background(), VoiceSession{ID: "abc"}))

	shutCtx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	require.NoError(t, wd.Shutdown(shutCtx))
	assert.True(t, called.Load())
}

func TestDispatch_ProcessorPanicIsRecovered(t *testing.T) {
	wd, _ := NewWorkerDispatch(2, func(_ context.Context, _ VoiceSession) error {
		panic("test panic")
	}, slog.Default())
	err := wd.Dispatch(context.Background(), VoiceSession{ID: "panic-sess"})
	require.NoError(t, err) // panic must not surface

	shutCtx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	_ = wd.Shutdown(shutCtx)
	assert.Equal(t, 0, wd.ActiveWorkers())
}

func TestDispatch_ConcurrencyLimit(t *testing.T) {
	var peak atomic.Int64
	var current atomic.Int64
	block := make(chan struct{})

	wd, _ := NewWorkerDispatch(2, func(_ context.Context, _ VoiceSession) error {
		cur := current.Add(1)
		if cur > peak.Load() {
			peak.Store(cur)
		}
		<-block
		current.Add(-1)
		return nil
	}, nil)

	// First two fill the pool; remaining dispatches are launched in goroutines
	// because they will block on the semaphore until a slot is freed.
	for i := 0; i < 2; i++ {
		require.NoError(t, wd.Dispatch(context.Background(), VoiceSession{ID: fmt.Sprintf("s%d", i)}))
	}
	for i := 2; i < 4; i++ {
		go func(idx int) {
			_ = wd.Dispatch(context.Background(), VoiceSession{ID: fmt.Sprintf("s%d", idx)})
		}(i)
	}
	time.Sleep(50 * time.Millisecond)
	assert.LessOrEqual(t, peak.Load(), int64(2))
	close(block)
}

func TestShutdown_WaitsForWorkers(t *testing.T) {
	wd, _ := NewWorkerDispatch(2, func(_ context.Context, _ VoiceSession) error {
		time.Sleep(60 * time.Millisecond)
		return nil
	}, nil)
	require.NoError(t, wd.Dispatch(context.Background(), VoiceSession{ID: "s1"}))
	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
	defer cancel()
	assert.NoError(t, wd.Shutdown(ctx))
}

func TestShutdown_TimesOut(t *testing.T) {
	block := make(chan struct{})
	wd, _ := NewWorkerDispatch(2, func(_ context.Context, _ VoiceSession) error {
		<-block
		return nil
	}, nil)
	require.NoError(t, wd.Dispatch(context.Background(), VoiceSession{ID: "s1"}))
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Millisecond)
	defer cancel()
	err := wd.Shutdown(ctx)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "shutdown timed out")
	close(block)
}
