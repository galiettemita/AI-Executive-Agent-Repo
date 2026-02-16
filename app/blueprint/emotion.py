from __future__ import annotations

from app.blueprint.contracts import EmotionState


_FRUSTRATED_HINTS = ("angry", "frustrated", "annoyed", "wtf", "ridiculous", "upset")
_RUSHED_HINTS = ("asap", "urgent", "right now", "hurry", "quickly")
_STRESSED_HINTS = ("overwhelmed", "stressed", "burned out", "panic", "anxious")
_EXCITED_HINTS = ("excited", "awesome", "amazing", "great news", "let's go")
_POSITIVE_HINTS = ("thanks", "thank you", "appreciate", "happy", "good", "nice")


def detect_emotion(*, text: str, transcription_confidence: float | None = None) -> EmotionState:
    normalized = (text or "").lower()

    if any(x in normalized for x in _FRUSTRATED_HINTS):
        return EmotionState.FRUSTRATED
    if any(x in normalized for x in _STRESSED_HINTS):
        return EmotionState.STRESSED
    if any(x in normalized for x in _RUSHED_HINTS):
        return EmotionState.RUSHED
    if any(x in normalized for x in _EXCITED_HINTS):
        return EmotionState.EXCITED
    if any(x in normalized for x in _POSITIVE_HINTS):
        return EmotionState.POSITIVE

    # Low-confidence voice transcripts trend toward neutral unless explicit emotion is clear.
    if transcription_confidence is not None and transcription_confidence < 0.55:
        return EmotionState.NEUTRAL
    return EmotionState.NEUTRAL
