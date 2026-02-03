# backend/app/services/preferences.py

from __future__ import annotations

import json
from datetime import datetime
from typing import Dict, Tuple

from sqlalchemy.orm import Session

from app.db.models import UserPreference


def get_preferences(db: Session, user_id: str) -> Dict[str, str]:
    pref = db.query(UserPreference).filter(UserPreference.user_id == user_id).first()
    if not pref or not pref.data_json:
        return {}
    try:
        data = json.loads(pref.data_json)
        return data if isinstance(data, dict) else {}
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
    return data


def update_preferences(db: Session, user_id: str, patch: Dict[str, str]) -> Dict[str, str]:
    current = get_preferences(db, user_id)
    current.update(patch)
    return _upsert(db, user_id, current)


def is_onboarding_complete(prefs: Dict[str, str]) -> bool:
    return prefs.get("onboarding_complete") is True


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
            "Perfect — you’re all set. "
            "Tell me what you want to do, and I’ll help.",
            prefs,
        )

    return "", prefs
