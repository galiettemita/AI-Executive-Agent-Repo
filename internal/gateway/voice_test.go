package gateway

import "testing"

func TestVoicePipelineDefaults(t *testing.T) {
	t.Parallel()

	cfg := DefaultVoicePipelineConfig()
	if cfg.STTPrimaryProvider != "openai_whisper" || cfg.TTSProvider != "openai_tts" {
		t.Fatalf("unexpected voice pipeline providers: %+v", cfg)
	}
	if cfg.ConfidenceThreshold != 0.7 || cfg.MaxAudioDurationSeconds != 120 || cfg.MaxResponseChars != 4096 {
		t.Fatalf("unexpected voice pipeline limits: %+v", cfg)
	}
	if cfg.DefaultVoice != "nova" {
		t.Fatalf("unexpected default voice: %s", cfg.DefaultVoice)
	}

	voices := AllowedTTSVoices()
	if len(voices) != 11 {
		t.Fatalf("unexpected voice count: %d", len(voices))
	}
	if !IsLowConfidenceTranscription(0.69) || IsLowConfidenceTranscription(0.70) {
		t.Fatalf("unexpected confidence threshold behavior")
	}
	if got := VoiceOutputFormatForChannel("imessage"); got != "m4a" {
		t.Fatalf("unexpected imessage voice format: %s", got)
	}
}
