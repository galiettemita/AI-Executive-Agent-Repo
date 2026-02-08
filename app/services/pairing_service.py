from __future__ import annotations

import secrets
from datetime import datetime, timedelta
from typing import Optional

import jwt
from sqlalchemy.orm import Session

from app.core.config import settings
from app.db.models import DevicePairingCode


def _now() -> datetime:
    return datetime.utcnow()


def _code_ttl_minutes(default_minutes: int | None = None) -> int:
    if default_minutes is not None and default_minutes > 0:
        return default_minutes
    return max(5, int(getattr(settings, "PAIRING_CODE_TTL_MINUTES", 30)))


def generate_pairing_code(db: Session, user_id: str, ttl_minutes: int | None = None) -> DevicePairingCode:
    ttl = _code_ttl_minutes(ttl_minutes)
    expires_at = _now() + timedelta(minutes=ttl)

    # Generate short human-friendly code
    for _ in range(5):
        raw = secrets.token_urlsafe(6)
        code = raw.replace("-", "").replace("_", "")[:8]
        if not code:
            continue
        existing = db.query(DevicePairingCode).filter(DevicePairingCode.code == code).first()
        if existing is None:
            record = DevicePairingCode(
                user_id=user_id,
                code=code,
                expires_at=expires_at,
                created_at=_now(),
            )
            db.add(record)
            db.commit()
            db.refresh(record)
            return record

    # Fallback to longer code
    code = secrets.token_hex(6)
    record = DevicePairingCode(
        user_id=user_id,
        code=code,
        expires_at=expires_at,
        created_at=_now(),
    )
    db.add(record)
    db.commit()
    db.refresh(record)
    return record


def consume_pairing_code(db: Session, code: str) -> Optional[DevicePairingCode]:
    if not code:
        return None
    record = db.query(DevicePairingCode).filter(DevicePairingCode.code == code).first()
    if not record:
        return None
    if record.used_at is not None:
        return None
    if record.expires_at and record.expires_at < _now():
        return None

    record.used_at = _now()
    db.commit()
    db.refresh(record)
    return record


def issue_access_token(user_id: str, hours: int | None = None) -> tuple[str, datetime]:
    ttl_hours = hours if hours and hours > 0 else int(getattr(settings, "JWT_ACCESS_TTL_HOURS", 168))
    expires_at = _now() + timedelta(hours=ttl_hours)
    payload = {
        "sub": user_id,
        "user_id": user_id,
        "exp": int(expires_at.timestamp()),
        "iat": int(_now().timestamp()),
    }
    token = jwt.encode(payload, settings.JWT_SECRET, algorithm="HS256")
    return token, expires_at


def pair_with_code(db: Session, code: str) -> Optional[dict]:
    record = consume_pairing_code(db, code)
    if not record:
        return None
    token, expires_at = issue_access_token(record.user_id)
    return {
        "access_token": token,
        "token_type": "bearer",
        "expires_at": expires_at.isoformat(),
        "user_id": record.user_id,
    }
