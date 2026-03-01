package gateway

import (
	"testing"
	"time"
)

func TestVoicePipelineDecisions(t *testing.T) {
	t.Parallel()

	if !IsSupportedVoiceInput("audio/ogg") || IsSupportedVoiceInput("video/mp4") {
		t.Fatal("unexpected voice input support decision")
	}
	if MaxInboundAudioDuration() != 120*time.Second {
		t.Fatalf("unexpected max inbound audio duration: %s", MaxInboundAudioDuration())
	}
	if !ShouldUseTTS(true, true) || ShouldUseTTS(true, false) {
		t.Fatal("unexpected tts trigger decision")
	}
}

func TestSynthesizeTranscription(t *testing.T) {
	t.Parallel()

	primary := STTResult{Text: "hello world", Confidence: 0.5, Provider: "whisper"}
	fallback := STTResult{Text: "hello world fallback", Confidence: 0.8, Provider: "google"}
	final := SynthesizeTranscription(primary, fallback, 0.7)
	if final.Provider != "google" {
		t.Fatalf("expected fallback provider, got %+v", final)
	}

	final = SynthesizeTranscription(primary, STTResult{}, 0.7)
	if final.Text[:len("[low confidence transcription]")] != "[low confidence transcription]" {
		t.Fatalf("expected low-confidence marker, got %+v", final)
	}
}
