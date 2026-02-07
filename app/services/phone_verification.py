from __future__ import annotations

import hashlib
import logging
import secrets
from datetime import datetime, timedelta
from typing import Optional

from sqlalchemy.orm import Session
from twilio.rest import Client

from app.core.config import settings
from app.db.models import PhoneVerification
from app.services.profile_service import update_profile
from app.services.preferences import update_preferences

logger = logging.getLogger(__name__)


def _hash_code(code: str) -> str:
    secret = settings.PII_ENCRYPTION_KEY or settings.JWT_SECRET or ""
    raw = f"{secret}:{code}".encode("utf-8")
    return hashlib.sha256(raw).hexdigest()


def _generate_code(length: int) -> str:
    max_val = 10 ** length
    return str(secrets.randbelow(max_val)).zfill(length)


def _twilio_client() -> Client:
    account_sid = settings.TWILIO_ACCOUNT_SID or ""
    auth_token = settings.TWILIO_AUTH_TOKEN or ""
    if not account_sid or not auth_token:
        raise RuntimeError("Twilio credentials not configured")
    return Client(account_sid, auth_token)


def _send_sms(phone_number: str, message: str) -> None:
    from_number = settings.TWILIO_PHONE_NUMBER or ""
    if not from_number:
        raise RuntimeError("TWILIO_PHONE_NUMBER not configured")
    client = _twilio_client()
    client.messages.create(
        to=phone_number,
        from_=from_number,
        body=message,
    )


def request_phone_verification(
    db: Session,
    user_id: str,
    phone_number: str,
    force_code: Optional[str] = None,
) -> dict:
    now = datetime.utcnow()
    phone_number = (phone_number or "").strip()
    if not phone_number:
        raise ValueError("phone_number is required")

    existing = (
        db.query(PhoneVerification)
        .filter(
            PhoneVerification.user_id == user_id,
            PhoneVerification.phone_number == phone_number,
            PhoneVerification.status == "pending",
        )
        .order_by(PhoneVerification.created_at.desc())
        .first()
    )

    if existing and existing.expires_at and existing.expires_at < now:
        existing.status = "expired"
        db.commit()
        existing = None

    if existing and existing.last_sent_at:
        cooldown = settings.PHONE_VERIFICATION_RESEND_COOLDOWN_SECONDS
        delta = (now - existing.last_sent_at).total_seconds()
        if delta < cooldown:
            return {
                "ok": True,
                "status": "cooldown",
                "retry_after_seconds": int(cooldown - delta),
                "expires_at": existing.expires_at.isoformat() if existing.expires_at else None,
            }

    code = force_code or _generate_code(settings.PHONE_VERIFICATION_CODE_LENGTH)
    code_hash = _hash_code(code)
    expires_at = now + timedelta(minutes=settings.PHONE_VERIFICATION_CODE_TTL_MINUTES)

    record = PhoneVerification(
        user_id=user_id,
        phone_number=phone_number,
        code_hash=code_hash,
        status="pending",
        attempts=0,
        max_attempts=settings.PHONE_VERIFICATION_MAX_ATTEMPTS,
        expires_at=expires_at,
        last_sent_at=now,
    )
    db.add(record)
    db.commit()

    message = f"Your Executive AI Agent verification code is {code}. It expires in {settings.PHONE_VERIFICATION_CODE_TTL_MINUTES} minutes."
    try:
        _send_sms(phone_number, message)
    except Exception as exc:
        if settings.ENV in ("production", "staging"):
            logger.error("Phone verification SMS failed: %s", exc)
            raise
        logger.warning("Phone verification SMS skipped in %s: %s", settings.ENV, exc)

    payload = {
        "ok": True,
        "status": "sent",
        "expires_at": expires_at.isoformat(),
    }
    if settings.ENV == "dev" and settings.PHONE_VERIFICATION_ALLOW_DEV_CODE_ECHO == "1":
        payload["code"] = code
    return payload


def verify_phone_code(db: Session, user_id: str, phone_number: str, code: str) -> dict:
    now = datetime.utcnow()
    phone_number = (phone_number or "").strip()
    if not phone_number or not code:
        raise ValueError("phone_number and code are required")

    record = (
        db.query(PhoneVerification)
        .filter(
            PhoneVerification.user_id == user_id,
            PhoneVerification.phone_number == phone_number,
            PhoneVerification.status == "pending",
        )
        .order_by(PhoneVerification.created_at.desc())
        .first()
    )

    if not record:
        raise ValueError("No pending verification found")

    if record.expires_at and record.expires_at < now:
        record.status = "expired"
        db.commit()
        raise ValueError("Verification code has expired")

    if record.attempts >= record.max_attempts:
        record.status = "locked"
        db.commit()
        raise ValueError("Too many attempts. Please request a new code")

    if _hash_code(code) != record.code_hash:
        record.attempts += 1
        if record.attempts >= record.max_attempts:
            record.status = "locked"
        db.commit()
        raise ValueError("Invalid verification code")

    record.status = "verified"
    record.verified_at = now
    db.commit()

    # Update user profile + preferences
    update_profile(
        db,
        user_id,
        {
            "phone": phone_number,
            "phone_verified_at": now.isoformat(),
        },
    )
    update_preferences(db, user_id, {"phone_verified": True})

    return {"ok": True, "status": "verified", "verified_at": now.isoformat()}
