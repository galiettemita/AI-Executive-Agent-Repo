package eq

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestEmotionToEQContext_Frustration_NoProfile(t *testing.T) {
	t.Parallel()
	ctx := EmotionToEQContext("frustration", nil)
	assert.Equal(t, "empathetic", ctx.ToneModifier)
	assert.Equal(t, "concise", ctx.Verbosity)
	assert.False(t, ctx.EmojiUse)
	assert.Equal(t, "medium", ctx.UrgencyLevel)
	assert.Equal(t, "detected_emotion", ctx.Source)
}

func TestEmotionToEQContext_Joy_NoProfile(t *testing.T) {
	t.Parallel()
	ctx := EmotionToEQContext("joy", nil)
	assert.Equal(t, "conversational", ctx.ToneModifier)
	assert.True(t, ctx.EmojiUse)
	assert.Equal(t, "casual", ctx.Formality)
}

func TestEmotionToEQContext_Anxiety_NoProfile(t *testing.T) {
	t.Parallel()
	ctx := EmotionToEQContext("anxiety", nil)
	assert.Equal(t, "reassuring", ctx.ToneModifier)
	assert.Equal(t, "concise", ctx.Verbosity)
	assert.Equal(t, "medium", ctx.UrgencyLevel)
}

func TestEmotionToEQContext_Sadness_NoProfile(t *testing.T) {
	t.Parallel()
	ctx := EmotionToEQContext("sadness", nil)
	assert.Equal(t, "empathetic", ctx.ToneModifier)
	assert.Equal(t, "standard", ctx.Verbosity)
	assert.Equal(t, "low", ctx.UrgencyLevel)
}

func TestEmotionToEQContext_Excitement_NoProfile(t *testing.T) {
	t.Parallel()
	ctx := EmotionToEQContext("excitement", nil)
	assert.Equal(t, "enthusiastic", ctx.ToneModifier)
	assert.True(t, ctx.EmojiUse)
	assert.Equal(t, "casual", ctx.Formality)
}

func TestEmotionToEQContext_Confusion_NoProfile(t *testing.T) {
	t.Parallel()
	ctx := EmotionToEQContext("confusion", nil)
	assert.Equal(t, "direct", ctx.ToneModifier)
	assert.Equal(t, "verbose", ctx.Verbosity)
}

func TestEmotionToEQContext_ProfileOverridesEmotion(t *testing.T) {
	t.Parallel()
	profile := &CommunicationProfile{
		Formality:  "formal",
		Directness: 0.9,
	}
	ctx := EmotionToEQContext("frustration", profile)
	// Profile overrides tone via high directness → "direct"
	assert.Equal(t, "direct", ctx.ToneModifier)
	// Profile overrides formality
	assert.Equal(t, "formal", ctx.Formality)
	// Verbosity not overridden — still concise from frustration
	assert.Equal(t, "concise", ctx.Verbosity)
	assert.Equal(t, "explicit_profile", ctx.Source)
}

func TestEmotionToEQContext_ProfileVerbosity(t *testing.T) {
	t.Parallel()
	profile := &CommunicationProfile{
		Verbosity: "detailed",
	}
	ctx := EmotionToEQContext("frustration", profile)
	// Verbosity overridden by profile
	assert.Equal(t, "verbose", ctx.Verbosity)
	// Tone still from emotion (directness < 0.7)
	assert.Equal(t, "empathetic", ctx.ToneModifier)
}

func TestEmotionToEQContext_NeutralDefault(t *testing.T) {
	t.Parallel()
	ctx := EmotionToEQContext("neutral", nil)
	assert.Equal(t, "neutral", ctx.ToneModifier)
	assert.Equal(t, "standard", ctx.Verbosity)
	assert.Equal(t, "semi_formal", ctx.Formality)
}

func TestEmotionToEQContext_UnknownEmotion_UsesDefault(t *testing.T) {
	t.Parallel()
	ctx := EmotionToEQContext("totally_unknown_emotion", nil)
	assert.Equal(t, DefaultEQContext().ToneModifier, ctx.ToneModifier)
	assert.Equal(t, DefaultEQContext().Verbosity, ctx.Verbosity)
}

func TestEmotionToEQContext_NegativeEmotion(t *testing.T) {
	t.Parallel()
	// "negative" is what DetectEmotion returns for negative sentiment
	ctx := EmotionToEQContext("negative", nil)
	assert.Equal(t, "empathetic", ctx.ToneModifier)
	assert.Equal(t, "concise", ctx.Verbosity)
}

func TestEmotionToEQContext_UrgentEmotion(t *testing.T) {
	t.Parallel()
	// "urgent" is what DetectEmotion returns for urgent messages
	ctx := EmotionToEQContext("urgent", nil)
	assert.Equal(t, "reassuring", ctx.ToneModifier)
	assert.Equal(t, "medium", ctx.UrgencyLevel)
}
