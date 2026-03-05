package gateway

type GatewaySkillProfile struct {
	SkillID            string
	ExternalAPICalled  string
	WhyGatewayNotHands string
	LatencyBudgetMs    int
	DelegatesToBrain   bool
}

func GatewaySkillProfiles() map[string]GatewaySkillProfile {
	return map[string]GatewaySkillProfile{
		"asr": {
			SkillID:            "asr",
			ExternalAPICalled:  "Whisper API or local whisper.cpp",
			WhyGatewayNotHands: "Voice input must be transcribed before Brain can classify intent",
			LatencyBudgetMs:    3000,
		},
		"gemini-stt": {
			SkillID:            "gemini-stt",
			ExternalAPICalled:  "Google Gemini API",
			WhyGatewayNotHands: "Premium STT with speaker labels in pre-Brain processing",
			LatencyBudgetMs:    5000,
		},
		"openai-tts": {
			SkillID:            "openai-tts",
			ExternalAPICalled:  "OpenAI Audio Speech API",
			WhyGatewayNotHands: "Voice output is synthesized after Brain response generation",
			LatencyBudgetMs:    2000,
		},
		"sag": {
			SkillID:            "sag",
			ExternalAPICalled:  "ElevenLabs API",
			WhyGatewayNotHands: "Premium post-Brain TTS path for voice output quality",
			LatencyBudgetMs:    3000,
		},
		"voice-wake-say": {
			SkillID:            "voice-wake-say",
			ExternalAPICalled:  "None (local macOS say)",
			WhyGatewayNotHands: "Local fallback TTS to avoid API cost and latency",
			LatencyBudgetMs:    500,
		},
		"whatsapp-styling-guide": {
			SkillID:            "whatsapp-styling-guide",
			ExternalAPICalled:  "None",
			WhyGatewayNotHands: "Pure channel formatting transformation on egress path",
			LatencyBudgetMs:    10,
		},
		"vocal-chat": {
			SkillID:            "vocal-chat",
			ExternalAPICalled:  "Combines ASR + OpenAI TTS",
			WhyGatewayNotHands: "Gateway orchestrates end-to-end voice I/O envelope",
			LatencyBudgetMs:    5000,
		},
		"autoresponder": {
			SkillID:            "autoresponder",
			ExternalAPICalled:  "Brain service via internal gRPC",
			WhyGatewayNotHands: "Intercepts ingress in gateway and delegates response generation to Brain",
			LatencyBudgetMs:    8000,
			DelegatesToBrain:   true,
		},
	}
}
