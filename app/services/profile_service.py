# backend/app/services/profile_service.py

from __future__ import annotations

import json
from datetime import datetime
from typing import Dict, Any

from sqlalchemy.orm import Session

from app.db.models import UserProfile
from app.services.encryption_service import encrypt_pii, decrypt_pii


SENSITIVE_FIELDS = {"email", "phone", "address"}


def _encrypt_profile(data: Dict[str, Any]) -> Dict[str, Any]:
    out = dict(data)
    for key in SENSITIVE_FIELDS:
        if key in out and out[key]:
            out[key] = encrypt_pii(str(out[key]))
    return out


def _decrypt_profile(data: Dict[str, Any]) -> Dict[str, Any]:
    out = dict(data)
    for key in SENSITIVE_FIELDS:
        if key in out and out[key]:
            try:
                out[key] = decrypt_pii(str(out[key]))
            except Exception:
                # leave as-is if already plaintext or key mismatch
                pass
    return out


def get_profile(db: Session, user_id: str) -> Dict[str, Any]:
    row = db.query(UserProfile).filter(UserProfile.user_id == user_id).first()
    if not row or not row.data_json:
        return {}
    try:
        payload = json.loads(row.data_json)
        return _decrypt_profile(payload if isinstance(payload, dict) else {})
    except Exception:
        return {}


def update_profile(db: Session, user_id: str, patch: Dict[str, Any]) -> Dict[str, Any]:
    current = get_profile(db, user_id)
    current.update(patch or {})
    encrypted = _encrypt_profile(current)

    row = db.query(UserProfile).filter(UserProfile.user_id == user_id).first()
    if not row:
        row = UserProfile(user_id=user_id, data_json=json.dumps(encrypted), created_at=datetime.utcnow())
        db.add(row)
    else:
        row.data_json = json.dumps(encrypted)
        row.updated_at = datetime.utcnow()
    db.commit()
    return current
