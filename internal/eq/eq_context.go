package eq

// EQContext carries the resolved emotional tone parameters to be injected into LLM prompts.
// It is the output of the EQ routing layer, after reconciling detected emotion with
// explicit CommunicationProfile overrides.
type EQContext struct {
	Formality    string `json:"formality"`     // formal | semi_formal | casual
	Verbosity    string `json:"verbosity"`     // concise | standard | verbose
	ToneModifier string `json:"tone_modifier"` // empathetic | reassuring | conversational | direct | enthusiastic | neutral
	EmojiUse     bool   `json:"emoji_use"`
	UrgencyLevel string `json:"urgency_level"` // low | medium | high
	Source       string `json:"source"`        // explicit_profile | detected_emotion | default
}

// DefaultEQContext returns the neutral baseline context used when EQ routing is disabled
// or when neither emotion nor profile data is available.
func DefaultEQContext() EQContext {
	return EQContext{
		Formality:    "semi_formal",
		Verbosity:    "standard",
		ToneModifier: "neutral",
		EmojiUse:     false,
		UrgencyLevel: "low",
		Source:       "default",
	}
}
