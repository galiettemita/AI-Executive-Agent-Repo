package gateway

import (
	"strings"
	"time"
)

type STTResult struct {
	Text       string
	Confidence float64
	Provider   string
}

func SupportedVoiceInputFormats() []string {
	return []string{"audio/ogg", "audio/opus", "audio/m4a", "audio/mpeg", "audio/wav"}
}

func IsSupportedVoiceInput(mime string) bool {
	normalized := strings.ToLower(strings.TrimSpace(mime))
	for _, allowed := range SupportedVoiceInputFormats() {
		if normalized == allowed {
			return true
		}
	}
	return false
}

func MaxInboundAudioDuration() time.Duration {
	return 120 * time.Second
}

func ShouldUseTTS(ttsEnabled bool, inboundWasAudio bool) bool {
	return ttsEnabled && inboundWasAudio
}

func SynthesizeTranscription(primary STTResult, fallback STTResult, threshold float64) STTResult {
	if threshold <= 0 {
		threshold = 0.7
	}
	if primary.Confidence >= threshold {
		return primary
	}
	if fallback.Text != "" {
		return fallback
	}
	if primary.Text == "" {
		return primary
	}
	primary.Text = "[low confidence transcription] " + primary.Text
	return primary
}
