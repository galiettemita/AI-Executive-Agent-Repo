package gateway

import (
	"context"
	"errors"
	"math"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// helper: create a 10ms frame of silence (160 zero samples at 16 kHz)
func silenceFrame(ts time.Duration) VADFrame {
	return VADFrame{PCM: make([]int16, 160), DurationMs: 10, Timestamp: ts}
}

// helper: create a 10ms frame of full-scale speech
func speechFrame(ts time.Duration) VADFrame {
	pcm := make([]int16, 160)
	for i := range pcm {
		pcm[i] = math.MaxInt16
	}
	return VADFrame{PCM: pcm, DurationMs: 10, Timestamp: ts}
}

func TestEnergyVADProvider_Name(t *testing.T) {
	p := NewEnergyVADProvider(EnergyVADConfig{})
	assert.Equal(t, "energy_vad", p.Name())
}

func TestEnergyVADProvider_InvalidFrameDuration(t *testing.T) {
	p := NewEnergyVADProvider(EnergyVADConfig{})
	frame := VADFrame{PCM: make([]int16, 240), DurationMs: 15, Timestamp: 0}
	_, err := p.Process(context.Background(), frame)
	require.Error(t, err)
	assert.True(t, errors.Is(err, ErrVADInvalidFrame))
}

func TestEnergyVADProvider_WrongSampleCount(t *testing.T) {
	p := NewEnergyVADProvider(EnergyVADConfig{})
	// 10ms should be 160 samples, not 120
	frame := VADFrame{PCM: make([]int16, 120), DurationMs: 10, Timestamp: 0}
	_, err := p.Process(context.Background(), frame)
	require.Error(t, err)
	assert.True(t, errors.Is(err, ErrVADInvalidFrame))
}

func TestEnergyVADProvider_Silence(t *testing.T) {
	p := NewEnergyVADProvider(EnergyVADConfig{})
	// Use a timestamp well beyond any hold period to ensure no hold effect
	result, err := p.Process(context.Background(), silenceFrame(10*time.Second))
	require.NoError(t, err)
	assert.False(t, result.IsSpeech)
}

func TestEnergyVADProvider_Speech(t *testing.T) {
	p := NewEnergyVADProvider(EnergyVADConfig{})
	result, err := p.Process(context.Background(), speechFrame(0))
	require.NoError(t, err)
	assert.True(t, result.IsSpeech)
	assert.Greater(t, result.Confidence, 0.0)
}

func TestEnergyVADProvider_HoldPeriod(t *testing.T) {
	p := NewEnergyVADProvider(EnergyVADConfig{SpeechHoldMs: 300})
	ctx := context.Background()

	// Speech frame at 0ms
	r, _ := p.Process(ctx, speechFrame(0))
	assert.True(t, r.IsSpeech)

	// Silence at 100ms — within 300ms hold, should still be speech
	r, _ = p.Process(ctx, silenceFrame(100*time.Millisecond))
	assert.True(t, r.IsSpeech)

	// Silence at 200ms — still within hold
	r, _ = p.Process(ctx, silenceFrame(200*time.Millisecond))
	assert.True(t, r.IsSpeech)
}

func TestEnergyVADProvider_HoldExpiry(t *testing.T) {
	p := NewEnergyVADProvider(EnergyVADConfig{SpeechHoldMs: 100})
	ctx := context.Background()

	// Speech frame at 0ms
	p.Process(ctx, speechFrame(0))

	// Silence well beyond hold period
	r, _ := p.Process(ctx, silenceFrame(500*time.Millisecond))
	assert.False(t, r.IsSpeech)
}

func TestComputeRMS_Zero(t *testing.T) {
	samples := make([]int16, 160)
	assert.Equal(t, 0.0, computeRMS(samples))
}

func TestComputeRMS_FullScale(t *testing.T) {
	samples := make([]int16, 160)
	for i := range samples {
		samples[i] = math.MaxInt16
	}
	rms := computeRMS(samples)
	// MaxInt16 / 32768.0 ≈ 0.99997, RMS of constant ≈ 0.99997
	assert.InDelta(t, 1.0, rms, 0.01)
}

func TestSpeechSegmentDetector_SpeechStartEvent(t *testing.T) {
	vad := NewEnergyVADProvider(EnergyVADConfig{})
	det, err := NewSpeechSegmentDetector(SpeechSegmentConfig{VAD: vad})
	require.NoError(t, err)

	event, err := det.ProcessFrame(context.Background(), speechFrame(0))
	require.NoError(t, err)
	require.NotNil(t, event)
	assert.Equal(t, SegmentEventSpeechStart, event.Type)
}

func TestSpeechSegmentDetector_SpeechEndEvent(t *testing.T) {
	vad := NewEnergyVADProvider(EnergyVADConfig{SpeechHoldMs: 1}) // minimal hold
	det, _ := NewSpeechSegmentDetector(SpeechSegmentConfig{
		VAD:                vad,
		SilenceThresholdMs: 50, // 50ms of silence to trigger end
		MinSpeechMs:        20, // min 20ms speech
	})
	ctx := context.Background()

	// 30ms of speech (3 x 10ms frames)
	for i := 0; i < 3; i++ {
		det.ProcessFrame(ctx, speechFrame(time.Duration(i*10)*time.Millisecond))
	}

	// Enough silence to trigger end (6 x 10ms = 60ms > 50ms threshold)
	var endEvent *SegmentEvent
	for i := 0; i < 6; i++ {
		ts := time.Duration(30+i*10) * time.Millisecond
		ev, _ := det.ProcessFrame(ctx, silenceFrame(ts))
		if ev != nil && ev.Type == SegmentEventSpeechEnd {
			endEvent = ev
			break
		}
	}
	require.NotNil(t, endEvent)
	assert.Equal(t, SegmentEventSpeechEnd, endEvent.Type)
}

func TestSpeechSegmentDetector_TooShortSegmentDiscarded(t *testing.T) {
	vad := NewEnergyVADProvider(EnergyVADConfig{SpeechHoldMs: 1})
	det, _ := NewSpeechSegmentDetector(SpeechSegmentConfig{
		VAD:                vad,
		SilenceThresholdMs: 30,
		MinSpeechMs:        100, // requires 100ms of speech
	})
	ctx := context.Background()

	// Only 10ms of speech (1 frame)
	det.ProcessFrame(ctx, speechFrame(0))

	// 50ms of silence — triggers end check but speech < MinSpeechMs, so discarded
	var gotEnd bool
	for i := 0; i < 5; i++ {
		ts := time.Duration(10+i*10) * time.Millisecond
		ev, _ := det.ProcessFrame(ctx, silenceFrame(ts))
		if ev != nil && ev.Type == SegmentEventSpeechEnd {
			gotEnd = true
		}
	}
	assert.False(t, gotEnd, "short segment should be discarded, not emit SpeechEnd")
}

func TestSpeechSegmentDetector_ForcedFlush(t *testing.T) {
	vad := NewEnergyVADProvider(EnergyVADConfig{})
	det, _ := NewSpeechSegmentDetector(SpeechSegmentConfig{
		VAD:          vad,
		MaxSegmentMs: 50, // force flush after 50ms
	})
	ctx := context.Background()

	var flushed bool
	// 6 x 10ms speech frames = 60ms > 50ms max
	for i := 0; i < 6; i++ {
		ev, _ := det.ProcessFrame(ctx, speechFrame(time.Duration(i*10)*time.Millisecond))
		if ev != nil && ev.Type == SegmentEventForcedFlush {
			flushed = true
			break
		}
	}
	assert.True(t, flushed)
}

func TestSpeechSegmentDetector_NilVAD(t *testing.T) {
	_, err := NewSpeechSegmentDetector(SpeechSegmentConfig{})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "VAD provider is required")
}
