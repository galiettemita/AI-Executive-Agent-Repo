# backend/app/services/preferences.py

from __future__ import annotations

import json
from datetime import datetime
from typing import Dict, Tuple

from sqlalchemy.orm import Session

from app.db.models import UserPreference
from app.core.config import settings
from app.core.redis import cache_get_json, cache_set_json


def get_preferences(db: Session, user_id: str) -> Dict[str, str]:
    cache_key = f"prefs:{user_id}"
    cached = cache_get_json(cache_key)
    if isinstance(cached, dict):
        return cached

    pref = db.query(UserPreference).filter(UserPreference.user_id == user_id).first()
    if not pref or not pref.data_json:
        return {}
    try:
        data = json.loads(pref.data_json)
        payload = data if isinstance(data, dict) else {}
        cache_set_json(cache_key, payload, ttl_seconds=settings.REDIS_PREFS_TTL_SECONDS)
        return payload
    except Exception:
        return {}


def _upsert(db: Session, user_id: str, data: Dict[str, str]) -> Dict[str, str]:
    pref = db.query(UserPreference).filter(UserPreference.user_id == user_id).first()
    payload = json.dumps(data, ensure_ascii=False)
    if not pref:
        pref = UserPreference(user_id=user_id, data_json=payload, updated_at=datetime.utcnow())
        db.add(pref)
    else:
        pref.data_json = payload
        pref.updated_at = datetime.utcnow()
    db.commit()
    cache_set_json(f"prefs:{user_id}", data, ttl_seconds=settings.REDIS_PREFS_TTL_SECONDS)
    return data


def update_preferences(db: Session, user_id: str, patch: Dict[str, str]) -> Dict[str, str]:
    current = get_preferences(db, user_id)
    current.update(patch)
    return _upsert(db, user_id, current)


def is_onboarding_complete(prefs: Dict[str, str]) -> bool:
    return prefs.get("onboarding_complete") is True


def is_wardrobe_onboarding_complete(prefs: Dict[str, str]) -> bool:
    return prefs.get("wardrobe_onboarding_complete") is True


def handle_onboarding_step(user_message: str, prefs: Dict[str, str]) -> Tuple[str, Dict[str, str]]:
    """
    Returns (reply, updated_prefs).
    If onboarding is complete, reply will be "".
    """
    step = int(prefs.get("onboarding_step") or 0)
    msg = (user_message or "").strip()

    if step <= 0:
        # Start onboarding
        prefs["onboarding_step"] = 1
        return (
            "Quick setup — what’s your style/taste? "
            "Examples: minimal, streetwear, luxury, sporty, cozy.",
            prefs,
        )

    if step == 1:
        prefs["taste"] = msg or "unspecified"
        prefs["onboarding_step"] = 2
        return (
            "Got it. What’s your usual budget range? "
            "Example: $50–$150, or “budget friendly”.",
            prefs,
        )

    if step == 2:
        prefs["budget"] = msg or "unspecified"
        prefs["onboarding_step"] = 3
        return (
            "Last one: what time windows are best for you? "
            "Example: weekdays after 6pm, mornings only, weekends.",
            prefs,
        )

    if step == 3:
        prefs["time_windows"] = msg or "unspecified"
        prefs["onboarding_step"] = 4
        prefs["onboarding_complete"] = True
        return (
            "Perfect — you're all set. "
            "Tell me what you want to do, and I'll help.",
            prefs,
        )

    return "", prefs


def handle_wardrobe_onboarding_step(user_message: str, prefs: Dict[str, str]) -> Tuple[str, Dict[str, str]]:
    """
    Returns (reply, updated_prefs) for wardrobe-specific onboarding.
    If wardrobe onboarding is complete, reply will be "".
    """
    step = int(prefs.get("wardrobe_onboarding_step") or 0)
    msg = (user_message or "").strip()

    if step <= 0:
        # Start wardrobe onboarding
        prefs["wardrobe_onboarding_step"] = 1
        return (
            "Great! Let me learn about your style preferences. "
            "What's your vibe? (e.g., classic, streetwear, minimal, sporty, boho, preppy)",
            prefs,
        )

    if step == 1:
        prefs["wardrobe_vibe"] = msg or "unspecified"
        prefs["wardrobe_onboarding_step"] = 2
        return (
            "Perfect! What are your sizes? "
            "Example: shirt M, pants 32x30, shoes 10",
            prefs,
        )

    if step == 2:
        prefs["wardrobe_sizes"] = msg or "unspecified"
        prefs["wardrobe_onboarding_step"] = 3
        return (
            "Got it. Any colors you love or want to avoid? "
            "Example: love navy and earth tones, avoid bright colors",
            prefs,
        )

    if step == 3:
        prefs["wardrobe_colors"] = msg or "unspecified"
        prefs["wardrobe_onboarding_step"] = 4
        return (
            "Last one: what's your typical budget for clothing? "
            "Example: $50-$150 per piece, or 'budget-friendly', 'mid-range', 'designer'",
            prefs,
        )

    if step == 4:
        prefs["wardrobe_budget"] = msg or "unspecified"
        prefs["wardrobe_onboarding_step"] = 5
        prefs["wardrobe_onboarding_complete"] = True
        return (
            "All set! Now tell me about the occasion or what you need help with.",
            prefs,
        )

    return "", prefs
