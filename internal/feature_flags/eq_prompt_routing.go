package feature_flags

import "os"

// FeatureEQPromptRouting is the environment variable controlling EQ prompt routing.
const FeatureEQPromptRouting = "FEATURE_EQ_PROMPT_ROUTING"

// EQPromptRoutingEnabled returns true when EQ emotional context should be injected
// into LLM prompts. Set FEATURE_EQ_PROMPT_ROUTING=true in the environment to enable.
// Default: false (disabled for gradual rollout).
func EQPromptRoutingEnabled() bool {
	return os.Getenv(FeatureEQPromptRouting) == "true"
}
