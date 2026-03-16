package gateway

import "strings"

type VoicePipelineConfig struct {
	STTPrimaryProvider      string
	STTFallbackProvider     string
	TTSProvider             string
	ConfidenceThreshold     float64
	MaxAudioDurationSeconds int
	MaxResponseChars        int
	DefaultVoice            string
}

func DefaultVoicePipelineConfig() VoicePipelineConfig {
	return VoicePipelineConfig{
		STTPrimaryProvider:      "deepgram_nova3",
		STTFallbackProvider:     "openai_whisper",
		TTSProvider:             "openai_tts",
		ConfidenceThreshold:     0.7,
		MaxAudioDurationSeconds: 120,
		MaxResponseChars:        4096,
		DefaultVoice:            "nova",
	}
}

func AllowedTTSVoices() []string {
	return []string{"alloy", "echo", "fable", "onyx", "nova", "shimmer"}
}

func IsLowConfidenceTranscription(confidence float64) bool {
	return confidence < DefaultVoicePipelineConfig().ConfidenceThreshold
}

func VoiceOutputFormatForChannel(channel string) string {
	switch strings.ToLower(strings.TrimSpace(channel)) {
	case "whatsapp":
		return "ogg/opus"
	case "imessage":
		return "m4a"
	default:
		return "ogg/opus"
	}
}
