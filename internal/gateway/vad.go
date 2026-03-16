package gateway

import (
	"context"
	"errors"
	"fmt"
	"math"
	"time"
)

// VADMode controls the aggressiveness of voice activity detection.
// Higher values filter out more non-speech at the cost of missing soft speech.
type VADMode int

const (
	VADModeQuality    VADMode = 0 // least aggressive, misses less speech
	VADModeBalanced   VADMode = 1
	VADModeAggressive VADMode = 2
	VADModeMax        VADMode = 3 // most aggressive, best noise filtering
)

// VADFrame is a single audio frame submitted for VAD analysis.
type VADFrame struct {
	PCM        []int16       // raw PCM samples at 16 kHz
	DurationMs int           // 10, 20, or 30 ms (must match frame size)
	Timestamp  time.Duration // position in the stream
}

// VADResult is the result for a single frame.
type VADResult struct {
	IsSpeech   bool
	Frame      VADFrame
	Confidence float64 // 0.0 = definitely silence, 1.0 = definitely speech
}

// ErrVADInvalidFrame is returned when a frame has an unsupported duration or sample count.
var ErrVADInvalidFrame = errors.New("vad: invalid frame (must be 10, 20, or 30 ms at 16 kHz)")

// VADProvider analyses audio frames for voice activity.
type VADProvider interface {
	// Process analyses a single audio frame.
	// Returns ErrVADInvalidFrame if the frame is not valid.
	Process(ctx context.Context, frame VADFrame) (*VADResult, error)

	// Name returns the provider identifier.
	Name() string
}

// ValidFrameDurations are the valid frame durations for WebRTC VAD (ms).
var ValidFrameDurations = map[int]bool{10: true, 20: true, 30: true}

// FrameSamplesAt16kHz returns the expected sample count for a frame of durationMs at 16 kHz.
func FrameSamplesAt16kHz(durationMs int) int {
	return 16 * durationMs // 16 samples per ms at 16 kHz
}

// ---------------------------------------------------------------------------
// EnergyVADProvider — pure-Go fallback (no CGo dependency)
// ---------------------------------------------------------------------------

// EnergyVADConfig configures the energy-based VAD.
type EnergyVADConfig struct {
	// RMSThreshold is the minimum RMS energy for a frame to be classified as speech.
	// Range 0.0–1.0 (normalised). Default: 0.02.
	RMSThreshold float64
	// SpeechHoldMs keeps VAD in "speech" state for this many ms after the last
	// speech frame (prevents rapid toggling on pauses). Default: 300 ms.
	SpeechHoldMs int
}

// EnergyVADProvider classifies speech using RMS energy thresholding.
// It is always available (no CGo dependency) and suitable for telephony-quality audio.
type EnergyVADProvider struct {
	threshold    float64
	holdMs       int
	lastSpeechAt time.Duration
}

// NewEnergyVADProvider creates an EnergyVADProvider.
func NewEnergyVADProvider(cfg EnergyVADConfig) *EnergyVADProvider {
	t := cfg.RMSThreshold
	if t <= 0 {
		t = 0.02
	}
	hold := cfg.SpeechHoldMs
	if hold <= 0 {
		hold = 300
	}
	return &EnergyVADProvider{threshold: t, holdMs: hold}
}

func (e *EnergyVADProvider) Name() string { return "energy_vad" }

func (e *EnergyVADProvider) Process(_ context.Context, frame VADFrame) (*VADResult, error) {
	if !ValidFrameDurations[frame.DurationMs] {
		return nil, ErrVADInvalidFrame
	}
	expectedSamples := FrameSamplesAt16kHz(frame.DurationMs)
	if len(frame.PCM) != expectedSamples {
		return nil, fmt.Errorf("%w: expected %d samples for %dms frame, got %d",
			ErrVADInvalidFrame, expectedSamples, frame.DurationMs, len(frame.PCM))
	}

	rms := computeRMS(frame.PCM)
	inHold := frame.Timestamp-e.lastSpeechAt <= time.Duration(e.holdMs)*time.Millisecond
	isSpeech := rms >= e.threshold || inHold

	if rms >= e.threshold {
		e.lastSpeechAt = frame.Timestamp
	}

	confidence := math.Min(rms/e.threshold, 1.0)
	if inHold && rms < e.threshold {
		confidence = 0.5 // mid-confidence during hold period
	}

	return &VADResult{
		IsSpeech:   isSpeech,
		Frame:      frame,
		Confidence: confidence,
	}, nil
}

func computeRMS(samples []int16) float64 {
	if len(samples) == 0 {
		return 0
	}
	var sum float64
	for _, s := range samples {
		v := float64(s) / 32768.0
		sum += v * v
	}
	return math.Sqrt(sum / float64(len(samples)))
}

// ---------------------------------------------------------------------------
// SpeechSegmentDetector — end-of-utterance detection
// ---------------------------------------------------------------------------

// SpeechSegmentConfig controls end-of-utterance detection.
type SpeechSegmentConfig struct {
	VAD                VADProvider
	SilenceThresholdMs int // silence duration before triggering end-of-utterance; default 700ms
	MinSpeechMs        int // minimum speech duration to be considered an utterance; default 300ms
	MaxSegmentMs       int // max segment before forced flush; default 30,000ms
}

// SegmentEvent signals a speech segment boundary.
type SegmentEvent struct {
	Type       SegmentEventType
	StartTime  time.Duration
	EndTime    time.Duration
	FrameCount int
}

// SegmentEventType identifies the kind of segment boundary.
type SegmentEventType string

const (
	SegmentEventSpeechStart SegmentEventType = "speech_start"
	SegmentEventSpeechEnd   SegmentEventType = "speech_end"    // end-of-utterance
	SegmentEventForcedFlush SegmentEventType = "forced_flush"
)

// SpeechSegmentDetector wraps a VADProvider and emits SegmentEvents.
type SpeechSegmentDetector struct {
	cfg         SpeechSegmentConfig
	inSpeech    bool
	speechStart time.Duration
	silenceMs   int
	speechMs    int
	frameCount  int
}

// NewSpeechSegmentDetector creates a SpeechSegmentDetector.
func NewSpeechSegmentDetector(cfg SpeechSegmentConfig) (*SpeechSegmentDetector, error) {
	if cfg.VAD == nil {
		return nil, fmt.Errorf("speech segment detector: VAD provider is required")
	}
	if cfg.SilenceThresholdMs <= 0 {
		cfg.SilenceThresholdMs = 700
	}
	if cfg.MinSpeechMs <= 0 {
		cfg.MinSpeechMs = 300
	}
	if cfg.MaxSegmentMs <= 0 {
		cfg.MaxSegmentMs = 30000
	}
	return &SpeechSegmentDetector{cfg: cfg}, nil
}

// ProcessFrame submits a frame and returns a SegmentEvent if a boundary is detected,
// or nil if no boundary yet.
func (d *SpeechSegmentDetector) ProcessFrame(ctx context.Context, frame VADFrame) (*SegmentEvent, error) {
	result, err := d.cfg.VAD.Process(ctx, frame)
	if err != nil {
		return nil, err
	}

	d.frameCount++

	if result.IsSpeech {
		d.silenceMs = 0
		d.speechMs += frame.DurationMs
		if !d.inSpeech {
			d.inSpeech = true
			d.speechStart = frame.Timestamp
			return &SegmentEvent{
				Type:       SegmentEventSpeechStart,
				StartTime:  frame.Timestamp,
				FrameCount: d.frameCount,
			}, nil
		}
		// Force flush on max segment duration.
		if d.speechMs >= d.cfg.MaxSegmentMs {
			event := &SegmentEvent{
				Type:       SegmentEventForcedFlush,
				StartTime:  d.speechStart,
				EndTime:    frame.Timestamp + time.Duration(frame.DurationMs)*time.Millisecond,
				FrameCount: d.frameCount,
			}
			d.resetSegment()
			return event, nil
		}
	} else {
		if d.inSpeech {
			d.silenceMs += frame.DurationMs
			if d.silenceMs >= d.cfg.SilenceThresholdMs {
				if d.speechMs >= d.cfg.MinSpeechMs {
					event := &SegmentEvent{
						Type:       SegmentEventSpeechEnd,
						StartTime:  d.speechStart,
						EndTime:    frame.Timestamp,
						FrameCount: d.frameCount,
					}
					d.resetSegment()
					return event, nil
				}
				// Below minimum speech duration; discard.
				d.resetSegment()
			}
		}
	}
	return nil, nil
}

func (d *SpeechSegmentDetector) resetSegment() {
	d.inSpeech = false
	d.speechStart = 0
	d.silenceMs = 0
	d.speechMs = 0
	d.frameCount = 0
}
