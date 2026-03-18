package eq

// EmotionToEQContext converts a detected emotion and optional CommunicationProfile
// into an EQContext for LLM prompt injection.
//
// Priority rules:
//  1. Explicit CommunicationProfile overrides apply per-field.
//  2. Detected emotion fills any non-overridden fields.
//  3. DefaultEQContext fills any remaining unset fields.
func EmotionToEQContext(emotion string, profile *CommunicationProfile) EQContext {
	base := emotionDefaults(emotion)

	if profile == nil {
		base.Source = "detected_emotion"
		return base
	}

	// Apply per-field overrides from CommunicationProfile.
	// Only override if the profile field is explicitly set (non-zero).
	if profile.Formality != "" {
		base.Formality = mapProfileFormality(profile.Formality)
	}
	if profile.Verbosity != "" {
		base.Verbosity = mapProfileVerbosity(profile.Verbosity)
	}
	// EmojiUse: profile always wins if profile exists (it's a bool).
	base.EmojiUse = profile.EmojiUse
	// Directness maps to tone: high directness → direct tone.
	if profile.Directness >= 0.7 {
		base.ToneModifier = "direct"
	}

	// Determine source: if any profile field was applied, mark as explicit_profile.
	if profile.Formality != "" || profile.Directness >= 0.7 {
		base.Source = "explicit_profile"
	} else {
		base.Source = "detected_emotion"
	}
	return base
}

// mapProfileFormality converts CommunicationProfile.Formality values
// (casual/balanced/formal) to EQContext.Formality values.
func mapProfileFormality(f string) string {
	switch f {
	case "formal":
		return "formal"
	case "casual":
		return "casual"
	default: // "balanced"
		return "semi_formal"
	}
}

// mapProfileVerbosity converts CommunicationProfile.Verbosity values
// (concise/balanced/detailed) to EQContext.Verbosity values.
func mapProfileVerbosity(v string) string {
	switch v {
	case "concise":
		return "concise"
	case "detailed":
		return "verbose"
	default: // "balanced"
		return "standard"
	}
}

// emotionDefaults returns the baseline EQContext for a given detected emotion.
func emotionDefaults(emotion string) EQContext {
	switch emotion {
	case "frustration", "frustrated", "negative":
		return EQContext{
			Formality:    "semi_formal",
			Verbosity:    "concise",
			ToneModifier: "empathetic",
			EmojiUse:     false,
			UrgencyLevel: "medium",
		}
	case "joy", "positive":
		return EQContext{
			Formality:    "casual",
			Verbosity:    "standard",
			ToneModifier: "conversational",
			EmojiUse:     true,
			UrgencyLevel: "low",
		}
	case "anxiety", "urgent":
		return EQContext{
			Formality:    "semi_formal",
			Verbosity:    "concise",
			ToneModifier: "reassuring",
			EmojiUse:     false,
			UrgencyLevel: "medium",
		}
	case "sadness":
		return EQContext{
			Formality:    "semi_formal",
			Verbosity:    "standard",
			ToneModifier: "empathetic",
			EmojiUse:     false,
			UrgencyLevel: "low",
		}
	case "excitement":
		return EQContext{
			Formality:    "casual",
			Verbosity:    "standard",
			ToneModifier: "enthusiastic",
			EmojiUse:     true,
			UrgencyLevel: "low",
		}
	case "confusion":
		return EQContext{
			Formality:    "semi_formal",
			Verbosity:    "verbose",
			ToneModifier: "direct",
			EmojiUse:     false,
			UrgencyLevel: "low",
		}
	default: // "neutral" and any unknown emotion
		return DefaultEQContext()
	}
}
