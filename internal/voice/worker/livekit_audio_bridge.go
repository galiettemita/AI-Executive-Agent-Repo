package worker

import (
	"context"
	"fmt"
	"time"

	"github.com/brevio/brevio/internal/gateway"
)

// AudioFrameSource delivers raw PCM audio frames.
// In production this is backed by a LiveKit RTP track reader.
type AudioFrameSource interface {
	// ReadFrame returns the next PCM frame or (nil, nil) when exhausted.
	ReadFrame(ctx context.Context) ([]byte, error)
	// SampleRate returns the Hz rate of the PCM data.
	SampleRate() int
}

// LiveKitAudioBridgeConfig configures the bridge.
type LiveKitAudioBridgeConfig struct {
	SessionID      string
	WorkspaceID    string
	AudioSource    AudioFrameSource
	DeepgramClient *gateway.DeepgramWSClient
	Pipeline       *gateway.StreamingPipeline
	BargeIn        *BargeInController
	VAD            gateway.VADProvider // optional
	SessionStore   SessionStore       // optional — for persisting transcript turns
}

// LiveKitAudioBridge connects a PCM audio source to the Deepgram WS and the streaming pipeline.
type LiveKitAudioBridge struct {
	cfg LiveKitAudioBridgeConfig
}

// NewLiveKitAudioBridge creates a LiveKitAudioBridge.
func NewLiveKitAudioBridge(cfg LiveKitAudioBridgeConfig) (*LiveKitAudioBridge, error) {
	if cfg.SessionID == "" {
		return nil, fmt.Errorf("audio bridge: SessionID required")
	}
	if cfg.AudioSource == nil {
		return nil, fmt.Errorf("audio bridge: AudioSource required")
	}
	if cfg.DeepgramClient == nil {
		return nil, fmt.Errorf("audio bridge: DeepgramWSClient required")
	}
	if cfg.Pipeline == nil {
		return nil, fmt.Errorf("audio bridge: StreamingPipeline required")
	}
	return &LiveKitAudioBridge{cfg: cfg}, nil
}

// Run starts the bridge. Blocks until ctx is cancelled or a fatal error occurs.
func (b *LiveKitAudioBridge) Run(ctx context.Context) error {
	if err := b.cfg.DeepgramClient.Connect(ctx); err != nil {
		return fmt.Errorf("audio bridge: deepgram connect: %w", err)
	}
	defer b.cfg.DeepgramClient.Close()

	transcriptRaw := make(chan gateway.TranscriptEvent, 64)
	transcriptErrs := make(chan error, 1)
	audioErrs := make(chan error, 1)
	pipelineErrs := make(chan error, 8)

	go func() {
		transcriptErrs <- b.cfg.DeepgramClient.StreamTranscripts(ctx, transcriptRaw)
	}()
	go func() {
		audioErrs <- b.sendAudioFrames(ctx)
	}()

	for {
		select {
		case <-ctx.Done():
			_ = b.cfg.DeepgramClient.CloseStream()
			return ctx.Err()

		case err := <-audioErrs:
			_ = b.cfg.DeepgramClient.CloseStream()
			if err != nil && err != context.Canceled {
				return fmt.Errorf("audio bridge: audio sender: %w", err)
			}
			return nil

		case err := <-transcriptErrs:
			if err != nil && err != context.Canceled {
				return fmt.Errorf("audio bridge: deepgram receiver: %w", err)
			}
			return nil

		case event, ok := <-transcriptRaw:
			if !ok {
				return nil
			}
			// Barge-in: interim transcripts mean user is speaking — interrupt TTS.
			if !event.IsFinal && b.cfg.BargeIn != nil {
				b.cfg.BargeIn.OnUserSpeechStart()
			}
			if event.IsFinal {
				go func(e gateway.TranscriptEvent) {
					evCh := make(chan gateway.TranscriptEvent, 1)
					audioSink := make(chan gateway.AudioChunkEvent, 128)
					evCh <- e
					close(evCh)

					pCtx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
					if b.cfg.BargeIn != nil {
						cleanup := b.cfg.BargeIn.RegisterTTSContext(cancel)
						defer cleanup()
					}
					defer cancel()

					if err := b.cfg.Pipeline.Process(pCtx, evCh, audioSink); err != nil {
						pipelineErrs <- err
					}
					for range audioSink {
					} // drain
				}(event)
			}

		case <-pipelineErrs:
			// Pipeline errors are non-fatal — log in production
		}
	}
}

func (b *LiveKitAudioBridge) sendAudioFrames(ctx context.Context) error {
	for {
		frame, err := b.cfg.AudioSource.ReadFrame(ctx)
		if err != nil {
			return err
		}
		if frame == nil {
			return nil
		}
		// Optional VAD filtering — skip non-speech frames
		if b.cfg.VAD != nil {
			samples := pcm16BytesToInt16(frame)
			dur := frameDurMs(len(frame), b.cfg.AudioSource.SampleRate())
			vf := gateway.VADFrame{PCM: samples, DurationMs: dur}
			result, err := b.cfg.VAD.Process(ctx, vf)
			if err == nil && !result.IsSpeech {
				continue
			}
		}
		if err := b.cfg.DeepgramClient.SendAudioFrame(frame); err != nil {
			return fmt.Errorf("send frame: %w", err)
		}
	}
}

// pcm16BytesToInt16 converts little-endian PCM16 bytes to int16 slice.
func pcm16BytesToInt16(b []byte) []int16 {
	if len(b)%2 != 0 {
		b = b[:len(b)-1]
	}
	samples := make([]int16, len(b)/2)
	for i := range samples {
		samples[i] = int16(b[2*i]) | int16(b[2*i+1])<<8
	}
	return samples
}

// frameDurMs returns duration in ms for a PCM16 frame.
func frameDurMs(byteLen, sampleRate int) int {
	if sampleRate <= 0 {
		return 0
	}
	return (byteLen / 2) * 1000 / sampleRate
}
