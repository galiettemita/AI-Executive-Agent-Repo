package worker

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewLiveKitVoiceSession_EmptySessionID(t *testing.T) {
	_, err := NewLiveKitVoiceSession(LiveKitVoiceSessionConfig{})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "SessionID required")
}

func TestNewLiveKitVoiceSession_NilBargeInDefaulted(t *testing.T) {
	sess, err := NewLiveKitVoiceSession(LiveKitVoiceSessionConfig{
		SessionID: "sess-1",
	})
	require.NoError(t, err)
	assert.NotNil(t, sess.cfg.BargeIn)
	assert.True(t, sess.cfg.BargeIn.IsEnabled())
}

func TestBargeInController_OnUserSpeechStart_NoRegistered(t *testing.T) {
	b := NewBargeInController()
	assert.False(t, b.OnUserSpeechStart())
	assert.Equal(t, 0, b.ActiveCount())
}

func TestBargeInController_OnUserSpeechStart_Cancels(t *testing.T) {
	b := NewBargeInController()
	ctx, cancel := context.WithCancel(context.Background())
	b.RegisterTTSContext(cancel)
	assert.Equal(t, 1, b.ActiveCount())

	triggered := b.OnUserSpeechStart()
	assert.True(t, triggered)
	assert.Equal(t, 0, b.ActiveCount())

	// Context should be cancelled.
	assert.Error(t, ctx.Err())
}

func TestBargeInController_OnUserSpeechStart_Disabled(t *testing.T) {
	b := NewBargeInController()
	b.SetEnabled(false)
	_, cancel := context.WithCancel(context.Background())
	b.RegisterTTSContext(cancel)

	triggered := b.OnUserSpeechStart()
	assert.False(t, triggered)
}

func TestBargeInController_SetEnabled_Toggle(t *testing.T) {
	b := NewBargeInController()
	assert.True(t, b.IsEnabled())

	b.SetEnabled(false)
	assert.False(t, b.IsEnabled())

	b.SetEnabled(true)
	assert.True(t, b.IsEnabled())
}

func TestLiveKitVoiceSession_NotifySpeechStart_TriggersBargeIn(t *testing.T) {
	bargeIn := NewBargeInController()
	ctx, cancel := context.WithCancel(context.Background())
	bargeIn.RegisterTTSContext(cancel)

	sess, err := NewLiveKitVoiceSession(LiveKitVoiceSessionConfig{
		SessionID: "sess-1",
		BargeIn:   bargeIn,
	})
	require.NoError(t, err)

	triggered := sess.NotifySpeechStart()
	assert.True(t, triggered)
	assert.Error(t, ctx.Err()) // context should be cancelled
}

func TestLiveKitVoiceSession_Close_SetsClosed(t *testing.T) {
	sess, _ := NewLiveKitVoiceSession(LiveKitVoiceSessionConfig{SessionID: "sess-1"})
	assert.False(t, sess.IsClosed())
	sess.Close()
	assert.True(t, sess.IsClosed())
}
