package worker

import (
	"context"
	"fmt"
	"sync"
	"sync/atomic"
)

// BargeInController detects when user speech starts during agent TTS playback
// and cancels the active synthesis.
type BargeInController struct {
	mu          sync.Mutex
	cancelFuncs []context.CancelFunc
	enabled     atomic.Bool
}

// NewBargeInController creates a BargeInController with barge-in enabled by default.
func NewBargeInController() *BargeInController {
	c := &BargeInController{}
	c.enabled.Store(true)
	return c
}

// RegisterTTSContext registers a cancellable context for the current TTS synthesis.
// Call this before starting each TTS stream. Returns a cleanup function.
func (b *BargeInController) RegisterTTSContext(cancel context.CancelFunc) func() {
	b.mu.Lock()
	idx := len(b.cancelFuncs)
	b.cancelFuncs = append(b.cancelFuncs, cancel)
	b.mu.Unlock()
	return func() {
		b.mu.Lock()
		defer b.mu.Unlock()
		if idx < len(b.cancelFuncs) {
			// Zero out instead of slice-splice to avoid index drift.
			b.cancelFuncs[idx] = nil
		}
	}
}

// OnUserSpeechStart is called when the VAD detects user speech has begun.
// If barge-in is enabled and TTS is active, all registered TTS contexts are cancelled.
// Returns true if an interruption was triggered.
func (b *BargeInController) OnUserSpeechStart() bool {
	if !b.enabled.Load() {
		return false
	}
	b.mu.Lock()
	defer b.mu.Unlock()
	triggered := false
	for _, cancel := range b.cancelFuncs {
		if cancel != nil {
			cancel()
			triggered = true
		}
	}
	b.cancelFuncs = nil
	return triggered
}

// SetEnabled enables or disables barge-in. Thread-safe.
func (b *BargeInController) SetEnabled(enabled bool) {
	b.enabled.Store(enabled)
}

// IsEnabled returns whether barge-in is currently enabled.
func (b *BargeInController) IsEnabled() bool {
	return b.enabled.Load()
}

// ActiveCount returns the number of registered TTS contexts.
func (b *BargeInController) ActiveCount() int {
	b.mu.Lock()
	defer b.mu.Unlock()
	count := 0
	for _, c := range b.cancelFuncs {
		if c != nil {
			count++
		}
	}
	return count
}

// ---------------------------------------------------------------------------
// LiveKitVoiceSession
// ---------------------------------------------------------------------------

// LiveKitVoiceSessionConfig configures a LiveKit-backed voice session.
type LiveKitVoiceSessionConfig struct {
	SessionID    string
	WorkspaceID  string
	UserID       string
	RoomName     string
	Token        string
	LiveKitURL   string
	BargeIn      *BargeInController
	SessionStore SessionStore
}

// LiveKitVoiceSession manages the lifecycle of a single voice session in a LiveKit room.
// It integrates VAD for end-of-utterance detection and barge-in interruption.
type LiveKitVoiceSession struct {
	cfg    LiveKitVoiceSessionConfig
	closed atomic.Bool
}

// NewLiveKitVoiceSession creates a LiveKitVoiceSession.
func NewLiveKitVoiceSession(cfg LiveKitVoiceSessionConfig) (*LiveKitVoiceSession, error) {
	if cfg.SessionID == "" {
		return nil, fmt.Errorf("livekit session: SessionID required")
	}
	if cfg.BargeIn == nil {
		cfg.BargeIn = NewBargeInController()
	}
	return &LiveKitVoiceSession{cfg: cfg}, nil
}

// NotifySpeechStart is called by the VAD pipeline when user speech is detected.
// It triggers barge-in if TTS is active.
func (l *LiveKitVoiceSession) NotifySpeechStart() bool {
	return l.cfg.BargeIn.OnUserSpeechStart()
}

// NotifySpeechEnd is called when an utterance ends (silence threshold crossed).
// In a full implementation, this would trigger the STT→LLM→TTS pipeline.
// For now it records the event.
func (l *LiveKitVoiceSession) NotifySpeechEnd(ctx context.Context, segmentDurationMs int) error {
	if l.cfg.SessionStore == nil {
		return nil
	}
	turn := TranscriptTurn{
		Speaker: "user",
		Text:    fmt.Sprintf("[speech segment %dms]", segmentDurationMs),
	}
	return l.cfg.SessionStore.AddTurn(ctx, l.cfg.SessionID, turn)
}

// Close marks the session as closed.
func (l *LiveKitVoiceSession) Close() {
	l.closed.Store(true)
}

// IsClosed returns whether Close has been called.
func (l *LiveKitVoiceSession) IsClosed() bool {
	return l.closed.Load()
}
