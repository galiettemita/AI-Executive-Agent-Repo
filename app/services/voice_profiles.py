# app/services/voice_profiles.py

from __future__ import annotations

from typing import Dict


VOICE_PROFILES: Dict[str, str] = {
    "calm": "21m00Tcm4TlvDq8ikWAM",
    "bright": "EXAVITQu4vr4xnSDxMaL",
    "warm": "AZnzlk1XvdvUeBnXmlld",
}


def resolve_voice_id(profile: str | None) -> str:
    if not profile:
        return VOICE_PROFILES["calm"]
    return VOICE_PROFILES.get(profile, VOICE_PROFILES["calm"])
